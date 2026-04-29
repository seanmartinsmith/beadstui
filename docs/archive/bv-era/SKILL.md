---
name: bt
description: "Beads TUI - Graph-aware triage engine for Beads projects. Computes PageRank, betweenness, critical path, and cycles. Use --robot-* flags for AI agents."
---

# BT - Beads TUI

A graph-aware triage engine for Beads projects (`.beads/beads.jsonl`). Computes 9 graph metrics, generates execution plans, and provides deterministic recommendations. Human TUI for browsing; robot flags for AI agents.

## Why BT vs Raw Beads

| Capability | Raw beads.jsonl | BT Robot Mode |
|------------|-----------------|---------------|
| Query | "List all issues" | "List the top 5 bottlenecks blocking the release" |
| Context Cost | High (linear with issue count) | Low (fixed summary struct) |
| Graph Logic | Agent must compute | Pre-computed (PageRank, betweenness, cycles) |
| Safety | Agent might miss cycles | Cycles explicitly flagged |

Use BT instead of parsing beads.jsonl directly. It computes graph metrics deterministically.

## CRITICAL: Robot Mode for Agents

**Never run bare `bt`**. It launches an interactive TUI that blocks your session.

Always use `--robot-*` flags:

```bash
bt --robot-triage        # THE MEGA-COMMAND: start here
bt --robot-next          # Minimal: just the single top pick
bt --robot-plan          # Parallel execution tracks
bt --robot-insights      # Full graph metrics
```

## The 9 Graph Metrics

BT computes these metrics to surface hidden project dynamics:

| Metric | What It Measures | Key Insight |
|--------|------------------|-------------|
| **PageRank** | Recursive dependency importance | Foundational blockers |
| **Betweenness** | Shortest-path traffic | Bottlenecks and bridges |
| **HITS** | Hub/Authority duality | Epics vs utilities |
| **Critical Path** | Longest dependency chain | Keystones with zero slack |
| **Eigenvector** | Influence via neighbors | Strategic dependencies |
| **Degree** | Direct connection counts | Immediate blockers/blocked |
| **Density** | Edge-to-node ratio | Project coupling health |
| **Cycles** | Circular dependencies | Structural errors (must fix!) |
| **Topo Sort** | Valid execution order | Work queue foundation |

## Two-Phase Analysis

BT uses async computation with timeouts:

- **Phase 1 (instant):** degree, topo sort, density
- **Phase 2 (500ms timeout):** PageRank, betweenness, HITS, eigenvector, cycles

Always check `status` field in output. For large graphs (>500 nodes), some metrics may be `approx` or `skipped`.

## Robot Commands Reference

### Triage & Planning

```bash
bt --robot-triage              # Full triage: recommendations, quick_wins, blockers_to_clear
bt --robot-next                # Single top pick with claim command
bt --robot-plan                # Parallel execution tracks with unblocks lists
bt --robot-priority            # Priority misalignment detection
```

### Graph Analysis

```bash
bt --robot-insights            # Full metrics: PageRank, betweenness, HITS, cycles, etc.
bt --robot-label-health        # Per-label health: healthy|warning|critical
bt --robot-label-flow          # Cross-label dependency flow matrix
bt --robot-label-attention     # Attention-ranked labels
```

### History & Changes

```bash
bt --robot-history             # Bead-to-commit correlations
bt --robot-diff --diff-since <ref>  # Changes since ref
```

### Other Commands

```bash
bt --robot-burndown <sprint>   # Sprint burndown, scope changes
bt --robot-forecast <id|all>   # ETA predictions
bt --robot-alerts              # Stale issues, blocking cascades
bt --robot-suggest             # Hygiene: duplicates, missing deps, cycle breaks
bt --robot-graph               # Dependency graph export (JSON, DOT, Mermaid)
bt --export-graph <file.html>  # Self-contained interactive HTML visualization
```

## Scoping & Filtering

```bash
bt --robot-plan --label backend              # Scope to label's subgraph
bt --robot-insights --as-of HEAD~30          # Historical point-in-time
bv --recipe actionable --robot-plan          # Pre-filter: ready to work
bv --recipe high-impact --robot-triage       # Pre-filter: top PageRank
bt --robot-triage --robot-triage-by-track    # Group by parallel work streams
bt --robot-triage --robot-triage-by-label    # Group by domain
```

## Built-in Recipes

| Recipe | Purpose |
|--------|---------|
| `default` | All open issues sorted by priority |
| `actionable` | Ready to work (no blockers) |
| `high-impact` | Top PageRank scores |
| `blocked` | Waiting on dependencies |
| `stale` | Open but untouched for 30+ days |
| `triage` | Sorted by computed triage score |
| `quick-wins` | Easy P2/P3 items with no blockers |
| `bottlenecks` | High betweenness nodes |

## Robot Output Structure

All robot JSON includes:
- `data_hash` - Fingerprint of beads.jsonl (verify consistency)
- `status` - Per-metric state: `computed|approx|timeout|skipped`
- `as_of` / `as_of_commit` - Present when using `--as-of`

### --robot-triage Output

```json
{
  "quick_ref": { "open": 45, "blocked": 12, "top_picks": [...] },
  "recommendations": [
    { "id": "bd-123", "score": 0.85, "reason": "Unblocks 5 tasks", "unblock_info": {...} }
  ],
  "quick_wins": [...],
  "blockers_to_clear": [...],
  "project_health": { "distributions": {...}, "graph_metrics": {...} },
  "commands": { "claim": "bd claim bd-123", "view": "bv --bead bd-123" }
}
```

### --robot-insights Output

```json
{
  "bottlenecks": [{ "id": "bd-123", "value": 0.45 }],
  "keystones": [{ "id": "bd-456", "value": 12.0 }],
  "influencers": [...],
  "hubs": [...],
  "authorities": [...],
  "cycles": [["bd-A", "bd-B", "bd-A"]],
  "clusterDensity": 0.045,
  "status": { "pagerank": "computed", "betweenness": "computed", ... }
}
```

## jq Quick Reference

```bash
bt --robot-triage | jq '.quick_ref'                        # At-a-glance summary
bt --robot-triage | jq '.recommendations[0]'               # Top recommendation
bt --robot-plan | jq '.plan.summary.highest_impact'        # Best unblock target
bt --robot-insights | jq '.status'                         # Check metric readiness
bt --robot-insights | jq '.cycles'                         # Circular deps (must fix!)
bt --robot-label-health | jq '.results.labels[] | select(.health_level == "critical")'
```

## Agent Workflow Pattern

```bash
# 1. Start with triage
TRIAGE=$(bt --robot-triage)
NEXT_TASK=$(echo "$TRIAGE" | jq -r '.recommendations[0].id')

# 2. Check for cycles first (structural errors)
CYCLES=$(bt --robot-insights | jq '.cycles')
if [ "$CYCLES" != "[]" ]; then
  echo "Fix cycles first: $CYCLES"
fi

# 3. Claim the task
bd claim "$NEXT_TASK"

# 4. Work on it...

# 5. Close when done
bd close "$NEXT_TASK"
```

## TUI Views (for Humans)

When running `bt` interactively (not for agents):

| Key | View |
|-----|------|
| `l` | List view (default) |
| `b` | Kanban board |
| `g` | Graph view (dependency DAG) |
| `E` | Tree view (parent-child hierarchy) |
| `i` | Insights dashboard (6-panel metrics) |
| `h` | History view (bead-to-commit correlation) |
| `a` | Actionable plan (parallel tracks) |
| `f` | Flow matrix (cross-label dependencies) |
| `]` | Attention view (label priority ranking) |

## Integration with bd CLI

BV reads from `.beads/beads.jsonl` created by the `bd` CLI:

```bash
bd init                    # Initialize beads in project
bd create "Task title"     # Create a bead
bd list                    # List beads
bd ready                   # Show actionable beads
bd claim bd-123            # Claim a bead
bd close bd-123            # Close a bead
```

## Integration with Agent Mail

Use bead IDs as thread IDs for coordination:

```
file_reservation_paths(..., reason="bd-123")
send_message(..., thread_id="bd-123", subject="[bd-123] Starting...")
```

## Graph Export Formats

```bash
bt --robot-graph                              # JSON (default)
bt --robot-graph --graph-format=dot           # Graphviz DOT
bt --robot-graph --graph-format=mermaid       # Mermaid diagram
bt --robot-graph --graph-root=bd-123 --graph-depth=3  # Subgraph
bt --export-graph report.html                 # Interactive HTML
```

## Time Travel

Compare against historical states:

```bash
bv --as-of HEAD~10                    # 10 commits ago
bv --as-of v1.0.0                     # At tag
bv --as-of "2024-01-15"               # At date
bt --robot-diff --diff-since HEAD~30  # Changes in last 30 commits
```

## Common Pitfalls

| Issue | Fix |
|-------|-----|
| TUI blocks agent | Use `--robot-*` flags only |
| Stale metrics | Check `status` field, results cached by `data_hash` |
| Missing cycles | Run `--robot-insights`, check `.cycles` |
| Wrong recommendations | Use `--recipe actionable` to filter to ready work |

## Performance Notes

- Phase 1 metrics (degree, topo, density): instant
- Phase 2 metrics (PageRank, betweenness, etc.): 500ms timeout
- Results cached by `data_hash`
- Prefer `--robot-plan` over `--robot-insights` when speed matters
