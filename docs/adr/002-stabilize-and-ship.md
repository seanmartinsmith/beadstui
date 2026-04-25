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
- [x] Fix --bql filter no-op'd in robot list — was bypassing robotPreRun (bt-111w, 2026-04-25)
- [ ] **Sub-stream surfaced 2026-04-25: post-Dolt migration of bt-derived data layer.** bt's correlator (`pkg/correlation/`) and sprint loader (`pkg/loader/sprint.go`) were built against the pre-v0.56.1 era of beads (JSONL backups available alongside Dolt). Beads is Dolt-only since v1.0.1. Affected subcommands: `history`, `related`, `causality` (correlator-bound — bt-08sh), `forecast`, `sprint show` (sprint-bound — bt-z5jj). Plus an ADR-flavored decision about canonical `.beads/` vs `.bt/` data-home split (bt-uahv). bt-vhn2 closed as superseded; original "--global routing" framing was wrong.
- [ ] Robot mode I/O contract: documented invariants + verify-test sweep (bt-ah53). Locks in stdout=structured-only / stderr=errors-only / exit-code-correct contract; prevents F2/F11-style regressions.
- [ ] Unknown `bt robot` subcommand prints help to stdout (bt-70cd). Cobra default; consumed by F-CONTRACT verify-test.
- [ ] Standalone `bt robot comments <id> --global` (bt-82w8). Cross-project comment fetch.
- [ ] Per-subcommand flag manifest in `bt robot schema` (bt-3qfa). Agent introspection.
- [ ] BQL parse-error hints for `id:` shorthand (bt-llh2). Smaller UX polish.

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
**Status**: DONE (epic closed 2026-04-22)
**Priority**: Complete
**Completed**: 2026-04-09 through 2026-04-10; epic bt-if3w closed 2026-04-22

Executed as a 4-phase refactor plan (`docs/plans/` has the execution plan):

- [x] **Phase 0** (2026-04-09): Mechanical Charm v2 migration - import paths, API renames, compat bridge for AdaptiveColor. 76 files updated.
- [x] **Phase 0.5** (2026-04-10): Test foundation - coverage for refactor safety net.
- [x] **Phase 1** (2026-04-10): Model decomposition - ViewMode enum (1.1), DataState/FilterState extraction (1.2), ModalType enum (1.3), Update() handler decomposition (1.4).
- [x] **Phase 1.5** (2026-04-10): Footer extraction as standalone component (bt-oim6).
- [x] **Phase 2** (2026-04-10): Kill AdaptiveColor - 174 occurrences eliminated, all colors resolved to `color.Color` at load time. Dark mode detection via `tea.BackgroundColorMsg`. `adaptive_color.go` deleted.
- [x] **Phase 3** (2026-04-10): Cobra CLI migration - main.go from 1,708 to 13 lines. 35+ robot subcommands migrated to `bt robot *`.

**Decision (2026-04-22)**: Epic bt-if3w closed. Residual hygiene decoupled from the epic frame and tracked as standalone P2 polish beads:

- `bt-t82t` — Phase 4 cleanup: stale `bv-` references in live tutorial/history/UI text, golden file regen, `go test ./...` + `go vet ./...` validation, verify bt-0cht items naturally resolved by Cobra migration.
- `bt-if3w.1` — Sprint view extraction as standalone component (`pkg/ui/sprint_view.go`), same pattern as the closed footer extraction (bt-oim6).
- **Cut (YAGNI)**: Pre-compute hot-path styles. Noted as "needs profiling first" — no profiling evidence exists that it's a bottleneck. If profiling later shows otherwise, file a fresh bead.

The two open beads compete against the broader backlog on their own merits rather than inheriting urgency from the epic frame. Gate bt-bo4a (8-day-old "what's in scope" gate) closed with this decision recorded.

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

Recent completions (2026-04-24 — alerts epic redesign):
- [x] bt-46p6.4 (P3): Alert type taxonomy renamed to human-readable names (dependency_loop, high_leverage, coupling_growth, centrality_change, issue_count_change, dependency_change, stale)
- [x] bt-46p6.8 (P2): Scope-aware alert computation — project-scoped only, no global aggregates; baseline schema v2 with per-project sections
- [x] bt-46p6.11 (P2): CLI alert system alignment retired — coordination bead folded into sibling acceptance
- [x] bt-lm2h (P3): Progress sort mode added as 6th entry in the s/S cycle (in_progress → review → open → hooked → blocked → pinned → deferred → closed → tombstone)
- [x] bt-7l5m (decision): Alert scope = project-scoped only; global aggregate metrics deferred to bt-46p6.18 (P4), TUI cross-project nav deferred to bt-46p6.19 (P3)

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

### Stream 9: Release engineering (added 2026-04-22)
**Status**: DONE — pre-tag gates cleared, binaries-only release path ready (2026-04-23)
**Priority**: P2-P3
**Beads**: bt-ncu7, bt-brid, bt-bntv, bt-lz7d, bt-4f7g (all closed); bt-zgzq (P4, deferred re-enable)
**Foundation**: `.goreleaser.yaml`, `.github/workflows/release.yml`, smoke-test findings in 2026-04-22 + 2026-04-23 CHANGELOG entries

Goreleaser pipeline inherited from Jeffrey's era. Verified via `goreleaser release --snapshot --clean` (v2.15.4) on 2026-04-22 — cross-compile works (linux/darwin/windows × amd64/arm64, 5 binaries), archives + checksums generated. Brew/scoop publishing explicitly stripped for pre-v1 releases (see bt-brid decision); tracked for restoration via bt-zgzq once bt hits the subjective v1 bar (dogfood-clean TUI, feature-complete to maintainer's standard).

Pre-tag gates (all cleared 2026-04-23):

- [x] **bt-ncu7** (P3, task): `net.JoinHostPort` swap for IPv6-safe address formatting — `go vet ./...` baseline clean.
- [x] **bt-brid** (P2, decision): Option 2 chosen — strip brew/scoop for v0.1, re-enable via bt-zgzq post-v1. Removed `brews:` + `scoops:` blocks and `HOMEBREW_TAP_GITHUB_TOKEN` env.
- [x] **bt-bntv** (P2, bug): closed as not-applicable — the brew formula test stanza disappeared when `brews:` was stripped. Fix re-applies via bt-zgzq step 7.
- [x] **bt-lz7d** (P2, task): `.goreleaser.yaml` migrated to v2 format (`version: 2`, `archives.formats`, `snapshot.version_template`). Workflow pinned to `~> v2.15`. `goreleaser check` exits clean.
- [x] **bt-4f7g** (P3, bug): ldflags template switched from `v{{.Version}}` to `{{.Tag}}` — single-v output on both snapshot and real-tag builds.

Real tag push (`git push origin v*`) now triggers GitHub Release with 5 cross-compiled archives + checksums only — no external package-manager publish. When v1 approaches, work through bt-zgzq to restore brew tap + scoop bucket channels.

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
