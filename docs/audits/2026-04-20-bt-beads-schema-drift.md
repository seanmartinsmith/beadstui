# bt vs beads schema drift audit

- **Date**: 2026-04-20
- **Bead**: bt-uc6k
- **Purpose**: identify every column/field upstream beads stores on issue-family
  tables that bt's Dolt readers cannot currently read. Scope input for bt-mhwy.0.
- **Sources**:
  - upstream: `~/System/tools/beads/internal/storage/schema/migrations/` (0001–0032)
  - bt: `internal/datasource/columns.go`, `internal/datasource/dolt.go`,
    `internal/datasource/global_dolt.go`, `pkg/model/types.go`

## Summary

| Table | Upstream columns | bt reads | Gap |
|---|---:|---:|---:|
| `issues` | 53 | 28 | **25 missing** |
| `dependencies` | 7 | 2 | 4 missing |
| `labels` | 2 | 1 (+ implicit `issue_id`) | 0 |
| `comments` | 5 | 4 (+ implicit `issue_id`) | 0 |

Two known drift points (`metadata`, `closed_by_session`) are real but undercount
the gap. **bt is 25 columns behind on `issues`**, most added before bt forked
from beads_viewer. Labels and comments are clean; dependencies is a small
enrichment opportunity, not a correctness issue.

## `issues` table — full column diff

Upstream schema = migration 0001 base + additive migrations 0023 (`no_history`)
and 0027 (`started_at`). bt's column set comes from the `IssuesColumns` constant.

Legend:  ✅ read by bt  ·  ❌ not read by bt  ·  ⚠ read via bt-specific path

| # | Column | Type | bt? | Group | Notes |
|---|---|---|---|---|---|
| 1 | `id` | VARCHAR(255) | ✅ | core | |
| 2 | `content_hash` | VARCHAR(64) | ❌ | core | Compaction / change detection |
| 3 | `title` | VARCHAR(500) | ✅ | core | |
| 4 | `description` | TEXT | ✅ | core | |
| 5 | `design` | TEXT | ✅ | core | |
| 6 | `acceptance_criteria` | TEXT | ✅ | core | |
| 7 | `notes` | TEXT | ✅ | core | |
| 8 | `status` | VARCHAR(32) | ✅ | core | |
| 9 | `priority` | INT | ✅ | core | |
| 10 | `issue_type` | VARCHAR(32) | ✅ | core | |
| 11 | `assignee` | VARCHAR(255) | ✅ | core | |
| 12 | `estimated_minutes` | INT | ✅ | core | |
| 13 | `created_at` | DATETIME | ✅ | core | |
| 14 | `created_by` | VARCHAR(255) | ❌ | provenance | Who created the issue |
| 15 | `owner` | VARCHAR(255) | ❌ | provenance | Distinct from `assignee` |
| 16 | `updated_at` | DATETIME | ✅ | core | |
| 17 | `closed_at` | DATETIME | ✅ | core | |
| 18 | `closed_by_session` | VARCHAR(255) | ❌ | **session** | **Required for bt-mhwy.1 first-class `closed_by_session` surfacing** |
| 19 | `external_ref` | VARCHAR(255) | ✅ | core | |
| 20 | `spec_id` | VARCHAR(1024) | ❌ | core | Link to spec document |
| 21 | `compaction_level` | INT | ✅ | compaction | |
| 22 | `compacted_at` | DATETIME | ✅ | compaction | |
| 23 | `compacted_at_commit` | VARCHAR(64) | ✅ | compaction | |
| 24 | `original_size` | INT | ✅ | compaction | |
| 25 | `sender` | VARCHAR(255) | ❌ | wisp | Message sender |
| 26 | `ephemeral` | TINYINT(1) | ✅ | wisp | |
| 27 | `wisp_type` | VARCHAR(32) | ❌ | wisp | |
| 28 | `pinned` | TINYINT(1) | ❌ | core | Pin flag (separate from `StatusPinned`) |
| 29 | `is_template` | TINYINT(1) | ✅ | wisp | |
| 30 | `mol_type` | VARCHAR(32) | ✅ | molecule | |
| 31 | `work_type` | VARCHAR(32) | ❌ | molecule | `mutex` / … |
| 32 | `source_system` | VARCHAR(255) | ❌ | federation | |
| 33 | `metadata` | JSON | ❌ | **session** | **Required for bt-mhwy.1 session-ID bridge (`metadata.created_by_session` / `.claimed_by_session`)** |
| 34 | `source_repo` | VARCHAR(512) | ✅ | core | |
| 35 | `close_reason` | TEXT | ✅ | core | |
| 36 | `event_kind` | VARCHAR(32) | ❌ | event | Event-wisp marker |
| 37 | `actor` | VARCHAR(255) | ❌ | event | |
| 38 | `target` | VARCHAR(255) | ❌ | event | |
| 39 | `payload` | TEXT | ❌ | event | |
| 40 | `await_type` | VARCHAR(32) | ✅ | gate | |
| 41 | `await_id` | VARCHAR(255) | ✅ | gate | |
| 42 | `timeout_ns` | BIGINT | ✅ | gate | |
| 43 | `waiters` | TEXT | ❌ | gate | Gate-waiter list |
| 44 | `hook_bead` | VARCHAR(255) | ❌ | agent | GUPP hook bead |
| 45 | `role_bead` | VARCHAR(255) | ❌ | agent | GUPP role bead |
| 46 | `agent_state` | VARCHAR(32) | ❌ | agent | |
| 47 | `last_activity` | DATETIME | ❌ | agent | |
| 48 | `role_type` | VARCHAR(32) | ❌ | agent | |
| 49 | `rig` | VARCHAR(255) | ❌ | agent | |
| 50 | `due_at` | DATETIME | ✅ | schedule | Maps to `Issue.DueDate` |
| 51 | `defer_until` | DATETIME | ❌ | schedule | Read-until gate |
| 52 | `no_history` | TINYINT(1) | ❌ | compaction | (0023) |
| 53 | `started_at` | DATETIME | ❌ | schedule | (0027) |

### Missing columns by group (25 total)

**Session (2)** — **blocks bt-mhwy.1**:
- `metadata` (JSON)
- `closed_by_session` (VARCHAR)

**Provenance (2)**:
- `created_by`
- `owner`

**Core flags (3)**:
- `content_hash`
- `spec_id`
- `pinned`

**Schedule (2)**:
- `defer_until`
- `started_at`

**Compaction (1)**:
- `no_history`

**Wisp / federation (4)**:
- `sender`, `wisp_type`, `work_type`, `source_system`

**Event wisp (4)**:
- `event_kind`, `actor`, `target`, `payload`

**Gate (1)**:
- `waiters`

**Agent / GUPP (6)**:
- `hook_bead`, `role_bead`, `agent_state`, `last_activity`, `role_type`, `rig`

## `dependencies` table

Upstream schema (migration 0002):

| Column | Type | bt SELECT? |
|---|---|---|
| `issue_id` | VARCHAR(255) | implicit WHERE |
| `depends_on_id` | VARCHAR(255) | ✅ |
| `type` | VARCHAR(32) | ✅ |
| `created_at` | DATETIME | ❌ |
| `created_by` | VARCHAR(255) | ❌ |
| `metadata` | JSON | ❌ |
| `thread_id` | VARCHAR(255) | ❌ |

bt's query: `SELECT depends_on_id, type FROM dependencies WHERE issue_id = ?`

bt's `model.Dependency` struct already has `CreatedAt` and `CreatedBy` fields —
they're populated by other sources (JSONL ingest) but Dolt path leaves them
zero-valued. This is presentation drift, not correctness: deps work, but
"when was this edge added" is unavailable in Dolt mode.

## `labels` table

Upstream schema (migration 0003):

| Column | Type | bt SELECT? |
|---|---|---|
| `issue_id` | VARCHAR(255) | implicit WHERE |
| `label` | VARCHAR(255) | ✅ |

**No drift.**

## `comments` table

Upstream schema (migration 0004):

| Column | Type | bt SELECT? |
|---|---|---|
| `id` | CHAR(36) | ✅ |
| `issue_id` | VARCHAR(255) | implicit WHERE |
| `author` | VARCHAR(255) | ✅ |
| `text` | TEXT | ✅ |
| `created_at` | DATETIME | ✅ |

**No drift.**

## Rename / retype findings

- `issues.due_at` (upstream) vs `Issue.DueDate` (bt Go field, JSON `due_date`) —
  **naming convention split**, not a DB/Dolt drift. bt maps the column name
  `due_at` to the Go field `DueDate`. No change needed; noted for clarity.
- `comments.id` — upstream is `CHAR(36)` UUID. bt already migrated to
  `string` ID (bt-ju7o). No further action.
- No SQLite-path column type mismatches in scope of this audit (SQLite path
  uses `dependency_type` column name instead of `type` — long-standing, pre-dates
  Dolt, not relevant to this catchup).

## Recommended scope for bt-mhwy.0

Land catchup in three tiers so the PR is reviewable and can stop at any tier
without regressing.

### Tier 1 — required for bt-mhwy.1 (non-negotiable)

| Column | Action |
|---|---|
| `metadata` | Add to `IssuesColumns`; scan into `sql.NullString` then `json.Unmarshal` into new `Issue.Metadata map[string]json.RawMessage`. |
| `closed_by_session` | Add to `IssuesColumns`; scan into `sql.NullString` → `Issue.ClosedBySession string`. |

Both must support the defensive-scan pattern so bt against an older Dolt schema
doesn't crash. Simplest: read `information_schema.columns` once at reader
construction and build the column list dynamically, or detect missing-column
errors and fall back.

### Tier 2 — cheap provenance / schedule wins (include if low-risk)

`created_by`, `owner`, `spec_id`, `defer_until`, `started_at`, `no_history`,
`pinned`, `content_hash`.

Straight string/int/timestamp scans. No JSON parsing. Adds `Issue` struct
fields and `Clone()` entries but no new failure modes. Good candidates to
bundle with Tier 1 since they share the same defensive-scan pattern.

### Tier 3 — defer to separate beads (out of scope)

Agent / GUPP / event-wisp / gate-waiter columns (16 columns) target features
bt does not render today. Adding them without a UI surface is dead weight.
File a follow-up task ("bt reads orchestration columns") if/when bt starts
surfacing Gastown state.

### Dependencies enrichment (optional for bt-mhwy.0)

Extend bt's `SELECT depends_on_id, type FROM dependencies` to `SELECT
depends_on_id, type, created_at, created_by FROM dependencies` so `Issue.
Dependencies[*].CreatedAt` / `.CreatedBy` populate in Dolt mode. Low risk.
Skip `metadata` and `thread_id` for now — no consumer in bt.

## Notes for bt-mhwy.0 author

- bt's Dolt readers live in **two** files that must stay in lockstep:
  `internal/datasource/dolt.go` (`LoadIssuesFiltered`, ~line 60) and
  `internal/datasource/global_dolt.go` (`scanIssue`, ~line 335). Any column
  added to `IssuesColumns` needs scan updates in both.
- SQLite path (`internal/datasource/sqlite.go`) is legacy and should not be
  extended with new fields — it's on its way out per upstream beads v0.56.1
  (SQLite removed from beads itself). Document any field as "Dolt-path-only".
- `pkg/model/types.go` `Issue.Clone()` must deep-copy any new pointer / map
  / slice field — this is a common omission source.
- Defensive scan: see existing pattern for optional fields, e.g.,
  `estimated_minutes` uses `sql.NullInt64`. For `metadata`, scan into
  `sql.NullString`, then `json.Unmarshal` only if `.Valid`.
