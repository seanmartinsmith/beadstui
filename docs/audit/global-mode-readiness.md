# Global Mode Readiness Audit

**Date:** 2026-04-08
**Status:** Issue loading works, but dependency/label/comment loading is BROKEN (bt-ktig)
**Related beads:** bt-ssk7 (global federation), bt-ll9v (migration tracker), sms-ncb (design problem), bt-ktig (dep loading bug)

## Key Finding

Global mode (`bt --global`) loads issues correctly across all 13 databases (2168 nodes). However, **dependency loading is silently broken** - loadAllDependencies runs without error but attaches nothing. This means graph analysis, blocking calculations, and triage recommendations all run without dependency data in global mode. Labels and comments may have the same issue (untested). See bt-ktig.

The implementation was discovered during this audit - prior sessions assumed it was unbuilt. Subsequent testing revealed the dependency loading bug.

## Architecture Overview

```
bt --global
  ├── DiscoverSharedServer()          → finds port via env var or port file
  ├── NewGlobalDataSource(host, port) → creates SourceTypeDoltGlobal source
  ├── LoadFromSource(globalSource)    → dispatches to GlobalDoltReader
  │   ├── EnumerateDatabases()        → queries information_schema for issues tables
  │   ├── buildIssuesQuery()          → UNION ALL across all db.issues
  │   ├── loadAllLabels()             → UNION ALL across all db.labels
  │   ├── loadAllDependencies()       → UNION ALL across all db.dependencies
  │   └── loadAllComments()           → UNION ALL across all db.comments
  ├── Extract repo prefixes from SourceRepo field
  └── EnableWorkspaceMode()           → activates repo picker, badges, filtering
```

## Component Status

| Component | Status | Location | Notes |
|-----------|--------|----------|-------|
| `--global` flag | Done | cmd/bt/main.go:118 | Bool flag, no short alias yet |
| `--repo` filter | Done | cmd/bt/main.go:119 | Case-insensitive db name filter |
| Mutual exclusivity | Done | cmd/bt/main.go:220-228 | --global vs --workspace, --global vs --as-of |
| Server discovery | Done | internal/datasource/global_dolt.go:38-65 | BT_GLOBAL_DOLT_PORT > port file |
| Database enumeration | Done | internal/datasource/global_dolt.go:115-155 | information_schema query, filters system dbs |
| UNION ALL queries | Done | internal/datasource/global_dolt.go:169-315 | Issues, labels, deps, comments, last modified |
| GlobalDoltReader | Done | internal/datasource/global_dolt.go | Full reader with LoadIssues, GetLastModified |
| Background polling | Done | pkg/ui/background_worker.go:2086-2115 | globalDoltPollOnce(), 3s interval |
| Workspace mode UI | Done | pkg/ui/model_modes.go:37-52 | EnableWorkspaceMode sets up repo picker |
| Repo picker (w key) | Done | pkg/ui/repo_picker.go | Modal with j/k/space/enter/esc |
| Footer badges | Done | pkg/ui/model_footer.go:514-518 | Shows repo count and active filter |
| SourceRepo population | Done | internal/datasource/global_dolt.go | Set to database name per issue |
| Exponential backoff | Done | pkg/ui/background_worker.go | Backoff on poll failures, reconnect after 3 |

## Data Loading Path (main.go:530-565)

Three branches in main.go for data loading:
1. **--as-of** (lines 475-502): Historical/time-travel via git
2. **--workspace** (lines 503-529): Multi-repo JSONL via workspace.yaml
3. **--global** (lines 530-565): Multi-database Dolt via shared server

Global mode:
1. Discovers shared server (env var > port file)
2. Creates GlobalDataSource with optional repo filter
3. Loads all data via UNION ALL queries
4. Extracts repo prefixes from loaded issues
5. Builds workspace LoadSummary for UI integration
6. Enables workspace mode (repo picker, badges, filtering)

## Polling System

### Single-repo mode
- `doltPollOnce()` at background_worker.go:2057-2081
- Queries `MAX(updated_at)` from one database
- Compares against stored timestamp
- Triggers refresh if changed

### Global mode
- `globalDoltPollOnce()` at background_worker.go:2086-2115
- Queries `MAX(updated_at)` across ALL databases via UNION ALL subqueries
- Same comparison/refresh logic
- Same 3-second interval
- Same exponential backoff on failure

Both use the same infrastructure - dispatch is based on `dataSource.Type`.

## Workspace Mode Requirements

Workspace mode (repo picker, badges, filtering) requires:
- `model.workspaceMode = true`
- `model.availableRepos` populated with repo prefix strings
- `model.activeRepos` initialized (nil = all repos visible)

Without these, pressing `w` shows: "Repo filter available only in workspace mode"

Global mode sets all three automatically via `EnableWorkspaceMode()`.

## Workspace Filtering

`workspacePrefilter()` at model_filter.go:898-910:
- Extracts repo prefix from issue ID
- Checks against `activeRepos` map
- `activeRepos == nil` means show all
- Applied BEFORE recipe/status/BQL filters

## Key Files

- **cmd/bt/main.go** - Flag definitions, loading branches, UI setup
- **internal/datasource/global_dolt.go** - GlobalDoltReader, UNION ALL queries, database enumeration
- **internal/datasource/dolt.go** - Single-database DoltReader (reference)
- **pkg/ui/background_worker.go** - Poll loops (single + global)
- **pkg/ui/model.go:472-475** - Workspace state fields
- **pkg/ui/model_modes.go:37-52** - EnableWorkspaceMode()
- **pkg/ui/repo_picker.go** - Repo picker modal
- **pkg/ui/workspace_repos.go** - Prefix normalization, formatting helpers
- **pkg/ui/model_filter.go:898-910** - workspacePrefilter()
- **pkg/ui/model_footer.go:514-518** - Workspace badge rendering
- **pkg/workspace/** - JSONL-only workspace loader (NOT used by global mode)

## Design Decision: Auto-Global (2026-04-08)

**Decision:** Global mode should be the default when shared server exists, not opt-in via `--global`.

**Proposed behavior:**
1. bt starts, checks for `~/.beads/shared-server/dolt-server.port`
2. If exists: connect to shared server, load all databases, but auto-filter to current repo's database (from `.beads/metadata.json` in cwd). Workspace mode active, `w` key available to widen scope.
3. If no shared server: current behavior (single-repo, per-project Dolt or JSONL)
4. `--global` flag becomes "start with all repos visible" instead of "enable global mode"

**Rationale:** Don't make users opt into the more capable mode. The default experience is identical (you see your repo's beads), but the global infrastructure is always available.

**Status:** Not yet implemented. Requires changes to main.go loading logic.

## Known Gaps / Future Work

1. **No `-g` short alias** - Only `--global`, no shorthand (one-line fix in pflag)
2. **Auto-global not implemented** - Still requires explicit `--global` flag
3. **Workspace loader is JSONL-only** - pkg/workspace/loader.go can't load Dolt sources. Not used by global mode (which has its own loader), but limits --workspace flag to JSONL repos.
4. **No cross-repo dependency validation** - Dependencies can reference issues in other databases but target existence isn't verified
5. **No database enumeration cache** - EnumerateDatabases() queries information_schema on every load
6. **No prefix collision handling** - If two databases have same issue ID, they'd collide (unlikely with beads prefixes but possible)
7. **No display name mapping** - Database names used as-is (decided: this is fine for now)
8. **TUI read/write not implemented** - bt-oiaj tracks this separately
