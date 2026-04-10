# Labels View Feature Plan

## Overview

Labels in bv are not just filters—they're **subgraph selectors** that unlock powerful analysis. This plan designs a Labels View that goes beyond simple filtering to provide label-based graph analysis, health metrics, and cross-label dependency visualization.

## Core Philosophy

### Labels as Graph Overlays

Each label defines a subgraph of the dependency network. Running graph algorithms on these subgraphs reveals label-specific structure:

- **Label PageRank**: Which issues are central *within this label*?
- **Label Critical Path**: What's the longest dependency chain *in this label*?
- **Label Bottlenecks**: Which issues have high betweenness *for this label*?

### Cross-Label Flow Analysis

Dependencies often cross label boundaries. Visualizing these flows reveals:

- Which labels "feed into" others (producer/consumer patterns)
- Bottleneck labels that block multiple downstream labels
- Isolated labels with no cross-dependencies

---

## Feature 1: Label Health Dashboard

### Health Score Components

Each label gets a composite health score (0-100) based on:

| Component | Weight | Calculation |
|-----------|--------|-------------|
| Velocity | 30% | Issues closed / week (normalized) |
| Freshness | 25% | Inverse of avg days since last activity |
| Flow | 25% | % of issues not blocked |
| Criticality | 20% | Sum of PageRank in label (relative) |

### Dashboard View

```
╭─ Label Health Dashboard ─────────────────────────────────────────╮
│                                                                  │
│  Label        │ Health │ Open │ Blocked │ Velocity │ Stalest    │
│  ─────────────┼────────┼──────┼─────────┼──────────┼──────────  │
│  frontend     │ ██░░ 85│   12 │    1    │  4.2/wk  │   3 days   │
│  backend      │ ██░░ 72│    8 │    3    │  2.1/wk  │  12 days   │
│  database     │ █░░░ 45│    5 │    4    │  0.5/wk  │  28 days   │ ← attention
│  testing      │ ███░ 91│    3 │    0    │  3.0/wk  │   1 day    │
│  docs         │ ██░░ 78│    6 │    0    │  1.5/wk  │   7 days   │
│                                                                  │
│  [j/k] Navigate  [enter] Drilldown  [h] Health details  [?] Help │
╰──────────────────────────────────────────────────────────────────╯
```

### Health Detail Popup

```
╭─ database: Health Breakdown ────────────────────╮
│                                                 │
│  Overall: 45/100 (Needs Attention)              │
│                                                 │
│  ┌─ Velocity ──────────────────────────────┐    │
│  │ 0.5 issues/week (target: 2.0)        ░░░│ 25%│
│  └─────────────────────────────────────────┘    │
│                                                 │
│  ┌─ Freshness ─────────────────────────────┐    │
│  │ Stalest issue: 28 days old           ░░░│ 20%│
│  │ Avg age: 14 days                        │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  ┌─ Flow ──────────────────────────────────┐    │
│  │ 4/5 issues blocked (80%)             ░░░│ 15%│
│  │ Blocking: backend(2), frontend(1)       │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  ┌─ Criticality ───────────────────────────┐    │
│  │ 32% of total PageRank               ████│ 85%│
│  │ High-impact issues need unblocking!     │    │
│  └─────────────────────────────────────────┘    │
│                                                 │
│  Recommendation: Unblock database issues to     │
│  unblock downstream backend and frontend work.  │
│                                                 │
╰─────────────────────────────────────────────────╯
```

---

## Feature 2: Label Drilldown View

When selecting a label, show a filtered list with label-specific analysis.

### Drilldown Layout

```
╭─ frontend (12 issues) ───────────────────────────────────────────╮
│                                                                  │
│ ┌─ Label Metrics ──────────────────────────────────────────────┐ │
│ │ Health: 85  Velocity: 4.2/wk  Blocked: 1  Critical Path: 3   │ │
│ └──────────────────────────────────────────────────────────────┘ │
│                                                                  │
│ ┌─ Cross-Label Dependencies ───────────────────────────────────┐ │
│ │ ← Depends on: backend(3), database(2)                        │ │
│ │ → Blocks: testing(4), docs(1)                                │ │
│ └──────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  #  │ Type │ Title                        │ Status      │ PR/Be  │
│ ────┼──────┼──────────────────────────────┼─────────────┼─────── │
│ >42 │ ✨   │ Add user preferences panel   │ in_progress │ 0.85   │
│  38 │ 🐛   │ Fix modal z-index            │ open        │ 0.72   │
│  45 │ 📋   │ Update nav component         │ blocked     │ 0.68   │
│  ...                                                             │
│                                                                  │
│  [j/k] Navigate  [enter] Detail  [g] Graph  [←] Back  [?] Help   │
╰──────────────────────────────────────────────────────────────────╯
```

### Label-Specific Graph Analysis

Press `g` to see graph metrics for the label's subgraph:

```
╭─ frontend: Graph Analysis ──────────────────────────────────────╮
│                                                                 │
│  ┌─ PageRank (Within Label) ──────────────────────────────────┐ │
│  │ #42 Add user preferences panel ........................ 0.85│ │
│  │ #38 Fix modal z-index ................................. 0.72│ │
│  │ #51 Implement dark mode toggle ........................ 0.65│ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌─ Critical Path ─────────────────────────────────────────────┐ │
│  │ #38 → #42 → #51 (3 issues, 2 dependencies)                  │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌─ Bottlenecks (High Betweenness) ────────────────────────────┐ │
│  │ #42 (betweenness: 0.67) - blocking 4 other issues           │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  [←] Back to drilldown  [?] Help                                │
╰─────────────────────────────────────────────────────────────────╯
```

---

## Feature 3: Cross-Label Flow Visualization

### Flow Matrix View

Show how labels relate through dependencies:

```
╭─ Cross-Label Dependency Flow ────────────────────────────────────╮
│                                                                  │
│  Dependencies: A depends on B (A ← B)                            │
│                                                                  │
│             │ frontend │ backend │ database │ testing │ docs │   │
│  ───────────┼──────────┼─────────┼──────────┼─────────┼──────│   │
│  frontend   │    -     │   ← 3   │   ← 2    │         │      │   │
│  backend    │          │    -    │   ← 5    │         │      │   │
│  database   │          │         │    -     │         │      │   │
│  testing    │   ← 4    │   ← 2   │          │    -    │      │   │
│  docs       │   ← 1    │   ← 1   │          │   ← 2   │  -   │   │
│                                                                  │
│  Reading: testing depends on 4 frontend issues, 2 backend issues │
│                                                                  │
│  Insight: database is a "source" label - no dependencies         │
│           testing/docs are "sink" labels - mostly consumers      │
│                                                                  │
│  [Enter] Explore connection  [?] Help                            │
╰──────────────────────────────────────────────────────────────────╯
```

### Flow Insight

When database is blocked, compute the impact cascade:

```
╭─ Blockage Impact Analysis ──────────────────────────────────────╮
│                                                                 │
│  database has 4 blocked issues. Downstream impact:              │
│                                                                 │
│  database(4 blocked)                                            │
│     │                                                           │
│     ├──→ backend: 3 issues waiting                              │
│     │       │                                                   │
│     │       └──→ testing: 2 issues waiting                      │
│     │                                                           │
│     └──→ frontend: 2 issues waiting                             │
│             │                                                   │
│             └──→ testing: 1 issue waiting                       │
│             └──→ docs: 1 issue waiting                          │
│                                                                 │
│  Total: 9 downstream issues blocked by database                 │
│                                                                 │
│  Recommendation: Prioritize unblocking database#31, database#34 │
│  (highest downstream PageRank impact)                           │
│                                                                 │
╰─────────────────────────────────────────────────────────────────╯
```

---

## Feature 4: Label Velocity Tracking

### Velocity Comparison View

```
╭─ Label Velocity (Last 4 Weeks) ──────────────────────────────────╮
│                                                                  │
│  Label      │ W-4 │ W-3 │ W-2 │ W-1 │ Avg  │ Trend │            │
│  ───────────┼─────┼─────┼─────┼─────┼──────┼───────┼            │
│  frontend   │   3 │   5 │   4 │   6 │  4.5 │  ↑↑   │ ████████   │
│  backend    │   4 │   3 │   2 │   1 │  2.5 │  ↓↓   │ █████      │
│  database   │   1 │   0 │   1 │   0 │  0.5 │  →    │ █          │
│  testing    │   2 │   3 │   3 │   4 │  3.0 │  ↑    │ ██████     │
│  docs       │   1 │   2 │   1 │   2 │  1.5 │  →    │ ███        │
│                                                                  │
│  Alert: backend velocity declining - investigate blockers        │
│                                                                  │
╰──────────────────────────────────────────────────────────────────╯
```

### Velocity History (Integration with Time-Travel)

When bead history feature is implemented, show historical velocity:

```
╭─ frontend: Velocity History ─────────────────────────────────────╮
│                                                                  │
│  Issues Closed Per Week                                          │
│                                                                  │
│    8│                                          ╭─╮               │
│    6│                              ╭─╮   ╭─╮   │ │   ╭─╮         │
│    4│         ╭─╮   ╭─╮   ╭─╮   ╭─╯ │   │ │   │ │   │ │         │
│    2│   ╭─╮   │ │   │ │   │ │   │   │   │ │   │ │   │ │         │
│    0│───┴─┴───┴─┴───┴─┴───┴─┴───┴───┴───┴─┴───┴─┴───┴─┴───      │
│      W-12 W-11 W-10 W-9  W-8  W-7  W-6  W-5  W-4  W-3  W-2  W-1  │
│                                                                  │
│  Average: 4.2/week  Std Dev: 1.8  Best: W-5 (7)  Worst: W-12 (1) │
│                                                                  │
╰──────────────────────────────────────────────────────────────────╯
```

---

## Feature 5: Label Attention Ranking

### Attention Score Calculation

```
attention_score = (pagerank_sum × staleness_factor × block_impact) / velocity
```

Where:
- `pagerank_sum`: Sum of PageRank for open issues in label
- `staleness_factor`: `1 + log(avg_days_since_activity)`
- `block_impact`: `1 + count_of_blocked_downstream_issues`
- `velocity`: Issues closed per week (min 0.1 to avoid division by zero)

### Attention Dashboard

```
╭─ Labels Needing Attention ───────────────────────────────────────╮
│                                                                  │
│  Rank │ Label    │ Attn  │ Why                                   │
│  ─────┼──────────┼───────┼──────────────────────────────────────│
│  1    │ database │ 94.2  │ High PageRank, 4 blocked, low velocity│
│  2    │ backend  │ 67.8  │ Declining velocity, 3 blocked         │
│  3    │ docs     │ 23.4  │ 2 stale issues (>14 days)            │
│  4    │ frontend │ 12.1  │ Healthy, minor staleness              │
│  5    │ testing  │  8.3  │ Excellent health                      │
│                                                                  │
│  Quick Actions:                                                  │
│  [1-5] Jump to label  [r] Refresh  [?] Help                      │
│                                                                  │
╰──────────────────────────────────────────────────────────────────╯
```

---

## Feature 6: Insights Integration

### Label-Aware Insights

Extend the existing Insights view with label intelligence:

```
╭─ Insights ───────────────────────────────────────────────────────╮
│                                                                  │
│  📊 Graph Analysis                                               │
│  ├─ Critical path: 7 issues (4 in database, 2 in backend)        │
│  └─ Bottleneck: #31 database migration (blocks 9 issues)         │
│                                                                  │
│  🏷️  Label Intelligence                                          │
│  ├─ database needs attention: 4 blocked + low velocity           │
│  ├─ backend velocity declining: 4→3→2→1 over 4 weeks            │
│  ├─ frontend healthy: 85/100 score, 4.2/wk velocity             │
│  └─ Cross-label bottleneck: database→backend→testing chain       │
│                                                                  │
│  🎯 Recommendations                                              │
│  ├─ 1. Unblock database#31 (highest downstream impact)           │
│  ├─ 2. Investigate backend slowdown                              │
│  └─ 3. Consider splitting database label (too broad?)            │
│                                                                  │
╰──────────────────────────────────────────────────────────────────╯
```

---

## Feature 7: Robot Protocol Extensions

### New Commands

```bash
# Label health for all labels
bv label-health --robot-json
# Output: {"labels": [{"name": "frontend", "health": 85, "velocity": 4.2, ...}]}

# Single label deep analysis
bv label-health database --robot-json
# Output: {"name": "database", "health": 45, "components": {...}, "recommendations": [...]}

# Cross-label flow analysis
bv label-flow --robot-json
# Output: {"flows": [{"from": "database", "to": "backend", "count": 5}], "insights": [...]}

# Labels needing attention
bv label-attention --robot-json --limit=3
# Output: {"attention": [{"label": "database", "score": 94.2, "reason": "..."}]}

# Velocity trends
bv label-velocity --robot-json --weeks=4
# Output: {"labels": [{"name": "frontend", "velocity": [3,5,4,6], "trend": "increasing"}]}
```

### Integration with List

```bash
# List filtered by label with graph metrics
bv list --label=frontend --robot-json --include-graph-metrics
# Output: issues with label_pagerank, label_betweenness fields

# List with label health context
bv list --robot-json --include-label-health
# Output: issues with their label's health score attached
```

---

## Implementation Architecture

### New Types

```go
// pkg/analysis/label_health.go

type LabelHealth struct {
    Name        string
    Health      float64 // 0-100 composite score

    // Components
    Velocity    VelocityMetrics
    Freshness   FreshnessMetrics
    Flow        FlowMetrics
    Criticality CriticalityMetrics

    // Cross-label
    DependsOn   []LabelDependency
    Blocks      []LabelDependency

    // Computed
    AttentionScore float64
    Recommendations []string
}

type VelocityMetrics struct {
    PerWeek     float64   // average
    History     []int     // last N weeks
    Trend       string    // "increasing", "decreasing", "stable"
}

type FlowMetrics struct {
    TotalOpen     int
    Blocked       int
    BlockedPct    float64
    DownstreamImpact int  // issues blocked transitively
}

type LabelDependency struct {
    Label string
    Count int
}

type CrossLabelFlow struct {
    From  string
    To    string
    Count int
}
```

### New Analysis Functions

```go
// pkg/analysis/labels.go

// ComputeLabelHealth calculates health for a single label
func ComputeLabelHealth(issues []bead.Bead, label string) LabelHealth

// ComputeAllLabelHealth calculates health for all labels
func ComputeAllLabelHealth(issues []bead.Bead) []LabelHealth

// ComputeCrossLabelFlows analyzes dependencies between labels
func ComputeCrossLabelFlows(issues []bead.Bead) []CrossLabelFlow

// ComputeLabelSubgraph extracts issues with a label and their dependencies
func ComputeLabelSubgraph(issues []bead.Bead, label string) []bead.Bead

// ComputeLabelPageRank runs PageRank on a label's subgraph
func ComputeLabelPageRank(issues []bead.Bead, label string) map[string]float64

// ComputeAttentionRanking ranks labels by attention needed
func ComputeAttentionRanking(health []LabelHealth) []LabelHealth
```

### UI Components

```go
// pkg/ui/label_dashboard.go
type LabelDashboard struct {
    labels []LabelHealth
    cursor int
    // ...
}

// pkg/ui/label_drilldown.go
type LabelDrilldown struct {
    label    string
    health   LabelHealth
    issues   []bead.Bead
    // ...
}

// pkg/ui/label_flow.go
type LabelFlowView struct {
    flows []CrossLabelFlow
    // ...
}
```

---

## Keybindings

### From Main List

- `L` - Open Label Dashboard
- `l` - Quick label filter (fuzzy search popup)

### In Label Dashboard

- `j/k` - Navigate labels
- `Enter` - Drilldown into label
- `h` - Health detail popup
- `f` - Flow matrix view
- `v` - Velocity comparison
- `a` - Attention ranking
- `?` - Help

### In Label Drilldown

- `j/k` - Navigate issues
- `Enter` - Issue detail
- `g` - Label graph analysis
- `←` or `Backspace` - Back to dashboard
- `?` - Help

---

## Phase Plan

### Phase 1: Foundation (Core Structures)
- LabelHealth type and basic computation
- Label extraction from issues
- Basic velocity calculation (if modified dates available)

### Phase 2: Dashboard UI
- LabelDashboard view component
- Health score display
- Navigation and keybindings

### Phase 3: Label Drilldown
- Filtered list view by label
- Label-specific metrics header
- Integration with existing list component

### Phase 4: Cross-Label Analysis
- CrossLabelFlow computation
- Flow matrix view
- Blockage impact analysis

### Phase 5: Graph Integration
- Label subgraph extraction
- Label-specific PageRank/Betweenness
- Critical path by label

### Phase 6: Attention & Insights
- Attention score computation
- Attention ranking view
- Insights integration

### Phase 7: Robot Protocol
- `label-health` command
- `label-flow` command
- `label-attention` command
- `label-velocity` command

### Phase 8: Velocity Tracking
- Historical velocity computation
- Trend detection
- Velocity charts (requires history feature)

---

## Success Metrics

1. **Discoverability**: Users can find labels needing attention in <3 seconds
2. **Actionability**: Each view provides clear next steps
3. **Integration**: Labels work with existing graph analysis
4. **Performance**: Dashboard renders in <100ms for 1000 issues
5. **AI-Ready**: Robot protocol provides structured label intelligence
