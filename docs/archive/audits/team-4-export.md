# Audit Report: Export & Rendering

**Team**: 4
**Scope**: pkg/export/ - export formats, graph rendering, wizard, deploy/publish features
**Lines scanned**: ~7,200 source lines across 15 production files, ~267 test functions across 25 test files, plus ~25 embedded viewer asset files

## Architecture Summary

The `pkg/export/` package is the largest single domain in the codebase, implementing a full export-to-deploy pipeline: data export (Markdown, SQLite, JSON, graph formats), graph visualization (interactive HTML, static SVG/PNG, Mermaid, DOT), a local preview server with SSE-based live reload, and deployment to GitHub Pages and Cloudflare Pages. The package also embeds a complete viewer application (HTML/JS/CSS/WASM) under `viewer_assets/` and two JavaScript libraries (`force-graph.min.js`, `marked.min.js`) at the package root for the interactive graph HTML.

The deployment wizard (`wizard.go`) orchestrates the full flow: collect export options via `huh` forms, check prerequisites (CLI tools, auth), preview locally, deploy, and verify. Configuration is persisted to `~/.config/bt/pages-wizard.json` (with a testable `wizardConfigHome` override). The SQLite exporter (`sqlite_export.go`) creates a client-side-queryable database with FTS5, materialized views, and chunking for large datasets, designed for the sql.js WASM architecture. The graph snapshot system (`graph_snapshot.go`) produces SVG/PNG images using `gg` and `ajstarks/svgo`, while `graph_render_beautiful.go` generates a massive (~1,857 lines) self-contained HTML file with embedded force-graph visualization, dark/light themes, heatmaps, path finder, minimap, and full keyboard shortcuts.

Data flows from `model.Issue` -> export functions -> output files. The package is consumed by `cmd/bt/main.go` and `pkg/ui/model.go`. Internal dependencies include `pkg/model`, `pkg/analysis` (GraphStats, TriageResult), and `pkg/correlation` (git history). External dependencies include `charmbracelet/huh` (forms), `fsnotify` (file watching), `gg` (2D graphics), `ajstarks/svgo`, `modernc.org/sqlite`, and `golang.org/x/term`.

## Feature Inventory

| Feature | Location | LOC | Dolt-Compatible | Tested | Functional | Notes |
|---------|----------|-----|-----------------|--------|------------|-------|
| Markdown export | markdown.go | ~800 | Yes | Yes (52 tests) | Yes | Full issue report with TOC, Mermaid graph, commands, priority brief |
| Priority brief (Markdown) | markdown.go:488-801 | ~313 | Yes | Partial | Partial | `GeneratePriorityBrief` returns placeholder text; `GeneratePriorityBriefFromTriageJSON` is functional |
| Mermaid graph generator | mermaid_generator.go | ~136 | Yes | Yes | Yes | Shared by markdown export and graph export; collision-free IDs via FNV hash |
| Graph export (JSON/DOT/Mermaid) | graph_export.go | ~560 | Yes | Yes (12 tests) | Yes | Label filter, root subgraph, depth limit, PageRank in adjacency nodes |
| Interactive graph HTML | graph_interactive.go + graph_render_beautiful.go | ~340 + ~1,857 | Yes | Yes (5 tests) | Yes | Self-contained HTML with force-graph.js, dark/light mode, heatmaps, path finder, context menu, docked/floating panel |
| Graph snapshot (SVG/PNG) | graph_snapshot.go | ~538 | Yes | Yes (36+4 bench) | Yes | Static image with summary header, legend, PageRank-based layout |
| SQLite export | sqlite_export.go + sqlite_schema.go + sqlite_types.go | ~885 + ~319 + ~172 | Yes | Yes (24 tests) | Yes | FTS5, materialized views, chunking, robot JSON outputs, graph layout pre-compute |
| Preview server | preview.go | ~373 | N/A | Yes (17 tests) | Yes | Local HTTP with auto-browser, no-cache, status endpoint, configurable live-reload |
| Live reload (SSE) | livereload.go | ~333 | N/A | Yes (8 tests) | Yes | fsnotify watcher, script injection before `</body>`, debouncing |
| Viewer asset embedding | viewer_embed.go | ~151 | N/A | Yes (13 tests) | Yes | Copies embedded assets, title replacement, cache-busting, auto-adds GH Actions workflow |
| GitHub Pages deploy | github.go | ~913 | N/A | Yes (33 tests) | Yes | Full flow: check CLI, auth, create repo, init+push, enable Pages, fallback to legacy |
| Cloudflare Pages deploy | cloudflare.go | ~714 | N/A | Yes (13 tests) | Yes | Full flow: check wrangler, auth, auto-create project, deploy, verify |
| Deployment wizard | wizard.go | ~958 | N/A | Yes (19 tests) | Yes | Interactive huh forms, saved config, TTY detection, pipe handling |

## Dependencies

- **Depends on**:
  - `pkg/model` - Issue, Dependency, Status, IssueType types
  - `pkg/analysis` - GraphStats (PageRank, Betweenness, CriticalPath, Cycles, etc.), TriageResult
  - `pkg/correlation` - HistoryReport, CorrelatedCommit (for interactive graph)
  - `charmbracelet/huh` - interactive form library (wizard)
  - `golang.org/x/term` - terminal detection
  - `fsnotify/fsnotify` - file system watching (live reload)
  - `git.sr.ht/~sbinet/gg` - 2D graphics (PNG rendering)
  - `github.com/ajstarks/svgo` - SVG generation
  - `golang.org/x/image/font/basicfont` - font face for PNG rendering
  - `modernc.org/sqlite` - pure-Go SQLite driver (export database)
  - External CLIs at runtime: `gh` (GitHub), `wrangler` (Cloudflare), `git`, `curl`, `npm`
- **Depended on by**:
  - `cmd/bt/main.go` - main entry point uses most export functions
  - `pkg/ui/model.go` - TUI model imports export (likely for shared types)

## Dead Code Candidates

1. **`GeneratePriorityBrief` (markdown.go:510)** - Takes an `interface{}` parameter, never does type assertion on it, and returns placeholder table rows with "Run `bt --robot-triage` for data". The production path uses `GeneratePriorityBriefFromTriageJSON` instead. This function appears to be a stub that was never completed.

2. **`DeployToCloudflarePages` (cloudflare.go:277)** - Nearly identical to `DeployToCloudflareWithAutoCreate` (which adds auto-create and verification). The wizard uses `DeployToCloudflareWithAutoCreate`. The simpler version may be dead.

3. **`DeployToGitHubPages` (github.go:397)** - Called only from `DeployToGitHubPagesWithFallback`. Not directly called externally. Could be inlined into the fallback version.

4. **`stringSliceContains` (sqlite_export.go:732)** - Defined but never called anywhere in the package.

5. **`ExportDependency`, `ExportMetrics` types (sqlite_types.go:42-57)** - Defined but never used in production code (only `ExportIssue` is used).

6. **`TriageRecommendation`, `QuickWin`, `ProjectHealth`, `Velocity`, `VelocityWeek` types (sqlite_types.go:121-172)** - These appear to duplicate types from `pkg/analysis`. Not clear if they're used.

7. **`getTypeIcon` (markdown.go:786)** - Duplicate of `getTypeEmoji` (markdown.go:286) with identical logic. Both exist in the same file.

8. **`AddGitHubWorkflowToBundle` (viewer_embed.go:148)** - One-line wrapper around `WriteGitHubActionsWorkflow`. CopyEmbeddedAssets already calls `WriteGitHubActionsWorkflow` directly. No callers found in `main.go` for this wrapper.

## Notable Findings

### Stale "bv" naming throughout
The package doc comments on multiple files still say "Package export provides data export functionality for bv." The interactive graph HTML displays a "bv" logo icon (line 601), links to beadstui as "bv" in the footer (line 863), and uses `bv-graph-theme`/`bv-graph-layout` localStorage keys. The wizard banner says "bv -> Static Site Deployment Wizard" (line 270). The `collectLocalConfig` default path is `./bv-pages` (line 462). `SuggestProjectName` and `SuggestRepoName` both check for `bv-pages` as a special case directory name. The `LiveReloadScript` logs `[bv]` to the browser console. None of these are bugs, but they're incomplete rename artifacts.

### `graph_render_beautiful.go` is a ~1,857-line Go file
~1,200 lines of it are an inline HTML/CSS/JS template inside a single `fmt.Sprintf` call. This is a remarkably feature-rich self-contained visualization: force-directed graph, DAG modes, radial layout, search with full-text matching, context menu, path finder between nodes, heatmap coloring, minimap, recently-viewed tracking, light/dark mode, keyboard shortcuts, and docked/floating detail panels with markdown rendering. However, this makes it essentially impossible to maintain, lint, or test the CSS/JS separately. The `%%` escaping throughout (required by fmt.Sprintf) adds another layer of fragility.

### Duplicate Mermaid generation code
`graph_export.go:generateMermaid()` (line 372) and `mermaid_generator.go:GenerateMermaidGraph()` contain nearly identical code - same classDef strings, same getSafeID closure with FNV hashing, same edge style logic. The markdown export uses `GenerateMermaidGraph` from `mermaid_generator.go`, while the graph export command uses `generateMermaid` from `graph_export.go`. One should be removed.

### Duplicate truncation helpers
Three truncation functions exist: `truncateString` (markdown.go:774, rune-safe with ellipsis character), `truncateRunes` (graph_export.go:358, with "..." suffix), and `truncate` (graph_snapshot.go:521, with "..." suffix). The latter two are nearly identical.

### SQLite export is substantial and well-architected
The SQLite export creates a production-quality database with proper schema (issues, dependencies, comments, metrics, triage recommendations), FTS5 full-text search, materialized views with denormalized data, performance indexes, and chunking for large databases. The output includes `graph_layout.json` for pre-computed positions and `meta.json` / `triage.json` / `project_health.json` for robot consumption. This is clearly the most "production-ready" part of the export system.

### Preview server has no authentication
The preview server binds to `127.0.0.1` (good - localhost only) but has no authentication. This is fine for local preview but worth noting.

### Viewer assets include a full WASM module
The embedded `viewer_assets/` includes `bv_graph_bg.wasm` and `bv_graph.js` (a Rust-compiled WASM module), sql.js WASM (`sql-wasm.wasm` + `sql-wasm.js`), multiple vendor JS libraries (Alpine.js, Chart.js, D3 v7, DOMPurify, force-graph, marked, mermaid, Tailwind CSS), and font files. This is a complete standalone web application. The directory name `bv-graph-wasm` in the vendor files is a carry-over from the original project.

### Two embedded JS files at package root
`force-graph.min.js` and `marked.min.js` exist both at `pkg/export/` (embedded via `//go:embed` in `graph_interactive.go`) AND inside `viewer_assets/vendor/`. The root-level copies are specifically for the interactive graph HTML generator, while the viewer_assets copies are for the SQLite-backed viewer. This means the binary carries two copies of each library.

### `wizard.go` stdin handling is complex
Lines 159-203 contain elaborate pipe-detection and timeout logic for non-TTY stdin, including a goroutine that reads all stdin with a 100ms timeout. This is defensive but fragile - the comment at line 199 explicitly notes a race condition.

## Questions for Synthesis

1. **Should the `bv` naming be cleaned up in this package?** There are dozens of stale "bv" references in comments, HTML output, localStorage keys, default paths, and console logs. These don't break functionality but are inconsistent with the bt/beadstui rename.

2. **Is the full Cloudflare Pages integration still needed?** Given that the project has pivoted to Dolt-only and the user's use case is a personal issue tracker TUI, is the Cloudflare deployment path justified, or is it dead weight?

3. **Is the full GitHub Pages deployment pipeline needed?** Same question. The wizard, repo creation, Pages enablement, Actions workflow, legacy fallback, and verification polling are sophisticated but may be unused.

4. **Should `graph_render_beautiful.go` be refactored?** The 1,200+ lines of inline HTML/CSS/JS in a Go string literal are difficult to maintain. Options: extract to a template file, use Go's `html/template`, or embed it like the viewer assets.

5. **Can the duplicate Mermaid generators be consolidated?** `generateMermaid` in `graph_export.go` and `GenerateMermaidGraph` in `mermaid_generator.go` are nearly identical.

6. **Are the `ExportDependency`, `ExportMetrics`, `TriageRecommendation`, `QuickWin`, `ProjectHealth`, `Velocity` types in `sqlite_types.go` actually used?** They may be consumed by the viewer JavaScript or external tools, but within the Go codebase they appear unused.

7. **Should the duplicate `force-graph.min.js` and `marked.min.js` be consolidated?** Having two copies embedded in the binary wastes space (these are non-trivial JS libraries).

8. **What is the relationship between the interactive graph HTML (`graph_render_beautiful.go`) and the viewer assets web app (`viewer_assets/`)?** They appear to be two independent visualization approaches - is one superseding the other?
