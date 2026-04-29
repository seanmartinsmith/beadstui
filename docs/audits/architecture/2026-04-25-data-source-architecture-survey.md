# Data Source Architecture Survey

**Date:** 2026-04-25
**Author:** sms (with claude-code session 190df5ce)
**Related:** ADR-003 (data source architecture post-Dolt), bt-mhcv (audit), bt-08sh (correlator), bt-z5jj (sprint), bt-uahv (data-home)
**Status:** Foundation document for ADR-003

## Purpose

Inventory how much of bt's read path is Dolt-native vs. JSONL-pinned, as ground truth for the ADR-003 decision on what to do about the SourceType abstraction now that beads is Dolt-only (since v1.0.1, March 2026).

## Goal-state architecture (`internal/datasource/`)

bt has a multi-source DataSource abstraction at `internal/datasource/source.go:25-47`:

| Source                    | Priority | Const                         | Reader LOC | Status                                                  |
|---------------------------|---------:|-------------------------------|-----------:|---------------------------------------------------------|
| `SourceTypeDolt`          |      110 | per-project Dolt              |        409 | First-class                                             |
| `SourceTypeDoltGlobal`    |    (n/a) | shared-server Dolt            |        705 | First-class                                             |
| `SourceTypeSQLite`        |      100 | local SQLite                  |        397 | **Dead** — beads removed SQLite in v0.56.1              |
| `SourceTypeJSONLWorktree` |       80 | git-worktree JSONL            |  (via loader) | Legacy, still discoverable                          |
| `SourceTypeJSONLLocal`    |       50 | `.beads/*.jsonl`              |  (via loader) | Legacy, still discoverable                          |

`DiscoverSources` (source.go:101) finds all candidates, `SelectBestSource` picks the freshest. When `.beads/metadata.json` declares `backend=dolt`, `RequireDolt: true` short-circuits the JSONL fallback via `ErrDoltRequired` (source.go:140-144) — Dolt-only projects do not silently downgrade to stale JSONL.

`LoadFromSource` (load.go:154) dispatches per type. The TUI cold-load path goes through this layer.

## Read paths through the smart layer (Dolt-aware)

These paths all benefit from discovery, validation, and the Dolt-first policy:

- **TUI cold load** — `cmd/bt/root.go` → `LoadIssuesWithSource` → `LoadFromSource` → `NewDoltReader.LoadIssues` (or `NewGlobalDoltReader.LoadIssues`).
- **Global mode bootstrap** — `cmd/bt/root.go:215-241` discovers shared server, uses `NewGlobalDataSource` + `NewGlobalDoltReader`.
- **Background poll/refresh** — `pkg/ui/background_worker.go:1990-1992` dispatches to `globalDoltPollOnce` (line 2097) or `doltPollOnce` (line 2068) based on the active `SourceType`.
- **TUI write→reload after CRUD** — same load path.

## Read paths that bypass the smart layer (JSONL-pinned)

These call `loader.FindJSONLPath` / `loader.LoadIssuesFromFile` directly, with no Dolt awareness. They will silently fail or return empty on Dolt-only projects that never opted into JSONL export.

### Production code

| File                            |     Lines | Surface                                                |
|---------------------------------|----------:|--------------------------------------------------------|
| `cmd/bt/robot_history.go`       | 36, 158, 224, 346, 444, 545, 623, 692, 807, 892 | **10 call sites — every `bt robot history` subcommand** |
| `pkg/workspace/loader.go`       |  158, 162 | Multi-repo workspace loader (load-bearing for global git history feature) |
| `cmd/bt/cobra_export.go`        |       319 | Static-site export per-repo                            |
| `cmd/bt/burndown.go`            |       442 | Sprint scope changes                                   |
| `cmd/bt/profiling.go`           |        18 | `--profile-startup`                                    |
| `cmd/bt/pages.go`               |       593 | Pages export                                           |
| `cmd/bt/robot_triage.go`        |        36 | `--robot-triage` JSONL fallback                        |
| `pkg/ui/model_editor.go`        |       310 | Open-in-editor convenience                             |

`pkg/ui/background_worker.go:1469` and `pkg/ui/model_update_data.go:543` call `LoadIssuesFromFileWithOptionsPooled` — these are legitimate uses (the JSONL polling path when `SourceType == JSONLLocal/Worktree`), not Dolt-bypassing pins.

### Tests

Many `loader.LoadIssuesFromFile` and `loader.FindJSONLPath` call sites in `pkg/loader/*_test.go`, `tests/e2e/*_test.go`, `pkg/search/*_test.go`, `pkg/analysis/*_test.go`. These are correct (tests use JSONL fixtures by design). Not in scope for the audit.

## Vestigial: the SQLite reader

`internal/datasource/sqlite.go` (397 LOC) plus `discoverSQLiteSources` in `source.go:204-224` plus `PrioritySQLite = 100` constant plus all of `internal/datasource/sqlite_test.go` and SQLite branches in `source_test.go`. Beads removed SQLite as a backend in v0.56.1. Any project on current `bd` will never have `.beads/beads.db` to discover. This is dead code that gets considered on every load.

The cleanest reading: SQLite was first-class when this abstraction was built, and the abstraction's *shape* assumes it still is. With SQLite gone, the design space the SourceType abstraction was solving for collapsed.

## Related stale-assumption beads (existing)

- **bt-mhcv** (P0, audit) — systematic audit of all open bt beads against post-Dolt-only reality.
- **bt-08sh** (P2, correlator) — `pkg/correlation/` migration from JSONL+git-diff witness to `dolt_log` / `dolt_history_issues`. Implementation should also sweep the 10 `loader.FindJSONLPath` call sites in `cmd/bt/robot_history.go`.
- **bt-z5jj** (P3, sprint) — `pkg/loader/sprint.go` reads non-existent `.beads/sprints.jsonl`. Decision pending: rebuild against Dolt or retire.
- **bt-uahv** (P3, data-home) — canonical `.beads/` (shared with `bd`) vs `.bt/` (bt-only cache) split.
- **bt-5hl9** (P2, CompactIssue) — session column migration; reads from upstream first-class columns vs. metadata blob.
- **bt-3ltq** (P2, just-filed) — global git history as data layer (multi-repo correlation). Hard-gated on `pkg/workspace/loader.go` JSONL pin being resolved.

## ADR-002 ancestry

ADR-002 ("Stabilize and ship") already surfaced the post-Dolt data-layer migration as a sub-stream on 2026-04-25, listing bt-08sh / bt-z5jj / bt-uahv. ADR-003 will extend that thread by addressing the SourceType abstraction shape itself (not just the per-feature migrations).

## Open questions (deferred to ADR-003)

1. Does the SourceType abstraction survive in its current 5-type shape after SQLite removal, or does it collapse to a Dolt-first design with explicit JSONL-fallback semantics?
2. Should JSONL-fallback support remain at all, or is it a non-goal once bt's audience is exclusively v1.0.1+ beads users?
3. Where do worktree JSONL files (`SourceTypeJSONLWorktree`) fit in the post-decision world? Beads upstream supports git worktrees against Dolt; the JSONL-worktree path may be obsolete on the same v1.0.1 boundary.

## Method

- File survey: `Grep` of `loader.LoadIssues|loader.LoadIssuesFromFile|loader.FindJSONLPath` across the whole tree, then manual classification (production vs. test, smart-layer vs. bypassing pin).
- Architecture survey: read of `internal/datasource/source.go`, `internal/datasource/load.go`, `cmd/bt/root.go:215-285`, `pkg/ui/background_worker.go:1980-2110`.
- Beads landscape: `bd search "..." --json` and `bd show` for the related beads above.
- Cross-checked against AGENTS.md "Beads architecture awareness" section (verified 2026-04-25).
