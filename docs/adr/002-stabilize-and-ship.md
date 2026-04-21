---
title: "ADR-002: Stabilize and ship"
status: active
date: 2026-04-03
updated: 2026-04-14
decision-makers: [seanmartinsmith]
---

# ADR-002: Stabilize and ship

## Status

**Active** - this is the current project spine. Supersedes [ADR-001](001-btui-fork-takeover.md) (fork takeover, completed).

## Context

The fork takeover is done. bt is a working TUI with BQL search, Dolt lifecycle management, theme system, and robot mode. The codebase has been audited, cleaned, renamed, and all tests pass cross-platform.

This ADR tracks the path from "working fork" to "shippable product."

## Development Arc

```
[DONE] Fork takeover -> [DONE] Audit -> [DONE] Stabilize -> [HERE] Polish -> Ship CRUD
```

Note: Charm v2 migration was originally a separate future phase but was completed as part of stabilization (Phase 0-3). The arc is simpler than originally planned.

## Work Streams

### Stream 1: Robot-mode hardening
**Status**: Partially done
**Priority**: P1
**Bead**: bt-0cht
**Foundation**: `docs/audit/cli-ergonomics-audit.md`, `docs/design/2026-04-20-bt-mhwy-1-compact-output.md`

The robot API is bt's primary agent interface. Phase 3 (Cobra migration) resolved the CLI structure issues. Robot log suppression fixed (2026-04-10). Compact output landed 2026-04-20 (bt-mhwy.1): default `bt robot list` dropped from 383KB to 38KB on 100 issues. Remaining items are envelope consistency and documentation.

Scope:
- [x] Migrate CLI from pflag to cobra subcommands (Phase 3, 2026-04-10)
- [x] Suppress log output in robot mode to prevent JSON corruption (2026-04-10)
- [x] Fix --robot-bql envelope (session 18, 2026-04-03)
- [x] Compact output shape default across 17 robot subcommands; `pkg/view/` projection package with versioned schema (bt-mhwy.1, 2026-04-20)
- [x] External dep resolution for cross-project graph analysis (bt-mhwy.5, 2026-04-20)
- [x] Portfolio subcommand — per-project health aggregates (bt-mhwy.4, 2026-04-20)
- [x] Pairs subcommand — cross-project paired bead detection with drift flags (bt-mhwy.2, 2026-04-20)
- [x] Refs subcommand — cross-project reference validation with broken/stale/orphaned_child flags (bt-mhwy.3, 2026-04-21)
- [x] Source filter — persistent `--source <project>[,<project>...]` robot flag scopes output to projects (bt-mhwy.6, 2026-04-21)
- [x] Pairs v2 — intent-based identity via cross-prefix dep edges (BFS over connected components); `--schema=v2` routing (bt-gkyn, 2026-04-21)
- [x] Refs v2 — intent-based identity via syntactic sigils (hand-rolled tokenizer + three recognizer modes); default `--schema` flipped to v2 for both pairs and refs (bt-vxu9, 2026-04-21)
- [ ] Fix envelope bypass in robot-search, robot-diff
- [ ] Add positional arg warning/support
- [ ] Document all 18 missing env vars in --robot-docs env
- [ ] Consolidate duplicate confidence/agent-count flags

### Stream 2: BQL completion
**Status**: Core shipped, bugs fixed, features remain
**Priority**: P2
**Bead**: bt-faaw (highlighting), bt-sytt (recipes)
**Foundation**: `docs/audit/bql-gap-analysis.md`

BQL parser, memory executor, TUI modal, and CLI flags are shipped. All five bugs from gap analysis fixed (2026-04-03).

Bugs:
- [x] Add ValidStatusValues to validator - session 18
- [x] Fix date equality (truncate to day) - session 18
- [x] Add ISO date parsing to lexer - session 18
- [x] Remove readySQL dead code from sql.go - session 18
- [x] Fix --robot-bql envelope - session 18

Remaining features:
- [ ] Syntax highlighting in BQL modal (bt-faaw)
- [ ] Status key redirect through BQL (design decision needed)
- [ ] Recipes as saved BQL queries (bt-sytt - high effort, needs architecture decision)

### Stream 3: Test suite fixes
**Status**: Partially done
**Priority**: P2
**Bead**: bt-5dvl, bt-t82t (Phase 4)
**Foundation**: `docs/audit/test-suite-audit.md`

Phase 0.5 (2026-04-10) built the test foundation for the refactor. Cross-platform fixes shipped in sessions 14-15. Some items remain.

- [x] Cross-platform path separator fixes (2026-03-16)
- [x] Golden file line ending normalization (2026-03-16)
- [x] e2e binary .exe suffix + file locking (2026-03-16)
- [x] Hooks test cross-platform (2026-03-16)
- [x] Test foundation for pkg/ui refactor (Phase 0.5, 2026-04-10)
- [x] Update tests for cobra subcommand syntax (Phase 3, 2026-04-10)
- [ ] Fix Windows path length panic in e2e export copyDirRecursive (P1)
- [ ] Fix remaining stale error message strings referencing old br/bv names (P2)
- [ ] Stale golden files and test validation (bt-t82t Phase 4)

### Stream 4: Charm v2 migration
**Status**: DONE
**Priority**: Complete
**Completed**: 2026-04-09 through 2026-04-10

Executed as a 4-phase refactor plan (`docs/plans/` has the execution plan):

- [x] **Phase 0** (2026-04-09): Mechanical Charm v2 migration - import paths, API renames, compat bridge for AdaptiveColor. 76 files updated.
- [x] **Phase 0.5** (2026-04-10): Test foundation - coverage for refactor safety net.
- [x] **Phase 1** (2026-04-10): Model decomposition - ViewMode enum (1.1), DataState/FilterState extraction (1.2), ModalType enum (1.3), Update() handler decomposition (1.4).
- [x] **Phase 2** (2026-04-10): Kill AdaptiveColor - 174 occurrences eliminated, all colors resolved to `color.Color` at load time. Dark mode detection via `tea.BackgroundColorMsg`. `adaptive_color.go` deleted.
- [x] **Phase 3** (2026-04-10): Cobra CLI migration - main.go from 1,708 to 13 lines. 35+ robot subcommands migrated to `bt robot *`.

**Deferred to Phase 4 (bt-t82t)**: Pre-compute hot-path styles, footer extraction as FooterData component.

### Stream 5: README and docs
**Status**: Draft ready, needs review
**Priority**: P2
**Bead**: bt-iuqy
**Foundation**: `docs/drafts/README-draft.md`

README prose rewrite drafted. Needs review against current state (the codebase has changed significantly since the draft was written).

- [ ] Review draft accuracy against current codebase
- [ ] Verify screenshot references exist
- [ ] Diff against current README.md
- [ ] Replace when verified

### Stream 6: Polish (UX bugs and visual fixes)
**Status**: Active - dogfooding in progress
**Priority**: P2-P3

Accumulated dogfood findings. Active work as of 2026-04-14.

Recent completions (2026-04-14 dogfood session):
- [x] Project filter picker redesign - rounded borders, ✓/• indicators, overlay compositing (bt-s4b7)
- [x] Fix project filter matching - use SourceRepo instead of ID prefix parsing (bt-dcby related)
- [x] ANSI-aware overlay system using charmbracelet/x/ansi

Recent completions (2026-04-13):
- [x] Gate/human/stale badge indicators in list and detail views
- [x] Wisp visibility toggle
- [x] Swarm wave visualization in graph view
- [x] Capability map in detail panel
- [x] Security hardening (URL injection, BQL recursion, SQL parameterization)

Open items:
- bt-s4b7 (P1): Project navigation redesign - quick switcher, categories (partially done - filter picker redesigned, full redesign ongoing)
- bt-xavk (P1): Help system redesign - plan exists at docs/plans/help-system-redesign.md
- bt-m9te (P2): Footer status bar rethink - layout, notifications, information hierarchy
- bt-dcby (P1): Global mode features don't respect project filter (filter matching fixed, child bugs may remain)
- bt-eorx (P1): Label picker freezes terminal when typing filter text
- bt-y0fv (P2): Responsive layout for small terminals
- bt-gf3d (P2): Keybinding consistency overhaul

Full list: `bd list --status=open` (50 items)

### Stream 7: CRUD from TUI
**Status**: Not started (design decided)
**Priority**: Deferred until polish is solid
**Bead**: bt-oiaj
**Decision**: Shell out to `bd` for writes, poll Dolt for changes. No beads fork needed.

This is the end goal - making bt interactive, not just a viewer. CWD tracking for multi-project writes designed as part of bt-s4b7 project navigation work.

### Stream 8: Cross-project features (added 2026-04-12)
**Status**: Infrastructure shipped, features in progress
**Priority**: P1-P2
**Beads**: bt-ushd (epic), bt-ssk7 (federation), bt-53du (feature surfacing)

New stream emerged from global mode work. Core infrastructure:
- [x] Global Dolt data layer with UNION ALL federation (2026-04-03)
- [x] Auto-detect shared Dolt server, default to global mode (2026-04-08)
- [x] Temporal cache with Dolt AS OF queries (2026-04-12)
- [x] Data model: gate/molecule columns, SourceRepo, capabilities (2026-04-13)
- [x] Feature surfacing: 4-wave plan fully executed (2026-04-13)

Open:
- bt-koz8 (P1): Dolt-native cross-repo data hydration
- bt-ssk7 (P1): Cross-database federation improvements
- bt-ghbl (P2): No raw SQL against shared server
- bt-ammc (P2): User-facing docs for global mode

## Decisions

### Decided

| Decision | Choice | Rationale | When |
|----------|--------|-----------|------|
| CRUD approach | Shell out to `bd` | No beads fork needed. Poll Dolt for changes. | ADR-001 |
| Charm v2 migration | 4-phase (mechanical, tests, decompose, kill adaptive, cobra) | Lower risk than big-bang. Each phase independently verifiable. | 2026-04-09 |
| Robot-mode CLI | Cobra subcommands (`bt robot *`) | Clean break from `--robot-*` flags. Pre-alpha, one consumer. | 2026-04-10 |
| Color system | Resolved `color.Color` at load time | Eliminates runtime light/dark branching. Simpler than AdaptiveColor. | 2026-04-10 |
| Project filter matching | Use `issue.SourceRepo` not ID prefix | Database name is authoritative. ID prefix parsing is fragile (mkt vs marketplace). | 2026-04-14 |
| Project filter overlay | ANSI-aware compositing over background | Uses `charmbracelet/x/ansi` Truncate/TruncateLeft. Preserves bg colors. | 2026-04-14 |

### Open

| Decision | Context | Blocking |
|----------|---------|----------|
| Status key BQL redirect | Pressing 'o' for OPEN - should it become a BQL filter? How to handle multi-status groups? | bt-faaw |
| Recipes as BQL migration | Replace YAML recipe system with saved BQL queries, or keep both? | bt-sytt |
| Footer information hierarchy | What goes in the status bar? Transient messages, project context, issue counts, notifications? | bt-m9te |
| Project quick switcher vs filter | Two UIs designed (bt-s4b7) but quick switcher not yet implemented | bt-s4b7 |

## Audit Reports (foundation for this ADR)

All in `docs/audit/`:
- `test-suite-audit.md` - 268 test files categorized
- `cli-ergonomics-audit.md` - 97 flags inventoried, severity-ranked issues
- `charm-v2-migration-scout.md` - 76 files, impact assessment, migration order
- `bql-gap-analysis.md` - 5 bugs, 3 remaining features, priority order
- `global-mode-readiness.md` - readiness audit for cross-project features
- Earlier audit (session 16): 10 team reports + architecture map from codebase audit

Draft in `docs/drafts/`:
- `README-draft.md` - complete prose rewrite for review
