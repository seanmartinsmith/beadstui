---
commit_hash: 5b3767830036f23ad425f1223edd62fb1982cf3b
analyzed_at: 2026-05-03T23:21:28Z
scope: cmd/**, pkg/**, internal/**
files_analyzed: 248 (non-test) + 266 (test)
total_loc: 103413
---

## Module

`github.com/seanmartinsmith/beadstui` (Go 1.25.8) — beadstui aka `bt`, a TUI/CLI dashboard over the Dolt-backed `beads` issue tracker. Fork of Dicklesworthstone/beads_viewer retargeted to upstream beads. Charm Bracelet v2 stack (bubbletea/v2, lipgloss/v2, bubbles/v2, glamour/v2, huh/v2).

## Top-level package map

| Package | Files (non-test/test) | LOC | Role |
|---|---|---|---|
| cmd/bt | 38 / 15 | 11,610 | Cobra entry, robot subcommands, CLI plumbing |
| pkg/ui | 58 / 70 | 38,908 | Bubble Tea v2 TUI: model, update, views, panes, modals |
| pkg/analysis | 29 / 47 | 15,912 | Graph metrics: PageRank, betweenness, HITS, eigenvector, cycles, triage |
| pkg/correlation | 20 / 19 | 8,022 | Bead↔commit correlation, git temporal walk |
| pkg/export | 15 / 26 | 8,705 | Static site export, SQLite snapshot artifact, markdown, livereload preview |
| pkg/search | 15 / 19 | 1,627 | Custom hash-based vector index `.bvvi`, lexical boost |
| internal/datasource | 9 / 5 | 2,982 | Multi-source loader: Dolt, DoltGlobal, SQLite, JSONL |
| pkg/bql | 8 / 4 | 2,080 | BQL parser/evaluator (adapted from perles, MIT) |
| pkg/loader | 5 / 11 | 1,271 | JSONL parser, git-rev resolution |
| pkg/cass | 5 / 5 | 1,438 | CASS adapter (sibling tool integration) |
| pkg/agents | 5 / 6 | 912 | AGENTS.md detection/edit (filename hardcoded) |
| pkg/view | 5 / 6 | 1,494 | Output projections: CompactIssue, robot shapes |
| pkg/watcher | 7 / 2 | 734 | fsnotify-driven Dolt change watcher |
| pkg/drift | 3 / 2 | 1,369 | Schema/baseline drift detection |
| pkg/tail | 3 / 1 | 575 | Live event stream |
| pkg/instance | 3 / 1 | 323 | Single-instance lockfile |
| pkg/workspace | 2 / 2 | 646 | Workspace discovery |
| pkg/hooks | 2 / 7 | 487 | Pre/post hook execution |
| pkg/recipe | 2 / 2 | 349 | Recipe definitions (actionable, high-impact) |
| pkg/metrics | 2 / 1 | 420 | Performance metrics |
| pkg/model | 2 / 1 | 456 | Issue, Dependency, Comment domain types |
| pkg/baseline | 1 / 1 | 273 | Git baseline snapshots |
| pkg/debug | 1 / 1 | 169 | Debug helpers |
| pkg/util | 1 / 1 | 191 | Shared utilities |
| pkg/version | 1 / 1 | 64 | Version constants |
| pkg/updater | 1 / 6 | 752 | Self-update |
| internal/doltctl | 1 / 1 | 199 | Dolt server lifecycle (`bd dolt start/stop`) |
| internal/settings | 1 / 1 | 147 | Persisted settings |

## Routes / cobra commands (cmd/bt)

Top-level commands (from cobra.Command Use: declarations):

| Use | File | Purpose |
|---|---|---|
| `bt` (root) | root.go | Launches TUI by default; subcommand routing |
| `bt agents [check\|add\|remove\|update]` | cobra_agents.go | AGENTS.md workflow blurb management |
| `bt baseline` | cobra_baseline.go, cli_baseline.go | Snapshot baseline state |
| `bt brief [priority\|agent]` | cobra_misc.go | Export briefs |
| `bt feedback [accept\|ignore\|reset\|show]` | cobra_misc.go | Recommendation feedback |
| `bt emit-script` | cobra_misc.go | Shell script for top-N recs |
| `bt export` | cobra_export.go | Static site export |
| `bt pages` | cobra_misc.go, pages.go | Interactive Pages deployment wizard |
| `bt update` | cobra_update.go, cli_update.go | Self-update |
| `bt version` | cobra_version.go | Version info |
| `bt tail` | cobra_tail.go | Live event stream |
| `bt reset-terminal` | cobra_reset_terminal.go | Recovery from corrupt terminal state |
| `bt robot` | cobra_robot.go | Machine-readable parent (~30+ subcommands) |

`bt robot` subcommands (machine-readable, JSON/TOON output):

| Subcommand | Purpose |
|---|---|
| triage | Mega-command: ranked recs + quick wins + blockers + health |
| next | Top pick + claim command |
| insights | Graph analysis output |
| plan | Dependency-respecting parallel execution plan |
| priority | Priority misalignment detection |
| graph | DOT/Mermaid/JSON graph |
| alerts | Drift + proactive alerts |
| search | Semantic search results |
| bql | BQL-filtered issues |
| history | Bead↔commit correlations |
| suggest | Hygiene: duplicates, missing deps |
| diff | Changes since git ref |
| metrics | Performance metrics |
| recipes | Available recipes |
| schema | JSON Schema definitions for all robot outputs |
| docs | Machine-readable agent docs |
| help | Agent help |
| drift | Drift checks |
| sprint [list\|show] | Sprint commands (NOTE: sprints are NOT a beads concept; bt-only) |
| labels [health] | Label analysis |
| forecast | ETA predictions |
| portfolio | Cross-project portfolio view |
| pairs | Cross-project paired beads |
| refs | Bead reference resolution |
| files | File-level analysis |
| correlation | Bead↔commit detail |
| baseline | Baseline state |
| ctx | Context bundle for agents |
| list | List issues with filters |
| analysis | Detailed analysis |

## Data sources

Defined in `internal/datasource/source.go`:

| Type constant | Reader | File | Used at runtime |
|---|---|---|---|
| `SourceTypeDolt` | `*DoltReader` | dolt.go | yes (primary) |
| `SourceTypeDoltGlobal` | `*GlobalDoltReader` | global_dolt.go | yes (cross-DB AS OF queries) |
| `SourceTypeSQLite` | `*SQLiteReader` | sqlite.go | yes — called from load.go:173 + pages.go:581 |
| `SourceTypeJSONLLocal` / `SourceTypeJSONLWorktree` | `loader.LoadIssuesFromFile` | (pkg/loader/loader.go) | yes |

**NOTE for personas**: project memory says "There is no SQLite at runtime" (only export artifact). This is contradicted by `internal/datasource/sqlite.go` which is wired through `LoadFromSource` switch. Either the memory is stale OR these paths are dead code that should be removed. Worth a finding.

## Domain model

`pkg/model/issue.go` — `Issue` is the central type. `pkg/model/compact.go` — derived shape for compact output.

`pkg/view/` — output projections layered on top of model: `CompactIssue` (bt-mhwy.1, est. 2026-04-20) is the canonical projection produced by graph analysis pipelines for robot output shapes.

## External executables shelled out

Always invoked via `exec.Command` or `exec.CommandContext`:

| External | Locations | Purpose |
|---|---|---|
| `bd` | cmd/bt/root.go:1009 (start dolt), internal/doltctl/doltctl.go:103, 180 (start/stop dolt) | Dolt lifecycle |
| `git` | pkg/loader/git.go (rev-parse, show, log, cat-file ×7), pkg/correlation/temporal.go:66, 143, pkg/correlation/stream.go:123, 160, 417, pkg/baseline/baseline.go:190, internal/datasource/source.go:289, cmd/bt/burndown.go:95, cmd/bt/main_test.go:65 | Git history walk, commit metadata |
| `wasm-pack` | cmd/bt/pages.go:701 | WASM build for Pages export |

`exec.Command` is also used in many `_test.go` to invoke the bt binary itself for E2E coverage.

## SQL surface

All in `internal/datasource/`. Two query styles:

1. **Parameter-bound** (safe): `db.Query(query, issueID)` — sqlite.go:286, dolt.go:352, etc.
2. **String-interpolated** (guarded): global_dolt.go uses `fmt.Sprintf` with database/schema names interpolated. Guarded by:
   - `backtickQuote()` (global_dolt.go:807) — replaces `` ` `` with `` `` ``
   - `escapeSQLString()` (global_dolt.go:814) — replaces `'` with `''`

Risk lines (read this list as candidates, not confirmed bugs):

- global_dolt.go:436, 455, 472, 490, 507, 526, 527 — string interpolation of `dbName` into SQL
- global_dolt.go:541, 717, 748, 783 — execution of the resulting query

Database names come from auto-discovery via `bd dolt` mechanisms — likely from filesystem, not user input. Personas should evaluate whether the threat surface is realistic.

## Concurrency primitives

125 occurrences of `sync.Mutex|sync.RWMutex|sync.WaitGroup|sync.Once|go func|chan ...|make(chan ...)` across 40 files. Heaviest:

| File | Count | Notes |
|---|---|---|
| pkg/analysis/graph.go | 14 | Two-phase analysis: Phase 1 sync, Phase 2 async with 500ms timeout |
| pkg/export/preview.go | 8 | Livereload preview server |
| pkg/cass/safety_test.go | 8 | Concurrency tests for cass adapter |
| pkg/analysis/bench_pathological_test.go | 6 | Benchmark scaffolding |
| pkg/cass/cache_test.go | 6 | Cache concurrency |
| pkg/wizard.go | 6 | Pages wizard |
| pkg/export/livereload.go | 5 | Server lifecycle |
| pkg/analysis/cache.go | 3 | Phase 2 result caching |
| pkg/correlation/cache.go | 2 | Correlation result caching |
| pkg/correlation/stream.go | 1 | Streaming git walk |

Project rule (CLAUDE.md): "sync.RWMutex for shared state; capture channels before unlock to avoid races." Personas should verify this is followed.

## Error / exit handling

Project rule: errors via `fmt.Errorf("context: %w", err)`, no raw prints.

`os.Exit(1)` appears extensively in cmd/bt/* (cli_agents.go × 27, root.go × 28, robot_triage.go × 8, robot_sprint.go, etc.). Most are inside `Run:` callbacks where cobra's RunE error path could be used instead — pattern audit candidate.

`panic(...)` appears only in vendor and never in first-party code (good).

`log.Fatal` appears only in vendor.

## TODO / FIXME hygiene

First-party code is essentially TODO-clean. Only matches in pkg/ are placeholder-detection tests (pkg/ui/context_help_test.go:243, pkg/ui/tutorial_test.go:933) and tutorial copy that mentions "TODO list" as user-facing example. Strong hygiene signal.

## Test coverage shape

266 test files vs 248 non-test files (1.07× ratio). Heavy test investment in pkg/ui (70 tests for 58 files) and pkg/analysis (47 tests for 29 files). Lighter in internal/datasource (5 tests for 9 files — a soft spot given SQL surface).

## Charm v2 migration status

Migration to Charm Bracelet v2 shipped 2026-04-10 via beads bt-ykqq / bt-k5zs / bt-zt9q (per project memory + AGENTS.md). All Bubble Tea code paths now on `charm.land/bubbletea/v2`.

## ADR spine

`docs/adr/002-stabilize-and-ship.md` is the active spine document. Personas should NOT speculate about strategic direction — defer to ADR-002 for what's in/out of scope.
