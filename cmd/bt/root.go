package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"

	json "github.com/goccy/go-json"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/internal/doltctl"
	"github.com/seanmartinsmith/beadstui/internal/settings"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/bql"
	"github.com/seanmartinsmith/beadstui/pkg/export"
	"github.com/seanmartinsmith/beadstui/pkg/hooks"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/recipe"
	"github.com/seanmartinsmith/beadstui/pkg/search"
	"github.com/seanmartinsmith/beadstui/pkg/ui"
	"github.com/seanmartinsmith/beadstui/pkg/workspace"

	tea "charm.land/bubbletea/v2"
)

// Persistent flag values set on rootCmd.
var (
	flagFormat    string
	flagToonStats bool
	flagGlobal    bool
	flagRepo      string

	// TUI-specific flags on root command.
	flagCPUProfile        string
	flagRecipe            string
	flagBQL               string
	flagAsOf              string
	flagDiffSince         string
	flagWorkspace         string
	flagBackgroundMode    bool
	flagNoBackgroundMode  bool
	flagDebugRender       string
	flagDebugWidth        int
	flagDebugHeight       int
	flagProfileStartup    bool
	flagProfileJSON       bool
	flagForceFullAnalysis bool
	flagAllowHooks        bool

	// Search flags
	flagSearch        string
	flagSearchLimit   int
	flagSearchMode    string
	flagSearchPreset  string
	flagSearchWeights string

	// Export flags (direct on root for backward compat during transition)
	flagExportMD string
)

// rootCmd is the top-level cobra command. Running `bt` with no subcommand launches the TUI.
var rootCmd = &cobra.Command{
	Use:   "bt",
	Short: "TUI viewer for beads issue tracker",
	Long:  "bt is a terminal UI for viewing and managing beads issues.",
	// Run (not RunE) ensures bare `bt` launches TUI, not help.
	Run: func(cmd *cobra.Command, args []string) {
		runRootTUI(cmd)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return rootPersistentPreRun(cmd)
	},
}

func init() {
	// Persistent flags available to all subcommands.
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagFormat, "format", "", "Structured output format: json or toon (env: BT_OUTPUT_FORMAT, TOON_DEFAULT_FORMAT)")
	pf.BoolVar(&flagToonStats, "stats", false, "Show JSON vs TOON token estimates on stderr (env: TOON_STATS=1)")
	pf.BoolVar(&flagGlobal, "global", false, "Show issues from all projects on shared Dolt server")
	pf.StringVar(&flagRepo, "repo", "", "Filter issues by repository prefix (e.g., 'api-' or 'api')")
	// --workspace affects issue loading (via loadIssues()), which every
	// subcommand exercises in its PreRun. It was root-only before the cobra
	// migration, which broke `bt --workspace <path> robot triage` flows.
	// Persistent here so the workspace loader is reachable from any subcommand.
	pf.StringVar(&flagWorkspace, "workspace", "", "Load issues from workspace config file (.bt/workspace.yaml)")

	// Root-only (TUI) flags.
	f := rootCmd.Flags()
	f.StringVar(&flagCPUProfile, "cpu-profile", "", "Write CPU profile to file")
	f.StringVarP(&flagRecipe, "recipe", "r", "", "Apply named recipe (e.g., triage, actionable, high-impact)")
	f.StringVar(&flagBQL, "bql", "", "BQL query to pre-filter issues (e.g., 'status:open priority<P2')")
	f.StringVar(&flagAsOf, "as-of", "", "View state at point in time (commit SHA, branch, tag, or date)")
	f.StringVar(&flagDiffSince, "diff-since", "", "Show changes since historical point (commit SHA, branch, tag, or date)")
	f.BoolVar(&flagBackgroundMode, "background-mode", false, "Enable experimental background snapshot loading (TUI only)")
	f.BoolVar(&flagNoBackgroundMode, "no-background-mode", false, "Disable experimental background snapshot loading (TUI only)")
	f.StringVar(&flagDebugRender, "debug-render", "", "Render a view and output to file (views: insights, board)")
	f.IntVar(&flagDebugWidth, "debug-width", 180, "Width for debug render")
	f.IntVar(&flagDebugHeight, "debug-height", 50, "Height for debug render")
	f.BoolVar(&flagProfileStartup, "profile-startup", false, "Output detailed startup timing profile for diagnostics")
	f.BoolVar(&flagProfileJSON, "profile-json", false, "Output profile in JSON format (use with --profile-startup)")
	f.BoolVar(&flagForceFullAnalysis, "force-full-analysis", false, "Compute all metrics regardless of graph size (may be slow for large graphs)")
	f.BoolVar(&flagAllowHooks, "allow-hooks", false, "Bypass trust check on .bt/hooks.yaml hooks (use only for trusted CI environments)")
	f.StringVar(&flagExportMD, "export-md", "", "Export issues to a Markdown file (e.g., report.md)")

	// Search flags on root.
	f.StringVar(&flagSearch, "search", "", "Semantic search query (vector-based; builds/updates index on first run)")
	f.IntVar(&flagSearchLimit, "search-limit", 10, "Max results for --search/--robot-search")
	f.StringVar(&flagSearchMode, "search-mode", "", "Search ranking mode: text or hybrid (default: BT_SEARCH_MODE or text)")
	f.StringVar(&flagSearchPreset, "search-preset", "", "Hybrid preset name (default: BT_SEARCH_PRESET or default)")
	f.StringVar(&flagSearchWeights, "search-weights", "", "Hybrid weights JSON (overrides preset; keys: text,pagerank,status,impact,priority,recency)")
}

// rootPersistentPreRun handles setup that all commands need: output format resolution,
// robot mode detection, and log suppression.
func rootPersistentPreRun(cmd *cobra.Command) error {
	// Resolve output format early (needed by all robot subcommands).
	robotOutputFormat = resolveRobotOutputFormat(flagFormat)
	robotToonEncodeOptions = resolveToonEncodeOptionsFromEnv()
	robotShowToonStats = flagToonStats || strings.TrimSpace(os.Getenv("TOON_STATS")) == "1"
	if robotOutputFormat != "json" && robotOutputFormat != "toon" {
		return fmt.Errorf("invalid --format %q (expected json|toon)", robotOutputFormat)
	}

	// --global and --workspace are mutually exclusive.
	if flagGlobal && flagWorkspace != "" {
		return fmt.Errorf("--global and --workspace are mutually exclusive")
	}
	if flagGlobal && flagAsOf != "" {
		return fmt.Errorf("--global and --as-of are mutually exclusive")
	}

	return nil
}

// loadIssues loads issues into appCtx. Separated from PersistentPreRunE because
// some commands (version, update, agents check) don't need data.
func loadIssues() error {
	loadStart := time.Now()
	envRobot := os.Getenv("BT_ROBOT") == "1"

	if flagAsOf != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		gitLoader := loader.NewGitLoader(cwd)
		issues, err := gitLoader.LoadAt(flagAsOf)
		if err != nil {
			return fmt.Errorf("loading issues at %s: %w", flagAsOf, err)
		}
		asOfResolved, _ := gitLoader.ResolveRevision(flagAsOf)
		appCtx.issues = issues
		appCtx.asOfResolved = asOfResolved
		appCtx.beadsPath = ""
		if !envRobot {
			if asOfResolved != "" {
				fmt.Fprintf(os.Stderr, "Loaded %d issues from %s (%s)\n", len(issues), flagAsOf, asOfResolved[:min(7, len(asOfResolved))])
			} else {
				fmt.Fprintf(os.Stderr, "Loaded %d issues from %s\n", len(issues), flagAsOf)
			}
		}
	} else if flagWorkspace != "" {
		loadedIssues, results, err := workspace.LoadAllFromConfig(context.Background(), flagWorkspace)
		if err != nil {
			return fmt.Errorf("loading workspace: %w", err)
		}
		appCtx.issues = loadedIssues
		summary := workspace.Summarize(results)
		appCtx.workspaceInfo = &summary
		if summary.FailedRepos > 0 && !envRobot {
			fmt.Fprintf(os.Stderr, "Warning: %d repos failed to load\n", summary.FailedRepos)
			for _, name := range summary.FailedRepoNames {
				fmt.Fprintf(os.Stderr, "  - %s\n", name)
			}
		}
		appCtx.beadsPath = ""
		workspaceRoot := filepath.Dir(filepath.Dir(flagWorkspace))
		_ = loader.EnsureBTInGitignore(workspaceRoot)
	} else if flagGlobal {
		host, port, err := datasource.DiscoverSharedServer()
		if err != nil {
			return fmt.Errorf("global mode error: %w", err)
		}
		globalSource := datasource.NewGlobalDataSource(host, port)
		if flagRepo != "" {
			globalSource.RepoFilter = flagRepo
		}
		result, err := datasource.LoadFromSource(globalSource)
		if err != nil {
			return fmt.Errorf("global mode load error: %w", err)
		}
		appCtx.issues = result
		appCtx.selectedSource = &globalSource
		appCtx.beadsPath = ""
		appCtx.workspaceInfo = buildWorkspaceInfoFromIssues(result)
	} else if loaded, autoErr := autoGlobalWithColdBoot(envRobot); autoErr != nil {
		return autoErr
	} else if !loaded {
		// Auto-global did not apply (e.g. inside a project, BT_TEST_MODE,
		// or shared-server discover failed and we have a local project to
		// fall back to). Let the local-project loader below handle it.
	}

	if len(appCtx.issues) == 0 && appCtx.selectedSource == nil && flagAsOf == "" && flagWorkspace == "" && !flagGlobal {
		result, err := datasource.LoadIssuesWithSource("")
		if errors.Is(err, datasource.ErrDoltRequired) {
			beadsDir, bdErr := loader.GetBeadsDir("")
			if bdErr != nil {
				return fmt.Errorf("getting beads directory: %w", bdErr)
			}
			ss, startErr := doltctl.EnsureServer(beadsDir, exec.LookPath)
			if startErr != nil {
				return fmt.Errorf("failed to start Dolt server: %w", startErr)
			}
			appCtx.serverState = ss
			result, err = datasource.LoadIssuesWithSource("")
			if err != nil {
				appCtx.serverState.StopIfOwned()
				return fmt.Errorf("Dolt connected but failed to load issues: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("loading beads: %w (make sure you are in a project initialized with 'bd init')", err)
		}
		appCtx.issues = result.Issues
		appCtx.selectedSource = &result.Source
		switch result.Source.Type {
		case datasource.SourceTypeJSONLLocal, datasource.SourceTypeJSONLWorktree:
			appCtx.beadsPath = result.Source.Path
		}
		beadsDir, _ := loader.GetBeadsDir("")
		projectDir := filepath.Dir(beadsDir)
		_ = loader.EnsureBTInGitignore(projectDir)
	}

	appCtx.loadDuration = time.Since(loadStart).Seconds()

	// Apply --repo filter.
	if flagRepo != "" {
		appCtx.issues = filterByRepo(appCtx.issues, flagRepo)
	}

	// Snapshot for search before further mutations.
	appCtx.issuesForSearch = appCtx.issues

	return nil
}

// buildWorkspaceInfoFromIssues constructs workspace info from loaded issues for the UI.
func buildWorkspaceInfoFromIssues(issues []model.Issue) *workspace.LoadSummary {
	var repoPrefixes []string
	seen := make(map[string]bool)
	for _, issue := range issues {
		if issue.SourceRepo != "" && !seen[issue.SourceRepo] {
			repoPrefixes = append(repoPrefixes, issue.SourceRepo)
			seen[issue.SourceRepo] = true
		}
	}
	return &workspace.LoadSummary{
		TotalRepos:   len(repoPrefixes),
		TotalIssues:  len(issues),
		RepoPrefixes: repoPrefixes,
	}
}

// runRootTUI is the default action when no subcommand is given: launch the TUI.
func runRootTUI(cmd *cobra.Command) {
	// CPU profiling support.
	if flagCPUProfile != "" {
		f, err := os.Create(flagCPUProfile)
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

	// Detect if stdout is being piped (non-TTY).
	stdoutIsTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// --diff-since with non-TTY auto-enables robot mode for JSON output.
	if flagDiffSince != "" && !stdoutIsTTY {
		_ = os.Setenv("BT_ROBOT", "1")
	}

	// Load issues.
	if err := loadIssues(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Load recipes.
	recipeLoader, err := recipe.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error loading recipes: %v\n", err)
		recipeLoader = recipe.NewLoader()
	}

	// Validate recipe name if provided.
	var activeRecipe *recipe.Recipe
	if flagRecipe != "" {
		activeRecipe = recipeLoader.Get(flagRecipe)
		if activeRecipe == nil {
			fmt.Fprintf(os.Stderr, "Error: Unknown recipe '%s'\n\n", flagRecipe)
			fmt.Fprintln(os.Stderr, "Available recipes:")
			for _, name := range recipeLoader.Names() {
				r := recipeLoader.Get(name)
				fmt.Fprintf(os.Stderr, "  %-15s %s\n", name, r.Description)
			}
			os.Exit(1)
		}
	}

	// Apply --bql filter.
	if flagBQL != "" {
		parsed, err := bql.Parse(flagBQL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "BQL parse error: %v\n", err)
			os.Exit(1)
		}
		if err := bql.Validate(parsed); err != nil {
			fmt.Fprintf(os.Stderr, "BQL validation error: %v\n", err)
			os.Exit(1)
		}
		issueMap := make(map[string]*model.Issue, len(appCtx.issues))
		for i := range appCtx.issues {
			issueMap[appCtx.issues[i].ID] = &appCtx.issues[i]
		}
		executor := bql.NewMemoryExecutor()
		appCtx.issues = executor.Execute(parsed, appCtx.issues, bql.ExecuteOpts{IssueMap: issueMap})
	}

	// Handle --search.
	if flagSearch != "" {
		runSearchCommand(appCtx.issues, appCtx.issuesForSearch, false)
		return
	}

	// Handle --diff-since.
	if flagDiffSince != "" {
		projectDir, _ := os.Getwd()
		rc := newRobotCtx(appCtx.issues, appCtx.issuesForSearch, appCtx.dataHash, projectDir, appCtx.beadsPath, projectDir, nil)
		rc.runDiffSince(flagDiffSince, false, flagAsOf, appCtx.asOfResolved)
		return
	}

	// Handle --profile-startup.
	if flagProfileStartup {
		runProfileStartup(appCtx.issues, time.Duration(appCtx.loadDuration*float64(time.Second)), flagProfileJSON, flagForceFullAnalysis)
		os.Exit(0)
	}

	// Handle --export-md.
	if flagExportMD != "" {
		runExportMDCommand(appCtx.issues)
		return
	}

	// Handle --as-of for TUI mode.
	if flagAsOf != "" {
		if len(appCtx.issues) == 0 {
			fmt.Printf("No issues found at %s.\n", flagAsOf)
			return
		}
		m := ui.NewModel(appCtx.issues, activeRecipe, "", nil)
		defer m.Stop()
		if err := runTUIProgram(m); err != nil {
			fmt.Printf("Error running beads viewer: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(appCtx.issues) == 0 {
		fmt.Println("No issues found. Create some with 'bd create'!")
		os.Exit(0)
	}

	// Apply recipe filters and sorting.
	if activeRecipe != nil {
		appCtx.issues = applyRecipeFilters(appCtx.issues, activeRecipe)
		appCtx.issues = applyRecipeSort(appCtx.issues, activeRecipe)
	}

	// Background mode rollout.
	if flagBackgroundMode && flagNoBackgroundMode {
		fmt.Fprintln(os.Stderr, "Error: --background-mode and --no-background-mode are mutually exclusive")
		os.Exit(2)
	}
	if flagBackgroundMode {
		_ = os.Setenv("BT_BACKGROUND_MODE", "1")
	} else if flagNoBackgroundMode {
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

	// Build TUI model.
	m := ui.NewModel(appCtx.issues, activeRecipe, appCtx.beadsPath, appCtx.selectedSource)

	// Set project name for footer.
	if beadsDir, err := loader.GetBeadsDir(""); err == nil {
		m.SetProjectName(filepath.Base(filepath.Dir(beadsDir)))
	}

	if appCtx.serverState != nil {
		m.SetDoltServer(appCtx.serverState, func(beadsDir string) error {
			newState, err := doltctl.EnsureServer(beadsDir, exec.LookPath)
			if err != nil {
				return err
			}
			appCtx.serverState.UpdateAfterReconnect(newState)
			return nil
		})
	}

	// Global mode reconnect.
	if flagGlobal {
		m.SetProjectName("global")
		m.SetDoltServer(nil, func(_ string) error {
			host, port, err := datasource.DiscoverSharedServer()
			if err != nil {
				return err
			}
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 2*time.Second)
			if err != nil {
				return fmt.Errorf("shared Dolt server unreachable at %s:%d", host, port)
			}
			conn.Close()
			return nil
		})
	}

	defer m.Stop()

	if appCtx.workspaceInfo != nil {
		m.EnableWorkspaceMode(ui.WorkspaceInfo{
			Enabled:      true,
			RepoCount:    appCtx.workspaceInfo.TotalRepos,
			FailedCount:  appCtx.workspaceInfo.FailedRepos,
			TotalIssues:  appCtx.workspaceInfo.TotalIssues,
			RepoPrefixes: appCtx.workspaceInfo.RepoPrefixes,
		})
	}

	if appCtx.currentProjectDB != "" {
		m.SetCurrentProjectDB(appCtx.currentProjectDB)
		m.SetActiveRepos(map[string]bool{appCtx.currentProjectDB: true})
	}

	// Debug render mode.
	if flagDebugRender != "" {
		output := m.RenderDebugView(flagDebugRender, flagDebugWidth, flagDebugHeight)
		fmt.Println(output)
		os.Exit(0)
	}

	// Run TUI.
	if err := runTUIProgram(m); err != nil {
		fmt.Printf("Error running beads viewer: %v\n", err)
		os.Exit(1)
	}
}

// runExportMDCommand handles --export-md flag.
func runExportMDCommand(issues []model.Issue) {
	fmt.Printf("Exporting to %s...\n", flagExportMD)

	cwd, _ := os.Getwd()
	ctx := hooks.ExportContext{
		ExportPath:   flagExportMD,
		ExportFormat: "markdown",
		IssueCount:   len(issues),
		Timestamp:    time.Now(),
	}
	executor, err := hooks.RunHooks(cwd, ctx, flagAllowHooks)
	if err != nil {
		fmt.Printf("Warning: failed to load hooks: %v\n", err)
	}
	if executor != nil {
		if err := executor.RunPreExport(); err != nil {
			exitOnHookError(err, "pre-export")
		}
	}

	if err := export.SaveMarkdownToFile(issues, flagExportMD); err != nil {
		fmt.Printf("Error exporting: %v\n", err)
		os.Exit(1)
	}

	if executor != nil {
		if err := executor.RunPostExport(); err != nil {
			fmt.Printf("Warning: post-export hook failed: %v\n", err)
		}
		if len(executor.Results()) > 0 {
			fmt.Println(executor.Summary())
		}
	}

	fmt.Println("Done!")
	os.Exit(0)
}

// exitOnHookError prints a friendly remediation message for untrusted-hooks
// errors and exits with code 78 (config error). Other hook errors exit 1.
func exitOnHookError(err error, phase string) {
	var ute *hooks.UntrustedHooksError
	if errors.As(err, &ute) {
		fmt.Fprintln(os.Stderr, ute.Error())
		os.Exit(78)
	}
	fmt.Fprintf(os.Stderr, "Error: %s hook failed: %v\n", phase, err)
	os.Exit(1)
}

// runSearchCommand handles the --search flag (human-readable or robot mode).
func runSearchCommand(issues, issuesForSearch []model.Issue, robotSearch bool) {
	embedCfg := search.EmbeddingConfigFromEnv()
	searchCfg, err := search.SearchConfigFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	searchCfg, err = applySearchConfigOverrides(searchCfg, flagSearchMode, flagSearchPreset, flagSearchWeights)
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
	if !robotSearch && !loaded {
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

	qvecs, err := embedder.Embed(ctx, []string{flagSearch})
	if err != nil || len(qvecs) != 1 {
		if err == nil {
			err = fmt.Errorf("embedder returned %d vectors for query", len(qvecs))
		}
		fmt.Fprintf(os.Stderr, "Error embedding query: %v\n", err)
		os.Exit(1)
	}

	limit := flagSearchLimit
	if limit <= 0 {
		limit = 10
	}
	fetchLimit := limit
	if searchCfg.Mode == search.SearchModeHybrid {
		fetchLimit = search.HybridCandidateLimit(limit, len(issuesForSearch), flagSearch)
	}
	results, err := idx.SearchTopK(qvecs[0], fetchLimit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching index: %v\n", err)
		os.Exit(1)
	}
	results = search.ApplyShortQueryLexicalBoost(results, flagSearch, docs)
	if isLikelyIssueID(flagSearch) {
		results = promoteExactSearchResult(flagSearch, results)
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
		weights = search.AdjustWeightsForQuery(weights, flagSearch)
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
		if isLikelyIssueID(flagSearch) {
			hybridResults = promoteExactHybridResult(flagSearch, hybridResults)
		}
		if len(hybridResults) > limit {
			hybridResults = hybridResults[:limit]
		}
	}

	if robotSearch {
		dataHash := analysis.ComputeDataHash(issues)
		out := robotSearchOutput{
			RobotEnvelope: NewRobotEnvelope(dataHash),
			Query:         flagSearch,
			Provider:      embedCfg.Provider,
			Model:         embedCfg.Model,
			Dim:           embedder.Dim(),
			IndexPath:     indexPath,
			Index:         syncStats,
			Loaded:        loaded,
			Limit:         limit,
			Mode:          searchCfg.Mode,
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

		enc := newRobotEncoder(os.Stdout)
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding robot-search: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Human-readable output.
	if !loaded || syncStats.Changed() {
		fmt.Fprintf(os.Stderr, "Index: +%d ~%d -%d (%d total) -> %s\n", syncStats.Added, syncStats.Updated, syncStats.Removed, idx.Size(), indexPath)
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

func runTUIProgram(m ui.Model) error {
	// Suppress log output while TUI is running.
	log.SetOutput(io.Discard)

	p := tea.NewProgram(
		m,
		tea.WithoutSignalHandler(),
	)
	// Mouse input is enabled per-view via tea.View.MouseMode = tea.MouseModeCellMotion
	// (see pkg/ui/model_view.go). bt-d8d1 wires MouseClickMsg into Update.
	//
	// Mouse-mode teardown (bt-ll7): every reachable exit path here funnels
	// through tea.Program.shutdown() -> cursedRenderer.close(), which emits
	// the matching disable sequence (\e[?1002l\e[?1003l\e[?1006l). The paths:
	//   - normal quit (q / Esc-Y / ctrl+c key in raw mode -> tea.Quit)
	//   - SIGINT/SIGTERM via the signal handler below (Quit then Kill after 5s)
	//   - panics in Update/View/Cmd (bubbletea v2 has built-in defer recover)
	//   - BT_TUI_AUTOCLOSE_MS (Quit then Kill after 2s)
	// SIGKILL / taskkill /F bypass userland entirely and cannot be caught from
	// inside the process; users can recover with `bt reset-terminal`.

	runDone := make(chan struct{})
	defer close(runDone)

	// Graceful shutdown on SIGINT/SIGTERM.
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

	// Optional auto-quit for automated tests.
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

// autoGlobalWithColdBoot is the bt-mxz9 Phase 2 replacement for the old
// auto-global fallback chain. It discovers the shared Dolt server, and
// if discovery fails AND we're not inside a beads project, attempts a
// cold-boot via `bd -C $anchor dolt start` using the persisted anchor
// (or BT_ANCHOR_PROJECT). When discovery succeeds and we ARE inside a
// project, the cwd's project path is recorded as the new anchor
// (latest-cwd-wins) so the next boot from `~` works without setup.
//
// Return semantics:
//   - (true, nil): global mode loaded successfully, appCtx populated.
//   - (false, nil): cold-boot did not apply; the caller should fall
//     through to the local-project loader. This happens when we're
//     inside a project, when BT_TEST_MODE=1, or when discover-then-
//     load failed but we're inside a project that local mode can
//     handle.
//   - (false, err): hard error; no fallback should be attempted. This
//     is the "no anchor available from outside any project" case and
//     genuine `bd -C` failures.
func autoGlobalWithColdBoot(envRobot bool) (bool, error) {
	host, port, discoverErr := datasource.DiscoverSharedServer()
	if discoverErr == nil {
		if loadGlobalMode(host, port) {
			maybeUpdateAnchor()
			return true, nil
		}
		// Discover succeeded (live listener) but load failed. If we're
		// inside a project, falling back to local is the right call.
		// Otherwise the user has nowhere else to go — surface the
		// failure rather than emit a misleading "no JSONL file" error
		// from the local-project loader (bt-6am7).
		if currentProjectRoot() != "" {
			return false, nil
		}
		return false, fmt.Errorf("shared Dolt server reachable but loading global mode failed and no local project to fall back to")
	}

	// Discover failed.
	if os.Getenv("BT_TEST_MODE") == "1" {
		// e2e tests always run with BT_TEST_MODE=1; cold-boot must not
		// reach for the developer's real shared server here.
		return false, nil
	}
	if currentProjectRoot() != "" {
		// Inside a project — local-project mode is the right fallback
		// (cheaper, no anchor indirection, exactly what the user is
		// pointing at with their cwd).
		return false, nil
	}

	// Outside any project: anchor is the only path to global mode.
	g, settingsErr := settings.Load()
	if settingsErr != nil {
		return false, fmt.Errorf("loading bt settings: %w", settingsErr)
	}
	anchor := g.Anchor()
	if anchor == "" {
		return false, fmt.Errorf("shared Dolt server unreachable and no anchor project is set\n  reason: %v\n  fix: cd into any beads project once (so bt records it as the cold-boot anchor), or export BT_ANCHOR_PROJECT=<path>", discoverErr)
	}

	if !envRobot {
		fmt.Fprintf(os.Stderr, "Starting shared Dolt server via anchor %s...\n", anchor)
	}
	if startErr := startSharedServerViaAnchor(anchor); startErr != nil {
		if isAnchorInvalidError(startErr) && !settings.AnchorFromEnv() {
			g.AnchorProject = ""
			_ = g.Save()
			return false, fmt.Errorf("anchor at %s is no longer a beads project; cd into a project once to re-anchor, or set BT_ANCHOR_PROJECT manually", anchor)
		}
		return false, fmt.Errorf("starting shared Dolt server via bd -C %s: %w", anchor, startErr)
	}

	// Server started — wait for the port file to appear and the listener
	// to come up. Polling is short (50ms × 40 = 2s) because bd writes the
	// port file immediately after binding.
	host, port, discoverErr = waitForSharedServer()
	if discoverErr != nil {
		return false, fmt.Errorf("shared server started via anchor but did not become discoverable within 2s: %w", discoverErr)
	}
	if !loadGlobalMode(host, port) {
		return false, fmt.Errorf("shared server started via anchor but loading global mode failed")
	}
	// No anchor auto-write here: cwd is not inside a project, so there's
	// nothing to record. The anchor we used is already correct.
	return true, nil
}

// loadGlobalMode populates appCtx with global-mode results. Returns false
// on load failure so the caller can decide between hard-erroring and
// falling through to local-project mode.
func loadGlobalMode(host string, port int) bool {
	globalSource := datasource.NewGlobalDataSource(host, port)
	result, err := datasource.LoadFromSource(globalSource)
	if err != nil {
		return false
	}
	appCtx.issues = result
	appCtx.selectedSource = &globalSource
	appCtx.beadsPath = ""
	appCtx.workspaceInfo = buildWorkspaceInfoFromIssues(result)
	appCtx.currentProjectDB = detectCurrentProjectDB()
	return true
}

// maybeUpdateAnchor records the cwd's project root as the cold-boot anchor
// (bt-mxz9 Phase 2, latest-cwd-wins). Called on every successful auto-global
// boot from inside a project. Skipped when:
//   - BT_ANCHOR_PROJECT is set (env-supplied anchors are never auto-modified).
//   - cwd is not inside a beads project (nothing to anchor).
//   - the persisted anchor already matches the cwd (no-op write avoided).
//
// Save errors are non-fatal; boot proceeds even if the anchor file is
// unwritable, since the live load already succeeded.
func maybeUpdateAnchor() {
	if settings.AnchorFromEnv() {
		return
	}
	root := currentProjectRoot()
	if root == "" {
		return
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	g, err := settings.Load()
	if err != nil {
		return
	}
	if g.AnchorProject == abs {
		return
	}
	g.AnchorProject = abs
	_ = g.Save()
}

// currentProjectRoot returns the absolute path of the cwd's beads project,
// or "" if the cwd is not inside any project. Built on loader.GetBeadsDir
// so the discovery shape stays in sync with the rest of bt.
//
// Project-ness is gated on the presence of `.beads/metadata.json`. Just
// having a `.beads/` directory isn't enough — `~/.beads/` exists on every
// install for shared-server config and a stray dead `registry.json`, and
// counting it as a project anchored bt to `~` (which can never start a
// shared server). bd's own `hasBeadsProjectFiles()` excludes registry-only
// `.beads/` from workspace resolution for the same reason.
func currentProjectRoot() string {
	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(beadsDir, "metadata.json")); err != nil {
		return ""
	}
	return filepath.Dir(beadsDir)
}

// startSharedServerViaAnchor shells out `bd -C <anchor> dolt start`. The
// `-C` flag (upstream PR #3442, merged 2026-04-27) makes bd resolve its
// `.beads/` discovery relative to the anchor path instead of the caller's
// cwd, which is the upstream-blessed way for an external tool to drive bd
// from a non-workspace directory.
func startSharedServerViaAnchor(anchor string) error {
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		return fmt.Errorf("bd CLI not found on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bdPath, "-C", anchor, "dolt", "start")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\noutput: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// isAnchorInvalidError matches the error shape bd emits when `-C <path>`
// resolves to a non-beads directory. We're conservative here — only the
// canonical "no active beads workspace" message clears the anchor.
// Transient failures (bd missing, port collision, network) keep the
// persisted anchor so the next boot can retry.
func isAnchorInvalidError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no active beads workspace")
}

// waitForSharedServer polls DiscoverSharedServer until either the server
// is discoverable (live port file + live listener) or the timeout
// expires. Used after `bd -C dolt start` returns successfully — bd
// writes the port file immediately after binding, so 2s × 50ms is a
// generous upper bound on the race.
func waitForSharedServer() (string, int, error) {
	const interval = 50 * time.Millisecond
	const total = 2 * time.Second
	deadline := time.Now().Add(total)
	for {
		host, port, err := datasource.DiscoverSharedServer()
		if err == nil {
			return host, port, nil
		}
		if time.Now().After(deadline) {
			return "", 0, err
		}
		time.Sleep(interval)
	}
}

// detectCurrentProjectDB reads the dolt_database name from the cwd's .beads/metadata.json.
func detectCurrentProjectDB() string {
	beadsDir, err := loader.GetBeadsDir("")
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(beadsDir, "metadata.json"))
	if err != nil {
		return ""
	}
	var meta struct {
		DoltDatabase string `json:"dolt_database"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return ""
	}
	return meta.DoltDatabase
}
