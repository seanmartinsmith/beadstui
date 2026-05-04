# Robot Mode Reference

> `bt`'s agent-facing API. All `bt robot <subcmd>` invocations emit deterministic JSON to stdout. Errors go to stderr. Exit codes follow standard Unix conventions (0 = success, non-zero = error).
>
> **CRITICAL**: bare `bt` launches an interactive TUI that blocks a session. Always use `bt robot <subcmd>`.

## Conventions

**Output format**: JSON to stdout by default. Pass `--format toon` for token-optimized TOON notation (~30-50% fewer tokens). Controlled by `BT_OUTPUT_FORMAT`.

**Output shape**: Two projections available via `--shape compact|full` (or aliases `--compact`/`--full`). Default is `compact`. Controlled by `BT_OUTPUT_SHAPE`.
- `compact` (schema `compact.v1`): index projection - `id`, `title`, `status`, `priority`, `type`, `labels`, relationship counts. Envelope carries `"schema": "compact.v1"`. Drill in via `bd show <id>`.
- `full`: pre-compact shape with `description`, `design`, `acceptance_criteria`, `notes`, `comments`, `close_reason`. Envelope omits `schema` field.

**Errors**: human-readable message to stderr; non-zero exit code.

**Two-phase analysis**: Phase 1 (degree, topo sort, density, k-core, articulation, slack) is instant. Phase 2 (PageRank, betweenness, HITS, eigenvector, cycles) runs async with timeouts - check `status` flags in output to see which metrics were computed vs. skipped.

## Common Flags

These flags apply to all `bt robot` subcommands unless noted otherwise:

| Flag | Description |
|---|---|
| `--label <name>` | Scope analysis to a label's subgraph |
| `--recipe <name>` | Apply named recipe filter (see `bt robot recipes`) |
| `--as-of <ref>` | View state at a point in time (commit SHA, branch, tag, or date) |
| `--bql <query>` | BQL query to pre-filter issues before analysis |
| `--shape compact\|full` | Output shape; aliases `--compact` / `--full` |
| `--format json\|toon` | Output format (default: json) |
| `--global` | Show issues from all projects on shared Dolt server |
| `--source <prefix,...>` | Filter by source project ID prefix (e.g. `bt,cass`) |
| `--repo <prefix>` | Filter by repository prefix |
| `--diff-since <ref>` | Show changes since historical point |
| `--history-limit <n>` | Max commits to analyze for correlations (default 500) |
| `--force-full-analysis` | Compute all metrics regardless of graph size |
| `--stats` | Show JSON vs TOON token estimates on stderr |
| `--workspace <path>` | Load issues from workspace config file (`.bt/workspace.yaml`) |

## Environment Variables

Key variables that affect robot mode behavior:

| Variable | Description |
|---|---|
| `BT_OUTPUT_FORMAT` | Default output format: `json` or `toon` |
| `BT_OUTPUT_SHAPE` | Default output shape: `compact` or `full` |
| `BT_NO_BROWSER` | Set to `1` to suppress browser-opening |
| `BT_TEST_MODE` | Set to `1` for test-mode guards |
| `BT_DEBUG` | Set to `1` to enable debug logging to stderr |
| `BT_PRETTY_JSON` | Set to `1` for indented JSON output |
| `BT_ROBOT` | Set to `1` to force robot mode (clean stdout) |
| `BT_INSIGHTS_MAP_LIMIT` | Per-map size limit in `insights` output |
| `BT_OUTPUT_SCHEMA` | Default schema for `pairs` and `refs`: `v1` or `v2` |
| `BT_SIGIL_MODE` | Default sigil mode for `refs --schema=v2`: `strict`, `verb`, or `permissive` |
| `BT_SEARCH_MODE` | Search ranking mode: `text` or `hybrid` |
| `BT_SEARCH_PRESET` | Hybrid search preset name |
| `TOON_DEFAULT_FORMAT` | Fallback format if `BT_OUTPUT_FORMAT` not set |
| `TOON_STATS` | Set to `1` to show token estimates on stderr |

Run `bt robot docs env` to see the full variable list from the binary.

---

## Top-level Subcommands

### bt robot triage

**Purpose**: Unified triage - the mega-command for AI agents. Ranked recommendations, quick wins, blockers, and a project-health snapshot. The primary entry point when an agent starts a session or needs to orient itself.

**Unique flags**:
- `--by-label`: Group triage recommendations by label
- `--by-track`: Group triage recommendations by execution track (for multi-agent coordination)

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `generated_at` | string (ISO 8601) | Timestamp |
| `data_hash` | string | Hash of input data (cache key) |
| `triage.meta` | object | Version, timing, issue count, phase2 readiness |
| `triage.quick_ref` | object | Fast summary: open/blocked/actionable counts, top 3 picks |
| `triage.recommendations` | array | Full scored list with `id`, `title`, `score`, `action`, `reasons`, `breakdown` |
| `triage.quick_wins` | array | High-unblocking-ratio items: `id`, `title`, `score`, `reason`, `unblocks_ids` |
| `triage.blockers_to_clear` | array | Items blocking the most downstream work |
| `triage.project_health` | object | Counts by status/type/priority, graph metrics, velocity |
| `triage.commands` | object | Ready-to-run `bd` commands for the top pick |
| `usage_hints` | array | `jq` snippets for common agent queries |

When `--by-track` is set, adds `triage.recommendations_by_track[]` (array of tracks, each with `top_pick` and `items`).
When `--by-label` is set, adds `triage.recommendations_by_label[]` (array of label groups).

**Examples**:
```bash
bt robot triage
bt robot triage --label area:tui
bt robot triage --recipe actionable
bt robot triage --by-track
bt robot triage --by-label
bt robot triage --global
```

**Sample output (truncated)**:
```json
{
  "generated_at": "2026-05-03T22:26:37Z",
  "data_hash": "fc65cfb431469765",
  "triage": {
    "meta": { "version": "1.0.0", "phase2_ready": true, "issue_count": 3576, "compute_time_ms": 7 },
    "quick_ref": {
      "open_count": 1078, "actionable_count": 972, "blocked_count": 106, "in_progress_count": 19,
      "top_picks": [{ "id": "bt-53du", "title": "Product vision: bt v1 (epic)", "score": 0.548, "unblocks": 12 }]
    },
    "recommendations": [{ "id": "bt-53du", "score": 0.548, "action": "Quick win - start here for fast progress", "reasons": [...] }],
    "quick_wins": [...],
    "blockers_to_clear": [...],
    "project_health": { "counts": {...}, "graph": {...}, "velocity": {...} }
  },
  "usage_hints": ["jq '.triage.quick_ref.top_picks[:3]' - Top 3 picks for immediate work"]
}
```

---

### bt robot next

**Purpose**: Single top-pick recommendation with a ready-to-run claim command. Use when an agent needs to start work immediately without analyzing the full triage list.

**Unique flags**: none

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `id` | string | Bead ID |
| `title` | string | Bead title |
| `score` | float | Triage score |
| `reasons` | array | Human-readable scoring reasons |
| `unblocks` | int | Count of directly unblocked downstream issues |
| `claim_command` | string | Ready-to-run `bd update <id> --status=in_progress` |
| `show_command` | string | Ready-to-run `bd show <id>` |

**Examples**:
```bash
bt robot next
bt robot next --label area:tui
bt robot next --global
```

**Sample output**:
```json
{
  "generated_at": "2026-05-03T22:26:42Z",
  "id": "bt-53du",
  "title": "Product vision: bt v1 (epic)",
  "score": 0.548,
  "reasons": ["Completing this unblocks 12 downstream issues", "High centrality (PageRank: 100%)"],
  "unblocks": 12,
  "claim_command": "bd update bt-53du --status=in_progress",
  "show_command": "bd show bt-53du"
}
```

---

### bt robot plan

**Purpose**: Dependency-respecting execution plan partitioned into parallel execution tracks. Use this to coordinate multiple agents or to get a topologically sorted work queue.

**Unique flags**: none

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `status` | object | Per-metric computation status (phase2 metrics largely skipped for plan) |
| `plan.tracks` | array | Parallel tracks; each has `track_id`, `items[]`, `reason` |
| `plan.tracks[].items[]` | array | Issues with `id`, `title`, `priority`, `status`, `unblocks` |

**Note**: Phase 2 metrics (PageRank, betweenness, HITS) are skipped for plan generation - not needed for topological scheduling.

**Examples**:
```bash
bt robot plan
bt robot plan --label area:tui
bt robot plan --recipe actionable
bt robot plan --global
```

**Sample output (truncated)**:
```json
{
  "plan": {
    "tracks": [
      {
        "track_id": "track-A",
        "reason": "Independent work stream",
        "items": [
          { "id": "bd-0il", "title": "Beads feature discovery gap", "priority": 1, "status": "in_progress", "unblocks": ["mkt-0il"] }
        ]
      }
    ]
  }
}
```

---

### bt robot priority

**Purpose**: Priority misalignment detection. Compares current priorities against graph-computed impact scores and suggests where priorities should be adjusted up or down.

**Unique flags**: none

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `status` | object | Phase 2 computation status |
| `recommendations` | array | Priority adjustment suggestions |
| `recommendations[].issue_id` | string | Bead ID |
| `recommendations[].current_priority` | int | Current priority (0=P0) |
| `recommendations[].suggested_priority` | int | Graph-computed suggestion |
| `recommendations[].direction` | string | `increase` or `decrease` |
| `recommendations[].impact_score` | float | Composite impact score |
| `recommendations[].confidence` | float | Confidence 0-1 |
| `recommendations[].reasoning` | array | Human-readable factors |
| `recommendations[].what_if` | object | Impact projection: direct/transitive unblocks, days saved |
| `recommendations[].explanation` | object | Top 3 scoring factors with weights |
| `field_descriptions` | object | Explains each output field |
| `summary` | object | Counts: total issues, recommendations, high-confidence |

**Examples**:
```bash
bt robot priority
bt robot priority --label area:tui
bt robot priority --global
```

---

### bt robot insights

**Purpose**: Full graph analysis and metrics. Most expensive command - runs all phase 2 algorithms. Use for deep structural analysis, not for routine agent orientation.

**Unique flags**: none (uses `--force-full-analysis` to override size limits)

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `status` | object | Per-metric computation status with `state` (`computed`/`skipped`/`approximate`), `ms`, `reason` |
| `Bottlenecks` | array | Issues with highest betweenness centrality: `ID`, `Value` |
| `PageRank` | map | Per-issue PageRank scores |
| `HITS` | object | Authority and hub scores |
| `KCore` | object | K-core decomposition |
| `ArticulationPoints` | array | Bridge nodes whose removal disconnects the graph |
| `CriticalPath` | object | Critical path analysis |
| `Slack` | map | Slack values per issue (0 = on critical path) |

**Note**: For large graphs (>2000 nodes), cycles are skipped. Betweenness uses approximate sampling when the graph is large. Check `status` before relying on any metric.

**Examples**:
```bash
bt robot insights
bt robot insights --label area:tui
bt robot insights --force-full-analysis
bt robot insights --global
```

---

### bt robot alerts

**Purpose**: Stale issue detection and blocking cascade alerts. Two alert types: `stale` (in-progress or open issues with no activity) and `blocking_cascade` (high-fan-out blockers).

**Unique flags**: none

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `alerts` | array | Alert objects |
| `alerts[].type` | string | `stale` or `blocking_cascade` |
| `alerts[].severity` | string | `warning` or `critical` |
| `alerts[].message` | string | Human-readable alert message |
| `alerts[].issue_id` | string | Affected bead ID |
| `alerts[].details` | array | Extra context strings |
| `alerts[].detected_at` | string | ISO 8601 timestamp |
| `alerts[].source_project` | string | Project scope |

**Examples**:
```bash
bt robot alerts
bt robot alerts --label area:tui
bt robot alerts --global
```

**Sample output (truncated)**:
```json
{
  "alerts": [
    {
      "type": "stale",
      "severity": "warning",
      "message": "Issue bd-edi inactive for 5 days",
      "issue_id": "bd-edi",
      "details": ["status=in_progress", "last_update=2026-04-28T21:15:51Z"]
    }
  ]
}
```

---

### bt robot diff

**Purpose**: Changes since a historical reference point - new issues, closed issues, status changes. Use for "what changed since I last ran?" workflows.

**Unique flags**:
- `--since <ref>`: Historical reference (commit SHA, branch, tag, date). Also aliased as global `--diff-since`.

**Note**: Requires `--since` (or `--diff-since`). Needs a `.beads/` directory with JSONL data at the historical ref. Will error if the historical ref doesn't have accessible beads data.

**Examples**:
```bash
bt robot diff --diff-since HEAD~10
bt robot diff --since main
bt robot diff --diff-since 2026-04-01
```

---

### bt robot forecast

**Purpose**: ETA predictions for one bead or all open beads. Uses velocity data (label-specific closures per day) plus graph depth to estimate completion dates.

**Unique flags**:
- `--agents <n>`: Number of parallel agents (default 1)
- `--forecast-label <name>`: Filter forecast by label
- `--forecast-sprint <id>`: Filter forecast by sprint ID

**Positional arg**: `[bead-id|all]` - single bead ID or `all`

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `agents` | int | Agent count used in simulation |
| `forecast_count` | int | Number of forecasted issues |
| `forecasts` | array | Per-issue forecasts |
| `forecasts[].issue_id` | string | Bead ID |
| `forecasts[].estimated_minutes` | int | Estimated effort in minutes |
| `forecasts[].estimated_days` | int | Calendar days to completion |
| `forecasts[].eta_date` | string | Point estimate (ISO 8601) |
| `forecasts[].eta_date_low` | string | Optimistic bound |
| `forecasts[].eta_date_high` | string | Pessimistic bound |
| `forecasts[].confidence` | float | Confidence 0-1 |
| `forecasts[].velocity_minutes_per_day` | float | Label velocity used |
| `forecasts[].factors` | array | Explanation strings |

**Examples**:
```bash
bt robot forecast bt-53du
bt robot forecast all --agents 3
bt robot forecast all --forecast-label area:tui
```

**Sample output**:
```json
{
  "agents": 1,
  "forecast_count": 1,
  "forecasts": [
    {
      "issue_id": "bt-iigg",
      "estimated_minutes": 120,
      "estimated_days": 4,
      "eta_date": "2026-05-07T18:31:42Z",
      "eta_date_low": "2026-05-06T07:58:06Z",
      "eta_date_high": "2026-05-09T05:05:18Z",
      "confidence": 0.55,
      "velocity_minutes_per_day": 30,
      "factors": ["estimate: median (60m)", "type: task×1.0", "velocity: label=area:docs (30 min/day, 15 samples/30d)"]
    }
  ]
}
```

---

### bt robot suggest

**Purpose**: Hygiene suggestions - potential duplicates, dependency cycles, missing links. Use periodically to keep the issue graph clean.

**Unique flags**: none

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `suggestions.suggestions` | array | Suggestion objects |
| `suggestions.suggestions[].type` | string | `potential_duplicate`, `cycle_warning`, `missing_dependency`, etc. |
| `suggestions.suggestions[].target_bead` | string | Primary bead ID |
| `suggestions.suggestions[].related_bead` | string | Related bead ID |
| `suggestions.suggestions[].summary` | string | One-line description |
| `suggestions.suggestions[].reason` | string | Why flagged |
| `suggestions.suggestions[].confidence` | float | Confidence 0-1 |
| `suggestions.suggestions[].action_command` | string | Optional ready-to-run fix command |

**Examples**:
```bash
bt robot suggest
bt robot suggest --global
bt robot suggest --label area:tui
```

---

### bt robot list

**Purpose**: Simple flag-based filtered issue list. For complex queries use `bt robot bql`. Produces a flat array of compact issue objects.

**Unique flags**:
- `--status <value>`: Filter by status (`open`, `blocked`, `in_progress`, `closed`; comma-separated)
- `--priority <value>`: Filter by priority: single (`0`) or range (`0-1`)
- `--type <value>`: Filter by issue type: `bug`, `feature`, `task`, `epic`, `chore`
- `--has-label <name>`: Filter to issues with this exact label
- `--limit <n>`: Max results (default 100; 0 = unlimited)

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `schema` | string | `compact.v1` (compact shape) |
| `query` | object | Applied filters echo |
| `total` | int | Total matching issues (pre-limit) |
| `count` | int | Issues returned |
| `truncated` | bool | Whether `limit` was hit |
| `issues` | array | Compact issue objects |

**Compact issue fields**: `id`, `title`, `status`, `priority`, `issue_type`, `labels`, `source_repo`, `parent_id`, `blockers_count`, `unblocks_count`, `children_count`, `relates_count`, `is_blocked`, `created_at`, `updated_at`, `created_by_session`.

**Examples**:
```bash
bt robot list --status open --priority 0-1
bt robot list --type bug --status open
bt robot list --has-label area:tui --limit 50
bt robot list --global --status in_progress
```

**Sample output**:
```json
{
  "schema": "compact.v1",
  "total": 1052,
  "truncated": true,
  "count": 3,
  "issues": [
    { "id": "bt-4ew7", "title": "Multi-select bead IDs in list view", "status": "open", "priority": 3, "issue_type": "feature", "labels": ["area:tui"], "blockers_count": 0, "unblocks_count": 0 }
  ]
}
```

---

### bt robot bql

**Purpose**: BQL (Beads Query Language) filtered issues. More expressive than `bt robot list` - use for complex filters involving multiple conditions.

**Unique flags**:
- `--query <bql>`: BQL query string
- `--limit <n>`: Max issues to return (0 = unlimited)
- `--offset <n>`: Skip first N issues (for pagination)

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `query` | string | The BQL query string that was executed |
| `total_count` | int | Full match set size before `--limit` / `--offset` is applied |
| `count` | int | Number of issues actually returned in the `issues` array |
| `offset` | int | Offset that was applied (omitted when 0) |
| `issues` | array | Returned issues (after offset + limit windowing) |

**Note**: BQL syntax is documented in code; a dedicated reference is tracked in bt-01pk. Also accepts `--bql` global flag as a pre-filter on top of the local `--query`. When run outside a beads project (no local `.beads/`), the error message will suggest `--global` to query the global Dolt server.

**Examples**:
```bash
bt robot bql --query "status=open priority<=1"
bt robot bql --query "type=bug label=area:tui"
bt robot bql --query "status=open blocked=false"
bt robot bql --query "status=open" --limit 25 --offset 50  # page 3 of 25
```

---

### bt robot search

**Purpose**: Semantic and hybrid text search over bead titles and descriptions. Returns relevance-ranked results.

**Unique flags**:
- `--query <text>`: Search query (required)
- `--limit <n>`: Max results (default 10)
- `--mode <text|hybrid>`: Ranking mode
- `--preset <name>`: Hybrid search preset
- `--weights <json>`: Hybrid weights JSON

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `query` | string | Query string |
| `provider` | string | Embedding provider (e.g. `hash`) |
| `mode` | string | `text` or `hybrid` |
| `index` | object | Index stats: `total`, `added`, `updated`, `removed`, `embedded` |
| `limit` | int | Max results |
| `results` | array | Ranked hits |
| `results[].issue_id` | string | Bead ID |
| `results[].score` | float | Relevance score 0-1 |
| `results[].title` | string | Bead title |

**Examples**:
```bash
bt robot search --query "robot mode schema"
bt robot search --query "authentication bug" --limit 5
bt robot search --query "dependency graph" --mode hybrid
```

**Sample output**:
```json
{
  "query": "robot mode schema",
  "mode": "text",
  "results": [
    { "issue_id": "bt-mhwy.1", "score": 0.490, "title": "Compact output mode across robot subcommands" },
    { "issue_id": "bt-0zk6",   "score": 0.481, "title": "E2E: Robot command matrix - comprehensive coverage" }
  ]
}
```

---

### bt robot graph

**Purpose**: Dependency graph as JSON adjacency list, DOT format, or Mermaid diagram. Use for programmatic graph traversal or visualization.

**Unique flags**:
- `--graph-format <json|dot|mermaid>`: Output format (default `json`)
- `--graph-root <id>`: Subgraph rooted at a specific issue
- `--graph-depth <n>`: Max depth from root (0 = unlimited)

**Top-level fields (JSON format)**:
| Field | Type | Description |
|---|---|---|
| `format` | string | `json` |
| `nodes` | int | Total node count |
| `edges` | int | Total edge count |
| `filters_applied` | object | Active filters |
| `explanation` | object | Usage guidance |
| `adjacency.nodes` | array | Issue nodes with `id`, `title`, `status`, `priority`, `labels`, `pagerank` |
| `adjacency.edges` | array | Directed edges (null when no edges) |

**Examples**:
```bash
bt robot graph
bt robot graph --graph-root bt-53du --graph-depth 2
bt robot graph --graph-format mermaid
bt robot graph --graph-format dot | dot -Tpng > graph.png
```

---

### bt robot related

**Purpose**: Find beads related to a specific bead by graph proximity, label overlap, and semantic similarity.

**Unique flags**:
- `--include-closed`: Include closed beads in results
- `--max-results <n>`: Max results per category (default 10)
- `--min-relevance <0-100>`: Minimum relevance score (default 20)

**Positional arg**: `<bead-id>` (required)

**Note**: Requires beads data accessible to bt. When using worktrees or non-workspace directories, pass `--global`. When run outside a beads project (no local `.beads/`), the error message will suggest `--global` to query the global Dolt server.

**Examples**:
```bash
bt robot related bt-53du
bt robot related bt-iigg --global --include-closed
bt robot related bt-53du --max-results 5 --min-relevance 40
```

---

### bt robot blocker-chain

**Purpose**: Full blocker chain analysis for a specific bead - traces the complete path from root blockers to the target issue, identifying whether the issue is currently actionable.

**Positional arg**: `<bead-id>` (required)

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `result.target_id` | string | Target bead ID |
| `result.is_blocked` | bool | Whether currently blocked |
| `result.chain_length` | int | Depth of blocking chain |
| `result.root_blockers` | array | Issues at the root of the block chain |
| `result.chain` | array | Full chain with `id`, `depth`, `is_root`, `actionable`, `blocks_count` |
| `result.has_cycle` | bool | Whether a dependency cycle was detected |

**Examples**:
```bash
bt robot blocker-chain bt-53du
bt robot blocker-chain bt-dcby --global
```

**Sample output**:
```json
{
  "result": {
    "target_id": "bt-53du",
    "is_blocked": false,
    "chain_length": 0,
    "root_blockers": [],
    "chain": [{ "id": "bt-53du", "depth": 0, "is_root": true, "actionable": true, "blocks_count": 0 }],
    "has_cycle": false
  }
}
```

---

### bt robot causality

**Purpose**: Causal chain analysis - traces the transitive cause-and-effect relationships from a bead outward. Requires `--global` for cross-project data.

**Positional arg**: `<bead-id>` (required)

**Note**: Requires `--global` when run outside a workspace (no local `.beads/` project), because causality analysis needs to resolve cross-project dependency chains from the shared Dolt server.

**Examples**:
```bash
bt robot causality bt-53du --global
```

---

### bt robot impact

**Purpose**: Analyzes the impact of modifying specific files - which open beads have touched those files and are therefore likely affected by changes.

**Positional arg**: `[paths]` - comma-separated file paths

**Examples**:
```bash
bt robot impact pkg/view/compact.go
bt robot impact pkg/view/compact.go,pkg/model/issue.go
```

---

### bt robot impact-network

**Purpose**: Full bead impact network - shows cluster structure, connectivity, and change propagation risk across the entire issue graph.

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `depth` | int | Analysis depth |
| `network.nodes` | map | Per-bead nodes with `degree`, `cluster_id`, `commit_count`, `file_count`, `connectivity` |
| `network.edges` | array | Impact edges |

**Examples**:
```bash
bt robot impact-network --global
bt robot impact-network --label area:tui
```

---

### bt robot drift

**Purpose**: Drift detection - checks for priority drift, status drift, and stale-in-progress issues compared to a saved baseline. Requires a saved baseline (see `bt robot baseline save`).

**Note**: Errors with "No baseline found" if no baseline has been saved for the current project.

**Examples**:
```bash
bt robot drift
bt robot drift --global
```

---

### bt robot metrics

**Purpose**: Runtime performance metrics - memory usage, GC stats. Useful for monitoring bt's resource consumption in long-running agent sessions.

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `memory.heap_alloc_mb` | float | Allocated heap (MB) |
| `memory.heap_sys_mb` | float | System heap (MB) |
| `memory.heap_objects_k` | float | Heap objects (thousands) |
| `memory.gc_cycles` | int | GC cycles |
| `memory.gc_pause_ms` | float | Last GC pause (ms) |
| `memory.goroutine_count` | int | Live goroutines |

**Examples**:
```bash
bt robot metrics
```

---

### bt robot orphans

**Purpose**: Commits that don't correlate to any known bead - candidates for retroactive tagging or cleanup. Requires git history + beads data.

**Note**: Requires beads data in `.beads/` directory. Use `--global` for cross-project scope. When run outside a beads project (no local `.beads/`), the error message will suggest `--global` to query the global Dolt server.

**Examples**:
```bash
bt robot orphans --global
bt robot orphans --history-limit 100
```

---

### bt robot history

**Purpose**: Bead-to-commit correlation output - which commits are linked to which beads. Requires JSONL beads data (not Dolt-only installs) or global mode.

**Note**: Requires accessible beads data. Will error if `.beads/` is not available in the local directory. When run outside a beads project (no local `.beads/`), the error message will suggest `--global` to query the global Dolt server.

**Examples**:
```bash
bt robot history --global
bt robot history --history-limit 200
```

---

### bt robot pairs

**Purpose**: Cross-project paired beads detection - finds beads with the same ID suffix across different project prefixes (e.g. `bt-zsy8` + `bd-zsy8`). Always requires `--global`.

**Unique flags**: `--schema v1|v2` (via env `BT_OUTPUT_SCHEMA`)

**Top-level fields** (schema `pair.v2`):
| Field | Type | Description |
|---|---|---|
| `schema` | string | `pair.v2` |
| `pairs` | array | Detected pairs |
| `pairs[].suffix` | string | Shared ID suffix |
| `pairs[].canonical` | object | Primary bead: `id`, `title`, `status`, `priority`, `source_repo` |
| `pairs[].mirrors` | array | Paired beads in other projects |
| `pairs[].drift` | array | Detected drift: `status`, `priority`, `closed_open` |
| `pairs[].intent_source` | string | How the pair was detected (`dep` = explicit dependency) |

**Note**: Requires `--global`. Will error without it: "bt robot pairs requires --global".

**Examples**:
```bash
bt robot pairs --global
bt robot pairs --global --source bt,bd
```

---

### bt robot refs

**Purpose**: Cross-project bead references in prose and dependency fields - finds bead IDs mentioned in descriptions/comments that cross project boundaries. Requires `--global`.

**Unique flags**: `--schema v1|v2` and `--sigils <mode>` (via env `BT_OUTPUT_SCHEMA` / `BT_SIGIL_MODE`)

**Top-level fields** (schema `ref.v2`):
| Field | Type | Description |
|---|---|---|
| `schema` | string | `ref.v2` |
| `sigil_mode` | string | `verb`, `strict`, or `permissive` |
| `refs` | array | Detected references |
| `refs[].source` | string | Bead containing the reference |
| `refs[].target` | string | Referenced bead ID |
| `refs[].location` | string | Where found: `description`, `comments`, `deps` |
| `refs[].flags` | array | `cross_project`, `stale`, `broken` |
| `refs[].sigil_kind` | string | How detected: `verb`, `inline_code`, etc. |

**Examples**:
```bash
bt robot refs --global
bt robot refs --global --source bt
```

---

### bt robot recipes

**Purpose**: Lists all available named recipes with descriptions. Recipes are named filters usable with `--recipe` on any robot subcommand.

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `recipes` | array | Available recipes |
| `recipes[].name` | string | Recipe name |
| `recipes[].description` | string | What it selects |
| `recipes[].source` | string | `builtin` |

**Built-in recipes**: `actionable`, `blocked`, `bottlenecks`, `closed`, `default`, `high-impact`, `quick-wins`, `recent`, `release-cut`, `stale`, `triage`.

**Examples**:
```bash
bt robot recipes
```

---

### bt robot schema

**Purpose**: JSON Schema definitions for all robot commands (or a specific one). Use for agent introspection - know what fields to expect before calling.

**Unique flags**:
- `--command <name>`: Output schema for specific command only. Accepts both the cobra path form (e.g. `triage`, **preferred**) and the legacy `robot-` prefixed form (e.g. `robot-triage`). The cobra path form mirrors the actual CLI subcommand name.

**Available command names for `--command`**: `triage`, `next`, `plan`, `insights`, `priority`, `alerts`, `suggest`, `diff`, `forecast`, `graph`, `burndown`, `pairs`, `refs` (each also accepted with the legacy `robot-` prefix).

**Examples**:
```bash
bt robot schema
bt robot schema --command triage           # preferred (cobra path form)
bt robot schema --command robot-triage     # legacy form, still works
bt robot schema --command insights
```

---

### bt robot docs

**Purpose**: Machine-readable JSON documentation for AI agents covering usage guide, command list, examples, environment variables, exit codes, or all topics combined.

**Unique flags**:
- `--topic <topic>`: One of `guide`, `commands`, `examples`, `env`, `exit-codes`, `all`

Positional arg also accepted: `bt robot docs <topic>`

**Examples**:
```bash
bt robot docs guide
bt robot docs env
bt robot docs exit-codes
bt robot docs all
```

---

### bt robot burndown

**Purpose**: Burndown chart data for a sprint. Returns time-series issue counts for plotting velocity over time.

**Positional arg**: `[sprint-id|current]`

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `sprint_count` | int | Number of sprints found |
| `sprints` | array | Sprint objects (empty if no sprints exist) |

**Note**: Sprints are a bt-only concept (not upstream beads). If no sprints have been created, returns an empty list.

**Examples**:
```bash
bt robot burndown current
bt robot burndown sprint-2026-04
```

---

### bt robot capacity

**Purpose**: Capacity simulation - estimates how many beads can be completed given N parallel agents and current velocity.

**Unique flags**:
- `--agents <n>`: Number of parallel agents (default 1)
- `--capacity-label <name>`: Filter by label

**Examples**:
```bash
bt robot capacity
bt robot capacity --agents 3
bt robot capacity --agents 3 --capacity-label area:tui
```

---

### bt robot portfolio

**Purpose**: Per-project health aggregates when using `--global`. Returns one health snapshot per project prefix.

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `schema` | string | `portfolio.v1` |
| `projects` | array | Per-project health objects |
| `projects[].project` | string | Project prefix (`local`, `bt`, etc.) |
| `projects[].counts` | object | `open`, `blocked`, `in_progress`, `closed_30d` |
| `projects[].priority` | object | `p0`, `p1` counts |
| `projects[].velocity` | object | `closures_7d`, `closures_30d`, `trend` |
| `projects[].health_score` | float | 0-1 health score |
| `projects[].top_blocker` | object | Highest-PageRank open issue |
| `projects[].stalest` | object | Longest-inactive open issue |

**Examples**:
```bash
bt robot portfolio --global
bt robot portfolio
```

---

## Nested Subcommand Groups

### bt robot files

File-bead correlation analysis. All subcommands correlate files to beads via git history. Requires git history accessible to bt.

#### bt robot files beads

**Purpose**: Which beads touched a given file path.

**Positional arg**: `<path>` (required)

**Unique flags**:
- `--limit <n>`: Max closed beads to show (default 20)

**Note**: When run outside a beads project (no local `.beads/`), the error message will suggest `--global` to query the global Dolt server.

**Examples**:
```bash
bt robot files beads pkg/view/compact.go --global
bt robot files beads cmd/bt/cobra_robot.go
```

#### bt robot files hotspots

**Purpose**: Files touched by the most beads - highest-churn files in the issue tracker history.

**Unique flags**:
- `--limit <n>`: Max hotspots to show (default 10)

**Note**: When run outside a beads project (no local `.beads/`), the error message will suggest `--global` to query the global Dolt server.

**Examples**:
```bash
bt robot files hotspots
bt robot files hotspots --limit 20 --global
```

#### bt robot files relations

**Purpose**: Files that frequently co-change with a given file (co-change correlation).

**Positional arg**: `<path>` (required)

**Unique flags**:
- `--limit <n>`: Max related files (default 10)
- `--threshold <float>`: Minimum correlation coefficient (default 0.5)

**Examples**:
```bash
bt robot files relations pkg/view/compact.go
bt robot files relations cmd/bt/cobra_robot.go --threshold 0.3
```

---

### bt robot correlation

Commit-to-bead correlation audit and feedback. Use to improve correlation accuracy over time.

#### bt robot correlation stats

**Purpose**: Aggregate feedback statistics on correlation accuracy.

**Top-level fields**: `total_feedback`, `confirmed`, `rejected`, `ignored`, `accuracy_rate`, `avg_confirm_conf`, `avg_reject_conf`.

**Examples**:
```bash
bt robot correlation stats
```

#### bt robot correlation explain

**Purpose**: Explains why a specific commit is linked to a specific bead.

**Positional arg**: `<SHA:beadID>` - colon-separated commit SHA and bead ID.

**Examples**:
```bash
bt robot correlation explain abc123def:bt-53du
```

#### bt robot correlation confirm

**Purpose**: Mark a correlation as correct (positive feedback).

**Positional arg**: `<SHA:beadID>`

**Unique flags**:
- `--reason <text>`: Reason for feedback
- `--by <text>`: Agent/user identifier

**Examples**:
```bash
bt robot correlation confirm abc123def:bt-53du --by agent-1 --reason "matches commit message"
```

#### bt robot correlation reject

**Purpose**: Mark a correlation as incorrect (negative feedback).

**Positional arg**: `<SHA:beadID>`

**Unique flags**:
- `--reason <text>`: Reason for rejection
- `--by <text>`: Agent/user identifier

**Examples**:
```bash
bt robot correlation reject abc123def:bt-999 --by agent-1 --reason "commit predates this bead"
```

---

### bt robot labels

Label health and flow analysis.

#### bt robot labels health

**Purpose**: Health metrics for every label - velocity, freshness, flow, criticality. Identifies labels with stale or blocked issues.

**Top-level fields**:
| Field | Type | Description |
|---|---|---|
| `total_labels` | int | Total label count |
| `healthy_count` | int | Labels at healthy threshold |
| `warning_count` | int | Labels at warning threshold |
| `critical_count` | int | Labels at critical threshold |
| `results.labels` | array | Per-label objects |
| `results.labels[].label` | string | Label name |
| `results.labels[].health` | int | 0-100 health score |
| `results.labels[].health_level` | string | `healthy`, `warning`, `critical` |
| `results.labels[].velocity` | object | Closure rate, trend |
| `results.labels[].freshness` | object | Staleness metrics |
| `results.labels[].flow` | object | Dependency flow in/out |
| `results.labels[].criticality` | object | PageRank, betweenness, bottleneck count |

**Examples**:
```bash
bt robot labels health
bt robot labels health --label area:tui
```

#### bt robot labels attention

**Purpose**: Attention-ranked labels - which labels need the most focus based on PageRank, blocked count, staleness, and velocity.

**Top-level fields**: `limit`, `total_labels`, `labels[]` with `rank`, `label`, `attention_score`, `normalized_score`, `reason`, `open_count`, `blocked_count`, `stale_count`.

**Examples**:
```bash
bt robot labels attention
bt robot labels attention --global
```

#### bt robot labels flow

**Purpose**: Cross-label dependency flow - which labels depend on which other labels and where work is stacking up.

**Examples**:
```bash
bt robot labels flow
bt robot labels flow --global
```

---

### bt robot baseline

Baseline management for drift detection. A baseline is a snapshot of project metrics saved for later comparison.

#### bt robot baseline save

**Purpose**: Saves current metrics as a baseline for future `bt robot drift` comparisons.

**Examples**:
```bash
bt robot baseline save
```

#### bt robot baseline info

**Purpose**: Shows information about the currently saved baseline (timestamp, metrics summary).

**Note**: Returns "No baseline found" if none has been saved yet.

**Examples**:
```bash
bt robot baseline info
```

---

### bt robot sprint

Sprint management. Sprints are a bt-only concept (not upstream beads).

#### bt robot sprint list

**Purpose**: All defined sprints as JSON.

**Top-level fields**: `sprint_count`, `sprints` array (empty if no sprints defined).

**Examples**:
```bash
bt robot sprint list
```

#### bt robot sprint show

**Purpose**: Details for a specific sprint.

**Positional arg**: `<sprint-id>`

**Examples**:
```bash
bt robot sprint show sprint-2026-04
```

---

## Common Agent Workflows

### Orient at session start
```bash
bt robot triage                                          # Full picture
bt robot next                                            # Single top recommendation
```

### Claim and work
```bash
bt robot next | jq -r '.claim_command'                  # Extract and run claim command
```

### Multi-agent coordination
```bash
bt robot plan --by-track                                 # Partition work into parallel tracks
bt robot triage --by-label                               # Partition by label area
```

### Investigate a specific bead
```bash
bt robot blocker-chain <id>                              # Why is it blocked?
bt robot related <id> --global                           # What's related?
bt robot forecast <id>                                   # When will it be done?
```

### Find work by criteria
```bash
bt robot list --status open --priority 0-1              # P0/P1 open issues
bt robot list --type bug --status open                  # Open bugs
bt robot bql --query "status=open blocked=false"        # Unblocked open issues
bt robot search --query "authentication timeout"        # Semantic search
```

### Monitor project health
```bash
bt robot alerts                                          # Stale + blocking cascades
bt robot priority                                        # Priority misalignment
bt robot labels attention                                # Which labels need focus?
bt robot portfolio --global                              # Cross-project health
```

### Analyze change impact
```bash
bt robot impact pkg/view/compact.go                     # What beads touched this file?
bt robot files hotspots                                  # Which files have the most churn?
bt robot impact-network --global                        # Full impact topology
```

### Cross-project visibility
```bash
bt robot pairs --global                                  # Paired beads across projects
bt robot refs --global                                   # Cross-project references in prose
```
