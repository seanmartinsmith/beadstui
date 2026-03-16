# Audit Report: Data Layer

**Team**: 5
**Scope**: internal/ - datasource (JSONL, SQLite, Dolt), dolt reader, doltctl lifecycle, models (pkg/model)
**Lines scanned**: 4,455 (internal/: 4,129 across 14 files; pkg/model/types.go: 326)

## Architecture Summary

The data layer is organized around three packages: `internal/datasource` (the multi-source discovery, validation, selection, and reading engine), `internal/doltctl` (Dolt server lifecycle management), and `pkg/model` (the shared Issue/Dependency/Comment/Sprint data structures). The model package lives in `pkg/` rather than `internal/` because it is consumed by ~140 files across the codebase. Notably, there is no `internal/dolt/` or `internal/models/` directory despite the task scope suggesting them - all data layer code lives in the two internal packages listed above.

The datasource package implements a "smart loading" pipeline: discover all available sources (Dolt, SQLite, JSONL local, JSONL worktree) -> validate each -> select the freshest/highest-priority valid source -> load issues from it. This pipeline supports graceful degradation: if Dolt is unreachable, it falls back to SQLite or JSONL unless `metadata.json` declares `backend=dolt`, in which case fallback is blocked via `ErrDoltRequired`. The `doltctl` package handles a separate concern: detecting whether a Dolt server is already running (via TCP dial), starting one via `bd dolt start` if not, tracking ownership via PID files, and cleanly stopping only servers that bt started.

The model package defines 7 primary types (Issue, Status, IssueType, Dependency, DependencyType, Comment, Sprint) plus 3 analytics types (Forecast, BurndownPoint, IssueMetrics). The Issue struct has 24 fields covering core tracking data, compaction metadata, and relationship pointers. The model is backend-agnostic - it knows nothing about how data is stored.

## Feature Inventory

| Feature | Location | LOC | Dolt-Compatible | Tested | Functional | Notes |
|---------|----------|-----|-----------------|--------|------------|-------|
| Source discovery (multi-backend) | source.go:97 | 388 | Yes | Yes | Yes | Discovers Dolt, SQLite, JSONL local, JSONL worktree |
| Dolt config & port resolution | metadata.go:38 | 104 | Yes | Yes (11 tests) | Yes | Port chain: BEADS_DOLT_SERVER_PORT > BT_DOLT_PORT > .port file > config.yaml > 3307 |
| Dolt reader (full query) | dolt.go:64 | 342 | Yes | Partial (unit, no integration) | Yes | 22 columns, labels/deps/comments from separate tables |
| Dolt reader (simple fallback) | dolt.go:187 | ~55 | Yes | No direct tests | Yes | 8-column fallback when full query fails |
| SQLite reader (full query) | sqlite.go:66 | 358 | N/A (legacy) | Yes | Yes | Labels stored as JSON in-row, deps/comments from tables |
| SQLite reader (simple fallback) | sqlite.go:191 | ~50 | N/A (legacy) | No direct tests | Yes | Same 8-column fallback pattern |
| JSONL discovery (local) | source.go:223 | ~50 | N/A (legacy) | Yes | Yes | Scans .beads/ for .jsonl files, skips backups/merge artifacts |
| JSONL discovery (worktree) | source.go:275 | ~65 | N/A (legacy) | No | Yes | Scans git worktree beads directories |
| Source validation (Dolt) | validate.go:85 | ~32 | Yes | No (needs server) | Yes | Pings server, checks issues table via information_schema |
| Source validation (SQLite) | validate.go:120 | ~83 | N/A | Yes (4 tests) | Yes | Integrity check, schema check, column check |
| Source validation (JSONL) | validate.go:205 | ~117 | N/A | Yes (4 tests) | Yes | Error rate threshold (10%), required field check, BOM handling |
| Source selection | select.go:54 | 260 | Yes | Yes (6 tests) | Yes | Freshest-first (default) or priority-first; age delta filter |
| Source fallback chain | select.go:192 | ~70 | Yes | Yes (3 tests) | Yes | Try sources in order, skip failing ones |
| Source diff / inconsistency detection | diff.go:1 | 269 | Yes | No | **Unused** | Never called from outside diff.go |
| Source watcher (fsnotify) | watch.go:1 | 210 | Partial | No | **Unused** | Watches file sources; not useful for Dolt (no file to watch) |
| Auto-refresh manager | watch.go:222 | 145 | Partial | No | **Unused** | Built on SourceWatcher; never instantiated anywhere |
| Smart loading (LoadIssues) | load.go:19 | 178 | Yes | No direct tests | Yes | Orchestrates discover->validate->select->load |
| LoadIssuesWithSource | load.go:94 | ~55 | Yes | No direct tests | Yes | Same as LoadIssues but returns selected DataSource |
| Dolt server lifecycle | doltctl.go:71 | 198 | Yes | Yes (9 tests) | Yes | EnsureServer, StopIfOwned, PID-based ownership |
| Issue model | pkg/model/types.go:9 | 97 | Yes | Yes | Yes | 24 fields, Clone(), Validate() |
| Status enum | pkg/model/types.go:120 | ~40 | Yes | Yes (3 tests) | Yes | 9 statuses including deferred, pinned, hooked, review |
| IssueType enum | pkg/model/types.go:160 | ~25 | Yes | Yes (2 tests) | Yes | Extensible - any non-empty string is valid |
| Dependency model | pkg/model/types.go:189 | ~50 | Yes | Yes | Yes | 4 types: blocks, related, parent-child, discovered-from |
| Comment model | pkg/model/types.go:242 | ~8 | Yes | Yes | Yes | Simple: id, issue_id, author, text, created_at |
| Sprint model | pkg/model/types.go:251 | ~30 | Yes | Yes | Yes | Not loaded from Dolt - only from JSONL via pkg/loader |
| Forecast model | pkg/model/types.go:285 | ~20 | N/A | Yes | Yes | Analytics model, not persisted in Dolt |
| BurndownPoint model | pkg/model/types.go:308 | ~20 | N/A | Yes | Yes | Analytics model, not persisted in Dolt |
| IssueMetrics model | pkg/model/types.go:198 | ~8 | N/A | No direct tests | Yes | Used by search/export, not loaded from data layer |

## Dependencies

- **Depends on**:
  - `pkg/model` - Issue, Dependency, Comment types (datasource imports model)
  - `pkg/loader` - JSONL loading, GetBeadsDir, FindJSONLPath (datasource.load imports loader)
  - `github.com/go-sql-driver/mysql` - Dolt/MySQL driver (datasource.dolt, datasource.source, datasource.validate)
  - `modernc.org/sqlite` - Pure-Go SQLite driver (datasource.sqlite, datasource.validate)
  - `github.com/goccy/go-json` - Fast JSON parser (datasource.metadata, datasource.validate)
  - `gopkg.in/yaml.v3` - YAML parser for Dolt config.yaml (datasource.metadata)
  - `github.com/fsnotify/fsnotify` - File watcher (datasource.watch) **unused dependency**

- **Depended on by**:
  - `cmd/bt/main.go` - imports both datasource and doltctl; orchestrates startup, loading, reconnection
  - `pkg/ui/model.go` - stores `*datasource.DataSource` to track active source type
  - `pkg/ui/background_worker.go` - uses `datasource.NewDoltReader` for poll loop; uses `datasource.LoadFromSource` for reloads
  - `pkg/model` - imported by ~140 files across every package (analysis, export, search, ui, loader, etc.)

## Dead Code Candidates

1. **watch.go (entire file, 355 LOC)**: `SourceWatcher`, `AutoRefreshManager`, `NewSourceWatcher`, `NewAutoRefreshManager`, `ForceRefresh` - none of these are referenced from any file outside watch.go. The TUI uses a Dolt poll loop (background_worker.go) instead of fsnotify for change detection. The file-based watcher design doesn't work for Dolt (no file to watch). This pulls in `github.com/fsnotify/fsnotify` as a dependency for zero functionality.

2. **diff.go (entire file, 269 LOC)**: `SourceDiff`, `StatusDifference`, `DetectInconsistencies`, `CompareSources`, `CheckAllSourcesConsistent`, `InconsistencyReport`, `GenerateInconsistencyReport` - all exported functions and types, but none are called from anywhere outside diff.go itself. The private helper `loadIssuesFromSource` is also only used within diff.go.

3. **SQLite reader (sqlite.go, 358 LOC)**: Upstream beads removed SQLite backend in v0.56.1. The code is reachable through the discovery pipeline (when `RequireDolt=false` and a beads.db file exists), but this path should never trigger for any current beads installation. It is also called directly from `cmd/bt/main.go:6851` in the `metadataPreferredSource` flow. The `modernc.org/sqlite` driver adds significant binary size.

4. **JSONL worktree discovery (source.go:275-340)**: The `discoverWorktreeSources` function scans `beads-worktrees` directories under `.git/`. With Dolt as the only backend, worktree JSONL is stale data.

5. **JSONL local discovery (source.go:223-272)**: Similarly, local JSONL files in `.beads/` are no longer the source of truth when Dolt is the backend.

6. **SelectWithFallback (select.go:192-260)**: Only tested in source_test.go, never called from production code. `SelectBestSource` and `SelectBestSourceWithOptions` are the actual production entry points.

## Notable Findings

**Schema divergence between Dolt and SQLite readers**: The Dolt reader queries `due_at` (line dolt.go:69) while the SQLite reader queries `due_date` (line sqlite.go:72). The Dolt reader reads `close_reason` (dolt.go:72) but the SQLite reader does not - it has no close_reason handling at all. The Dolt reader uses column name `type` for dependencies (dolt.go:263: `SELECT depends_on_id, type FROM dependencies`) while SQLite uses `dependency_type` (sqlite.go:246). These reflect actual Dolt vs. SQLite schema differences in the upstream beads data, but if the SQLite path were ever resurrected, issues loaded from SQLite would be missing CloseReason.

**N+1 query pattern in Dolt reader**: `LoadIssuesFiltered` (dolt.go:64) loads all issues in one query, but then for each issue, issues 3 additional queries: `loadLabels`, `loadDependencies`, `loadComments`. For a project with N issues, this executes 1 + 3N queries. With 100 issues, that's 301 round-trips to the Dolt server. The SQLite reader has the same pattern for dependencies and comments (but labels are inline JSON in SQLite).

**Silent row skip on scan error**: Both Dolt reader (dolt.go:100-102) and SQLite reader (sqlite.go:103-105) silently `continue` when `rows.Scan` fails. This means corrupted or schema-mismatched rows are silently dropped with no log entry or metric. The `loadIssuesSimple` fallback is triggered only on the initial `db.Query` error, not per-row scan failures.

**Dolt validation uses different table check than reader**: `validateDolt` (validate.go:98) queries `information_schema.tables` to check if the issues table exists. `NewDoltReader` (dolt.go:41) uses `SHOW TABLES LIKE 'issues'`. Both work but the inconsistency is unnecessary.

**Missing columns vs upstream schema**: The beads upstream schema (v6) has ~50+ columns on the issues table including wisps, molecules, agents, gates, federation-related fields. The bt Dolt reader only queries 22 columns. The `loadIssuesSimple` fallback uses only 8. There are no Dolt system table queries (dolt_log, dolt_diff, dolt_status, etc.) anywhere in the codebase - this is relevant for the open bead bt-thpq (history view) which would need these.

**Port discovery chain is correct and well-tested**: The chain in metadata.go (config.yaml -> port file -> BT_DOLT_PORT env -> BEADS_DOLT_SERVER_PORT env) is thoroughly tested with 11 tests covering every override scenario. The same chain is used by doltctl.go via `datasource.ReadDoltConfig` - single source of truth.

**IssueType.IsValid() is deliberately permissive**: Any non-empty string passes validation. The comment (types.go:172) explains this is for extensibility with the beads ecosystem's Gastown orchestration types (role, agent, molecule). This is intentional, not a bug.

**DependencyType empty string treated as blocking**: `IsBlocking()` (types.go:231) returns true for empty string for backward compatibility with legacy beads data. This is documented in the code comment.

**Sprint/Forecast/BurndownPoint not loaded from Dolt**: These models exist in pkg/model but are only loaded via JSONL (pkg/loader/sprint.go) or computed in-memory (pkg/analysis). The Dolt reader has no Sprint or Forecast queries. If beads adds Sprint tables to Dolt, bt would need new reader code.

**IssueMetrics not persisted**: The `IssueMetrics` struct exists in the model but is computed at runtime by pkg/search and pkg/analysis. It's never loaded from any data source.

**Comment.ID is int64 but Dependency has no ID field**: The Comment struct uses `int64` for ID (matching SQLite auto-increment), but the Dependency struct has no ID field at all - dependencies are identified by the (issue_id, depends_on_id) composite key.

**doltctl has good test coverage**: 9 tests cover parsing, PID matching, PID-gone, PID-mismatch, stop failure, and bd-not-found scenarios. The `stopFunc` injection pattern enables unit testing without a real Dolt server.

**Dual JSON libraries**: `metadata.go` and `validate.go` use `github.com/goccy/go-json` (fast), while `sqlite.go` uses `encoding/json` (stdlib). Not a bug, but inconsistent.

## Questions for Synthesis

1. **Should watch.go and diff.go be removed entirely?** They account for 624 LOC of dead code and pull in the fsnotify dependency. The Dolt poll loop in background_worker.go supersedes the file watcher design.

2. **Should the SQLite reader be removed?** Upstream beads removed SQLite in v0.56.1. Keeping it adds ~358 LOC of code to maintain, the `modernc.org/sqlite` dependency (~15MB binary impact), and schema divergence risk. Counter-argument: it provides a fallback for projects using older beads versions.

3. **Should SelectWithFallback be removed?** It's tested but never called from production code. The existing `SelectBestSource` flow handles the use case.

4. **Is the N+1 query pattern in the Dolt reader acceptable?** For small projects (tens of issues) it's fine. For large projects it could add noticeable latency. A JOIN-based approach or batch loading could reduce round trips.

5. **Should rows.Scan errors in Dolt/SQLite readers be logged rather than silently skipped?** Silent data loss is generally undesirable, especially during schema transitions when beads upstream adds new columns.

6. **Cross-team: does the UI layer (Team 3/4) depend on any of the dead code paths identified here?** The background_worker.go uses `datasource.NewDoltReader` and `datasource.LoadFromSource` directly - need to confirm it doesn't use watch.go or diff.go.

7. **Should Sprint loading be added to the Dolt reader?** The Sprint model exists and the JSONL Sprint loader exists in pkg/loader/sprint.go, but there's no Dolt Sprint query. This depends on whether upstream beads has a sprints table.
