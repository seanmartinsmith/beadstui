package main

import (
	"context"
	"errors"
	flag "github.com/spf13/pflag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	json "github.com/goccy/go-json"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/internal/doltctl"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/bql"
	"github.com/seanmartinsmith/beadstui/pkg/baseline"
	"github.com/seanmartinsmith/beadstui/pkg/export"
	"github.com/seanmartinsmith/beadstui/pkg/hooks"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/recipe"
	"github.com/seanmartinsmith/beadstui/pkg/search"
	"github.com/seanmartinsmith/beadstui/pkg/ui"
	"github.com/seanmartinsmith/beadstui/pkg/version"
	"github.com/seanmartinsmith/beadstui/pkg/watcher"
	"github.com/seanmartinsmith/beadstui/pkg/workspace"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cpuProfile := flag.String("cpu-profile", "", "Write CPU profile to file")
	help := flag.Bool("help", false, "Show help")
	versionFlag := flag.Bool("version", false, "Show version")
	// Update flags (bv-182)
	updateFlag := flag.Bool("update", false, "Update bt to the latest version")
	checkUpdateFlag := flag.Bool("check-update", false, "Check if a new version is available")
	rollbackFlag := flag.Bool("rollback", false, "Rollback to the previous version (from backup)")
	yesFlag := flag.Bool("yes", false, "Skip confirmation prompts (use with --update)")
	exportFile := flag.String("export-md", "", "Export issues to a Markdown file (e.g., report.md)")
	robotHelp := flag.Bool("robot-help", false, "Show AI agent help")
	robotDocs := flag.String("robot-docs", "", "Machine-readable JSON docs for AI agents. Topics: guide, commands, examples, env, exit-codes, all")
	outputFormat := flag.String("format", "", "Structured output format for --robot-* commands: json or toon (env: BT_OUTPUT_FORMAT, TOON_DEFAULT_FORMAT)")
	toonStats := flag.Bool("stats", false, "Show JSON vs TOON token estimates on stderr (env: TOON_STATS=1)")
	robotInsights := flag.Bool("robot-insights", false, "Output graph analysis and insights as JSON for AI agents")
	robotPlan := flag.Bool("robot-plan", false, "Output dependency-respecting execution plan as JSON for AI agents")
	robotPriority := flag.Bool("robot-priority", false, "Output priority recommendations as JSON for AI agents")
	robotTriage := flag.Bool("robot-triage", false, "Output unified triage as JSON (the mega-command for AI agents)")
	robotTriageByTrack := flag.Bool("robot-triage-by-track", false, "Group triage recommendations by execution track (bv-87)")
	robotTriageByLabel := flag.Bool("robot-triage-by-label", false, "Group triage recommendations by label (bv-87)")
	robotNext := flag.Bool("robot-next", false, "Output only the top pick recommendation as JSON (minimal triage)")
	robotDiff := flag.Bool("robot-diff", false, "Output diff as JSON (use with --diff-since)")
	robotRecipes := flag.Bool("robot-recipes", false, "Output available recipes as JSON for AI agents")
	robotLabelHealth := flag.Bool("robot-label-health", false, "Output label health metrics as JSON for AI agents")
	robotLabelFlow := flag.Bool("robot-label-flow", false, "Output cross-label dependency flow as JSON for AI agents")
	robotLabelAttention := flag.Bool("robot-label-attention", false, "Output attention-ranked labels as JSON for AI agents")
	attentionLimit := flag.Int("attention-limit", 5, "Limit number of labels in --robot-label-attention output")
	robotAlerts := flag.Bool("robot-alerts", false, "Output alerts (drift + proactive) as JSON for AI agents")
	robotMetrics := flag.Bool("robot-metrics", false, "Output performance metrics (timing, cache, memory) as JSON")
	// JSON Schema for robot outputs (bd-2kxo)
	robotSchema := flag.Bool("robot-schema", false, "Output JSON Schema definitions for all robot commands")
	schemaCommand := flag.String("schema-command", "", "Output schema for specific command only (e.g., robot-triage)")
	// Smart suggestions (bv-180)
	robotSuggest := flag.Bool("robot-suggest", false, "Output smart suggestions (duplicates, dependencies, labels, cycles) as JSON")
	suggestType := flag.String("suggest-type", "", "Filter suggestions by type: duplicate, dependency, label, cycle")
	suggestConfidence := flag.Float64("suggest-confidence", 0.0, "Minimum confidence for suggestions (0.0-1.0)")
	suggestBead := flag.String("suggest-bead", "", "Filter suggestions for specific bead ID")
	// Graph export (bv-136)
	robotGraph := flag.Bool("robot-graph", false, "Output dependency graph as JSON/DOT/Mermaid for AI agents")
	graphFormat := flag.String("graph-format", "json", "Graph output format: json, dot, mermaid")
	graphRoot := flag.String("graph-root", "", "Subgraph from specific root issue ID")
	graphDepth := flag.Int("graph-depth", 0, "Max depth for subgraph (0 = unlimited)")
	// Graph snapshot export (bv-94)
	exportGraph := flag.String("export-graph", "", "Export graph: .html for interactive, .png/.svg for static (auto-names if empty)")
	graphPreset := flag.String("graph-preset", "compact", "Graph layout preset: compact (default) or roomy")
	graphTitle := flag.String("graph-title", "", "Title for graph export (default: project name)")
	// Robot output filters (bv-84)
	robotMinConf := flag.Float64("robot-min-confidence", 0.0, "Filter robot outputs by minimum confidence (0.0-1.0)")
	robotMaxResults := flag.Int("robot-max-results", 0, "Limit robot output count (0 = use defaults)")
	robotByLabel := flag.String("robot-by-label", "", "Filter robot outputs by label (exact match)")
	robotByAssignee := flag.String("robot-by-assignee", "", "Filter robot outputs by assignee (exact match)")
	// Label subgraph scoping (bv-122)
	labelScope := flag.String("label", "", "Scope analysis to label's subgraph (affects --robot-insights, --robot-plan, --robot-priority)")
	alertSeverity := flag.String("severity", "", "Filter robot alerts by severity (info|warning|critical)")
	alertType := flag.String("alert-type", "", "Filter robot alerts by alert type (e.g., stale_issue)")
	alertLabel := flag.String("alert-label", "", "Filter robot alerts by label match")
	recipeName := flag.StringP("recipe", "r", "", "Apply named recipe (e.g., triage, actionable, high-impact)")
	bqlQuery := flag.String("bql", "", "BQL query to pre-filter issues (e.g., 'status:open priority<P2')")
	robotBQL := flag.Bool("robot-bql", false, "Output BQL-filtered issues as JSON for AI agents (use with --bql)")
	semanticQuery := flag.String("search", "", "Semantic search query (vector-based; builds/updates index on first run)")
	robotSearch := flag.Bool("robot-search", false, "Output semantic search results as JSON for AI agents (use with --search)")
	searchLimit := flag.Int("search-limit", 10, "Max results for --search/--robot-search")
	searchMode := flag.String("search-mode", "", "Search ranking mode: text or hybrid (default: BT_SEARCH_MODE or text)")
	searchPreset := flag.String("search-preset", "", "Hybrid preset name (default: BT_SEARCH_PRESET or default)")
	searchWeights := flag.String("search-weights", "", "Hybrid weights JSON (overrides preset; keys: text,pagerank,status,impact,priority,recency)")
	diffSince := flag.String("diff-since", "", "Show changes since historical point (commit SHA, branch, tag, or date)")
	asOf := flag.String("as-of", "", "View state at point in time (commit SHA, branch, tag, or date)")
	forceFullAnalysis := flag.Bool("force-full-analysis", false, "Compute all metrics regardless of graph size (may be slow for large graphs)")
	profileStartup := flag.Bool("profile-startup", false, "Output detailed startup timing profile for diagnostics")
	profileJSON := flag.Bool("profile-json", false, "Output profile in JSON format (use with --profile-startup)")
	noHooks := flag.Bool("no-hooks", false, "Skip running hooks during export")
	workspaceConfig := flag.String("workspace", "", "Load issues from workspace config file (.bt/workspace.yaml)")
	repoFilter := flag.String("repo", "", "Filter issues by repository prefix (e.g., 'api-' or 'api')")
	saveBaseline := flag.String("save-baseline", "", "Save current metrics as baseline with optional description")
	baselineInfo := flag.Bool("baseline-info", false, "Show information about the current baseline")
	checkDrift := flag.Bool("check-drift", false, "Check for drift from baseline (exit codes: 0=OK, 1=critical, 2=warning)")
	robotDriftCheck := flag.Bool("robot-drift", false, "Output drift check as JSON (use with --check-drift)")
	robotHistory := flag.Bool("robot-history", false, "Output bead-to-commit correlations as JSON")
	beadHistory := flag.String("bead-history", "", "Show history for specific bead ID")
	historySince := flag.String("history-since", "", "Limit history to commits after this date/ref (e.g., '30 days ago', '2024-01-01')")
	historyLimit := flag.Int("history-limit", 500, "Max commits to analyze (0 = unlimited)")
	minConfidence := flag.Float64("min-confidence", 0.0, "Filter correlations by minimum confidence (0.0-1.0)")
	// Correlation audit flags (bv-e1u6)
	robotExplainCorrelation := flag.String("robot-explain-correlation", "", "Explain why a commit is linked to a bead (format: SHA:beadID)")
	robotConfirmCorrelation := flag.String("robot-confirm-correlation", "", "Confirm a correlation is correct (format: SHA:beadID)")
	robotRejectCorrelation := flag.String("robot-reject-correlation", "", "Reject an incorrect correlation (format: SHA:beadID)")
	correlationFeedbackBy := flag.String("correlation-by", "", "Agent/user identifier for correlation feedback")
	correlationFeedbackReason := flag.String("correlation-reason", "", "Reason for correlation feedback")
	robotCorrelationStats := flag.Bool("robot-correlation-stats", false, "Output correlation feedback statistics as JSON")
	// Orphan commit detection flags (bv-jdop)
	robotOrphans := flag.Bool("robot-orphans", false, "Output orphan commit candidates (commits that should be linked but aren't) as JSON")
	orphansMinScore := flag.Int("orphans-min-score", 30, "Minimum suspicion score for orphan candidates (0-100)")
	// File-bead index flags (bv-hmib)
	robotFileBeads := flag.String("robot-file-beads", "", "Output beads that touched a file path as JSON")
	fileBeadsLimit := flag.Int("file-beads-limit", 20, "Max closed beads to show (use with --robot-file-beads)")
	fileHotspots := flag.Bool("robot-file-hotspots", false, "Output files touched by most beads as JSON")
	hotspotsLimit := flag.Int("hotspots-limit", 10, "Max hotspots to show (use with --robot-file-hotspots)")
	// Impact analysis flag (bv-19pq)
	robotImpact := flag.String("robot-impact", "", "Analyze impact of modifying files (comma-separated paths)")
	// Co-change detection flag (bv-7a2f)
	robotFileRelations := flag.String("robot-file-relations", "", "Output files that frequently co-change with the given file path")
	relationsThreshold := flag.Float64("relations-threshold", 0.5, "Minimum correlation threshold (0.0-1.0) for related files")
	relationsLimit := flag.Int("relations-limit", 10, "Max related files to show")
	// Related work discovery flag (bv-jtdl)
	robotRelatedWork := flag.String("robot-related", "", "Output beads related to a specific bead ID as JSON")
	relatedMinRelevance := flag.Int("related-min-relevance", 20, "Minimum relevance score (0-100) for related work")
	relatedMaxResults := flag.Int("related-max-results", 10, "Max results per category for related work")
	relatedIncludeClosed := flag.Bool("related-include-closed", false, "Include closed beads in related work results")
	// Blocker chain analysis flag (bv-nlo0)
	robotBlockerChain := flag.String("robot-blocker-chain", "", "Output full blocker chain analysis for issue ID as JSON")
	// Impact network graph flag (bv-48kr)
	robotImpactNetwork := flag.String("robot-impact-network", "", "Output bead impact network as JSON (empty for full, or bead ID for subnetwork)")
	networkDepth := flag.Int("network-depth", 2, "Depth of subnetwork when querying specific bead (1-3)")
	// Temporal causality analysis flag (bv-j74w)
	robotCausality := flag.String("robot-causality", "", "Output causal chain analysis for bead ID as JSON")
	// Sprint flags (bv-156)
	robotSprintList := flag.Bool("robot-sprint-list", false, "Output sprints as JSON")
	robotSprintShow := flag.String("robot-sprint-show", "", "Output specific sprint details as JSON")
	// Forecast flags (bv-158)
	robotForecast := flag.String("robot-forecast", "", "Output ETA forecast for bead ID, or 'all' for all open issues")
	forecastLabel := flag.String("forecast-label", "", "Filter forecast by label")
	forecastSprint := flag.String("forecast-sprint", "", "Filter forecast by sprint ID")
	forecastAgents := flag.Int("forecast-agents", 1, "Number of parallel agents for capacity calculation")
	// Capacity simulation flags (bv-160)
	robotCapacity := flag.Bool("robot-capacity", false, "Output capacity simulation and completion projection as JSON")
	capacityAgents := flag.Int("agents", 1, "Number of parallel agents for capacity simulation")
	capacityLabel := flag.String("capacity-label", "", "Filter capacity simulation by label")
	// Burndown flags (bv-159)
	robotBurndown := flag.String("robot-burndown", "", "Output burndown data for sprint ID, or 'current' for active sprint")
	// Action script emission flags (bv-89)
	emitScript := flag.Bool("emit-script", false, "Emit shell script for top-N recommendations (agent workflows)")
	scriptLimit := flag.Int("script-limit", 5, "Limit number of items in emitted script (use with --emit-script)")
	scriptFormat := flag.String("script-format", "bash", "Script format: bash, fish, or zsh (use with --emit-script)")
	// Feedback loop flags (bv-90)
	feedbackAccept := flag.String("feedback-accept", "", "Record accept feedback for issue ID (tunes recommendation weights)")
	feedbackIgnore := flag.String("feedback-ignore", "", "Record ignore feedback for issue ID (tunes recommendation weights)")
	feedbackReset := flag.Bool("feedback-reset", false, "Reset all feedback data to defaults")
	feedbackShow := flag.Bool("feedback-show", false, "Show current feedback status and weight adjustments")
	// Priority brief export (bv-96)
	priorityBrief := flag.String("priority-brief", "", "Export priority brief to Markdown file (e.g., brief.md)")
	// Agent brief bundle (bv-131)
	agentBrief := flag.String("agent-brief", "", "Export agent brief bundle to directory (includes triage.json, insights.json, brief.md, helpers.md)")
	// Static pages export flags (bv-73f)
	exportPages := flag.String("export-pages", "", "Export static site to directory (e.g., ./bv-pages)")
	pagesTitle := flag.String("pages-title", "", "Custom title for static site")
	pagesIncludeClosed := flag.Bool("pages-include-closed", true, "Include closed issues in export (default: true)")
	pagesIncludeHistory := flag.Bool("pages-include-history", true, "Include git history for time-travel (default: true)")
	previewPages := flag.String("preview-pages", "", "Preview existing static site bundle")
	previewNoLiveReload := flag.Bool("no-live-reload", false, "Disable live-reload in preview mode")
	watchExport := flag.Bool("watch-export", false, "Watch for beads changes and auto-regenerate export (use with --export-pages)")
	pagesWizard := flag.Bool("pages", false, "Launch interactive Pages deployment wizard")
	// Debug rendering flag (for diagnosing TUI issues)
	debugRender := flag.String("debug-render", "", "Render a view and output to file (views: insights, board)")
	debugWidth := flag.Int("debug-width", 180, "Width for debug render")
	debugHeight := flag.Int("debug-height", 50, "Height for debug render")
	// Experimental background snapshot worker (bv-o11l)
	backgroundMode := flag.Bool("background-mode", false, "Enable experimental background snapshot loading (TUI only)")
	noBackgroundMode := flag.Bool("no-background-mode", false, "Disable experimental background snapshot loading (TUI only)")
	// Agent blurb management (bv-105)
	agentsAdd := flag.Bool("agents-add", false, "Add beads workflow instructions to AGENTS.md (creates file if needed)")
	agentsRemove := flag.Bool("agents-remove", false, "Remove beads workflow instructions from AGENTS.md")
	agentsUpdate := flag.Bool("agents-update", false, "Update beads workflow instructions to latest version")
	agentsCheck := flag.Bool("agents-check", false, "Check AGENTS.md blurb status (default if no --agents-* action)")
	agentsDryRun := flag.Bool("agents-dry-run", false, "Show what would happen without executing (use with --agents-*)")
	agentsForce := flag.Bool("agents-force", false, "Skip confirmation prompts (use with --agents-*)")
	// Override pflag's default usage so -h/--help prints our custom header.
	flag.Usage = func() {
		fmt.Println("Usage: bt [options]")
		fmt.Println("\nA TUI viewer for beads issue tracker.")
		flag.PrintDefaults()
	}
	flag.Parse()

	// CPU profiling support
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Could not start CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	envRobot := os.Getenv("BT_ROBOT") == "1"
	stdoutIsTTY := term.IsTerminal(int(os.Stdout.Fd()))

	robotMode := envRobot ||
		*robotHelp ||
		*robotInsights ||
		*robotPlan ||
		*robotPriority ||
		*robotTriage ||
		*robotTriageByTrack ||
		*robotTriageByLabel ||
		*robotNext ||
		*robotDiff ||
		*robotRecipes ||
		*robotLabelHealth ||
		*robotLabelFlow ||
		*robotLabelAttention ||
		*robotAlerts ||
		*robotMetrics ||
		*robotSchema ||
		*robotSuggest ||
		*robotGraph ||
		*robotSearch ||
		*robotDriftCheck ||
		*robotHistory ||
		*robotFileBeads != "" ||
		*fileHotspots ||
		*robotImpact != "" ||
		*robotFileRelations != "" ||
		*robotRelatedWork != "" ||
		*robotBlockerChain != "" ||
		*robotImpactNetwork != "" ||
		*robotCausality != "" ||
		*robotSprintList ||
		*robotSprintShow != "" ||
		*robotForecast != "" ||
		*robotBurndown != "" ||
		*robotByLabel != "" ||
		*robotByAssignee != "" ||
		*robotCapacity ||
		*robotDocs != "" ||
		// When stdout is non-TTY, --diff-since auto-enables JSON output. Mark this
		// as robot mode early so parsers keep stdout JSON clean.
		(*diffSince != "" && !stdoutIsTTY)

	// Mark robot mode for downstream packages (e.g., parsers) to keep stdout JSON clean.
	if robotMode && !envRobot {
		_ = os.Setenv("BT_ROBOT", "1")
		envRobot = true
	}

	// Structured output format for --robot-* commands.
	robotOutputFormat = resolveRobotOutputFormat(*outputFormat)
	robotToonEncodeOptions = resolveToonEncodeOptionsFromEnv()
	robotShowToonStats = *toonStats || strings.TrimSpace(os.Getenv("TOON_STATS")) == "1"
	if robotOutputFormat != "json" && robotOutputFormat != "toon" {
		fmt.Fprintf(os.Stderr, "Invalid --format %q (expected json|toon)\n", robotOutputFormat)
		os.Exit(2)
	}

	if *help {
		fmt.Println("Usage: bt [options]")
		fmt.Println("\nA TUI viewer for beads issue tracker.")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *robotHelp {
		printRobotHelp() // calls os.Exit(0)
	}


	if *versionFlag {
		fmt.Printf("bt %s\n", version.Version)
		os.Exit(0)
	}

	// Handle --check-update (bv-182)
	if *checkUpdateFlag {
		runCheckUpdate()
	}

	// Handle --update (bv-182)
	if *updateFlag {
		runUpdate(*yesFlag)
	}

	// Handle --rollback (bv-182)
	if *rollbackFlag {
		runRollback()
	}

	// Handle --agents-* commands (bv-105)
	agentsAnyAction := *agentsAdd || *agentsRemove || *agentsUpdate || *agentsCheck
	agentsAnyFlag := agentsAnyAction || *agentsDryRun || *agentsForce
	if agentsAnyFlag {
		runAgentsCommand(*agentsAdd, *agentsRemove, *agentsUpdate, *agentsCheck, *agentsDryRun, *agentsForce, robotMode)
	}

	// Handle feedback commands (bv-90)
	if *feedbackAccept != "" || *feedbackIgnore != "" || *feedbackReset || *feedbackShow {
		runFeedback(*feedbackAccept, *feedbackIgnore, *feedbackReset, *feedbackShow)
	}

	// Load recipes (needed for both --robot-recipes and --recipe)
	recipeLoader, err := recipe.LoadDefault()
	if err != nil {
		if !envRobot {
			fmt.Fprintf(os.Stderr, "Warning: Error loading recipes: %v\n", err)
		}
		// Create empty loader to continue
		recipeLoader = recipe.NewLoader()
	}

	// Handle --robot-recipes (before loading issues)
	if *robotRecipes {
		summaries := recipeLoader.ListSummaries()
		// Sort by name for consistent output
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
			fmt.Fprintf(os.Stderr, "Error encoding recipes: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --robot-schema (bd-2kxo)
	if *robotSchema {
		schemas := generateRobotSchemas()

		// Filter to specific command if requested
		if *schemaCommand != "" {
			if schema, ok := schemas.Commands[*schemaCommand]; ok {
				singleOutput := map[string]interface{}{
					"schema_version": schemas.SchemaVersion,
					"generated_at":   schemas.GeneratedAt,
					"command":        *schemaCommand,
					"schema":         schema,
				}
				encoder := newRobotEncoder(os.Stdout)
				if err := encoder.Encode(singleOutput); err != nil {
					fmt.Fprintf(os.Stderr, "Error encoding schema: %v\n", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", *schemaCommand)
			fmt.Fprintln(os.Stderr, "Available commands:")
			for cmd := range schemas.Commands {
				fmt.Fprintf(os.Stderr, "  %s\n", cmd)
			}
			os.Exit(1)
		}

		encoder := newRobotEncoder(os.Stdout)
		if err := encoder.Encode(schemas); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding schemas: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Machine-readable robot docs (bd-2v50)
	if *robotDocs != "" {
		docs := generateRobotDocs(*robotDocs)
		encoder := newRobotEncoder(os.Stdout)
		if err := encoder.Encode(docs); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding robot-docs: %v\n", err)
			os.Exit(1)
		}
		if _, hasErr := docs["error"]; hasErr {
			os.Exit(2) // Invalid arguments per documented exit codes
		}
		os.Exit(0)
	}

	// Get project directory for baseline operations (moved up to allow info check without loading issues)
	projectDir, _ := os.Getwd()
	baselinePath := baseline.DefaultPath(projectDir)

	// Handle --baseline-info
	if *baselineInfo {
		if !baseline.Exists(baselinePath) {
			fmt.Println("No baseline found.")
			fmt.Println("Create one with: bt --save-baseline \"description\"")
			os.Exit(0)
		}
		bl, err := baseline.Load(baselinePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading baseline: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(bl.Summary())
		os.Exit(0)
	}

	// Validate recipe name if provided (before loading issues)
	var activeRecipe *recipe.Recipe
	if *recipeName != "" {
		activeRecipe = recipeLoader.Get(*recipeName)
		if activeRecipe == nil {
			fmt.Fprintf(os.Stderr, "Error: Unknown recipe '%s'\n\n", *recipeName)
			fmt.Fprintln(os.Stderr, "Available recipes:")
			for _, name := range recipeLoader.Names() {
				r := recipeLoader.Get(name)
				fmt.Fprintf(os.Stderr, "  %-15s %s\n", name, r.Description)
			}
			os.Exit(1)
		}
	}

	// Load issues from current directory or workspace (with timing for profile)
	loadStart := time.Now()
	var issues []model.Issue
	var beadsPath string
	var selectedSource *datasource.DataSource
	var serverState *doltctl.ServerState
	var workspaceInfo *workspace.LoadSummary
	var asOfResolved string // Resolved commit SHA when using --as-of (for robot output metadata)

	if *asOf != "" {
		// Time-travel mode: load historical issues from git
		// Note: --as-of takes precedence over --workspace (can't combine historical + multi-repo)
		if *workspaceConfig != "" {
			fmt.Fprintf(os.Stderr, "Warning: --workspace is ignored when --as-of is specified\n")
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}
		gitLoader := loader.NewGitLoader(cwd)
		issues, err = gitLoader.LoadAt(*asOf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading issues at %s: %v\n", *asOf, err)
			os.Exit(1)
		}
		// Resolve to commit SHA for metadata
		asOfResolved, _ = gitLoader.ResolveRevision(*asOf)
		// No live reload for historical view
		beadsPath = ""
		if !envRobot {
			if asOfResolved != "" {
				fmt.Fprintf(os.Stderr, "Loaded %d issues from %s (%s)\n", len(issues), *asOf, asOfResolved[:min(7, len(asOfResolved))])
			} else {
				fmt.Fprintf(os.Stderr, "Loaded %d issues from %s\n", len(issues), *asOf)
			}
		}
	} else if *workspaceConfig != "" {
		// Load from workspace configuration
		loadedIssues, results, err := workspace.LoadAllFromConfig(context.Background(), *workspaceConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading workspace: %v\n", err)
			os.Exit(1)
		}
		issues = loadedIssues
		summary := workspace.Summarize(results)
		workspaceInfo = &summary

		// Print workspace loading summary
		if summary.FailedRepos > 0 {
			if !envRobot {
				fmt.Fprintf(os.Stderr, "Warning: %d repos failed to load\n", summary.FailedRepos)
				for _, name := range summary.FailedRepoNames {
					fmt.Fprintf(os.Stderr, "  - %s\n", name)
				}
			}
		}
		// No live reload for workspace mode (multiple files)
		beadsPath = ""

		// Automatically ensure .bt/ is in .gitignore at workspace root
		// Workspace config is typically at .bt/workspace.yaml, so project root is two levels up
		workspaceRoot := filepath.Dir(filepath.Dir(*workspaceConfig))
		_ = loader.EnsureBTInGitignore(workspaceRoot)
	} else {
		// Load from single repo (original behavior)
		result, err := datasource.LoadIssuesWithSource("")
		if errors.Is(err, datasource.ErrDoltRequired) {
			// Dolt server not running - try to start it (bt-07jp)
			beadsDir, bdErr := loader.GetBeadsDir("")
			if bdErr != nil {
				fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", bdErr)
				os.Exit(1)
			}
			ss, startErr := doltctl.EnsureServer(beadsDir, exec.LookPath)
			if startErr != nil {
				fmt.Fprintf(os.Stderr, "Failed to start Dolt server: %v\n", startErr)
				os.Exit(1)
			}
			serverState = ss
			// Retry data load now that server is running
			result, err = datasource.LoadIssuesWithSource("")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Dolt connected but failed to load issues: %v\n", err)
				serverState.StopIfOwned()
				os.Exit(1)
			}
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading beads: %v\n", err)
			fmt.Fprintln(os.Stderr, "Make sure you are in a project initialized with 'bd init'.")
			os.Exit(1)
		}
		issues = result.Issues
		selectedSource = &result.Source

		// beadsPath only for file-based sources (JSONL/SQLite) - Dolt uses poll-based refresh
		switch result.Source.Type {
		case datasource.SourceTypeJSONLLocal, datasource.SourceTypeJSONLWorktree, datasource.SourceTypeSQLite:
			beadsPath = result.Source.Path
		}

		// Automatically ensure .bt/ is in .gitignore to prevent polluting git
		// with search indexes, baselines, and other bt-specific files.
		// This is done silently and only in single-repo mode.
		beadsDir, _ := loader.GetBeadsDir("")
		projectDir := filepath.Dir(beadsDir)
		_ = loader.EnsureBTInGitignore(projectDir)
	}
	loadDuration := time.Since(loadStart)

	// Apply --repo filter if specified
	if *repoFilter != "" {
		issues = filterByRepo(issues, *repoFilter)
	}

	// Apply --bql filter if specified
	if *robotBQL && *bqlQuery == "" {
		fmt.Fprintln(os.Stderr, "Error: --robot-bql requires --bql \"query\"")
		os.Exit(1)
	}
	if *bqlQuery != "" {
		parsed, err := bql.Parse(*bqlQuery)
		if err != nil {
			fmt.Fprintf(os.Stderr, "BQL parse error: %v\n", err)
			os.Exit(1)
		}
		if err := bql.Validate(parsed); err != nil {
			fmt.Fprintf(os.Stderr, "BQL validation error: %v\n", err)
			os.Exit(1)
		}
		issueMap := make(map[string]*model.Issue, len(issues))
		for i := range issues {
			issueMap[issues[i].ID] = &issues[i]
		}
		executor := bql.NewMemoryExecutor()
		issues = executor.Execute(parsed, issues, bql.ExecuteOpts{IssueMap: issueMap})

		if *robotBQL {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(issues); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding robot-bql: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	issuesForSearch := issues

	// Stable data hash for robot outputs (after repo filter but before recipes/TUI)
	dataHash := analysis.ComputeDataHash(issues)

	// Label subgraph scoping (bv-122)
	// When --label is specified, extract the label's subgraph and use it for all robot analysis.
	// This includes label health context in the output.
	var labelScopeContext *analysis.LabelHealth
	if *labelScope != "" {
		sg := analysis.ComputeLabelSubgraph(issues, *labelScope)
		if sg.IssueCount == 0 {
			if !envRobot {
				fmt.Fprintf(os.Stderr, "Warning: No issues found with label %q\n", *labelScope)
			}
		} else {
			// Replace issues with the subgraph issues
			subgraphIssues := make([]model.Issue, 0, len(sg.AllIssues))
			for _, id := range sg.AllIssues {
				if iss, ok := sg.IssueMap[id]; ok {
					subgraphIssues = append(subgraphIssues, iss)
				}
			}
			issues = subgraphIssues
			// Compute label health for context
			cfg := analysis.DefaultLabelHealthConfig()
			allHealth := analysis.ComputeAllLabelHealth(issues, cfg, time.Now().UTC(), nil)
			for i := range allHealth.Labels {
				if allHealth.Labels[i].Label == *labelScope {
					labelScopeContext = &allHealth.Labels[i]
					break
				}
			}
		}
	}

	// Apply recipe filtering early for robot modes (bv-93)
	// This ensures --recipe filters are applied before robot modes exit.
	// dataHash uses pre-filtered issues for stability.
	if activeRecipe != nil && (*robotTriage || *robotNext || *robotTriageByTrack || *robotTriageByLabel || *robotPriority || *robotInsights || *robotPlan) {
		issues = applyRecipeFilters(issues, activeRecipe)
		issues = applyRecipeSort(issues, activeRecipe)
	}

	// Handle semantic search CLI (bv-9gf.3)
	if *robotSearch && *semanticQuery == "" {
		fmt.Fprintln(os.Stderr, "Error: --robot-search requires --search \"query\"")
		os.Exit(1)
	}
	if *semanticQuery != "" {
		embedCfg := search.EmbeddingConfigFromEnv()
		searchCfg, err := search.SearchConfigFromEnv()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		searchCfg, err = applySearchConfigOverrides(searchCfg, *searchMode, *searchPreset, *searchWeights)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		embedder, err := search.NewEmbedderFromConfig(embedCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		projectDir, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		indexPath := search.DefaultIndexPath(projectDir, embedCfg)
		idx, loaded, err := search.LoadOrNewVectorIndex(indexPath, embedder.Dim())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		docs := search.DocumentsFromIssues(issuesForSearch)
		if !*robotSearch && !loaded {
			fmt.Fprintf(os.Stderr, "Building semantic index (%d issues)...\n", len(docs))
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		syncStats, err := search.SyncVectorIndex(ctx, idx, embedder, docs, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building semantic index: %v\n", err)
			os.Exit(1)
		}
		if !loaded || syncStats.Changed() {
			if err := idx.Save(indexPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving semantic index: %v\n", err)
				os.Exit(1)
			}
		}

		qvecs, err := embedder.Embed(ctx, []string{*semanticQuery})
		if err != nil || len(qvecs) != 1 {
			if err == nil {
				err = fmt.Errorf("embedder returned %d vectors for query", len(qvecs))
			}
			fmt.Fprintf(os.Stderr, "Error embedding query: %v\n", err)
			os.Exit(1)
		}

		limit := *searchLimit
		if limit <= 0 {
			limit = 10
		}
		fetchLimit := limit
		if searchCfg.Mode == search.SearchModeHybrid {
			fetchLimit = search.HybridCandidateLimit(limit, len(issuesForSearch), *semanticQuery)
		}
		results, err := idx.SearchTopK(qvecs[0], fetchLimit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error searching index: %v\n", err)
			os.Exit(1)
		}
		results = search.ApplyShortQueryLexicalBoost(results, *semanticQuery, docs)
		if isLikelyIssueID(*semanticQuery) {
			results = promoteExactSearchResult(*semanticQuery, results)
		}

		titleByID := make(map[string]string, len(issuesForSearch))
		for _, iss := range issuesForSearch {
			titleByID[iss.ID] = iss.Title
		}

		var hybridResults []search.HybridScore
		var resolvedPreset search.PresetName
		var resolvedWeights *search.Weights
		if searchCfg.Mode == search.SearchModeHybrid {
			weights, presetName, err := resolveSearchWeights(searchCfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			weights = weights.Normalize()
			weights = search.AdjustWeightsForQuery(weights, *semanticQuery)
			resolvedPreset = presetName
			resolvedWeights = &weights

			cache := search.NewMetricsCache(search.NewAnalyzerMetricsLoader(issuesForSearch))
			if err := cache.Refresh(); err != nil {
				fmt.Fprintf(os.Stderr, "Error computing hybrid metrics: %v\n", err)
				os.Exit(1)
			}

			scorer := search.NewHybridScorer(weights, cache)
			hybridResults, err = buildHybridScores(results, scorer)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error scoring hybrid results: %v\n", err)
				os.Exit(1)
			}
			if isLikelyIssueID(*semanticQuery) {
				hybridResults = promoteExactHybridResult(*semanticQuery, hybridResults)
			}
			if len(hybridResults) > limit {
				hybridResults = hybridResults[:limit]
			}
		}

		if *robotSearch {
			out := robotSearchOutput{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				DataHash:    dataHash,
				Query:       *semanticQuery,
				Provider:    embedCfg.Provider,
				Model:       embedCfg.Model,
				Dim:         embedder.Dim(),
				IndexPath:   indexPath,
				Index:       syncStats,
				Loaded:      loaded,
				Limit:       limit,
				Mode:        searchCfg.Mode,
			}
			if searchCfg.Mode == search.SearchModeHybrid {
				out.Preset = resolvedPreset
				out.Weights = resolvedWeights
			}
			out.Results = make([]robotSearchResult, 0, max(len(results), len(hybridResults)))
			if searchCfg.Mode == search.SearchModeHybrid {
				for _, r := range hybridResults {
					out.Results = append(out.Results, robotSearchResult{
						IssueID:         r.IssueID,
						Score:           r.FinalScore,
						TextScore:       r.TextScore,
						Title:           titleByID[r.IssueID],
						ComponentScores: r.ComponentScores,
					})
				}
				out.UsageHints = []string{
					"jq '.results[] | {id: .issue_id, score: .score, text: .text_score}' - Extract scores",
					"jq '.results[] | {id: .issue_id, components: .component_scores}' - Hybrid breakdown",
					"jq '.index' - Index update stats (added/updated/removed/embedded)",
				}
			} else {
				for _, r := range results {
					out.Results = append(out.Results, robotSearchResult{
						IssueID: r.IssueID,
						Score:   r.Score,
						Title:   titleByID[r.IssueID],
					})
				}
				out.UsageHints = []string{
					"jq '.results[] | {id: .issue_id, score: .score, title: .title}' - Extract results",
					"jq '.index' - Index update stats (added/updated/removed/embedded)",
				}
			}

			if err := writeRobotSearchOutput(os.Stdout, out); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding robot-search: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		// Human-readable output
		if !loaded || syncStats.Changed() {
			fmt.Fprintf(os.Stderr, "Index: +%d ~%d -%d (%d total) → %s\n", syncStats.Added, syncStats.Updated, syncStats.Removed, idx.Size(), indexPath)
		}
		if searchCfg.Mode == search.SearchModeHybrid {
			for _, r := range hybridResults {
				fmt.Printf("%.4f\t%s\t%s\n", r.FinalScore, r.IssueID, titleByID[r.IssueID])
			}
		} else {
			for _, r := range results {
				fmt.Printf("%.4f\t%s\t%s\n", r.Score, r.IssueID, titleByID[r.IssueID])
			}
		}
		os.Exit(0)
	}

	// Handle --pages wizard (bv-10g)
	if *pagesWizard {
		if err := runPagesWizard(beadsPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --preview-pages (before export since it doesn't need analysis)
	if *previewPages != "" {
		if err := runPreviewServer(*previewPages, !*previewNoLiveReload); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting preview server: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --export-pages (bv-73f) with optional --watch-export (bv-55)
	if *exportPages != "" {
		// Define export function for reuse in watch mode
		exportCount := 0
		doExport := func(allIssues []model.Issue) error {
			exportCount++
			if exportCount > 1 {
				fmt.Printf("\n[%s] Re-exporting (change #%d)...\n", time.Now().Format("15:04:05"), exportCount-1)
			} else {
				fmt.Println("Exporting static site...")
			}
			fmt.Printf("  → Loading %d issues\n", len(allIssues))

			// Filter closed issues if not requested
			exportIssues := allIssues
			if !*pagesIncludeClosed {
				var openIssues []model.Issue
				for _, issue := range allIssues {
					if issue.Status != model.StatusClosed {
						openIssues = append(openIssues, issue)
					}
				}
				exportIssues = openIssues
				fmt.Printf("  → Filtering to %d open issues\n", len(exportIssues))
			}

			// Load and run pre-export hooks (bv-qjc.3)
			cwd, _ := os.Getwd()
			var pagesExecutor *hooks.Executor
			if !*noHooks {
				hookLoader := hooks.NewLoader(hooks.WithProjectDir(cwd))
				if err := hookLoader.Load(); err != nil {
					fmt.Printf("  → Warning: failed to load hooks: %v\n", err)
				} else if hookLoader.HasHooks() {
					fmt.Println("  → Running pre-export hooks...")
					ctx := hooks.ExportContext{
						ExportPath:   *exportPages,
						ExportFormat: "html",
						IssueCount:   len(exportIssues),
						Timestamp:    time.Now(),
					}
					pagesExecutor = hooks.NewExecutor(hookLoader.Config(), ctx)
					pagesExecutor.SetLogger(func(msg string) {
						fmt.Printf("  → %s\n", msg)
					})

					if err := pagesExecutor.RunPreExport(); err != nil {
						return fmt.Errorf("pre-export hook failed: %w", err)
					}
				}
			}

			// Build graph and compute stats
			fmt.Println("  → Running graph analysis...")
			analyzer := analysis.NewAnalyzer(exportIssues)
			stats := analyzer.AnalyzeAsync(context.Background())
			stats.WaitForPhase2()

			// Compute triage
			fmt.Println("  → Generating triage data...")
			triage := analysis.ComputeTriage(exportIssues)

			// Extract dependencies
			var deps []*model.Dependency
			for i := range exportIssues {
				issue := &exportIssues[i]
				for _, dep := range issue.Dependencies {
					if dep == nil || !dep.Type.IsBlocking() {
						continue
					}
					deps = append(deps, &model.Dependency{
						IssueID:     issue.ID,
						DependsOnID: dep.DependsOnID,
						Type:        dep.Type,
					})
				}
			}

			// Create exporter
			issuePointers := make([]*model.Issue, len(exportIssues))
			for i := range exportIssues {
				issuePointers[i] = &exportIssues[i]
			}
			exporter := export.NewSQLiteExporter(issuePointers, deps, stats, &triage)
			if *pagesTitle != "" {
				exporter.Config.Title = *pagesTitle
			}

			// Export SQLite database
			fmt.Println("  → Writing database and JSON files...")
			if err := exporter.Export(*exportPages); err != nil {
				return fmt.Errorf("exporting: %w", err)
			}

			// Copy viewer assets
			fmt.Println("  → Copying viewer assets...")
			if err := copyViewerAssets(*exportPages, *pagesTitle); err != nil {
				return fmt.Errorf("copying assets: %w", err)
			}

			// Generate README.md with project stats (useful for GitHub Pages deployment)
			fmt.Println("  → Generating README.md...")
			if err := generateREADME(*exportPages, *pagesTitle, "", exportIssues, &triage, stats); err != nil {
				fmt.Printf("  → Warning: failed to generate README: %v\n", err)
			}

			// Export history data for time-travel feature (bv-z38b)
			if *pagesIncludeHistory {
				fmt.Println("  → Generating time-travel history data...")
				if historyReport, err := generateHistoryForExport(allIssues); err == nil && historyReport != nil {
					historyPath := filepath.Join(*exportPages, "data", "history.json")
					if historyJSON, err := json.MarshalIndent(historyReport, "", "  "); err == nil {
						if err := os.WriteFile(historyPath, historyJSON, 0644); err != nil {
							fmt.Printf("  → Warning: failed to write history.json: %v\n", err)
						} else {
							fmt.Printf("  → history.json (%d commits)\n", len(historyReport.Commits))
						}
					}
				} else if err != nil {
					fmt.Printf("  → Warning: failed to generate history: %v\n", err)
				}
			}

			// Run post-export hooks (bv-qjc.3)
			if pagesExecutor != nil {
				fmt.Println("  → Running post-export hooks...")
				if err := pagesExecutor.RunPostExport(); err != nil {
					fmt.Printf("  → Warning: post-export hook failed: %v\n", err)
				}

				if len(pagesExecutor.Results()) > 0 {
					fmt.Println("")
					fmt.Println(pagesExecutor.Summary())
				}
			}

			fmt.Printf("✓ Export complete [%s]\n", time.Now().Format("15:04:05"))
			return nil
		}

		// Initial export
		if err := doExport(issues); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Watch mode (bv-55): monitor .beads/ for changes and auto-regenerate
		if *watchExport {
			fmt.Println("")
			fmt.Println("Watch mode enabled. Monitoring for changes...")

			// Collect all issues.jsonl files to watch
			var watchFiles []string
			var watchers []*watcher.Watcher

			if *workspaceConfig != "" {
				// Workspace mode: watch all repos' issues.jsonl files (bv-79)
				wsConfig, err := workspace.LoadConfig(*workspaceConfig)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error loading workspace config: %v\n", err)
					os.Exit(1)
				}
				workspaceRoot := filepath.Dir(filepath.Dir(*workspaceConfig))

				for _, repo := range wsConfig.Repos {
					if !repo.IsEnabled() {
						continue
					}
					repoPath := repo.Path
					if !filepath.IsAbs(repoPath) {
						repoPath = filepath.Join(workspaceRoot, repoPath)
					}
					beadsDir := filepath.Join(repoPath, repo.GetBeadsPath())
					issuesFile, err := loader.FindJSONLPath(beadsDir)
					if err != nil {
						fmt.Printf("  → Warning: could not find issues.jsonl for repo %s: %v\n", repo.GetName(), err)
						continue
					}
					watchFiles = append(watchFiles, issuesFile)
				}

				if len(watchFiles) == 0 {
					fmt.Fprintf(os.Stderr, "Error: no valid issues.jsonl files found in workspace\n")
					os.Exit(1)
				}
			} else {
				// Single-repo mode: watch current directory's issues.jsonl
				cwd, _ := os.Getwd()
				issuesFile := filepath.Join(cwd, ".beads", "issues.jsonl")
				watchFiles = append(watchFiles, issuesFile)
			}

			// Print watched files
			for _, f := range watchFiles {
				fmt.Printf("  → Watching: %s\n", f)
			}
			fmt.Println("  → Press Ctrl+C to stop")
			fmt.Println("")
			fmt.Println("To preview with auto-refresh, run in another terminal:")
			fmt.Printf("  bt --preview-pages %s\n", *exportPages)

			// Create a merged change channel for all watchers
			mergedChangeCh := make(chan struct{}, 1)

			// Create file watchers with 500ms debounce for each file
			for _, watchFile := range watchFiles {
				w, err := watcher.NewWatcher(watchFile,
					watcher.WithDebounceDuration(500*time.Millisecond),
					watcher.WithOnError(func(err error) {
						fmt.Printf("  → Watch error: %v\n", err)
					}),
				)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error creating watcher for %s: %v\n", watchFile, err)
					os.Exit(1)
				}

				if err := w.Start(); err != nil {
					fmt.Fprintf(os.Stderr, "Error starting watcher for %s: %v\n", watchFile, err)
					os.Exit(1)
				}
				watchers = append(watchers, w)

				// Forward changes to merged channel
				go func(ch <-chan struct{}) {
					for range ch {
						select {
						case mergedChangeCh <- struct{}{}:
						default:
							// Already a change pending, skip
						}
					}
				}(w.Changed())
			}

			// Cleanup all watchers on exit
			defer func() {
				for _, w := range watchers {
					w.Stop()
				}
			}()

			// Set up signal handling for graceful shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigCh)

			// Watch loop
			for {
				select {
				case <-mergedChangeCh:
					// Reload issues from disk using appropriate method
					var freshIssues []model.Issue
					var err error
					if *workspaceConfig != "" {
						freshIssues, _, err = workspace.LoadAllFromConfig(context.Background(), *workspaceConfig)
					} else {
						freshIssues, err = datasource.LoadIssues("")
					}
					if err != nil {
						fmt.Printf("  → Error reloading issues: %v\n", err)
						continue
					}
					if err := doExport(freshIssues); err != nil {
						fmt.Printf("  → Export error: %v\n", err)
					}
				case <-sigCh:
					fmt.Println("\nStopping watch mode...")
					os.Exit(0)
				}
			}
		}

		fmt.Println("")
		fmt.Printf("✓ Static site exported to: %s\n", *exportPages)
		fmt.Println("")
		fmt.Println("To preview locally:")
		fmt.Printf("  bt --preview-pages %s\n", *exportPages)
		fmt.Println("")
		fmt.Println("Or open in browser:")
		fmt.Printf("  open %s/index.html\n", *exportPages)
		os.Exit(0)
	}

	// Construct robotCtx for dispatching extracted handlers.
	// This is created once and reused by all robot command methods below.
	rc := newRobotCtx(issues, issuesForSearch, dataHash, projectDir, beadsPath, projectDir, labelScopeContext)

	// Handle --robot-label-health
	if *robotLabelHealth {
		rc.runLabelHealth()
	}

	// Handle --robot-label-flow (can be used stand-alone to avoid full health computation)
	if *robotLabelFlow {
		rc.runLabelFlow()
	}

	// Handle --robot-label-attention (bv-121)
	if *robotLabelAttention {
		rc.runLabelAttention(*attentionLimit)
	}

	// Handle --robot-graph (bv-136)
	if *robotGraph {
		rc.runRobotGraph(*graphFormat, *labelScope, *graphRoot, *graphDepth)
	}

	// Handle --export-graph (bv-94) - PNG/SVG/HTML export
	if *exportGraph != "" {
		rc.runExportGraph(*exportGraph, *labelScope, *graphTitle, *graphPreset)
	}

	// Handle --robot-alerts (drift + proactive)
	if *robotAlerts {
		rc.runAlerts(*alertSeverity, *alertType, *alertLabel)
	}

	// Handle --robot-suggest (bv-180)
	if *robotSuggest {
		rc.runSuggest(*suggestType, *suggestConfidence, *suggestBead)
	}

	// Handle --profile-startup
	if *profileStartup {
		runProfileStartup(issues, loadDuration, *profileJSON, *forceFullAnalysis)
		os.Exit(0)
	}

	// Handle --save-baseline
	if *saveBaseline != "" {
		rc.runSaveBaseline(*saveBaseline, *forceFullAnalysis)
	}

	// Handle --check-drift
	if *checkDrift {
		rc.runCheckDrift(*robotDriftCheck, *forceFullAnalysis)
	}

	if *robotInsights {
		rc.runInsights(*forceFullAnalysis, *asOf, asOfResolved, *labelScope)
	}

	if *robotPlan {
		rc.runPlan(*forceFullAnalysis, *asOf, asOfResolved, *labelScope)
	}

	if *robotPriority {
		rc.runPriority(*forceFullAnalysis, *asOf, asOfResolved, *labelScope, *robotMinConf, *robotMaxResults, *robotByLabel, *robotByAssignee)
	}

	if *robotTriage || *robotNext || *robotTriageByTrack || *robotTriageByLabel {
		rc.runTriage(*robotNext, *robotTriageByTrack, *robotTriageByLabel, *historyLimit, *asOf, asOfResolved)
	}

	// Handle --priority-brief flag (bv-96)
	if *priorityBrief != "" {
		rc.runPriorityBrief(*priorityBrief)
	}

	// Handle --agent-brief flag (bv-131)
	if *agentBrief != "" {
		rc.runAgentBrief(*agentBrief)
	}

	// Handle --emit-script flag (bv-89)
	if *emitScript {
		rc.runEmitScript(*scriptLimit, *scriptFormat)
	}

	// Handle --robot-history flag
	if *robotHistory || *beadHistory != "" {
		rc.runHistory(*beadHistory, *historySince, *historyLimit, *minConfidence)
	}

	// Handle correlation audit commands (bv-e1u6)
	if *robotExplainCorrelation != "" || *robotConfirmCorrelation != "" || *robotRejectCorrelation != "" || *robotCorrelationStats {
		rc.runCorrelationAudit(*robotExplainCorrelation, *robotConfirmCorrelation, *robotRejectCorrelation, *robotCorrelationStats, *correlationFeedbackBy, *correlationFeedbackReason)
	}

	// Handle --robot-orphans flag (bv-jdop)
	if *robotOrphans {
		rc.runOrphans(*historyLimit, *orphansMinScore)
	}

	// Handle --robot-file-beads and --robot-file-hotspots flags (bv-hmib)
	if *robotFileBeads != "" || *fileHotspots {
		rc.runFileBeads(*robotFileBeads, *fileHotspots, *fileBeadsLimit, *hotspotsLimit, *historyLimit)
	}

	// Handle --robot-impact flag (bv-19pq)
	if *robotImpact != "" {
		rc.runImpact(*robotImpact, *historyLimit)
	}

	// Handle --robot-file-relations flag (bv-7a2f)
	if *robotFileRelations != "" {
		rc.runFileRelations(*robotFileRelations, *relationsThreshold, *relationsLimit, *historyLimit)
	}

	// Handle --robot-related flag (bv-jtdl)
	if *robotRelatedWork != "" {
		rc.runRelatedWork(*robotRelatedWork, *relatedMinRelevance, *relatedMaxResults, *historyLimit, *relatedIncludeClosed)
	}

	// Handle --robot-blocker-chain flag (bv-nlo0)
	if *robotBlockerChain != "" {
		rc.runBlockerChain(*robotBlockerChain)
	}

	// Handle --robot-impact-network flag (bv-48kr)
	if *robotImpactNetwork != "" {
		rc.runImpactNetwork(*robotImpactNetwork, *networkDepth, *historyLimit)
	}

	// Handle --robot-causality flag (bv-j74w)
	if *robotCausality != "" {
		rc.runCausality(*robotCausality, *historyLimit)
	}

	// Handle --robot-sprint-list and --robot-sprint-show flags (bv-156)
	if *robotSprintList || *robotSprintShow != "" {
		rc.runSprintListOrShow(*robotSprintShow)
	}

	// Handle --robot-burndown flag (bv-159)
	if *robotBurndown != "" {
		rc.runBurndown(*robotBurndown)
	}

	// Handle --robot-forecast flag (bv-158)
	if *robotForecast != "" {
		rc.runForecast(*robotForecast, *forecastLabel, *forecastSprint, *forecastAgents)
	}

	// Handle --robot-capacity flag (bv-160)
	if *robotCapacity {
		rc.runCapacity(*capacityAgents, *capacityLabel)
	}

	// Handle --robot-metrics flag (bv-84tp)
	if *robotMetrics {
		rc.runMetrics()
	}

	// Handle --diff-since flag
	if *diffSince != "" {
		rc.runDiffSince(*diffSince, *robotDiff, *asOf, asOfResolved)
	}

	// Handle --as-of flag for TUI mode (robot commands already handled above with historical data)
	if *asOf != "" {
		if len(issues) == 0 {
			fmt.Printf("No issues found at %s.\n", *asOf)
			return
		}

		// Launch TUI with historical issues (already loaded, no live reload)
		m := ui.NewModel(issues, activeRecipe, "", nil)
		defer m.Stop()
		if err := runTUIProgram(m); err != nil {
			fmt.Printf("Error running beads viewer: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *exportFile != "" {
		fmt.Printf("Exporting to %s...\n", *exportFile)

		// Load and run pre-export hooks
		cwd, _ := os.Getwd()
		var executor *hooks.Executor
		if !*noHooks {
			hookLoader := hooks.NewLoader(hooks.WithProjectDir(cwd))
			if err := hookLoader.Load(); err != nil {
				fmt.Printf("Warning: failed to load hooks: %v\n", err)
			} else if hookLoader.HasHooks() {
				ctx := hooks.ExportContext{
					ExportPath:   *exportFile,
					ExportFormat: "markdown",
					IssueCount:   len(issues),
					Timestamp:    time.Now(),
				}
				executor = hooks.NewExecutor(hookLoader.Config(), ctx)

				// Run pre-export hooks
				if err := executor.RunPreExport(); err != nil {
					fmt.Printf("Error: pre-export hook failed: %v\n", err)
					os.Exit(1)
				}
			}
		}

		// Perform the export
		if err := export.SaveMarkdownToFile(issues, *exportFile); err != nil {
			fmt.Printf("Error exporting: %v\n", err)
			os.Exit(1)
		}

		// Run post-export hooks
		if executor != nil {
			if err := executor.RunPostExport(); err != nil {
				fmt.Printf("Warning: post-export hook failed: %v\n", err)
				// Don't exit, just warn
			}

			// Print hook summary if any hooks ran
			if len(executor.Results()) > 0 {
				fmt.Println(executor.Summary())
			}
		}

		fmt.Println("Done!")
		os.Exit(0)
	}

	if len(issues) == 0 {
		fmt.Println("No issues found. Create some with 'bd create'!")
		os.Exit(0)
	}

	// Apply recipe filters and sorting if specified
	if activeRecipe != nil {
		issues = applyRecipeFilters(issues, activeRecipe)
		issues = applyRecipeSort(issues, activeRecipe)
	}

	// Background mode rollout (bv-o11l):
	// - CLI flags override env var
	// - env var overrides user config file
	if *backgroundMode && *noBackgroundMode {
		fmt.Fprintln(os.Stderr, "Error: --background-mode and --no-background-mode are mutually exclusive")
		os.Exit(2)
	}
	if *backgroundMode {
		_ = os.Setenv("BT_BACKGROUND_MODE", "1")
	} else if *noBackgroundMode {
		_ = os.Setenv("BT_BACKGROUND_MODE", "0")
	} else if v, ok := os.LookupEnv("BT_BACKGROUND_MODE"); ok && strings.TrimSpace(v) != "" {
		// Respect explicit user env var.
	} else if enabled, ok := loadBackgroundModeFromUserConfig(); ok {
		if enabled {
			_ = os.Setenv("BT_BACKGROUND_MODE", "1")
		} else {
			_ = os.Setenv("BT_BACKGROUND_MODE", "0")
		}
	}

	// Initial Model with live reload support
	m := ui.NewModel(issues, activeRecipe, beadsPath, selectedSource)
	if serverState != nil {
		m.SetDoltServer(serverState, func(beadsDir string) error {
			newState, err := doltctl.EnsureServer(beadsDir, exec.LookPath)
			if err != nil {
				return err
			}
			serverState.UpdateAfterReconnect(newState)
			return nil
		})
	}
	defer m.Stop() // Clean up file watcher + Dolt server

	// Enable workspace mode if loading from workspace config
	if workspaceInfo != nil {
		m.EnableWorkspaceMode(ui.WorkspaceInfo{
			Enabled:      true,
			RepoCount:    workspaceInfo.TotalRepos,
			FailedCount:  workspaceInfo.FailedRepos,
			TotalIssues:  workspaceInfo.TotalIssues,
			RepoPrefixes: workspaceInfo.RepoPrefixes,
		})
	}

	// Debug render mode - output a view to file and exit
	if *debugRender != "" {
		output := m.RenderDebugView(*debugRender, *debugWidth, *debugHeight)
		fmt.Println(output)
		os.Exit(0)
	}

	// Run Program
	if err := runTUIProgram(m); err != nil {
		fmt.Printf("Error running beads viewer: %v\n", err)
		os.Exit(1)
	}
}

func runTUIProgram(m ui.Model) error {
	// Suppress log.Printf output while TUI is running (bv-9x36).
	// The background worker's logEvent() writes JSON to stderr via log.Printf,
	// which corrupts the TUI display. Debug logging uses BT_WORKER_TRACE_FILE instead.
	log.SetOutput(io.Discard)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithoutSignalHandler(),
	)

	runDone := make(chan struct{})
	defer close(runDone)

	// Graceful shutdown on SIGINT/SIGTERM (bv-bzt8).
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-runDone:
			return
		case <-sigCh:
		}

		p.Quit()

		select {
		case <-runDone:
			return
		case <-sigCh:
		case <-time.After(5 * time.Second):
		}

		p.Kill()
	}()

	// Optional auto-quit for automated tests: set BT_TUI_AUTOCLOSE_MS.
	if v := os.Getenv("BT_TUI_AUTOCLOSE_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			go func() {
				timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
				defer timer.Stop()

				select {
				case <-runDone:
					return
				case <-timer.C:
				}

				p.Quit()

				select {
				case <-runDone:
					return
				case <-time.After(2 * time.Second):
				}

				p.Kill()
			}()
		}
	}

	finalModel, err := p.Run()
	// Print Dolt shutdown message after TUI exits (bt-llek)
	if fm, ok := finalModel.(ui.Model); ok {
		if msg := fm.DoltShutdownMsg(); msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
	}
	if err != nil && errors.Is(err, tea.ErrProgramKilled) {
		if err == tea.ErrProgramKilled || errors.Is(err, tea.ErrInterrupted) {
			return nil
		}
	}
	return err
}

func loadBackgroundModeFromUserConfig() (bool, bool) {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return false, false
	}
	configPath := filepath.Join(homeDir, ".config", "bt", "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, false
	}

	var cfg struct {
		Experimental struct {
			BackgroundMode *bool `yaml:"background_mode"`
		} `yaml:"experimental"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return false, false
	}
	if cfg.Experimental.BackgroundMode == nil {
		return false, false
	}
	return *cfg.Experimental.BackgroundMode, true
}
