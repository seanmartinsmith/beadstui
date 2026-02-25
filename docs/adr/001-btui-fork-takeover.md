---
title: "ADR-001: beads_viewer -> beadstui fork takeover"
status: active
date: 2026-02-25
decision-makers: [seanmartinsmith]
---

# ADR-001: beads_viewer -> beadstui fork takeover

## Status

**Active** - living document. Updated as decisions are made and work progresses.

## Context

beads_viewer (bv) was built by Jeffrey Emanuel as a TUI for **beads** (Go, Steve Yegge). When beads development diverged from Jeffrey's vision, he forked beads into **beads_rust (br)** and retargeted beads_viewer to br instead of upstream beads.

We forked beads_viewer to restore upstream beads compatibility. This fork is now its own project - we've added close_reason support, Dolt datasource integration, and will continue diverging from Jeffrey's version. We maintain a beads fork and can push fixes upstream to Yegge's repo, so both sides of the integration are within our control.

**License**: MIT with OpenAI/Anthropic rider. We can fork, modify, rename, distribute. Must keep Jeffrey's copyright + rider. We add our own copyright line.

## Decisions

### Decided

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Fork direction | Upstream beads (Go/Dolt) | Jeffrey targets br; we target Yegge's beads |
| Binary name | `bt` | Fits beads 2-letter convention (`bd` CLI, `bv` viewer, `bt` TUI). Less collision risk than `bd` or `bv` - no Homebrew formula, no Linux pkg, no active project. |
| Project/repo name | `beadstui` | Self-explanatory. Repo: `github.com/seanmartinsmith/beadstui`. Binary is `bt` (short alias, like `rg` for ripgrep). |
| GitHub org | `seanmartinsmith` (personal) | Attribution for portfolio. Module: `github.com/seanmartinsmith/beadstui` |
| Copyright approach | Add our line, keep Jeffrey's | MIT license requires it; rider must stay |

### Open

| Decision | Options | Blocking |
|----------|---------|----------|
| Stale SQLite cleanup | Beads-side migration cleanup vs bt explicit source selection | Stream 1 (Dolt verification) |
| Dolt table schema compatibility | Fix bt's DoltReader vs fix beads DoltWriter | Stream 1 (Dolt verification) |

### Decided (Session 2, 2026-02-25)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Env var migration | Hard rename `BT_*`, no `BV_*` fallback | Zero external users of our fork. 42 vars, 400+ refs - mechanical. Clean break avoids confusion with Jeffrey's `bv`. |

## Work Streams

### Stream 1: Verify Dolt end-to-end
**Status**: Mostly done
**Blocks**: Stream 3

- [x] Test Ctrl+R with actual beads Dolt server - works
- [x] Identify schema mismatches - none found, 541 issues load correctly
- [x] Test auto-refresh - works (Dolt poll loop auto-enables, no env var needed)
- [ ] Resolve SQLite ghost state at code level (stale SQLite fallback when metadata says Dolt)

**Key risk**: SQLite (priority 100) beats Dolt (priority 110) when Dolt connection is flaky. If metadata says `backend: dolt`, bt should not silently fall back to stale SQLite.

### Stream 2: Rename bv -> beadstui/bt (mechanical, parallelizable)
**Status**: Ready to start (env var strategy decided)
**Depends on**: Nothing (env var strategy resolved: hard rename BT_*)
**Blocks**: Nothing (can ship independently)
**Bead**: bv-nk9c

Scope:
- Go module path: `github.com/Dicklesworthstone/beads_viewer` -> `github.com/seanmartinsmith/beadstui`
- Binary: `bv` -> `bt`
- Package path: `cmd/bv/` -> `cmd/bt/`
- Import paths: ~100+ .go files
- Env vars: `BV_*` -> `BT_*` (hard rename, no fallback - 42 vars, 400+ refs)
- Data directory: `.bv/` -> `.bt/`
- User-facing strings: `br` -> `beads` (CLI references, help text, error messages)
- goreleaser: binary name, tap/bucket, descriptions
- AGENTS.md: extensive br command references
- LICENSE: add our copyright line
- .gitignore: `.bv/` patterns

### Stream 3: Data migration + dogfooding
**Status**: In progress
**Depends on**: Stream 1 (Dolt must work for bt to read it)

- [x] Migrate existing JSONL/SQLite beads to Dolt (541 issues migrated 2026-02-25)
- [x] Clean stale SQLite artifacts (beads.db-shm, beads.db-wal, beads.db.migrated, old daemon files)
- [x] Set up Dolt remote sync to seanmartinsmith/beadstui
- [x] Triage stale issues from Jeffrey's beads_rust era (closed all 21 open issues, created 9 fresh beads for our work)
- [x] Restart daemon post-migration (Dolt server restarted, verified)
- [x] Use this repo as bt's own dogfood environment (actively using beads to track fork takeover work)
- [ ] Verify bt (once renamed) reads from Dolt correctly (Stream 1 overlap)

### Stream 4: Spring cleaning (parallelizable with Stream 2)
**Status**: In progress
**Depends on**: Nothing

Codebase audit and cleanup as part of the fork takeover:
- [x] **Filesystem audit**: full audit of root .md files, docs/, build artifacts, .gitignore gaps (session 2)
- [x] **Archive Jeffrey-era artifacts**: moved to `docs/archive/` (tracked in git). CLEANED_UP_PROMPTS, optimization plans -> `jeffrey-era/`. Perf research docs -> `optimization-research/`. AGENT_FRIENDLINESS_REPORT, TOON_INTEGRATION_BRIEF -> root archive.
- [x] **Build artifact cleanup**: removed bv_profile (50MB), coverage_report.txt. Fixed .gitignore gaps.
- [ ] **Release vs personal audit**: separate what ships vs dev-only. .goreleaser.yaml excludes, .beads/ state handling.
- [ ] **Documentation refresh**: README rewrite, AGENTS.md rewrite, SKILL.md rewrite (all depend on Stream 2 naming)
- [ ] **Root .md consolidation**: move UPGRADE_LOG.md and GOLANG_BEST_PRACTICES.md to docs/ (bv-a3g8). AGENTS.md:368 references GOLANG_BEST_PRACTICES.md - must update together.
- [ ] Review .beads/ and .beads-local/ state
- [ ] Audit test data and benchmark artifacts

**New bug discovered**: bv-1p3a - Dolt poll loop floods TUI with connection errors when server is down. No backoff, no suppression. Affects all projects using bv with Dolt.

## Implementation Already Done

From the Dolt integration session (2026-02-25):

| File | Changes |
|------|---------|
| `internal/datasource/load.go` | `LoadResult`, `LoadIssuesWithSource()` |
| `cmd/bv/main.go` | Uses `LoadIssuesWithSource`, passes DataSource to NewModel |
| `pkg/ui/model.go` | `dataSource` field, `isDoltSource()`, `reloadFromDataSource()`, `replaceIssues()`, `DataSourceReloadMsg` handler, Ctrl+R |
| `pkg/ui/background_worker.go` | `dataSource` in config/struct, Dolt poll loop, `processLoop` stays alive for Dolt |
| All test files | 88 `NewModel` call sites updated for new signature |

Build passes, tests pass (pre-existing failures only).

## Related Plans

<!-- Link spawned plans here as they're created -->
<!-- - [[plan-dolt-verification]] - Stream 1 detail -->
<!-- - [[plan-bt-rename]] - Stream 2 detail -->

## Process

This ADR is the spine. Each work stream spawns its own plan (linked above). Within sessions:
- Use Claude Code tasks for anything > 3 steps
- Create beads issues for work that persists across sessions
- Commits reference relevant beads issue IDs
- Plans reference back to this ADR

## Changelog

| Date | Change |
|------|--------|
| 2026-02-25 | ADR created from brainstorm session. Dolt integration code implemented. |
| 2026-02-25 | Naming finalized: project/repo `beadstui`, binary `bt`, module `github.com/seanmartinsmith/beadstui`. Name collision research confirmed `bt` has lower risk than existing beads ecosystem names (`bd`, `bv`). GitHub org: `seanmartinsmith` (personal, for portfolio attribution). |
| 2026-02-25 | Repo created at `seanmartinsmith/beadstui`, pushed main branch. Commit history rewritten to remove s070681 attribution. Remotes: origin=beadstui (SSH), upstream=Dicklesworthstone/beads_viewer (read-only reference). Git config set to seanmartinsmith for future commits. Added Stream 4 (spring cleaning). |
| 2026-02-25 | Stream 1 mostly verified: Ctrl+R, live auto-refresh, and schema compatibility all confirmed working. Fixed Dolt auto-polling to not require BV_BACKGROUND_MODE. Migrated 541 issues to Dolt, set up remote sync, cleaned stale SQLite. Handoff plan written. |
| 2026-02-25 (session 2) | Closed all 21 Jeffrey-era beads issues (clean slate). Created fresh beads for fork work streams. Env var strategy decided: hard rename BT_* no fallback (42 vars, 400+ refs). Spring cleaning Phase 1: archived 8 files to docs/archive/, removed build artifacts (50MB), fixed .gitignore. Discovered bv-1p3a: Dolt poll loop floods TUI when server down - affects all Dolt projects. SKILL.md confirmed as legitimate agent CLI guide (keep + rewrite). AGENTS.md has code dependency in pkg/agents/ (15 Go files). Beads data separation researched: refs/dolt/data is hidden from git clone by default (more isolated than old branch sync) - deferred until contributors are a concern. Handoff doc (beads-dolt-migration.md) has issues: Dolt remote didn't persist, Mac bootstrap flow incomplete. Set BD_ACTOR=sms in PowerShell profile. Commits: 715d412, 2cae49d. |
