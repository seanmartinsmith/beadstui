---
commit_hash: 5b3767830036f23ad425f1223edd62fb1982cf3b
---

## Cluster taxonomy (10 clusters)

### Cluster 1: CLI / Robot Surface
**Files:** `cmd/bt/*.go` (38 files / 11.6k LOC)
**Key entities:** `rootCmd`, `robotCmd`, `triageCmd`, `nextCmd`, `bqlCmd`, `agentsCmd`, `pagesCmd` …
**External deps:** spf13/cobra, spf13/pflag
**Risk areas:**
- Heavy `os.Exit(1)` use inside cobra `Run:` callbacks instead of cobra's RunE error path (see cli_agents.go × 27, root.go × 28). Inconsistent error semantics across subcommands.
- Robot mode JSON output is the contract for AI agents — schema regressions are silent unless `bt robot schema` test catches them.
- TUI launch path collides with subcommand routing — robot subcommands MUST never trigger TUI startup paths (would hang automation).

### Cluster 2: TUI (Bubble Tea v2)
**Files:** `pkg/ui/*.go` (58 files / 38.9k LOC) — by far the largest cluster
**Key entities:** model, update loop, view, panes, modals, tutorial, keybindings, context_help
**External deps:** charm.land/* v2, mattn/go-runewidth
**Risk areas:**
- Largest LOC concentration — model decomposition status is unclear from grep; long view functions = readability + perf risk
- Charm v2 migration shipped 2026-04-10 — recent enough that subtle API drift may exist
- Tutorial system has its own content (tutorial_content.go) and test that detects placeholder text — strong hygiene signal
- Watcher integration (pkg/watcher) feeds TUI live updates — verify race-free message passing

### Cluster 3: Graph Analysis
**Files:** `pkg/analysis/*.go` (29 files / 15.9k LOC)
**Key entities:** `Graph`, PageRank, betweenness (incl. `betweenness_approx.go`), HITS, eigenvector, cycles, triage_context, recommendations
**External deps:** gonum.org/v1/gonum/graph
**Risk areas:**
- Two-phase: Phase 1 sync (degree, topo, density) — instant. Phase 2 async (PageRank, betweenness, HITS, eigenvector, cycles) with **500ms timeout**.
- 14 concurrency primitives in graph.go alone — race surface
- Cache invariants: `pkg/analysis/cache.go` — must invalidate on data refresh
- Pathological inputs benched in `bench_pathological_test.go` — verify what counts as pathological
- `feedback.go` adjusts ranking weights — feedback loop integrity matters

### Cluster 4: Data Layer (multi-source)
**Files:** `internal/datasource/*.go` (9 files / 3.0k LOC) + `pkg/loader/*.go` (5 files / 1.3k LOC)
**Key entities:** `DataSource`, `LoadFromSource`, `DoltReader`, `GlobalDoltReader`, `SQLiteReader`, JSONL loader
**External deps:** go-sql-driver/mysql, modernc.org/sqlite
**Risk areas:**
- **CRITICAL** — SQLite reader (`internal/datasource/sqlite.go`) is wired through `LoadFromSource` switch (load.go:172) and called from `cmd/bt/pages.go:581`. Project memory + AGENTS.md state "no SQLite at runtime" — this contradicts code. Either dead code or stale docs.
- SQL string interpolation in `global_dolt.go` (lines 436, 455, 472, 490, 507, 526-527). Guarded by `backtickQuote()` and `escapeSQLString()` (lines 807-816). Db names come from auto-discovery (filesystem) — likely safe, but if attacker controls a db name they win.
- JSONL legacy path still active even though Dolt is system of record per upstream beads
- Test coverage thin (5 tests for 9 files) relative to other packages

### Cluster 5: Dolt Server Lifecycle
**Files:** `internal/doltctl/doltctl.go` (199 LOC), `cmd/bt/root.go:1009`
**Key entities:** `Start`, `Stop`, regex `bdStartOutputRe`
**External deps:** `bd` binary on PATH
**Risk areas:**
- Server start parses `bd dolt start` stdout via regex `"Dolt server started (PID XXXXX, port YYYYY)"` (doltctl.go:39) — silently breaks if upstream changes the message format
- Zombie process risk if bt crashes between Start and Stop
- Per CLAUDE.md: bd v0.59-0.61 removed persistent dolt daemon; each bd command auto-starts/stops with ephemeral ports. `bd dolt start` still exists for external tools (like bt) needing persistent servers. The `DoltStore.Close()` won't kill bt's server. Coordination is fragile.

### Cluster 6: Correlation (bead↔commit)
**Files:** `pkg/correlation/*.go` (20 files / 8.0k LOC)
**Key entities:** `Witness`, `Stream`, `Temporal`, `Cache`, `Feedback`, `Explicit`
**External deps:** git (subprocess), pkg/loader for rev resolution
**Risk areas:**
- Per project memory + bt-08sh: correlator is JSONL+git-diff witness, planned to migrate to `dolt_log` + `dolt_history_issues`. NOT a beads concept upstream — purely bt's domain.
- Git stdout parsing across 5 files — locale/version brittleness
- `feedback.go` and `cache.go` use mutexes — verify lock ordering, no nested locks
- Stream.go has 3 separate `exec.Command("git", ...)` invocations (lines 123, 160, 417) — composition pattern? sequential? parallel?

### Cluster 7: Search (hybrid hash+lexical)
**Files:** `pkg/search/*.go` (15 files / 1.6k LOC)
**Key entities:** `VectorIndex` (custom `.bvvi` binary format), lexical boost, search loops
**External deps:** none (zero ML deps — hash-based embeddings)
**Risk areas:**
- Custom binary index format (`.bvvi`) under `.bt/semantic/` — version compatibility, format migration
- "Hash-based semantic" embeddings — quality ceiling vs real embeddings; user expectations
- 1 sync primitive in vector_index.go — read/write contention?

### Cluster 8: Export (static site, SQLite snapshot, livereload)
**Files:** `pkg/export/*.go` (15 files / 8.7k LOC)
**Key entities:** `WriteStatic`, `SQLiteExport` (FTS5 + materialized views), `Preview` (livereload), `Wizard` (interactive)
**External deps:** modernc.org/sqlite, ajstarks/svgo
**Risk areas:**
- 8 sync primitives in preview.go — livereload server lifecycle; potential goroutine leak on cancel
- Local HTTP server bound by livereload — verify localhost-only, port discovery, no auth
- SQLite export with FTS5 — schema migrations on output format
- `pages.go:701` shells out to `wasm-pack` for WASM build — fallback if absent? error handling?

### Cluster 9: BQL Parser
**Files:** `pkg/bql/*.go` (8 files / 2.1k LOC)
**Key entities:** lexer, parser, evaluator, `LICENSE` (MIT, adapted from perles by Zach Rosen)
**External deps:** none
**Risk areas:**
- Trust boundary: BQL queries enter via CLI flags (`bt robot list --bql`, `bt robot bql --query`). User-controlled.
- Property tests via pgregory.net/rapid — verify coverage of query injection, escaping
- Adapted code — divergence from upstream perles is unmanaged

### Cluster 10: Watcher / Tail / Live state
**Files:** `pkg/watcher/*.go` (7 files / 734 LOC), `pkg/tail/*.go` (3 files / 575 LOC), `pkg/instance/lock.go` (3 files / 323 LOC)
**Key entities:** fsnotify watcher, debouncer, single-instance lock
**External deps:** fsnotify, golang.org/x/sys
**Risk areas:**
- Watcher race: file modify event vs reader open
- Debouncer correctness — last-write-wins vs first-write-wins
- Single-instance lock — what happens if process holding lock crashes? (stale lock handling)
- Tail mode is "Monitor-tool compatible" per CLAUDE.md — line-buffered? back-pressure?

## Cross-cluster invariants (worth checking)

1. Concurrency rule (per CLAUDE.md): "capture channels before unlock to avoid races" — sample 5 mutex+channel sites and verify
2. Browser safety (per CLAUDE.md): all browser-opening gated by `BT_NO_BROWSER` / `BT_TEST_MODE` — grep all `xdg-open|exec.Command.*open|start ` invocations
3. Error wrapping (per CLAUDE.md): `fmt.Errorf("context: %w", err)` always — sample
4. Division safety (per CLAUDE.md): guard against div-by-zero in averages/ratios — graph.go, analysis/* prime locations
5. Robot output stability: schema is contract for agents — must NOT regress silently. `cmd/bt/robot_schema_*` tests are the guardrail
6. AGENTS.md filename: hardcoded in 15 Go files per CLAUDE.md — verify count

## Risk hotspot map (composite)

| File | Composite risk | Reasons |
|---|---|---|
| internal/datasource/sqlite.go | HIGH — dead-code suspect | Memory says SQLite is export-only; reader is wired into LoadFromSource and called from pages.go |
| internal/datasource/global_dolt.go | MEDIUM — SQL surface | String interpolation guarded but trust boundary on dbName worth verifying |
| internal/doltctl/doltctl.go | MEDIUM — fragile coupling | Stdout regex parsing of bd output |
| pkg/analysis/graph.go | MEDIUM — concurrency | 14 primitives, two-phase timeout fall-through |
| pkg/correlation/stream.go | MEDIUM — git brittleness | 3 git shell-outs; stdout parsing |
| pkg/export/preview.go | MEDIUM — server lifecycle | 8 primitives, livereload bound port |
| cmd/bt/root.go | LOW-MEDIUM — error semantics | 28 os.Exit calls, mix with cobra error path |
| pkg/ui/* | LOW-MEDIUM — size-driven | Largest cluster; complexity by volume |

This composite map is a STARTING POINT for personas — not the answer. Personas must verify each item in source before claiming a finding.
