package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	json "github.com/goccy/go-json"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/export"
	"github.com/seanmartinsmith/beadstui/pkg/hooks"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/watcher"
	"github.com/seanmartinsmith/beadstui/pkg/workspace"
)

// handleHookError converts hook-trust refusals into an exit-78 path while
// surfacing other hook errors as wrapped RunE errors. Cobra exits 1 on RunE
// errors by default, so we exit directly here for the trust case to land on
// the documented "config error" exit code.
func handleHookError(err error, phase string) error {
	var ute *hooks.UntrustedHooksError
	if errors.As(err, &ute) {
		fmt.Fprintln(os.Stderr, ute.Error())
		os.Exit(78)
	}
	return fmt.Errorf("%s hook failed: %w", phase, err)
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export issues in various formats",
}

// bt export md
var exportMDCmd = &cobra.Command{
	Use:   "md [output-file]",
	Short: "Export issues to a Markdown file",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			return err
		}
		outputFile := ""
		if len(args) > 0 {
			outputFile = args[0]
		}
		if outputFile == "" {
			outputFile = "report.md"
		}
		fmt.Printf("Exporting to %s...\n", outputFile)

		allowHooks, _ := cmd.Flags().GetBool("allow-hooks")
		cwd, _ := os.Getwd()
		ctx := hooks.ExportContext{
			ExportPath:   outputFile,
			ExportFormat: "markdown",
			IssueCount:   len(appCtx.issues),
			Timestamp:    time.Now(),
		}
		executor, err := hooks.RunHooks(cwd, ctx, allowHooks)
		if err != nil {
			fmt.Printf("Warning: failed to load hooks: %v\n", err)
		}
		if executor != nil {
			if err := executor.RunPreExport(); err != nil {
				return handleHookError(err, "pre-export")
			}
		}

		if err := export.SaveMarkdownToFile(appCtx.issues, outputFile); err != nil {
			return fmt.Errorf("exporting: %w", err)
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
		return nil
	},
}

// bt export pages
var exportPagesCmd = &cobra.Command{
	Use:   "pages [output-dir]",
	Short: "Export static site to directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			return err
		}
		outputDir := ""
		if len(args) > 0 {
			outputDir = args[0]
		}
		if outputDir == "" {
			return fmt.Errorf("output directory required")
		}

		allowHooks, _ := cmd.Flags().GetBool("allow-hooks")
		title, _ := cmd.Flags().GetString("title")
		includeClosed, _ := cmd.Flags().GetBool("include-closed")
		includeHistory, _ := cmd.Flags().GetBool("include-history")
		watchMode, _ := cmd.Flags().GetBool("watch")

		exportCount := 0
		doExport := func(allIssues []model.Issue) error {
			exportCount++
			if exportCount > 1 {
				fmt.Printf("\n[%s] Re-exporting (change #%d)...\n", time.Now().Format("15:04:05"), exportCount-1)
			} else {
				fmt.Println("Exporting static site...")
			}
			fmt.Printf("  -> Loading %d issues\n", len(allIssues))

			exportIssues := allIssues
			if !includeClosed {
				var openIssues []model.Issue
				for _, issue := range allIssues {
					if issue.Status != model.StatusClosed {
						openIssues = append(openIssues, issue)
					}
				}
				exportIssues = openIssues
				fmt.Printf("  -> Filtering to %d open issues\n", len(exportIssues))
			}

			cwd, _ := os.Getwd()
			ctx := hooks.ExportContext{
				ExportPath:   outputDir,
				ExportFormat: "html",
				IssueCount:   len(exportIssues),
				Timestamp:    time.Now(),
			}
			pagesExecutor, err := hooks.RunHooks(cwd, ctx, allowHooks)
			if err != nil {
				fmt.Printf("  -> Warning: failed to load hooks: %v\n", err)
			}
			if pagesExecutor != nil {
				fmt.Println("  -> Running pre-export hooks...")
				pagesExecutor.SetLogger(func(msg string) {
					fmt.Printf("  -> %s\n", msg)
				})
				if err := pagesExecutor.RunPreExport(); err != nil {
					return handleHookError(err, "pre-export")
				}
			}

			fmt.Println("  -> Running graph analysis...")
			analyzer := analysis.NewAnalyzer(exportIssues)
			stats := analyzer.AnalyzeAsync(context.Background())
			stats.WaitForPhase2()

			fmt.Println("  -> Generating triage data...")
			triage := analysis.ComputeTriage(exportIssues)

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

			issuePointers := make([]*model.Issue, len(exportIssues))
			for i := range exportIssues {
				issuePointers[i] = &exportIssues[i]
			}
			exporter := export.NewSQLiteExporter(issuePointers, deps, stats, &triage)
			if title != "" {
				exporter.Config.Title = title
			}

			fmt.Println("  -> Writing database and JSON files...")
			if err := exporter.Export(outputDir); err != nil {
				return fmt.Errorf("exporting: %w", err)
			}

			fmt.Println("  -> Copying viewer assets...")
			if err := copyViewerAssets(outputDir, title); err != nil {
				return fmt.Errorf("copying assets: %w", err)
			}

			fmt.Println("  -> Generating README.md...")
			if err := generateREADME(outputDir, title, "", exportIssues, &triage, stats); err != nil {
				fmt.Printf("  -> Warning: failed to generate README: %v\n", err)
			}

			if includeHistory {
				fmt.Println("  -> Generating time-travel history data...")
				if historyReport, err := generateHistoryForExport(allIssues); err == nil && historyReport != nil {
					historyPath := filepath.Join(outputDir, "data", "history.json")
					if historyJSON, err := json.MarshalIndent(historyReport, "", "  "); err == nil {
						if err := os.WriteFile(historyPath, historyJSON, 0644); err != nil {
							fmt.Printf("  -> Warning: failed to write history.json: %v\n", err)
						} else {
							fmt.Printf("  -> history.json (%d commits)\n", len(historyReport.Commits))
						}
					}
				} else if err != nil {
					fmt.Printf("  -> Warning: failed to generate history: %v\n", err)
				}
			}

			if pagesExecutor != nil {
				fmt.Println("  -> Running post-export hooks...")
				if err := pagesExecutor.RunPostExport(); err != nil {
					fmt.Printf("  -> Warning: post-export hook failed: %v\n", err)
				}
				if len(pagesExecutor.Results()) > 0 {
					fmt.Println("")
					fmt.Println(pagesExecutor.Summary())
				}
			}

			fmt.Printf("Done! [%s]\n", time.Now().Format("15:04:05"))
			return nil
		}

		if err := doExport(appCtx.issues); err != nil {
			return err
		}

		if watchMode {
			return runExportWatchLoop(outputDir, doExport)
		}

		fmt.Println("")
		fmt.Printf("Static site exported to: %s\n", outputDir)
		fmt.Println("")
		fmt.Println("To preview locally:")
		fmt.Printf("  bt export preview %s\n", outputDir)
		fmt.Println("")
		fmt.Println("Or open in browser:")
		fmt.Printf("  open %s/index.html\n", outputDir)
		return nil
	},
}

// bt export preview
var exportPreviewCmd = &cobra.Command{
	Use:   "preview [dir]",
	Short: "Preview existing static site bundle",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := ""
		if len(args) > 0 {
			dir = args[0]
		}
		if dir == "" {
			return fmt.Errorf("directory required")
		}
		noLiveReload, _ := cmd.Flags().GetBool("no-live-reload")
		return runPreviewServer(dir, !noLiveReload)
	},
}

// bt export graph
var exportGraphCmd = &cobra.Command{
	Use:   "graph [output-path]",
	Short: "Export graph: .html for interactive, .png/.svg for static",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := loadIssues(); err != nil {
			return err
		}
		outputPath := ""
		if len(args) > 0 {
			outputPath = args[0]
		}
		if outputPath == "" {
			outputPath = "graph.html"
		}
		dataHash := analysis.ComputeDataHash(appCtx.issues)
		projectDir, _ := os.Getwd()
		rc := newRobotCtx(appCtx.issues, appCtx.issuesForSearch, dataHash, projectDir, appCtx.beadsPath, projectDir, nil)
		graphTitle, _ := cmd.Flags().GetString("title")
		graphPreset, _ := cmd.Flags().GetString("preset")
		labelScope, _ := cmd.Flags().GetString("label")
		rc.runExportGraph(outputPath, labelScope, graphTitle, graphPreset)
		return nil
	},
}

func runExportWatchLoop(exportDir string, doExport func([]model.Issue) error) error {
	fmt.Println("")
	fmt.Println("Watch mode enabled. Monitoring for changes...")

	var watchFiles []string
	var watchers []*watcher.Watcher

	if flagWorkspace != "" {
		wsConfig, err := workspace.LoadConfig(flagWorkspace)
		if err != nil {
			return fmt.Errorf("loading workspace config: %w", err)
		}
		workspaceRoot := filepath.Dir(filepath.Dir(flagWorkspace))

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
				fmt.Printf("  -> Warning: could not find issues.jsonl for repo %s: %v\n", repo.GetName(), err)
				continue
			}
			watchFiles = append(watchFiles, issuesFile)
		}

		if len(watchFiles) == 0 {
			return fmt.Errorf("no valid issues.jsonl files found in workspace")
		}
	} else {
		cwd, _ := os.Getwd()
		issuesFile := filepath.Join(cwd, ".beads", "issues.jsonl")
		watchFiles = append(watchFiles, issuesFile)
	}

	for _, f := range watchFiles {
		fmt.Printf("  -> Watching: %s\n", f)
	}
	fmt.Println("  -> Press Ctrl+C to stop")
	fmt.Println("")
	fmt.Println("To preview with auto-refresh, run in another terminal:")
	fmt.Printf("  bt export preview %s\n", exportDir)

	mergedChangeCh := make(chan struct{}, 1)

	for _, watchFile := range watchFiles {
		w, err := watcher.NewWatcher(watchFile,
			watcher.WithDebounceDuration(500*time.Millisecond),
			watcher.WithOnError(func(err error) {
				fmt.Printf("  -> Watch error: %v\n", err)
			}),
		)
		if err != nil {
			return fmt.Errorf("creating watcher for %s: %w", watchFile, err)
		}
		if err := w.Start(); err != nil {
			return fmt.Errorf("starting watcher for %s: %w", watchFile, err)
		}
		watchers = append(watchers, w)
		go func(ch <-chan struct{}) {
			for range ch {
				select {
				case mergedChangeCh <- struct{}{}:
				default:
				}
			}
		}(w.Changed())
	}
	defer func() {
		for _, w := range watchers {
			w.Stop()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-mergedChangeCh:
			var freshIssues []model.Issue
			var err error
			if flagWorkspace != "" {
				freshIssues, _, err = workspace.LoadAllFromConfig(context.Background(), flagWorkspace)
			} else {
				freshIssues, err = datasource.LoadIssues("")
			}
			if err != nil {
				fmt.Printf("  -> Error reloading issues: %v\n", err)
				continue
			}
			if err := doExport(freshIssues); err != nil {
				fmt.Printf("  -> Export error: %v\n", err)
			}
		case <-sigCh:
			fmt.Println("\nStopping watch mode...")
			return nil
		}
	}
}

func init() {
	// export md
	exportMDCmd.Flags().Bool("allow-hooks", false, "Bypass trust check on .bt/hooks.yaml hooks (use only for trusted CI environments)")
	exportCmd.AddCommand(exportMDCmd)

	// export pages
	exportPagesCmd.Flags().String("title", "", "Custom title for static site")
	exportPagesCmd.Flags().Bool("include-closed", true, "Include closed issues (default: true)")
	exportPagesCmd.Flags().Bool("include-history", true, "Include git history for time-travel (default: true)")
	exportPagesCmd.Flags().Bool("watch", false, "Watch for beads changes and auto-regenerate export")
	exportPagesCmd.Flags().Bool("allow-hooks", false, "Bypass trust check on .bt/hooks.yaml hooks (use only for trusted CI environments)")
	exportCmd.AddCommand(exportPagesCmd)

	// export preview
	exportPreviewCmd.Flags().Bool("no-live-reload", false, "Disable live-reload in preview mode")
	exportCmd.AddCommand(exportPreviewCmd)

	// export graph
	exportGraphCmd.Flags().String("title", "", "Title for graph export")
	exportGraphCmd.Flags().String("preset", "compact", "Graph layout preset: compact or roomy")
	exportGraphCmd.Flags().String("label", "", "Scope to label subgraph")
	exportCmd.AddCommand(exportGraphCmd)

	rootCmd.AddCommand(exportCmd)
}
