# bd CLI Surface Map

Exhaustive enumeration of `bd` (beads CLI) v1.0.3 commands, flags, output shapes, and TUI-fit assessments. Built from `bd <cmd> --help` output across every subcommand and grepping the upstream source at `C:\Users\sms\System\tools\beads`.

**Headline facts**

- bd has **70+ subcommands** across 8 declared categories + an "Additional Commands" bucket.
- bd has **no native `robot` mode**. Machine-readable output is `--json` (and a small number of commands also support `--csv`). There is no `bd robot ...` umbrella.
- Almost every command supports the global `--json` flag. A handful (`restore`, `info`) declare their own local `--json` because they don't open the standard root pipeline.
- `--global` flag selects the shared-server `beads_global` database (cross-project search/show).
- The **dependency type vocabulary** is broader than just `blocks`: `blocks | tracks | related | parent-child | discovered-from | until | caused-by | validates | relates-to | supersedes` (and `relate` shorthand for bidirectional related-to).
- The **issue type vocabulary** includes: `bug | feature | task | epic | chore | decision | merge-request | molecule | gate | convoy | event | wisp` plus custom types via `types.custom` config.
- Most commands respect a "last touched issue" stack — `bd close` and `bd update` work without an ID by acting on the last-created/updated/shown issue.

---

## Top-Level Global Flags

These are inherited by every subcommand:

| Flag | Purpose |
|------|---------|
| `--actor <name>` | Override audit-trail actor (defaults: `$BEADS_ACTOR` → git user.name → `$USER`) |
| `--db <path>` | Override DB path (defaults: auto-discover `.beads/*.db`) |
| `--dolt-auto-commit <off\|on\|batch>` | Dolt commit policy. `batch` defers commits to `bd dolt commit`; SIGTERM/SIGHUP flushes pending. Override via `dolt.auto-commit` config. |
| `--global` | Use `beads_global` shared-server database (cross-project queries) |
| `--json` | Emit JSON output |
| `--profile` | Generate CPU profile |
| `-q, --quiet` | Errors-only output |
| `--readonly` | Block writes (worker-sandbox safety) |
| `--sandbox` | Disable auto-sync |
| `-v, --verbose` | Debug output |
| `-V, --version` | Print version (top-level only) |

There is no `--output <fmt>` (per-format flags exist on individual commands: `--csv` on `sql`, `--dot/--html/--box/--compact` on `graph`, `--format` on `list`/`dep tree`).

---

## Environment Variables

Sourced from `cmd/bd/*.go` and `internal/`:

### Identity & actor
- `BEADS_ACTOR` — primary actor name for audit
- `BD_ACTOR` — deprecated fallback for actor
- `CLAUDE_SESSION_ID` — auto-stamped into session columns on create/update/close (Phase 1a)
- `BEADS_IDENTITY` — config-tier identity override

### Workspace & DB discovery
- `BEADS_DIR` — override resolved beads dir (overrides cwd discovery)
- `BEADS_DB` — explicit DB path
- `BEADS_TEST_IGNORE_REPO_CONFIG` — test guard
- `BEADS_TEST_GUARD_DISABLE` — disable repo guard rails

### Dolt server / connection
- `BEADS_DOLT_PASSWORD` — Dolt SQL password
- `BEADS_DOLT_PORT` — auto-discovery port hint
- `BEADS_DOLT_SERVER_MODE` — force server mode
- `BEADS_DOLT_SHARED_SERVER` — opt into shared server
- `BEADS_DOLT_SERVER_HOST` / `BEADS_DOLT_SERVER_PORT` / `BEADS_DOLT_SERVER_USER` / `BEADS_DOLT_SERVER_DATABASE` / `BEADS_DOLT_SERVER_SOCKET` / `BEADS_DOLT_SERVER_TLS`
- `BEADS_DOLT_DATA_DIR` — custom data dir
- `BEADS_DOLT_REMOTESAPI_PORT` — remotesapi port override
- `BEADS_DOLT_AUTO_START` — disable embedded auto-start
- `BEADS_DOLT_READY_TIMEOUT` — startup timeout
- `BEADS_SHARED_SERVER_DIR` — override shared-server data directory
- `BEADS_TEST_MODE` / `BEADS_TEST_EMBEDDED_DOLT` — test toggles
- `DOLT_REMOTE_USER` / `DOLT_REMOTE_PASSWORD` — DoltHub backup auth

### CLI / UX
- `BD_NON_INTERACTIVE` — skip prompts (CI mode)
- `BD_NO_PAGER` — disable pager
- `BD_PAGER` — pager command override
- `BD_NO_EMOJI` — disable emoji
- `BD_AGENT_MODE` — switch to agent-friendly UI styling
- `BD_DEBUG` — enable debug logs
- `BD_GIT_HOOK` — set when invoked from a hook (changes UI/auto-export)
- `BEADS_TIP_SEED` — deterministic tips
- `CLAUDE_CODE` / `ANTHROPIC_CLI` — auto-detected for tip suppression and styling

### Mail / integrations
- `BEADS_MAIL_DELEGATE` / `BD_MAIL_DELEGATE` — delegate target for `bd mail`
- `BEADS_CREDENTIALS_FILE` — encrypted creds path
- `ANTHROPIC_API_KEY` — for `find-duplicates --method ai`, AI compaction
- `JIRA_API_TOKEN` / `JIRA_USERNAME` / `JIRA_PROJECTS`
- `LINEAR_API_KEY` / `LINEAR_TEAM_ID` / `LINEAR_TEAM_IDS`
- `GITHUB_TOKEN` / `GITHUB_OWNER` / `GITHUB_REPO` / `GITHUB_REPOSITORY` / `GITHUB_API_URL`
- `GITLAB_URL` / `GITLAB_TOKEN` / `GITLAB_PROJECT_ID` / `GITLAB_GROUP_ID` / `GITLAB_DEFAULT_PROJECT_ID`
- `AZURE_DEVOPS_ORG` / `AZURE_DEVOPS_PROJECT` / `AZURE_DEVOPS_PROJECTS` / `AZURE_DEVOPS_PAT` / `AZURE_DEVOPS_URL`

### Orchestrator
- `GT_ROOT` — orchestrator root (formula search path, shared-server resolution)

---

## Working With Issues

### Reads (no mutation)

| Cmd | Purpose | Key flags | Output | Composability | TUI fit |
|-----|---------|-----------|--------|---------------|---------|
| `children <parent>` | List children of a parent (alias for `list --parent <id> --status all`) | none beyond globals | text/json/--pretty (tree) | Pipes into `show`, parent-detail panes | **Core daily-driver** — primary nav for hierarchical issues |
| `comments [id]` | Show or manage comments | `--local-time` | text/json | Subcommand `add` for writes | **Core** — full-text reading |
| `list` | List issues with rich filtering | `--status`, `--type`, `--label[/-any/-pattern/-regex]`, `--exclude-label`, `--priority[-min/-max]`, `--assignee/--no-assignee`, `--no-labels`, `--parent/--no-parent`, `--pinned/--no-pinned`, `--has-metadata-key`, `--metadata-field k=v`, `--ready`, `--deferred`, `--overdue`, `--defer-{after,before}`, `--due-{after,before}`, `--created-{after,before}`, `--updated-{after,before}`, `--closed-{after,before}`, `--title[-contains]`, `--desc-contains`, `--notes-contains`, `--empty-description`, `--id <csv>`, `--spec`, `--mol-type`, `--wisp-type`, `--include-gates`, `--include-infra`, `--include-templates`, `--no-pager`, `--flat/--tree`, `--pretty`, `--long`, `-w/--watch`, `--format <digraph\|dot\|gotmpl>`, `--sort`, `-r/--reverse`, `-n/--limit` | text/json/dot/digraph/gotmpl, tree default | The omnibus query tool — feeds nearly every workflow | **Core daily-driver** — main view |
| `query <expr>` | Filter via SQL-ish DSL (status, priority, type, assignee, owner, label, title, description, notes, dates with relative `7d`/`24h`, id wildcards, pinned, ephemeral, template, parent, mol_type) with `AND/OR/NOT` and grouping | `-a/--all`, `-n`, `--long`, `--sort`, `-r`, `--parse-only` | text/json | Power-user equivalent of `list` | **Core** — advanced search pane |
| `search [query]` | Title/ID search (excludes closed by default; ID-prefix match for `bd-123`) | `--query`, `--status`, `--type`, `--label*`, `--priority-{min,max}`, dates, `--desc-contains`, `--external-contains`, `--has-metadata-key`, `--metadata-field` | text/json | Quick lookup, paginated | **Core** — search bar |
| `show [id...]` | Issue detail | `--current` (last touched), `--children`, `--refs` (reverse lookup), `--thread` (messages), `--as-of <ref>` (Dolt-history snapshot), `--short`, `--long`, `--local-time`, `-w/--watch`, `--id` (escape hatch for IDs that look like flags) | text/json | Backbone of all detail panes | **Core daily-driver** — detail view, must support `--watch` for live mode |
| `state <id> <dim>` | Read a state-dimension label (e.g. `patrol`, `mode`, `health`) | subcommand `state list` | text/json | Pairs with `set-state` write | **Workflow-secondary** — specialty observable |
| `status` (alias `stats`) | DB overview & 24h activity from git | `--no-activity`, `--all`, `--assigned` | text/json | Status-bar/dashboard | **Core** — status header |

### Writes (mutate)

| Cmd | Purpose | Key flags | TUI fit |
|-----|---------|-----------|---------|
| `assign <id> <name>` | Shorthand for `update --assignee` | none | **Workflow-secondary** — keybind `a` |
| `close [id...]` (alias `done`) | Close issues; defaults to last-touched if no ID | `-r/--reason`, `--reason-file`, `-f/--force` (closes pinned/unsatisfied gates), `--claim-next` (atomic close+claim next), `--continue` (advance molecule step), `--no-auto`, `--suggest-next`, `--session` | **Core daily-driver** — keybind `x`/`d` |
| `comment <id> [text]` | Add comment (shorthand for `comments add`) | `--file`, `--stdin` | **Core** — modal text entry |
| `comments add <id>` | Add comment | `-f/--file` | (Same surface) |
| `create [title]` (alias `new`) | Create issue (or batch from `--file` markdown / `--graph` JSON plan) | `-t/--type`, `-p/--priority`, `-d/--description`, `--body-file`, `--stdin`, `--design[-file]`, `--acceptance`, `--notes`, `--append-notes`, `-l/--labels`, `-a/--assignee`, `-e/--estimate`, `--due`, `--defer`, `--parent`, `--id` (explicit ID), `--repo`, `--deps <type:id>`, `--ephemeral`, `--wisp-type`, `--mol-type`, `--no-history`, `--no-inherit-labels`, `--metadata`, `--external-ref`, `--spec-id`, `--skills`, `--context`, `--silent`, `--validate`, `--waits-for[-gate]`, `--event-{actor,category,payload,target}`, `--session`, `--dry-run`, `--force` | **Core daily-driver** — primary write modal |
| `create-form` | Interactive TUI form for create | `--parent` | (Already a TUI — bt would replace it) |
| `delete <id>...` | Hard-delete; updates references to `[deleted:ID]` | `--from-file`, `-f/--force`, `--cascade` (recursive), `--dry-run` | **Workflow-secondary** — destructive, modal confirm |
| `edit [id]` | `$EDITOR` modal for description/title/design/notes/acceptance | `--description` (default), `--title`, `--design`, `--notes`, `--acceptance` | **Core** — built-in textarea instead of shelling to `$EDITOR` |
| `link <id1> <id2>` | Shorthand for `dep add` | `-t/--type` (`blocks\|tracks\|related\|parent-child\|discovered-from`) | **Workflow-secondary** — picker UI |
| `note <id> [text]` | Append to notes (shorthand for `update --append-notes`) | `--file`, `--stdin` | **Workflow-secondary** — quick capture |
| `priority <id> <n>` | Shorthand for `update --priority` | none | **Workflow-secondary** — keybind |
| `promote <wisp-id>` | Promote ephemeral wisp to permanent bead (preserves ID, labels, deps, events, comments) | `-r/--reason` | **Workflow-secondary** |
| `q [title]` | "Quick capture": create issue, output ONLY the ID (for shell composition) | `-l/--labels`, `-p/--priority`, `-t/--type` | **Cross-cutting** — bt would use internally |
| `reopen [id...]` | Reopen closed issues; emits `Reopened` event | `-r/--reason` | **Workflow-secondary** |
| `set-state <id> <dim>=<val>` | Atomic state transition (creates event bead + replaces dimension label, e.g. `patrol=muted`) | `--reason` | **Workflow-secondary** — operator panel |
| `tag <id> <label>` | Shorthand for `update --add-label` | none | **Workflow-secondary** — keybind |
| `update [id...]` | The omnibus mutator | `--title`, `--description`, `--body-file`, `--stdin`, `--design[-file]`, `--acceptance`, `--notes`, `--append-notes`, `-a/--assignee`, `-p/--priority`, `-s/--status`, `-t/--type`, `-e/--estimate`, `--due`, `--defer`, `--parent` (reparent), `--add-label`, `--remove-label`, `--set-labels`, `--metadata`, `--set-metadata k=v`, `--unset-metadata k`, `--external-ref`, `--spec-id`, `--ephemeral`/`--persistent`, `--no-history`/`--history`, `--await-id`, `--allow-empty-description`, `--claim` (atomic in-progress + assignee), `--session` | **Core daily-driver** — most editing routes here |
| `todo` | Convenience wrapper for task issues (`add`, `done`, `list`) | (subcommand-specific) | **Workflow-secondary** — a "TODO panel" |

### Subcommand groups under "Working With Issues"

| Group | Subcommands | Notes |
|-------|-------------|-------|
| `comments` | `add` | Read-side is the bare `bd comments [id]` |
| `gate` | `add-waiter`, `check`, `create`, `discover`, `list`, `resolve`, `show` | Async wait conditions on workflow steps. Types: `human` (manual close), `timer` (auto-expire), `gh:run` (GH Actions), `gh:pr` (PR merge), `bead` (cross-rig). `gate check` polls GH via `gh` CLI. |
| `label` | `add`, `list` (per-issue), `list-all` (all labels in DB), `propagate` (parent → children), `remove` | |
| `merge-slot` | `acquire`, `check`, `create`, `release` | Per-rig exclusive merge slot at `<prefix>-merge-slot`. Holder + priority-ordered `waiters` queue in metadata. Used to serialize conflict resolution under high agent contention. |
| `state` | `list` | Reads dimension labels |
| `todo` | `add`, `done`, `list` | |

---

## Views & Reports

| Cmd | Read/Write | Key flags | Output | TUI fit |
|-----|-----------|-----------|--------|---------|
| `count` | Read | `--by-status`, `--by-priority`, `--by-type`, `--by-assignee`, `--by-label`, plus all `list`-style filters | text/json | **Core** — dashboard tiles |
| `diff <from-ref> <to-ref>` | Read | (refs: commits, branches, `HEAD~N`) | text/json | **Workflow-secondary** — change-since-ref view |
| `find-duplicates` (alias `find-dups`) | Read | `--method <mechanical\|ai>`, `--threshold`, `-n`, `-s/--status`, `--model` | text/json | **Workflow-secondary** — hygiene pane |
| `history <id>` | Read | `--limit` | text/json | **Workflow-secondary** — "git log" for an issue |
| `lint [id...]` | Read | `--type`, `--status` | text/json | **Workflow-secondary** — checks for missing template sections by issue type |
| `stale` | Read | `-d/--days` (default 30), `-n/--limit`, `--status` | text/json | **Workflow-secondary** — hygiene pane |
| `status` (alias `stats`) | Read | `--no-activity`, `--all`, `--assigned` | text/json | **Core** — header (already listed above) |
| `statuses` | Read | none | text/json | **Admin/setup** — config introspection |
| `types` | Read | none | text/json | **Admin/setup** — config introspection |

---

## Dependencies & Structure

### Top-level

| Cmd | Read/Write | Key flags | TUI fit |
|-----|-----------|-----------|---------|
| `dep [id] --blocks <id>` | Write (shortcut) | `-b/--blocks`, `--no-cycle-check` | (See subcommands) |
| `duplicate <id> --of <can>` | Write | `--of` (required) | **Workflow-secondary** — closes dup, links to canonical |
| `duplicates` | Read+Write | `--auto-merge`, `--dry-run` | **Workflow-secondary** — exact-content dup detector (vs `find-duplicates` which is fuzzy) |
| `epic` | (group) | (subcommands) | |
| `graph [id]` | Read | `--all`, `--box` (ASCII layered), `--compact` (tree), `--dot` (Graphviz), `--html` (D3.js interactive); subcommand `check` for graph integrity | **Core daily-driver** — bt's biggest leverage point. Layered DAG default, columns + box-drawing. |
| `supersede <id> --with <new>` | Write | `--with` (required) | **Workflow-secondary** |
| `swarm` | (group) | | |

### `dep` subcommands

| Cmd | Purpose | Key flags |
|-----|---------|-----------|
| `dep add <id> <depends-on>` | Add dep edge | `--blocked-by`/`--depends-on` (alias of positional), `-t/--type` (`blocks\|tracks\|related\|parent-child\|discovered-from\|until\|caused-by\|validates\|relates-to\|supersedes`), `--file <jsonl>` (bulk), `--no-cycle-check`. Also accepts `external:<project>:<capability>` for cross-project deps. |
| `dep cycles` | Detect cycles | none |
| `dep list [id...]` | List deps or dependents | `--direction <down\|up>`, `-t/--type` |
| `dep relate <id1> <id2>` | Bidirectional `relates_to` link | none |
| `dep remove <id1> <id2>` | Remove edge | none |
| `dep tree [id]` | Tree-render | `--direction <down\|up\|both>`, `--max-depth` (default 50), `--status`, `--show-all-paths`, `--format <mermaid>` |
| `dep unrelate <id1> <id2>` | Remove `relates_to` | none |

### `epic` subcommands

| Cmd | Purpose | Key flags |
|-----|---------|-----------|
| `epic close-eligible` | Close epics where all children complete | `--dry-run` |
| `epic status` | Epic completion status | `--eligible-only` |

### `swarm` subcommands

| Cmd | Purpose | Key flags |
|-----|---------|-----------|
| `swarm create [epic-id]` | Create swarm molecule on an epic (auto-wraps single issues into epic) | `--coordinator`, `--force` |
| `swarm list` | List swarm molecules | none |
| `swarm status [epic-or-swarm-id]` | Computed status (Completed/Active/Ready/Blocked) | none |
| `swarm validate [epic-id]` | Pre-flight: cycles, orphans, ready fronts, max parallelism | `--verbose` |

---

## Sync & Data

### Top-level

| Cmd | Read/Write | Key flags | TUI fit |
|-----|-----------|-----------|---------|
| `branch [name]` | Read or Write | (no name → list, name → create); requires Dolt | **Admin/setup** — surface in dolt-aware modal |
| `export` | Read (writes to file) | `-o/--output`, `--all`, `--include-infra`, `--no-memories`, `--scrub` | **Admin/setup** |
| `import [file\|-]` | Write | `-i/--input`, `--dedup`, `--dry-run` | **Admin/setup** — tracks memories too |
| `restore <id>` | Read | `--json` (local flag) | **Workflow-secondary** — restore full pre-compaction content from Dolt history |

### `backup` subcommands

| Cmd | Purpose | Key flags |
|-----|---------|-----------|
| `backup init <path>` | Set up filesystem or DoltHub backup destination | (DoltHub URL: `https://doltremoteapi.dolthub.com/<user>/<repo>`; auth via `DOLT_REMOTE_USER`/`DOLT_REMOTE_PASSWORD`) |
| `backup remove` | Remove destination | |
| `backup restore [path]` | Restore from backup | |
| `backup status` | Last backup status | |
| `backup sync` | Push to backup destination | |

### `federation` subcommands

| Cmd | Purpose | Key flags |
|-----|---------|-----------|
| `federation add-peer <name> <url>` | Add peer (DoltHub, host:port, file://) | `-u/--user`, `-p/--password`, `--sovereignty <T1\|T2\|T3\|T4>` |
| `federation list-peers` | List peers | |
| `federation remove-peer <name>` | Remove peer | |
| `federation status` | Sync status | |
| `federation sync` | Push+pull | `--peer`, `--strategy <ours\|theirs>` |

### `vc` subcommands

| Cmd | Purpose | Key flags |
|-----|---------|-----------|
| `vc commit` | Commit pending Dolt changes | (typical -m) |
| `vc merge` | Merge a branch into current | |
| `vc status` | Branch + uncommitted changes | |

**TUI fit (Sync & Data overall): Cross-cutting.** Backup/federation/vc/branch are admin surfaces, but `restore`, `diff`, `history` deserve hotkeys.

---

## Setup & Configuration

| Cmd | Read/Write | Notes | TUI fit |
|-----|-----------|-------|---------|
| `bootstrap` | Write | Non-destructive setup. Auto-detects: configured remote → clone, git origin Dolt data → clone, `.beads/backup/*.jsonl` → restore, `issues.jsonl` → import, no DB → fresh, existing DB → validate. Flags: `--dry-run`, `--non-interactive`/`--yes`. | **Admin/setup** |
| `config` | (group) | namespaces: `export.*`, `jira.*`, `linear.*`, `github.*`, `custom.*`, `status.*`, `doctor.suppress.*`, `dolt.auto-commit`, `mail.delegate`, `ai.api_key`/`ai.model`, `linear.*` (priority/state/label/relation maps), `external_projects.*`, `repos.*`. Subcommands: `apply`, `drift`, `get`, `list`, `set`, `set-many`, `show` (with provenance), `unset`, `validate` | **Admin/setup** — settings pane |
| `context` | Read | Shows backend identity (repo path, role, backend, mode, database, server, project_id). Reads config files directly (works in degraded states). | **Cross-cutting** — diagnostic header |
| `dolt` | (group) | `start`, `stop`, `status`, `show`, `set <k> <v>`, `test`, `commit`, `push`, `pull`, `remote add/list/remove`, `clean-databases`, `killall`. Config keys: `database`, `host`, `port`, `user`, `data-dir`. | **Admin/setup** — core for daemon mgmt |
| `forget <key>` | Write | Remove memory by key | **Cross-cutting** — memories pane |
| `hooks` | (group) | `install`, `list`, `run`, `uninstall`. pre-commit, post-merge, pre-push, post-checkout, prepare-commit-msg | **Admin/setup** |
| `human` | (group) | Subset of commands for non-agent users; `human list/respond/dismiss/stats` work on issues with `human` label | **Cross-cutting** — "human-needed" inbox |
| `info` | Read | DB path, issue count; `--schema`, `--whats-new`, `--thanks`, local `--json` | **Admin/setup** — about pane |
| `init` | Write | `-p/--prefix`, `--remote`, `--server[-host/-port/-user/-socket]`, `--shared-server`, `--external`, `--database`, `--from-jsonl`, `--stealth`, `--setup-exclude`, `--skip-agents`, `--skip-hooks`, `--agents-{file,profile,template}`, `--reinit-local`, `--discard-remote`, `--destroy-token`, `--force` (alias), `--non-interactive`, `--role`, `--contributor`, `--team` | **Admin/setup** |
| `init-safety` | Read | Documentation for init flag semantics + DESTROY-token format (exit codes 10/11/12) | **Admin/setup** |
| `kv` | (group) | `set`, `get`, `clear`, `list` — generic per-DB key-value store | **Cross-cutting** — internal state for plugins/clients |
| `memories [search]` | Read | List/search persistent memories (text-fuzzy) | **Cross-cutting** — memories pane |
| `onboard` | Read | Print agent-instruction snippet for AGENTS.md | **Admin/setup** |
| `prime` | Read | Output AI workflow context (`--mcp` brief, `--full` full, `--stealth`, `--export`). Auto-detects MCP. Custom override at `.beads/PRIME.md`. | **Cross-cutting** — agent integration |
| `quickstart` | Read | Quick-start guide text | **Admin/setup** |
| `recall <key>` | Read | Get memory contents | **Cross-cutting** — memories pane |
| `remember "<text>"` | Write | `--key` (auto-gen if absent; upsert) | **Cross-cutting** — memories pane |
| `setup [recipe]` | Write | Recipes: `cursor`, `claude`, `gemini`, `aider`, `factory`, `codex`, `mux`, `opencode`, `junie`, `windsurf`, `cody`, `kilocode`. Flags: `--add`, `--check`, `--global`, `--list`, `-o/--output`, `--print`, `--project`, `--remove`, `--stealth` | **Admin/setup** |
| `where` | Read | Active beads location with redirect info | **Cross-cutting** — diagnostic |

---

## Maintenance

| Cmd | Read/Write | Notes |
|-----|-----------|-------|
| `batch` | Write | Read commands from stdin/file. **Single-transaction** with rollback. Grammar: `close <id> [reason]`, `update <id> k=v ...`, `create <type> <pri> <title>`, `dep add <from> <to> [type]`, `dep remove <from> <to>`. Supported update keys: `status`, `priority`, `title`, `assignee`. Flags: `-f/--file`, `--dry-run`, `-m/--message` (Dolt commit msg). |
| `compact` | Write | Squash old Dolt commits. `--days` (default 30), `--dry-run`, `-f/--force` |
| `doctor [path]` | Read+Write | Massive surface: `--fix`, `--fix-child-parent`, `--dry-run`, `-i/--interactive`, `--force`, `--source=jsonl`, `--check <artifacts\|conventions\|pollution\|validate>`, `--clean`, `--deep`, `--server`, `--migration <pre\|post>`, `--agent` (rich AI-facing JSON), `--perf`, `--orchestrator`, `--orchestrator-duplicates-threshold`, `-o/--output`, `--check-health` |
| `flatten` | Write | Nuclear: squash ALL Dolt history into one commit. `--dry-run`, `-f/--force` |
| `gc` | Write | Three phases: decay (delete closed >N days, default 90), compact, Dolt GC. `--older-than`, `--skip-decay`, `--skip-dolt`, `-f/--force`, `--dry-run` |
| `migrate` | Write | Subcommands: `hooks` (marker-managed format), `issues` (move between repos), `sync` (sync.branch workflow). Top-level: `--inspect` (agent analysis), `--update-repo-id`, `--dry-run`, `--yes` |
| `ping` | Read | Open store + trivial query + timing. Exit 0/1. |
| `preflight` | Read+Write | Pre-PR checklist. `--check` (run), `--fix` (placeholder), `--skip-lint`, `--json` |
| `prune` | Write | Delete closed non-ephemeral beads. **Requires** `--older-than` or `--pattern` (safety). `-f/--force`, `--dry-run` |
| `purge` | Write | Delete closed ephemeral beads (wisps, transient mols). `--older-than`, `--pattern`, `-f`, `--dry-run` |
| `rename-prefix <new>` | Write | Rename prefix DB-wide. `--repair` (consolidate multiple prefixes), `--dry-run` |
| `rules` | (group) | `audit`, `compact` — Claude rule auditing |
| `sql <query>` | Read+Write | Raw SQL passthrough. `--csv`, `--json`. **Special**: only command other than per-cmd local flags that emits CSV. |
| `upgrade` | (group) | `ack`, `review`, `status` — version diff vs last-acknowledged |
| `worktree` | (group) | `create <path>`, `info`, `list`, `remove <path>` (`--branch` on create) |

**TUI fit:** Maintenance is almost entirely **Admin/setup**, surfaced through a `:cmd` command palette or settings → maintenance subscreen. Notable exception: `doctor` is rich enough to deserve its own pane in agent-mode UIs (`--agent --json` is purpose-built for it).

---

## Integrations & Advanced

External-issue-tracker bridges. All follow the same shape: `<integration> {pull, push, sync, status}` with config keys per provider. Output: text/json. **TUI fit: Admin/setup** with optional sync-status indicator in the status bar.

| Cmd | Subcommands | Notes |
|-----|-------------|-------|
| `admin` | `cleanup` (delete closed), `compact` (semantic compaction of closed), `reset` (full wipe) | **Admin** — destructive |
| `ado` (Azure DevOps) | `projects`, `pull`, `push`, `status`, `sync` | |
| `github` | `repos`, `pull`, `push`, `status`, `sync` | |
| `gitlab` | `projects`, `pull`, `push`, `status`, `sync` | |
| `jira` | `pull`, `push`, `status`, `sync` | |
| `linear` | `teams`, `pull`, `push`, `status`, `sync` | |
| `notion` | `init` (create DB in Notion), `connect` (existing), `pull`, `push`, `status`, `sync` | |
| `repo` | `add`, `list`, `remove`, `sync` | Multi-repo hydration into single DB. |

---

## Additional Commands (the rest)

| Cmd | Read/Write | Notes | TUI fit |
|-----|-----------|-------|---------|
| `audit` | Write | `record`, `label` — append to `.beads/interactions.jsonl` for SFT/RL | **Cross-cutting** — agent telemetry |
| `blocked` | Read | Show blocked issues | `--parent` (descendants of bead/epic) | **Core** — alongside `ready` |
| `completion <shell>` | Read | Cobra autogen — skip in TUI |
| `cook <formula>` | Read+(opt-write) | Compile formula → proto JSON. Modes: `compile` (keep `{{vars}}`), `runtime` (substitute via `--var k=v`). `--persist` writes proto bead. `--prefix`, `--dry-run`, `--force`, `--search-path`, `--mode` | **Cross-cutting** — formula tooling |
| `defer [id...]` | Write | `--until <time>` (relative or absolute; defaults to status-based defer) | **Workflow-secondary** — keybind |
| `formula` | Read | `list`, `show`, `convert` (JSON↔TOML). Search paths: project formulas → `~/.beads/formulas/` → `$GT_ROOT/.beads/formulas/` | **Cross-cutting** — molecule/wisp underpinning |
| `mol` (alias `protomolecule`) | mixed | See subcommand table below | **Workflow-secondary** with `current`/`progress` getting first-class panels |
| `orphans` | Read+Write | Issues referenced in commits but still open. `--details`, `-f/--fix` (close with confirm), `--label`/`--label-any` | **Workflow-secondary** — cleanup |
| `ready` | Read | Open + no active blockers. `--mol <id>` (steps within molecule), `--gated` (mols ready for gate-resume), `--explain` (dependency-aware reasoning), `--include-deferred`, `--include-ephemeral`, `--exclude-label`, `--exclude-type`, `--unassigned`, `--sort <priority\|hybrid\|oldest>`, `--mol-type`, `--parent`, `--pretty`/`--plain`, label/metadata filters | **Core daily-driver** — primary work picker |
| `rename <old> <new>` | Write | Update primary ID + all references | **Workflow-secondary** |
| `ship <capability>` | Write | Adds `provides:<cap>` label to a closed `export:<cap>`-labeled issue (cross-project resolution). `--force`, `--dry-run` | **Cross-cutting** — cross-project plumbing |
| `undefer [id...]` | Write | Restore deferred to open | **Workflow-secondary** — keybind |
| `version` | Read | Print version | (Built-in) |

### `mol` subcommands

| Cmd | Purpose |
|-----|---------|
| `mol bond <A> <B>` (alias `fart`) | Polymorphic combine: formula+formula, formula+proto, formula+mol, proto+proto, proto+mol, mol+mol. `--type <sequential\|parallel\|conditional>`, `--pour`/`--ephemeral`, `--ref <pattern>` (e.g. `arm-{{name}}`), `--var k=v`, `--as <title>`, `--dry-run` |
| `mol burn <mol-id>...` | Delete mol without digest. `--force`, `--dry-run` |
| `mol current` | Show current position in workflow |
| `mol distill <epic-id> [name]` | Reverse of pour: extract reusable formula from existing epic. `--var <var>=<value>` (recommended) or `<value>=<var>`, `--output`, `--dry-run` |
| `mol last-activity` | Last activity timestamp |
| `mol pour <proto-id>` | Instantiate proto as **persistent** mol. `--var k=v`, `--assignee`, `--attach`, `--attach-type <sequential\|parallel\|conditional>`, `--dry-run` |
| `mol progress` | Progress summary |
| `mol ready` | Mols ready for gate-resume dispatch |
| `mol seed` | Verify formula accessibility |
| `mol show <mol-id>` | Mol structure. `-p/--parallel` highlights parallelizable steps |
| `mol squash` | Compress mol execution into digest (clears Ephemeral, promotes) |
| `mol stale` | Detect complete-but-unclosed mols |
| `mol wisp [proto-id]` | Instantiate as **ephemeral** wisp. Subcommands: `create`, `gc`, `list`. `--var k=v`, `--root-only`, `--dry-run` |

### `gate` subcommands

| Cmd | Purpose |
|-----|---------|
| `gate add-waiter` | Add waiter to gate |
| `gate check` | Evaluate gate conditions, auto-close resolved (`gh:run` resolved on completed+success; `gh:pr` on MERGED; `timer` on elapsed; `bead` on target closed). Escalates failed/expired with `-e/--escalate`. Filters: `-t/--type <gh\|gh:run\|gh:pr\|timer\|bead\|all>`, `--dry-run`, `-l/--limit` |
| `gate create` | Create ad-hoc gate. `--blocks <id>` (required), `-t/--type`, `-r/--reason`, `--timeout`, `--await-id` |
| `gate discover` | Discover `await_id` for `gh:run` gates |
| `gate list` | `-a/--all`, `-n/--limit` |
| `gate resolve <id>` | Manual close |
| `gate show <id>` | Show gate issue |

### `merge-slot` subcommands

`acquire` (`--holder`, `--wait`), `check`, `create`, `release`. Per-rig at `<prefix>-merge-slot`, status flips between `open` (available) and `in_progress` (held), with `metadata.holder` and priority-ordered `metadata.waiters`.

---

## TUI-fit Summary by Cluster

### Core daily-drivers (must be first-class keybinds/views)
- `ready`, `list`, `query`, `search` — work picking + filtering
- `show`, `children`, `comments` — detail nav
- `create`, `update`, `close` — primary writes
- `edit`, `comment`, `note` — text input modals
- `graph`, `dep tree` — DAG visualization (bt's biggest leverage)
- `count`, `status` — dashboard tiles
- `blocked` — companion to `ready`

### Workflow-secondary (menu-invoked)
- `assign`, `tag`, `priority`, `set-state` — quick-edit shortcuts
- `link`, `dep add`/`remove`/`relate`/`unrelate`, `duplicate`, `supersede` — relationship management
- `defer`/`undefer`, `reopen`, `delete` — lifecycle
- `lint`, `stale`, `find-duplicates`, `duplicates`, `orphans`, `epic close-eligible` — hygiene
- `history`, `diff`, `restore` — time travel
- `promote`, `rename` — admin-y but useful
- `mol pour`/`wisp`/`bond`/`burn`/`squash`/`distill`/`show`/`current`/`progress` — molecule lifecycle (only if molecules are a real workflow in bt's audience)
- `swarm validate`/`status`/`create`, `epic status` — multi-agent surfaces
- `human respond`/`dismiss` — inbox for `human:` items
- `todo` — lightweight task panel

### Admin/setup (rare, command-palette only)
- `init`, `bootstrap`, `init-safety`, `setup`, `onboard`, `quickstart`
- All `dolt`/`backup`/`federation`/`vc`/`branch`/`hooks`/`worktree` admin
- All `migrate`/`compact`/`flatten`/`gc`/`prune`/`purge`/`rename-prefix`/`admin` maintenance
- `doctor` (with optional dedicated agent-mode pane)
- All integration groups: `jira`/`linear`/`github`/`gitlab`/`ado`/`notion`/`repo`
- `config`, `where`, `info`, `context`, `version`, `ping`, `preflight`, `upgrade`, `statuses`, `types`
- `sql`, `batch` — power-user escape hatches
- `rules` — Claude-rule audit

### Cross-cutting (powers other features, not standalone UI)
- `q` — quick-capture used by other flows
- `kv` — key-value backing for plugins
- `memories`/`recall`/`remember`/`forget` — persistent memory; surface via memory panel
- `prime`, `onboard` — agent integration
- `audit` — agent telemetry sink
- `cook`, `formula` — molecule template plumbing
- `ship` — cross-project capability resolution
- `gate`, `merge-slot` — async coordination primitives that show up via gates blocking issues, slot held indicators
- `state`, `set-state` — observability dimension labels (foundation for "patrol", "mode", "health" indicators)
- `mail` — delegated mail to orchestrator

---

## Notable Surface Surprises

1. **No `bd robot` umbrella.** Machine output is purely `--json` (and a few `--csv`s on `sql`). bt's robot mode has no upstream parallel — this is bt-distinctive.
2. **Native `events` table is a primitive.** `set-state` writes events + labels atomically; bt does not need to roll its own audit chain.
3. **`batch` is a real transactional primitive.** Single Dolt transaction, rollback on error, supports stdin pipelines. bt should consider piping multi-issue edits through this rather than N round-trips.
4. **`graph --html` produces a self-contained D3.js visualization.** Already a working "graph viewer" — bt's value-add must be interactive, not just rendered.
5. **`gate check` depends on `gh` CLI.** `gh:run`/`gh:pr` gates shell out to `gh run view` / `gh pr view`. A bt-side gate dashboard inherits this dependency.
6. **`merge-slot` is a coordination primitive.** Per-rig exclusive lock with priority-ordered waiters, kept as a regular bead (`<prefix>-merge-slot`, label `gt:slot`). Surface this in any multi-agent UI.
7. **`mol bond` is polymorphic** (formula+formula, formula+proto, formula+mol, proto+proto, proto+mol, mol+mol) and supports parametric child IDs via `--ref arm-{{name}} --var name=ace` — Christmas-ornament pattern for swarm fanout.
8. **`mol distill` reverses pour** — extract a reusable formula from an ad-hoc epic. Powerful for capturing tribal knowledge.
9. **`ship` + `external:<project>:<cap>` deps** are the cross-project plumbing. `bd dep add gt-xyz external:beads:mol-run-assignee` blocks until the beads project ships that capability.
10. **`update --claim` is atomic** (sets assignee + status=in_progress, idempotent), with optional `--session` stamping. Use this everywhere instead of two-step claim.
11. **`close --claim-next`** atomically closes and claims the next-priority ready issue — a one-keystroke "next ticket" workflow.
12. **`show --as-of <ref>`** reads issue state at a Dolt commit/branch — bt could surface this for time-travel comparisons.
13. **`list -w/--watch`** and **`show -w/--watch`** already implement live-update. bt's TUI must compete with these baseline.
14. **`bd q` outputs only the ID** — designed for shell composition. bt should use it internally for "create + immediately do something with it" flows rather than parsing `bd create` output.
15. **`init-safety` is a command** (just docs). Exit codes 10/11/12 carry semantic meaning for refused destructive operations — bt admin flows should detect these.
16. **`sandbox` global flag** disables auto-sync — useful for read-heavy bt sessions to avoid surprise federation pulls.
17. **Output style adapts to `CLAUDE_CODE`/`ANTHROPIC_CLI` env** — bt may want to set/clear these when shelling to bd to control behavior.
18. **`prime` has a `.beads/PRIME.md` override** — agents can customize bd's session-start context per-project.
19. **`bd close --continue`** auto-advances to the next molecule step.
20. **`doctor --agent` outputs ZFC-compliant diagnostics** with observed/expected/explanation/commands/source-files/severity — purpose-built for AI agent consumption. bt's status pane could ingest this directly.

---

## Output Format Coverage

- **`--json` (global)**: works on essentially every read command (`list`, `show`, `ready`, `blocked`, `query`, `search`, `dep tree`, `graph` with limitations, `count`, `status`, `info`, etc.) and on most write commands (returns the touched record(s)).
- **`--csv` (local on `sql`)**: only on `bd sql`.
- **`--dot` / `--html` / `--box` / `--compact` (local on `graph`)**: graph rendering variants.
- **`--format <digraph\|dot\|gotmpl>` (local on `list`)**: `digraph` for `golang.org/x/tools/cmd/digraph`, `dot` for Graphviz, Go template strings.
- **`--format mermaid` (local on `dep tree`)**: Mermaid.js flowchart.
- **No `--robot` flag exists anywhere.**

---

## Bottom-line Inventory

- **8 declared categories** + Additional + Integrations: 70+ leaf subcommands.
- **Densest categories**: Working With Issues (~28), Maintenance (~14), Setup & Configuration (~17 if you count subcommands).
- **Sparsest with most depth**: `mol` (14 subcommands), `dep` (8), `gate` (7), `dolt` (12), `config` (9), `doctor` (single command but ~20 modes).
- **Biggest under-radar surface**: molecules + formulas + wisps + bond + distill (the chemistry layer), gates + merge-slot (async coord), ship + external:/ deps (cross-project), session columns (auto via `--session`/CLAUDE_SESSION_ID).
