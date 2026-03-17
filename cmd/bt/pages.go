package main

import (
	"context"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/export"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// runPreviewServer starts a local HTTP server to preview the static site.
func runPreviewServer(dir string, liveReload bool) error {
	cfg := export.DefaultPreviewConfig()
	cfg.BundlePath = dir
	cfg.LiveReload = liveReload
	return export.StartPreviewWithConfig(cfg)
}

// runPagesWizard runs the interactive deployment wizard (bv-10g).
func runPagesWizard(beadsPath string) error {
	wizard := export.NewWizard(beadsPath)

	// Run interactive wizard to collect configuration
	_, err := wizard.Run()
	if err != nil {
		return err
	}

	config := wizard.GetConfig()

	// Resolve the actual source of issues for this deployment.
	// This ensures updates always use the originally-deployed dataset,
	// even if the user runs bv from a different directory.
	source, err := resolvePagesSource(config, beadsPath)
	if err != nil {
		return err
	}
	issues := source.Issues

	// Filter issues based on config
	exportIssues := issues
	if !config.IncludeClosed {
		var openIssues []model.Issue
		for _, issue := range issues {
			if issue.Status != model.StatusClosed {
				openIssues = append(openIssues, issue)
			}
		}
		exportIssues = openIssues
	}

	// Create temp directory for bundle
	bundlePath := config.OutputPath
	if bundlePath == "" {
		tmpDir, err := os.MkdirTemp("", "bv-pages-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		bundlePath = tmpDir
	}

	// Ensure output directory exists
	if err := os.MkdirAll(bundlePath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Perform export
	wizard.PerformExport(bundlePath)

	if source.BeadsDir != "" {
		fmt.Printf("  -> Using beads source: %s (%s)\n", source.BeadsDir, source.Reason)
	}

	fmt.Println("Exporting static site...")
	fmt.Printf("  -> Loading %d issues\n", len(exportIssues))

	// Build graph and compute stats
	fmt.Println("  -> Running graph analysis...")
	analyzer := analysis.NewAnalyzer(exportIssues)
	stats := analyzer.AnalyzeAsync(context.Background())
	stats.WaitForPhase2()

	// Compute triage
	fmt.Println("  -> Generating triage data...")
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
	if config.Title != "" {
		exporter.Config.Title = config.Title
	}

	// Export SQLite database
	fmt.Println("  -> Writing database and JSON files...")
	if err := exporter.Export(bundlePath); err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Copy viewer assets
	fmt.Println("  -> Copying viewer assets...")
	if err := copyViewerAssets(bundlePath, config.Title); err != nil {
		return fmt.Errorf("failed to copy assets: %w", err)
	}

	// Generate README.md with project stats (for GitHub Pages)
	if config.DeployTarget == "github" {
		fmt.Println("  -> Generating README.md...")
		// Compute the GitHub Pages URL from username and repo name
		pagesURL := ""
		if ghStatus, err := export.CheckGHStatus(); err == nil && ghStatus.Authenticated && ghStatus.Username != "" {
			repoName := config.RepoName
			// Handle repo names that already include owner (e.g., "owner/repo")
			if strings.Contains(repoName, "/") {
				parts := strings.Split(repoName, "/")
				// Validate we have both owner and repo parts
				if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
					pagesURL = fmt.Sprintf("https://%s.github.io/%s/", parts[0], parts[1])
				}
			}
			// Fallback to username + repo name if no valid owner/repo format
			if pagesURL == "" && repoName != "" {
				// Strip any leading/trailing slashes from repo name
				cleanRepo := strings.Trim(repoName, "/")
				if cleanRepo != "" {
					pagesURL = fmt.Sprintf("https://%s.github.io/%s/", ghStatus.Username, cleanRepo)
				}
			}
		}
		if err := generateREADME(bundlePath, config.Title, pagesURL, exportIssues, &triage, stats); err != nil {
			fmt.Printf("  -> Warning: failed to generate README: %v\n", err)
		}
	}

	// Export history data for time-travel feature if requested
	if config.IncludeHistory {
		fmt.Println("  -> Generating time-travel history data...")
		if historyReport, err := generateHistoryForExport(exportIssues); err == nil && historyReport != nil {
			historyPath := filepath.Join(bundlePath, "data", "history.json")
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

	fmt.Printf("  -> Bundle created: %s\n", bundlePath)
	fmt.Println("")

	// Offer preview and deploy (for GitHub and Cloudflare)
	if config.DeployTarget == "github" || config.DeployTarget == "cloudflare" {
		action, err := wizard.OfferPreview()
		if err != nil {
			return err
		}

		if action == "cancel" {
			// User cancelled after preview - show local result instead
			fmt.Println("Deployment cancelled. Bundle available at:", bundlePath)
			result := &export.WizardResult{
				BundlePath:   bundlePath,
				DeployTarget: "local",
			}
			wizard.PrintSuccess(result)
		} else {
			// Perform deployment with issue count for verification
			result, err := wizard.PerformDeployWithIssueCount(len(exportIssues))
			if err != nil {
				return err
			}

			wizard.PrintSuccess(result)
		}
	} else {
		// Local export - just show success
		result := &export.WizardResult{
			BundlePath:   bundlePath,
			DeployTarget: "local",
		}
		wizard.PrintSuccess(result)
	}

	// Persist source metadata and last-export info for reliable updates.
	if source.BeadsDir != "" {
		config.SourceBeadsDir = source.BeadsDir
	}
	if source.RepoRoot != "" {
		config.SourceRepoRoot = source.RepoRoot
	}
	config.LastIssueCount = len(exportIssues)
	config.LastDataHash = analysis.ComputeDataHash(exportIssues)

	// Save config for next run
	export.SaveWizardConfig(config)

	return nil
}

type pagesSource struct {
	Issues   []model.Issue
	BeadsDir string
	RepoRoot string
	Reason   string
}

type pagesSourceCandidate struct {
	BeadsDir string
	Reason   string
}

func resolvePagesSource(config *export.WizardConfig, beadsPath string) (pagesSource, error) {
	var candidates []pagesSourceCandidate
	seen := map[string]bool{}

	addCandidate := func(dir, reason string) {
		if dir == "" {
			return
		}
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		if seen[dir] {
			return
		}
		seen[dir] = true
		candidates = append(candidates, pagesSourceCandidate{BeadsDir: dir, Reason: reason})
	}

	if config.SourceBeadsDir != "" {
		addCandidate(config.SourceBeadsDir, "saved source")
	}
	if config.SourceRepoRoot != "" {
		addCandidate(filepath.Join(config.SourceRepoRoot, ".beads"), "saved repo root")
	}
	if beadsPath != "" {
		addCandidate(filepath.Dir(beadsPath), "current beads path")
	}
	if dir, err := loader.GetBeadsDir(""); err == nil {
		addCandidate(dir, "current repo")
	}

	var lastErr error
	for _, cand := range candidates {
		if info, err := os.Stat(cand.BeadsDir); err != nil || !info.IsDir() {
			continue
		}
		issues, err := loadIssuesFromBeadsDir(cand.BeadsDir)
		if err != nil {
			lastErr = err
			continue
		}
		src := pagesSource{
			Issues:   issues,
			BeadsDir: cand.BeadsDir,
			RepoRoot: filepath.Dir(cand.BeadsDir),
			Reason:   cand.Reason,
		}

		// If the issue count looks wildly off, try to auto-detect a better source.
		if isSuspiciousIssueCount(len(issues), config.LastIssueCount) {
			if improved, ok := findBetterPagesSource(config, src, beadsPath); ok {
				return improved, nil
			}
		}
		return src, nil
	}

	if lastErr != nil {
		return pagesSource{}, lastErr
	}
	return pagesSource{}, fmt.Errorf("no valid beads source found for pages export")
}

func findBetterPagesSource(config *export.WizardConfig, current pagesSource, beadsPath string) (pagesSource, bool) {
	expected := config.LastIssueCount
	currentCount := len(current.Issues)
	currentDiff := absInt(currentCount - expected)

	repoHint := strings.ToLower(strings.TrimSpace(config.RepoName))
	altHint := repoHint
	if strings.HasPrefix(altHint, "beads-for-") {
		altHint = strings.TrimPrefix(altHint, "beads-for-")
	}

	roots := []string{}
	seenRoots := map[string]bool{}
	addRoot := func(root string) {
		if root == "" {
			return
		}
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		if seenRoots[root] {
			return
		}
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			return
		}
		seenRoots[root] = true
		roots = append(roots, root)
	}

	if config.SourceRepoRoot != "" {
		addRoot(config.SourceRepoRoot)
	}
	if beadsPath != "" {
		addRoot(filepath.Dir(filepath.Dir(beadsPath)))
	}
	if cwd, err := os.Getwd(); err == nil {
		addRoot(cwd)
	}
	if home, err := os.UserHomeDir(); err == nil {
		addRoot(home)
	}
	if info, err := os.Stat("/dp"); err == nil && info.IsDir() {
		addRoot("/dp")
	}

	bestDir := ""
	bestCount := 0
	bestDiff := 0
	bestHintDir := ""
	bestHintCount := 0
	bestHintDiff := 0

	for _, root := range roots {
		for _, beadsDir := range discoverBeadsDirs(root, 4) {
			if beadsDir == current.BeadsDir {
				continue
			}
			count, err := countIssuesInBeadsDir(beadsDir)
			if err != nil || count == 0 {
				continue
			}

			pathLower := strings.ToLower(beadsDir)
			hintMatch := repoHint != "" && (strings.Contains(pathLower, repoHint) || strings.Contains(pathLower, altHint))

			if expected > 0 {
				diff := absInt(count - expected)
				if bestDir == "" || diff < bestDiff || (diff == bestDiff && count > bestCount) {
					bestDir = beadsDir
					bestCount = count
					bestDiff = diff
				}
				if hintMatch && (bestHintDir == "" || diff < bestHintDiff || (diff == bestHintDiff && count > bestHintCount)) {
					bestHintDir = beadsDir
					bestHintCount = count
					bestHintDiff = diff
				}
			} else if count > bestCount {
				if hintMatch && count > bestHintCount {
					bestHintDir = beadsDir
					bestHintCount = count
				} else if bestHintDir == "" {
					bestDir = beadsDir
					bestCount = count
				}
			}
		}
	}

	if bestHintDir != "" && (bestDir == "" || bestHintDiff <= bestDiff) {
		bestDir = bestHintDir
		bestCount = bestHintCount
		bestDiff = bestHintDiff
	}

	if bestDir == "" {
		return pagesSource{}, false
	}

	if expected > 0 && bestDiff >= currentDiff {
		return pagesSource{}, false
	}
	if expected == 0 && bestCount <= currentCount {
		return pagesSource{}, false
	}

	issues, err := loadIssuesFromBeadsDir(bestDir)
	if err != nil {
		return pagesSource{}, false
	}
	return pagesSource{
		Issues:   issues,
		BeadsDir: bestDir,
		RepoRoot: filepath.Dir(bestDir),
		Reason:   "auto-detected better source",
	}, true
}

func discoverBeadsDirs(root string, maxDepth int) []string {
	var dirs []string
	root = filepath.Clean(root)
	sep := string(os.PathSeparator)

	skip := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		"target":       true,
		".cache":       true,
		".bt":          true,
		".idea":        true,
		".vscode":      true,
	}

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		name := d.Name()
		if skip[name] && path != root {
			return fs.SkipDir
		}

		rel := strings.TrimPrefix(strings.TrimPrefix(path, root), sep)
		if rel != "" {
			if depth := len(strings.Split(rel, sep)); depth > maxDepth {
				return fs.SkipDir
			}
		}

		if name == ".beads" {
			dirs = append(dirs, path)
			return fs.SkipDir
		}
		return nil
	})
	return dirs
}

func countIssuesInBeadsDir(beadsDir string) (int, error) {
	if path, typ := metadataPreferredSource(beadsDir); path != "" {
		info, err := os.Stat(path)
		if err == nil {
			priority := datasource.PriorityJSONLLocal
			if typ == datasource.SourceTypeSQLite {
				priority = datasource.PrioritySQLite
			}
			source := datasource.DataSource{
				Type:     typ,
				Path:     path,
				Priority: priority,
				ModTime:  info.ModTime(),
				Size:     info.Size(),
			}
			if err := datasource.ValidateSource(&source); err == nil {
				return source.IssueCount, nil
			}
		}
	}

	sources, err := datasource.DiscoverSources(datasource.DiscoveryOptions{
		BeadsDir:               beadsDir,
		ValidateAfterDiscovery: true,
		IncludeInvalid:         false,
	})
	if err != nil {
		return 0, err
	}
	if len(sources) == 0 {
		return 0, fmt.Errorf("no sources in %s", beadsDir)
	}
	result, err := datasource.SelectBestSourceDetailed(sources, datasource.DefaultSelectionOptions())
	if err != nil {
		return 0, err
	}
	return result.Selected.IssueCount, nil
}

type beadsMetadata struct {
	Database     string `json:"database"`
	JSONLExport  string `json:"jsonl_export"`
	Backend      string `json:"backend"`
	DoltMode     string `json:"dolt_mode"`
	DoltDatabase string `json:"dolt_database"`
}

func metadataPreferredSource(beadsDir string) (string, datasource.SourceType) {
	metaPath := filepath.Join(beadsDir, "metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return "", ""
	}
	var meta beadsMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", ""
	}

	// Dolt backend takes priority when configured
	if meta.Backend == "dolt" {
		cfg, ok := datasource.ReadDoltConfig(beadsDir)
		if ok {
			return cfg.DSN(), datasource.SourceTypeDolt
		}
	}

	if meta.Database != "" {
		path := meta.Database
		if !filepath.IsAbs(path) {
			path = filepath.Join(beadsDir, path)
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, datasource.SourceTypeSQLite
		}
	}
	if meta.JSONLExport != "" {
		path := meta.JSONLExport
		if !filepath.IsAbs(path) {
			path = filepath.Join(beadsDir, path)
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, datasource.SourceTypeJSONLLocal
		}
	}
	return "", ""
}

func loadIssuesFromBeadsDir(beadsDir string) ([]model.Issue, error) {
	if path, typ := metadataPreferredSource(beadsDir); path != "" {
		switch typ {
		case datasource.SourceTypeDolt:
			reader, err := datasource.NewDoltReader(datasource.DataSource{
				Type: datasource.SourceTypeDolt,
				Path: path,
			})
			if err != nil {
				break
			}
			defer reader.Close()
			if issues, err := reader.LoadIssues(); err == nil {
				return issues, nil
			}
		case datasource.SourceTypeSQLite:
			reader, err := datasource.NewSQLiteReader(datasource.DataSource{
				Type: datasource.SourceTypeSQLite,
				Path: path,
			})
			if err != nil {
				break
			}
			defer reader.Close()
			if issues, err := reader.LoadIssues(); err == nil {
				return issues, nil
			}
		case datasource.SourceTypeJSONLLocal:
			if issues, err := loader.LoadIssuesFromFile(path); err == nil {
				return issues, nil
			}
		}
	}
	return datasource.LoadIssuesFromDir(beadsDir)
}

// ============================================================================
// Static Pages Export Helpers (bv-73f)
// ============================================================================

// copyViewerAssets copies the viewer HTML/JS/CSS assets to the output directory.
// If title is provided, it replaces the default title in index.html.
func copyViewerAssets(outputDir, title string) error {
	// First try to use embedded assets (production builds)
	if export.HasEmbeddedAssets() {
		return export.CopyEmbeddedAssets(outputDir, title)
	}

	// Fall back to filesystem-based approach (development mode)
	assetsDir := findViewerAssetsDir()
	if assetsDir == "" {
		return fmt.Errorf("viewer assets not found")
	}

	if err := maybeBuildHybridWasmAssets(assetsDir); err != nil {
		return err
	}

	// Files to copy
	files := []string{
		"index.html",
		"viewer.js",
		"styles.css",
		"graph.js",
		"charts.js",
		"hybrid_scorer.js",
		"wasm_loader.js",
		"coi-serviceworker.js",
	}

	for _, file := range files {
		src := filepath.Join(assetsDir, file)
		dst := filepath.Join(outputDir, file)

		// Special handling for index.html to replace title and add cache-busting
		if file == "index.html" {
			if err := copyFileWithTitleAndCacheBusting(src, dst, title); err != nil {
				return fmt.Errorf("copy %s: %w", file, err)
			}
			continue
		}

		if err := copyFile(src, dst); err != nil {
			// Skip missing optional files
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("copy %s: %w", file, err)
		}
	}

	// Copy vendor directory
	vendorSrc := filepath.Join(assetsDir, "vendor")
	vendorDst := filepath.Join(outputDir, "vendor")
	if err := copyDir(vendorSrc, vendorDst); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("copy vendor: %w", err)
		}
	}

	// Copy optional WASM directory
	wasmSrc := filepath.Join(assetsDir, "wasm")
	wasmDst := filepath.Join(outputDir, "wasm")
	if err := copyDir(wasmSrc, wasmDst); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("copy wasm: %w", err)
		}
	}

	// Always add GitHub Actions workflow for reliable Pages deployment
	// This ensures the workflow is in the bundle regardless of deployment target
	if err := export.WriteGitHubActionsWorkflow(outputDir); err != nil {
		// Non-fatal - just log a warning
		fmt.Printf("  Warning: Could not add GitHub Actions workflow: %v\n", err)
	}

	return nil
}

func maybeBuildHybridWasmAssets(assetsDir string) error {
	if os.Getenv("BT_BUILD_HYBRID_WASM") == "" {
		return nil
	}

	wasmPackPath, err := exec.LookPath("wasm-pack")
	if err != nil {
		return fmt.Errorf("BT_BUILD_HYBRID_WASM is set but wasm-pack was not found in PATH")
	}

	wasmSrc := filepath.Join(assetsDir, "..", "wasm_scorer")
	info, err := os.Stat(wasmSrc)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("hybrid wasm source directory not found at %s", wasmSrc)
	}

	outDir := filepath.Join(assetsDir, "wasm")
	cmd := exec.Command(wasmPackPath, "build", "--release", "--target", "web", "--out-dir", outDir)
	cmd.Dir = wasmSrc
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build hybrid wasm: %w", err)
	}
	return nil
}

// findViewerAssetsDir locates the viewer assets directory.
func findViewerAssetsDir() string {
	// Try relative to current working directory (development)
	candidates := []string{
		"pkg/export/viewer_assets",
		"../pkg/export/viewer_assets",
		"../../pkg/export/viewer_assets",
	}

	// Try relative to executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "pkg/export/viewer_assets"),
			filepath.Join(exeDir, "../pkg/export/viewer_assets"),
			filepath.Join(exeDir, "../../pkg/export/viewer_assets"),
		)
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return ""
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// copyFileWithTitleAndCacheBusting copies a file while replacing the default title
// and adding cache-busting query parameters to script tags.
func copyFileWithTitleAndCacheBusting(src, dst, title string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	result := string(content)

	// Replace title in <title> tag and in the h1 header (if title provided)
	if title != "" {
		safeTitle := html.EscapeString(title)
		result = strings.Replace(result, "<title>Beads Viewer</title>", "<title>"+safeTitle+"</title>", 1)
		result = strings.Replace(result, `<h1 class="text-xl font-semibold">Beads Viewer</h1>`, `<h1 class="text-xl font-semibold">`+safeTitle+`</h1>`, 1)
	}

	// Always add cache-busting to script tags to prevent CDN from serving stale JS files
	result = export.AddScriptCacheBusting(result)

	return os.WriteFile(dst, []byte(result), 0644)
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// generateREADME creates a README.md file for the GitHub Pages repository.
// It includes actionable insights, graph analysis, and a direct link to the live site.
func generateREADME(bundlePath, title, pagesURL string, issues []model.Issue, triage *analysis.TriageResult, stats *analysis.GraphStats) error {
	var b strings.Builder

	// Title
	if title == "" {
		title = "Project Dashboard"
	}
	b.WriteString(fmt.Sprintf("# %s\n\n", title))

	// Prominent live link - THE MOST IMPORTANT THING
	if pagesURL != "" {
		b.WriteString(fmt.Sprintf("## 🔗 [View Live Dashboard](%s)\n\n", pagesURL))
	}

	// Executive summary - not boring counts, but actionable intelligence
	if triage != nil {
		health := triage.ProjectHealth.Counts

		// Quick status line
		completionPct := float64(0)
		if health.Total > 0 {
			completionPct = float64(health.Closed) / float64(health.Total) * 100
		}

		b.WriteString("## 📊 Executive Summary\n\n")
		b.WriteString(fmt.Sprintf("**%d** total issues | **%.0f%%** complete | **%d** ready to work | **%d** blocked\n\n",
			health.Total, completionPct, health.Actionable, health.Blocked))

		// Health assessment
		if health.Blocked > 0 && health.Actionable > 0 {
			blockRatio := float64(health.Blocked) / float64(health.Actionable)
			if blockRatio > 1.0 {
				b.WriteString("⚠️ **Health Warning:** More issues are blocked than actionable. Focus on clearing blockers.\n\n")
			}
		}
	}

	// TOP RECOMMENDATIONS - the actual useful content
	if triage != nil && len(triage.QuickRef.TopPicks) > 0 {
		b.WriteString("## 🎯 Top Priorities\n\n")
		b.WriteString("The graph analysis identified these as the highest-impact items to work on:\n\n")

		for i, pick := range triage.QuickRef.TopPicks {
			b.WriteString(fmt.Sprintf("### %d. %s\n", i+1, pick.Title))
			b.WriteString(fmt.Sprintf("**ID:** `%s` | **Impact Score:** %.2f", pick.ID, pick.Score))
			if pick.Unblocks > 0 {
				b.WriteString(fmt.Sprintf(" | **Unblocks:** %d issues", pick.Unblocks))
			}
			b.WriteString("\n\n")

			if len(pick.Reasons) > 0 {
				b.WriteString("**Why this matters:**\n")
				for _, reason := range pick.Reasons {
					b.WriteString(fmt.Sprintf("- %s\n", reason))
				}
				b.WriteString("\n")
			}
		}
	}

	// CRITICAL BLOCKERS - what's holding everything up
	if triage != nil && len(triage.BlockersToClear) > 0 {
		b.WriteString("## 🚧 Critical Bottlenecks\n\n")
		b.WriteString("These issues are blocking the most downstream work. Clearing them has outsized impact:\n\n")

		maxBlockers := 5
		if len(triage.BlockersToClear) < maxBlockers {
			maxBlockers = len(triage.BlockersToClear)
		}

		b.WriteString("| Issue | Title | Unblocks | Status |\n")
		b.WriteString("|-------|-------|----------|--------|\n")
		for i := 0; i < maxBlockers; i++ {
			blocker := triage.BlockersToClear[i]
			status := "Ready"
			if !blocker.Actionable {
				status = fmt.Sprintf("Blocked by %d", len(blocker.BlockedBy))
			}
			// Escape title to prevent markdown table breakage
			safeTitle := escapeMarkdownTableCell(truncateTitle(blocker.Title, 40))
			b.WriteString(fmt.Sprintf("| `%s` | %s | **%d** issues | %s |\n",
				blocker.ID, safeTitle, blocker.UnblocksCount, status))
		}
		if len(triage.BlockersToClear) > 5 {
			b.WriteString(fmt.Sprintf("\n*+%d more bottlenecks in the dashboard*\n", len(triage.BlockersToClear)-5))
		}
		b.WriteString("\n")
	}

	// CYCLES - these are BUGS in the project structure!
	// Cache cycles since Cycles() does a deep copy each call
	var cycles [][]string
	if stats != nil {
		cycles = stats.Cycles()
	}
	if len(cycles) > 0 {
		b.WriteString("## 🔴 Dependency Cycles Detected!\n\n")
		b.WriteString("**These are structural bugs** that make completion impossible. Fix immediately:\n\n")

		maxCycles := 3
		if len(cycles) < maxCycles {
			maxCycles = len(cycles)
		}
		for i := 0; i < maxCycles; i++ {
			cycle := cycles[i]
			b.WriteString(fmt.Sprintf("- `%s`\n", strings.Join(cycle, "` → `")))
		}
		if len(cycles) > 3 {
			b.WriteString(fmt.Sprintf("\n*+%d more cycles - see dashboard for details*\n", len(cycles)-3))
		}
		b.WriteString("\n")
	}

	// ALERTS - important warnings
	if triage != nil && len(triage.Alerts) > 0 {
		hasCritical := false
		hasWarning := false
		for _, alert := range triage.Alerts {
			if alert.Severity == "critical" {
				hasCritical = true
			} else if alert.Severity == "warning" {
				hasWarning = true
			}
		}

		if hasCritical || hasWarning {
			b.WriteString("## ⚠️ Alerts\n\n")
			for _, alert := range triage.Alerts {
				if alert.Severity == "critical" || alert.Severity == "warning" {
					icon := "🟡"
					if alert.Severity == "critical" {
						icon = "🔴"
					}
					b.WriteString(fmt.Sprintf("- %s **%s**: %s\n", icon, alert.Type, alert.Message))
				}
			}
			b.WriteString("\n")
		}
	}

	// GRAPH ANALYSIS INSIGHTS - what the analysis tells us
	if stats != nil && stats.NodeCount > 0 {
		b.WriteString("## 📈 Graph Analysis\n\n")

		// Density interpretation
		densityHealth := "🟢 Healthy"
		densityDesc := "Issues are well-isolated and can be parallelized"
		if stats.Density > 0.15 {
			densityHealth = "🔴 High Coupling"
			densityDesc = "Many inter-dependencies; changes cascade widely"
		} else if stats.Density > 0.05 {
			densityHealth = "🟡 Moderate"
			densityDesc = "Normal coupling for a complex project"
		}

		b.WriteString(fmt.Sprintf("- **Dependency Density:** %.3f (%s) — %s\n", stats.Density, densityHealth, densityDesc))
		b.WriteString(fmt.Sprintf("- **Graph Size:** %d issues with %d dependencies\n", stats.NodeCount, stats.EdgeCount))

		// Use cached cycles variable (already fetched above)
		if len(cycles) > 0 {
			b.WriteString(fmt.Sprintf("- **Cycles:** %d circular dependencies detected (must fix!)\n", len(cycles)))
		} else if stats.EdgeCount > 0 {
			b.WriteString("- **Cycles:** None detected ✓\n")
		}
		b.WriteString("\n")
	}

	// QUICK WINS - low effort, high impact
	if triage != nil && len(triage.QuickWins) > 0 {
		b.WriteString("## 🏃 Quick Wins\n\n")
		b.WriteString("Low-effort items that clear the path forward:\n\n")

		maxWins := 5
		if len(triage.QuickWins) < maxWins {
			maxWins = len(triage.QuickWins)
		}
		for i := 0; i < maxWins; i++ {
			qw := triage.QuickWins[i]
			unblockText := ""
			if len(qw.UnblocksIDs) > 0 {
				unblockText = fmt.Sprintf(" (unblocks %d)", len(qw.UnblocksIDs))
			}
			b.WriteString(fmt.Sprintf("- **%s**: %s%s\n", qw.ID, qw.Title, unblockText))
			if qw.Reason != "" {
				b.WriteString(fmt.Sprintf("  - *%s*\n", qw.Reason))
			}
		}
		if len(triage.QuickWins) > 5 {
			b.WriteString(fmt.Sprintf("\n*+%d more quick wins in the dashboard*\n", len(triage.QuickWins)-5))
		}
		b.WriteString("\n")
	}

	// SUMMARY STATS - compact reference at the end
	if triage != nil {
		health := triage.ProjectHealth.Counts
		b.WriteString("## 📋 Status Summary\n\n")

		// Priority breakdown inline
		if len(health.ByPriority) > 0 {
			var prioItems []string
			priorities := []int{0, 1, 2, 3, 4}
			for _, p := range priorities {
				if count, ok := health.ByPriority[p]; ok && count > 0 {
					prioItems = append(prioItems, fmt.Sprintf("P%d: %d", p, count))
				}
			}
			if len(prioItems) > 0 {
				b.WriteString(fmt.Sprintf("**By Priority:** %s\n\n", strings.Join(prioItems, " | ")))
			}
		}

		// Type breakdown inline
		if len(health.ByType) > 0 {
			var typeItems []string
			for t, count := range health.ByType {
				if count > 0 {
					typeItems = append(typeItems, fmt.Sprintf("%s: %d", t, count))
				}
			}
			if len(typeItems) > 0 {
				sort.Strings(typeItems)
				b.WriteString(fmt.Sprintf("**By Type:** %s\n\n", strings.Join(typeItems, " | ")))
			}
		}
	}

	// Footer with timestamp and links
	b.WriteString("---\n\n")
	b.WriteString(fmt.Sprintf("*Generated %s by [bv](https://github.com/seanmartinsmith/beadstui)*\n\n", time.Now().Format("Jan 2, 2006 at 3:04 PM MST")))

	if pagesURL != "" {
		b.WriteString(fmt.Sprintf("**[Open Interactive Dashboard](%s)** for full details, dependency graph, search, and time-travel.\n", pagesURL))
	}

	// Write to file
	readmePath := filepath.Join(bundlePath, "README.md")
	return os.WriteFile(readmePath, []byte(b.String()), 0644)
}

