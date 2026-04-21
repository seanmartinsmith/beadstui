# Codebase Audit Plan

**Date**: 2026-03-12 (Session 13)
**Parent**: docs/brainstorms/2026-03-12-post-takeover-roadmap.md
**Executes in**: Session A (scan) + Session B (synthesis)

## Goal

Produce a complete inventory of every feature, module, and subsystem in the bt codebase. Each item gets factual scoring by agents. Human applies keep/cut/improve judgment in Session B.

Output artifacts:
1. **Architecture map** - how the codebase is structured, what depends on what
2. **Feature inventory** - every user-facing feature cataloged with factual scores
3. **Module reports** - per-domain deep analysis from each audit team
4. **ADR-002 skeleton** - created during synthesis to become the new spine document

## Codebase Overview

- **Production Go**: ~88k lines across ~200 non-test files
- **Tests**: ~102k lines across ~230 test files
- **Key directories**: cmd/bt/, pkg/ui/, pkg/analysis/, pkg/export/, pkg/agents/, internal/datasource/, pkg/drift/, tests/

## Session A: The Scan

### Pre-flight

Before launching teams, the orchestrator should:
1. Read this audit plan (`docs/plans/2026-03-12-codebase-audit-plan.md`) for full context
2. Read the brainstorm (`docs/brainstorms/2026-03-12-post-takeover-roadmap.md`) for goals/decisions
3. Read `AGENTS.md` for project conventions (auto-imported from root `CLAUDE.md`)
4. Run `go build ./cmd/bt/` to confirm the build is clean
5. Run `go test ./... 2>&1 | tail -20` to get current test status
6. Create `docs/audit/` directory for team reports

### Team Assignments

Launch 8 domain teams as parallel subagents. Each team gets:
1. Their domain-specific instructions (below)
2. The **Report Template** (at the end of this section) - each team MUST produce output in this format
3. Instruction to save their report to `docs/audit/team-{id}-{domain}.md`

---

#### Team 1a: UI Views & Rendering (`pkg/ui/` - visual features)
**Scope**: ~30k lines - the view/rendering side of the UI
**Files**: board.go, tree.go, graph.go, insights.go, history.go, tutorial.go, flow_matrix.go, panel.go, theme_loader.go, helpers.go, and any other view-specific files

**Key questions**:
- What views/modes exist? (list, board, graph, tree, insights, history, flow matrix, etc.)
- For each view: is it functional? Does it render correctly? Is it wired to Dolt data?
- What modals/overlays exist? (help, alerts, agent prompt, cass session, etc.)
- What styling/theming is used? (Lipgloss styles, theme tokens, hardcoded values)
- Flag any dead code paths (unreachable views, unused renderers)
- Which views feel finished vs half-baked?

**Special attention to**:
- `tutorial.go` - 2.5k lines. What does the tutorial do? How does it work? Is it interesting?
- `history.go` - 3.4k lines. Does it use Dolt history features?
- Panel/modal consistency - are they all using the same styling patterns?

---

#### Team 1b: UI Core & Interaction (`pkg/ui/` - state and behavior)
**Scope**: ~28k lines - the engine side of the UI
**Files**: model.go (8.3k), background_worker.go (2k), update handlers, keyboard handling, mouse handling, filter/search logic, snapshot management

**Key questions**:
- What's the component architecture? (Model, Update, View pattern + sub-components)
- How is state managed? (focused panel, filters, selection, etc.)
- What keyboard shortcuts exist? Map ALL of them.
- What mouse support exists?
- How does the snapshot/poll system work? Document the data flow.
- What message types flow through the Update loop?
- Flag any files > 500 lines that could be split

**Special attention to**:
- `model.go` - it's 8.3k lines. What's in there vs what should be extracted?
- `background_worker.go` - the poll/snapshot engine. Document the data flow.
- State complexity - how many independent state variables does the Model have?

---

#### Team 2: CLI & Entry Point (`cmd/bt/`)
**Scope**: 9k lines
**Files**: main.go (8.1k) + any other files in cmd/bt/

**Key questions**:
- What CLI flags exist? Document ALL of them.
- What modes can bt run in? (normal, workspace, robot-search, robot-show, etc.)
- What's the startup flow? (flag parsing -> data loading -> TUI init)
- What's the robot/agent mode? How does it work?
- What environment variables are used? List ALL BT_* vars.
- Are there dead flags or unreachable code paths?
- How is the Bubble Tea program initialized?
- What error handling exists at startup?

**Special attention to**:
- `main.go` at 8.1k lines - what's in there that shouldn't be?
- Robot mode - is it working? Is it useful?
- Flag parsing - are there redundant or confusing flags?

---

#### Team 3: Analysis Engine (`pkg/analysis/`)
**Scope**: graph.go (2.9k), label_health.go (2.3k), triage.go (1.6k), advanced_insights.go (966), + others
**Files**: All files in pkg/analysis/

**Key questions**:
- What analysis features exist? (graph metrics, triage scoring, label health, insights, etc.)
- For each: what does it compute? What data does it need? Does it work with Dolt?
- Are these analyses exposed in the UI, or just computed but unused?
- What's the graph analysis? (PageRank, betweenness, etc. on the dependency graph)
- What's triage scoring? How does it prioritize issues?
- What's label health? What does it track?
- Are there performance concerns with large issue sets?

**Special attention to**:
- Which analyses are genuinely useful for a beads workflow vs academic exercises
- Data dependencies - do these need JSONL/SQLite, or do they work from the issue model?

---

#### Team 4: Export & Rendering (`pkg/export/`)
**Scope**: graph_render_beautiful.go (1.9k), markdown export, + others
**Files**: All files in pkg/export/

**Key questions**:
- What export formats exist? (Markdown, JSON, graph images, etc.)
- What does the "beautiful graph render" do?
- Are exports functional? Do they work with current data?
- Are there export features that are useful for agents (robot mode output)?

---

#### Team 5: Data Layer (`internal/datasource/`, `internal/dolt/`, `internal/models/`)
**Scope**: ~3.6k lines in datasource, plus dolt/ and models/
**Files**: All files in internal/

**Key questions**:
- What data sources are supported? (JSONL, SQLite, Dolt)
- How does the Dolt reader work? (connection, queries, schema)
- What's the Issue model structure? What fields does it have?
- How does source detection work? (auto-detect vs explicit)
- Is there dead JSONL/SQLite code that should be removed now that Dolt is the only path?
- What Dolt tables/queries are used? What's available but not used? (dolt_log, dolt_diff, etc.)

**Special attention to**:
- The JSONL/SQLite code paths - upstream beads removed these backends in v0.56.1
- Any hardcoded schema assumptions that might break with beads updates

---

#### Team 6: Agents & Automation (`pkg/agents/`)
**Scope**: ~2.9k lines across 15 files
**Files**: All files in pkg/agents/

**Key questions**:
- What does the agents package do?
- How does AGENTS.md / SKILL.md integrate?
- Is this the robot mode backend?
- What agent capabilities are defined?
- Is this package actively used or vestigial?

---

#### Team 7: Tests & Quality (`tests/`, `*_test.go` everywhere)
**Scope**: ~102k lines of tests
**Files**: All *_test.go files + tests/ directory

**Key questions**:
- Run `go test ./... 2>&1` and capture full output (note: may take several minutes on 102k lines - use a 10-minute timeout)
- How many tests pass? How many fail? Categorize failures (Windows path issues, missing Dolt, dead features, etc.)
- What's the test coverage distribution? (which packages have good coverage, which have none?)
- Are there E2E tests? What do they test?
- Are there tests for dead/removed features that should be cleaned up?
- What test helpers/fixtures exist?

**Special attention to**:
- The 10 known pre-existing Windows path failures
- Tests that reference old bv/beads_viewer naming
- Tests for JSONL/SQLite paths (should they be removed?)

---

#### Team 8: Miscellaneous & Config (`pkg/drift/`, `bv-graph-wasm/`, root files)
**Scope**: pkg/drift/, bv-graph-wasm/, go.mod, .goreleaser.yml, Makefile, ci.yml, and any other files not covered by Teams 1-7
**Files**: Everything not in the other teams' scope

**Key questions**:
- What is `pkg/drift/`? What does it do? Is it used?
- What is `bv-graph-wasm/`? Is it a separate build target? Does it work? Is it worth keeping?
- What Go dependencies are in go.mod? Flag any that look unused or concerning.
- What's the build/release pipeline? (goreleaser config, CI, Makefile targets)
- Are there any config files, scripts, or docs not covered elsewhere?
- Any stale GitHub Actions or CI config?

**Special attention to**:
- `bv-graph-wasm/` still has the old `bv` prefix - is it wired into anything?
- Dependency audit - are there heavy deps pulled in for features we might cut?

---

### Report Template

Each team produces a report in this format, saved to `docs/audit/team-N-domain.md`:

```markdown
# Audit Report: [Domain Name]

**Team**: [N]
**Scope**: [files and directories]
**Lines scanned**: [N]

## Architecture Summary

[2-3 paragraphs: how this domain is structured, key abstractions, data flow]

## Feature Inventory

| Feature | Location | LOC | Dolt-Compatible | Tested | Functional | Notes |
|---------|----------|-----|-----------------|--------|------------|-------|
| [name]  | [file:line] | [N] | Yes/No/Partial | Yes/No/Partial | Yes/No/Broken | [brief note] |

## Dependencies

- **Depends on**: [what this domain imports/uses]
- **Depended on by**: [what imports/uses this domain]

## Dead Code Candidates

[List of functions, files, or code paths that appear unused or unreachable]

## Notable Findings

[Anything surprising, interesting, or concerning. Hidden gems, subtle bugs, architectural issues.]

## Questions for Synthesis

[Things the team couldn't determine that need human judgment or cross-team context]
```

### Session A Wrap-Up

After all teams report, the orchestrator should:
1. Verify all 8 reports saved to `docs/audit/`
2. Read each report and synthesize `docs/audit/architecture-map.md` - a high-level diagram of how domains connect, data flows, and cross-domain dependencies
3. Update ADR-001 changelog with session A entry
4. Commit all audit artifacts to git
5. Summarize what's ready for Session B synthesis

## Session B: Synthesis

### Pre-flight

1. Read all team reports from `docs/audit/`
2. Read the brainstorm doc for context on goals/phases
3. Pull up the open beads list for reference

### Review Process

Go through each team's feature inventory with sms. For each feature:
- Agent presents the facts (from the report)
- sms gives the verdict: **KEEP**, **CUT**, **IMPROVE**, **INVESTIGATE**
- Capture rationale for non-obvious decisions

### Output

1. **Feature inventory with verdicts** - annotated version of the combined inventories
2. **ADR-002 skeleton** - the new spine document:
   - Stream 1: Cleanup (cut dead code, remove dead features)
   - Stream 2: Stabilize (fix what we're keeping, ensure Dolt compatibility)
   - Stream 3: UX Polish (visual consistency, interaction improvements)
   - Stream 4: Interactive (CRUD from TUI)
   - Stream 5: Aspiration (features that make bt the official companion)
3. **Updated beads** - new issues for work items discovered, existing issues updated/closed
4. **Epic(s)** - group related beads under epics if the scope warrants it

### Alignment Check

At the end of Session B, confirm:
- [ ] Do the phase priorities still make sense given what we found?
- [ ] Are there any blocking discoveries that change the roadmap?
- [ ] Is the ADR-002 structure right?
- [ ] What's the first work item to tackle in the next implementation session?
