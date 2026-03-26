---
date: 2026-03-26
topic: bd-bt-compatibility-surface
source: beads repo session, analyzing 44-commit pull (c329f0da..5ca6bb88)
---

# bd <> bt Compatibility Surface

## Purpose

bt reads beads' Dolt storage directly and shells out to `bd` for writes. This
doc maps the exact contract between the two tools so that upstream beads changes
can be evaluated for bt impact quickly.

## Current Schema bt Depends On

### issues table (bt reads 22 of ~50+ columns)

bt's Dolt reader (`internal/datasource/dolt.go:LoadIssuesFiltered`) selects:

```sql
SELECT
    id, title, description, status, priority, issue_type,
    assignee, estimated_minutes, created_at, updated_at,
    due_at, closed_at, external_ref, compaction_level,
    compacted_at, compacted_at_commit, original_size,
    design, acceptance_criteria, notes, source_repo,
    close_reason
FROM issues
WHERE status != 'tombstone'
```

bt's SQLite reader (`internal/datasource/sqlite.go`) selects a slightly
different set (uses `due_date` instead of `due_at`, has `labels` as JSON column,
filters on `tombstone` column instead of status).

Fallback query (both readers): `id, title, description, status, priority,
issue_type, created_at, updated_at` only.

### labels table

```sql
SELECT label FROM labels WHERE issue_id = ?
```

### dependencies table

Dolt reader:
```sql
SELECT depends_on_id, type FROM dependencies WHERE issue_id = ?
```

SQLite reader (uses different column name):
```sql
SELECT depends_on_id, dependency_type FROM dependencies WHERE issue_id = ?
```

### comments table

```sql
SELECT id, author, text, created_at FROM comments WHERE issue_id = ? ORDER BY created_at
```

### Aggregate queries

```sql
SELECT COUNT(*) FROM issues WHERE status != 'tombstone'
SELECT MAX(updated_at) FROM issues
```

## Columns bt Does NOT Read (Potential Features)

These exist in beads schema but bt ignores them. Grouped by relevance:

### High relevance for bt features

| Column | Why bt should care |
|---|---|
| `defer_until` | Deferred beads should show differently in board/list view |
| `created_by` / `owner` | Attribution display, agent tracking |
| `pinned` | Pinned beads should be visually distinct, sticky in views |
| `ephemeral` | Ephemeral beads could be dimmed or filtered |

### Medium relevance (Gas Town / orchestration)

| Column | Why |
|---|---|
| `agent_state`, `rig`, `role_bead`, `hook_bead` | Agent visibility in multi-agent views |
| `event_kind`, `actor`, `target`, `payload` | Event stream / activity feed |
| `await_type`, `await_id`, `timeout_ns`, `waiters` | Blocking visualization |
| `work_type`, `mol_type`, `wisp_type` | Filtering by work category |

### Low relevance (internal / system)

| Column | Why |
|---|---|
| `content_hash`, `spec_id`, `sender` | Internal bookkeeping |
| `closed_by_session` | Audit trail |
| `is_template` | Template management |
| `metadata` (JSON) | Extensibility bucket |
| `no_history` (new in 0023) | History opt-out flag |

### New tables bt doesn't read

| Table | Relevance |
|---|---|
| `wisps` | Low for now (agent messaging layer) |
| `federation_peers` | Low (multi-remote sync) |
| `interactions` | Medium (activity tracking for insights) |
| `routes` | Low (message routing) |
| `issue_snapshots` | High (could power history/burndown) |
| `compaction_snapshots` | Low (internal) |
| `repo_mtimes` | Low (sync bookkeeping) |
| `child_counters` | Low (ID generation) |
| `config` | Medium (bt could read bd config for display) |

## Known Bugs / Drift

### Bug: Dolt reader missing due_at

bt's Dolt reader doesn't SELECT `due_at`, so due dates are silently lost when
reading from Dolt. The SQLite reader does read `due_date`. Fix: add `due_at` to
the Dolt SELECT and map it to `issue.DueDate`.

### Drift: dependency type column name

Dolt schema uses `type`. SQLite schema uses `dependency_type`. bt handles this
with separate queries per reader, but it's fragile - if beads changes the Dolt
column name, bt silently loses dependency type info (query would fail, deps
loaded without type).

### Drift: tombstone filtering

Dolt reader filters `WHERE status != 'tombstone'`. SQLite reader filters
`WHERE (tombstone IS NULL OR tombstone = 0)`. These are different mechanisms -
the SQLite path assumes a boolean `tombstone` column that doesn't exist in Dolt.
Not a current bug (separate code paths), but a maintenance smell.

## bd CLI Commands bt Should Shell Out To (Write Ops)

For Phase 4 (interactive editing), bt needs to invoke:

| Operation | Command | Priority |
|---|---|---|
| Create issue | `bd create --title="..." --type=task --priority=2` | P1 |
| Update status | `bd update <id> --status=in_progress` | P1 |
| Close issue | `bd close <id>` | P1 |
| Claim issue | `bd update <id> --claim` | P1 |
| Edit title | `bd update <id> --title="..."` | P2 |
| Edit description | `bd update <id> --description="..."` | P2 |
| Add comment | `bd comment <id> "text"` | P2 |
| Add/remove label | `bd label add <id> <label>` / `bd label rm <id> <label>` | P2 |
| Add dependency | `bd dep add <issue> <depends-on>` | P2 |
| Remove dependency | `bd dep rm <issue> <depends-on>` | P2 |
| Defer issue | `bd defer <id> --until="date"` | P3 |
| Change priority | `bd update <id> --priority=1` | P3 |
| Change assignee | `bd update <id> --assignee=name` | P3 |

bt already shells out to `bd dolt start` / `bd dolt stop` via `internal/doltctl/`.
The pattern is proven - extend it for write operations.

### bd output parsing

bt should use `--format json` where available for machine-readable output. Need
to audit which bd commands support `--format json` - this may be inconsistent.
Community PR #2638 (DreadPirateRobertz) added `--format json` alias work.

## What Changed in the 44-Commit Pull (2026-03-26)

### Breaking potential: NONE (bt is safe)

bt's queries don't reference any removed columns. The HOP columns
(`crystallizes`, `quality_score`) that were removed from embedded schema were
never in bt's SELECT lists.

### Architectural changes to watch

1. **New `issueops/` package**: Blocked detection, bulk operations, compaction,
   federation, statistics extracted from embeddeddolt into standalone package.
   If bt ever imports beads as a Go module, this is the new interface.

2. **New `versioncontrolops/` package**: Backup, restore, flatten, branches,
   commit, GC extracted. Same note about future module import.

3. **`RunInTransaction` refactor**: Internal to beads storage - no bt impact
   since bt only reads via SQL.

4. **Embedded test infrastructure**: coffeegoddd built out massive test suites
   for nearly every command. Good signal that the CLI API is stabilizing.

5. **`resolveCommandBeadsDir` fix (#2775)**: No longer falls back to CWD-based
   discovery. If bt shells out to `bd` commands, they need to be run from a
   directory where `.beads/` is discoverable (project root). bt's doltctl
   already handles this via `BeadsDir` field.

6. **Circuit breaker files moved to subdirectory (#2799)**: Was scanning `/tmp`
   previously. No bt impact but good to know the `.beads/` internal structure
   is evolving.

## Recommendations

### Immediate (before next bt feature work)

1. **Fix the due_at bug** in Dolt reader - add `due_at` to SELECT list
2. **Add `defer_until`** to Dolt reader - needed for deferred status display
3. **Add `pinned`** to Dolt reader - needed for pinned bead behavior

### Before Phase 4 (interactive editing)

4. **Audit bd --format json support** across all write commands
5. **Design the DoltWriter abstraction** in bt - mirror of DoltReader but for
   bd CLI shell-outs
6. **Add `created_by` / `owner`** to reader for attribution in create/edit flows

### Future (Gas Town integration)

7. **Add agent columns** when multi-agent views are built
8. **Read `issue_snapshots`** for history/burndown features
9. **Read `config` table** to surface bd configuration in bt UI
