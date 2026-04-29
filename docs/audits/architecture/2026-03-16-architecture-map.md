# Architecture Map

**Generated**: Session 16 (codebase audit scan)
**Codebase**: ~88k production Go, ~102k test Go, ~7.5k Rust (WASM)

## High-Level Architecture

```
                              ┌─────────────────────┐
                              │     cmd/bt/main.go   │
                              │   (8.1k lines, CLI)  │
                              │  ~110 flags, ~30     │
                              │  robot commands      │
                              └──────────┬───────────┘
                                         │
                    ┌────────────────────┼────────────────────┐
                    │                    │                    │
                    ▼                    ▼                    ▼
           ┌──────────────┐   ┌──────────────────┐  ┌──────────────┐
           │  pkg/ui/     │   │  pkg/export/     │  │  Robot Mode  │
           │  TUI Layer   │   │  Export Pipeline │  │  (inline in  │
           │  (~58k lines)│   │  (~7.2k lines)   │  │   main.go)   │
           └──────┬───────┘   └────────┬─────────┘  └──────┬───────┘
                  │                    │                    │
                  └────────────────────┼────────────────────┘
                                       │
                    ┌──────────────────┼──────────────────┐
                    │                  │                  │
                    ▼                  ▼                  ▼
           ┌──────────────┐  ┌──────────────────┐  ┌──────────────┐
           │ pkg/analysis/│  │ pkg/correlation/ │  │ pkg/search/  │
           │ Graph Engine │  │ Git Correlation  │  │ Hybrid Search│
           │ (~7.8k lines)│  │ (~4.5k lines)    │  │ (~1.5k lines)│
           └──────┬───────┘  └────────┬─────────┘  └──────────────┘
                  │                   │
                  ▼                   ▼
           ┌──────────────────────────────────────────────┐
           │              pkg/model/ (327 lines)          │
           │   Issue, Status, Dependency, Comment, Sprint │
           │           (~140 importers)                   │
           └──────────────────────┬───────────────────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    │             │             │
                    ▼             ▼             ▼
           ┌──────────────┐ ┌──────────┐ ┌──────────────┐
           │  internal/   │ │ internal/│ │  pkg/loader/ │
           │  datasource/ │ │ doltctl/ │ │  JSONL Parse │
           │  (~3.6k)     │ │ (~200)   │ │  (~1.3k)     │
           └──────┬───────┘ └────┬─────┘ └──────────────┘
                  │              │
                  ▼              ▼
           ┌──────────────────────────┐
           │      Dolt Server         │
           │   (MySQL protocol)       │
           │  Started via bd dolt     │
           └──────────────────────────┘
```

## Data Flow

### Startup Flow
```
main.go: parse flags -> detect robot mode
    │
    ├─ Robot mode: load issues -> run command -> JSON output -> exit
    │
    └─ TUI mode:
         doltctl.EnsureServer() -> detect/start Dolt
         datasource.LoadIssues() -> discover sources -> validate -> load
         ui.NewModel(issues) -> init Bubble Tea program
```

### Runtime Data Flow (TUI)
```
Dolt Poll Loop (5s interval)
    │
    ▼
BackgroundWorker.process()
    │
    ├─ datasource.LoadFromSource()  -- reload issues from Dolt
    ├─ SnapshotBuilder.Build()      -- sort, analyze, build views
    │     ├─ Phase 1 (sync): degree, topo sort, density
    │     └─ Phase 2 (async): PageRank, betweenness, HITS, cycles, k-core
    │
    ▼
SnapshotReadyMsg -> Model.Update()
    │
    ├─ Atomic snapshot swap
    ├─ Rebuild list items, board state, graph layout
    └─ Preserve selection, update footer badges
```

## Domain Summaries

### cmd/bt/ (Team 2) - 9.2k lines
The CLI entry point. A single 8.1k-line main.go with ~110 flags and ~30 robot commands. Dispatches to TUI mode or robot mode. Contains substantial helper code (~2.5k lines) that should be extracted: burndown calculation, static pages export, README generation, profile reporting.

**Key issue**: 5 robot handlers reload issues from disk instead of using the pre-loaded slice, bypassing filters.

### pkg/ui/ (Teams 1a + 1b) - ~58k lines
The TUI layer, split between views/rendering (~33k) and core/state (~28k). Follows Bubble Tea MVU with a monolithic Model (~90 fields, 8.3k lines). 9 major views (board, tree, graph, insights, history, flow matrix, tutorial, label dashboard, velocity comparison), 6 modals, 70+ keyboard shortcuts.

**Key asset**: Three-layer YAML theme system, immutable DataSnapshot architecture, comprehensive tutorial (30 pages).
**Key concern**: model.go at 8.3k lines should be split into ~10 files. Five mutually exclusive view booleans should become a ViewMode enum.

### pkg/analysis/ (Team 3) - 7.8k lines
Graph analysis engine operating entirely on in-memory `[]model.Issue`. Two-phase computation (fast metrics sync, expensive metrics async). 35+ features including triage scoring, execution planning, what-if analysis, duplicate detection, label health. Fully Dolt-compatible by design.

**Key asset**: Compact custom graph implementation replacing gonum's allocating maps. Deterministic output for robot consumption.
**Key concern**: label_health.go at ~2k lines (25% of package) may be over-engineered for current project size.

### pkg/export/ (Team 4) - 7.2k lines
Full export-to-deploy pipeline: Markdown, SQLite+FTS5, JSON, DOT, Mermaid, interactive HTML, GitHub Pages, Cloudflare Pages deployment. graph_render_beautiful.go is 1.8k lines with ~1.2k of inline HTML/CSS/JS.

**Key asset**: SQLite export is production-quality with FTS5, materialized views, robot JSON outputs.
**Key concern**: Deploy pipelines (GitHub Pages, Cloudflare) may be dead weight for a personal TUI tool.

### internal/datasource/ + internal/doltctl/ (Team 5) - 4.5k lines
Multi-source discovery/validation/loading pipeline (Dolt, SQLite, JSONL) plus Dolt server lifecycle management. Smart fallback chain with `ErrDoltRequired` for Dolt-mandatory projects.

**Key asset**: Port discovery chain is solid and well-tested. doltctl PID-based ownership is clean.
**Key concern**: 624 LOC confirmed dead (watch.go, diff.go). SQLite reader (358 LOC) is legacy. N+1 query pattern in Dolt reader.

### pkg/agents/ (Team 6) - 2.9k lines (800 source, 2k test)
AGENTS.md/CLAUDE.md blurb injection utility. Pure file management, no framework dependencies. tty_guard.go sets CI=1 globally to suppress Termenv probes in robot mode.

**Key asset**: 2.6:1 test-to-source ratio. Thorough edge case coverage.
**Key concern**: HTML markers still use `bv-` prefix (wire format issue). tty_guard init() is a global side effect from a domain-specific package.

### pkg/correlation/ (Team 8a) - ~4.5k lines
Sophisticated git history correlation engine linking commits to issues. Multiple strategies: co-commit, explicit ID matching, temporal, orphan detection. Includes file index, impact network, causal chains.

**Critical question**: This operates on git diffs of JSONL files. If upstream beads no longer commits JSONL to git (Dolt-only), the entire engine's data source may disappear.

### Supporting packages (Team 8a) - ~4.7k lines (remaining)
16 packages providing infrastructure: hooks, recipes, search, metrics, drift, baseline, instance locking, file watching, workspace, updater, debug logging, top-K collector.

**Key asset**: pkg/model is the gravity well (~140 importers). pkg/loader has production-quality JSONL parsing. pkg/updater has security-aware token handling.
**Key concern**: Stale `bv-` patterns in orphan.go and cass/correlation.go. Instance lock file still named `.bv.lock`.

### Build & WASM (Team 8b) - ~10.2k lines
GoReleaser pipeline, CI (build+test+coverage+benchmarks+fuzz), Nix flake, install scripts. Two Rust WASM modules: bv-graph-wasm (7.5k LOC, graph algorithms) and wasm_scorer (169 LOC, hybrid scoring).

**Key concern**: Root Makefile is broken (references `bv`/`cmd/bv`). bv-graph-wasm is not built by CI and algorithms are reimplemented in Go. flake.nix has stale version "0.14.4". Install scripts require Go 1.21 but go.mod needs 1.25.

## Cross-Domain Dependency Graph

```
pkg/model  ──────────────────────────── ~140 importers (everything)
    │
    ├── pkg/analysis ───────────────── pkg/ui, pkg/export, pkg/drift, pkg/search
    ├── pkg/correlation ────────────── pkg/ui (history), pkg/export, cmd/bt
    ├── pkg/loader ─────────────────── internal/datasource, pkg/workspace, cmd/bt
    ├── internal/datasource ────────── cmd/bt, pkg/ui
    ├── internal/doltctl ───────────── cmd/bt, pkg/ui
    ├── pkg/cass ───────────────────── pkg/ui
    ├── pkg/recipe ─────────────────── pkg/ui, cmd/bt
    ├── pkg/search ─────────────────── pkg/ui, cmd/bt
    ├── pkg/agents ─────────────────── pkg/ui, cmd/bt
    ├── pkg/export ─────────────────── cmd/bt, pkg/ui
    ├── pkg/hooks ──────────────────── cmd/bt
    ├── pkg/drift ──────────────────── cmd/bt, pkg/ui
    ├── pkg/baseline ───────────────── cmd/bt, pkg/drift
    ├── pkg/updater ────────────────── pkg/ui, cmd/bt
    ├── pkg/instance ───────────────── pkg/ui
    ├── pkg/watcher ────────────────── cmd/bt, pkg/ui
    ├── pkg/workspace ──────────────── cmd/bt
    ├── pkg/metrics ────────────────── cmd/bt
    ├── pkg/debug ──────────────────── pkg/ui
    └── pkg/version ────────────────── cmd/bt, pkg/ui, pkg/updater
```

## Cross-Cutting Findings

### Stale `bv` Naming (across all teams)
| Location | Issue |
|----------|-------|
| tutorial_content.go | CLI references say `bv` in tutorial text |
| pkg/correlation/orphan.go | Regex patterns match `bv-` not `bt-` |
| pkg/cass/correlation.go | Bead ID regex uses `bv-` prefix |
| pkg/instance/lock.go | Lock file named `.bv.lock` |
| pkg/loader/gitignore.go | Function named `EnsureBVInGitignore` |
| pkg/export/ (multiple) | Package docs, HTML output, localStorage keys |
| cmd/bt/main.go | `EnsureBVInGitignore` function name |
| AGENTS.md markers | `<!-- bv-agent-instructions-v1 -->` (wire format) |
| model.go | "Quit bv?" in quit confirmation |
| Makefile | Builds `bv` from `cmd/bv` |
| E2E tests (42 files) | 327 `bv` references in variable/function names |
| bv-graph-wasm/ | Directory name, Cargo.toml, all Rust code |

### Dead Code Summary
| Domain | LOC | Items |
|--------|-----|-------|
| datasource (watch.go, diff.go) | 624 | File watcher + source diff (never called) |
| datasource (sqlite.go) | 358 | Legacy SQLite reader (upstream removed) |
| bv-graph-wasm/ | 7,493 | Rust WASM crate (pre-compiled artifacts used instead) |
| analysis (deprecated functions) | ~200 | computeCounts, buildBlockersToClear, standalone wrappers |
| export (stubs/duplicates) | ~150 | GeneratePriorityBrief stub, duplicate Mermaid, duplicate truncation |
| ui (legacy sync path) | ~420 | FileChangedMsg handler for JSONL sources |
| Makefile | 21 | Broken, references `bv` |
| **Total estimated** | **~9,300** | |

### Test Coverage Gaps
1. **No Dolt integration tests** - the production data path has zero end-to-end coverage
2. **TUI tests skip on Windows** - primary dev platform has no TUI rendering tests
3. **E2E tests use JSONL fixtures exclusively** - never exercise the Dolt path
4. **TOON tests permanently skipped** - `tru` binary never present
5. **cmd/bt test coverage thin** - only 4 of ~30 robot commands tested at binary level

### Performance Patterns
- Two-phase analysis: fast sync metrics, expensive async metrics
- Immutable DataSnapshot with atomic swap (no locks on read path)
- Tiered computation: small/medium/large/huge issue sets
- Incremental graph stats cache (5min TTL, 8 max entries)
- Custom compact graph (adjacency lists vs gonum's map-backed sets)
- Dolt poll with exponential backoff (5s base, 2min cap)

## Ready for Session B

Session B (synthesis) should walk through each team's feature inventory with the user to assign verdicts (KEEP/CUT/IMPROVE/INVESTIGATE) and build ADR-002.

Key decisions for Session B:
1. How much dead code to cut immediately vs defer?
2. Is pkg/correlation viable with Dolt-only upstream?
3. Should deploy pipelines (GitHub Pages, Cloudflare) be removed?
4. What's the plan for the 8.1k-line main.go and 8.3k-line model.go?
5. Is the TOON format worth keeping?
6. Priority of stale `bv` naming cleanup?
