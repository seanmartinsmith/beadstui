# Beads Ecosystem Audit

**Date**: 2026-03-17
**bd version**: 0.61.0 (389f9795)
**Purpose**: Deep investigation of the beads ecosystem to inform bt's feature strategy.
**Rules**: Research only - no code changes.

---

## 1. Current Schema

### Issues Table (54 columns)

The issues table is the central data structure. Schema version 8. 577 total issues (564 closed, 12 open, 1 deferred).

**Core tracking fields** (what bt already reads - 22 columns):
- `id` (PK), `title`, `description`, `status`, `priority`, `issue_type`
- `assignee`, `estimated_minutes`, `created_at`, `created_by`, `owner`
- `updated_at`, `closed_at`, `closed_by_session`, `close_reason`
- `external_ref`, `spec_id`, `design`, `acceptance_criteria`, `notes`
- `source_repo`, `due_at`

**Compaction/lifecycle fields** (bt reads most of these):
- `content_hash` - SHA256 of content for dedup/change detection
- `compaction_level`, `compacted_at`, `compacted_at_commit`, `original_size`
- `ephemeral` (bool), `wisp_type` (heartbeat, ping, patrol, gc_report, etc.)
- `pinned` (bool), `is_template` (bool), `no_history` (bool)
- `defer_until` (datetime)

**Molecule/workflow fields** (bt does NOT read these):
- `mol_type` (swarm, patrol, work), `work_type` (default: mutex), `crystallizes` (bool)

**Agent/orchestration fields** (bt does NOT read these):
- `hook_bead`, `role_bead`, `agent_state` (idle/spawning/running/working/stuck/done/stopped/dead)
- `last_activity`, `role_type`, `rig`
- `sender` - for message-type issues

**Event/gate fields** (bt does NOT read these):
- `event_kind`, `actor`, `target`, `payload` - for event-type issues
- `await_type` (human, timer, gh:run, gh:pr, bead), `await_id`, `timeout_ns`, `waiters` - for gate issues

**Other fields** (bt does NOT read):
- `quality_score`, `source_system`, `metadata` (JSON)

### Gap Analysis: bt reads 22 of 54 columns

bt currently SELECTs these columns from Dolt:
```
id, title, description, status, priority, issue_type,
assignee, estimated_minutes, created_at, updated_at,
due_at, closed_at, external_ref, compaction_level,
compacted_at, compacted_at_commit, original_size,
design, acceptance_criteria, notes, source_repo,
close_reason
```

**Fields bt should consider adding to its model**:
- `owner`, `created_by` - attribution (shown in bd show output)
- `pinned` - pinned issues stay open indefinitely
- `ephemeral` + `wisp_type` - to filter/display wisps differently
- `defer_until` - deferred issues hidden from ready work
- `metadata` (JSON) - custom key-value pairs
- `content_hash` - for efficient change detection
- `is_template` - to filter out proto templates from regular views
- `rig` - multi-rig environments

**Fields bt probably does NOT need**:
- Agent fields (hook_bead, role_bead, agent_state, last_activity, role_type) - internal orchestration
- Event fields (event_kind, actor, target, payload) - event beads aren't user-facing issues
- Gate fields (await_type, await_id, timeout_ns, waiters) - gate beads aren't user-facing issues
- Molecule fields (mol_type, work_type, crystallizes) - workflow engine internals

### Other Tables (22 total)

| Table | Rows | Purpose | bt Should Use? |
|-------|------|---------|---------------|
| `issues` | 577 | Core issue data | Yes (already does) |
| `labels` | 668 | Issue labels (many-to-many) | Yes (already does) |
| `dependencies` | 808 | Issue relationships | Yes (already does) |
| `events` | 658 | Issue lifecycle events (created, closed, status_changed, etc.) | **Yes - for history view** |
| `comments` | 4 | Issue comments | Yes (already does) |
| `config` | 17 | Project config (issue_prefix, schema_version, sync.mode, etc.) | Maybe (prefix, custom types) |
| `metadata` | 6 | Project metadata (project_id, bd_version, etc.) | Maybe (version display) |
| `blocked_issues` | VIEW | Derived: issues with blocking deps | Could use for board view |
| `ready_issues` | VIEW | Derived: ready-to-work issues | Could use for board view |
| `wisps` | 0 | Ephemeral issues (dolt_ignored) | Low priority |
| `wisp_*` (comments, deps, events, labels) | 0 each | Wisp relations | Low priority |
| `federation_peers` | 0 | P2P federation config | Not yet |
| `interactions` | 0 | Agent audit trail | Not relevant |
| `routes` | 0 | Multi-rig routing | Future |
| `issue_counter` | 0 | ID generation | Not relevant |
| `child_counters` | 0 | Child ID generation | Not relevant |
| `issue_snapshots` | 0 | Compaction snapshots | Not relevant |
| `compaction_snapshots` | 0 | Compaction data | Not relevant |
| `repo_mtimes` | 0 | JSONL sync tracking | Not relevant |

### Dolt System Tables (key for history features)

These are virtual tables provided by the Dolt engine:

| System Table | Purpose | bt Use Case |
|-------------|---------|-------------|
| `dolt_log` | Commit history (140 commits) | History view - commit timeline |
| `dolt_diff_issues` | Per-commit field-level diffs | History view - what changed per issue |
| `dolt_history_issues` | Full issue state at each commit | Point-in-time issue state |
| `dolt_branches` | Branch list | Multi-branch support |
| `dolt_status` | Uncommitted changes | Status indicator |
| `dolt_remotes` | Remote config | Sync status |

**The `dolt_diff_issues` table is extremely powerful** - it provides from/to values for every column between commits, with commit hash and date. This is the foundation for a native issue history view that doesn't need the correlation engine's git-log-based approach.

---

## 2. Mutation API

### Issue CRUD

**`bd create`** - Full-featured issue creation:
- Core: `--title`, `--description`, `--type`, `--priority` (0-4), `--assignee`
- Rich text: `--design`, `--acceptance`, `--notes`, `--body-file`, `--stdin`
- Relationships: `--deps` (type:id format), `--parent`, `--waits-for`
- Lifecycle: `--ephemeral`, `--wisp-type`, `--no-history`, `--defer`, `--due`
- Identity: `--id` (explicit), `--prefix`, `--rig`
- Batch: `--file` (create multiple from markdown)
- Metadata: `--metadata` (JSON), `--labels`, `--external-ref`, `--spec-id`
- Output: `--silent` (ID only), `--dry-run`, `--json`

**`bd update`** - Update any field(s):
- All create fields except relationship ones
- Status changes: `--status`, `--claim` (atomic claim with assignee set)
- Labels: `--add-label`, `--remove-label`, `--set-labels`
- Metadata: `--set-metadata`, `--unset-metadata`
- Parent: `--parent` (reparent)
- Lifecycle: `--ephemeral`, `--persistent`, `--no-history`, `--history`
- Operates on last-touched issue if no ID given

**`bd close`** - Close with options:
- `--reason` - close reason text
- `--suggest-next` - show newly unblocked issues
- `--claim-next` - auto-claim next priority issue
- `--continue` - advance in molecule workflow
- `--force` - close pinned/unsatisfied gates
- Batch: `bd close id1 id2 id3`

**`bd reopen`** - Explicit reopen with `--reason`

**`bd delete`** - Destructive delete with dep cleanup:
- `--cascade` (recursive), `--force`, `--from-file`, `--dry-run`

### Dependency Management

**`bd dep add <blocked> <blocker>`** - Add dependency
**`bd dep remove`** - Remove dependency
**`bd dep list`** - List deps/dependents
**`bd dep tree`** - Dependency tree
**`bd dep cycles`** - Detect cycles
**`bd dep relate`** / `bd dep unrelate` - Bidirectional relates_to links

Dependency types in the database:
- `blocks` (697 in this project) - true blocking
- `parent-child` (110) - hierarchy
- `discovered-from` (1) - provenance

### Label Management

**`bd label add/remove`** - Per-issue
**`bd label list/list-all`** - Query
**`bd label propagate`** - Parent-to-children propagation

### Comments

**`bd comments <id>`** - List comments
**`bd comments add <id> "text"` / `--file`** - Add comment

### Other Write Commands

| Command | Purpose | bt Relevance |
|---------|---------|-------------|
| `bd edit` | Opens $EDITOR for field editing | Not usable from TUI (interactive) |
| `bd refile <id> <rig>` | Move issue to different rig | Possible future feature |
| `bd promote <wisp-id>` | Promote wisp to permanent bead | Low priority |
| `bd set-state <id> dim=val` | Set operational state label | Agent orchestration |
| `bd move <id> <rig>` | Move with dep remapping | Advanced |
| `bd supersede <id> --by=<new>` | Mark superseded | Possible from TUI |
| `bd duplicate <id> --of=<target>` | Mark as duplicate | Possible from TUI |
| `bd defer <id>` / `bd undefer <id>` | Defer/undefer | Useful from TUI |
| `bd rename <id>` | Rename issue ID | Rare |

---

## 3. Dolt System Tables & Capabilities

### Dolt Server Management

The Dolt server is managed by bd itself:
- `bd dolt start` / `bd dolt stop` / `bd dolt status`
- bt already handles lifecycle via `internal/doltctl/` (EnsureServer, StopIfOwned, PID-based ownership)
- Port is ephemeral (OS-assigned since beads v0.60)
- Server auto-starts transparently when bd commands need it

### Version Control

- `bd dolt commit` - Create Dolt commit
- `bd dolt push` / `bd dolt pull` - Sync with remote
- `bd vc commit` / `bd vc merge` / `bd vc status` - Git-like operations
- `bd branch` - List/create Dolt branches

### History Queries

`bd history <id>` provides per-issue version history. Internally, this queries `dolt_history_issues`.

`bd diff <ref1> <ref2>` shows changes between commits/branches using `dolt_diff_issues`.

Both require Dolt backend and support `--json` output.

### What bt Can Query Directly

Since bt has a MySQL connection to the Dolt server, it can query system tables directly:

**Issue history** (field-level diffs):
```sql
SELECT from_status, to_status, from_title, to_title, from_commit_date, to_commit_date
FROM dolt_diff_issues
WHERE to_id = 'bt-xavk'
ORDER BY to_commit_date DESC
```

**Commit log**:
```sql
SELECT commit_hash, message, committer, date
FROM dolt_log
ORDER BY date DESC
LIMIT 20
```

**Point-in-time state**:
```sql
SELECT * FROM dolt_history_issues
WHERE id = 'bt-xavk'
ORDER BY commit_date DESC
```

**Uncommitted changes**:
```sql
SELECT * FROM dolt_status
```

This is significantly more powerful than the correlation engine's git-log-based approach, which parses JSONL file diffs.

---

## 4. bd CLI Surface Area

### Complete Command Inventory (70+ commands)

**Issue CRUD** (12 commands):
- `create` (alias: `new`), `show` (alias: `view`), `list`, `update`, `close` (alias: `done`)
- `reopen`, `delete`, `edit`, `rename`, `q` (quick capture), `create-form` (interactive)
- `comments` (+ `add` subcommand)

**Search & Query** (7 commands):
- `search` - Text search with rich filters (status, priority, label, date ranges, assignee, etc.)
- `query` - SQL-like query language (`status=open AND priority>1`)
- `count` - Count with filters
- `ready` - Ready work (no blockers)
- `blocked` - Blocked issues
- `stale` - Not recently updated
- `find-duplicates` - Semantic similarity

**Dependencies & Structure** (8 commands):
- `dep` (add, remove, list, tree, cycles, relate, unrelate)
- `graph` - Visual dependency graph (DAG, box, compact, dot, html)
- `duplicate`, `duplicates` (find/merge)
- `supersede`, `children`
- `epic` (close-eligible, status)
- `swarm` (create, list, status, validate)

**Labels & State** (4 commands):
- `label` (add, remove, list, list-all, propagate)
- `set-state` - Operational state dimensions
- `promote` - Wisp to permanent
- `defer` / `undefer`

**Molecules & Formulas** (3 commands):
- `formula` (list, show, convert)
- `mol` (bond, burn, current, distill, last-activity, pour, progress, ready, seed, show, squash, stale, wisp)
- `cook` - Compile formula to proto

**Version Control & Sync** (5 commands):
- `dolt` (start, stop, status, show, set, test, commit, push, pull, remote, clean-databases, killall)
- `vc` (commit, merge, status)
- `branch` - List/create
- `diff` - Between commits/branches
- `history` - Per-issue version history

**Backup & Recovery** (3 commands):
- `backup` (export-git, fetch-git, init, restore, status, sync)
- `restore` - Restore compacted issue from history
- `compact` / `flatten` - Squash Dolt history

**Configuration & Setup** (9 commands):
- `config` (get, set, list, unset, validate)
- `init`, `bootstrap`, `setup`, `hooks`
- `context`, `info`, `where`

**Maintenance** (7 commands):
- `doctor`, `gc`, `migrate`, `preflight`
- `purge` - Delete closed ephemeral beads
- `rename-prefix` - Rename all issue IDs
- `sql` - Raw SQL access

**Agent & Orchestration** (5 commands):
- `agent` (state, heartbeat, show, backfill-labels)
- `gate` (list, check, resolve, show, add-waiter, discover)
- `merge-slot` - Serialized conflict resolution
- `slot` - Agent bead slots
- `audit` - Append-only JSONL interaction log

**Integrations** (7 commands):
- `github`, `gitlab`, `jira`, `linear` - External tracker sync
- `federation` (add-peer, list-peers, remove-peer, status, sync)
- `mail` - Delegate to mail provider
- `repo` - Multi-repository config

**Knowledge & Memory** (5 commands):
- `remember`, `recall`, `forget`, `memories`
- `kv` - General key-value store

**Reports & Intelligence** (4 commands):
- `status` - Project overview
- `types` - List valid types
- `lint` - Check issue quality
- `orphans` - Orphaned issues

**Workflow** (5 commands):
- `move`, `refile` - Move issues between rigs
- `ship` - Publish cross-project capability
- `todo` - Convenience wrapper for tasks
- `worktree` - Git worktree management

**Help & AI** (6 commands):
- `help`, `human`, `quickstart`, `onboard`
- `prime` - AI-optimized context dump
- `completion` - Shell autocompletion

**Export** (2 commands):
- `export` - JSONL export
- `import` - JSONL import

### Global Flags
- `--json` - JSON output (consistent across all commands)
- `--actor` - Audit trail identity
- `--readonly` - Block writes (sandbox mode)
- `--sandbox` - Disable auto-sync
- `--quiet` / `--verbose` - Output control
- `--db` - Override database path
- `--dolt-auto-commit` - Commit policy (off/on/batch)
- `--profile` - CPU profiling

---

## 5. Ecosystem Context

### .beads/ Directory Structure

```
.beads/
  config.yaml          # sync mode config (dolt-native)
  metadata.json        # backend: dolt, database: bv, project_id
  issues.jsonl         # 1.4MB - git-tracked (legacy, still committed by bd backup export-git)
  .gitignore           # Excludes dolt/, logs, locks, ephemeral stores
  .beads-credential-key # Encryption key for federation
  .local_version       # Tracks bd version for upgrade notifications
  feedback.json        # Correlation feedback data
  correlation_feedback.jsonl
  last-touched         # Last-touched issue tracking
  push-state.json      # Dolt push state
  dolt-server.activity # Keepalive file for idle monitor
  dolt-server.log      # Server logs
  dolt-server.lock     # Server lock
  dolt/                # Dolt data directory (git-ignored)
    .dolt/             # Dolt internal state
    .doltcfg/          # Dolt config
    bv/                # Database directory (named "bv" - historical)
    config.yaml        # Dolt server config
  backup/              # Auto-exported JSONL backup (git-ignored by .beads/.gitignore)
    backup_state.json
    issues.jsonl, events.jsonl, dependencies.jsonl, labels.jsonl, comments.jsonl, config.jsonl
  conventions/         # Label conventions and reference docs
    labels.md
    reference.md
```

### Issue Types

**Built-in** (6): task, bug, feature, chore, epic, decision

**Custom types** configured in this project (9): molecule, gate, convoy, merge-request, slot, agent, role, rig, message

bt's model only knows 5 types (TypeBug, TypeFeature, TypeTask, TypeEpic, TypeChore). The IsValid() method accepts any non-empty string, so custom types render fine but with default icons.

### Config State

Schema version 8. Sync mode: dolt-native. Issue prefix: bt. Compaction disabled. 3 persistent memories stored. Custom types configured for the full Gas Town orchestration vocabulary.

### bd's Design Philosophy

From `bd prime` output and `bd human`:
- bd is CLI-first, with `--json` flags everywhere for machine consumption
- No built-in UI by design - external tools (like bt) build UIs on top
- `bd prime` outputs AI-optimized workflow context for agent sessions
- `bd human` shows a curated subset of commands for human users (vs. the full 70+)
- Agent-oriented: `--readonly`, `--sandbox`, agent state management, interaction audit trail
- Session-aware: `--session` flag on close/update, CLAUDE_SESSION_ID env var

---

## 6. Where bt Fits

### What beads intentionally leaves to external tools

1. **Visual issue boards** - bd has no Kanban/board view, just text lists and graphs
2. **Rich TUI** - bd is purely CLI, no interactive terminal UI
3. **Graph visualization** - bd can generate DOT/HTML files but has no inline renderer
4. **Persistent dashboard** - bd is command-then-exit, no live-updating view
5. **Cross-session context** - bd's prime command is single-shot; bt can maintain persistent state

### Read operations bt should prioritize

| Operation | Current bt | bd Provides | bt Should Add |
|-----------|-----------|-------------|---------------|
| Issue list + board | Board view with columns | `list --json` | Already has (via Dolt query) |
| Issue detail | Detail pane | `show --json --long` | Add missing fields (owner, pinned, defer_until, metadata) |
| Dependency graph | Graph view | `graph --dot/--html` | Already has (via analysis engine) |
| **Issue history** | None | `history --json`, `dolt_diff_issues` | **High value - query dolt_diff directly** |
| **Events timeline** | None | `events` table | **Medium value - show lifecycle events** |
| Blocked issues | Filtered view | `blocked --json` | Already has (via dependency analysis) |
| Ready work | Can filter | `ready --json` | Already has |
| Search | No search | `search/query --json` | **Medium value - add search/filter** |
| Labels overview | Shows labels | `label list-all` | Already shows per-issue |

### Write operations bt should support (via shelling out to bd)

**Tier 1 - Core CRUD** (most valuable for TUI workflow):
- `bd create --title "..." --description "..." --type task --priority 2` - Create issue
- `bd update <id> --status in_progress` / `--claim` - Claim/status change
- `bd close <id> --reason "..."` - Close with reason
- `bd update <id> --title/--description/--notes/--priority` - Edit fields
- `bd update <id> --add-label X --remove-label Y` - Label management
- `bd comments add <id> "text"` - Add comment

**Tier 2 - Relationship management**:
- `bd dep add <blocked> <blocker>` - Add dependency
- `bd dep remove` - Remove dependency
- `bd update <id> --parent <parent>` - Reparent

**Tier 3 - Workflow operations**:
- `bd reopen <id>` - Reopen
- `bd defer <id>` / `bd undefer <id>` - Defer management
- `bd supersede <id> --by=<new>` - Mark superseded
- `bd duplicate <id> --of=<target>` - Mark duplicate

### Dolt-native queries for history/diff views

**Issue changelog** - query `dolt_diff_issues` filtered to a specific issue, showing which fields changed between commits. This gives per-field diffs (old value -> new value) with timestamps.

**Project activity feed** - query `dolt_log` for recent commits, each of which maps to a bd operation (create, update, close, etc.).

**Point-in-time view** - query `dolt_history_issues` to show what an issue looked like at any past commit.

**These Dolt-native queries are strictly superior to the correlation engine's git-log-based approach** for issue history. The correlation engine parses JSONL file diffs from git commits; Dolt provides first-class field-level diffs through SQL.

---

## 7. Recommendations (Key Questions)

### Keep/cut correlation engine (~4.5k LOC)?

**Recommendation: CUT (with caveats)**

The correlation engine (`pkg/correlation/`) extracts issue history by parsing git commits that modify JSONL files. With Dolt as the only backend:

1. **Dolt provides the same data natively and better** - `dolt_diff_issues` gives field-level diffs, `dolt_history_issues` gives point-in-time snapshots, `dolt_log` gives the commit timeline. All queryable via SQL.
2. **JSONL is still committed to git** (issues.jsonl is tracked), but sync mode is `dolt-native`. The JSONL file is a backup artifact from `bd backup export-git`, not the primary data path.
3. **The git-based approach adds ~200ms+ latency** (shelling out to git log/diff) vs. millisecond SQL queries.
4. **Some features have no Dolt equivalent**: co-committed file analysis (which source files were changed alongside issue changes) and orphan commit detection (commits mentioning issue IDs). These are useful for code-issue correlation but require git history access regardless of the issue backend.

**Migration path**: Build a `dolt_diff`-based history provider first. Then evaluate whether the git co-commit analysis features justify keeping the correlation engine. If they do, extract just those features into a smaller module (~500 LOC) and cut the rest.

### Keep/cut deploy pipelines (~1.6k LOC)?

**Recommendation: CUT**

The export pipeline (`pkg/export/`) includes GitHub Pages and Cloudflare Pages deployment (wizard.go, github.go, cloudflare.go). This is a beads_viewer feature designed to publish static issue dashboards. For bt:

1. **bt is a TUI, not a static site generator.** Publishing beads data as a website is a bd concern, not a bt concern.
2. **bd itself has no publishing story** - it exports JSONL and does backup, but doesn't deploy dashboards. This was Jeffrey's feature for beads_viewer.
3. **The viewer assets (HTML/JS/WASM)** are embedded for the deploy target, not for bt's TUI.

**Keep**: The graph rendering code (graph_render_beautiful.go, graph_snapshot.go) has value if bt ever wants to export graph images. The markdown export is also potentially useful. But the deploy wizard and platform-specific deployment code should go.

### Keep/cut TOON format?

**Recommendation: KEEP (small footprint, ecosystem-aligned)**

TOON (Token-Optimized Object Notation) is used for bt's `--format=toon` robot output. It's a single vendored file (`toon-go/toon.go`) plus ~50 lines of integration in `robot_output.go`. The total code footprint is small.

Beads (bd) itself uses `--json` everywhere and doesn't natively support TOON. However, TOON is part of the broader Gas Town ecosystem (Jeffrey created it). Given:
- Small footprint in bt
- Useful for token-efficient AI agent consumption
- No maintenance burden (stable, vendored library)

Keep it. It's not worth the churn of removing it.

### Which robot commands matter?

**Recommendation: Audit against bd's actual agent workflow**

bd's agent workflow uses these patterns:
- `bd ready` -> `bd update <id> --claim` -> do work -> `bd close <id> --reason "..."`
- `bd agent state <id> <state>` for ZFC-compliant state reporting
- `bd agent heartbeat <id>` for liveness
- `bd prime` for session context recovery

bt's robot commands (`--robot-*`) serve a different purpose: they expose bt's analysis engine to AI agents. The most valuable robot commands are:
- `--robot-triage` - Triage scoring and recommendations
- `--robot-graph` - Dependency graph data
- `--robot-insights` - Graph analysis
- `--robot-plan` - Execution planning

These don't overlap with bd's agent commands - they complement them. **Keep all robot commands** that expose analysis data. Cut robot commands that duplicate bd functionality (if any exist).

### SQLite reader - remove or keep?

**Recommendation: REMOVE**

Beads v0.56.1+ removed SQLite and JSONL backends. Dolt is the only storage path for any beads installation from v0.56 onward. The SQLite reader:
- Is 358 LOC in `internal/datasource/sqlite.go`
- Requires a `modernc.org/sqlite` dependency (heavy, pure-Go SQLite)
- Will never receive new data from beads
- bt already fails hard when metadata says Dolt (ErrDoltRequired)

No backward compatibility argument holds: anyone running beads < v0.56 has a 6+ month old installation and should upgrade. bt is a new project with zero legacy users.

### CRUD from TUI - what fields/workflow?

**Recommended CRUD scope for bt TUI**:

**Phase 1 - Status transitions** (highest value, lowest risk):
- Change status: open -> in_progress -> closed (with reason)
- Claim issue (bd update --claim)
- Reopen issue
- These are single-field mutations with clear bd commands

**Phase 2 - Field editing**:
- Edit title, priority, assignee
- Add/remove labels
- Add comment
- Each maps to a single `bd update` or `bd comments add` call

**Phase 3 - Issue creation**:
- Quick create (title + type + priority)
- Full create (description, labels, deps, parent)
- `bd create` supports all fields via flags

**Phase 4 - Relationship management**:
- Add/remove dependencies
- Reparent issues
- Mark as duplicate/superseded

**Implementation pattern**: Shell out to `bd` for all writes. Poll Dolt for changes (existing mechanism). Display success/failure in status bar. Never write to Dolt directly from bt.

### History view - Dolt native?

**Recommendation: YES - use Dolt system tables directly**

bt already has a MySQL connection to the Dolt server. The history view should query:

1. **`dolt_diff_issues`** - For field-level change tracking:
   ```sql
   SELECT from_status, to_status, from_priority, to_priority,
          from_assignee, to_assignee, diff_type, from_commit_date, to_commit_date
   FROM dolt_diff_issues
   WHERE to_id = ?
   ORDER BY to_commit_date DESC
   ```

2. **`dolt_log`** - For commit messages (which are structured: "bd: close bt-mo7r"):
   ```sql
   SELECT commit_hash, message, committer, date
   FROM dolt_log
   ORDER BY date DESC
   ```

3. **`events` table** - bd's own event system (created, closed, status_changed, claimed, renamed, updated):
   ```sql
   SELECT event_type, actor, old_value, new_value, comment, created_at
   FROM events
   WHERE issue_id = ?
   ORDER BY created_at DESC
   ```

**The events table + dolt_diff_issues together provide a complete issue history** without needing the git-based correlation engine. The events table gives high-level lifecycle events; dolt_diff gives precise field-level changes.

---

## Summary

The beads ecosystem is significantly richer than what bt currently surfaces. The biggest opportunities:

1. **History view** using Dolt native queries (dolt_diff_issues + events table) - high value, unique to bt
2. **CRUD from TUI** via `bd` shell-outs - the main feature gap vs. using bd directly
3. **Richer data model** - bt reads 22 of 54 issue columns; adding ~8 more would enable better filtering and display
4. **Search** - bd has powerful search/query; bt could expose this through a search modal

The biggest cuts:
1. **SQLite reader** - dead backend, heavy dependency
2. **Deploy pipelines** - wrong product (bt is a TUI, not a site generator)
3. **Correlation engine** - replaceable by Dolt-native queries (with possible small extraction for git co-commit analysis)
