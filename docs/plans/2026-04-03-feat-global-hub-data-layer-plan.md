---
title: "feat: Global Hub Data Layer (bt-6wbd phase 1)"
type: feat
status: active
date: 2026-04-03
origin: docs/plans/2026-04-03-global-hub-design.md
bead: bt-6wbd
---

# Global Hub Data Layer (bt-6wbd phase 1)

## Overview

Build a `GlobalDoltReader` that connects to beads' shared Dolt server, enumerates all project databases, and loads issues from all of them via `UNION ALL` queries with `database.table` qualified names. Adapt the poll-based refresh system to detect changes across multiple databases. Populate `Issue.SourceRepo` from the database name.

Phases 1-3 are data layer only - pure Go, no Charm/Bubble Tea dependency, framework-agnostic. They will survive the Charm v2 migration untouched. Phase 4 has two parts: 4a-4b are pure CLI/flag work (no Charm dependency), 4c-4d wire into the Bubble Tea UI model via `EnableWorkspaceMode` and will need review during the Charm v2 migration (bt-zta9).

## Problem Statement

bt currently connects to one Dolt database per session. Users with 9+ beads-tracked projects have no way to see all issues in a single view. The shared Dolt server (beads v1.0.0+) hosts all project databases on one server, but bt can't query across them.

## Design Decisions (from origin doc + codebase verification)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Activation | `--global` flag | Explicit, no magic. Auto-detect later. |
| Poll strategy | Option B: single aggregated MAX | One query, one timestamp, simplest for phase 1. Full reload on any change. |
| N+1 labels/deps/comments | Batch into 3 UNION ALL queries | Unqualified table names fail without a selected DB. N+1 is 900+ queries at 10 DBs. |
| Interface contract | New `SourceTypeDoltGlobal` + dispatch | Surgical addition, no broad refactor. No new Reader interface. |
| Shared server lifecycle | Fail with clear error if not running | No auto-start. Shared server is user-managed or started by `bd dolt start --shared`. |
| Partial DB failure | Skip broken databases, warn in status bar | One stale project shouldn't take down the entire global view. |
| SourceRepo | Always set from database name | Overrides column value in global mode. Database name is the authoritative source identity. |
| DB enumeration | Once at startup | Re-enumeration on each poll adds complexity for minimal benefit in phase 1. |
| SQL quoting | Always backtick-quote database names | Defensive. Handles names like `my-project` that contain hyphens. |
| Flag interactions | `--global` is mutually exclusive with `--workspace` and `--as-of` | Composes with `--repo`, `--bql`, `--robot-*` |

## Technical Approach

### Architecture

```
cmd/bt/main.go
  --global flag
  |
  v
internal/datasource/global_dolt.go   (NEW)
  GlobalDoltReader struct
  - DiscoverSharedServer()     -> port from ~/.beads/shared-server/dolt-server.port
  - EnumerateDatabases()       -> SHOW DATABASES, filter, validate
  - LoadIssues()               -> UNION ALL across db.issues
  - LoadLabels()               -> UNION ALL across db.labels, grouped by issue_id
  - LoadDependencies()         -> UNION ALL across db.dependencies
  - LoadComments()             -> UNION ALL across db.comments
  - GetLastModified()          -> SELECT MAX(m) FROM (UNION ALL of MAX(updated_at) per db)
  - Close()
  |
  v
internal/datasource/source.go        (MODIFY)
  + SourceTypeDoltGlobal
  |
  v
internal/datasource/load.go          (MODIFY)
  + LoadFromSource case for SourceTypeDoltGlobal
  |
  v
pkg/ui/background_worker.go          (MODIFY)
  + globalDoltPollOnce() variant
  + dispatch on source type in poll loop starter
```

### File Inventory

| File | Action | What Changes |
|------|--------|-------------|
| `internal/datasource/global_dolt.go` | **CREATE** | GlobalDoltReader struct, all methods |
| `internal/datasource/global_dolt_test.go` | **CREATE** | Unit tests for enumeration, query building, SourceRepo population |
| `internal/datasource/columns.go` | **CREATE** | Shared `IssuesColumns` constant used by both DoltReader and GlobalDoltReader |
| `internal/datasource/dolt.go` | MODIFY | Replace inline column list with `IssuesColumns` reference |
| `internal/datasource/source.go` | MODIFY | Add `SourceTypeDoltGlobal` constant, add `RepoFilter` field to `DataSource` |
| `internal/datasource/load.go` | MODIFY | Add `SourceTypeDoltGlobal` case in `LoadFromSource()` |
| `pkg/ui/background_worker.go` | MODIFY | Add `globalDoltPollOnce()`, dispatch in `startDoltPollLoop()` |
| `cmd/bt/main.go` | MODIFY | Add `--global` flag, new loading branch, mutual exclusion with `--workspace`/`--as-of` |

### Implementation Phases

#### Phase 1: GlobalDoltReader Core (internal/datasource/global_dolt.go)

**Deliverables**: New file with the reader struct and all query methods.

**GlobalDoltReader struct**:
```go
type GlobalDoltReader struct {
    db        *sql.DB
    databases []string // validated beads databases
    dsn       string
}
```

**1a. Shared server discovery**

`DiscoverSharedServer() -> (host string, port int, error)`
- Read `~/.beads/shared-server/dolt-server.port` (same format as per-project port files)
- Env var override: `BT_GLOBAL_DOLT_PORT` (parallel to `BT_DOLT_PORT`)
- Default host: `127.0.0.1`, user: `root`
- DSN: `` root@tcp(127.0.0.1:<port>)/?parseTime=true&timeout=2s `` (no database in path)
- If port file doesn't exist: return clear error `"shared Dolt server not running - ensure at least one project is configured with 'bd init --shared-server'"`

**1b. Database enumeration and validation**

`EnumerateDatabases(db *sql.DB) -> ([]string, error)`
- Run `SHOW DATABASES`
- Filter deny-list: `information_schema`, `mysql`, `dolt`, `dolt_procedures`, `sys`
- For each remaining database, verify it has an `issues` table:
  ```sql
  SELECT TABLE_SCHEMA FROM information_schema.tables 
  WHERE TABLE_NAME = 'issues' 
  AND TABLE_SCHEMA NOT IN ('information_schema','mysql','dolt','dolt_procedures','sys')
  ```
  (Single query instead of N validation queries)
- If zero valid databases: return error `"no beads databases found on shared server"`
- Log discovered databases at slog.Info level: `"global mode: discovered N databases [db1, db2, ...] - restart bt to pick up new projects"`
- Print to stderr on startup (not just slog): `"global mode: N databases (db1, db2, db3, ...)"` so the user sees what's loaded even without debug logging

**1c. Shared column list + UNION ALL query builder**

First, extract the column list from `dolt.go:65-72` into a shared constant in `internal/datasource/columns.go`:
```go
// IssuesColumns is the canonical column list for the issues table.
// Used by both DoltReader and GlobalDoltReader. One place to update
// when beads adds columns upstream.
const IssuesColumns = `id, title, description, status, priority, issue_type,
    assignee, estimated_minutes, created_at, updated_at,
    due_at, closed_at, external_ref, compaction_level,
    compacted_at, compacted_at_commit, original_size,
    design, acceptance_criteria, notes, source_repo,
    close_reason`
```

Then update `DoltReader.LoadIssuesFiltered` (dolt.go:65) to use `IssuesColumns` instead of its inline column list. This is a prerequisite - do it first and verify tests still pass before creating GlobalDoltReader.

`buildIssuesQuery(databases []string) -> string`
- For each database, generate:
  ```sql
  SELECT <IssuesColumns>, '<db_name>' AS _global_source
  FROM `<db_name>`.issues
  WHERE status != 'tombstone'
  ```
- Join parts with `UNION ALL`
- Append `ORDER BY updated_at DESC`
- The `_global_source` column carries the database name alongside whatever `source_repo` may contain
- Both readers use `IssuesColumns` - if beads adds columns, update one constant

**1d. Batch labels/dependencies/comments**

Three UNION ALL queries, one per relation type:

```sql
-- Labels
SELECT issue_id, label, '<db>' AS _db FROM `<db1>`.labels
UNION ALL
SELECT issue_id, label, '<db>' AS _db FROM `<db2>`.labels

-- Dependencies  
SELECT issue_id, depends_on_id, type, '<db>' AS _db FROM `<db1>`.dependencies
UNION ALL
SELECT issue_id, depends_on_id, type, '<db>' AS _db FROM `<db2>`.dependencies

-- Comments
SELECT id, issue_id, author, text, created_at, '<db>' AS _db FROM `<db1>`.comments
UNION ALL
SELECT id, issue_id, author, text, created_at, '<db>' AS _db FROM `<db2>`.comments
ORDER BY created_at
```

Group results in Go by `issue_id` and attach to the corresponding Issue. Total: 4 queries (issues + labels + deps + comments) regardless of database count.

**1e. SourceRepo population**

After scanning each issue row:
```go
issue.SourceRepo = globalSource // from the _global_source column
```

Always overrides whatever is in the `source_repo` column. In global mode, the database name IS the project identity.

**1f. GetLastModified for poll**

```sql
SELECT MAX(m) FROM (
  SELECT MAX(updated_at) AS m FROM `db1`.issues
  UNION ALL
  SELECT MAX(updated_at) AS m FROM `db2`.issues
  ...
) t
```

Returns single timestamp. Semantics match existing `DoltReader.GetLastModified()`.

**Success criteria**:
- [ ] `GlobalDoltReader` connects to shared server without database in DSN
- [ ] `EnumerateDatabases` filters system DBs and validates beads schema
- [ ] `LoadIssues` returns issues from all databases with `SourceRepo` set
- [ ] Labels, dependencies, comments loaded via batch UNION ALL (4 total queries)
- [ ] `GetLastModified` returns aggregate max across all databases
- [ ] Broken databases are skipped with slog.Warn, not fatal
- [ ] Database names are backtick-quoted in all generated SQL

#### Phase 2: Source Type Integration (source.go, load.go)

**Deliverables**: Wire GlobalDoltReader into the existing datasource framework.

**2a. New source type** (`source.go`):
```go
SourceTypeDoltGlobal SourceType = "dolt_global"
```
Global sources are only created by the `--global` flag, not by `DiscoverSources()`, so the priority field is unused.

**2b. LoadFromSource dispatch** (`load.go`):
Add case in `LoadFromSource()`:
```go
case SourceTypeDoltGlobal:
    reader, err := NewGlobalDoltReader(source)
    if err != nil {
        return nil, err
    }
    defer reader.Close()
    return reader.LoadIssues()
```

**2c. DataSource construction for global mode**:
New function `NewGlobalDataSource(host string, port int) DataSource` that builds a `DataSource` with:
- Type: `SourceTypeDoltGlobal`
- Path: DSN without database (the connection string)
- RepoFilter: optional string field on `DataSource` (empty = all databases, non-empty = case-insensitive match against database names during enumeration)
- No project-specific metadata

**Success criteria**:
- [ ] `SourceTypeDoltGlobal` added to source type enum
- [ ] `LoadFromSource` dispatches to `GlobalDoltReader`
- [ ] `NewGlobalDataSource` constructs a valid DataSource for global mode

#### Phase 3: Poll Loop Adaptation (background_worker.go + main.go reconnect)

**Deliverables**: Enable poll-based change detection in global mode.

This is the riskiest phase: the poll loop constructs a fresh `DoltReader` per cycle (line 2053) and assumes a single database. The change detection, backoff, and reconnect paths all need a second codepath for global mode. Regression risk is high since a bug here causes silent stale data or connection storms.

**3a. New poll function** `globalDoltPollOnce`:
```go
func (w *BackgroundWorker) globalDoltPollOnce(lastModified *time.Time, firstPoll *bool) error {
    reader, err := datasource.NewGlobalDoltReader(*w.dataSource)
    if err != nil {
        return err
    }
    defer reader.Close()
    
    modTime, err := reader.GetLastModified()
    if err != nil {
        return err
    }
    
    // NULL timestamp (all databases empty) scans as zero time in Go - skip comparison
    if modTime.IsZero() {
        return nil
    }
    
    // Same change detection logic as doltPollOnce
    if *firstPoll {
        *lastModified = modTime
        *firstPoll = false
        return nil
    }
    if !modTime.Equal(*lastModified) {
        *lastModified = modTime
        w.noteFileChange(time.Now())
        w.TriggerRefresh()
    }
    return nil
}
```

**3b. Poll loop dispatch** in `startDoltPollLoop`:
At line ~2060 where `doltPollOnce` is called, branch on source type:
```go
if w.dataSource.Type == datasource.SourceTypeDoltGlobal {
    err = w.globalDoltPollOnce(lastModified, firstPoll)
} else {
    err = w.doltPollOnce(lastModified, firstPoll)
}
```

**3c. Auto-reconnect adaptation**:
The existing `doltReconnectFn` calls `doltctl.EnsureServer(beadsDir)`. For global mode, the reconnect function should:
- Re-read the shared server port file
- Attempt TCP dial to the shared server
- NOT try to start the server (shared server is user-managed)

Set `doltReconnectFn` in `main.go` when in global mode:
```go
if *globalFlag {
    m.SetDoltServer(nil, func(beadsDir string) error {
        // Just verify the shared server is reachable, don't start it
        host, port, err := datasource.DiscoverSharedServer()
        if err != nil {
            return err
        }
        // TCP dial check
        conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 2*time.Second)
        if err != nil {
            return fmt.Errorf("shared Dolt server unreachable at %s:%d", host, port)
        }
        conn.Close()
        return nil
    })
}
```

**Success criteria**:
- [ ] `globalDoltPollOnce` detects changes across all databases
- [ ] Poll loop dispatches to correct function based on source type
- [ ] Auto-reconnect works for global mode (verify reachability, no auto-start)
- [ ] DoltVerifiedMsg and DoltConnectionStatusMsg emitted correctly
- [ ] Exponential backoff behavior preserved

#### Phase 4: main.go Integration (cmd/bt/main.go)

**Deliverables**: `--global` flag, new loading branch, mutual exclusion.

**4a. Flag definition** (near line 117):
```go
globalFlag := flag.Bool("global", false, "Show issues from all projects on shared Dolt server")
```

**4b. Flag validation and --repo interaction** (after `flag.Parse()`):
```go
if *globalFlag && *workspaceConfig != "" {
    fmt.Fprintln(os.Stderr, "Error: --global and --workspace are mutually exclusive")
    os.Exit(1)
}
if *globalFlag && *asOfRef != "" {
    fmt.Fprintln(os.Stderr, "Error: --global and --as-of are mutually exclusive")
    os.Exit(1)
}
```

**--repo interaction with --global**: `bt --global --repo bt` narrows the enumerated database list to only include databases matching the `--repo` value. This filtering happens in main.go's loading branch (4c), BEFORE constructing the reader - so only the matching database is queried:
```go
// In the --global loading branch, after DiscoverSharedServer:
globalSource := datasource.NewGlobalDataSource(host, port)
if *repoFilter != "" {
    globalSource.RepoFilter = *repoFilter // stored on DataSource, read by GlobalDoltReader.EnumerateDatabases
}
```
`EnumerateDatabases` checks `RepoFilter` and applies a case-insensitive match against database names, reducing the list before building any UNION ALL queries. This avoids adding methods to the `DataSource` struct - just a new optional field. The existing `filterByRepo()` in `cmd/bt/helpers.go:61-63` still runs afterward as a safety net but is effectively a no-op when `RepoFilter` was applied upstream.

**4c. Loading branch** (between workspace and single-repo, around line 517):
```go
} else if *globalFlag {
    host, port, err := datasource.DiscoverSharedServer()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Global mode error: %v\n", err)
        os.Exit(1)
    }
    
    globalSource := datasource.NewGlobalDataSource(host, port)
    result, err := datasource.LoadFromSource(globalSource)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Global mode load error: %v\n", err)
        os.Exit(1)
    }
    
    issues = result
    selectedSource = &globalSource
    // beadsPath stays empty (poll-based refresh handles liveness)
    
    // Build workspace info for UI (reuse workspace mode's badge/picker system)
    var repoPrefixes []string
    seen := make(map[string]bool)
    for _, issue := range issues {
        if issue.SourceRepo != "" && !seen[issue.SourceRepo] {
            repoPrefixes = append(repoPrefixes, issue.SourceRepo)
            seen[issue.SourceRepo] = true
        }
    }
    workspaceInfo = &workspace.LoadSummary{
        TotalRepos:   len(repoPrefixes),
        TotalIssues:  len(issues),
        RepoPrefixes: repoPrefixes, // verified: field exists at workspace/loader.go:256
    }
```

**4d. UI mode activation** (Charm-dependent - review during bt-zta9 migration)

The existing `EnableWorkspaceMode` call (main.go:1427-1435) already fires when `workspaceInfo != nil`. Global mode populates `workspaceInfo`, so the repo picker, badges, and prefilter activate automatically. No new UI code needed, but this wiring point touches `pkg/ui/model_modes.go:37-52` and will need review when Charm v2 migration changes the model struct.

**Note**: Steps 4a-4b are pure CLI work with no Charm dependency. Steps 4c-4d create the coupling between the data layer and the Bubble Tea model. During Charm v2 migration, only 4c-4d need revisiting - the `workspace.LoadSummary` construction is framework-agnostic, but `EnableWorkspaceMode` and `SetDoltServer` are Charm model methods.

**Success criteria**:
- [ ] `--global` flag recognized and documented in `--help`
- [ ] Mutual exclusion with `--workspace` and `--as-of`
- [ ] Issues load from all shared server databases
- [ ] Workspace mode UI activates (badges, `w` key picker, prefilter)
- [ ] Poll loop starts for global mode
- [ ] `--repo`, `--bql`, `--robot-*` flags compose correctly with `--global`

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Shared server not running | Exit with: `"shared Dolt server not running - ensure at least one project is configured with 'bd init --shared-server'"` |
| Port file exists but server unreachable | Exit with: `"shared Dolt server at 127.0.0.1:<port> is not responding"` |
| Zero valid beads databases | Exit with: `"no beads databases found on shared server"` |
| Some databases have schema issues | Skip broken DBs, log `slog.Warn`, continue with healthy DBs |
| All databases broken | Exit with: `"all databases on shared server failed validation"` |
| UNION ALL query fails mid-execution | Return error, poll loop enters backoff/reconnect |
| ID collision across databases | Not detected in phase 1 (known limitation, documented) |
| Connection drops mid-session | Existing backoff + reconnect pattern handles this |

## Known Limitations (Phase 1)

1. **No auto-start for shared server** - shared server starts automatically when a `bd` command runs in a project configured with `bd init --shared-server`. bt does not start it.
2. **Database list is static** - discovered at startup, printed to stderr. New projects added to the shared server require restarting bt to appear.
3. **ID collisions undetected** - two databases with overlapping prefixes silently overwrites in issueMap
4. **Full reload on any change** - one edit in one project reloads all databases. Acceptable at 10 DBs, not at 100.
5. **No write support** - global mode is read-only. Writing back requires knowing which project's `bd` to invoke.
6. **Cross-database dependencies** - will resolve if both databases are loaded. This is accidental correctness, not a designed feature.

## Testing Plan

### Unit Tests (global_dolt_test.go)

| Test | What It Validates |
|------|-------------------|
| `TestBuildIssuesQuery` | UNION ALL SQL generation for 1, 3, 10 databases. Backtick quoting. Column count. |
| `TestBuildIssuesQuery_Empty` | Zero databases returns error, not empty SQL |
| `TestBuildLabelsQuery` | Labels UNION ALL with _db column |
| `TestBuildDependenciesQuery` | Dependencies UNION ALL with correct column names (type, not dependency_type) |
| `TestBuildCommentsQuery` | Comments UNION ALL ordered by created_at |
| `TestBuildLastModifiedQuery` | Aggregated MAX(MAX(updated_at)) across databases |
| `TestFilterSystemDatabases` | Deny-list filtering (information_schema, mysql, dolt, etc.) |
| `TestBacktickQuoting` | Database names with hyphens, underscores, numbers |
| `TestDiscoverSharedServer_PortFile` | Reads port from ~/.beads/shared-server/dolt-server.port |
| `TestDiscoverSharedServer_EnvOverride` | BT_GLOBAL_DOLT_PORT takes precedence |
| `TestDiscoverSharedServer_Missing` | Clear error when port file doesn't exist |
| `TestSourceRepoPopulation` | SourceRepo set from database name, not from column |
| `TestPartialDatabaseFailure` | Broken DB skipped, healthy DBs loaded, warning logged |

### Integration Tests (requires running Dolt server)

These run against a real shared Dolt server with test databases. Gated behind `BT_INTEGRATION_TEST=1`.

| Test | What It Validates |
|------|-------------------|
| `TestGlobalDoltReader_EndToEnd` | Full flow: discover, enumerate, load, verify SourceRepo |
| `TestGlobalDoltReader_LiveChange` | Create issue in one DB, verify GetLastModified changes |
| `TestGlobalDoltReader_CrossDBLabels` | Labels from multiple DBs correctly assigned to issues |

## Sources & References

### Origin
- **Design doc:** [docs/plans/2026-04-03-global-hub-design.md](../plans/2026-04-03-global-hub-design.md) - empirical verification of shared server queries, open questions answered by codebase verification (session 18)
- Key decisions carried forward: multi-database federation (Option B), UNION ALL approach, shared server discovery path

### Internal References
- Existing DoltReader pattern: `internal/datasource/dolt.go:14-47`
- DSN construction: `internal/datasource/metadata.go:22-26`
- Port discovery chain: `internal/datasource/metadata.go:70-101`
- Poll loop: `pkg/ui/background_worker.go:1956-2076`
- LoadFromSource dispatch: `internal/datasource/load.go:154-178`
- SourceType enum: `internal/datasource/source.go:27-37`
- Issue.SourceRepo field: `pkg/model/types.go:35`
- Workspace mode activation: `pkg/ui/model_modes.go:37-52`
- EnableWorkspaceMode reuse: `cmd/bt/main.go:1427-1435`
- Data layer audit: `docs/audit/team-5-data.md`

### External References
- go-sql-driver/mysql DSN format: `user@tcp(host:port)/dbname?params`
- Dolt SHOW DATABASES: standard MySQL syntax, returns all databases on the server
- Beads shared server: `bd init --shared-server` (v0.39.0+, confirmed working v1.0.0)
