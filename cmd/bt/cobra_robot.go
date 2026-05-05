package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/baseline"
	"github.com/seanmartinsmith/beadstui/pkg/bql"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/recipe"
)

// robotCmd is the parent command for all machine-readable subcommands.
var robotCmd = &cobra.Command{
	Use:   "robot",
	Short: "Machine-readable output for AI agents",
	Long:  "All subcommands under 'bt robot' produce structured output (JSON or TOON) for consumption by AI agents and scripts.",
	// Silence cobra's default usage/error printing on RunE failures so
	// unknown-subcommand errors land on stderr (via main()) without dumping
	// help to stdout. Robot-mode contract is stdout=structured-data only.
	// (bt-70cd)
	SilenceUsage:  true,
	SilenceErrors: true,
	// RunE handles the "bt robot <unknown>" case. Cobra's default when no
	// subcommand matches is to invoke the parent's Run with the leftover
	// args, which previously printed help to stdout and exited 0 — breaking
	// pipes to jq and shell scripts that check $?. With no args, keep the
	// human-friendly behavior (help on stdout). With args, return an error
	// so main() prints to stderr and exits non-zero. (bt-70cd)
	RunE: unknownSubcommandRunE,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Run root persistent pre-run first.
		if err := rootPersistentPreRun(cmd); err != nil {
			return err
		}
		// Resolve output shape (flag > alias > env > compact default).
		shape, err := resolveRobotOutputShape(robotFlagShape, robotFlagCompact, robotFlagFull)
		if err != nil {
			return err
		}
		robotOutputShape = shape
		// Mark robot mode.
		_ = os.Setenv("BT_ROBOT", "1")
		log.SetOutput(io.Discard)
		return nil
	},
}

// robotFlagLabelScope is the --label flag shared across robot subcommands.
var robotFlagLabelScope string

// robotFlagForceFullAnalysis is the --force-full-analysis flag.
var robotFlagForceFullAnalysis bool

// robotFlagAsOf is the --as-of flag for robot commands.
var robotFlagAsOf string

// robotFlagHistoryLimit is the --history-limit flag.
var robotFlagHistoryLimit int

// robotFlagDiffSince is the --diff-since flag.
var robotFlagDiffSince string

// robotFlagRecipe is the --recipe flag for robot commands.
var robotFlagRecipe string

// robotFlagBQL is the --bql flag for robot commands.
var robotFlagBQL string

// robotFlagSource is the --source flag: comma-separated list of project
// prefixes to scope robot output to. bt-mhwy.6. Filtering happens in
// robotPreRun (for handlers that call it) and inside robot list's RunE
// (which bypasses robotPreRun). Unknown prefixes produce an empty result,
// not an error — same contract as beads' other filter flags.
var robotFlagSource string

func init() {
	rootCmd.AddCommand(robotCmd)

	pf := robotCmd.PersistentFlags()
	pf.StringVar(&robotFlagLabelScope, "label", "", "Scope analysis to label's subgraph")
	pf.BoolVar(&robotFlagForceFullAnalysis, "force-full-analysis", false, "Compute all metrics regardless of graph size")
	pf.StringVar(&robotFlagAsOf, "as-of", "", "View state at point in time (commit SHA, branch, tag, or date)")
	pf.IntVar(&robotFlagHistoryLimit, "history-limit", 500, "Max commits to analyze (0 = unlimited)")
	pf.StringVar(&robotFlagDiffSince, "diff-since", "", "Show changes since historical point")
	pf.StringVarP(&robotFlagRecipe, "recipe", "r", "", "Apply named recipe filter")
	pf.StringVar(&robotFlagBQL, "bql", "", "BQL query to pre-filter issues")
	pf.StringVar(&robotFlagSource, "source", "", "Filter issues by source project (comma-separated, e.g. 'cass,bt'). Matches ID prefix or SourceRepo. Unknown prefixes yield empty results.")
	pf.StringVar(&robotFlagShape, "shape", "", "Output shape: compact (default) or full (env: BT_OUTPUT_SHAPE)")
	pf.BoolVar(&robotFlagCompact, "compact", false, "Alias for --shape=compact")
	pf.BoolVar(&robotFlagFull, "full", false, "Alias for --shape=full")
}

// robotPreRun loads issues and applies common robot pre-processing (label scope, recipe, BQL).
// Call this at the start of each robot subcommand's RunE.
func robotPreRun() (*robotCtx, error) {
	if err := loadIssues(); err != nil {
		if !flagGlobal {
			return nil, fmt.Errorf("%w (try --global if no local Dolt server is reachable)", err)
		}
		return nil, err
	}

	issues := appCtx.issues

	// Apply --source filter. Runs before BQL/label scope so every downstream
	// filter sees the already-narrowed set. Unknown projects produce empty
	// results silently; callers can distinguish from "no matches" via the
	// flag echo in list output.
	if robotFlagSource != "" {
		issues = filterBySource(issues, robotFlagSource)
	}

	// Apply --bql filter.
	if robotFlagBQL != "" {
		parsed, err := bql.Parse(robotFlagBQL)
		if err != nil {
			return nil, fmt.Errorf("BQL parse error: %w", err)
		}
		if err := bql.Validate(parsed); err != nil {
			return nil, fmt.Errorf("BQL validation error: %w", err)
		}
		issueMap := make(map[string]*model.Issue, len(issues))
		for i := range issues {
			issueMap[issues[i].ID] = &issues[i]
		}
		executor := bql.NewMemoryExecutor()
		issues = executor.Execute(parsed, issues, bql.ExecuteOpts{IssueMap: issueMap})
	}

	// Stable data hash (after repo filter but before recipes/label scope).
	dataHash := analysis.ComputeDataHash(issues)
	appCtx.dataHash = dataHash

	// Label subgraph scoping.
	var labelScopeContext *analysis.LabelHealth
	if robotFlagLabelScope != "" {
		sg := analysis.ComputeLabelSubgraph(issues, robotFlagLabelScope)
		if sg.IssueCount > 0 {
			subgraphIssues := make([]model.Issue, 0, len(sg.AllIssues))
			for _, id := range sg.AllIssues {
				if iss, ok := sg.IssueMap[id]; ok {
					subgraphIssues = append(subgraphIssues, iss)
				}
			}
			issues = subgraphIssues
			cfg := analysis.DefaultLabelHealthConfig()
			allHealth := analysis.ComputeAllLabelHealth(issues, cfg, time.Now().UTC(), nil)
			for i := range allHealth.Labels {
				if allHealth.Labels[i].Label == robotFlagLabelScope {
					labelScopeContext = &allHealth.Labels[i]
					break
				}
			}
		}
	}

	// Apply recipe filter if specified.
	if robotFlagRecipe != "" {
		recipeLoader, err := recipe.LoadDefault()
		if err != nil {
			return nil, fmt.Errorf("loading recipes: %w", err)
		}
		r := recipeLoader.Get(robotFlagRecipe)
		if r == nil {
			return nil, fmt.Errorf("unknown recipe '%s'", robotFlagRecipe)
		}
		issues = applyRecipeFilters(issues, r)
		issues = applyRecipeSort(issues, r)
	}

	appCtx.issues = issues

	projectDir, _ := os.Getwd()
	rc := newRobotCtx(issues, appCtx.issuesForSearch, dataHash, projectDir, appCtx.beadsPath, projectDir, labelScopeContext)
	return rc, nil
}

// --- Robot Subcommands ---

// bt robot triage
var robotTriageCmd = &cobra.Command{
	Use:   "triage",
	Short: "Output unified triage as JSON (the mega-command for AI agents)",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		byTrack, _ := cmd.Flags().GetBool("by-track")
		byLabel, _ := cmd.Flags().GetBool("by-label")
		rc.runTriage(false, byTrack, byLabel, robotFlagHistoryLimit, robotFlagAsOf, appCtx.asOfResolved)
		return nil // runTriage calls os.Exit
	},
}

// bt robot next
var robotNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Output only the top pick recommendation as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runTriage(true, false, false, robotFlagHistoryLimit, robotFlagAsOf, appCtx.asOfResolved)
		return nil
	},
}

// bt robot insights
var robotInsightsCmd = &cobra.Command{
	Use:   "insights",
	Short: "Output graph analysis and insights as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runInsights(robotFlagForceFullAnalysis, robotFlagAsOf, appCtx.asOfResolved, robotFlagLabelScope)
		return nil
	},
}

// bt robot plan
var robotPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Output dependency-respecting execution plan as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runPlan(robotFlagForceFullAnalysis, robotFlagAsOf, appCtx.asOfResolved, robotFlagLabelScope)
		return nil
	},
}

// bt robot priority
var robotPriorityCmd = &cobra.Command{
	Use:   "priority",
	Short: "Output priority recommendations as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		minConf, _ := cmd.Flags().GetFloat64("min-confidence")
		maxResults, _ := cmd.Flags().GetInt("max-results")
		byLabel, _ := cmd.Flags().GetString("by-label")
		byAssignee, _ := cmd.Flags().GetString("by-assignee")
		rc.runPriority(robotFlagForceFullAnalysis, robotFlagAsOf, appCtx.asOfResolved, robotFlagLabelScope, minConf, maxResults, byLabel, byAssignee)
		return nil
	},
}

// bt robot graph
var robotGraphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Output dependency graph as JSON/DOT/Mermaid",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		format, _ := cmd.Flags().GetString("graph-format")
		root, _ := cmd.Flags().GetString("graph-root")
		depth, _ := cmd.Flags().GetInt("graph-depth")
		rc.runRobotGraph(format, robotFlagLabelScope, root, depth)
		return nil
	},
}

// bt robot alerts
var robotAlertsCmd = &cobra.Command{
	Use:   "alerts",
	Short: "Output alerts (drift + proactive) as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		if describe, _ := cmd.Flags().GetBool("describe-types"); describe {
			rc.runDescribeAlertTypes()
			return nil
		}
		severity, _ := cmd.Flags().GetString("severity")
		alertType, _ := cmd.Flags().GetString("alert-type")
		alertLabel, _ := cmd.Flags().GetString("alert-label")
		rc.runAlerts(severity, alertType, alertLabel)
		return nil
	},
}

// bt robot search
var robotSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Output semantic search results as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			return err
		}
		query, _ := cmd.Flags().GetString("query")
		if query == "" && len(args) > 0 {
			query = strings.Join(args, " ")
		}
		if query == "" {
			return fmt.Errorf("search query required (use --query or pass as argument)")
		}
		// Set the flag for the shared search function.
		flagSearch = query
		flagSearchLimit, _ = cmd.Flags().GetInt("limit")
		flagSearchMode, _ = cmd.Flags().GetString("mode")
		flagSearchPreset, _ = cmd.Flags().GetString("preset")
		flagSearchWeights, _ = cmd.Flags().GetString("weights")
		runSearchCommand(appCtx.issues, appCtx.issuesForSearch, true)
		return nil
	},
}

// bt robot bql
var robotBQLCmd = &cobra.Command{
	Use:   "bql",
	Short: "Output BQL-filtered issues as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			if !flagGlobal {
				return fmt.Errorf("%w (try --global if no local Dolt server is reachable)", err)
			}
			return err
		}
		query, _ := cmd.Flags().GetString("query")
		if query == "" && len(args) > 0 {
			query = strings.Join(args, " ")
		}
		if query == "" {
			return fmt.Errorf("BQL query required (use --query or pass as argument)")
		}
		parsed, err := bql.Parse(query)
		if err != nil {
			return fmt.Errorf("BQL parse error: %w", err)
		}
		if err := bql.Validate(parsed); err != nil {
			return fmt.Errorf("BQL validation error: %w", err)
		}
		issues := appCtx.issues
		issueMap := make(map[string]*model.Issue, len(issues))
		for i := range issues {
			issueMap[issues[i].ID] = &issues[i]
		}
		executor := bql.NewMemoryExecutor()
		filtered := executor.Execute(parsed, issues, bql.ExecuteOpts{IssueMap: issueMap})

		// Apply --offset and --limit for pagination.
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		totalCount := len(filtered)
		if offset > 0 {
			if offset >= len(filtered) {
				filtered = nil
			} else {
				filtered = filtered[offset:]
			}
		}
		if limit > 0 && len(filtered) > limit {
			filtered = filtered[:limit]
		}

		bqlHash := analysis.ComputeDataHash(filtered)
		output := struct {
			RobotEnvelope
			Query      string        `json:"query"`
			TotalCount int           `json:"total_count"`
			Count      int           `json:"count"`
			Offset     int           `json:"offset,omitempty"`
			Issues     []model.Issue `json:"issues"`
		}{
			RobotEnvelope: NewRobotEnvelope(bqlHash),
			Query:         query,
			TotalCount:    totalCount,
			Count:         len(filtered),
			Offset:        offset,
			Issues:        filtered,
		}
		enc := newRobotEncoder(os.Stdout)
		if err := enc.Encode(output); err != nil {
			return fmt.Errorf("encoding robot-bql: %w", err)
		}
		os.Exit(0)
		return nil
	},
}

// bt robot history
var robotHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Output bead-to-commit correlations as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		beadID, _ := cmd.Flags().GetString("bead")
		since, _ := cmd.Flags().GetString("since")
		minConf, _ := cmd.Flags().GetFloat64("min-confidence")
		rc.runHistory(beadID, since, robotFlagHistoryLimit, minConf)
		return nil
	},
}

// bt robot suggest
var robotSuggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Output smart suggestions as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		suggestType, _ := cmd.Flags().GetString("type")
		confidence, _ := cmd.Flags().GetFloat64("min-confidence")
		bead, _ := cmd.Flags().GetString("bead")
		rc.runSuggest(suggestType, confidence, bead)
		return nil
	},
}

// bt robot diff
var robotDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Output diff as JSON (use --diff-since)",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		since, _ := cmd.Flags().GetString("since")
		if since == "" {
			since = robotFlagDiffSince
		}
		if since == "" {
			return fmt.Errorf("--since is required for robot diff")
		}
		rc.runDiffSince(since, true, robotFlagAsOf, appCtx.asOfResolved)
		return nil
	},
}

// bt robot metrics
var robotMetricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Output performance metrics as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runMetrics()
		return nil
	},
}

// bt robot recipes
var robotRecipesCmd = &cobra.Command{
	Use:   "recipes",
	Short: "Output available recipes as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		recipeLoader, err := recipe.LoadDefault()
		if err != nil {
			recipeLoader = recipe.NewLoader()
		}
		summaries := recipeLoader.ListSummaries()
		sort.Slice(summaries, func(i, j int) bool {
			return summaries[i].Name < summaries[j].Name
		})
		output := struct {
			Recipes []recipe.RecipeSummary `json:"recipes"`
		}{
			Recipes: summaries,
		}
		encoder := newRobotEncoder(os.Stdout)
		if err := encoder.Encode(output); err != nil {
			return fmt.Errorf("encoding recipes: %w", err)
		}
		os.Exit(0)
		return nil
	},
}

// bt robot schema
var robotSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Output JSON Schema definitions for all robot commands",
	Long: "Output JSON Schema definitions for all robot command outputs. " +
		"Use --command to fetch a single schema. Accepts both the cobra path form " +
		"(e.g., --command=triage) and the legacy robot-prefixed form " +
		"(e.g., --command=robot-triage); the cobra path form is preferred.",
	RunE: func(cmd *cobra.Command, args []string) error {
		schemas := generateRobotSchemas()
		schemaCmd, _ := cmd.Flags().GetString("command")
		if schemaCmd != "" {
			// Accept both cobra path form ("triage") and legacy form ("robot-triage").
			// Normalize: if the key is not found as-is, try prepending "robot-".
			resolvedCmd := schemaCmd
			if _, ok := schemas.Commands[resolvedCmd]; !ok {
				withPrefix := "robot-" + resolvedCmd
				if _, ok2 := schemas.Commands[withPrefix]; ok2 {
					resolvedCmd = withPrefix
				}
			}
			if schema, ok := schemas.Commands[resolvedCmd]; ok {
				singleOutput := map[string]interface{}{
					"schema_version": schemas.SchemaVersion,
					"generated_at":   schemas.GeneratedAt,
					"command":        resolvedCmd,
					"schema":         schema,
				}
				encoder := newRobotEncoder(os.Stdout)
				if err := encoder.Encode(singleOutput); err != nil {
					return fmt.Errorf("encoding schema: %w", err)
				}
				os.Exit(0)
				return nil
			}
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", schemaCmd)
			fmt.Fprintln(os.Stderr, "Available commands (use cobra path form, e.g., 'triage' not 'robot-triage'):")
			for c := range schemas.Commands {
				fmt.Fprintf(os.Stderr, "  %s\n", c)
			}
			os.Exit(1)
		}
		encoder := newRobotEncoder(os.Stdout)
		if err := encoder.Encode(schemas); err != nil {
			return fmt.Errorf("encoding schemas: %w", err)
		}
		os.Exit(0)
		return nil
	},
}

// bt robot docs
var robotDocsCmd = &cobra.Command{
	Use:   "docs [topic]",
	Short: "Machine-readable JSON docs for AI agents",
	Long:  "Topics: guide, commands, examples, env, exit-codes, all",
	RunE: func(cmd *cobra.Command, args []string) error {
		topic, _ := cmd.Flags().GetString("topic")
		if topic == "" && len(args) > 0 {
			topic = args[0]
		}
		if topic == "" {
			topic = "all"
		}
		docs := generateRobotDocs(topic)
		encoder := newRobotEncoder(os.Stdout)
		if err := encoder.Encode(docs); err != nil {
			return fmt.Errorf("encoding robot-docs: %w", err)
		}
		if _, hasErr := docs["error"]; hasErr {
			os.Exit(2)
		}
		os.Exit(0)
		return nil
	},
}

// bt robot help
var robotHelpCmd = &cobra.Command{
	Use:   "help",
	Short: "Show AI agent help",
	Run: func(cmd *cobra.Command, args []string) {
		printRobotHelp()
	},
}

// bt robot drift
var robotDriftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Output drift check as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runCheckDrift(true, robotFlagForceFullAnalysis)
		return nil
	},
}

// unknownSubcommandRunE returns a RunE that prints help on stdout when
// invoked bare and returns an error (routed to stderr + non-zero exit by
// main()) for any unknown subcommand. Used on parent-only command groups
// under `bt robot` so unknown subcommands no longer dump help to stdout
// with exit 0. (bt-70cd)
func unknownSubcommandRunE(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	return fmt.Errorf("unknown subcommand %q for %q\nRun '%s --help' for usage", args[0], cmd.CommandPath(), cmd.CommandPath())
}

// --- Robot Sprint Subcommands ---

var robotSprintCmd = &cobra.Command{
	Use:           "sprint",
	Short:         "Sprint-related robot commands",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          unknownSubcommandRunE,
}

var robotSprintListCmd = &cobra.Command{
	Use:   "list",
	Short: "Output sprints as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runSprintListOrShow("")
		return nil
	},
}

var robotSprintShowCmd = &cobra.Command{
	Use:   "show [sprint-id]",
	Short: "Output specific sprint details as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		sprintID := ""
		if len(args) > 0 {
			sprintID = args[0]
		}
		rc.runSprintListOrShow(sprintID)
		return nil
	},
}

// --- Robot Labels Subcommands ---

var robotLabelsCmd = &cobra.Command{
	Use:           "labels",
	Short:         "Label analysis robot commands",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          unknownSubcommandRunE,
}

var robotLabelsHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Output label health metrics as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runLabelHealth()
		return nil
	},
}

var robotLabelsFlowCmd = &cobra.Command{
	Use:   "flow",
	Short: "Output cross-label dependency flow as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runLabelFlow()
		return nil
	},
}

var robotLabelsAttentionCmd = &cobra.Command{
	Use:   "attention",
	Short: "Output attention-ranked labels as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		rc.runLabelAttention(limit)
		return nil
	},
}

// --- Robot Files Subcommands ---

var robotFilesCmd = &cobra.Command{
	Use:           "files",
	Short:         "File-bead analysis robot commands",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          unknownSubcommandRunE,
}

var robotFilesBeadsCmd = &cobra.Command{
	Use:   "beads [path]",
	Short: "Output beads that touched a file path as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		filePath := ""
		if len(args) > 0 {
			filePath = args[0]
		}
		fileBeadsLimit, _ := cmd.Flags().GetInt("limit")
		rc.runFileBeads(filePath, false, fileBeadsLimit, 0, robotFlagHistoryLimit)
		return nil
	},
}

var robotFilesHotspotsCmd = &cobra.Command{
	Use:   "hotspots",
	Short: "Output files touched by most beads as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		hotspotsLimit, _ := cmd.Flags().GetInt("limit")
		rc.runFileBeads("", true, 0, hotspotsLimit, robotFlagHistoryLimit)
		return nil
	},
}

var robotFilesRelationsCmd = &cobra.Command{
	Use:   "relations [path]",
	Short: "Output files that frequently co-change with the given file",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		filePath := ""
		if len(args) > 0 {
			filePath = args[0]
		}
		threshold, _ := cmd.Flags().GetFloat64("threshold")
		relLimit, _ := cmd.Flags().GetInt("limit")
		rc.runFileRelations(filePath, threshold, relLimit, robotFlagHistoryLimit)
		return nil
	},
}

// --- Robot Correlation Subcommands ---

var robotCorrelationCmd = &cobra.Command{
	Use:           "correlation",
	Short:         "Correlation audit robot commands",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          unknownSubcommandRunE,
}

var robotCorrelationExplainCmd = &cobra.Command{
	Use:   "explain [SHA:beadID]",
	Short: "Explain why a commit is linked to a bead",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}
		rc.runCorrelationAudit(arg, "", "", false, "", "")
		return nil
	},
}

var robotCorrelationConfirmCmd = &cobra.Command{
	Use:   "confirm [SHA:beadID]",
	Short: "Confirm a correlation is correct",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}
		by, _ := cmd.Flags().GetString("by")
		reason, _ := cmd.Flags().GetString("reason")
		rc.runCorrelationAudit("", arg, "", false, by, reason)
		return nil
	},
}

var robotCorrelationRejectCmd = &cobra.Command{
	Use:   "reject [SHA:beadID]",
	Short: "Reject an incorrect correlation",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}
		by, _ := cmd.Flags().GetString("by")
		reason, _ := cmd.Flags().GetString("reason")
		rc.runCorrelationAudit("", "", arg, false, by, reason)
		return nil
	},
}

var robotCorrelationStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Output correlation feedback statistics as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runCorrelationAudit("", "", "", true, "", "")
		return nil
	},
}

// --- Remaining Robot Commands ---

// bt robot impact
var robotImpactCmd = &cobra.Command{
	Use:   "impact [paths]",
	Short: "Analyze impact of modifying files (comma-separated paths)",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		paths := ""
		if len(args) > 0 {
			paths = strings.Join(args, ",")
		}
		rc.runImpact(paths, robotFlagHistoryLimit)
		return nil
	},
}

// bt robot related
var robotRelatedCmd = &cobra.Command{
	Use:   "related [bead-id]",
	Short: "Output beads related to a specific bead ID as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		beadID := ""
		if len(args) > 0 {
			beadID = args[0]
		}
		minRelevance, _ := cmd.Flags().GetInt("min-relevance")
		maxResults, _ := cmd.Flags().GetInt("max-results")
		includeClosed, _ := cmd.Flags().GetBool("include-closed")
		rc.runRelatedWork(beadID, minRelevance, maxResults, robotFlagHistoryLimit, includeClosed)
		return nil
	},
}

// bt robot blocker-chain
var robotBlockerChainCmd = &cobra.Command{
	Use:   "blocker-chain [bead-id]",
	Short: "Output full blocker chain analysis as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		beadID := ""
		if len(args) > 0 {
			beadID = args[0]
		}
		rc.runBlockerChain(beadID)
		return nil
	},
}

// bt robot impact-network
var robotImpactNetworkCmd = &cobra.Command{
	Use:   "impact-network [bead-id]",
	Short: "Output bead impact network as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		beadID := ""
		if len(args) > 0 {
			beadID = args[0]
		}
		depth, _ := cmd.Flags().GetInt("depth")
		rc.runImpactNetwork(beadID, depth, robotFlagHistoryLimit)
		return nil
	},
}

// bt robot causality
var robotCausalityCmd = &cobra.Command{
	Use:   "causality [bead-id]",
	Short: "Output causal chain analysis as JSON",
	Long: "Traces the causal chain for a bead: why it exists, what it unblocks, and what it was blocked by. " +
		"Requires --global when run outside a workspace (no local .beads/ project), because causality " +
		"analysis needs to resolve cross-project dependency chains from the shared Dolt server.",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		beadID := ""
		if len(args) > 0 {
			beadID = args[0]
		}
		rc.runCausality(beadID, robotFlagHistoryLimit)
		return nil
	},
}

// bt robot orphans
var robotOrphansCmd = &cobra.Command{
	Use:   "orphans",
	Short: "Output orphan commit candidates as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		minScore, _ := cmd.Flags().GetInt("min-score")
		rc.runOrphans(robotFlagHistoryLimit, minScore)
		return nil
	},
}

// bt robot portfolio
var robotPortfolioCmd = &cobra.Command{
	Use:   "portfolio",
	Short: "Output per-project health aggregates as JSON",
	Long: "Returns one PortfolioRecord per project with counts, priority breakdown (P0/P1 open), velocity with trend, composite health score, top blocker, and stalest issue. " +
		"Use --global to aggregate across every project on the shared Dolt server. " +
		"Output is already compact-by-construction; --shape is accepted but no-op (envelope.schema is always portfolio.v1).",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runPortfolio()
		return nil
	},
}

// bt robot pairs
var robotPairsCmd = &cobra.Command{
	Use:   "pairs",
	Short: "Detect cross-project paired beads (same ID suffix, different prefixes)",
	Long: "Returns one PairRecord per paired set — canonical bead (first-created) plus mirrors, " +
		"with drift flags for status, priority, and closed/open mismatches. " +
		"Requires --global because pair detection is inherently cross-project. " +
		"--schema=v1 is the Phase 1 default; v2 flips the intent signal from suffix match to dep edge.",
	RunE: func(cmd *cobra.Command, args []string) error {
		schema, err := pairsValidate(robotFlagSchema, robotFlagOrphaned)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runPairs(schema)
		return nil
	},
}

// bt robot refs
var robotRefsCmd = &cobra.Command{
	Use:   "refs",
	Short: "Detect cross-project bead references in prose and dep fields",
	Long: "Scans description, notes, comments, and external: dependencies for cross-project bead IDs " +
		"and validates each against the global set. Flags: broken (target missing), " +
		"stale (target closed), orphaned_child (target's parent closed but target still open), " +
		"cross_project (always present; v1 surfaces cross-project refs only). " +
		"Requires --global. --schema=v1 is the Phase 1 default; --sigils is v2-only.",
	RunE: func(cmd *cobra.Command, args []string) error {
		schema, sigils, err := refsValidate(robotFlagSchema, robotFlagSigils)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		rc.runRefs(schema, sigils)
		return nil
	},
}

// bt robot forecast
var robotForecastCmd = &cobra.Command{
	Use:   "forecast [bead-id|all]",
	Short: "Output ETA forecast as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		target := ""
		if len(args) > 0 {
			target = args[0]
		}
		forecastLabel, _ := cmd.Flags().GetString("forecast-label")
		forecastSprint, _ := cmd.Flags().GetString("forecast-sprint")
		forecastAgents, _ := cmd.Flags().GetInt("agents")
		rc.runForecast(target, forecastLabel, forecastSprint, forecastAgents)
		return nil
	},
}

// bt robot burndown
var robotBurndownCmd = &cobra.Command{
	Use:   "burndown [sprint-id|current]",
	Short: "Output burndown data as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		sprintID := ""
		if len(args) > 0 {
			sprintID = args[0]
		}
		rc.runBurndown(sprintID)
		return nil
	},
}

// bt robot capacity
var robotCapacityCmd = &cobra.Command{
	Use:   "capacity",
	Short: "Output capacity simulation as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		agents, _ := cmd.Flags().GetInt("agents")
		capacityLabel, _ := cmd.Flags().GetString("capacity-label")
		rc.runCapacity(agents, capacityLabel)
		return nil
	},
}

// bt robot baseline save
var robotBaselineSaveCmd = &cobra.Command{
	Use:   "save [description]",
	Short: "Save current metrics as baseline",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, err := robotPreRun()
		if err != nil {
			return err
		}
		desc := ""
		if len(args) > 0 {
			desc = strings.Join(args, " ")
		}
		rc.runSaveBaseline(desc, robotFlagForceFullAnalysis)
		return nil
	},
}

// bt robot baseline info
var robotBaselineInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show information about the current baseline",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := os.Getwd()
		baselinePath := baseline.DefaultPath(projectDir)
		if !baseline.Exists(baselinePath) {
			fmt.Println("No baseline found.")
			fmt.Println("Create one with: bt baseline save \"description\"")
			os.Exit(0)
		}
		bl, err := baseline.Load(baselinePath)
		if err != nil {
			return fmt.Errorf("loading baseline: %w", err)
		}
		fmt.Print(bl.Summary())
		os.Exit(0)
		return nil
	},
}

// --- Register all robot subcommands ---

func init() {
	// triage
	robotTriageCmd.Flags().Bool("by-track", false, "Group triage recommendations by execution track")
	robotTriageCmd.Flags().Bool("by-label", false, "Group triage recommendations by label")
	robotCmd.AddCommand(robotTriageCmd)

	// next
	robotCmd.AddCommand(robotNextCmd)

	// insights
	robotCmd.AddCommand(robotInsightsCmd)

	// plan
	robotCmd.AddCommand(robotPlanCmd)

	// priority
	robotPriorityCmd.Flags().Float64("min-confidence", 0.0, "Filter by minimum confidence (0.0-1.0)")
	robotPriorityCmd.Flags().Int("max-results", 0, "Limit output count (0 = use defaults)")
	robotPriorityCmd.Flags().String("by-label", "", "Filter by label (exact match)")
	robotPriorityCmd.Flags().String("by-assignee", "", "Filter by assignee (exact match)")
	robotCmd.AddCommand(robotPriorityCmd)

	// graph
	robotGraphCmd.Flags().String("graph-format", "json", "Graph output format: json, dot, mermaid")
	robotGraphCmd.Flags().String("graph-root", "", "Subgraph from specific root issue ID")
	robotGraphCmd.Flags().Int("graph-depth", 0, "Max depth for subgraph (0 = unlimited)")
	robotCmd.AddCommand(robotGraphCmd)

	// alerts
	robotAlertsCmd.Flags().String("severity", "", "Filter by severity (info|warning|critical)")
	robotAlertsCmd.Flags().String("alert-type", "", alertTypeFilterHelp())
	robotAlertsCmd.Flags().String("alert-label", "", "Filter by label match")
	robotAlertsCmd.Flags().Bool("describe-types", false, "Emit the full alert-type taxonomy with plain-English definitions as JSON, then exit")
	robotCmd.AddCommand(robotAlertsCmd)

	// search
	robotSearchCmd.Flags().String("query", "", "Search query")
	robotSearchCmd.Flags().Int("limit", 10, "Max results")
	robotSearchCmd.Flags().String("mode", "", "Search ranking mode: text or hybrid")
	robotSearchCmd.Flags().String("preset", "", "Hybrid preset name")
	robotSearchCmd.Flags().String("weights", "", "Hybrid weights JSON")
	robotCmd.AddCommand(robotSearchCmd)

	// bql
	robotBQLCmd.Flags().String("query", "", "BQL query string")
	robotBQLCmd.Flags().Int("limit", 0, "Max issues to return (0 = unlimited)")
	robotBQLCmd.Flags().Int("offset", 0, "Skip first N issues (for pagination)")
	robotCmd.AddCommand(robotBQLCmd)

	// list
	robotListCmd.Flags().String("status", "", "Filter by status: open,blocked,in_progress,closed (comma-separated)")
	robotListCmd.Flags().String("priority", "", "Filter by priority: single (e.g. '0') or range (e.g. '0-1')")
	robotListCmd.Flags().String("type", "", "Filter by issue type: bug, feature, task, epic, chore, decision, merge-request, molecule, gate, convoy (aliases: mr, feat, mol, dec, adr)")
	robotListCmd.Flags().String("has-label", "", "Filter to issues with this label (exact match)")
	robotListCmd.Flags().Int("limit", 100, "Max issues to return (0 = unlimited)")
	robotCmd.AddCommand(robotListCmd)

	// history
	robotHistoryCmd.Flags().String("bead", "", "Show history for specific bead ID")
	robotHistoryCmd.Flags().String("since", "", "Limit history to commits after this date/ref")
	robotHistoryCmd.Flags().Float64("min-confidence", 0.0, "Filter correlations by minimum confidence")
	robotCmd.AddCommand(robotHistoryCmd)

	// suggest
	robotSuggestCmd.Flags().String("type", "", "Filter suggestions by type: duplicate, dependency, label, cycle")
	robotSuggestCmd.Flags().Float64("min-confidence", 0.0, "Minimum confidence for suggestions (0.0-1.0)")
	robotSuggestCmd.Flags().String("bead", "", "Filter suggestions for specific bead ID")
	robotCmd.AddCommand(robotSuggestCmd)

	// diff
	robotDiffCmd.Flags().String("since", "", "Show changes since historical point")
	robotCmd.AddCommand(robotDiffCmd)

	// metrics
	robotCmd.AddCommand(robotMetricsCmd)

	// recipes
	robotCmd.AddCommand(robotRecipesCmd)

	// schema
	robotSchemaCmd.Flags().String("command", "", "Output schema for specific command only")
	robotCmd.AddCommand(robotSchemaCmd)

	// docs
	robotDocsCmd.Flags().String("topic", "", "Doc topic: guide, commands, examples, env, exit-codes, all")
	robotCmd.AddCommand(robotDocsCmd)

	// help (robot-specific)
	robotCmd.AddCommand(robotHelpCmd)

	// health (disk-only diagnostic, paired with `bt status`; bt-uu73)
	robotCmd.AddCommand(robotHealthCmd)

	// drift
	robotCmd.AddCommand(robotDriftCmd)

	// sprint
	robotSprintCmd.AddCommand(robotSprintListCmd)
	robotSprintCmd.AddCommand(robotSprintShowCmd)
	robotCmd.AddCommand(robotSprintCmd)

	// labels
	robotLabelsAttentionCmd.Flags().Int("limit", 5, "Limit number of labels")
	robotLabelsCmd.AddCommand(robotLabelsHealthCmd)
	robotLabelsCmd.AddCommand(robotLabelsFlowCmd)
	robotLabelsCmd.AddCommand(robotLabelsAttentionCmd)
	robotCmd.AddCommand(robotLabelsCmd)

	// files
	robotFilesBeadsCmd.Flags().Int("limit", 20, "Max closed beads to show")
	robotFilesHotspotsCmd.Flags().Int("limit", 10, "Max hotspots to show")
	robotFilesRelationsCmd.Flags().Float64("threshold", 0.5, "Minimum correlation threshold")
	robotFilesRelationsCmd.Flags().Int("limit", 10, "Max related files to show")
	robotFilesCmd.AddCommand(robotFilesBeadsCmd)
	robotFilesCmd.AddCommand(robotFilesHotspotsCmd)
	robotFilesCmd.AddCommand(robotFilesRelationsCmd)
	robotCmd.AddCommand(robotFilesCmd)

	// correlation
	robotCorrelationConfirmCmd.Flags().String("by", "", "Agent/user identifier")
	robotCorrelationConfirmCmd.Flags().String("reason", "", "Reason for feedback")
	robotCorrelationRejectCmd.Flags().String("by", "", "Agent/user identifier")
	robotCorrelationRejectCmd.Flags().String("reason", "", "Reason for feedback")
	robotCorrelationCmd.AddCommand(robotCorrelationExplainCmd)
	robotCorrelationCmd.AddCommand(robotCorrelationConfirmCmd)
	robotCorrelationCmd.AddCommand(robotCorrelationRejectCmd)
	robotCorrelationCmd.AddCommand(robotCorrelationStatsCmd)
	robotCmd.AddCommand(robotCorrelationCmd)

	// impact
	robotCmd.AddCommand(robotImpactCmd)

	// related
	robotRelatedCmd.Flags().Int("min-relevance", 20, "Minimum relevance score (0-100)")
	robotRelatedCmd.Flags().Int("max-results", 10, "Max results per category")
	robotRelatedCmd.Flags().Bool("include-closed", false, "Include closed beads")
	robotCmd.AddCommand(robotRelatedCmd)

	// blocker-chain
	robotCmd.AddCommand(robotBlockerChainCmd)

	// impact-network
	robotImpactNetworkCmd.Flags().Int("depth", 2, "Depth of subnetwork (1-3)")
	robotCmd.AddCommand(robotImpactNetworkCmd)

	// causality
	robotCmd.AddCommand(robotCausalityCmd)

	// orphans
	robotOrphansCmd.Flags().Int("min-score", 30, "Minimum suspicion score (0-100)")
	robotCmd.AddCommand(robotOrphansCmd)

	// portfolio
	robotCmd.AddCommand(robotPortfolioCmd)

	// pairs
	robotPairsCmd.Flags().StringVar(&robotFlagSchema, "schema", "", "Projection schema (v1|v2). Default v1 in Phase 1; flips to v2 once pair.v2 reader ships. Env: BT_OUTPUT_SCHEMA.")
	robotPairsCmd.Flags().BoolVar(&robotFlagOrphaned, "orphaned", false, "Under --schema=v1, emit a JSONL checklist of v1-detected pairs missing the cross-prefix dep edge v2 requires. Read-only: lists the bd dep add commands to run manually.")
	robotCmd.AddCommand(robotPairsCmd)

	// refs
	robotRefsCmd.Flags().StringVar(&robotFlagSchema, "schema", "", "Projection schema (v1|v2). Default v1 in Phase 1; flips to v2 once ref.v2 reader ships. Env: BT_OUTPUT_SCHEMA.")
	robotRefsCmd.Flags().StringVar(&robotFlagSigils, "sigils", "", "Sigil recognition mode (strict|verb|permissive). Requires --schema=v2. Env: BT_SIGIL_MODE.")
	robotCmd.AddCommand(robotRefsCmd)

	// forecast
	robotForecastCmd.Flags().String("forecast-label", "", "Filter forecast by label")
	robotForecastCmd.Flags().String("forecast-sprint", "", "Filter forecast by sprint ID")
	robotForecastCmd.Flags().Int("agents", 1, "Number of parallel agents")
	robotCmd.AddCommand(robotForecastCmd)

	// burndown
	robotCmd.AddCommand(robotBurndownCmd)

	// capacity
	robotCapacityCmd.Flags().Int("agents", 1, "Number of parallel agents")
	robotCapacityCmd.Flags().String("capacity-label", "", "Filter by label")
	robotCmd.AddCommand(robotCapacityCmd)

	// baseline (under robot for robot-friendly access)
	robotBaselineCmd := &cobra.Command{
		Use:           "baseline",
		Short:         "Baseline management",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          unknownSubcommandRunE,
	}
	robotBaselineCmd.AddCommand(robotBaselineSaveCmd)
	robotBaselineCmd.AddCommand(robotBaselineInfoCmd)
	robotCmd.AddCommand(robotBaselineCmd)
}
