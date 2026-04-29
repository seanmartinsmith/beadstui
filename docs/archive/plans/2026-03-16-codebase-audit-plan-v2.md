# Codebase Audit Plan (v2)

**Date**: 2026-03-16 (Session 15)
**Parent**: docs/brainstorms/2026-03-12-post-takeover-roadmap.md
**Previous version**: docs/archive/plans/2026-03-12-codebase-audit-plan.md (v1, session 13)
**Executes in**: Session A (scan) + Session B (synthesis)

> **Why v2?** Sessions 14-15 rewrote the Dolt lifecycle and fixed all 39 Windows
> test failures. The codebase state has changed materially since v1. This version
> updates factual context and orientation so a fresh audit session starts from
> accurate assumptions. The team structure (now 9 teams, split from the original 8) and report template are largely unchanged.

## Goal

Produce a complete inventory of every feature, module, and subsystem in the bt codebase. Each item gets factual scoring by agents. Human applies keep/cut/improve judgment in Session B.

The audit should also help prioritize 12 open beads issues (listed at the bottom) - are they still valid? Are there new issues the audit surfaces?

Output artifacts:
1. **Architecture map** - how the codebase is structured, what depends on what
2. **Feature inventory** - every user-facing feature cataloged with factual scores
3. **Module reports** - per-domain deep analysis from each audit team
4. **ADR-002 skeleton** - created during synthesis to become the new spine document

## Current Codebase State (as of session 15)

- **Production Go**: ~88k lines across ~200 non-test files
- **Tests**: ~102k lines across ~230 test files
- **Build**: `go build ./cmd/bt/` passes clean
- **Test suite**: `go test ./...` passes with **0 failures** across all 26 packages (fixed in session 15)
- **Key directories**: cmd/bt/, pkg/ui/, pkg/analysis/, pkg/export/, pkg/agents/, internal/datasource/, internal/doltctl/, pkg/drift/, tests/
- **Binary**: `bt` (installed via `go install ./cmd/bt/`)
- **Module**: `github.com/seanmartinsmith/beadstui`
- **Naming**: bv->bt rename is 100% complete (source, tests, env vars, CLI refs)
- **Dolt lifecycle**: bt auto-starts Dolt via `bd dolt start`, PID-based ownership shutdown (`internal/doltctl/`)
- **Data backends**: Dolt is the only active path. JSONL/SQLite code still exists but upstream beads removed those backends in v0.56.1.

### Files touched in sessions 14-15

These packages had active edits recently. Not conclusions about quality - just orientation for where recent changes landed:

- `internal/doltctl/` - new module (session 14): Dolt server lifecycle management
- `cmd/bt/main.go` - Dolt startup flow changes, flag definitions
- `cmd/bt/main_test.go`, `cmd/bt/main_robot_test.go` - test binary naming fixes
- `pkg/ui/model.go` - Dolt connection integration, EnsureServer calls
- `pkg/ui/tutorial_progress.go` - added configHome field for testability
- `pkg/analysis/plan.go` - fixed ComputeUnblocks to filter by blocking edge type
- `pkg/correlation/explicit.go` - fixed bv->bt in regex patterns, normalizeBeadID prefix
- `pkg/export/wizard.go` - added wizardConfigHome for testability
- `pkg/export/markdown_test.go` - fixed slug collision test data
- `pkg/agents/blurb.go` - fixed CLI command references (br->bd)
- `pkg/hooks/executor_test.go` - skip guards, CWD restore fix
- `pkg/testutil/assertions.go` - golden file \r\n normalization
- `tests/e2e/drift_test.go` - .exe suffix for Windows
- `tests/e2e/update_flow_test.go` - robot flag --double-dash fix

## Session A: The Scan

### Pre-flight

Before launching teams, the orchestrator should:
1. Read this audit plan for full context
2. Read the brainstorm (`docs/brainstorms/2026-03-12-post-takeover-roadmap.md`) for goals/decisions
3. Read `AGENTS.md` for project conventions (auto-imported from root `CLAUDE.md`)
4. Run `go build ./cmd/bt/` to confirm the build is clean
5. Run `go test ./... 2>&1 | tail -30` to confirm tests pass (expect 0 failures, ~26 packages)
6. Create `docs/audit/` directory for team reports

### Team Assignments

Launch 9 domain teams as parallel subagents. Each team gets:
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
- Panel/modal consistency - are they all using the same styling patterns? (`panel.go` has `RenderTitledPanel` - is it used consistently?)
- `tutorial_progress.go` - recently refactored to add configHome for testability. The singleton pattern (`GetTutorialProgressManager`) is worth noting.

---

#### Team 1b: UI Core & Interaction (`pkg/ui/` - state and behavior)
**Scope**: ~28k lines - the engine side of the UI
**Files**: model.go (8.3k), background_worker.go (2k), update handlers, keyboard handling, mouse handling, filter/search logic, snapshot management, triage_preservation.go, update_test.go

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
- Dolt connection state: `doltConnected` bool, `EnsureServer` calls, poll loop auto-reconnect after 3 failures
- `model.Stop()` - cleanup method that releases instance locks, stops watcher, stops Dolt if owned

---

#### Team 2: CLI & Entry Point (`cmd/bt/`)
**Scope**: ~9k lines
**Files**: main.go (8.1k) + any other files in cmd/bt/

**Key questions**:
- What CLI flags exist? Document ALL of them.
- What modes can bt run in? (normal, workspace, robot-search, robot-show, etc.)
- What's the startup flow? (flag parsing -> data loading -> Dolt lifecycle -> TUI init)
- What's the robot/agent mode? How does it work?
- What environment variables are used? List ALL BT_* vars.
- Are there dead flags or unreachable code paths?
- How is the Bubble Tea program initialized?
- What error handling exists at startup?

**Special attention to**:
- `main.go` at 8.1k lines - what's in there that shouldn't be?
- Robot mode - is it working? Is it useful?
- Flag parsing - note that `--recipe` has `-r` shorthand which conflicts with single-dash robot flags (fixed in session 15 to use `--robot-*` double-dash)
- Dolt startup: the flow now calls `doltctl.EnsureServer()` early, which auto-starts Dolt if not running
- TOON format output (--format=toon) - what is it, is it used?

---

#### Team 3: Analysis Engine (`pkg/analysis/`)
**Scope**: graph.go (2.9k), label_health.go (2.3k), triage.go (1.6k), plan.go, advanced_insights.go (966), + others
**Files**: All files in pkg/analysis/

**Key questions**:
- What analysis features exist? (graph metrics, triage scoring, label health, insights, plan generation, etc.)
- For each: what does it compute? What data does it need? Does it work with Dolt?
- Are these analyses exposed in the UI, or just computed but unused?
- What's the graph analysis? (PageRank, betweenness, etc. on the dependency graph)
- What's triage scoring? How does it prioritize issues?
- What's label health? What does it track?
- Are there performance concerns with large issue sets?
- What's `computeUnblocks` in plan.go? (recently fixed to filter by blocking dependency type only - not parent-child edges)

**Special attention to**:
- Which analyses are genuinely useful for a beads workflow vs academic exercises
- Data dependencies - do these need JSONL/SQLite, or do they work from the issue model?
- `triage.go` has 2 unused functions (`computeCounts`, `buildBlockersToClear`) per LSP diagnostics

---

#### Team 4: Export & Rendering (`pkg/export/`)
**Scope**: graph_render_beautiful.go (1.9k), markdown export, wizard, deploy, cloudflare, + others
**Files**: All files in pkg/export/

**Key questions**:
- What export formats exist? (Markdown, JSON, graph images, etc.)
- What does the "beautiful graph render" do?
- Are exports functional? Do they work with current data?
- Are there export features that are useful for agents (robot mode output)?
- What is the wizard? What does it configure? How does deploy/publish work?
- What is the Cloudflare Pages integration? GitHub Pages?

**Special attention to**:
- `wizard.go` is a large file - recently had wizardConfigHome added for testability
- Deploy/publish features (Cloudflare, GitHub Pages) - are these functional? Are they used?
- Golden file tests in this package use `pkg/testutil/assertions.go` GoldenFile helper (recently fixed for \r\n)

---

#### Team 5: Data Layer (`internal/datasource/`, `internal/dolt/`, `internal/doltctl/`, `internal/models/`)
**Scope**: ~3.6k lines in datasource, plus dolt/, doltctl/, and models/
**Files**: All files in internal/

**Key questions**:
- What data sources are supported? (JSONL, SQLite, Dolt)
- How does the Dolt reader work? (connection, queries, schema)
- What's the Issue model structure? What fields does it have?
- How does source detection work? (auto-detect vs explicit)
- Is there dead JSONL/SQLite code that should be removed now that Dolt is the only path?
- What Dolt tables/queries are used? What's available but not used? (dolt_log, dolt_diff, etc.)

**Special attention to**:
- The JSONL/SQLite code paths - upstream beads removed these backends in v0.56.1. Are they dead weight?
- Any hardcoded schema assumptions that might break with beads updates
- `internal/doltctl/` - new module added in session 14. EnsureServer(), StopIfOwned(), PID-based ownership. Port discovery chain: BEADS_DOLT_SERVER_PORT > BT_DOLT_PORT > .beads/dolt-server.port > config.yaml > 3307

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
- `blurb.go` contains the AgentBlurb constant injected into AGENTS.md files - recently fixed to use `bd` CLI refs instead of `br`

---

#### Team 7: Tests & Quality (`tests/`, `*_test.go` everywhere)
**Scope**: ~102k lines of tests
**Files**: All *_test.go files + tests/ directory

**Key questions**:
- Run `go test ./... 2>&1` first (takes ~8 minutes - kick it off before reading code). Expect 0 failures. Capture full output.
- Count tests per package - which packages have heavy coverage, which have sparse?
- Are there E2E tests? What do they test? How long do they take?
- Are there tests for dead/removed features that should be cleaned up?
- What test helpers/fixtures exist? (pkg/testutil/, proptest/)
- Are there tests that test internal implementation details vs behavior?

**Context from session 15** (for orientation, not conclusions - form your own assessment):
- The test suite was fixed across 7 categories: rename stragglers, logic bugs, path separators, config path testability, Unix permission skips, shell-dependent hooks skips, .exe suffix, golden file normalization
- Some tests use `t.Skip("requires Unix...")` on Windows - these are intentional platform guards, not failures
- `tests/e2e/` takes the longest (~7 minutes) due to binary compilation and deployment timeout tests

---

#### Team 8a: Supporting Packages (`pkg/` packages not covered by Teams 1-7)
**Scope**: All pkg/ directories not assigned to other teams.
**Files**: pkg/correlation/, pkg/hooks/, pkg/loader/, pkg/cass/, pkg/model/, pkg/recipe/, pkg/search/, pkg/metrics/, pkg/baseline/, pkg/drift/, pkg/instance/, pkg/watcher/, pkg/workspace/, pkg/updater/, pkg/debug/, pkg/util/

**Key questions**:
- For each package: what does it do? How big is it? Is it used? What depends on it?
- `pkg/correlation/` - git history correlation engine. What does it correlate? How? Is it useful?
- `pkg/hooks/` - hook system. What hooks exist? How are they triggered?
- `pkg/loader/` - data loading. How does it find and load beads data?
- `pkg/cass/` - what is this? (appears to be a Cass AI integration)
- `pkg/drift/` - configuration drift detection. What does it detect? Is it used?
- `pkg/recipe/` - what are recipes? How do they work?
- `pkg/updater/` - self-update mechanism. Does it work?
- `pkg/instance/` - instance locking. How does it prevent multiple bt instances?
- `pkg/watcher/` - file watching. What does it watch?

**Special attention to**:
- `pkg/correlation/` is substantial (~several thousand lines with explicit matching, caching, incremental correlation, file indexing, orphan detection, causality analysis, cocommit analysis, reverse correlation). Is this complexity justified?
- Cross-cutting dependencies - which of these packages are used by many others vs only one consumer?

---

#### Team 8b: Build, Config & WASM
**Scope**: Build pipeline, configuration, and anything not covered by other teams.
**Files**: bv-graph-wasm/, go.mod, go.sum, .goreleaser.yml, Makefile, .github/workflows/ci.yml, and any other root-level files or directories not covered by Teams 1-8a

**Key questions**:
- `bv-graph-wasm/` - still has old `bv` prefix. Is it wired into anything? Does it build? Is it a separate Go module?
- What Go dependencies are in go.mod? Flag any that look unused or concerning.
- What's the build/release pipeline? (goreleaser config, CI, Makefile targets)
- Are there any stale GitHub Actions or CI config?
- Are there config files, scripts, or docs not covered elsewhere?

**Special attention to**:
- `bv-graph-wasm/` - is it wired into anything or is it dead weight?
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
1. Verify all 9 reports saved to `docs/audit/`
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

## Open Beads (for cross-reference during audit)

The audit should help validate or reprioritize these:

| ID | Pri | Type | Title |
|----|-----|------|-------|
| bt-xavk | P1 | feature | Redesign help system: layered, task-oriented, context-aware |
| bt-95mp | P2 | bug | Alerts panel needs visual refresh (rounded borders, theme alignment) |
| bt-b8gl | P2 | bug | Status bar filter badge (ALL/OPEN/etc.) hard to read |
| bt-ks0w | P2 | feature | Mouse click support: panel focus + issue selection |
| bt-lgbz | P2 | bug | Card expand broken when empty columns are visible |
| bt-spzz | P2 | feature | Smarter reload status: show what changed |
| bt-2bns | P3 | feature | Center divider in details pane |
| bt-46fa | P3 | feature | Redesign issue list column header |
| bt-79eg | P3 | task | Audit test suite for beadstui takeover relevance |
| bt-pfic | P3 | task | Audit bt CLI ergonomics for agent/robot usage |
| bt-thpq | P3 | feature | Investigate Dolt changelog/history view in bt |
| bt-ztrz | P3 | feature | Investigate: manual Dolt refresh/reconnect keybind |
