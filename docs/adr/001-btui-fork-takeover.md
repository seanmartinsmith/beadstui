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
| Env var migration strategy | Hard rename `BT_*` vs `BT_*` with `BV_*` fallback | Stream 2 (rename) |
| Stale SQLite cleanup | Beads-side migration cleanup vs bt explicit source selection | Stream 1 (Dolt verification) |
| Dolt table schema compatibility | Fix bt's DoltReader vs fix beads DoltWriter | Stream 1 (Dolt verification) |

## Work Streams

### Stream 1: Verify Dolt end-to-end (BLOCKING)
**Status**: Not started
**Depends on**: User's actual beads+Dolt setup
**Blocks**: Stream 3

- [ ] Test Ctrl+R with actual beads Dolt server
- [ ] Identify schema mismatches (beads Go DoltWriter vs bt DoltReader)
- [ ] Fix mismatches (may require changes on beads side)
- [ ] Test auto-refresh with `BV_BACKGROUND_MODE=1`
- [ ] Resolve SQLite ghost state (stale .beads/beads.db competing with Dolt)

**Key risk**: SQLite (priority 100) beats Dolt (priority 110) when Dolt connection is flaky. If metadata says `backend: dolt`, bt should not silently fall back to stale SQLite.

### Stream 2: Rename bv -> beadstui/bt (mechanical, parallelizable)
**Status**: Not started
**Depends on**: Env var strategy (open question)
**Blocks**: Nothing (can ship independently)

Scope:
- Go module path: `github.com/Dicklesworthstone/beads_viewer` -> `github.com/seanmartinsmith/beadstui`
- Binary: `bv` -> `bt`
- Package path: `cmd/bv/` -> `cmd/bt/`
- Import paths: ~100+ .go files
- Env vars: `BV_*` -> `BT_*` (strategy TBD)
- Data directory: `.bv/` -> `.bt/`
- User-facing strings: `br` -> `beads` (CLI references, help text, error messages)
- goreleaser: binary name, tap/bucket, descriptions
- AGENTS.md: extensive br command references
- LICENSE: add our copyright line
- .gitignore: `.bv/` patterns

### Stream 3: Data migration + dogfooding
**Status**: Not started
**Depends on**: Stream 1 (Dolt must work)

- [ ] Migrate existing JSONL/SQLite beads to Dolt (this repo's .beads/)
- [ ] Triage stale issues from Jeffrey's beads_rust era
- [ ] Use this repo as bt's own dogfood environment

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
