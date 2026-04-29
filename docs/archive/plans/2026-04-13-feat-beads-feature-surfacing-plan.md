---
title: "feat: Beads feature surfacing in bt TUI"
type: feat
status: active
date: 2026-04-13
origin: docs/audits/gaps/2026-04-13-bt-mol-fk82-synthesis.md
parent_issue: bt-53du
---

# Beads Feature Surfacing in bt TUI

## Overview

The bd-0il feature audit found 103 beads subcommands with 65% invisible to agents. A research-audit molecule (bt-mol-qvjm) analyzed what bt should surface and produced a 4-wave implementation plan with 7 new beads. This plan sequences those waves into executable sessions, addresses gaps found in spec-flow analysis, and integrates with the ADR-002 development arc.

## Orientation

Read these files before implementing:

| File | Purpose |
|------|---------|
| `docs/audits/gaps/2026-04-13-bt-mol-fk82-synthesis.md` | Research findings and wave definitions |
| `internal/datasource/columns.go:9-14` | IssuesColumns constant (single extension point) |
| `internal/datasource/dolt.go:64-176` | LoadIssuesFiltered scan pattern |
| `internal/datasource/global_dolt.go:420-502` | scanGlobalIssue (must match dolt.go) |
| `internal/datasource/sqlite.go` | Independent column list (NOT using IssuesColumns) |
| `pkg/model/types.go:8-97` | Issue struct + Clone() |
| `pkg/ui/delegate.go:36-298` | List item rendering (row layout budget) |
| `pkg/ui/model_filter.go:615-788` | Detail panel rendering |
| `pkg/ui/styles.go:190-252` | Badge system (RenderPriorityBadge, RenderStatusBadge) |

## Problem Statement

bt reads from Dolt but only surfaces 22 of 50+ columns from the issues table. Gate status (why something is blocked), molecule membership (workflow context), and operational state (dimension:value labels) are invisible. Users staring at blocked issues have no idea what they're waiting for. Additionally, existing model fields (DueDate, Dependencies, Labels) aren't being used for visual indicators that would improve at-a-glance project health.

## Proposed Solution

Four implementation sessions executing four waves of work, preceded by a quick schema verification prerequisite. AGENTS.md trim (bt-mj1l) is independent and can run alongside any session.

## Spec-Flow Gaps (addressed in sessions below)

The spec-flow analysis found 5 issues not in the original molecule output:

1. **Upstream schema column names are unverified** - Column names assumed but never checked against live Dolt. Addressed in Session 0.
2. **SQLite reader is a 5th file** - `sqlite.go` has its own column list independent of `IssuesColumns`. Added to Session 1 scope.
3. **loadIssuesSimple fallback** - `dolt.go:178-232` falls back to 8 columns on schema mismatch. Must be tested in Session 1.
4. **Row width budget is strained** - delegate.go row is already packed. Badge priority system needed before adding indicators - designed in Session 2.
5. **Human flag has two data sources** - "human" label (advisory) and `await_type="human"` (blocking gate). Decision: human label = yellow advisory flag, await_type=human = red blocking gate. Two distinct indicators, two data sources. Implemented in Session 2.

## Implementation Phases

### Session 0: Schema Verification (prerequisite for Session 1)
**Beads**: inline (fast investigation, not a full session)
**Dependencies**: None
**Blocks**: Session 1 only

- [ ] Connect to a live Dolt database with beads schema v6
- [ ] Run `DESCRIBE issues` or read upstream Go schema at `steveyegge/beads`
- [ ] Confirm exact column names for gate fields (await_type? gate_type? blocking_type?)
- [ ] Confirm exact column names for molecule fields (ephemeral? is_ephemeral? mol_type?)
- [ ] Confirm whether gate data is on the issues table or a separate table
- [ ] Confirm `bd swarm validate --json` exists and inspect output schema
- [ ] Update bt-c4da description with verified column names
- [ ] Update bt-1knw (swarm viz) with swarm JSON schema or mark as blocked if command doesn't exist

**If columns don't exist where expected**: Check whether gate data lives in a separate table (like labels/dependencies do). If so, the approach changes from "add columns to SELECT" to "add a new batch loader" following the loadLabels/loadDependencies pattern. Update bt-c4da scope before proceeding.

**Acceptance criteria**:
- [ ] Upstream column names verified and documented in bt-c4da notes

### Independent: AGENTS.md Trim (bt-mj1l)
**Bead**: bt-mj1l (P0)
**Dependencies**: None
**Blocks**: Nothing - do this whenever convenient, including parallel with any session

- [ ] Classify every section: every-turn essential vs session-start vs on-demand
- [ ] Move session-start content to `.beads/PRIME.md`
- [ ] Move on-demand content to skill references
- [ ] Preserve Workflow Formulas section
- [ ] Target: under 200 lines
- [ ] Verify fresh session still has all necessary context

**Acceptance criteria**:
- [ ] AGENTS.md under 200 lines, no content lost

---

### Session 1: Wave 0 - Data Model Extension (bt-c4da)
**Bead**: bt-c4da (P0)
**Dependencies**: Session 0 (verified column names)
**Blocks**: Session 2 (gate indicators), Session 4 (wisp toggle)

#### 1.1 Extend IssuesColumns constant
- [ ] Add verified column names to `internal/datasource/columns.go:9-14`
- [ ] Maintain consistent column order (append to end)

#### 1.2 Update DoltReader scan
- [ ] Add sql.Null* variables in `dolt.go` (after line 82)
- [ ] Add scan parameters in `dolt.go` (after line 91) - ORDER MUST MATCH columns.go
- [ ] Add validity checks in `dolt.go` (after line 154)
- [ ] Verify `loadIssuesSimple` fallback (line 178) still works when new columns don't exist

#### 1.3 Update GlobalDoltReader scan
- [ ] Add scan parameters in `global_dolt.go:scanGlobalIssue()` - MUST match dolt.go order
- [ ] Verify global UNION ALL query handles databases that lack new columns

#### 1.4 Update SQLite reader (or drop it)
- [ ] If SQLite still supported: add columns to sqlite.go's independent column list and scan
- [ ] If SQLite dropped upstream: remove sqlite.go or add deprecation notice
- [ ] Update `load.go` dispatch if SQLite behavior changes

#### 1.5 Extend Issue struct
- [ ] Add 6 pointer fields to `pkg/model/types.go:8-36` with json tags
- [ ] Update `Clone()` at types.go:38-97 for new pointer fields
- [ ] Verify JSON serialization includes new fields (omitempty for all)

#### 1.6 Tests
- [ ] Unit test: scan new columns from mock Dolt result set
- [ ] Unit test: Clone() deep-copies new pointer fields
- [ ] Unit test: fallback path produces issues with nil gate fields (not errors)
- [ ] Integration: `go build ./cmd/bt/` succeeds
- [ ] Integration: `go test ./internal/datasource/...` passes
- [ ] Integration: `go test ./pkg/model/...` passes

**Acceptance criteria**:
- [ ] bt connects to Dolt database with gate/molecule data and loads without error
- [ ] bt connects to Dolt database WITHOUT gate/molecule columns and falls back gracefully
- [ ] All existing tests pass
- [ ] New fields visible in `--robot` JSON output (automatic from struct tags)

**Files**: MODIFY columns.go, dolt.go, global_dolt.go, sqlite.go, types.go + their test files

---

### Session 2: Wave 1 - Gate and Human Indicators (bt-c69c)
**Bead**: bt-c69c (P0/P1)
**Dependencies**: Session 1 (needs gate fields in model)
**Blocks**: Session 3 (shares delegate.go rendering code)

#### 2.0 Design badge priority system
Before adding any indicators, define the priority ordering for the delegate.go row:
- [ ] Define which indicators appear at which terminal widths
- [ ] Establish priority ordering: status > gate > priority > triage > overdue > stale > epic > assignee
- [ ] Document as constants or comments in delegate.go
- [ ] Account for existing width thresholds (lines 72, 89, 98, 105)

#### 2.1 Gate status badge in list view
- [ ] Add gate type detection in `delegate.go` (after triage indicator, ~line 240)
- [ ] Follow existing triage indicator pattern for width-aware rendering
- [ ] Gate icons by type: human=hand, timer=clock, gh:run=CI, gh:pr=PR, bead=chain
- [ ] Only show on issues where AwaitType is non-nil
- [ ] Respect badge priority system from 2.0

#### 2.2 Human flag badge (advisory)
- [ ] Detect "human" label in issue.Labels
- [ ] Render as yellow flag icon, distinct from gate badge
- [ ] Only show when AwaitType is nil (if AwaitType=human, show gate badge instead)

#### 2.3 Gate section in detail panel
- [ ] Add gate section to `model_filter.go` after labels section (~line 652)
- [ ] Show: gate type, await target, timeout (if set), "waiting since" duration
- [ ] Render as markdown with appropriate formatting
- [ ] For human advisory (no gate): show "Flagged for human input" banner

#### 2.4 Colors and styles
- [ ] Add gate colors to `styles.go:28-96` (ColorGateHuman, ColorGateTimer, ColorGateCI)
- [ ] Add gate badge rendering function to `styles.go` following RenderStatusBadge pattern
- [ ] Add gate icons to `theme.go:197-214` GetTypeIcon

#### 2.5 Tests
- [ ] Rendering test: gate badge appears for gated issues
- [ ] Rendering test: human label shows advisory badge (not gate badge) when no gate
- [ ] Rendering test: gate section renders in detail panel
- [ ] Width test: gate badge suppressed at narrow widths per priority system

**Acceptance criteria**:
- [ ] Blocked-by-gate issues show gate type icon in list view
- [ ] Human-flagged (advisory) issues show distinct yellow indicator
- [ ] Detail panel shows gate metadata section
- [ ] Advisory vs blocking distinction is visually obvious

**Files**: MODIFY delegate.go, model_filter.go, styles.go, theme.go + test files

---

### Session 3: Wave 2 - Existing-Model Indicators (bt-5oqf, bt-waeh, bt-jprp)
**Beads**: bt-5oqf (P2), bt-waeh (P2), bt-jprp (P2)
**Dependencies**: Session 2 (badge priority system and delegate.go rendering patterns)
**Note**: No data model dependency on Session 1 - these use existing fields (DueDate, Dependencies, Labels). The dependency on Session 2 is practical: both sessions modify delegate.go and model_filter.go, so running them in parallel causes merge conflicts. Session 2 establishes the badge priority system that Session 3 must follow.

#### 3.1 Stale/overdue indicators (bt-5oqf)
- [ ] Overdue: issue.DueDate != nil && issue.DueDate.Before(time.Now()) && !closed
- [ ] Stale: time.Since(issue.UpdatedAt) > staleThreshold && status in (open, in_progress)
- [ ] Stale threshold: 14 days default, configurable via `BT_STALE_DAYS` env var
- [ ] List view: red clock for overdue, dimmed/faded for stale (delegate.go)
- [ ] Detail panel: overdue/stale notice with dates (model_filter.go)

#### 3.2 Epic progress indicator (bt-waeh)
- [ ] Count children by status for epic-type issues (via Dependencies with DepParentChild)
- [ ] List view: compact "3/7" counter next to epic issues (delegate.go)
- [ ] Detail view: progress section with child breakdown (model_filter.go)
- [ ] Only show for issue_type == "epic" with at least one child

#### 3.3 State dimension parsing (bt-jprp)
- [ ] Parse labels matching `dimension:value` pattern (exclude known non-state prefixes: export:, provides:, external:)
- [ ] Detail view: structured badges section for state dimensions
- [ ] Known dimensions (health, patrol, mode) get distinct colors
- [ ] List view: optional compact state indicator (stretch goal - may skip for row budget)

#### 3.4 Tests
- [ ] Time-dependent tests for stale/overdue with configurable threshold
- [ ] Epic progress with 0/N, partial, and N/N children
- [ ] State dimension parsing edge cases (colons in values, non-state prefixed labels)

**Acceptance criteria**:
- [ ] Overdue issues show red indicator
- [ ] Stale issues show dimmed styling
- [ ] Epics show child completion counter
- [ ] State labels render as structured badges in detail view
- [ ] All existing tests still pass

**Files**: MODIFY delegate.go, model_filter.go, styles.go, helpers.go (new parsing helpers) + test files

---

### Session 4: Waves 3+4 - Advanced Features (bt-1knw, bt-t0z6, bt-9kdo)
**Beads**: bt-1knw (P2), bt-t0z6 (P2), bt-9kdo (P3)
**Dependencies**: Session 1 (bt-9kdo needs ephemeral field), Session 0 (bt-1knw needs swarm CLI verification)
**Note**: These are independent of each other. If session is running long, bt-9kdo (simplest) can be deferred.

#### 4.1 Swarm visualization (bt-1knw) - CONDITIONAL on Session 0 verification
- [ ] Shell to `bd swarm validate <epic-id> --json` via exec.CommandContext
- [ ] Parse JSON output for wave assignments per issue
- [ ] Add wave-based coloring to ViewGraph (wave 0=green, 1=yellow, 2+=blue gradient)
- [ ] Show max parallelism in insights panel
- [ ] Keybinding to toggle swarm coloring on/off in graph view

#### 4.2 Capability map for global mode (bt-t0z6)
- [ ] Scan labels across databases for export:/provides:/external: patterns
- [ ] Build project-to-project edge map from parsed labels
- [ ] Render as graph overlay or new view mode (ViewCapabilities)
- [ ] Highlight unresolved capabilities (export without matching provides) in red
- [ ] Single-project mode: show only that project's capabilities

#### 4.3 Wisp visibility toggle (bt-9kdo)
- [ ] Add filter toggle keybinding (suggest: `w` for wisps)
- [ ] Filter on issue.Ephemeral field (from Wave 0)
- [ ] Default: hide wisps (match bd ready behavior)
- [ ] Visual distinction for ephemeral issues when visible (dimmed or ghost icon)
- [ ] Toggle state persists within session only

**Acceptance criteria**:
- [ ] Swarm graph colors nodes by wave number (if swarm CLI verified)
- [ ] Global mode shows capability map with project edges
- [ ] Wisp toggle shows/hides ephemeral issues

**Files**: MODIFY graph.go, delegate.go, model_filter.go, model_keys.go. NEW: capability view if new view mode.

---

## ADR-002 Integration

This work creates a **new Stream 8: Beads feature surfacing** in ADR-002, spanning the gap between the existing streams and CRUD (Stream 7). It specifically:
- Extends Stream 1 (robot-mode): new fields appear in `--robot` JSON automatically
- Complements Stream 7 (CRUD): gate/human visibility is prerequisite context for CRUD actions like "respond to human flag"
- Independent of Streams 2-6

After completing Sessions 0-4, update ADR-002 with Stream 8 status.

## Risk Analysis

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Upstream column names differ from assumptions | Medium | HIGH - Wave 0 scope changes | Session 0 prerequisite verifies schema |
| Row layout overflow at narrow widths | High | Medium - badges truncated | Badge priority system (Session 0.3) |
| delegate.go merge conflicts if Wave 2 parallels Wave 1 | Medium | Low - resolvable | Sequence Sessions 2 and 3 |
| bd swarm validate --json doesn't exist | Low | Medium - bt-1knw deferred | Verify in Session 0 |
| N+1 query amplified by new features | Low | Low - existing problem | Out of scope, flag for future |

## Future Considerations (out of scope)

- **BQL field registration**: Once gate/molecule fields exist, users will want `await_type = "human"` queries
- **N+1 query fix**: Migrate DoltReader to batch loading pattern (already done in GlobalDoltReader)
- **CRUD actions for gates**: "Respond to human flag" action in TUI (depends on Stream 7)
- **Deferred visual enhancement**: The synthesis listed Wave 2d (defer/undefer actions) but no bead was created. File if needed.
- **Export format**: Verify model_export.go includes new fields (likely automatic from struct tags)

## Sources & References

### Origin
- **Research-audit molecule**: bt-mol-qvjm (completed 2026-04-13)
- **Synthesis doc**: [docs/audits/gaps/2026-04-13-bt-mol-fk82-synthesis.md](../../audits/gaps/2026-04-13-bt-mol-fk82-synthesis.md) - wave definitions, dependency graph, implementation estimates
- **Feature audit**: bd-0il (beads project) - 103 subcommands cataloged, 65% invisible to agents

### Internal References
- ADR-002: [docs/adr/002-stabilize-and-ship.md](../adr/002-stabilize-and-ship.md) - project spine
- Product vision: [docs/brainstorms/2026-04-09-product-vision.md](../brainstorms/2026-04-09-product-vision.md) - three pillars, development arc
- Column constant: `internal/datasource/columns.go:9-14`
- Badge patterns: `pkg/ui/styles.go:190-252`
- Detail rendering: `pkg/ui/model_filter.go:615-788`

### Connected Beads
- bt-53du: Product vision epic (parent)
- bt-c4da: Wave 0 data model (P0)
- bt-c69c: Wave 1 gate/human indicators (P0/P1)
- bt-5oqf: Wave 2 stale/overdue (P2)
- bt-waeh: Wave 2 epic progress (P2)
- bt-jprp: Wave 2 state dimensions (P2)
- bt-1knw: Wave 3 swarm viz (P2)
- bt-t0z6: Wave 3 capability map (P2)
- bt-9kdo: Wave 4 wisp toggle (P3)
- bt-mj1l: AGENTS.md trim (P0, independent)
- bd-0il: Feature audit (beads project, origin)
- mkt-0il: Teaching layer (marketplace, will package what bt surfaces)
