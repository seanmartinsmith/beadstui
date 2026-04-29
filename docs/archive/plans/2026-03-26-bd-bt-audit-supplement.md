---
date: 2026-03-26
topic: bd-bt-audit-supplement
source: beads repo session - cross-project analysis of bd<>bt relationship
parent: docs/plans/2026-03-16-codebase-audit-plan-v2.md
related:
  - ~/System/tools/bt/docs/brainstorms/2026-03-26-bd-bt-compatibility-surface.md
  - ~/System/tools/beads/docs/brainstorms/2026-03-26-global-beads-exploration.md
  - ~/.files/docs/investigations/2026-03-26-subagent-prompt-bloat.md
status: ready-for-bt-session
---

# Audit Supplement: What the Beads Repo Knows About bt

## Why This Exists

On 2026-03-26, a beads repo session analyzed bt from the outside: what bt
reads from beads, what it doesn't, what's broken, and what the 44-commit
upstream pull means for bt. This doc captures findings that a bt session
needs but wouldn't discover on its own (because the context lives in the
beads repo, not the bt repo).

This supplements the existing codebase audit plan v2 (2026-03-16). It does
NOT replace it. Use both.

**Audit status**: The v2 audit WAS executed. All 9 team reports exist at
`docs/audit/team-*.md` plus `docs/audit/architecture-map.md` and
`docs/audit/beads-ecosystem-audit.md`. The ecosystem audit was done at bd
v0.61.0 (pre-44-commit-pull). Still open from that audit: bt-79eg (test suite
relevance) and bt-pfic (CLI ergonomics). This supplement adds the delta from
the 44-commit pull and the bd<>bt compatibility analysis done from the beads
side.

## What Changed Upstream (beads v0.62.0-dev, 44 commits)

Pulled 2026-03-26 (c329f0da..5ca6bb88). Key changes relevant to bt:

### Storage architecture refactor

Major extraction of logic into new packages:
- `internal/storage/issueops/` - blocked detection, bulk ops, compaction,
  federation, statistics (extracted from embeddeddolt)
- `internal/storage/versioncontrolops/` - backup, restore, flatten, branches,
  commit, GC (extracted from dolt storage)

**bt impact**: None direct (bt reads via SQL, not Go imports). But if bt ever
imports beads as a module, these are the new interfaces.

### Schema changes

- **HOP columns removed**: `crystallizes` and `quality_score` dropped from
  embedded schema. bt never referenced these - safe.
- **New migration 0023**: `no_history` column added to issues table
- **New tables**: wisps, federation_peers, interactions, routes,
  issue_snapshots, compaction_snapshots, repo_mtimes, child_counters

### Bug fixes relevant to bt

- `resolveCommandBeadsDir` no longer falls back to CWD-based discovery (#2775).
  If bt shells out to `bd` commands (Phase 4 writes), they must be run from
  a directory where `.beads/` is discoverable.
- Circuit breaker files moved to subdirectory (#2799) - `.beads/` internal
  structure changed.
- UTC timestamp fix (#2819) - `defer_until` comparisons now use UTC.

### Massive test infrastructure

coffeegoddd built embedded tests for nearly every `bd` command. This signals
the CLI API is stabilizing. Good news for bt Phase 4 (shelling out to `bd`).

## Known Bugs in bt (Found From Beads Side)

### Bug: Dolt reader missing due_at

`internal/datasource/dolt.go:LoadIssuesFiltered` does NOT select `due_at`.
Due dates are silently lost when reading from Dolt. The SQLite reader does
read `due_date` (different column name). Fix: add `due_at` to the Dolt
SELECT and map to `issue.DueDate`.

Location: `internal/datasource/dolt.go`, the main SELECT query (~line 75).

### Drift: dependency type column name

Dolt schema uses `type`. SQLite uses `dependency_type`. bt handles this with
separate queries per reader. If beads renames the Dolt column, bt silently
loses dependency type info (the query fails, deps load without type).

Dolt reader: `internal/datasource/dolt.go:loadDependencies`
SQLite reader: `internal/datasource/sqlite.go:loadDependencies`

### Drift: tombstone filtering

Dolt reader filters `WHERE status != 'tombstone'`. SQLite reader filters
`WHERE (tombstone IS NULL OR tombstone = 0)`. Different mechanisms. Not a
current bug (separate code paths) but a maintenance smell.

## Columns bt Should Start Reading

Prioritized by relevance to planned features:

### Immediate (before Phase 4 write support)

| Column | Why | Where to add |
|---|---|---|
| `due_at` | Bug fix - due dates missing in Dolt path | dolt.go SELECT |
| `defer_until` | Deferred beads should show differently | dolt.go SELECT + model |
| `pinned` | Pinned beads should be visually distinct | dolt.go SELECT + model |
| `created_by` / `owner` | Attribution for create/edit flows | dolt.go SELECT + model |

### Before global beads

| Column | Why |
|---|---|
| `source_repo` | Already in bt model, used for multi-repo filtering |
| `agent_state`, `rig` | Multi-agent visibility |
| `work_type` | Filtering by work category |

### Tables to read

| Table | Why |
|---|---|
| `issue_snapshots` | Could power history/burndown (currently computed from git) |
| `config` | Surface bd configuration in bt UI |

## The --robot-* Surface: Dolt Compatibility Audit

This is the critical question the existing audit plan doesn't emphasize
enough. bt has 25+ `--robot-*` commands. Most were built by Jeffrey Emmanuel
for beads_viewer, which used SQLite/JSONL. The question: **which ones
actually work with Dolt?**

### Full --robot-* command list

```
--robot-alerts          Drift + proactive alerts as JSON
--robot-blocker-chain   Full blocker chain for issue ID
--robot-burndown        Burndown data for sprint
--robot-capacity        Capacity simulation + projection
--robot-causality       Causal chain analysis for bead ID
--robot-confirm-correlation    Confirm commit-bead link
--robot-correlation-stats      Correlation feedback stats
--robot-diff            Diff as JSON (with --diff-since)
--robot-docs            Machine-readable docs for agents
--robot-drift           Drift check as JSON
--robot-explain-correlation    Why commit links to bead
--robot-file-beads      Beads that touched a file path
--robot-file-hotspots   Files touched by most beads
--robot-file-relations  Files that co-change with given file
--robot-forecast        ETA forecast for bead or all open
--robot-graph           Dependency graph as JSON/DOT/Mermaid
--robot-help            AI agent help
--robot-history         Bead-to-commit correlations
--robot-impact          Impact of modifying files
--robot-impact-network  Bead impact network
--robot-insights        Graph analysis + insights
--robot-label-attention Attention-ranked labels
--robot-label-flow      Cross-label dependency flow
--robot-label-health    Label health metrics
--robot-metrics         Performance metrics
--robot-next            Top pick recommendation (minimal triage)
--robot-orphans         Orphan commit candidates
--robot-plan            Dependency-respecting execution plan
--robot-priority        Priority recommendations
--robot-recipes         Available recipes
--robot-reject-correlation     Reject incorrect correlation
--robot-related         Beads related to specific bead
--robot-schema          JSON Schema for all robot commands
--robot-search          Semantic search results
--robot-sprint-list     Sprints as JSON
--robot-sprint-show     Sprint details
--robot-suggest         Smart suggestions (dupes, deps, labels, cycles)
--robot-triage          Unified triage (the mega-command)
--robot-triage-by-label    Triage grouped by label
--robot-triage-by-track    Triage grouped by execution track
```

### Audit approach for Dolt compatibility

For each --robot-* command, the audit should determine:

1. **Data source**: Does it read from the issue model (Dolt-safe) or does it
   hit SQLite/JSONL directly?
2. **Git dependency**: Does it need git history? (commit correlation,
   file-beads, hotspots, history commands all likely do)
3. **Functional**: Does it actually produce output with current Dolt data?
4. **Useful**: Is the output valuable for beads workflows?

Likely categories after audit:

| Category | Commands | Notes |
|---|---|---|
| Works with Dolt | graph, insights, priority, plan, triage, next, suggest, alerts, schema, docs, help, recipes | These operate on the issue model, not storage directly |
| Needs git history | history, file-beads, file-hotspots, file-relations, orphans, causality, correlation-*, impact, impact-network, explain-correlation | These correlate git commits to beads - should work if repo has git history |
| Needs investigation | burndown, capacity, forecast, sprint-*, diff, drift, label-* | May have SQLite assumptions or missing data |
| Likely broken | search (needs vector index setup) | Semantic search requires embedding infrastructure |

### How to run the audit

For each command, from a project directory with active Dolt beads:

```bash
# Quick smoke test
bt --robot-triage 2>&1 | head -20
bt --robot-insights 2>&1 | head -20
bt --robot-graph 2>&1 | head -20
# etc.

# Check for errors
bt --robot-triage 2>&1 | grep -i "error\|panic\|nil pointer\|not found"
```

Use a project with real beads data (bt's own repo has 28 open issues - good
test bed). Run each command, capture output, note which work / error / produce
empty results.

## Connection to Global Beads Vision

A brainstorm doc in the beads repo explores bt as the intelligence layer for
cross-project beads management:

- `~/System/tools/beads/docs/brainstorms/2026-03-26-global-beads-exploration.md`

Key points for bt sessions:
- bt already has `--workspace` support for multi-repo
- The --robot-* API is the agent interface for cross-project intelligence
- bd would provide global repo registry + basic listing (data plumbing)
- bt would provide global triage, cross-project graph, unified forecasting
- steveyegge/beads#2826 proposes remote Dolt hydration - potential enabler
- DoltHub private repos cost ~$50/mo - constraint for cross-machine sync

The ecosystem vision: bd (data) + bt (intelligence) + updoots (discovery) +
tpane (workspace automation). All Go + Bubble Tea. All sms's tools.

## Recommended Session Plan

### Session A: --robot-* smoke test (1 session, ~1 hour)

1. Build bt: `go build ./cmd/bt/`
2. Run every --robot-* command against bt's own beads data
3. Categorize: works / errors / empty / needs-investigation
4. Write results to `docs/audit/robot-commands-dolt-compat.md`
5. Update bt-79eg and bt-pfic beads with findings

### Session B: Fix the known bugs (1 session)

1. Fix due_at bug in Dolt reader
2. Add defer_until and pinned to Dolt reader and model
3. Run tests, verify
4. This is a small PR - could go to seanmartinsmith/beadstui

### Session C: Full codebase audit (use existing v2 plan)

With the --robot-* smoke test results and bug fixes done, the full 9-team
audit (from codebase-audit-plan-v2.md) can proceed with better orientation.
The smoke test tells you which features are alive vs dead.

### Session D+: Global beads prototype

Extend bt's --workspace to read from multiple local .beads/dolt/ databases.
Add --robot-triage --global flag. This is the minimum viable cross-project
view.
