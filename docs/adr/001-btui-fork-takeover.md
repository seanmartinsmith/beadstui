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
| Version numbering | Reset to v0.1.0 for fork identity vs continue from Jeffrey's v0.14.4 | Release readiness |

### Decided (Session 4, 2026-02-26)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Stale SQLite cleanup (bv-nkil) | Fail hard when metadata says Dolt; keep legacy SQLite/JSONL for non-Dolt projects | Upstream beads v0.56+ removed SQLite/JSONL entirely - Dolt is the only backend. The `beads.db` file is a dead migration artifact beads will never write to again. Silent fallback to it is actively harmful. |
| Dolt table schema compatibility | No action needed | Schema verified compatible in session 1 (541 issues loaded). DoltReader already handles both full and simple column sets with fallback. |

### Decided (Session 2, 2026-02-25)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Env var migration | Hard rename `BT_*`, no `BV_*` fallback | Zero external users of our fork. 42 vars, 400+ refs - mechanical. Clean break avoids confusion with Jeffrey's `bv`. |

## Work Streams

### Stream 1: Verify Dolt end-to-end
**Status**: Done (final item has plan, implementation pending)
**Blocks**: Stream 3

- [x] Test Ctrl+R with actual beads Dolt server - works
- [x] Identify schema mismatches - none found, 541 issues load correctly
- [x] Test auto-refresh - works (Dolt poll loop auto-enables, no env var needed)
- [ ] Resolve SQLite ghost state at code level (bv-nkil) - **plan ready, see Related Plans**

**Key context (discovered session 4)**: Upstream beads v0.56+ removed SQLite and JSONL backends entirely. Dolt server mode is the only storage path. The Dolt server is transient (30-min idle timeout with idle monitor sidecar). bt needs to touch `.beads/dolt-server.activity` to prevent the idle monitor from killing Dolt while bt is actively displaying data.

### Stream 2: Rename bv -> beadstui/bt (mechanical, parallelizable)
**Status**: DONE (session 3, 2026-02-25)
**Depends on**: Nothing (env var strategy resolved: hard rename BT_*)
**Blocks**: Nothing (can ship independently)
**Bead**: bv-nk9c

Scope (all completed):
- [x] Go module path: `github.com/Dicklesworthstone/beads_viewer` -> `github.com/seanmartinsmith/beadstui`
- [x] Binary: `bv` -> `bt`
- [x] Package path: `cmd/bv/` -> `cmd/bt/`
- [x] Import paths: 179 .go files (302 occurrences)
- [x] Env vars: `BV_*` -> `BT_*` (hard rename, no fallback - 46 files, 235 occurrences)
- [x] Data directory: `.bv/` -> `.bt/`
- [x] User-facing strings: `br` -> `bd` (CLI command references)
- [x] goreleaser: binary name, tap/bucket, descriptions, URLs
- [x] AGENTS.md content rewrite (filename stays - code dependency)
- [x] SKILL.md content rewrite
- [x] README.md mechanical rename pass
- [x] LICENSE: added Sean Martin Smith copyright line
- [x] .gitignore: `.bv/` -> `.bt/` patterns
- [x] install.sh, install.ps1: repo owner, name, binary
- [x] flake.nix: pname, subPackages, ldflags, meta
- [x] CI workflows: build paths, module paths
- [x] Project CLAUDE.md updated (removed "In Transition" section)

### Stream 3: Data migration + dogfooding
**Status**: Done
**Depends on**: Stream 1 (Dolt must work for bt to read it)

- [x] Migrate existing JSONL/SQLite beads to Dolt (541 issues migrated 2026-02-25)
- [x] Clean stale SQLite artifacts (beads.db-shm, beads.db-wal, beads.db.migrated, old daemon files)
- [x] Set up Dolt remote sync to seanmartinsmith/beadstui
- [x] Triage stale issues from Jeffrey's beads_rust era (closed all 21 open issues, created 9 fresh beads for our work)
- [x] Restart daemon post-migration (Dolt server restarted, verified)
- [x] Use this repo as bt's own dogfood environment (actively using beads to track fork takeover work)
- [x] Verify bt (post-rename) reads from Dolt correctly (session 4 smoke test: 551 issues loaded, TUI renders, background mode enabled)

### Stream 4: Spring cleaning (parallelizable with Stream 2)
**Status**: In progress
**Depends on**: Nothing

Codebase audit and cleanup as part of the fork takeover:
- [x] **Filesystem audit**: full audit of root .md files, docs/, build artifacts, .gitignore gaps (session 2)
- [x] **Archive Jeffrey-era artifacts**: moved to `docs/archive/` (tracked in git). CLEANED_UP_PROMPTS, optimization plans -> `jeffrey-era/`. Perf research docs -> `optimization-research/`. AGENT_FRIENDLINESS_REPORT, TOON_INTEGRATION_BRIEF -> root archive.
- [x] **Build artifact cleanup**: removed bv_profile (50MB), coverage_report.txt. Fixed .gitignore gaps.
- [ ] **Release vs personal audit**: separate what ships vs dev-only. .goreleaser.yaml excludes, .beads/ state handling.
- [x] **Documentation refresh**: AGENTS.md and SKILL.md content rewritten, README.md mechanical rename (session 3). Full README prose rewrite still TODO.
- [x] **Root .md consolidation**: moved UPGRADE_LOG.md and GOLANG_BEST_PRACTICES.md to docs/ (bv-a3g8). Updated AGENTS.md:368 reference.
- [ ] Review .beads/ and .beads-local/ state
- [ ] Audit test data and benchmark artifacts

**Bug fixed (session 3)**: bv-1p3a - Dolt poll loop now has exponential backoff (5s -> 2min cap), duplicate suppression (first error only at warn level, subsequent at trace), and status bar integration via DoltConnectionStatusMsg.

## Implementation Already Done

From the Dolt integration session (2026-02-25):

| File | Changes |
|------|---------|
| `internal/datasource/load.go` | `LoadResult`, `LoadIssuesWithSource()` |
| `cmd/bt/main.go` | Uses `LoadIssuesWithSource`, passes DataSource to NewModel |
| `pkg/ui/model.go` | `dataSource` field, `isDoltSource()`, `reloadFromDataSource()`, `replaceIssues()`, `DataSourceReloadMsg` handler, Ctrl+R |
| `pkg/ui/background_worker.go` | `dataSource` in config/struct, Dolt poll loop, `processLoop` stays alive for Dolt |
| All test files | 88 `NewModel` call sites updated for new signature |

Build passes, tests pass (pre-existing failures only).

## Related Plans

- `~/.claude/plans/woolly-tinkering-snowflake.md` - bv-nkil fix: SQLite ghost state + Dolt activity keepalive (session 4)

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
| 2026-02-25 (session 3) | **Fixed bv-1p3a**: Dolt poll loop now has exponential backoff (5s base, 2min cap), duplicate error suppression (first at warn, subsequent at trace), and status bar integration via new DoltConnectionStatusMsg. **Stream 2 DONE**: Full atomic rename - module path (179 files), cmd/bv->cmd/bt, BV_*->BT_ (46 files), .bv->.bt, br->bd (CLI refs), goreleaser, install scripts, flake.nix, CI workflows, AGENTS.md/SKILL.md content rewrites, README.md mechanical rename, LICENSE copyright, .gitignore, project CLAUDE.md. Build passes, no new test failures. **Spring cleaning Phase 2 partial**: moved UPGRADE_LOG.md and GOLANG_BEST_PRACTICES.md to docs/ (bv-a3g8), updated AGENTS.md:368 reference. |
| 2026-02-26 (session 4) | Built and installed `bt` binary (v0.14.4). **Stream 3 DONE**: smoke test confirmed bt reads 551 issues from Dolt post-rename. Added version numbering as open decision. **Critical discovery**: upstream beads v0.56+ removed SQLite and JSONL entirely - Dolt server mode is the only backend. Dolt server is transient (30-min idle timeout via idle monitor sidecar, activity tracked in `.beads/dolt-server.activity`). This means the SQLite ghost state (bv-nkil) is worse than thought - bt falls back to a dead artifact, not stale data. **Decided**: fail hard when metadata says `backend: dolt` and Dolt unreachable; keep legacy SQLite/JSONL for non-Dolt projects. Also: bt must touch activity file to prevent idle monitor from killing Dolt during active sessions. Plan written for bv-nkil implementation. Cleaned up orphan Dolt processes (4 killed). Researched beads repo architecture for alignment. |
