# Complementary Features Analysis

## Current State: 9 Open Epics

| Epic | Priority | Theme | Key Capability |
|------|----------|-------|----------------|
| bv-80 | P1 | Agent-first priority | Priority scoring with graph signals |
| bv-62 | P1 | Bead history | Git-to-bead correlation |
| bv-53 | P1 | Robot UX | Hardened agent interfaces |
| bv-99 | P2 | Labels view | Label health, attention, flows |
| bv-92 | P2 | Priority visuals | Graph exports, TUI panels |
| bv-epf | P2 | Multi-repo | Aggregate across repositories |
| bv-qjc | P2 | Exports & hooks | Automation pipelines |
| bv-52t | P3 | Drift alerts | Baseline comparison, CI |
| bv-9gf | P3 | Semantic search | Vector similarity |

---

## Gap Analysis

### Gap 1: Multi-Agent Coordination (CRITICAL)

bv is "agent-first" but implicitly assumes **single agent** operation. In practice:
- Multiple AI agents may work on the same project simultaneously
- Agents need to avoid stepping on each other
- Work should be partitioned efficiently
- Handoffs between agents need a protocol

**Missing capabilities:**
- Agent registry (who's active on this project?)
- Work claiming/releasing (I'm working on bv-42)
- File conflict detection (two agents touching same files)
- Parallel work recommendations (these 5 issues can be done simultaneously)
- Handoff messages (passing context between agents)

**Why this is critical:**
- AI coding agents are increasingly multi-agent (Claude Code + Devin + Cursor)
- Without coordination, agents will conflict
- Graph analysis already knows parallelizable tracks (--robot-plan has this)

---

### Gap 2: Time Dimension - Forecasting & Sprints

History (bv-62) adds the **past**, but there's no **future**:
- No ETA estimation per bead
- No sprint/milestone grouping
- No deadline tracking
- No capacity planning
- No burndown projections

**Missing capabilities:**
- Sprint definition and assignment
- ETA calculation (based on velocity + complexity)
- Burndown data generation
- Capacity simulation ("what if we add 2 agents?")
- Deadline risk alerts

**Why this matters:**
- Planning requires future projections
- Velocity data (from labels view) enables forecasting
- Agents need to know realistic completion estimates

---

### Gap 3: Unified Triage Intelligence

Intelligence is scattered across multiple commands:
- `--robot-priority` for priority scores
- `--robot-insights` for graph metrics
- `--robot-label-health` for label status
- `--robot-plan` for execution order

**No single "what should I work on NOW?" command that combines:**
- Priority ranking
- Label attention scores
- Blocker status
- Velocity context
- Risk assessment
- Staleness warnings

**Missing capabilities:**
- Unified recommendation engine
- Contextual "next action" suggestion
- Risk-adjusted priority
- Staleness alerts
- Blocking cascade warnings

---

### Gap 4: Proactive Alerts & Anomaly Detection

Current features are **reactive** (agent asks, bv answers). No **proactive** intelligence:
- No "this issue has been in_progress for 2 weeks with no commits"
- No "label X velocity dropped 50% this week"
- No "closing bv-42 will unblock 8 downstream issues"
- No "bv-37 looks like a duplicate of bv-12"

**Missing capabilities:**
- Staleness detection with thresholds
- Velocity anomaly alerts
- High-impact unblock opportunities
- Potential duplicate detection
- Dependency cycle warnings on creation

---

### Gap 5: Visual Dependency Graph

The dependency graph is **computed** but **invisible**:
- PageRank, betweenness, critical path all calculated
- No way to actually SEE the graph
- bv-94 (graph export) is a single task, not comprehensive

**Missing capabilities:**
- DOT format export for Graphviz
- ASCII art visualization in TUI
- Filterable by label/status/depth
- Critical path highlighting
- Interactive exploration

---

### Gap 6: Cross-Feature Query Language

Features are siloed. Can't easily ask compound questions:
- "High-priority issues in label 'backend' with declining velocity"
- "Commits touching issues that are now blocked"
- "Issues with PageRank > 0.5 and no activity in 7 days"

**Missing capabilities:**
- Query language or compound filters
- Cross-feature joins
- Saved queries / recipes
- Query builder in TUI

---

## Proposed New Features

### Feature 1: Agent Swarm Protocol (P1)

Enable multiple AI agents to work on the same project without conflicts.

**Core concepts:**
- **Agent Registry**: Track active agents (name, model, heartbeat, current work)
- **Work Claims**: Soft locks on beads during active work
- **File Reservations**: Advisory locks on file paths
- **Conflict Detection**: Alert when agents overlap
- **Partition Recommendations**: Suggest how to split work across N agents

**Robot commands:**
```bash
bv --robot-agents                    # List active agents
bv --robot-claim bv-42               # Claim a bead
bv --robot-release bv-42             # Release a claim
bv --robot-conflicts                 # Check for conflicts
bv --robot-partition --agents=3      # Recommend work split for 3 agents
```

**Data model:**
```go
type AgentRegistration struct {
    Name        string    // "claude-opus-1"
    Model       string    // "claude-opus-4"
    StartedAt   time.Time
    LastSeen    time.Time
    ClaimedWork []string  // bead IDs
    FileHints   []string  // files being touched
}

type WorkClaim struct {
    BeadID    string
    Agent     string
    ClaimedAt time.Time
    ExpiresAt time.Time  // auto-release after inactivity
}
```

**Integration points:**
- Uses `--robot-plan` parallel tracks for partitioning
- Integrates with history (who did what)
- Works with labels (assign agents by label)

---

### Feature 2: Sprint & Forecast System (P2)

Add time dimension for planning and projections.

**Core concepts:**
- **Sprints/Milestones**: Time-boxed groupings of beads
- **ETA Estimation**: Per-bead completion forecast
- **Burndown**: Track progress over time
- **Capacity Modeling**: Project completion based on team size

**Robot commands:**
```bash
bv --robot-sprint                    # Current sprint status
bv --robot-sprint create "v0.12" --start=2025-01-01 --end=2025-01-15
bv --robot-forecast bv-42            # ETA for specific bead
bv --robot-forecast --label=backend  # ETA for all backend work
bv --robot-burndown                  # Burndown data (JSON)
bv --robot-capacity --agents=2       # Project completion with 2 agents
```

**ETA calculation:**
```
eta = complexity_estimate / (label_velocity * agent_count)
complexity_estimate = 1 + dependency_depth + log(description_length)
confidence = min(0.9, 0.3 + 0.1 * historical_data_points)
```

**Integration points:**
- Uses velocity from labels view (bv-99)
- Uses history for trend data (bv-62)
- Feeds into priority (deadline proximity boost)

---

### Feature 3: Unified Triage Command (P1)

Single command for "what should I work on?"

**The `--robot-triage` command combines:**
1. Priority scores (from bv-80)
2. Label attention (from bv-99)
3. Graph criticality (PageRank, betweenness)
4. Blocker status
5. Staleness factor
6. Unblock impact (what gets freed by completing this)

**Output:**
```json
{
  "recommendations": [
    {
      "bead_id": "bv-42",
      "action": "work",
      "score": 94.2,
      "reasons": [
        "High PageRank (0.82) - central to project",
        "Blocks 5 downstream issues",
        "Label 'database' needs attention (health: 45)",
        "No activity in 12 days"
      ],
      "estimated_impact": "Unblocks: bv-43, bv-44, bv-51, bv-52, bv-58"
    },
    {
      "bead_id": "bv-38",
      "action": "review",
      "score": 87.1,
      "reasons": [
        "In progress for 14 days",
        "3 commits but no status change",
        "May be stuck - consider closing or updating"
      ]
    }
  ],
  "alerts": [
    {"type": "stale", "bead_id": "bv-29", "days_inactive": 21},
    {"type": "velocity_drop", "label": "frontend", "change": "-40%"},
    {"type": "cycle_risk", "beads": ["bv-55", "bv-56"]}
  ],
  "quick_wins": ["bv-61", "bv-62"],  // low effort, high impact
  "blockers_to_clear": ["bv-31"]      // highest downstream impact
}
```

**Integration points:**
- Synthesizes ALL other analysis
- Single entry point for agents
- Actionable, not just informative

---

### Feature 4: Proactive Alerts Engine (P2)

Background analysis that surfaces issues without being asked.

**Alert types:**
| Type | Trigger | Severity |
|------|---------|----------|
| stale_issue | No activity > threshold | warning |
| velocity_drop | Label velocity down > 30% | warning |
| blocking_cascade | Issue blocks > 5 others | info |
| potential_duplicate | Similarity > 0.8 | info |
| cycle_introduced | New dependency creates cycle | error |
| high_impact_unblock | Completing X unblocks > 3 | info |
| abandoned_claim | Agent claim expired | warning |

**Robot command:**
```bash
bv --robot-alerts                    # All current alerts
bv --robot-alerts --severity=warning # Filter by severity
bv --robot-alerts --type=stale       # Filter by type
```

**Configuration:**
```yaml
# .beads/config.yaml
alerts:
  stale_threshold_days: 14
  velocity_drop_threshold: 0.3
  duplicate_similarity: 0.8
  blocking_cascade_threshold: 5
```

**Integration points:**
- Uses history for staleness
- Uses labels for velocity
- Uses semantic search for duplicates (when available)
- Hooks into exports (bv-qjc)

---

### Feature 5: Dependency Graph Visualization (P2)

Make the computed graph visible.

**Output formats:**
1. **DOT**: For Graphviz rendering
2. **ASCII**: Inline in terminal
3. **JSON**: For custom rendering
4. **Mermaid**: For markdown embedding

**Robot commands:**
```bash
bv --robot-graph                     # Full graph as DOT
bv --robot-graph --format=ascii      # ASCII art
bv --robot-graph --format=mermaid    # Mermaid syntax
bv --robot-graph --label=backend     # Filter to label
bv --robot-graph --root=bv-42        # Subgraph from root
bv --robot-graph --depth=2           # Limit depth
bv --robot-graph --highlight=critical-path
```

**ASCII example:**
```
bv-31 [database migration]
  │
  ├──→ bv-42 [API schema update] ★ critical
  │      │
  │      ├──→ bv-51 [endpoint tests]
  │      └──→ bv-52 [docs update]
  │
  └──→ bv-43 [seed data]
         │
         └──→ bv-53 [integration tests]

Legend: ★ = critical path, ● = blocked, ○ = open
```

**TUI integration:**
- New view: Graph Explorer
- Navigate with hjkl
- Enter to jump to bead detail
- Filter by label, status

---

### Feature 6: Smart Suggestions (P3)

Intelligent recommendations to improve project hygiene.

**Suggestion types:**
1. **Missing dependency**: "bv-42 mentions 'auth' - should it depend on bv-31 (auth system)?"
2. **Duplicate detection**: "bv-55 is 85% similar to bv-23 - possible duplicate?"
3. **Label suggestion**: "bv-42 mentions 'database' - add label 'database'?"
4. **Stale cleanup**: "5 issues have been closed > 30 days, archive?"
5. **Cycle prevention**: "Adding this dependency would create a cycle"

**Robot command:**
```bash
bv --robot-suggest                   # All suggestions
bv --robot-suggest bv-42             # Suggestions for specific bead
bv --robot-suggest --type=dependency # Only dependency suggestions
```

**Integration points:**
- Uses semantic search (bv-9gf) when available
- Falls back to keyword matching
- Integrates with labels

---

## Priority Ranking

| Feature | Priority | Rationale |
|---------|----------|-----------|
| Agent Swarm Protocol | P1 | Critical for multi-agent workflows |
| Unified Triage | P1 | Immediate value, synthesizes existing work |
| Sprint & Forecast | P2 | Adds time dimension, builds on velocity |
| Proactive Alerts | P2 | Makes bv proactive, not just reactive |
| Graph Visualization | P2 | Makes invisible graph tangible |
| Smart Suggestions | P3 | Nice to have, requires semantic search |

---

## Cross-Feature Synergies

These features create powerful combinations with existing epics:

```
Agent Swarm + Labels = Assign agents by label domain
Agent Swarm + History = Track which agent did what
Agent Swarm + Multi-repo = Coordinate across repositories

Triage + Priority = Unified recommendations
Triage + Labels = Attention-aware suggestions
Triage + History = Staleness detection

Forecast + Labels = Per-label ETAs
Forecast + History = Trend-based predictions
Forecast + Priority = Deadline-boosted priority

Alerts + Labels = Velocity drop detection
Alerts + History = Staleness detection
Alerts + Semantic = Duplicate detection

Graph Viz + Labels = Filter by label
Graph Viz + Priority = Highlight critical path
Graph Viz + Exports = PNG/SVG output
```

---

## Implementation Order

**Phase 1: Foundation**
1. Unified Triage (quick win, synthesizes existing)
2. Agent Registry (data model, basic commands)

**Phase 2: Core**
3. Work Claims & Conflicts
4. Proactive Alerts
5. Graph Visualization (DOT export)

**Phase 3: Advanced**
6. Sprint System
7. ETA Forecasting
8. Agent Partitioning

**Phase 4: Polish**
9. ASCII graph in TUI
10. Smart Suggestions
11. Query Language
