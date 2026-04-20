# Changelog

Development log for beadstui. Each entry covers one Claude Code session's work, keyed by date.

For architectural decisions, see `docs/adr/`. For issue tracking, use `bd list`.

---

## 2026-04-20 - Portfolio subcommand (bt-mhwy.4)

**New `bt robot portfolio` subcommand answers "which project needs attention?" at the org level.** One PortfolioRecord per project with counts, priority breakdown, velocity with trend, composite health score, top blocker, and stalest issue.

### What shipped

- **`pkg/view/portfolio_record.go`** — new projection (`portfolio.v1`). `ComputePortfolioRecord(project, projectIssues, allIssues, pagerank, now)` is a pure function; `allIssues` lets the Blocked count see cross-project blockers under `--global` after bt-mhwy.5 external dep resolution.

- **Shared reverse-map helpers** — extracted `buildChildrenMap`, `buildUnblocksMap`, `buildOpenBlockersMap` from `CompactAll`'s single-pass loop. Both `CompactAll` and `ComputePortfolioRecord` consume them; behavior-identical refactor (CompactIssue golden fixtures unchanged).

- **`cmd/bt/robot_portfolio.go`** — `rc.runPortfolio()` handler. Groups issues by `SourceRepo` under `--global`; single-project mode emits exactly one record keyed by `rc.repoName` (falls back to a uniform SourceRepo, then `"local"`). Empty SourceRepo in global mode buckets to `"unknown"` so agents never lose data.

- **Cobra wiring** in `cmd/bt/cobra_robot.go` — `robotPortfolioCmd` registered alongside other robot subcommands. No new flags. `--shape` is inherited but no-op (envelope.schema is unconditionally `portfolio.v1` because the payload IS a versioned projection).

### Design

- **Health formula**: equal-weight mean of `closure_ratio`, `(1 − blocker_ratio)`, `(1 − stale_norm)` with clamping to `[0,1]` and 3-decimal rounding. Simple, explainable, no magic weights.
- **Trend classifier**: recent 2-week window vs prior 4-week window normalized to 2-week-equivalent, with ±20% thresholds — smoother than raw week-over-week.
- **Top blocker**: PageRank among project-scoped open/in_progress issues with `unblocks_count > 0` — excludes isolated leaves with high PageRank that aren't holding anyone hostage.

Full rationale in `docs/design/2026-04-20-bt-mhwy-4-portfolio.md`.

### Tests

- `pkg/view/portfolio_record_test.go` — unit tests for empty project, counts, trend classifier with boundary cases (±20%), health-score formula, top-blocker isolated-leaf filter, stalest selection.
- `pkg/view/projections_test.go` — 4 new golden fixtures exercised via `TestPortfolioRecordGolden`: empty, single healthy, single unhealthy, multi-project (cross-project blocker).
- `cmd/bt/robot_portfolio_test.go` — contract: envelope shape, `schema == "portfolio.v1"` across all `--shape` variants, `--shape=compact` ≡ `--shape=full` byte-identical (no-op), single-project mode returns exactly one record, projects sorted by name.
- `cmd/bt/robot_all_subcommands_test.go` — portfolio added to the flag-acceptance matrix (4 permutations).

### Smoke

`bt robot portfolio --global` ranks 15 real projects side-by-side with sensible health scores (0.464–0.985), per-project trends, and cross-project TopBlocker detection.

---

## 2026-04-20 - Compact output for robot subcommands (bt-mhwy.1)

**Default `bt robot list` output shape changes from full issues to compact projections.** 3 commits, 1 new package (`pkg/view/`), 1 bellwether integration, 1 compact projection for `robot diff`, 70+ new tests.

### Breaking change (pre-alpha)

- **Default `bt robot list` shape is now `compact`.** Full-body output is opt-in via `--full` (or `--shape=full`, or `BT_OUTPUT_SHAPE=full`). Rationale: `bt robot list --global` dropped from 383KB to 38KB on a 100-issue sample (~90% reduction) — agents were burning context windows on `description`/`design`/`acceptance_criteria`/`notes`/`comments`/`close_reason` bodies they never read.

### What shipped

- **`pkg/view/` package** (Commit 1) - Home for graph-derived consumer-facing projections. `CompactIssue` is the first resident. Ships with a reusable golden-file harness (`projections_test.go`), a committed JSON Schema (`schemas/compact_issue.v1.json`), and projection-pattern conventions in `doc.go`. Future projections (portfolio records, pair records, reference records) follow the same file-per-projection, schema-versioned pattern.

- **`robot list` bellwether** (Commit 2) - Persistent `--shape` / `--compact` / `--full` flags on `robotCmd` (inherited by every subcommand) with `BT_OUTPUT_SHAPE` env var. New `schema` field on `RobotEnvelope` (`omitempty`) carries `"compact.v1"` in compact mode and is absent in full mode, keeping `--full` byte-identical to pre-change output. Compact projection computed over the full pre-filter issue set so reverse-graph counts (`children_count`, `unblocks_count`, `is_blocked`) reflect the real graph regardless of `--status` / `--priority` / `--type` / `--has-label` / `--limit`.

- **`robot diff` compact projection** (Commit 3) - Projects the four `[]model.Issue` slots on `analysis.SnapshotDiff` (`new_issues`, `closed_issues`, `removed_issues`, `reopened_issues`) into `[]view.CompactIssue` when `shape=compact`. Reverse-graph counts computed over the UNION of historical and current issues so `children_count` / `unblocks_count` / `is_blocked` stay accurate across snapshots. `--full` keeps the original `*analysis.SnapshotDiff` wire shape.

- **15 other robot subcommands** - `triage`, `next`, `insights`, `plan`, `priority`, `alerts`, `search`, `suggest`, `drift`, `blocker-chain`, `impact-network`, `causality`, `related`, `impact`, `orphans` all inherit the persistent `--shape` flag and accept it without flag-parse errors. These subcommands' outputs use purpose-built wrapper types (`Recommendation`, `TopPick`, `PlanItem`, `EnhancedPriorityRecommendation`, `BlockerChainEntry`, `NetworkNode`, `RelatedWorkBead`, `AffectedBead`, `CausalChain`, `OrphanCandidate`) that are already compact-by-construction and emit no fat body fields, so no per-subcommand projection was needed.

### Flag resolution order

1. `--shape=compact` / `--shape=full` (explicit)
2. `--compact` / `--full` (alias; errors if combined with conflicting `--shape`)
3. `BT_OUTPUT_SHAPE` env var
4. `compact` default

### Tests

- `pkg/view/compact_issue_test.go` — unit (7 cases): nil/empty safety, field copying, labels aliasing, reverse-map correctness, `is_blocked` semantics across open/closed/in-progress/external blockers, `relates_count` local-only, metadata bridge, schema-constant check.
- `pkg/view/projections_test.go` — golden-file harness exercising 5 fixtures (minimal, fully-populated, blocked, epic-with-children, global-multiproject). Regenerate with `GENERATE_GOLDEN=1`.
- `cmd/bt/robot_compact_flag_test.go` — 14 flag-resolution cases (defaults, explicit, aliases, env, conflicts, bad values).
- `cmd/bt/robot_list_compact_test.go` — contract suite: no forbidden body fields leak, `--full` restores bodies, all flag/env permutations resolve consistently, reverse-graph counts (`is_blocked`, `parent_id`, `blockers_count`, `relates_count`), `--full` key regression.
- `cmd/bt/robot_all_subcommands_test.go` — 64 subtests across 16 subcommands × 4 flag permutations verifying flag acceptance, plus compact/full contract tests for `robot diff`.

### Blocks / unblocks

- **Unblocks**: bt-mhwy.2 (pairs), bt-mhwy.3 (refs), bt-mhwy.4 (portfolio), bt-mhwy.5 (external dep resolution), bt-mhwy.6 (provenance surfacing).
- **Prerequisites** (both landed earlier this session): bt-uc6k (schema-drift audit), bt-mhwy.0 (column catchup for `metadata` + `closed_by_session`).

---

## 2026-04-14 - Quick wins, footer extraction, label picker redesign

**Bug fixes + footer decomposition + label picker UX overhaul.** 17 commits, 4 bugs fixed, 1 refactor, 12 new tests.

### Bug fixes

- **Label picker freeze** (bt-eorx, P1) - Label picker lacked the early-return pattern used by other modals. Typed characters (g, i, a) were intercepted by global handlers that triggered expensive operations on 2500+ issues. Fix: added early return for ModalLabelPicker before global key handlers.

- **Status bar message not displaying** (bt-6k0f, P2) - `handleKeyPress` cleared `statusMsg` on every keypress but did not reset `statusSetAt`. New messages set via direct assignment had a stale timestamp from a previous message, causing `renderFooter`'s auto-dismiss to clear them before they rendered. Fix: reset `statusSetAt` in the clear-on-keypress block. Also migrated y-key copy handlers to use `setStatus()`/`setStatusError()`.

- **Label dashboard leaves split view disabled** (bt-trqo, P2) - Global `esc` handler for ViewLabelDashboard set `mode=ViewList` but forgot to restore `isSplitView=true`. Global `q` handler had no ViewLabelDashboard check at all (fell through to `tea.Quit`). Fix: added `isSplitView=true` to both global handlers.

### Refactor

- **Footer extraction** (bt-oim6, P2) - Extracted 650-line `renderFooter()` into `FooterData` value struct + `Render()` method. `Model.footerData()` extracts ~35 Model fields into plain values, `FooterData.Render()` does pure rendering with no Model access. 12 tests cover status bar, badges, worker levels, alerts, time travel, hint truncation.

### Skipped

- **bt-8jds** (wisp toggle key conflict) - Blocked by bt-tkhq (keybinding research, human gate). Both `w` and `W` are taken, needs keybinding audit before choosing a new key.

### Refactor epic status (bt-if3w)

5/7 children complete (oim6 closed this session). Remaining: bt-t82t (stale refs/golden files), bt-if3w.1 (sprint view extraction).

### Label picker redesign (bt-36h7, dogfooded)

- **Overlay compositing** - converted from full-screen replacement to OverlayCenter overlay, matching project filter pattern
- **RenderTitledPanel** - round borders with "Filter by Label" title in border
- **Search input** - all letter keys go to text input (no j/k/h/l navigation conflicts), arrow keys only for nav
- **Multi-select** - space toggles labels (checkmarks), enter applies compound OR filter
- **Composing filters** - label filter is now independent of status filter (open + area:tui works)
- **Selected labels pinned** - toggled labels stay at top of list even when filtered by search
- **Stable modal** - fixed width (computed from all labels), fixed height (padded to maxVisible), page-aligned windowing
- **Page navigation** - left/right arrows, PageUp lands at top, PageDown at bottom

### UX improvements

- **Filter toggle** - o/c/r keys now toggle (press again to revert to "all")
- **Sort cycle** - reordered to updated -> created newest -> created oldest -> priority
- **Esc clears everything** - status filter, label filter, sort mode, search all reset on esc

**All tests pass. Build clean.**

---

## 2026-04-13 - Beads Feature Surfacing Wave 4: Wisps, Swarm, Capabilities (bt-9kdo, bt-1knw, bt-t0z6)

**Final session of the 4-wave feature surfacing plan.** 3 commits (parallel subagents), 740 lines added across 12 files, 20 new tests.

### What shipped
- **Wisp visibility toggle** (bt-9kdo) - `w` key hides/shows ephemeral issues. Default: hidden (matches `bd ready`). Wisps render dimmed+italic when visible. Footer badge shows state. Filter applied across all view paths (list, board, graph, BQL, recipes).
- **Swarm wave visualization** (bt-1knw) - `s` key in graph view shells to `bd swarm validate --json`, colors nodes by wave (green=wave 0/ready, yellow=wave 1, blue=wave 2+). Metrics panel shows wave position, max parallelism, estimated sessions. 5-second timeout with graceful error handling.
- **Capability map** (bt-t0z6) - Parses `export:`, `provides:`, `external:<project>:<cap>` labels. Detail panel shows capabilities section in workspace/global mode. `aggregateCapabilities()` builds cross-project edge graph with unresolved dependency detection.

### Key design decisions
- Wisp `w` key reuses the existing global handler - fires wisp toggle in non-workspace mode, project picker in workspace mode
- Swarm data loaded via `exec.CommandContext` (same pattern as other bd integrations) - no direct Dolt writes
- Capability map is a detail panel section, not a new ViewMode - lower effort, 80% of the value

### Parent epic: bt-53du (beads feature surfacing)
All 4 waves complete. Sessions 0-1 (data model), Session 2 (gate indicators), Session 3 (stale/epic/state dims), Session 4 (wisps/swarm/capabilities).

---

## 2026-04-12 - Temporal Infrastructure: Dolt AS OF queries + TemporalCache (bt-ph1z.7)

**Foundation for cross-project trending features.** 4 commits, 955 lines added across 8 files, 13 new tests.

### What shipped
- **`LoadIssuesAsOf(timestamp)`** on `GlobalDoltReader` - queries each database individually using Dolt `AS OF` syntax. Per-database error handling: if one database has no commit at the requested timestamp, it's skipped with a warning (others still load).
- **`TemporalCache`** in `pkg/analysis/temporal.go` - stores `map[time.Time][]model.Issue` snapshots. TTL-based staleness (default 1hr, configurable via `BT_TEMPORAL_CACHE_TTL`). Max 30 snapshots cap (`BT_TEMPORAL_MAX_SNAPSHOTS`). Concurrent populate guard. Oldest-first eviction.
- **`SnapshotMetrics`** - lightweight summary struct (open/blocked/closed counts, 7-day velocity) computed per snapshot. `ComputeMetricsSeries()` produces a time-ordered series from cache data.
- **Background worker integration** - `startTemporalCacheLoop()` goroutine runs on the cache TTL cadence (hourly), independent of the 3-second UI poll. 5-second startup delay to avoid competing with main data load. `TemporalCacheReadyMsg` notifies the UI.
- **`DataSnapshot.TemporalCache`** field carries the cache reference to the UI layer.

### Key design decisions
- Per-database queries (not UNION ALL) for AS OF - databases have different commit histories
- Background goroutine separate from poll loop - slow cadence, own connection
- `IssueLoader` interface on `TemporalCache.Populate()` - testable without a live Dolt server
- Timestamps, not commit refs - simpler across databases with different commit cadences

### What this unlocks
- bt-ph1z.2: Sparkline snapshots (needs TemporalCache data)
- bt-ph1z.3: Diff mode (needs LoadIssuesAsOf for two-snapshot comparison)
- bt-ph1z.4: Timeline view (needs SnapshotMetrics series)

**All 1483 package tests pass. Build clean.**

---

## 2026-04-10c - Phase 2 + Phase 3: Theme redesign + Cobra CLI (bt-k5zs, bt-oim6, bt-zt9q)

**Two parallel refactors shipped**: Phase 2 (theme/color system) and Phase 3 (CLI structure) executed as parallel worktree agents since they touch disjoint file sets (pkg/ui/ vs cmd/bt/).

### Phase 2: AdaptiveColor kill + resolved color system (bt-k5zs)
- **174 `AdaptiveColor` occurrences eliminated** across 25 files. All color fields now use `color.Color` (resolved at load time based on `isDarkBackground`).
- **Dark mode detection**: `tea.BackgroundColorMsg` in Init()/Update() - the canonical Charm v2 pattern. Replaces the Phase 0 shim that defaulted to dark.
- **Theme struct redesigned**: All color fields changed from `AdaptiveColor` to `color.Color`. `resolveColor(light, dark)` helper resolves based on `isDarkBackground`.
- **styles.go**: All 52 package-level `Color*` vars changed to `color.Color`. New `resolveColors()` function rebuilds everything when dark/light changes.
- **theme_loader.go**: `AdaptiveHex.toColor()` resolves at load time. Fallback maps provide light/dark defaults for partial YAML overrides.
- **Glamour**: Style selection now dynamic (`"dark"`/`"light"`) based on `isDarkBackground`.
- **`adaptive_color.go` deleted**. The Phase 0 compatibility shim is gone.

### Phase 3: Cobra CLI migration (bt-zt9q)
- **main.go: 1,708 -> 13 lines**. Just `rootCmd.Execute()`.
- **Cobra subcommand tree**: `bt robot triage`, `bt robot graph`, `bt export pages`, `bt agents add`, `bt baseline check`, `bt version`, etc.
- **35+ robot subcommands** migrated from `--robot-*` flags to `bt robot *` subcommands.
- **Bare `bt` launches TUI** (not help). Uses `rootCmd.Run` + `SilenceUsage: true`.
- **Data loading deferred**: Only commands that need data call `loadIssues()`. `bt version`, `bt robot recipes`, `bt robot schema` skip it entirely.
- **Clean break**: No backward compat for old `--robot-*` flags (pre-alpha, one consumer).
- **Tests updated** for new subcommand syntax.

**Steps deferred to Phase 4 (bt-t82t)**: Pre-compute hot-path styles (optimization, needs profiling), footer extraction as FooterData (Phase 1.5, bt-oim6 - separate decomposition concern).

**All tests green. Build clean. 26 packages pass.**

---

## 2026-04-10b - Phase 1: Model decomposition (bt-98v9)

**Core refactor shipped**: 4 commits, 21 files, 3,235 insertions / 3,030 deletions.

**Step 1.1 - ViewMode enum**: Replaced 7 mutually exclusive boolean view flags (`isBoardView`, `isGraphView`, etc.) with an 11-value `ViewMode` enum. All routing (View(), Update(), key dispatch) now switches on `m.mode`.

**Step 1.2 - State extraction**: Moved ~50 fields from Model into focused sub-structs: `DataState` (pointer, issues/snapshot/worker), `FilterState` (pointer, filters/BQL/recipes), `AnalysisCache` (pointer, triage scores/counts). `DoltState` and `WorkspaceState` embedded as value types. Model copy per frame: ~1.6KB -> ~240 bytes.

**Step 1.3 - Modal state**: Replaced 19 `show*` booleans with single `activeModal ModalType` enum (16 values). Added `modalActive()`, `openModal()`, `closeModal()` helpers.

**Step 1.4 - Update() decomposition**: Split 2,387-line Update() into 147-line thin router + 3 handler files: `model_update_data.go` (871 lines), `model_update_input.go` (1,217 lines), `model_update_analysis.go` (348 lines). model.go: 3,684 -> 1,438 lines.

**Step 1.5 deferred**: Footer extraction (bt-oim6) - `model_footer.go` touches 35+ Model fields. Natural to bundle with Phase 2 theme redesign.

**Process**: 2 worker agents, ~65 min wall clock. Worker 1 exhausted context on a sed overshoot (replaced `m.issues` in FlowMatrixModel/InsightsModel receivers). Monitor caught it early. Worker 2 finished cleanly through Step 1.4.

**All 24 test packages green. Build clean.**

---

## 2026-04-03c - Global hub data layer (bt-6wbd phase 1)

**GlobalDoltReader shipped**: `internal/datasource/global_dolt.go` - connects to shared Dolt server without a database in the DSN, enumerates all beads project databases, loads issues via UNION ALL with backtick-quoted `database.table` syntax.

**Key implementation**:
- `DiscoverSharedServer()` reads `~/.beads/shared-server/dolt-server.port`, env override via `BT_GLOBAL_DOLT_PORT`
- `EnumerateDatabases()` uses `information_schema.tables` (single query, not N validation queries), filters system DBs
- `LoadIssues()` via UNION ALL across all databases, `SourceRepo` set from database name (overrides column)
- Batch labels/deps/comments via 3 UNION ALL queries (not N+1 per-issue)
- `GetLastModified()` via aggregated `MAX(MAX(updated_at))` across all databases
- Partial failure: broken DBs skipped with `slog.Warn`, healthy DBs loaded

**Source type integration**: `SourceTypeDoltGlobal` added to source.go, `RepoFilter` field on `DataSource`, `LoadFromSource` dispatch case in load.go.

**Poll loop**: `globalDoltPollOnce()` in background_worker.go, dispatched when source type is `SourceTypeDoltGlobal`. Reconnect does TCP dial only (no auto-start, shared server is user-managed).

**CLI**: `--global` flag, mutually exclusive with `--workspace` and `--as-of`. `--repo` filters database list at enumeration (before UNION ALL). Workspace mode UI activates automatically (badges, picker, prefilter).

**Shared column list**: Extracted `IssuesColumns` constant to `columns.go`, used by both `DoltReader` and `GlobalDoltReader`.

**Tests**: 16 new unit tests in `global_dolt_test.go` (query building, system DB filtering, backtick quoting, discovery, DSN construction). Full suite: 27 packages, 0 failures.

**Files created**: `internal/datasource/global_dolt.go`, `internal/datasource/global_dolt_test.go`, `internal/datasource/columns.go`
**Files modified**: `internal/datasource/dolt.go`, `internal/datasource/source.go`, `internal/datasource/load.go`, `pkg/ui/background_worker.go`, `cmd/bt/main.go`

**Bead closed**: bt-6wbd

## 2026-04-03b - BQL bug fixes + global hub planning

**BQL bugs (bt-bjk4)**: Fixed all 5 bugs from gap analysis:
1. Status enum validation - added `ValidStatusValues` map, catches typos like `status=opne`
2. `--robot-bql` envelope - now uses `RobotEnvelope` + `robotEncoder` (adds metadata, TOON support)
3. Dead code removal - removed unused `WithReadySQL` from sql.go
4. Date equality semantics - `created_at = today` now matches any time on that day (truncates to midnight)
5. ISO date parsing - `created_at > 2026-01-15` now works in lexer, parser, and executor

Tests added for all fixes. Full suite passes (27 packages, 0 failures).

**Triage**: bt-dx7k reopened (blocked, not in-progress), bt-28g8 closed (audit done), bt-2bns deferred (Charm v2), bt-xft1 closed (resolved by shared server architecture).

**Global hub design verification**: Verified 5 assumptions from the beads session's design doc against actual codebase. Updated open questions with findings. Key correction: poll system needs real refactoring, not just a query swap.

**Global hub data layer plan**: `docs/plans/2026-04-03-feat-global-hub-data-layer-plan.md` - 4-phase implementation plan for GlobalDoltReader. Batch N+1 queries into 3 UNION ALL, single aggregated MAX for poll, --global flag, workspace UI reuse.

**Beads closed**: bt-bjk4 (BQL bugs), bt-28g8 (keybinding audit), bt-xft1 (data separation)
**ADR-002 updated**: Stream 2 bugs all checked off, Stream 1 robot-bql checked off

## 2026-04-03 - Parallel audit swarm

Burned expiring weekly credits on 5 parallel research agents. All read-only, no code changes.

**Reports produced**:
- `docs/audit/test-suite-audit.md` - 268 test files: 93% KEEP, 0% REMOVE, 1 Windows P1
- `docs/audit/cli-ergonomics-audit.md` - 97 flags inventoried, 3 critical robot-mode envelope bugs
- `docs/audit/charm-v2-migration-scout.md` - 76 files affected, 60% mechanical, theme system is the hard part
- `docs/audit/bql-gap-analysis.md` - corrected stale memory (--bql/--robot-bql already shipped), found 5 bugs
- `docs/drafts/README-draft.md` - complete prose rewrite draft

**Beads closed**: bt-79eg (test audit), bt-pfic (CLI audit)
**Beads created**: bt-0cht (P1, robot-mode fixes), bt-5dvl (P2, test fixes), bt-bjk4 (P2, BQL bugs), bt-iuqy (P2, README review)
**ADR-001 closed out**, ADR-002 created as new project spine. Changelog extracted to this file.

## 2026-04-01 - BQL composable search

New package `pkg/bql/` - BQL parser vendored from zjrosen/perles (MIT), adapted for bt.

- Parser layer: lexer, parser, AST, tokens, validator, SQL builder (~1,500 LOC)
- MemoryExecutor: in-memory evaluation against model.Issue (522 LOC, 28 tests)
- TUI integration: `:` keybind opens BQL modal, dedicated `applyBQL()` filter path
- CLI: `--bql` and `--robot-bql` flags
- Syntax: =, !=, <, >, <=, >=, ~, !~, IN, NOT IN, AND/OR/NOT, parens, P0-P4, date literals, ORDER BY, EXPAND

22 files, ~3,950 lines, 27 packages pass, 0 failures.

## 2026-03-16b - Cross-platform test suite fixes

39 failing Windows tests -> 0 failures across all 26 packages.

- Phase 1: Renamed bv->bt stragglers in 8 files
- Phase 1b: Fixed ComputeUnblocks (filter blocking edges only), slug collision expectations
- Phase 2: filepath.FromSlash/Join in cass and tree test expectations
- Phase 3: configHome override for tutorial progress + wizard config (HOME env doesn't work on Windows)
- Phase 4: runtime.GOOS skip guards for 6 Unix-only permission tests
- Phase 5: Shell-dependent hooks tests skipped on Windows; fixed -r shorthand conflict
- Phase 6: .exe suffix for drift test binaries; file locking fix (defer order)
- Phase 7: Normalized \r\n in golden file comparison

**Closed**: bt-s3xg, bt-zclt, bt-3ju6, bt-7y06, bt-ri5b, bt-dwbl, bt-kmxe, bt-mo7r (8 issues)

## 2026-03-16a - Dolt lifecycle adaptation

New module `internal/doltctl/` for Dolt server management.

- EnsureServer: detects running server (TCP dial) or starts via `bd dolt start`
- StopIfOwned: PID-based ownership check before `bd dolt stop`
- Auto-reconnect: poll loop retries EnsureServer after 3 consecutive failures
- Port discovery chain: BEADS_DOLT_SERVER_PORT > BT_DOLT_PORT > .beads/dolt-server.port > config.yaml > 3307
- Database identity check: `SHOW TABLES LIKE 'issues'` after connecting
- Dead code removed: touchDoltActivity keepalive

11 doltctl tests + 6 metadata tests. **Closed**: bt-07jp (P1), bt-tebr (P2, subsumed)

## 2026-03-12 - Brainstorm + audit planning

No code changes. Post-takeover roadmap brainstorm + codebase audit design.

- Defined 4 phases: Audit -> Stabilize -> Polish -> Interactive
- Key decision: CRUD via bd shell-out (no beads fork needed)
- Designed 8-team parallel codebase audit (~190k LOC)
- Created 8 dogfood beads from TUI usage
- Docs: `docs/brainstorms/2026-03-12-post-takeover-roadmap.md`, `docs/plans/2026-03-12-codebase-audit-plan.md`

## 2026-03-11b - Dolt freshness + responsive help

- **bt-3ynd**: Fixed false STALE indicator - freshness tracks last successful poll, not snapshot build time
- **bt-aog1**: Responsive help overlay - 4x2 grid (wide), 2x4 (medium), single column (narrow)
- **bt-xavk**: Created help system redesign plan (docs/plans/help-system-redesign.md)

## 2026-03-11a - Dogfood polish

- Absolute timestamps in details pane + expanded card
- Priority shows P0-P4 text next to icon
- Status bar auto-clear after 3s
- Help overlay: centered titles, auto-sized panels, 4x2 grid, status indicators panel
- Board: auto-hide empty columns on card expand
- Shortcut audit: found 22 undocumented keys

## 2026-03-07 - Beads migration

Renamed issue prefix bv->bt (553 issues). Set beads.role=maintainer. Local folder renamed. Memory migrated.

## 2026-03-05c - ADR review cleanup

Fixed 14 stale `bv` CLI refs in AGENTS.md. Fixed insights detail panel viewport off-by-one.

## 2026-03-05b - Titled panels

Converted insights, board, and help overlay to RenderTitledPanel. Added BorderColor/TitleColor overrides. Board cards use RoundedBorder + border-only selection.

## 2026-03-05a - Tomorrow Night theme

Visual overhaul: Tomorrow Night + matcha-dark-sea teal. Theme config system (embedded defaults, layered loading). TitledPanel helper. Swapped all Color* vars. 18 new tests.

## 2026-02-25 to 2026-03-04 - Fork takeover

See [ADR-001](docs/adr/001-btui-fork-takeover.md) for detailed session-by-session changelog of the fork takeover work (streams 1-4: Dolt verification, rename, data migration, spring cleaning).
