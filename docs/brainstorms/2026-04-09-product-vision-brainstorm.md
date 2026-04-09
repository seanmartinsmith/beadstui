# Product Vision Brainstorm: bt v1

**Date**: 2026-04-09
**Status**: Active (living document, updating as dogfood continues)
**Context**: Dogfooding session driving product vision crystallization

---

## What We're Building

**bt** is the frontend to beads. Beads is the backend (Dolt-powered, Go, CLI-first issue tracker with first-class dependency support). bt extends beads where beads deliberately stops: no built-in UI, no cross-project aggregation, no visual analytics.

### Three Pillars

1. **TUI** - Daily driver for managing work visually. Built for the maintainer's workflow first, production-quality for eventual public release.
2. **CLI/Robot Outputs** - Agent-facing capabilities that `bd` doesn't ship natively, particularly the global cross-project layer.
3. **Analytics** - Insights, graph analysis, triage scoring, flow matrices - capabilities that only make sense with a visual/aggregate layer on top of the raw issue data.

### What bt Is NOT

- Not a beads fork - bt never writes to Dolt directly, shells out to `bd` for all writes
- Not competing with beads - extending it. Beads is the engine, bt is the dashboard + global plugin
- Not a general-purpose project manager - specifically built for beads-powered workflows

---

## Product Identity

| Layer | Tool | Role |
|-------|------|------|
| Storage | Dolt (via beads) | SQL database, version control, replication |
| Backend | `bd` (beads CLI) | Issue CRUD, dependency management, federation, hooks |
| Frontend | `bt` (beadstui) | TUI, global aggregation, analytics, agent CLI outputs |

bt's unique value: **the global layer**. Individual projects use `bd`. Cross-project visibility, aggregation, search, and analytics - that's bt's territory.

---

## Current State Assessment

### What Works Well
- 10+ views (board, list, graph, tree, insights, history, flow matrix, etc.)
- BQL composable search (parser, memory executor, TUI modal, CLI flags)
- Global mode TUI (`--global` flag, `GlobalDoltReader`, workspace mode with repo picker)
- Theme system (3-layer YAML merge, Tomorrow Night palette)
- Background worker with Dolt polling, freshness monitoring, auto-reconnect
- Robust robot-mode JSON outputs for agents (20+ `--robot-*` flags)
- Cross-platform (Windows, macOS, Linux), 268 test files, 27 packages

### What's Broken or Missing

#### Architecture
- **model.go is 8,366 lines** with ~90 fields on the Model struct
- **2,300+ line Update() switch** - single function longer than most packages
- **Mutually exclusive booleans** instead of ViewMode enum (isBoardView, isGraphView, etc.)
- **Two competing panel rendering approaches** (hand-drawn box chars vs Lipgloss borders)
- **Sprint view pattern violation** - method on Model instead of standalone model like every other view
- **Dead code and stale references** (bv- prefixes in comments, old CLI names in tutorial)

#### Charm v1 EOL
- All Charm dependencies on v1, v2 has shipped
- 76 files importing Charm libraries need migration
- 161 AdaptiveColor occurrences (biggest single migration item, architectural)
- Scout report complete: `docs/audit/charm-v2-migration-scout.md`

#### No Write Operations
- TUI is 100% read-only today
- CRUD design decided (shell out to `bd`, poll Dolt for changes) but not implemented
- Zero `exec.Command("bd", ...)` calls in the UI package for writes

#### Global Mode CLI Gap
- `bt --global` launches TUI - works
- `bt --global --robot-*` outputs JSON - works for agents
- **No human-readable CLI output** for global mode (no `bt --global --list` table output)
- No cross-project health dashboard
- No `EnumerateDatabases()` CLI surface (admin: list projects on server)
- bt has two output modes (TUI or robot JSON) - missing the middle ground of "just print a table"

#### Beads Dependency Tracking
- Beads has had 30+ releases in 4 months, rapidly evolving
- No automated way to detect upstream breaking changes
- Schema version compatibility checked at runtime but no proactive monitoring
- Need periodic changelog/release analysis to surface compatibility deltas

---

## Key Decisions Made

### 1. Refactor + Charm v2 = One Work Stream
The Charm v2 migration forces touching 76 files. The refactor targets the same files. Doing them separately means touching everything twice. Strategy:
- **Prep pass**: Mechanical changes (import paths, KeyMsg rename, View() signature) as one commit
- **Structural pass**: Extract + migrate together file by file. When pulling model_footer.go out of model.go, also convert its 39 global NewStyle() calls. When redesigning Theme struct, kill AdaptiveColor and move to LightDark(isDark).
- **AdaptiveColor**: Redesign theme system (not compat shim) since we're restructuring anyway

### 2. CRUD Without Waiting for Full Refactor
CRUD can be added incrementally alongside refactoring:
- Start with status changes (single keystroke -> `bd update/close` -> status toast -> poll refresh)
- Then create modal (follows BQL modal pattern)
- Then inline edit (most complex, builds on proven bd-exec-poll loop)
- New `internal/bdcli/` package (~100 lines) to centralize `bd` command execution

### 3. Refactor Is About Ownership
Beyond code quality, the refactor transforms the codebase from Jeffrey's fork into the maintainer's project. Every structural decision replaces inherited assumptions. This matters for:
- Project identity and direction
- Contributor readiness (human or agent)
- Long-term maintainability
- Reducing cognitive load when navigating code

### 4. Beads Is the Backend, bt Adapts
- Maintainer is a beads contributor - can influence upstream within reason
- bt doesn't try to change beads' direction, builds on top of what beads exposes
- Dolt as shared storage enables the global layer without beads changes
- Federation (`bd federation`) exists upstream for peer-to-peer sync

---

## Vision: Main Board/List View

The main UI page must be "as good as it can be" - this is the first thing users see and where most work happens.

### Current Board View
- 4-column kanban (Open, In Progress, Blocked, Closed)
- Swimlane modes (by status, priority, type)
- Card format: priority icon, type badge, status color, ID, title, dependency indicators
- Inline card expansion, detail panel toggle (tab)
- Board search (`/`), BQL modal (`:`)
- Filter badges in footer

### What Needs to Change for CRUD
- **Status changes**: Direct key actions on selected card (claim, close, reopen)
- **Create**: Modal form (title, type, priority, description) - follows BQL modal UX pattern
- **Quick edit**: Inline title editing on selected card
- **Full edit**: Open in detail panel with field-by-field editing, or shell to $EDITOR
- **Dependency management**: Visual add/remove from graph or board view
- **Feedback loop**: Status toast -> Dolt poll -> board refresh (0-5s latency)

### Aesthetic/UX Notes
*(Accumulating from dogfood - update as more observations come in)*

- TBD: Card density vs readability tradeoff
- TBD: Detail panel width and content rendering
- TBD: Color contrast and accessibility
- TBD: Keyboard discoverability (40+ keybindings, 22 undocumented)

---

## Vision: Global Mode

### TUI (exists, needs polish)
- Cross-project board with repo picker filtering
- Aggregate stats across all projects
- Visual health dashboard (which projects are blocked, stale, etc.)

### CLI for Agents (priority - build first)
- `bt -g robot bql "query"`: cross-project BQL (exists as flags, migrate to subcommand)
- `bt -g robot search "query"`: cross-project search (exists)
- `bt -g robot metrics`: cross-project metrics (exists)
- Gap: No robot output for database enumeration or project-level health
- Prioritize what's actually useful for LLMs, document gaps for future

### CLI for Humans (future - document now, build later)
- `bt -g ls`: human-readable table of issues across projects
- `bt -g stats`: aggregate statistics printed to terminal
- `bt -g health`: cross-project health summary
- `bt -g databases`: list all beads projects on shared server
- Design the UX now, implement after agent outputs are solid

---

## Vision: Beads Dependency Monitoring

Need a systematic way to stay current with upstream beads:
- Periodic job or bt subcommand that pulls beads changelog/release notes
- Diff against what bt currently supports (schema version, CLI flags, features)
- Surface a "compatibility report" - new tables, deprecated flags, schema migrations
- Could be a self-updating beads issue: "upstream at v1.2.3, bt verified against v1.0.0"
- Maintainer is a beads contributor, so can influence upstream for needed features

---

## Resolved Questions

1. **Subcommand vs flag UX**: **Yes, move to subcommands.** Target: `bt -g ls`, `bt global search`, etc. Migrate from 170-flag pflag monolith to cobra. This is part of the refactor stream (bt-if3w) - CLI architecture is as monolithic as the TUI. `-g` shorthand for `--global` already works via pflag. Medium effort (2-3 agent sessions to scaffold + migrate).

2. **Human-readable CLI priority**: **Prioritize agent (robot) outputs first.** Figure out what's actually useful for LLMs, build those, document human-readable CLI as a future adaptation. Don't lose context on the human side but don't build it yet.

3. **Global mode as default**: **No - project-first.** If you're in a folder with `bd init` / `.beads/`, show that project. Don't pollute with everything. Global mode is opt-in (`bt -g` or `bt global`) UNLESS you're in a folder without beads, then global is the natural fallback. Current auto-global behavior (auto-detect shared server, auto-filter to current project) is correct. Explicit `--global` / `-g` for "show me everything."

4. **Refactor scope**: **Proper.** Go best practices, not half-measures. ViewMode enum, proper sub-models, message bus, cobra CLI, the works. This is a Go-only codebase. Do it right.

5. **Release timing**: **Already public** (github.com/seanmartinsmith/beadstui). Marketing starts once it's further along - likely by engaging in the beads GitHub repo community first. No gate on refactor/Charm v2 for incremental releases, but the marketing push waits.

## Open Questions

*(None currently - all resolved. New questions will be added as they arise.)*

## Resolved Questions (continued)

6. **Project navigation UX**: **Two separate UIs.** (a) Quick switcher: modal overlay above main screen, select project, press enter, switches to that project exclusively. Same overlay pattern as current filter but single-select behavior. (b) Filter: separate UI for multi-select filtering, with project categories/groups support. Current picker tries to be both and fails at both. See bt-s4b7.

7. **CWD and write context**: **bt cds to the project it's looking at.** When you switch to project A, bt internally changes CWD to project A's directory. No ambiguity about where `bd` writes go. bt needs to know project paths (discoverable from Dolt database names + config or a project registry). See bt-s4b7.

8. **Project categorization**: **Yes.** Projects support groups/tags for filtering (e.g. "tools": bt, cass, tpane; "content": portfolio, world). Ties into the filter UI redesign. Categories could be user-defined in bt config or auto-derived from project metadata.

---

## Work Streams (Rough Priority Order)

1. **Refactor + Charm v2 Migration** - One combined stream, file by file
2. **CRUD from TUI** - Status changes first, then create modal, then inline edit
3. **Global CLI Outputs** - Human-readable table/stats/health for global mode
4. **Beads Dependency Monitoring** - Automated compatibility tracking
5. **Main UI Polish** - Help system redesign, card expand bugs, keyboard discoverability
6. **BQL Completion** - Syntax highlighting, status redirect, recipes as BQL
7. **Test Suite Fixes** - Windows path panic, stale strings
8. **README and Docs** - Draft exists, needs review and accuracy pass

---

*This document is a living brainstorm. Updated as dogfood observations accumulate.*
