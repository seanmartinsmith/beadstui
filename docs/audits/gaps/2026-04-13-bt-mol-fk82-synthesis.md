# Synthesis: bt Feature Surfacing from bd-0il Audit

*Produced by bt-mol-fk82 (synthesize step of research-audit molecule bt-mol-qvjm)*

## Cross-Angle Confirmed Findings

### 1. Gate/molecule data plumbing is the critical bottleneck (all angles agree)
- Model has NO gate fields (await_type, await_id, timeout) - Angle 1, Angle 3
- Model has NO molecule fields (ephemeral, template, mol_type) - Angle 1, Angle 3
- IssuesColumns constant is the single extension point (columns.go:9-14) - Angle 3
- Global mode UNION ALL uses same constant, so extending it fixes both paths - Angle 1, Angle 3
- Scan order in dolt.go AND global_dolt.go must be updated in lockstep - Angle 3
- **Estimated: ~40-50 lines across 4 files** - this is the foundation all P0 features need

### 2. TUI rendering is the EASY part (Angle 2 confirmed)
- Badge system (styles.go:190-252) has established patterns - just add new badge types
- Detail panel (model_filter.go:615-788) has clear insertion points for new sections
- List delegate (delegate.go:36-298) has flexible row layout with triage indicator pattern to follow
- Type icons (theme.go:197-214) are a simple map - add gate/human/molecule entries
- **Estimated: ~50 lines total for basic indicators**

### 3. Already-implemented features are stronger than expected
- BQL: FULL parser/lexer/executor/validator in pkg/bql/ - no work needed for query capability
- Comments: fully loaded and rendered in detail panel
- Stale/overdue: analysis engine has freshness metrics
- Epic progress: parent-child deps tracked via DepParentChild
- Deferred: full visual treatment (DEFR badge, colors, graph rendering)
- Charm v2: already migrated, no framework debt

### 4. BD CLI execution pattern exists but needs extension
- exec.CommandContext pattern in doltctl.go and root.go - established
- No background worker for periodic bd command execution
- For read-only features (gate status, swarm validate): direct SQL is better than shelling out
- For write features (respond to human flag, defer/undefer): shell out to bd

## Contradictions / Corrections

### bt-angle doc said "rich markdown rendering may need minor changes" - actually it is done
The detail panel already renders description, design, acceptance_criteria, and notes as markdown sections (model_filter.go:727-748). This P0 item from the bt-angle doc is effectively complete. The only gap is ensuring glamour renders checklists/code blocks properly, which is a glamour capability, not a bt issue.

### bt-angle doc said "bt already has markdown.go" - confirmed but scope is larger
pkg/ui/markdown.go exists AND glamour v2 is integrated. The markdown rendering pipeline is complete. This can be removed from the P0 list.

## Priority Reassessment

Based on research findings, here is the corrected priority ordering:

### Wave 0: Foundation (MUST do first, blocks everything else)
**Extend data model for gate and molecule columns**
- Update columns.go, dolt.go scan, global_dolt.go scan, types.go struct + Clone()
- ~50 lines, 4 files, no behavioral change - pure plumbing
- Blocks: all P0 visual features
- Complexity: LOW (mechanical, pattern-following)

### Wave 1: P0 Visual Features (high value, low effort, enabled by Wave 0)
**1a. Gate status indicators**
- List view: gate type icon on blocked issues (delegate.go, ~10 lines)
- Detail panel: gate section showing type, await_id, timeout (model_filter.go, ~20 lines)
- Status badge: extend RenderStatusBadge for gate subtypes (styles.go, ~10 lines)
- Complexity: LOW

**1b. Human flag indicator**
- Parse "human" label as distinct visual badge (not just label text)
- Detail panel: "Awaiting human input" banner
- Already have label data - this is rendering logic only
- Complexity: LOW (no model changes needed - uses existing Labels field)

### Wave 2: P1 Incremental Features (medium value, low effort, no blockers)
**2a. Stale/overdue visual indicators**
- DueDate and UpdatedAt already in model
- Add color/badge to list items for overdue (red clock) and stale (dimmed)
- analysis/label_health.go already computes freshness metrics
- Complexity: LOW

**2b. Epic progress indicator**
- Parent-child already tracked via DepParentChild
- Add "3/7 complete" counter to epic-type issues in list and detail
- Complexity: LOW

**2c. State dimension parsing**
- Labels already loaded - parse dimension:value patterns
- Display as structured badges in detail panel
- In global mode: state dashboard across projects
- Complexity: LOW (parsing) to MODERATE (global dashboard view)

**2d. Deferred visual treatment enhancement**
- Already has DEFR badge and colors
- Add defer/undefer actions in detail panel (shell to bd)
- Complexity: LOW

### Wave 3: P1 Medium-Effort Features
**3a. Swarm visualization**
- Requires shelling to `bd swarm validate --json` OR direct SQL for wave assignments
- Color graph nodes by wave number in existing ViewGraph
- Complexity: MODERATE (new data source + graph coloring)

**3b. Ship/provides capability map (global mode)**
- Scan labels for export:/provides:/external: patterns across databases
- Labels already loaded in global mode via UNION ALL
- New view or overlay showing project-to-project capability edges
- Complexity: MODERATE (new view, cross-project aggregation)

**3c. Query language bridge**
- BQL already fully implemented
- Bridge to bd query syntax OR add "bd query passthrough" mode
- Option 2 (passthrough via exec) is simpler
- Complexity: MODERATE

### Wave 4: P2 Nice-to-Have Features
- **4a. Formula/molecule browser** - Complex (new data loading from TOML/YAML files)
- **4b. Merge slot indicator** - Simple but niche (global mode footer)
- **4c. KV store viewer** - Simple (bd kv list --json)
- **4d. Duplicate detection surface** - Moderate (background bd find-duplicates)
- **4e. Wisp visibility toggle** - Simple (filter on ephemeral field, needs Wave 0)
- **4f. Version timeline** - Moderate (bd history --json parsing)
- **4g. Integration sync status** - Moderate (per-project bd tracker status)

### NOT NEEDED (already done or N/A)
- Rich markdown rendering: ALREADY COMPLETE (model_filter.go:727-748 + glamour v2)
- Batch operations: internal optimization, not surfacing concern
- Backup/restore: admin operations, not TUI appropriate

## Implementation Dependencies

```
Wave 0: Data Model Extension
  |-- Wave 1a: Gate Indicators (needs await_type, await_id, timeout)
  |-- Wave 1b: Human Flag (no model deps - uses Labels, but benefits from Wave 0 for gate distinction)
  
Wave 2: Independent of Wave 0 (uses existing model fields)
  |-- 2a: Stale/overdue (DueDate, UpdatedAt)
  |-- 2b: Epic progress (Dependencies)
  |-- 2c: State dimensions (Labels)
  |-- 2d: Defer enhancement (Status)

Wave 3: Mix of dependencies
  |-- 3a: Swarm viz (independent - bd CLI or SQL)
  |-- 3b: Ship/capability map (independent - Labels in global mode)
  |-- 3c: Query bridge (independent - existing BQL)

Wave 4: Various, mostly independent
  |-- 4e: Wisp toggle (needs Wave 0 ephemeral field)
```

## Key Insight: Wave 2 can run in parallel with Wave 0+1

Wave 2 features use EXISTING model fields (DueDate, Labels, Dependencies, Status). They can be implemented independently of the gate/molecule column extension. This means:
- Wave 0 + Wave 1 (gate plumbing + indicators): one work stream
- Wave 2 (stale, epic, state, defer): parallel work stream
- Both streams can start immediately

## Beads to Create (Step 4 guidance)

1. **Extend data model with gate and molecule columns** (Wave 0, P0, task)
   - Blocks Wave 1 items
   - Files: columns.go, dolt.go, global_dolt.go, types.go
   
2. **Update bt-c69c** with refined scope from this research (Wave 1, P0)
   - Already exists, add implementation details from this synthesis
   
3. **Stale/overdue visual indicators in list and detail** (Wave 2, P1, feature)
   - Independent, can start immediately

4. **Epic progress indicator** (Wave 2, P1, feature)
   - Independent, can start immediately

5. **State dimension parsing and display** (Wave 2, P1, feature)
   - Independent, can start immediately

6. **Swarm visualization in graph view** (Wave 3, P1, feature)
   - Depends on bd swarm --json availability

7. **Ship/provides capability map for global mode** (Wave 3, P1, feature)
   - Global mode only

8. **Wisp visibility toggle** (Wave 4, P2, feature)
   - Depends on Wave 0 ephemeral field

## Molecule Process Feedback

This is the first molecule executed in bt. Observations:
- Orient step was fast and focused - reading input docs + defining questions worked well
- Parallel research agents were highly effective - 3 angles completed simultaneously
- The angle-based decomposition (catalog/design/implementation) gave non-overlapping coverage
- Having file:line references from research agents made synthesis easy - no ambiguity
- The molecule structure (orient -> parallel research -> synthesize -> file) matches this task naturally
- One friction point: molecule beads do not have a built-in place to store angle-specific findings (used notes field on the parent research bead as a workaround)
- Suggestion for marketplace: research-audit formula should pre-create child beads for each angle under the research step, so agents can write findings to individual beads rather than one giant notes field
