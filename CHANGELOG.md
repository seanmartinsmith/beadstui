# Changelog

Development log for beadstui. Each entry covers one Claude Code session's work, keyed by date.

For architectural decisions, see `docs/adr/`. For issue tracking, use `bd list`.

---

## 2026-04-03c - Global hub data layer (bt-6wbd phase 1)

**GlobalDoltReader shipped**: `internal/datasource/global_dolt.go` - connects to shared Dolt server without a database in the DSN, enumerates all beads project databases, loads issues via UNION ALL with backtick-quoted `database.table` syntax.

**Key implementation**:
- `DiscoverSharedServer()` reads `~/.beads/shared-server/dolt-server.port`, env override via `BT_GLOBAL_DOLT_PORT`
- `EnumerateDatabases()` uses `information_schema.tables` (single query, not N validation queries), filters system DBs
- `LoadIssues()` via UNION ALL across all databases, `SourceRepo` set from database name (overrides column)
- Batch labels/deps/comments via 3 UNION ALL queries (not N+1 per-issue)
- `GetLastModified()` via aggregated `MAX(MAX(updated_at))` across all databases
- Partial failure: broken DBs skipped with `slog.Warn`, healthy DBs loaded

**Source type integration**: `SourceTypeDoltGlobal` added to source.go, `RepoFilter` field on `DataSource`, `LoadFromSource` dispatch case in load.go.

**Poll loop**: `globalDoltPollOnce()` in background_worker.go, dispatched when source type is `SourceTypeDoltGlobal`. Reconnect does TCP dial only (no auto-start, shared server is user-managed).

**CLI**: `--global` flag, mutually exclusive with `--workspace` and `--as-of`. `--repo` filters database list at enumeration (before UNION ALL). Workspace mode UI activates automatically (badges, picker, prefilter).

**Shared column list**: Extracted `IssuesColumns` constant to `columns.go`, used by both `DoltReader` and `GlobalDoltReader`.

**Tests**: 16 new unit tests in `global_dolt_test.go` (query building, system DB filtering, backtick quoting, discovery, DSN construction). Full suite: 27 packages, 0 failures.

**Files created**: `internal/datasource/global_dolt.go`, `internal/datasource/global_dolt_test.go`, `internal/datasource/columns.go`
**Files modified**: `internal/datasource/dolt.go`, `internal/datasource/source.go`, `internal/datasource/load.go`, `pkg/ui/background_worker.go`, `cmd/bt/main.go`

**Bead closed**: bt-6wbd

## 2026-04-03b - BQL bug fixes + global hub planning

**BQL bugs (bt-bjk4)**: Fixed all 5 bugs from gap analysis:
1. Status enum validation - added `ValidStatusValues` map, catches typos like `status=opne`
2. `--robot-bql` envelope - now uses `RobotEnvelope` + `robotEncoder` (adds metadata, TOON support)
3. Dead code removal - removed unused `WithReadySQL` from sql.go
4. Date equality semantics - `created_at = today` now matches any time on that day (truncates to midnight)
5. ISO date parsing - `created_at > 2026-01-15` now works in lexer, parser, and executor

Tests added for all fixes. Full suite passes (27 packages, 0 failures).

**Triage**: bt-dx7k reopened (blocked, not in-progress), bt-28g8 closed (audit done), bt-2bns deferred (Charm v2), bt-xft1 closed (resolved by shared server architecture).

**Global hub design verification**: Verified 5 assumptions from the beads session's design doc against actual codebase. Updated open questions with findings. Key correction: poll system needs real refactoring, not just a query swap.

**Global hub data layer plan**: `docs/plans/2026-04-03-feat-global-hub-data-layer-plan.md` - 4-phase implementation plan for GlobalDoltReader. Batch N+1 queries into 3 UNION ALL, single aggregated MAX for poll, --global flag, workspace UI reuse.

**Beads closed**: bt-bjk4 (BQL bugs), bt-28g8 (keybinding audit), bt-xft1 (data separation)
**ADR-002 updated**: Stream 2 bugs all checked off, Stream 1 robot-bql checked off

## 2026-04-03 - Parallel audit swarm

Burned expiring weekly credits on 5 parallel research agents. All read-only, no code changes.

**Reports produced**:
- `docs/audit/test-suite-audit.md` - 268 test files: 93% KEEP, 0% REMOVE, 1 Windows P1
- `docs/audit/cli-ergonomics-audit.md` - 97 flags inventoried, 3 critical robot-mode envelope bugs
- `docs/audit/charm-v2-migration-scout.md` - 76 files affected, 60% mechanical, theme system is the hard part
- `docs/audit/bql-gap-analysis.md` - corrected stale memory (--bql/--robot-bql already shipped), found 5 bugs
- `docs/drafts/README-draft.md` - complete prose rewrite draft

**Beads closed**: bt-79eg (test audit), bt-pfic (CLI audit)
**Beads created**: bt-0cht (P1, robot-mode fixes), bt-5dvl (P2, test fixes), bt-bjk4 (P2, BQL bugs), bt-iuqy (P2, README review)
**ADR-001 closed out**, ADR-002 created as new project spine. Changelog extracted to this file.

## 2026-04-01 - BQL composable search

New package `pkg/bql/` - BQL parser vendored from zjrosen/perles (MIT), adapted for bt.

- Parser layer: lexer, parser, AST, tokens, validator, SQL builder (~1,500 LOC)
- MemoryExecutor: in-memory evaluation against model.Issue (522 LOC, 28 tests)
- TUI integration: `:` keybind opens BQL modal, dedicated `applyBQL()` filter path
- CLI: `--bql` and `--robot-bql` flags
- Syntax: =, !=, <, >, <=, >=, ~, !~, IN, NOT IN, AND/OR/NOT, parens, P0-P4, date literals, ORDER BY, EXPAND

22 files, ~3,950 lines, 27 packages pass, 0 failures.

## 2026-03-16b - Cross-platform test suite fixes

39 failing Windows tests -> 0 failures across all 26 packages.

- Phase 1: Renamed bv->bt stragglers in 8 files
- Phase 1b: Fixed ComputeUnblocks (filter blocking edges only), slug collision expectations
- Phase 2: filepath.FromSlash/Join in cass and tree test expectations
- Phase 3: configHome override for tutorial progress + wizard config (HOME env doesn't work on Windows)
- Phase 4: runtime.GOOS skip guards for 6 Unix-only permission tests
- Phase 5: Shell-dependent hooks tests skipped on Windows; fixed -r shorthand conflict
- Phase 6: .exe suffix for drift test binaries; file locking fix (defer order)
- Phase 7: Normalized \r\n in golden file comparison

**Closed**: bt-s3xg, bt-zclt, bt-3ju6, bt-7y06, bt-ri5b, bt-dwbl, bt-kmxe, bt-mo7r (8 issues)

## 2026-03-16a - Dolt lifecycle adaptation

New module `internal/doltctl/` for Dolt server management.

- EnsureServer: detects running server (TCP dial) or starts via `bd dolt start`
- StopIfOwned: PID-based ownership check before `bd dolt stop`
- Auto-reconnect: poll loop retries EnsureServer after 3 consecutive failures
- Port discovery chain: BEADS_DOLT_SERVER_PORT > BT_DOLT_PORT > .beads/dolt-server.port > config.yaml > 3307
- Database identity check: `SHOW TABLES LIKE 'issues'` after connecting
- Dead code removed: touchDoltActivity keepalive

11 doltctl tests + 6 metadata tests. **Closed**: bt-07jp (P1), bt-tebr (P2, subsumed)

## 2026-03-12 - Brainstorm + audit planning

No code changes. Post-takeover roadmap brainstorm + codebase audit design.

- Defined 4 phases: Audit -> Stabilize -> Polish -> Interactive
- Key decision: CRUD via bd shell-out (no beads fork needed)
- Designed 8-team parallel codebase audit (~190k LOC)
- Created 8 dogfood beads from TUI usage
- Docs: `docs/brainstorms/2026-03-12-post-takeover-roadmap.md`, `docs/plans/2026-03-12-codebase-audit-plan.md`

## 2026-03-11b - Dolt freshness + responsive help

- **bt-3ynd**: Fixed false STALE indicator - freshness tracks last successful poll, not snapshot build time
- **bt-aog1**: Responsive help overlay - 4x2 grid (wide), 2x4 (medium), single column (narrow)
- **bt-xavk**: Created help system redesign plan (docs/plans/help-system-redesign.md)

## 2026-03-11a - Dogfood polish

- Absolute timestamps in details pane + expanded card
- Priority shows P0-P4 text next to icon
- Status bar auto-clear after 3s
- Help overlay: centered titles, auto-sized panels, 4x2 grid, status indicators panel
- Board: auto-hide empty columns on card expand
- Shortcut audit: found 22 undocumented keys

## 2026-03-07 - Beads migration

Renamed issue prefix bv->bt (553 issues). Set beads.role=maintainer. Local folder renamed. Memory migrated.

## 2026-03-05c - ADR review cleanup

Fixed 14 stale `bv` CLI refs in AGENTS.md. Fixed insights detail panel viewport off-by-one.

## 2026-03-05b - Titled panels

Converted insights, board, and help overlay to RenderTitledPanel. Added BorderColor/TitleColor overrides. Board cards use RoundedBorder + border-only selection.

## 2026-03-05a - Tomorrow Night theme

Visual overhaul: Tomorrow Night + matcha-dark-sea teal. Theme config system (embedded defaults, layered loading). TitledPanel helper. Swapped all Color* vars. 18 new tests.

## 2026-02-25 to 2026-03-04 - Fork takeover

See [ADR-001](docs/adr/001-btui-fork-takeover.md) for detailed session-by-session changelog of the fork takeover work (streams 1-4: Dolt verification, rename, data migration, spring cleaning).
