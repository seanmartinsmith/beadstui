# Cross-Project Management Gaps

**Date:** 2026-04-12
**Status:** Brainstorm complete
**Origin:** pewejekubam feedback on gastownhall/beads#3008, filed as bt-ph1z
**Parent issue:** bt-ph1z

## Context

A beads user (pewejekubam) independently arrived at the same shared-server + database-per-project architecture we use, then built ops tooling on top. Their needs map directly to BT's roadmap as a cross-project manager. This brainstorm shapes three feature areas they surfaced, plus validates what BT already has.

### What BT Already Has

- **Global Dolt mode** - production-ready. Connects to shared server, enumerates all databases, loads issues via `UNION ALL`. Auto-discovery, background polling.
- **Workspace mode** - plumbing exists (config, ID resolver, aggregate loader) but UI is dormant.
- **Graph analysis engine** - betweenness, critical path, PageRank, k-core, articulation points, HITS, eigenvector centrality. Currently single-project scoped.
- **Velocity tracking** - 7/30-day closure counts, trend direction, sparklines. Shallow - no historical trending.
- **Time-travel** - git-based `--as-of` only. No Dolt `SELECT AS OF` queries.

## What We're Building

Four feature areas, each independent, each buildable incrementally:

### 1. Portfolio Health Dashboard

**What:** Cross-project scoreboard as the default Projects view, with drill-down to full per-project graph analysis.

**Shape:**
- Scoreboard landing page: each project row shows open/blocked/closed counts, velocity trend arrow, health color
- Drill into a project to get the full graph analysis (betweenness, critical path, cycles, etc.)
- Progressive disclosure - quick scan at portfolio level, deep analysis per project

**Why this approach:** The data pipeline already exists (global Dolt mode loads everything). The analytics engine already works per-project. The gap is running it cross-project and surfacing it in the Projects view. Scoreboard-first keeps the default view fast and scannable.

**Builds on:** `internal/datasource/global_dolt.go`, `pkg/analysis/`, `pkg/ui/insights.go`

### 2. Temporal Trending

**What:** Leverage Dolt's version history to show how projects evolve over time. Three sub-features:

**a) Sparkline snapshots** - Daily snapshots of key metrics (open count, blocked count, velocity) rendered as sparklines in existing views. Lightweight, no new views needed.

**b) Dedicated timeline view** - Full-screen analytics: burndown charts, blocker trends, velocity curves over configurable time windows.

**c) Diff mode** - Compare "now" vs "N days ago." Show which issues are new, which got unblocked, which stalled. Like git diff but for project state.

**Why this approach:** Dolt stores full version history of every row. `SELECT ... AS OF` queries give us point-in-time snapshots for free. Sparklines are the cheapest win (slot into existing views). Timeline view is the richest. Diff mode is the most actionable for daily triage.

**Builds on:** Dolt's `AS OF` syntax, `pkg/ui/velocity_comparison.go` (existing sparkline rendering)

### 3. Cross-Project Dependencies

**What:** Graph analysis that spans project boundaries.

**Shape:**
- Default: per-project graphs with cross-project deps shown as "external" edges, highlighted differently
- Toggle: unified supergraph across all projects for org-level bottleneck analysis
- Same toggle pattern as portfolio health (per-project default, cross-project on demand)

**Why this approach:** Per-project keeps the default view clean and performant. Supergraph reveals org-level bottlenecks but gets noisy with many projects. Toggle lets the system grow into the supergraph naturally as cross-project deps increase.

**Resolved:** bd supports `external:<project>:<capability>` syntax. bt parses these refs and resolves against the shared server's databases to build graph edges. No new format needed.

**Builds on:** `pkg/analysis/graph.go`, `pkg/workspace/` (ID resolver, aggregate loader)

### 4. DR Replication (Documentation + Status)

**What:** Not a bt feature. Document the cron + `dolt pull` pattern for DR. Show sync/replication status in the Projects scoreboard.

**Shape:**
- Documentation: how to set up replication (it's a 3-line cron script using native Dolt commands)
- Status indicator in Projects view: "last sync: 2h ago", "replica: healthy/stale"
- Query Dolt system tables for commit history and remote state

**Why this approach:** Dolt already has native replication (`dolt backup`, `dolt push/pull`). Building bt admin commands would just wrap existing CLI. The value-add for bt is visibility, not management.

## Key Decisions

1. **Portfolio health uses progressive disclosure** - scoreboard default, drill-down for full analysis
2. **Temporal trending includes all three levels** - sparklines (cheap), timeline view (rich), diff mode (actionable)
3. **Cross-project graph defaults to per-project** with supergraph toggle - grows naturally as deps increase
4. **DR is docs + status indicator** - no management commands, leverage native Dolt replication

## Resolved Questions

1. **Cross-project deps format:** bd supports `external:<project>:<capability>` syntax. Stored as-is, resolved at query time via `external_projects` config. bt needs to parse these refs and resolve against the shared server's databases to build graph edges. No new format needed.
2. **Timeline view scope:** Both per-project and portfolio-level, default per-project. Same progressive disclosure pattern as everything else.

## Open Questions

1. What's the performance profile of `SELECT AS OF` across many commits? Need to benchmark before committing to sparkline snapshot frequency.

## Priority Ordering

These are independent and can be built in any order. Suggested sequence based on existing infrastructure:

1. **Portfolio health scoreboard** - smallest gap (data pipeline exists, just need UI)
2. **Temporal trending: sparklines** - cheapest trending win, slots into existing views
3. **Temporal trending: diff mode** - high daily-use value for triage
4. **Cross-project dependencies** - needs the most new plumbing
5. **Temporal trending: timeline view** - richest but most complex, benefits from sparkline/diff groundwork
6. **DR status indicator** - lowest priority, depends on portfolio scoreboard existing first
