---
title: "feat: Cross-Project Management"
type: feat
status: active
date: 2026-04-12
origin: docs/brainstorms/2026-04-12-cross-project-management-brainstorm.md
parent_issue: bt-ph1z
---

# Cross-Project Management

## Overview

Build bt into a cross-project management tool by closing three gaps identified from real user feedback (gastownhall/beads#3008): portfolio health visibility, temporal trending, and cross-project dependency analysis. A fourth area (DR replication status) adds lightweight ops visibility.

bt already has global Dolt mode (production-ready multi-database loading), a sophisticated graph analysis engine, and velocity tracking. The work is extending these capabilities across project boundaries and adding the time dimension.

## Problem Statement / Motivation

A beads user (pewejekubam) independently built the same shared-server + database-per-project architecture, then built ops tooling for cross-project visibility. Their needs - central management, trending, replication monitoring - map directly to bt's natural role as the cross-project layer above `bd`.

bt loads issues from all projects today but doesn't analyze them cross-project. The analytics engine runs per-project. There's no temporal dimension (Dolt has full version history but bt only uses git-based `--as-of`). These are the gaps.

## Proposed Solution

Eight phases (0-7), each independently shippable. Phase 0 fixes a known blocker. Phases 1-7 build features incrementally with shared infrastructure extracted as it emerges.

(see brainstorm: `docs/brainstorms/2026-04-12-cross-project-management-brainstorm.md`)

## Technical Approach

### Architecture

**Data flow extension:**

```
Dolt shared server
  ├── GlobalDoltReader.LoadIssues()          [exists]
  ├── GlobalDoltReader.LoadIssuesAsOf(ts)    [NEW - Phase 2]
  ├── GlobalDoltReader.LoadDoltLog(db)       [NEW - Phase 7]
  └── GlobalDoltReader.LoadRemoteStatus(db)  [NEW - Phase 7]
       │
       ▼
  []model.Issue (with SourceRepo set)
       │
       ├── Per-project filter -> Analyzer -> ProjectHealth  [NEW - Phase 1]
       ├── Multi-snapshot -> TemporalCache -> Sparklines    [NEW - Phases 2-3]
       ├── Two-snapshot diff -> DiffResult                  [NEW - Phase 4]
       ├── ExternalDepResolver -> cross-project edges       [NEW - Phase 5]
       └── System table query -> ReplicaStatus              [NEW - Phase 6]
```

**Key architectural decisions:**
- **New ViewMode: `ViewPortfolio`** - dedicated full-screen view, not a modal overlay. The repo picker modal stays as a filter mechanism. The portfolio view is the scoreboard landing page with drill-down. (SpecFlow gap #1)
- **Filter-based drill-down, not view stack** - drilling into a project from the scoreboard applies a project filter and switches to ViewInsights. Escape clears the filter and returns to ViewPortfolio. No view stack needed yet. (SpecFlow gap #6)
- **Temporal data via background worker** - AS OF queries run in a separate goroutine on a slow cadence (hourly default, configurable). Results cached in a `TemporalCache` struct on the snapshot. Never on the 3-second poll cycle. (SpecFlow gap #5)
- **AS OF uses timestamps, not commit refs** - `AS OF 'YYYY-MM-DD'` is simpler and works across databases with different commit cadences. Dolt resolves to the nearest commit before that timestamp. (SpecFlow gap #4a)
- **Schema mismatch handling** - AS OF queries use explicit column lists matching the current schema. If a column doesn't exist at the old commit, the query fails for that database - catch the error, skip that database's snapshot with a log warning. Don't use `SELECT *` (column order is unreliable across schema versions). (SpecFlow gap #4c)

### Implementation Phases

#### Phase 0: Fix bt-ktig (Prerequisite)

**Issue:** bt-ktig
**Why:** Dependency loading is silently broken in global mode. Runs without error but attaches nothing. Blocks all graph-based features (portfolio health scores, cross-project graphs, blocking counts in scoreboard).

**Tasks:**
- [ ] Diagnose root cause in `GlobalDoltReader.loadAllDependencies()` (`internal/datasource/global_dolt.go`)
- [ ] Fix issueMap key matching (likely prefix qualification mismatch between dep.DependsOnID and issue.ID)
- [ ] Verify labels and comments also load correctly in global mode (same UNION ALL pattern, may have same bug)
- [ ] Add test: load issues from 2+ databases, verify dependencies are attached cross-database
- [ ] Verify graph analysis produces non-empty results in global mode

**Success criteria:** `bd dep list` output matches what bt shows in global mode. Graph view shows edges between issues.

**Files:**
- `internal/datasource/global_dolt.go` - dependency loading
- `pkg/ui/background_worker.go` - globalDoltPollOnce refresh path

---

#### Phase 1: Portfolio Health Scoreboard (bt-ph1z.1)

**Issue:** bt-ph1z.1
**Depends on:** Phase 0 (bt-ktig)

**What:** New `ViewPortfolio` showing a table of projects with health metrics. Drill into any project for full analysis.

**Tasks:**

**1.1 Add ViewPortfolio ViewMode**
- [ ] Add `ViewPortfolio` to ViewMode enum in `pkg/ui/model.go`
- [ ] Add `PortfolioModel` struct in new file `pkg/ui/portfolio.go`
- [ ] Add focus state `FocusPortfolio` to focus enum
- [ ] Wire key handler `handlePortfolioKeys()` into Update switch
- [ ] Wire view renderer into View() rendering section
- [ ] Assign key binding: `P` (uppercase, currently unused for view switching)

**1.2 Compute per-project health metrics**
- [ ] Add `ProjectHealth` struct: project name, open/blocked/closed/total counts, velocity (7d/30d closures), trend direction, health color
- [ ] Add `ComputeProjectHealth(issues []model.Issue, projectName string) ProjectHealth` in `pkg/analysis/`
- [ ] Add `ComputeAllProjectHealth(issues []model.Issue) []ProjectHealth` - groups by SourceRepo, computes per-project
- [ ] Health color logic: green (no cycles, <20% blocked), yellow (some blocked or stale), red (cycles or >50% blocked)
- [ ] Run in SnapshotBuilder, not UI thread

**1.3 Render scoreboard table**
- [ ] Columns: Project | Open | Blocked | Closed | Velocity (7d) | Trend | Health
- [ ] Sort by health (worst first) by default. `s` cycles: health, name, open count, velocity
- [ ] Cursor navigation (j/k), enter to drill down
- [ ] Responsive: hide velocity columns on narrow terminals (same pattern as delegate.go)

**1.4 Drill-down to project analysis**
- [ ] Enter on a project row: set `m.filter.repoFilter = projectName`, switch to ViewInsights
- [ ] Escape from ViewInsights when repoFilter is set: clear filter, return to ViewPortfolio
- [ ] Insights view shows per-project graph analysis (already works if issues are filtered)

**Acceptance criteria:**
- [ ] `P` key opens portfolio scoreboard from any view
- [ ] All projects on the shared server appear as rows
- [ ] Health color reflects dependency/blocker state
- [ ] Enter drills into per-project insights, Escape returns to scoreboard
- [ ] Works in global mode with 2+ databases

**Files:**
- `pkg/ui/model.go` - ViewMode enum, focus enum, model fields
- `pkg/ui/portfolio.go` - new file, PortfolioModel
- `pkg/ui/model_keys.go` - key binding for P, handlePortfolioKeys
- `pkg/ui/model_modes.go` - enterPortfolioMode, drill-down/return logic
- `pkg/analysis/project_health.go` - new file, ProjectHealth computation
- `pkg/ui/snapshot.go` - add ProjectHealthData to DataSnapshot

---

#### Phase 2: Temporal Infrastructure (Shared Groundwork)

**Issue:** bt-ph1z.7

**What:** Add Dolt `AS OF` query capability to GlobalDoltReader. Build the TemporalCache.

**Tasks:**

**2.1 Add LoadIssuesAsOf to GlobalDoltReader**
- [ ] Add method `LoadIssuesAsOf(timestamp time.Time) ([]model.Issue, error)` to `global_dolt.go`
- [ ] Query pattern: `SELECT <cols> FROM \`db\`.issues AS OF '<timestamp>' WHERE status != 'tombstone'` per database
- [ ] Handle errors gracefully per database: if AS OF fails (no commit at that time, schema mismatch), skip that database with a log warning, don't fail the whole query
- [ ] Return issues with SourceRepo set, same as LoadIssues

**2.2 Benchmark AS OF performance**
- [ ] Run AS OF queries against production databases with varying commit depths (10, 100, 1000 commits)
- [ ] Measure query time for single-database and multi-database (UNION ALL) cases
- [ ] Determine safe snapshot frequency (daily, every 6 hours, etc.)
- [ ] Document results in `docs/performance.md`

**2.3 Build TemporalCache**
- [ ] Add `TemporalCache` struct in new file `pkg/analysis/temporal.go`
- [ ] Stores `map[time.Time][]model.Issue` - snapshots keyed by date
- [ ] `TemporalCache.LoadRange(reader, from, to, interval)` - loads snapshots at interval within range
- [ ] Cache invalidation: TTL-based (default 1 hour, configurable via `BT_TEMPORAL_CACHE_TTL`)
- [ ] Max snapshots cap (default 30) to bound memory
- [ ] Background goroutine for cache population - not on the 3-second poll cycle

**2.4 Add temporal snapshot metrics**
- [ ] `SnapshotMetrics` struct: timestamp, open count, blocked count, closed count, velocity, cycle count
- [ ] `ComputeSnapshotMetrics(issues []model.Issue, ts time.Time) SnapshotMetrics`
- [ ] `ComputeMetricsSeries(cache *TemporalCache) []SnapshotMetrics` - metrics across all cached snapshots

**Acceptance criteria:**
- [ ] `LoadIssuesAsOf` returns valid issues for timestamps within Dolt commit history
- [ ] Gracefully handles databases with no commits at the requested timestamp
- [ ] Performance benchmark documented
- [ ] TemporalCache populates in background without blocking UI

**Files:**
- `internal/datasource/global_dolt.go` - LoadIssuesAsOf method
- `pkg/analysis/temporal.go` - new file, TemporalCache and SnapshotMetrics
- `pkg/ui/background_worker.go` - temporal cache refresh goroutine
- `pkg/ui/snapshot.go` - add TemporalCache to DataSnapshot

---

#### Phase 3: Sparkline Snapshots (bt-ph1z.2)

**Issue:** bt-ph1z.2
**Depends on:** Phase 2 (temporal infrastructure)

**What:** Render sparklines from historical Dolt snapshots in existing views.

**Tasks:**

**3.1 Integrate sparklines into existing views**
- [ ] Add sparkline column to portfolio scoreboard (Phase 1): 7-day metric trend per project
- [ ] Add sparkline to issue list delegate: per-project mini sparkline in header when grouped by project
- [ ] Reuse existing `buildSparkline()` from `velocity_comparison.go` (Unicode block characters)

**3.2 Sparkline data source**
- [ ] Sparklines use TemporalCache data (AS OF-based), not ClosedAt-based velocity
- [ ] `SparklineData` struct: `[]float64` values (one per day/interval), label, trend direction
- [ ] `ComputeSparklines(metrics []SnapshotMetrics, field string) SparklineData` - extract one metric across snapshots
- [ ] Fields: "open", "blocked", "velocity", "cycle_count"

**3.3 Coexistence with existing velocity**
- [ ] Existing velocity comparison view (`velocity_comparison.go`) keeps ClosedAt-based data for per-label breakdown
- [ ] New sparklines show project-level trends from AS OF data
- [ ] Different data sources, different granularity - both are valid, don't try to unify

**Acceptance criteria:**
- [ ] Portfolio scoreboard shows 7-day sparkline per project
- [ ] Sparklines update when TemporalCache refreshes (hourly)
- [ ] Sparklines gracefully handle insufficient history (show flat line or "no data")

**Files:**
- `pkg/ui/portfolio.go` - sparkline column in scoreboard
- `pkg/analysis/temporal.go` - SparklineData, ComputeSparklines
- `pkg/ui/velocity_comparison.go` - reuse buildSparkline()

---

#### Phase 4: Diff Mode (bt-ph1z.3)

**Issue:** bt-ph1z.3
**Depends on:** Phase 2 (temporal infrastructure)

**What:** Compare current state vs N days ago. Show new, unblocked, and stalled issues.

**Tasks:**

**4.1 Diff computation**
- [ ] `DiffResult` struct: new issues, closed issues, newly blocked, newly unblocked, stalled (open then, still open now, no update)
- [ ] `ComputeDiff(current []model.Issue, historical []model.Issue) DiffResult`
- [ ] Match issues by ID across snapshots. Handle ID not found in historical (new issue) or current (closed/deleted)
- [ ] "Stalled" = open in both snapshots and `UpdatedAt` hasn't changed

**4.2 Diff UI**
- [ ] Extend existing time-travel mechanism in `model_modes.go` for Dolt mode
- [ ] New key: `D` (diff mode toggle from any list view)
- [ ] Prompt for time range: default 7 days, accept number input (same input pattern as time-travel modal)
- [ ] Render diff as filtered issue list with diff badges: `+NEW`, `-CLOSED`, `UNBLOCKED`, `STALLED`
- [ ] Color coding: green for new/unblocked, red for closed, yellow/dim for stalled

**4.3 Diff in global mode**
- [ ] Diff works per-project (uses repoFilter) or across all projects
- [ ] In portfolio view, `D` shows aggregate diff across all projects

**Acceptance criteria:**
- [ ] `D` enters diff mode, shows comparison with 7-day default
- [ ] Each issue has a clear badge indicating its diff status
- [ ] Works in both single-project and global mode
- [ ] Escape exits diff mode and returns to normal view

**Files:**
- `pkg/analysis/temporal.go` - DiffResult, ComputeDiff
- `pkg/ui/model_modes.go` - enterDiffMode, Dolt-native time-travel path
- `pkg/ui/delegate.go` - diff badge rendering
- `pkg/ui/model_keys.go` - D key binding

---

#### Phase 5: Cross-Project Dependency Graphs (bt-ph1z.5)

**Issue:** bt-ph1z.5
**Depends on:** Phase 0 (bt-ktig)

**What:** Graph analysis spanning project boundaries with toggle between per-project and supergraph views.

**Tasks:**

**5.1 Parse external dependency format**
- [ ] Add `ParseExternalDep(raw string) (project, capability string, isExternal bool)` in `pkg/analysis/`
- [ ] Parse `external:<project>:<capability>` from `Dependency.DependsOnID`
- [ ] Determine what `<capability>` maps to by checking upstream beads code. Likely an issue ID in the target project.
- [ ] Handle unresolvable refs: log warning, render as dashed edge in graph

**5.2 Build cross-project graph edges**
- [ ] `ExternalDepResolver` struct: takes `map[string][]model.Issue` (issues grouped by project), resolves external refs to concrete issue IDs
- [ ] On resolution: replace `external:projectB:cap-123` edge with direct edge to `projectB-cap-123` issue
- [ ] Unresolvable refs: keep as placeholder nodes with "external" type for display

**5.3 Extend graph view with supergraph toggle**
- [ ] Add `supergraphMode bool` to GraphModel
- [ ] Default: per-project graph (filtered by repoFilter). External deps shown as outgoing edges to labeled placeholder nodes
- [ ] Toggle key: `G` (uppercase) switches between per-project and supergraph
- [ ] Supergraph: all issues from all projects in one graph. Color-code nodes by project. External edges highlighted
- [ ] Supergraph uses same Phase 2 async analysis - may be slow for large datasets, that's OK (existing timeout guards apply)

**5.4 Cross-project metrics in insights**
- [ ] When in supergraph mode, insights show org-level metrics: cross-project bottlenecks, articulation points that span projects
- [ ] Add "Cross-Project" panel to InsightsModel: top N issues that block issues in other projects

**Acceptance criteria:**
- [ ] External deps parsed and resolved when both projects are on the shared server
- [ ] Per-project graph shows external edges as highlighted outgoing edges
- [ ] `G` toggles to supergraph with all projects
- [ ] Supergraph identifies cross-project bottlenecks

**Files:**
- `pkg/analysis/external_deps.go` - new file, ParseExternalDep, ExternalDepResolver
- `pkg/ui/graph.go` - supergraph toggle, project-colored nodes
- `pkg/ui/insights.go` - cross-project panel
- `pkg/ui/model_keys.go` - G key binding

---

#### Phase 6: Timeline View (bt-ph1z.4)

**Issue:** bt-ph1z.4
**Depends on:** Phases 2, 3 (temporal infrastructure + sparklines)

**What:** Full-screen analytics view with burndown charts, blocker trends, velocity curves.

**Tasks:**

**6.1 Add ViewTimeline ViewMode**
- [ ] Add `ViewTimeline` to ViewMode enum
- [ ] Add `TimelineModel` struct in new file `pkg/ui/timeline.go`
- [ ] Key binding: `T` (check availability - may conflict with tree view)
- [ ] If `T` conflicts, use `ctrl+t` or find available key

**6.2 Render charts**
- [ ] Burndown chart: ASCII line chart (open issues over time), rendered from SnapshotMetrics
- [ ] Blocker trend: stacked area (blocked vs unblocked over time)
- [ ] Velocity curve: line chart of closures per period
- [ ] Use Lipgloss for styling, Unicode box-drawing characters for chart axes
- [ ] Configurable time window: 7d, 14d, 30d (cycle with left/right keys)

**6.3 Per-project and portfolio toggle**
- [ ] Default: per-project (filtered by repoFilter or current project)
- [ ] Toggle: portfolio-level (aggregate across all projects)
- [ ] Same toggle pattern as supergraph: key to switch, label in footer

**Acceptance criteria:**
- [ ] Timeline view renders burndown, blocker, and velocity charts
- [ ] Charts scale to terminal width
- [ ] Time window is adjustable
- [ ] Works at per-project and portfolio level

**Files:**
- `pkg/ui/timeline.go` - new file, TimelineModel
- `pkg/ui/model.go` - ViewTimeline enum
- `pkg/ui/model_keys.go` - key binding
- `pkg/analysis/temporal.go` - chart data preparation

---

#### Phase 7: DR Status Indicator (bt-ph1z.6)

**Issue:** bt-ph1z.6
**Depends on:** Phase 1 (portfolio scoreboard)

**What:** Show replication/sync status in the portfolio scoreboard.

**Tasks:**

**7.1 Query Dolt system tables**
- [ ] Add `LoadDoltLog(db string, limit int) ([]DoltCommit, error)` to GlobalDoltReader
- [ ] Query `dolt_log` system table per database: `SELECT commit_hash, committer, date FROM \`db\`.dolt_log ORDER BY date DESC LIMIT N`
- [ ] Add `LoadRemoteStatus(db string) ([]DoltRemote, error)` - query `dolt_remotes` for configured remotes
- [ ] `ReplicaStatus` struct: last commit timestamp, remote count, last push timestamp (if available)

**7.2 Add status column to scoreboard**
- [ ] New column in portfolio scoreboard: "Sync" or "Last Activity"
- [ ] Show relative time of last commit ("2h ago", "3d ago")
- [ ] Color: green (<24h), yellow (1-7d), red (>7d) - configurable via `BT_STALE_THRESHOLD`
- [ ] If remotes configured, show sync indicator

**7.3 Documentation**
- [ ] Write docs page: `docs/guides/disaster-recovery.md`
- [ ] Document the cron + `dolt pull` pattern for DR replication
- [ ] Include example crontab entry and verification steps

**Acceptance criteria:**
- [ ] Portfolio scoreboard shows last activity time per project
- [ ] Color indicates staleness
- [ ] DR setup documented

**Files:**
- `internal/datasource/global_dolt.go` - LoadDoltLog, LoadRemoteStatus
- `pkg/ui/portfolio.go` - sync status column
- `docs/guides/disaster-recovery.md` - new file

## System-Wide Impact

### Interaction Graph

Phase 0 (bt-ktig fix) -> Phases 1 and 5 (both need working deps in global mode)
Phase 2 (temporal infra) -> Phases 3, 4, and 6 (all temporal features)
Phase 1 (portfolio) -> Phase 7 (DR status column lives in scoreboard)

Phase 2 can start immediately (no dependency on Phase 0). Phases 1 and 5 can run in parallel after Phase 0. All three tracks are independent of each other.

### Error Propagation

- AS OF queries can fail per-database (schema mismatch, no commit at timestamp). Errors are per-database, never per-query - one database failing doesn't block others.
- External dep resolution can fail (project not on server, capability not found). Unresolved deps render as placeholder nodes, not errors.
- Background temporal cache population failure should log and retry, never crash the UI.

### State Lifecycle Risks

- TemporalCache grows with snapshot count. Capped at 30 snapshots by default. LRU eviction prevents unbounded growth.
- Per-project analysis in portfolio view creates N Analyzer instances. CPU-bound. Run in SnapshotBuilder with per-project timeout (1 second per project, total budget 10 seconds).
- DataSnapshot grows with new fields. All new data is immutable once built - same safety model as existing snapshot architecture.

### Performance Budget

- Portfolio scoreboard: per-project health computation must complete within 10 seconds total for up to 20 projects. Use Phase 1 metrics only (counts, velocity) - skip Phase 2 graph analysis for the scoreboard. Full graph analysis only on drill-down.
- AS OF queries: budget 500ms per database per snapshot. With 13 databases and 30 snapshots, worst case is 195 seconds for a full cache population - runs in background over minutes, not blocking.
- Supergraph analysis: existing timeout guards (Phase 2 async, size-based algorithm selection) apply. No additional budget needed.

## Dependencies & Prerequisites

| Phase | Depends On | Blocking Issue |
|-------|-----------|---------------|
| 0 | None | bt-ktig |
| 1 | Phase 0 | bt-ph1z.1 |
| 2 | None | bt-ph1z.7 |
| 3 | Phase 2 | bt-ph1z.2 |
| 4 | Phase 2 | bt-ph1z.3 |
| 5 | Phase 0 | bt-ph1z.5 |
| 6 | Phases 2, 3 | bt-ph1z.4 |
| 7 | Phase 1 | bt-ph1z.6 |

## Risk Analysis & Mitigation

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|-----------|
| Dolt AS OF performance too slow for sparklines | Medium | High | Benchmark in Phase 2 before committing. Reduce snapshot frequency or use ClosedAt-based approximation as fallback. |
| AS OF schema mismatch breaks UNION ALL | Medium | Medium | Per-database error handling. Skip databases with incompatible schemas at old commits. |
| Supergraph too noisy with many projects | Low | Medium | Default to per-project view. Supergraph is opt-in toggle. |
| `external:` dep format doesn't resolve to issue IDs | Low | High | Check upstream beads code in Phase 5. If capability != issue ID, need upstream discussion. |
| Key binding conflicts | Low | Low | Check availability before assigning. Use uppercase or ctrl+ variants. |

## Open Questions

1. **Dolt AS OF performance** - Benchmark needed (Phase 2). Determines sparkline viability and cache frequency.
2. **`external:<project>:<capability>` semantics** - Is `<capability>` an issue ID? Need to check upstream beads code before Phase 5.
3. **Key binding for timeline view** - `T` may conflict with tree view. Determine in Phase 6.

## Sources & References

### Origin

- **Brainstorm document:** [docs/brainstorms/2026-04-12-cross-project-management-brainstorm.md](docs/brainstorms/2026-04-12-cross-project-management-brainstorm.md) - Key decisions: progressive disclosure for all views, AS OF timestamps not commit refs, DR is docs+status not management commands.

### Internal References

- Global Dolt reader: `internal/datasource/global_dolt.go`
- Analysis engine: `pkg/analysis/graph.go`, `pkg/analysis/insights.go`
- View architecture: `pkg/ui/model.go` (ViewMode enum, focus routing)
- Velocity sparklines: `pkg/ui/velocity_comparison.go`
- Performance guide: `docs/performance.md`
- Global mode audit: `docs/audit/global-mode-readiness.md`
- Known bug: bt-ktig (broken deps in global mode)

### Related Issues

- bt-ph1z - parent epic
- bt-ph1z.1 through bt-ph1z.6 - child issues per feature area
- bt-ktig - prerequisite fix for dependency loading
- gastownhall/beads#3008 - upstream issue with user feedback
