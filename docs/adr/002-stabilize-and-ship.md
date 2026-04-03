---
title: "ADR-002: Stabilize and ship"
status: active
date: 2026-04-03
decision-makers: [seanmartinsmith]
---

# ADR-002: Stabilize and ship

## Status

**Active** - this is the current project spine. Supersedes [ADR-001](001-btui-fork-takeover.md) (fork takeover, completed).

## Context

The fork takeover is done. bt is a working TUI with BQL search, Dolt lifecycle management, theme system, and robot mode. The codebase has been audited, cleaned, renamed, and all tests pass cross-platform.

What's missing: the codebase is on Charm v1 (approaching EOL), robot-mode has consistency bugs, and CRUD requires shelling out to `bd` (not yet implemented in TUI). The README doesn't reflect current state.

This ADR tracks the path from "working fork" to "shippable product."

## Development Arc

```
[DONE] Fork takeover -> [DONE] Audit -> [HERE] Stabilize -> Polish -> Charm v2 -> Ship CRUD
```

## Work Streams

### Stream 1: Robot-mode hardening
**Status**: Audited, not started
**Priority**: P1
**Bead**: bt-0cht
**Foundation**: `docs/audit/cli-ergonomics-audit.md`

The robot API is bt's primary agent interface. Three critical bugs where output skips the standard RobotEnvelope (--robot-search, --robot-bql, --robot-diff). 18 undocumented env vars. Positional args silently ignored.

Scope:
- [ ] Fix envelope bypass in robot-search, robot-bql, robot-diff
- [ ] Add positional arg warning/support
- [ ] Document all 18 missing env vars in --robot-docs env
- [ ] Consolidate duplicate confidence/agent-count flags
- [ ] Fix stale bv/br references in help text and SKILL.md

### Stream 2: BQL completion
**Status**: Core shipped, bugs found
**Priority**: P2
**Bead**: bt-bjk4 (bugs), bt-faaw (highlighting), bt-sytt (recipes)
**Foundation**: `docs/audit/bql-gap-analysis.md`

BQL parser, memory executor, TUI modal, and CLI flags are shipped. Five bugs found in gap analysis. Three features remain.

Bugs:
- [ ] Add ValidStatusValues to validator (parallel to ValidTypeValues)
- [ ] Fix date equality (truncate to day, not exact time match)
- [ ] Add ISO date parsing to lexer
- [ ] Remove readySQL dead code from sql.go
- [ ] Fix --robot-bql envelope (overlaps with Stream 1)

Remaining features:
- [ ] Syntax highlighting in BQL modal (bt-faaw)
- [ ] Status key redirect through BQL (design decision needed: how to handle multi-status groups like "closed")
- [ ] Recipes as saved BQL queries (bt-sytt - high effort, needs architecture decision)

### Stream 3: Test suite fixes
**Status**: Audited, not started
**Priority**: P2
**Bead**: bt-5dvl
**Foundation**: `docs/audit/test-suite-audit.md`

268 files scanned, 93% clean. One P1 blocker, rest is cosmetic.

- [ ] Fix Windows path length panic in e2e export copyDirRecursive (P1)
- [ ] Fix 3 stale error message strings referencing old br/bv names (P2)
- [ ] Clean historical bv-XXXX issue ID refs in comments (P3, opportunistic)

### Stream 4: Charm v2 migration
**Status**: Scouted, not started
**Priority**: P2
**Bead**: bt-zta9
**Foundation**: `docs/audit/charm-v2-migration-scout.md`

76 files need changes. 60% mechanical (import paths, API renames). The hard part is AdaptiveColor removal (161 occurrences across theme struct, loader, and styles).

Recommended two-phase approach:
1. **Compile on v2**: Use compat bridge for AdaptiveColor, mechanical API updates
2. **Make idiomatic**: Replace compat bridge with native v2 patterns, refactor theme system

Good news: 475/550 style creations already use v2-preferred NewStyle() pattern.

Migration order: Lipgloss -> Bubble Tea -> Bubbles -> Glamour/Huh

### Stream 5: README and docs
**Status**: Draft ready
**Priority**: P2
**Bead**: bt-iuqy
**Foundation**: `docs/drafts/README-draft.md`

README prose rewrite drafted. Needs review against current state before replacing README.md (may be forward-looking on some features).

- [ ] Review draft accuracy against current codebase
- [ ] Verify screenshot references exist
- [ ] Diff against current README.md
- [ ] Replace when verified

### Stream 6: Polish (UX bugs and visual fixes)
**Status**: Backlog
**Priority**: P2-P3

Accumulated dogfood findings. None block shipping but all affect perceived quality.

Key items:
- bt-xavk (P1): Help system redesign - plan exists at docs/plans/help-system-redesign.md
- bt-lgbz (P2): Card expand broken with empty columns (needs design decision)
- bt-k8rk (P2): h/H key behavior buggy
- bt-npnh (P2): History view broken at smaller terminals
- bt-vhhh (P2): Detail-only view arrow key navigation broken

Full list: `bd list --status=open` (30+ items)

### Stream 7: CRUD from TUI
**Status**: Not started (design decided)
**Priority**: Deferred until Streams 1-4 stable
**Decision**: Shell out to `bd` for writes, poll Dolt for changes. No beads fork needed.

This is the end goal - making bt interactive, not just a viewer. Deferred until the foundation (robot-mode, BQL, Charm v2) is solid.

## Decisions

### Decided (from ADR-001 + sessions 1-17)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| CRUD approach | Shell out to `bd` | No beads fork needed. Poll Dolt for changes after writes. |
| Charm v2 migration | Two-phase (compat bridge, then idiomatic) | Reduces risk. AdaptiveColor is deeply embedded in theme system. |
| Robot-mode fixes | Envelope consistency first | Agents can't reliably parse bt output if format varies by command. |

### Open

| Decision | Context | Blocking |
|----------|---------|----------|
| Status key BQL redirect | Pressing 'o' for OPEN - should it become a BQL filter? How to handle multi-status groups? | bt-faaw |
| Card expand with empty columns | Auto-hide empty cols? Span multiple? Redistribute width? | bt-lgbz |
| Recipes as BQL migration | Replace YAML recipe system with saved BQL queries, or keep both? | bt-sytt |

## Audit Reports (foundation for this ADR)

All in `docs/audit/`:
- `test-suite-audit.md` - 268 test files categorized
- `cli-ergonomics-audit.md` - 97 flags inventoried, severity-ranked issues
- `charm-v2-migration-scout.md` - 76 files, impact assessment, migration order
- `bql-gap-analysis.md` - 5 bugs, 3 remaining features, priority order
- Earlier audit (session 16): 10 team reports + architecture map from codebase audit

Draft in `docs/drafts/`:
- `README-draft.md` - complete prose rewrite for review
