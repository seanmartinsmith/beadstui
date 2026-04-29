# Audit Report: CLI & Entry Point

**Team**: 2
**Scope**: cmd/bt/ - CLI flags, startup flow, robot mode, Dolt lifecycle init
**Lines scanned**: 9,164 (main.go: 8,143, search_output.go: 150, main_test.go: 275, main_robot_test.go: 279, burndown_test.go: 96, profile_test.go: 221)

## Architecture Summary

The CLI entry point lives in `cmd/bt/` with two production files: `main.go` (8,143 lines) and `search_output.go` (150 lines). The entire binary is driven from a single `main()` function that uses `github.com/spf13/pflag` for flag parsing, then dispatches to one of many code paths based on which flags are set. There is no subcommand architecture - everything is a top-level flag.

The binary operates in two fundamental modes: **TUI mode** (default, launches Bubble Tea) and **robot mode** (headless JSON/TOON output for AI agents). Robot mode is activated by any `--robot-*` flag, `BT_ROBOT=1`, or piping stdout to a non-TTY. Robot mode covers approximately 30 distinct commands, each implemented as inline logic in `main()` with a pattern of: build data -> serialize to JSON -> `os.Exit(0)`. The TUI mode is launched at the very end of `main()` after all robot/export paths have been exhausted.

The file also contains substantial helper code that does not belong in a CLI entry point: burndown calculation (~200 lines), static site export/wizard logic (~900 lines), README generation (~250 lines), profile reporting (~200 lines), recipe filtering/sorting (~200 lines), TOON encoding infrastructure (~100 lines), JSON Schema generation (~270 lines), machine-readable docs generation (~250 lines), and various utility functions. `search_output.go` properly extracts search-related types and helpers into a separate file, but this pattern was not applied elsewhere.

## Feature Inventory

### CLI Flags (Complete List)

| Feature | Location | LOC | Dolt-Compatible | Tested | Functional | Notes |
|---------|----------|-----|-----------------|--------|------------|-------|
| `--help` | main.go:56 | 6 | N/A | No | Yes | Prints usage |
| `--version` | main.go:57 | 4 | N/A | No | Yes | Prints version string |
| `--cpu-profile` | main.go:55 | 13 | N/A | No | Yes | pprof CPU profile to file |
| `--update` | main.go:59 | 42 | N/A | No | Yes | Self-update from GitHub releases |
| `--check-update` | main.go:60 | 13 | N/A | No | Yes | Check for new version |
| `--rollback` | main.go:61 | 7 | N/A | No | Yes | Rollback to backup |
| `--yes` | main.go:62 | 1 | N/A | No | Yes | Skip confirmation prompts |
| `--export-md` | main.go:63 | 49 | Yes | No | Yes | Export issues to Markdown |
| `--format` | main.go:66 | 8 | N/A | Partial | Yes | json or toon output encoding |
| `--stats` | main.go:67 | 1 | N/A | Yes | Yes | TOON token stats on stderr |
| `--robot-help` | main.go:64 | 460 | N/A | No | Yes | Huge inline help text |
| `--robot-docs` | main.go:65 | 12 | N/A | No | Yes | Machine-readable JSON docs |
| `--robot-insights` | main.go:68 | 160 | Yes | Yes | Yes | Graph analysis JSON |
| `--robot-plan` | main.go:69 | 67 | Yes | Yes | Yes | Execution plan JSON |
| `--robot-priority` | main.go:70 | 133 | Yes | Yes | Yes | Priority recommendations |
| `--robot-triage` | main.go:71 | 163 | Yes | No | Yes | Unified triage mega-command |
| `--robot-triage-by-track` | main.go:72 | (shared) | Yes | No | Yes | Grouped by execution track |
| `--robot-triage-by-label` | main.go:73 | (shared) | Yes | No | Yes | Grouped by label |
| `--robot-next` | main.go:74 | 55 | Yes | No | Yes | Single top pick |
| `--robot-diff` | main.go:75 | 65 | Yes | No | Yes | Changes since ref |
| `--robot-recipes` | main.go:76 | 20 | N/A | Yes | Yes | List available recipes |
| `--robot-label-health` | main.go:77 | 28 | Yes | No | Yes | Label health metrics |
| `--robot-label-flow` | main.go:78 | 26 | Yes | No | Yes | Cross-label dependency flow |
| `--robot-label-attention` | main.go:79 | 82 | Yes | No | Yes | Attention-ranked labels |
| `--robot-alerts` | main.go:81 | 128 | Yes | No | Yes | Drift + proactive alerts |
| `--robot-metrics` | main.go:82 | 8 | Yes | No | Yes | Performance metrics |
| `--robot-schema` | main.go:84 | 34 | N/A | Yes | Yes | JSON Schema definitions |
| `--robot-suggest` | main.go:87 | 31 | Yes | No | Yes | Smart suggestions |
| `--robot-graph` | main.go:92 | 36 | Yes | No | Yes | Graph export JSON/DOT/Mermaid |
| `--robot-search` | main.go:112 | (shared) | Yes | No | Yes | Semantic search JSON output |
| `--robot-history` | main.go:129 | 92 | Partial | No | Yes | Bead-to-commit correlations |
| `--robot-drift` | main.go:128 | (shared) | Yes | No | Yes | Drift check JSON |
| `--robot-orphans` | main.go:142 | 96 | Partial | No | Yes | Orphan commit detection |
| `--robot-file-beads` | main.go:145 | (shared) | Partial | No | Yes | File-to-bead lookup |
| `--robot-file-hotspots` | main.go:147 | (shared) | Partial | No | Yes | High-churn file detection |
| `--robot-impact` | main.go:150 | 75 | Partial | No | Yes | Impact analysis for files |
| `--robot-file-relations` | main.go:152 | 72 | Partial | No | Yes | Co-change detection |
| `--robot-related` | main.go:156 | 92 | Partial | No | Yes | Related work discovery |
| `--robot-blocker-chain` | main.go:161 | 41 | Yes | No | Yes | Blocker chain analysis |
| `--robot-impact-network` | main.go:163 | 86 | Partial | No | Yes | Impact network graph |
| `--robot-causality` | main.go:166 | 83 | Partial | No | Yes | Temporal causality |
| `--robot-sprint-list` | main.go:168 | (shared) | Yes | No | Yes | List sprints |
| `--robot-sprint-show` | main.go:169 | (shared) | Yes | No | Yes | Show sprint details |
| `--robot-forecast` | main.go:171 | 100 | Yes | No | Yes | ETA forecast |
| `--robot-burndown` | main.go:180 | 60 | Yes | Yes | Yes | Sprint burndown data |
| `--robot-capacity` | main.go:176 | 102 | Yes | No | Yes | Capacity simulation |
| `--robot-explain-correlation` | main.go:135 | (shared) | Partial | No | Yes | Explain commit-bead link |
| `--robot-confirm-correlation` | main.go:136 | (shared) | Partial | No | Yes | Confirm correlation |
| `--robot-reject-correlation` | main.go:137 | (shared) | Partial | No | Yes | Reject correlation |
| `--robot-correlation-stats` | main.go:140 | (shared) | Partial | No | Yes | Correlation feedback stats |
| `--search` | main.go:111 | 190 | Yes | No | Yes | Semantic search (human + robot) |
| `--search-limit` | main.go:113 | 1 | N/A | No | Yes | Max results for search |
| `--search-mode` | main.go:114 | 1 | N/A | No | Yes | text or hybrid mode |
| `--search-preset` | main.go:115 | 1 | N/A | No | Yes | Hybrid preset name |
| `--search-weights` | main.go:116 | 1 | N/A | No | Yes | Custom hybrid weights JSON |
| `--diff-since` | main.go:117 | 65 | Yes | No | Yes | Historical diff |
| `--as-of` | main.go:118 | 30 | Yes | No | Yes | Time-travel view |
| `--force-full-analysis` | main.go:119 | 1 | N/A | No | Yes | Force all metrics |
| `--profile-startup` | main.go:120 | 5 | Yes | Yes | Yes | Startup profiling |
| `--profile-json` | main.go:121 | 1 | N/A | Yes | Yes | JSON profile output |
| `--no-hooks` | main.go:122 | 1 | N/A | No | Yes | Skip export hooks |
| `--workspace` | main.go:123 | 22 | Partial | No | Yes | Multi-repo workspace |
| `--repo` | main.go:124 | 3 | N/A | Yes | Yes | Filter by repo prefix |
| `--save-baseline` | main.go:125 | 59 | Yes | No | Yes | Save metrics baseline |
| `--baseline-info` | main.go:126 | 13 | N/A | No | Yes | Show baseline info |
| `--check-drift` | main.go:127 | 107 | Yes | No | Yes | Drift detection |
| `--recipe` / `-r` | main.go:110 | 13 | N/A | Yes | Yes | Named recipe filter |
| `--emit-script` | main.go:182 | 72 | Yes | No | Yes | Shell script generation |
| `--script-limit` | main.go:183 | 1 | N/A | No | Yes | Script item limit |
| `--script-format` | main.go:184 | 1 | N/A | No | Yes | bash/fish/zsh |
| `--feedback-accept` | main.go:186 | (shared) | Yes | No | Yes | Record accept feedback |
| `--feedback-ignore` | main.go:187 | (shared) | Yes | No | Yes | Record ignore feedback |
| `--feedback-reset` | main.go:188 | (shared) | Yes | No | Yes | Reset feedback |
| `--feedback-show` | main.go:189 | (shared) | Yes | No | Yes | Show feedback status |
| `--priority-brief` | main.go:191 | 28 | Yes | No | Yes | Export priority brief MD |
| `--agent-brief` | main.go:193 | 83 | Yes | No | Yes | Export agent brief bundle |
| `--export-pages` | main.go:195 | ~300 | Yes | No | Yes | Static site export |
| `--pages-title` | main.go:196 | 1 | N/A | No | Yes | Custom site title |
| `--pages-include-closed` | main.go:197 | 1 | N/A | No | Yes | Include closed issues |
| `--pages-include-history` | main.go:198 | 1 | N/A | No | Yes | Include git history |
| `--preview-pages` | main.go:199 | 6 | N/A | No | Yes | Preview static site |
| `--no-live-reload` | main.go:200 | 1 | N/A | No | Yes | Disable live-reload |
| `--watch-export` | main.go:201 | ~130 | Partial | No | Yes | Auto-regenerate on change |
| `--pages` | main.go:202 | 6 | Yes | No | Yes | Interactive wizard |
| `--export-graph` | main.go:97 | 80 | Yes | No | Yes | PNG/SVG/HTML graph export |
| `--graph-format` | main.go:93 | 1 | N/A | No | Yes | json/dot/mermaid |
| `--graph-root` | main.go:94 | 1 | N/A | No | Yes | Subgraph root ID |
| `--graph-depth` | main.go:95 | 1 | N/A | No | Yes | Max subgraph depth |
| `--graph-preset` | main.go:98 | 1 | N/A | No | Yes | compact or roomy |
| `--graph-title` | main.go:99 | 1 | N/A | No | Yes | Custom graph title |
| `--debug-render` | main.go:204 | 5 | N/A | No | Yes | Render view to stdout |
| `--debug-width` | main.go:205 | 1 | N/A | No | Yes | Debug render width |
| `--debug-height` | main.go:206 | 1 | N/A | No | Yes | Debug render height |
| `--background-mode` | main.go:208 | (shared) | N/A | No | Yes | Enable background snapshots |
| `--no-background-mode` | main.go:209 | (shared) | N/A | No | Yes | Disable background snapshots |
| `--agents-add` | main.go:211 | (shared) | N/A | No | Yes | Add AGENTS.md blurb |
| `--agents-remove` | main.go:212 | (shared) | N/A | No | Yes | Remove AGENTS.md blurb |
| `--agents-update` | main.go:213 | (shared) | N/A | No | Yes | Update AGENTS.md blurb |
| `--agents-check` | main.go:214 | (shared) | N/A | No | Yes | Check AGENTS.md status |
| `--agents-dry-run` | main.go:215 | (shared) | N/A | No | Yes | Dry run for agents-* |
| `--agents-force` | main.go:216 | (shared) | N/A | No | Yes | Skip confirmation |
| `--label` | main.go:106 | 26 | Yes | No | Yes | Scope analysis to label |
| `--severity` | main.go:107 | 1 | N/A | No | Yes | Filter alert severity |
| `--alert-type` | main.go:108 | 1 | N/A | No | Yes | Filter alert type |
| `--alert-label` | main.go:109 | 1 | N/A | No | Yes | Filter alerts by label |
| `--attention-limit` | main.go:80 | 1 | N/A | No | Yes | Limit attention labels |
| `--robot-min-confidence` | main.go:101 | 1 | N/A | No | Yes | Filter by min confidence |
| `--robot-max-results` | main.go:102 | 1 | N/A | No | Yes | Limit result count |
| `--robot-by-label` | main.go:103 | 1 | N/A | No | Yes | Filter by label |
| `--robot-by-assignee` | main.go:104 | 1 | N/A | No | Yes | Filter by assignee |
| `--schema-command` | main.go:85 | 1 | N/A | No | Yes | Schema for specific command |
| `--suggest-type` | main.go:88 | 1 | N/A | No | Yes | Filter suggestion type |
| `--suggest-confidence` | main.go:89 | 1 | N/A | No | Yes | Min suggestion confidence |
| `--suggest-bead` | main.go:90 | 1 | N/A | No | Yes | Filter for specific bead |
| `--bead-history` | main.go:130 | 1 | N/A | No | Yes | History for specific bead |
| `--history-since` | main.go:131 | 1 | N/A | No | Yes | Limit history date |
| `--history-limit` | main.go:132 | 1 | N/A | No | Yes | Max commits to analyze |
| `--min-confidence` | main.go:133 | 1 | N/A | No | Yes | Correlation confidence filter |
| `--correlation-by` | main.go:138 | 1 | N/A | No | Yes | Agent/user identifier |
| `--correlation-reason` | main.go:139 | 1 | N/A | No | Yes | Reason for feedback |
| `--orphans-min-score` | main.go:143 | 1 | N/A | No | Yes | Min suspicion score |
| `--file-beads-limit` | main.go:146 | 1 | N/A | No | Yes | Max closed beads shown |
| `--hotspots-limit` | main.go:148 | 1 | N/A | No | Yes | Max hotspots shown |
| `--relations-threshold` | main.go:153 | 1 | N/A | No | Yes | Min correlation threshold |
| `--relations-limit` | main.go:154 | 1 | N/A | No | Yes | Max related files |
| `--related-min-relevance` | main.go:157 | 1 | N/A | No | Yes | Min relevance score |
| `--related-max-results` | main.go:158 | 1 | N/A | No | Yes | Max results per category |
| `--related-include-closed` | main.go:159 | 1 | N/A | No | Yes | Include closed beads |
| `--network-depth` | main.go:164 | 1 | N/A | No | Yes | Subnetwork depth 1-3 |
| `--forecast-label` | main.go:172 | 1 | N/A | No | Yes | Filter forecast by label |
| `--forecast-sprint` | main.go:173 | 1 | N/A | No | Yes | Filter forecast by sprint |
| `--forecast-agents` | main.go:174 | 1 | N/A | No | Yes | Parallel agents count |
| `--agents` (capacity) | main.go:177 | 1 | N/A | No | Yes | Agents for capacity sim |
| `--capacity-label` | main.go:178 | 1 | N/A | No | Yes | Capacity label filter |

**Total flags defined: ~110**

### Environment Variables (Complete List)

| Variable | Location | Purpose |
|----------|----------|---------|
| `BT_ROBOT` | main.go:262 | Force robot mode (clean stdout) |
| `BT_OUTPUT_FORMAT` | main.go:7573 | Default output format: json or toon |
| `BT_INSIGHTS_MAP_LIMIT` | main.go:2625 | Cap metric map sizes in insights output |
| `BT_TUI_AUTOCLOSE_MS` | main.go:5015 | Auto-quit TUI after N ms (testing) |
| `BT_BUILD_HYBRID_WASM` | main.go:5888 | Trigger WASM build for static site |
| `BT_PRETTY_JSON` | main.go:7553 | Pretty-print JSON output |
| `BT_BACKGROUND_MODE` | main.go:4926 | Enable/disable background snapshot loading |
| `BT_SEARCH_MODE` | (flag help:114) | Search ranking mode: text or hybrid |
| `BT_SEARCH_PRESET` | (flag help:115) | Hybrid search preset name |
| `BT_EXPORT_PATH` | (robot-help:566) | Mentioned in hook env docs |
| `BT_EXPORT_FORMAT` | (robot-help:566) | Mentioned in hook env docs |
| `BT_ISSUE_COUNT` | (robot-help:567) | Mentioned in hook env docs |
| `BT_TIMESTAMP` | (robot-help:567) | Mentioned in hook env docs |
| `BT_WORKER_TRACE_FILE` | (comment:4978) | Debug logging for background worker |
| `TOON_DEFAULT_FORMAT` | main.go:7576 | Fallback format if BT_OUTPUT_FORMAT not set |
| `TOON_STATS` | main.go:316 | Show token estimates on stderr |
| `TOON_KEY_FOLDING` | main.go:7587 | TOON key folding mode |
| `TOON_INDENT` | main.go:7590 | TOON indentation level |
| `BEADS_DOLT_SERVER_PORT` | (via doltctl) | Dolt server port override |
| `BT_DOLT_PORT` | (via doltctl) | Dolt port fallback |

### Modes of Operation

| Mode | Activation | Description |
|------|-----------|-------------|
| **TUI** | No special flags (default) | Launches Bubble Tea interactive viewer |
| **Robot** | Any `--robot-*` flag, `BT_ROBOT=1`, or non-TTY stdout | Headless JSON/TOON output |
| **Workspace** | `--workspace CONFIG` | Multi-repo aggregation |
| **Time-travel** | `--as-of REF` | Historical issue state |
| **Export-MD** | `--export-md FILE` | Markdown export |
| **Export-Pages** | `--export-pages DIR` | Static HTML site export |
| **Watch** | `--watch-export` + `--export-pages` | Auto-regenerate on changes |
| **Preview** | `--preview-pages DIR` | Local HTTP server for previewing |
| **Wizard** | `--pages` | Interactive Pages deployment |
| **Profile** | `--profile-startup` | Startup timing analysis |
| **Search** | `--search QUERY` | Semantic vector search |
| **Agents** | `--agents-*` | AGENTS.md management |
| **Feedback** | `--feedback-*` | Recommendation feedback loop |
| **Baseline** | `--save-baseline` / `--check-drift` | Metrics baselining and drift detection |
| **Debug** | `--debug-render VIEW` | Render TUI view to stdout |
| **Update** | `--update` / `--rollback` | Self-update mechanism |

## Startup Flow

1. **Flag parsing** (lines 55-223): 110+ flags defined via pflag, `flag.Parse()` at line 223
2. **CPU profiling** (lines 226-238): Conditional pprof start
3. **Build tag guards** (lines 241-260): Blank identifier assignments to prevent unused-var errors when build tags strip features
4. **Robot mode detection** (lines 262-311): Checks ~35 conditions to determine if robot mode is active
5. **Output format resolution** (lines 314-320): Resolves JSON vs TOON from flag, `BT_OUTPUT_FORMAT`, `TOON_DEFAULT_FORMAT`
6. **Pre-data-load commands** (lines 322-1286): Help, version, update, rollback, agents-*, feedback-*, recipes, schema, robot-docs, baseline-info, recipe validation - these exit before loading issues
7. **Issue loading** (lines 1288-1396): Three paths - `--as-of` (git history), `--workspace` (multi-repo), or default (single repo via datasource). Dolt auto-start via `doltctl.EnsureServer()` on `ErrDoltRequired`
8. **Repo filter** (lines 1399-1401): `--repo` prefix filter applied
9. **Data hash** (line 1406): Stable hash computed for robot output consistency
10. **Label scope** (lines 1412-1437): `--label` subgraph extraction
11. **Recipe pre-filter** (lines 1442-1445): Early recipe application for robot commands
12. **Robot command dispatch** (lines 1448-4835): Sequential if-else chain processing each `--robot-*` command, each ending with `os.Exit(0)`
13. **Export commands** (lines 4854-4901): `--export-md` handler
14. **Recipe filter for TUI** (lines 4910-4913): Late recipe application for TUI mode
15. **Background mode** (lines 4918-4934): Experimental background snapshot config
16. **TUI launch** (lines 4937-4972): `ui.NewModel()`, optional Dolt server attachment, workspace mode, debug render, then `runTUIProgram()`

## Dependencies

- **Depends on** (18 internal packages):
  - `internal/datasource` - Issue loading from JSONL/SQLite/Dolt
  - `internal/doltctl` - Dolt server lifecycle (EnsureServer, StopIfOwned)
  - `pkg/agents` - AGENTS.md blurb management
  - `pkg/analysis` - Graph analysis, triage, insights, impact scores, suggestions, ETA, capacity
  - `pkg/baseline` - Baseline save/load/comparison
  - `pkg/correlation` - Git history correlation, orphan detection, file lookup, related work, causality
  - `pkg/drift` - Drift detection and alerting
  - `pkg/export` - Markdown, SQLite, graph, static pages, interactive HTML, priority brief export
  - `pkg/hooks` - Pre/post-export hook execution
  - `pkg/loader` - JSONL/git loading, beads dir discovery, gitignore management
  - `pkg/metrics` - Performance metrics collection
  - `pkg/model` - Issue, Sprint, Dependency data structures
  - `pkg/recipe` - Recipe loading, parsing, time utilities
  - `pkg/search` - Vector index, embedding, hybrid search
  - `pkg/ui` - Bubble Tea TUI model
  - `pkg/updater` - Self-update mechanism
  - `pkg/version` - Version string
  - `pkg/watcher` - File system watching
  - `pkg/workspace` - Multi-repo workspace loading

- **External dependencies** (6):
  - `github.com/spf13/pflag` - Flag parsing
  - `github.com/goccy/go-json` - Fast JSON encoding
  - `github.com/Dicklesworthstone/toon-go` - TOON format encoding
  - `golang.org/x/term` - TTY detection
  - `gopkg.in/yaml.v3` - YAML config parsing
  - `github.com/charmbracelet/bubbletea` - TUI framework

- **Depended on by**: Nothing (this is the top-level entry point)

## Dead Code Candidates

1. **`_ = exportPages` guard block** (lines 241-260): These blank assignments exist as "build tag guards" to prevent compiler errors when features are stripped. The comment says "Ensure static export flags are retained even when build tags strip features in some environments." However, there are no build tags in the codebase that would strip these features. All the guarded flags are used later in the same function. These 20 lines appear unnecessary.

2. **`medianMinutes` variable** (line 4592): Declared as `medianMinutes := 60` in the capacity handler, then suppressed at line 4749 with `_ = medianMinutes`. It was likely planned for use in capacity estimation but never wired in.

3. **Redundant issues reload in `--robot-file-relations` and `--robot-related`** (lines 3900, 3975): Both handlers call `datasource.LoadIssues(cwd)` to reload issues fresh from disk, even though issues were already loaded at lines 1288-1396. Similarly `--robot-blocker-chain` (line 4064), `--robot-impact-network` (line 4121), and `--robot-causality` (line 4201) all reload issues instead of using the already-loaded slice. This is either a bug (ignoring `--as-of`, `--workspace`, `--repo` filters) or unnecessary duplication.

4. **`RobotMeta` struct** (lines 7492-7498): Defined but never used anywhere. It was intended as optional metadata for robot outputs but no command populates it.

5. **TOON format**: The TOON output mode depends on an external `tru` binary being available at runtime. All tests that exercise TOON (`TestTOONOutputFormat`, `TestTOONRoundTrip`, `TestTOONTokenStats`, `TestTOONSchemaOutput`) skip when `tru` is not found. The `toonRobotEncoder.Encode()` method falls back to JSON if tru is unavailable. This is functional but the TOON dependency seems like a niche feature with marginal adoption potential.

6. **`EnsureBVInGitignore`** (lines 1351, 1394): The function name still uses `BV` (old project name), though it correctly manages `.bt/` in gitignore. The naming mismatch is cosmetic but notable for the rename audit.

## Notable Findings

### 1. main.go is a 8,143-line monolith
The single `main()` function runs from line 54 to line 4973 (~4,900 lines). This is an extreme god function. The dispatch logic is a linear chain of `if *flagName {` blocks with no routing table or command registry. Adding a new robot command means adding another ~50-100 line block in the middle of this chain. Extraction candidates:

- **Burndown calculation** (~200 LOC, lines 6878-7280): `BurndownOutput`, `calculateBurndownAt`, `generateDailyBurndown`, `generateIdealLine`, `computeSprintScopeChanges` and helpers - none of this is CLI-specific
- **Static pages export** (~900 LOC, lines 5804-6876): `copyViewerAssets`, `findViewerAssetsDir`, `maybeBuildHybridWasmAssets`, `copyFile`, `copyDir`, `runPreviewServer`, `runPagesWizard`, `resolvePagesSource`, `findBetterPagesSource`, `discoverBeadsDirs`, `countIssuesInBeadsDir`, `metadataPreferredSource`, `loadIssuesFromBeadsDir` - this is a complete subsystem
- **README generation** (~250 LOC, lines 6017-6279): `generateREADME`, `truncateTitle`, `escapeMarkdownTableCell` - export helper code
- **Profile reporting** (~200 LOC, lines 5474-5696): `runProfileStartup`, `printProfileReport`, `printMetricLine`, `printCyclesLine`, `formatDuration`, `getSizeTier`, `generateProfileRecommendations`
- **Recipe filtering/sorting** (~200 LOC, lines 5259-5472): `applyRecipeFilters`, `applyRecipeSort`, `naturalLess`
- **Robot encoding** (~120 LOC, lines 7478-7613): `RobotEnvelope`, `NewRobotEnvelope`, `robotEncoder`, `toonRobotEncoder`, `newRobotEncoder`, `newJSONRobotEncoder`, encoding options resolution
- **Robot docs** (~250 LOC, lines 7615-7867): `generateRobotDocs`
- **Robot schemas** (~270 LOC, lines 7869-8143): `RobotSchemas`, `generateRobotSchemas`
- **Diff summary** (~120 LOC, lines 5088-5218): `printDiffSummary`, `repeatChar`, `formatCycle`
- **JQ helpers** (~80 LOC, lines 7282-7363): `generateJQHelpers`
- **Time-travel history** (~100 LOC, lines 7366-7476): `TimeTravelHistory`, `generateHistoryForExport`

### 2. Issues reloaded from disk in 5 robot handlers
As noted in Dead Code, the `--robot-file-relations`, `--robot-related`, `--robot-blocker-chain`, `--robot-impact-network`, and `--robot-causality` handlers call `datasource.LoadIssues(cwd)` instead of using the already-loaded `issues` slice. This bypasses any `--as-of`, `--workspace`, `--repo`, `--label`, or `--recipe` filters that were applied. This is likely a bug: if a user runs `bt --robot-blocker-chain X --repo api`, the blocker chain will analyze all issues, not just the API-filtered ones.

### 3. Robot mode help is 460 lines of fmt.Println
The `--robot-help` handler (lines 329-793) prints help text using 460+ individual `fmt.Println` calls. This is the largest single block of inline documentation. It duplicates information that also exists in `--robot-docs` (which outputs the same info as structured JSON). This text is not tested for accuracy and will drift from actual command behavior.

### 4. Flag naming inconsistency
Some flags use `--robot-` prefix consistently, but modifiers do not: `--robot-min-confidence` vs `--min-confidence` (the latter is for correlation, the former for robot output filtering). `--agents` (capacity) vs `--forecast-agents` - both control parallel agent count but have different names. `--label` serves double duty (subgraph scoping for analysis AND graph export filtering).

### 5. Dolt lifecycle integration is clean
The Dolt auto-start path (lines 1355-1374) is well-structured: catches `ErrDoltRequired`, calls `doltctl.EnsureServer()`, retries the load, and cleans up on failure. The server state is passed to the TUI model for lifecycle management, and `m.Stop()` handles cleanup via defer.

### 6. TUI program initialization is solid
`runTUIProgram()` (lines 4975-5047) properly configures Bubble Tea with alt screen, mouse support, signal handling with graceful shutdown (two SIGINT = force kill), and an optional auto-close mechanism for automated tests.

### 7. TOON format is a custom serialization
TOON (Token-Optimized Object Notation) is a non-standard format from `github.com/Dicklesworthstone/toon-go` that claims 30-50% token savings for LLM consumption. It requires an external `tru` binary. The implementation includes fallback to JSON, stats output, and configurable encoding options. This is a novel dependency with unknown adoption.

### 8. Old "bv" naming artifacts
Multiple references to the old project name persist:
- `EnsureBVInGitignore` function name (lines 1351, 1394)
- Comment references: "bv-182", "bv-84", "bv-87", etc. throughout the file (107 occurrences of `bv-` in comments/strings)
- `--robot-recipes` output says sources include `'user' (~/.config/bv/recipes.yaml)` (line 595)
- Robot-help text says "br show" and "br update" in some emit-script sections

### 9. Build tag guard pattern is suspicious
Lines 241-260 assign 20 flag variables to blank identifier `_` with a comment about "build tags strip features." There are no build tags in the file and no conditional compilation. These lines appear to be cargo-culted from a template or an earlier codebase version where features could be conditionally compiled out. They serve no current purpose.

### 10. Test coverage is thin
4 test files cover 871 lines total, but the coverage is narrow:
- `main_test.go`: filterByRepo, 4 robot flags produce valid JSON (binary test), recipe filter/sort, formatCycle
- `main_robot_test.go`: Plan/priority metadata, TOON format, TOON round-trip, TOON stats, TOON schema
- `burndown_test.go`: calculateBurndownAt (2 scenarios)
- `profile_test.go`: formatDuration, getSizeTier, printProfileReport, buildMetricItems, printDiffSummary

Not tested at all: most robot commands, the full startup flow, Dolt lifecycle, workspace mode, time-travel, search, export-pages, wizard, agents-*, feedback-*, emit-script, all baseline/drift commands, debug-render, background mode. The binary-level test (`TestRobotFlagsOutputJSON`) only covers 4 of ~30 robot commands.

## Questions for Synthesis

1. **Should main.go be split?** This is the highest-leverage refactoring opportunity. The file is nearly 2x the size estimated in the audit brief (8.1k actual vs ~8.1k stated). A command registry or handler-per-file pattern would make the code maintainable. The question is: does the project plan to add more commands, or is this feature-complete?

2. **Are the 5 redundant `datasource.LoadIssues(cwd)` calls bugs?** They bypass all filters applied to the main `issues` slice. If intentional (wanting unfiltered data for correlation), they should be documented. If unintentional, they need to use the pre-loaded issues.

3. **Is TOON a strategic dependency or experimental?** It adds complexity (external binary, fallback logic, encoding options, stats) for a niche use case. Should it be kept, feature-flagged, or removed?

4. **Should `--robot-help` (460 lines of println) be replaced by `--robot-docs`?** The docs command already provides the same information as structured JSON. The help text is a maintenance burden that will drift from reality.

5. **What is the intended scope of the `--pages` wizard system?** The `resolvePagesSource` and `findBetterPagesSource` functions implement a heuristic search across the filesystem (including `/dp`, home directory, and up to 4 levels of directory tree walking) to find beads data. This is a surprising amount of magic for a CLI tool and may have security/performance implications.

6. **Cross-team question for Teams 4/5 (datasource/analysis)**: Several robot handlers duplicate issue counting, status tallying, and metric extraction that seem like they belong in the analysis or datasource packages. Is there an opportunity to consolidate these patterns?

7. **What is the BV->BT rename status in this file?** 107 `bv-` references remain in comments (mostly beads issue IDs like `bv-182`). Function name `EnsureBVInGitignore` and one user-facing string (`~/.config/bv/recipes.yaml`) still reference the old name. Are the comment IDs intentional (historical references) or should they be updated?
