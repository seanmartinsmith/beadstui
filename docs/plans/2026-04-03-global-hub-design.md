# Global Hub: Live Cross-Project Visibility via Shared Dolt Server

## Goal

A single bt instance that shows all issues from all projects, live, with the ability to scope down to a single project. No sync commands, no staleness, no JSONL.

```
cd ~/.obs && bt          → see everything across all projects
cd ~/System/tools/tpane && bt  → see only tpane issues
```

## Architecture: Multi-Database Federation (Option B)

All projects use beads' shared Dolt server mode. Each project gets its own database on the server. bt queries across all databases using SQL `UNION ALL` with `database.table` syntax.

```
Shared Dolt Server (localhost:3308, ~/.beads/shared-server/)
├── tpn   (tpane's database)
├── mkt   (marketplace's database)
├── bt    (beadstui's database)
├── cass  (cass's database)
└── ...   (any project with bd init --shared-server)
```

**No beads code changes required.** This uses existing beads infrastructure:
- `bd init --shared-server` (ships in v0.39.0+, confirmed working v1.0.0)
- Per-project databases with per-project prefixes
- Shared Dolt server at `~/.beads/shared-server/`

All the work is in bt.

## Empirical Verification (2026-04-03)

Tested on beads v1.0.0 with two temp repos on the same shared server:

| Test | Result |
|------|--------|
| Two repos, separate databases, one server | Works - no prefix clobbering |
| Correct prefix-scoped ID generation | `aaa-cni`, `bbb-3z7` - isolated |
| Cross-database `UNION ALL` query | Returns all issues from all databases in one query |
| Live updates (create in A, query from B) | Instant - zero delay |
| Status changes across databases | Close in A, immediately reflected in cross-DB query |
| Server discovery | Port at `~/.beads/shared-server/dolt-server.port` |
| Database enumeration | `SHOW DATABASES` lists all project databases |

### The query that powers the global view

```sql
SELECT id, title, status, priority, issue_type, created_at, updated_at
FROM tpn.issues
UNION ALL
SELECT id, title, status, priority, issue_type, created_at, updated_at
FROM mkt.issues
UNION ALL
SELECT id, title, status, priority, issue_type, created_at, updated_at
FROM cass.issues
ORDER BY updated_at DESC
```

This is what bt's global mode executes. Built dynamically from `SHOW DATABASES`.

## What bt Needs to Implement

### 1. Shared server connection mode

bt currently connects to a single project's Dolt database. It needs a second mode: connect to the shared server directly and query across databases.

**Discovery:**
- Read `~/.beads/shared-server/dolt-server.port` for the port
- Connect to `root@127.0.0.1:<port>` (same as beads does)
- Run `SHOW DATABASES` to enumerate project databases
- Filter out system databases (`information_schema`, `mysql`, `dolt`)

**Configuration (TBD - bt session should decide):**
- Detect shared server automatically when `~/.beads/shared-server/dolt-server.port` exists?
- Or require explicit `bt --global` flag?
- Or use a `.bt/hub.yaml` config that lists which databases to include?

### 2. Global mode vs local mode

**Local mode** (existing behavior): `cd tpane && bt`
- Reads tpane's `.beads/metadata.json` to find database name and server
- Queries only that database
- Shows only tpane issues
- This is the current behavior - no changes needed

**Global mode** (new): `cd ~/.obs && bt` (or `bt --global` from anywhere)
- Connects to shared server
- Enumerates all project databases
- Builds `UNION ALL` query across all `*.issues` tables
- Shows everything
- Filterable by project/prefix in the TUI (repo picker, like workspace mode's `w` key)

### 3. Poll-based refresh for global mode

bt already has Dolt poll-based refresh for single-repo mode. The global mode needs the same:
- Poll the `UNION ALL` query on a timer
- Detect new/changed/closed issues
- Update the TUI live

The existing poll infrastructure should work - it's just a different query.

### 4. Project grouping and filtering in TUI

Once all issues are loaded, the TUI needs:
- Project badge per issue (like workspace mode's colored `[BT]`/`[CASS]` badges)
- Filter by project (repo picker via `w` key, already exists in workspace mode)
- Sort by project, priority, status, updated_at
- `bd human` integration across all projects

The project name can be derived from the database name (which matches the prefix).

### 5. Relational data

The global query needs more than just the `issues` table. For each database, bt may also need:
- `labels` - for label-based filtering
- `dependencies` - for blocker visibility
- `comments` - for comment counts

These follow the same `database.table` pattern: `tpn.labels`, `tpn.dependencies`, etc.

## What Beads Needs (Separate Track)

These are upstream contributions, independent of the global hub work:

1. **`bd repo sync` Dolt-native fix** - Plan complete at `beads/docs/superpowers/plans/2026-04-03-dolt-native-repo-sync.md`. Fixes sync to read from Dolt instead of JSONL. Useful for people who don't use shared server mode. Separate PR.

2. **Prefix-scoped query issue** - In shared-database configurations (not our path, but still a valid use case for others), `bd list` should auto-filter by configured prefix. Separate issue.

Neither of these blocks the global hub work. bt's federation approach works with beads as-is.

## Configuration Recipe

To onboard an existing project to the shared server:

```bash
cd ~/System/tools/tpane
bd init --shared-server --prefix tpn
```

For projects already initialized with embedded mode, they'll need re-initialization or migration to server mode. This is a one-time setup per project. TBD: document the migration path (may need `bd export` + re-init + `bd import`).

### Metadata gotcha: pre-server-mode repos

Repos initialized before server mode was added have `metadata.json` without `dolt_mode`, `dolt_server_host`, or `dolt_server_port`. Even if a Dolt server is running (PID/port files exist), `bd` falls back to embedded mode and fails. This was discovered on bt's own repo - the server was running on port 14710 but `bd list` returned nothing and `bd doctor --fix` said "not yet supported in embedded mode."

Fix is manual: add the server fields to `.beads/metadata.json`:
```json
{
  "dolt_mode": "server",
  "dolt_server_host": "127.0.0.1",
  "dolt_server_port": 3308
}
```

When migrating repos to the shared server, the onboarding script/docs must check for this and fix it. `bd init --shared-server` on a fresh repo handles it automatically, but existing repos need the metadata patched.

## Open Questions for bt Session

1. **How does global mode get triggered?** Auto-detect from `~/.beads/shared-server/` existence? Explicit `--global` flag? Config in `.obs/.bt/`? What feels right for the UX?

2. ~~**Can bt's existing Dolt connection code handle cross-database queries?**~~ **ANSWERED.** The SQL driver (`go-sql-driver/mysql`) fully supports `database.table` syntax and `USE database` - it's standard MySQL wire protocol. The blockers are in bt's code, not the driver:
   - DSN locks to one database: `metadata.go:22-26` bakes database name into `root@tcp(host:port)/database`
   - All queries use unqualified table names: `FROM issues` not `FROM db.issues` (dolt.go:65-76)
   - Validation uses `TABLE_SCHEMA = DATABASE()` (validate.go:98) assuming single-DB context
   - **Fix**: Connect without a database in DSN (`root@tcp(host:port)/`), use `database.table` qualified names. Straightforward.

3. **Database-to-project-name mapping.** Database names are `tpn`, `mkt`, etc. (derived from prefix). Should bt show these as-is, or maintain a display name mapping (e.g., `tpn` → "tpane", `mkt` → "marketplace")? Where does that mapping live?

4. ~~**What happens to workspace mode?**~~ **ANSWERED.** Workspace mode is NOT a dead end - it has strong reusable components (badge rendering in visuals.go:145-178, repo picker in repo_picker.go, `workspacePrefilter()` in model_filter.go:898-912, model state in model.go:472-476). However, workspace's loading layer is **JSONL-only** (workspace/loader.go:162) and **disables live reload** (main.go:512). Global mode reuses the UI components but needs its own Dolt-native loading path. Keep workspace mode as fallback for non-shared-server setups.

5. ~~**Refresh rate for global mode.**~~ **PARTIALLY ANSWERED.** The poll system is more constrained than the design doc assumed (see Corrections below). The current poll loop (`background_worker.go:1956-2047`) is deeply single-source: one `dataSource`, one `DoltReader` per poll, one `lastModified` timestamp. Three options for global mode:
   - **(A) Per-database poll loops** - most granular, most complex
   - **(B) Single aggregated poll** - `SELECT MAX(m) FROM (SELECT MAX(updated_at) FROM db1.issues UNION ALL ...) t` - simplest, one comparison
   - **(C) Per-database MAX in one query** - middle ground, detect which DB changed
   - At 9 databases, option B adds ~9 trivial subqueries to one poll. Dolt should handle this in <100ms. Default 5s interval is likely fine.

6. **Project categorization.** The user wants repos grouped by type (tools, projects, areas). Where does this metadata live? bt config? Labels in beads? Folder structure inference?

7. **Global mode entry point in main.go.** (NEW - verified) Clean insertion point exists. main.go has three data-loading branches (lines 463-561): `--as-of` / `--workspace` / default single-repo. Global mode slots between workspace and single-repo. Flag definition at ~line 117 next to `--workspace`. UI activation can reuse `EnableWorkspaceMode()` or extend it with a `GlobalMode` variant that also configures the Dolt poll loop.

8. **Issue.SourceRepo field.** (NEW - discovered) The Issue model already has a `SourceRepo string` field (model/types.go:35). Currently underutilized. Global mode should populate this with the database name so filtering, badges, and grouping work out of the box.

## Corrections to Design Doc Assumptions

These claims in the design doc need revision:

1. **"The existing poll infrastructure should work - it's just a different query"** (Section 3) - **Wrong.** The poll system is tightly coupled to single-database operation: one `dataSource` per BackgroundWorker (line 172), one `lastModified` local variable (line 1964), one `NewDoltReader(dataSource)` per poll (line 2053). Adapting it requires refactoring the poll loop's state management, not just swapping a query.

2. **"repo picker via `w` key, already exists in workspace mode"** (Section 4) - **Correct but incomplete.** The repo picker exists and is reusable. What the doc doesn't mention: workspace mode is JSONL-only and disables live reload. Global mode can reuse the picker UI but NOT workspace's loading infrastructure.

3. **"No special handling needed" for back-propagation** (Non-Goals) - **Partially wrong.** `USE database` before a write would work at the SQL level, but bt currently has no write path at all - it shells out to `bd` for mutations. `bd` commands run against the project's own database. In global mode, bt would need to either (a) `cd` to the correct project dir before invoking `bd`, or (b) pass the database context to `bd` somehow. This IS special handling.

4. **"Cross-database dependencies won't be visible"** (Non-Goals) - **Correct for now**, but worth noting: if global mode builds a `UNION ALL` across `db1.dependencies` and `db2.dependencies`, cross-project blockers become queryable. The dependency graph renderer in bt already handles cross-issue refs. This could be a natural extension, not a hard limitation.

## Non-Goals

- **Back-propagation** - editing issues in global mode writes to the correct project database (which Dolt handles naturally via `USE database` before the write). No special handling needed.
- **Cross-project dependencies** - dependencies between `tpn-1` and `mkt-2` live in their respective databases and won't be visible cross-database. This is a future problem.
- **Remote/multi-machine** - shared server is localhost only. Remote access would need Dolt remotes or SSH tunneling. Out of scope.
