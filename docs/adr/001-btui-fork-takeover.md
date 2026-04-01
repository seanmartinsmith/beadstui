---
title: "ADR-001: beads_viewer -> beadstui fork takeover"
status: completed (pending review)
date: 2026-02-25
decision-makers: [seanmartinsmith]
---

# ADR-001: beads_viewer -> beadstui fork takeover

## Status

**Completed (pending review)** - all 4 work streams done. Next session should run an ADR review to grade execution quality before moving to new feature work.

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

No open decisions.

### Decided (Session 5, 2026-02-26)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Version numbering | Reset to v0.0.1 | Different project, different module path, different target. Clean break from Jeffrey's v0.14.x lineage. v0.0.x signals pre-alpha. Release is manual: push a git tag, goreleaser builds + publishes. Bump criteria: patch (0.0.x) for bug fixes, minor (0.x.0) for features, major for breaking changes. |

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
**Status**: DONE
**Blocks**: Stream 3

- [x] Test Ctrl+R with actual beads Dolt server - works
- [x] Identify schema mismatches - none found, 541 issues load correctly
- [x] Test auto-refresh - works (Dolt poll loop auto-enables, no env var needed)
- [x] Resolve SQLite ghost state at code level (bv-nkil) - **implemented session 5**

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
**Status**: DONE (remaining: README prose rewrite)
**Depends on**: Nothing

Codebase audit and cleanup as part of the fork takeover:
- [x] **Filesystem audit**: full audit of root .md files, docs/, build artifacts, .gitignore gaps (session 2)
- [x] **Archive Jeffrey-era artifacts**: moved to `docs/archive/` (tracked in git). CLEANED_UP_PROMPTS, optimization plans -> `jeffrey-era/`. Perf research docs -> `optimization-research/`. AGENT_FRIENDLINESS_REPORT, TOON_INTEGRATION_BRIEF -> root archive.
- [x] **Build artifact cleanup**: removed bv_profile (50MB), coverage_report.txt. Fixed .gitignore gaps.
- [x] **Release vs personal audit**: goreleaser clean (correct owner/module/binary). Version numbering decided: v0.0.1 reset. `.beads/` dogfood data intentionally tracked. (session 5)
- [x] **Documentation refresh**: AGENTS.md and SKILL.md content rewritten, README.md mechanical rename (session 3). Full README prose rewrite still TODO.
- [x] **Root .md consolidation**: moved UPGRADE_LOG.md and GOLANG_BEST_PRACTICES.md to docs/ (bv-a3g8). Updated AGENTS.md:368 reference.
- [x] Review .beads/ and .beads-local/ state: cleaned dead artifacts (4.3MB backup db, merge leftovers, sync_base.jsonl). Fixed .bv.lock -> .bt.lock and .br_history -> .bd_history in `.beads/.gitignore`. `.beads-local/` is inert pre-Dolt data, untracked, no action needed. (session 5)
- [x] Audit test data and benchmark artifacts: removed 6 Jeffrey-era benchmark result files (stale, old module path). Test fixtures (testdata/, tests/testdata/) are valid. Fixed missed rename in scripts/coverage.sh. (session 5)

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
| 2026-02-26 (session 5) | **bv-nkil implemented (Stream 1 DONE)**: Added `ErrDoltRequired` sentinel + `RequireDolt` gate in `DiscoverSources()`. When `metadata.json` says `backend: dolt`, discovery skips SQLite/JSONL entirely - if Dolt unreachable, returns clear error ("Dolt server not running. Start it with: bd dolt start") instead of silently falling back to stale data. JSONL fallback in `LoadIssues`/`LoadIssuesWithSource` also guarded. Legacy SQLite/JSONL discovery preserved for non-Dolt projects. Added Dolt activity keepalive: `touchDoltActivity()` writes epoch to `.beads/dolt-server.activity` on each successful poll, preventing the idle monitor from killing Dolt while bt is running. 2 new tests, all 34 datasource tests pass, build clean. ~75 lines changed across 6 files. **Version numbering decided**: v0.0.1 reset (clean break from Jeffrey's v0.14.x). Updated `pkg/version/version.go` fallback. **Stream 4 cleanup**: removed 6 Jeffrey-era benchmark results, cleaned dead `.beads/` artifacts (4.3MB backup db, merge leftovers, sync_base.jsonl), fixed `.beads/.gitignore` renames (.bv.lock -> .bt.lock, .br_history -> .bd_history), fixed missed rename in scripts/coverage.sh. Created bv-9x36 for Dolt disconnect UX improvement. Closed bv-nkil. Installed bt v0.0.1. **ADR marked completed (pending review)**. |

| 2026-03-05 (session 8) | **Titled panels pass**: Converted insights (6 render functions), board (column headers, cards, detail), and help overlay to RenderTitledPanel. Added BorderColor/TitleColor overrides to PanelOpts. Board cards use RoundedBorder + border-only selection (no background fill). Help overlay uses per-section colors from Tomorrow Night gradient. Added 2 panel tests. |
| 2026-03-05 (session 9) | **ADR review findings cleanup**: Fixed 14 stale `bv` CLI refs in AGENTS.md (cmd/bv->cmd/bt, bv --search->bt --search, bv --recipe->bt --recipe, section title bv->bt). Verified items 1-3 and 5 from session 6 findings were already fixed. Fixed insights detail panel viewport off-by-one (vpHeight was height-2, should be height-1 - eliminated blank line at bottom). Remaining `bv` refs in AGENTS.md are intentional: bv-graph-wasm (WASM module), bv-123/bv-xxx (beads issue ID format). Line 855 beads_rust ref is inside historical quote block. |
| 2026-03-05 (session 7) | **Visual overhaul: Tomorrow Night theme + lazygit borders**. Replaced Dracula purple palette with Tomorrow Night + matcha-dark-sea teal. Implemented theme config system: embedded defaults YAML, layered loading (~/.config/bt/theme.yaml, .bt/theme.yaml), globals bridge. Swapped all ~44 Color* vars in styles.go. Updated inline hex in 15+ files (board.go, model.go, delegate.go, actionable.go, insights.go, history.go, shortcuts_sidebar.go, velocity_comparison.go, visuals.go, tutorial_*.go, markdown.go, agent_prompt_modal.go, cass_session_modal.go). Created TitledPanel helper (panel.go) with box-drawing borders and inlined titles. Converted split view to titled "Issues"/"Details" panels. Switched all borders from RoundedBorder to NormalBorder (except graph nodes). Title case convention (BOARD->Board, HISTORY->History, DETAILS->Details). Added ColorTextSecondary and ColorBgContrast tokens. New files: theme_loader.go, panel.go, defaults/theme.yaml, theme_loader_test.go, panel_test.go. Regenerated graph golden files. 18 new/fixed tests pass, no new test failures introduced. |
| 2026-03-07 (session 10) | **Beads migration**: Renamed issue prefix bv->bt (553 issues via `bd rename-prefix`). Set beads.role=maintainer. Local folder renamed beads_viewer->bt. Claude memory copied to new project path. Updated ADR remaining work refs bv->bt, MEMORY.md beads refs bv->bt, fixed stale .gitignore comment. |

| 2026-03-11 (session 11) | **Dogfood polish session**. Added absolute timestamps (FormatTimeAbs) to details pane meta table and expanded card. Priority shows P0-P4 text next to icon. Status bar "Reloaded N issues" auto-clears after 3s (statusClearMsg + seq counter). Help overlay: centered titles (CenterTitle PanelOpts), auto-sized panels to content width, restructured from 3x3 to 4x2 grid, separated Status Indicators to own panel. Board: auto-hide empty columns on card expand. Shortcut audit found 22 undocumented keys (backtick=tutorial, F1-F4, 0/$, n/N, v, E, V, U). Created beads: bt-79eg (test audit), bt-2bns (divider centering), bt-lgbz (card expand+empty cols), bt-aog1 (help narrow terminals), bt-xavk P1 (help system redesign per Nielsen heuristics), bt-3ynd (Dolt status bar redesign). Researched Dolt freshness system - STALE indicator conflates "no changes" with "broken". |
| 2026-03-11 (session 12) | **bt-3ynd**: Fixed false STALE indicator - freshness now tracks last successful Dolt poll (DoltVerifiedMsg) instead of snapshot build time. Poll confirms data is current even when unchanged, preventing false STALE after 2min. Added doltConnected state tracking. **bt-aog1**: Responsive help overlay - 4x2 grid for wide (>=140), 2x4 for medium (>=80), single column for narrow. Status indicators panel auto-hides when terminal too short. **bt-xavk**: Created design plan (docs/plans/help-system-redesign.md) - 3-layer progressive disclosure system. |
| 2026-03-12 (session 13) | **Brainstorm + audit planning session** (no code changes). Investigated Dolt keepalive mechanism - confirmed bt's 5s poll prevents idle timeout. Created 8 dogfood beads: bt-tebr (P2, auto-start Dolt), bt-ztrz (P3, manual refresh keybind), bt-pfic (P3, CLI ergonomics audit), bt-ks0w (P2, mouse click support), bt-spzz (P2, smarter reload status), bt-thpq (P3, Dolt changelog/history), bt-b8gl (P2, status bar contrast), bt-46fa (P3, column header redesign), bt-95mp (P2, alerts panel visual refresh). Added bd subcommand permissions to settings.json. **Post-takeover roadmap brainstorm**: defined 4 phases (Audit -> Stabilize -> Polish -> Interactive). Key decisions: CRUD via bd shell-out (no beads fork), domain-based audit with 8 parallel teams, facts-from-agents/taste-from-human rubric. **Codebase audit plan** designed: 8 teams covering ~190k LOC, structured report template, 2-session execution (scan + synthesis). Codebase is ~88k production Go + ~102k test lines across 431 files. ADR-002 to be created during synthesis session. Docs: `docs/brainstorms/2026-03-12-post-takeover-roadmap.md`, `docs/plans/2026-03-12-codebase-audit-plan.md`. |

## Review Criteria (Next Session)

Before moving to new feature work, a fresh session should review this ADR's execution. Grade each area:

### Completeness
- [ ] Every stream item checked off - are any actually incomplete or punted?
- [ ] All decisions documented with rationale - any gaps?
- [ ] Changelog entries cover all material changes across sessions?

### Code Quality
- [ ] `go build ./cmd/bt/` clean
- [ ] `go test ./...` - catalog pre-existing failures vs anything introduced by fork work
- [ ] No stale Jeffrey-era references in tracked files (grep for `Dicklesworthstone`, `beads_viewer`, `beads_rust`, `bv` in places it should be `bt`)
- [ ] No dead code left behind from the migration

### Release Readiness
- [ ] `bt --version` shows v0.0.1
- [ ] goreleaser config correct (owner, module, binary, taps)
- [ ] README accurately describes what bt is and how to use it (prose rewrite still TODO)
- [ ] LICENSE correct (Jeffrey's copyright + our line + rider)
- [ ] .gitignore covers all runtime/build artifacts

### Architecture
- [x] Dolt lifecycle: bt auto-starts Dolt via `bd dolt start`, PID-based ownership shutdown (bt-07jp)
- [x] Dead keepalive code removed (touchDoltActivity - no consumer since beads v0.59)
- [x] Auto-reconnect on server death (3 consecutive failures -> EnsureServer retry)
- [x] Database identity check (SHOW TABLES LIKE 'issues')
- [ ] Legacy SQLite/JSONL path still works for non-Dolt projects

### Remaining Work (not part of this ADR, but surface for prioritization)
- bt-9x36: Dolt disconnect UX polish
- README prose rewrite
- bt-xft1: beads data separation (deferred)
- 11 pre-existing test failures (Windows path separators, golden files, tutorial tests)

## Changelog

### 2026-03-17 - Session 18: Phase 3 investigation + targeted fixes (5 parallel agents)
- **Robot reload bug**: 5 handlers bypassed --repo/--label/--as-of filters by calling datasource.LoadIssues() instead of rc.issues. Fixed.
- **Duplicate consolidation**: Removed getTypeIcon (kept getTypeEmoji), duplicate generateMermaid, duplicate truncateRunes, truncateStrSprint, private getStatusColor (extended public Theme.GetStatusColor)
- **Lock file migration**: Already implemented (.bv.lock -> .bt.lock with migration path). Added 3 test cases.
- **ACFS workflows**: Deleted duplicate notify-acfs.yml, kept acfs-checksums-dispatch.yml
- **FileChangedMsg**: Investigated, left in place - part of functional JSONL pipeline. Proper cleanup = remove JSONL support entirely (Tier 2 decision)
- **LOC delta**: -123 net (179 added, 302 removed)

### 2026-03-17 - Session 18: Phase 2 monolith splitting (2 parallel tasks)
- **Executed**: `docs/plans/2026-03-17-phase2-monolith-splitting.md` - 2 parallel agents + 1 follow-up
- **model.go split** (8,332 -> 3,482 LOC): 8 new files + semantic_search.go additions
  - model_keys.go (1,009), model_view.go (1,081), model_footer.go (688), model_filter.go (905)
  - model_modes.go (271), model_export.go (280), model_editor.go (402), model_alerts.go (194)
  - 7 semantic search helpers moved to existing semantic_search.go (+124 LOC)
- **main.go split** (8,108 -> 1,507 LOC): 18 new files via robotCtx method pattern
  - robot_ctx.go (44), robot_output.go (143), robot_help.go (474), robot_graph.go (382)
  - robot_triage.go (223), robot_analysis.go (382), robot_labels.go (153), robot_alerts.go (145)
  - robot_history.go (989), robot_sprint.go (488), cli_update.go (80), cli_agents.go (234)
  - cli_baseline.go (186), cli_misc.go (489), helpers.go (585), burndown.go (613)
  - profiling.go (228), pages.go (1,055)
- **LOC delta**: net ~+397 (file headers, robotCtx struct + constructor, method signatures)
- **Verification**: go build + go vet + go test all pass (0 failures, both packages)
- **Next**: Phase 3 investigation items (duplicate funcs, lock migration, robot reload bug)

### 2026-03-16 - Session 17: Phase 1 mechanical cleanup (8 parallel tasks)
- **Executed**: `docs/plans/2026-03-16-phase1-cleanup.md` - 8 parallel agents, conflict-free file partitioning
- **Dead code deleted** (~1.5k+ LOC Go, ~7.5k LOC Rust):
  - `internal/datasource/watch.go` (355 LOC) + `diff.go` (269 LOC) - SourceWatcher/AutoRefreshManager/SourceDiff never instantiated
  - 8 deprecated functions from `pkg/analysis/` (computeCounts, buildBlockersToClear, ComputeTriageScores variants, ComputeImpactScore, TopImpactScores, ParallelGain)
  - `FlowMatrixView`, `ReadyTimeoutMsg`/`Cmd`, empty graph stubs (`ensureVisible`/`ScrollLeft`/`ScrollRight`), `TreeModeBlocking` from `pkg/ui/`
  - `GeneratePriorityBrief`, `stringSliceContains`, `DeployToCloudflarePages`, `AddGitHubWorkflowToBundle` from `pkg/export/`
  - `AgentFileExists` from `pkg/agents/`
  - `RobotMeta`, build tag guards, `medianMinutes` from `cmd/bt/main.go`
  - `Makefile` (referenced non-existent bv/cmd/bv)
  - `bv-graph-wasm/` directory (~7.5k LOC Rust, algorithms reimplemented in Go, WASM artifacts in viewer_assets/ preserved)
- **Stale naming fixed**:
  - `EnsureBVInGitignore` -> `EnsureBTInGitignore` (pkg/loader + cmd/bt)
  - `.bv-atomic-*` -> `.bt-atomic-*` temp prefix (pkg/agents/file.go)
  - "Quit bv?" -> "Quit bt?" (pkg/ui/model.go)
  - Tutorial text bv/br -> bt/bd (pkg/ui/tutorial_content.go, tutorial.go)
  - Package doc comments bv -> bt across 9 pkg/export/ files
  - Script comment headers bv -> bt (coverage.sh, benchmark.sh)
  - Orphan detection + cass correlation regexes now match both `bv-` and `bt-` prefixes (additive)
- **E2E test renames**: 327+ bv->bt renames across 46 test files (variable names, function names, temp dir prefixes, comments)
- **Config updates**: flake.nix version 0.14.4 -> 0.0.1, install scripts Go minimum 1.21 -> 1.25
- **~20 dead test functions** removed alongside their dead production code
- **Verification**: go build + go vet + go test ./... all pass (0 failures, 26 packages)

### 2026-03-16 - Session 16: Codebase audit scan (Session A)
- **Executed**: 9-team parallel codebase audit per `docs/plans/2026-03-16-codebase-audit-plan-v2.md`
- **Reports**: 10 files in `docs/audit/` (teams 1a, 1b, 2, 3, 4, 5, 6, 7, 8a, 8b)
- **Architecture map**: `docs/audit/architecture-map.md` - cross-domain dependency graph + findings synthesis
- **Scale scanned**: ~88k production Go + ~102k test Go + ~7.5k Rust (WASM) + build/CI configs
- **Key findings**:
  - ~9.3k LOC identified as dead code candidates (watch.go, diff.go, SQLite reader, bv-graph-wasm/, deprecated analysis functions, broken Makefile)
  - Stale `bv` naming persists in ~15 locations across tutorial content, correlation regexes, lock files, HTML markers, export output
  - model.go (8.3k) and main.go (8.1k) are monoliths needing extraction
  - No Dolt integration tests exist - production data path has zero E2E coverage
  - pkg/correlation (~4.5k LOC) may lose its data source if upstream beads stops committing JSONL to git
  - Deploy pipelines (GitHub Pages, Cloudflare) may be dead weight for personal TUI
- **Next**: Session B = synthesis with user (review feature inventories, assign KEEP/CUT/IMPROVE verdicts, build ADR-002)

### 2026-03-16 - Session 15: Cross-platform test suite fixes (bt-s3xg et al)
- **Result**: 39 failing Windows tests -> 0 failures across all 26 packages
- **Phase 1**: Renamed bv->bt stragglers in 8 files (test binaries, blurb CLI refs, correlation regex/normalizeBeadID, gitignore, updater archive)
- **Phase 1b**: Fixed ComputeUnblocks to filter blocking edges only (not parent-child); fixed slug collision test expectations
- **Phase 2**: Used filepath.FromSlash/Join in cass and tree test expectations
- **Phase 3**: Added configHome override to tutorial progress manager and wizard config path (HOME env doesn't work on Windows)
- **Phase 4**: Added runtime.GOOS skip guards for 6 Unix-only permission tests
- **Phase 5**: Skipped shell-dependent hooks tests on Windows; fixed robot flag parsing (-r shorthand conflict with --recipe)
- **Phase 6**: Added .exe suffix to drift test binaries; fixed file locking (defer m.Stop(), CWD restore order)
- **Phase 7**: Normalized \r\n line endings in golden file comparison
- **Closed**: bt-s3xg, bt-zclt, bt-3ju6, bt-7y06, bt-ri5b, bt-dwbl, bt-kmxe, bt-mo7r (8 issues)

### 2026-04-01 - Session 16: BQL composable search
- **New package**: `pkg/bql/` - BQL parser vendored from zjrosen/perles (MIT), adapted for bt
- **Parser layer**: lexer, parser, AST, tokens, validator, SQL builder (~1,500 LOC from perles)
- **MemoryExecutor**: in-memory BQL evaluation against model.Issue - comparisons, contains, IN/NOT IN, label arrays, date literals, priority shorthand, boolean logic, blocked computed field, ORDER BY, EXPAND dependency traversal (522 LOC, 28 tests)
- **TUI integration**: `:` keybind opens BQL modal from any view, dedicated `applyBQL()` path (parallel to `applyRecipe()`), workspace pre-filter, Dolt refresh re-application, footer badge
- **Architecture decisions**: BQL as additive (coexists with `/` fuzzy/semantic), dedicated filter path for set-level operations, `Executor` interface designed for future SQL/Dolt executor (global beads)
- **Competitive context**: perles has BQL but no CRUD/multi-project; bt now has BQL + workspace mode + Dolt lifecycle
- **Docs**: brainstorm at `docs/brainstorms/2026-04-01-bql-import-brainstorm.md`, plan at `docs/plans/2026-04-01-feat-bql-import-composable-search-plan.md`
- **Total**: 22 files, ~3,950 lines, 27 packages pass, 0 failures

### 2026-03-16 - Session 14: Dolt lifecycle adaptation (bt-07jp, bt-tebr)
- **New module**: `internal/doltctl/` - server detection, startup via `bd dolt start`, PID-based ownership shutdown
- **Port discovery**: added env var overrides (BEADS_DOLT_SERVER_PORT > BT_DOLT_PORT) to ReadDoltConfig
- **Startup**: replaced hard-exit on ErrDoltRequired with EnsureServer + retry flow
- **Shutdown**: Model.Stop() calls StopIfOwned() - only stops server if bt started it (PID verification)
- **Auto-reconnect**: poll loop attempts EnsureServer after 3 consecutive failures
- **Dead code removed**: touchDoltActivity (keepalive for removed idle monitor)
- **Database identity check**: verifies `issues` table exists after connecting
- **Tests**: 11 doltctl tests + 6 metadata tests (42 total in modified packages)
- **Closed**: bt-07jp (P1), bt-tebr (P2 - subsumed)
