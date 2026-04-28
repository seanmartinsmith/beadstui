# Writable TUI Design Surface

> Exhaustive design analysis for bt's read-only -> read/write transition.
> Architectural commitment (not re-litigated): **bt shells out to `bd` for ALL writes.**
> No direct SQL writes. bd is the integrity boundary.
> Source: `docs/brainstorms/2026-03-16-dolt-lifecycle-adaptation.md` (Approach C, Hybrid)

Status: design surface only — not an implementation plan, not a sequencing plan.
Author context: feeds the structural reorganization that subsumes bt-oiaj's umbrella.

---

## 1. Operations to write-expose

This catalog walks the full bd write surface (from `bd --help`) and proposes
TUI surface options + tier per operation. Tiering rule: **P0** = without this,
"writable TUI" is a marketing claim, not a feature; **P1** = workflow-essential
for daily-driver use; **P2** = nice-to-have, can follow.

### 1.1 Status-class operations (the smallest meaningful CRUD slice)

#### Close issue — `bd close <id> --reason=<text>` (alias `done`)

- TUI surfaces: keybind in list/board/detail (`x` is conventional in bt today
  for export — see bt-n7i5; conflict to resolve), confirm prompt, then reason
  textarea modal.
- Inputs: `id` (from selection), `reason` (multi-line text required by quality
  conventions — but bd itself permits empty reason).
- Tier: **P0**. Closing is the highest-frequency mutation in any tracker.
- Edge cases:
  - Project conventions (`.beads/conventions/reference.md`) require a structured
    "Summary / Change / Files / Verify / Risk / Notes" reason. TUI should not
    enforce this, but a template scaffold is the right ergonomic. Per-project,
    not hard-coded — read from conventions if present.
  - `--force` needed for pinned issues / unsatisfied gates. Surface a
    second-tap escalation, not a hidden default.
  - `--claim-next` and `--continue` are workflow accelerators — not P0, but
    worth surfacing once core close lands (see §2 Atomicity / molecules).

#### Reopen — `bd reopen <id> --reason=<text>`

- TUI surfaces: same as close, gated on `status=closed` selection.
- Tier: **P1**. Less frequent but trivially derivable from close UX.
- Edge: bd emits a `Reopened` event distinct from a generic status update —
  prefer this command over `update --status=open`.

#### Claim — `bd update <id> --claim`

- TUI surfaces: single keystroke (Enter or Space, per bt-oiaj design notes),
  no modal needed. Idempotent if already claimed by you.
- Tier: **P0**. This is the cheapest mutation to wire and the loudest
  proof-of-concept for the whole pipeline.
- Edge: bd records `claimed_by_session` from `CLAUDE_SESSION_ID` env.
  bt is run by a *human* most of the time — the session column should be
  empty when bt is the actor, OR bt should mint a stable bt-session id.
  Cross-cutting decision (§3).

#### Set status (other transitions) — `bd update <id> --status <name>`

- TUI surfaces: status picker (single-key for common statuses + list modal for
  full set). `bd statuses` enumerates valid values.
- Tier: **P1**. After claim and close, in_progress<->blocked<->open
  transitions are common but lower-volume than the canonical close.
- Edge: status changes that imply attribution (in_progress, closed) consume
  `--session`. Same actor-identity question as claim.

### 1.2 Field-edit operations

All of these compose into `bd update <id> --<field> ...`. They are listed
separately because their UX shapes differ.

#### Title — `bd update <id> --title <text>`

- TUI surfaces: inline textinput replacing the title cell, OR a single-line
  modal. Inline matches the lazygit pattern; modal matches the BQL pattern
  already in `pkg/ui/bql_modal.go`. Pick once, apply uniformly (§3).
- Tier: **P1**. Title fixes are common.
- Edge: titles are a single line; description is multi-line — different
  components.

#### Description — `bd update <id> --description <text>` or `--body-file -`

- TUI surfaces: full-screen textarea modal with markdown preview. Glamour is
  already vendored. Use `--body-file` via tempfile, not `--description` —
  command-line argv has Windows cp1252 corruption for non-ASCII (see
  `feedback_bd_utf8_command_line.md` in MEMORY).
- Tier: **P1**.
- Edge: launching `$EDITOR` (vim/nano/etc.) is the lazygit-style escape hatch
  for serious edits — bt should support it via tea v2's exec pattern. Bubble
  Tea v2 has `tea.ExecProcess` (vendored at `vendor/charm.land/bubbletea/v2/exec.go`).
- Edge: `--allow-empty-description` exists but is a footgun. Default off.

#### Priority — `bd update <id> --priority <0-4 or P0-P4>`

- TUI surfaces: single-key bind (e.g., `1`/`2`/`3`/`4` directly, conventional
  in many trackers) OR small picker.
- Tier: **P1**.
- Edge: bd accepts both numeric and `P0`-`P4` forms. Pick one canonical
  representation in bt's UX.

#### Type — `bd update <id> --type <bug|feature|task|epic|chore|decision>`

- TUI surfaces: picker modal. Validate against `bd types` (which queries
  config-allowed custom types; bt-h5jz tracks the `decision` gap).
- Tier: **P2**. Type is rarely changed after creation.
- Edge: changing an issue's type can break parent-child invariants
  (`epic -> task` hierarchy). bd may or may not enforce — let bd be source
  of truth, surface the error.

#### Assignee — `bd update <id> --assignee <name>` (or `bd assign`)

- TUI surfaces: free-text or picker (read recent assignees from history).
- Tier: **P2** for solo use, **P1** for Gas Town / multi-agent. bt's primary
  user is solo right now, so P2 is defensible.

#### Labels (add/remove/set) — `bd update <id> --add-label <l>`,
  `--remove-label <l>`, `--set-labels <l,l,l>` (or `bd label add/remove`)

- TUI surfaces: existing `pkg/ui/label_picker.go` is read-only today. Promote
  it to a writer: multi-select with toggle, add/remove on Enter, set on
  Shift-Enter. Or split add/remove into separate flows (clearer mental model).
- Tier: **P1**. Labels drive routing and filtering — frequent.
- Edge: `.beads/conventions/labels.md` is the project's source of truth for
  valid labels. **bt must read this file** and not let users invent new labels
  at the TUI level. (Project memory: "No invent labels.") Consequence: this is
  a per-project policy, must respect global mode's per-project conventions.
- Edge: BDExecutor at `_tmp/perles/internal/beads/infrastructure/bd_executor.go`
  shows that `--set-labels` doesn't combine cleanly with other flags, and
  empty-label clearing requires a separate `--remove-label` loop. This
  composition trap is worth abstracting in `internal/bdcli/`.
- Edge: `bd label propagate` — propagate parent's label down to children.
  Bulk operation; consider P2 for the TUI but P1 for an `--robot-*` flag.

#### Notes (append-only) — `bd update <id> --append-notes <text>` or `bd note`

- TUI surfaces: textarea modal, single Submit appends with newline separator.
- Tier: **P1**. Notes are how working sessions accumulate breadcrumbs.
- Edge: bd is append-only via `--append-notes`; bt should not expose
  destructive `--notes` (which replaces) without an explicit "edit notes"
  affordance. Two distinct surfaces, not one.

#### Acceptance criteria — `bd update <id> --acceptance <text>`

- TUI surfaces: textarea modal; same shape as description.
- Tier: **P2**. Important for bead quality (per project conventions) but
  rarely edited mid-flight.

#### Design — `bd update <id> --design <text>` / `--design-file -`

- Same shape as acceptance. Tier: **P2**.

#### Estimate / due / defer — `--estimate <minutes>`, `--due <expr>`, `--defer <expr>`

- TUI surfaces: small modals with format hint (`+1d`, `tomorrow`, `2026-04-15`).
- Tier: **P2**. Solo workflow rarely uses these.
- Edge: `--defer` interacts with `bd ready`'s filter. Worth surfacing visually.

#### Parent — `bd update <id> --parent <id>` (empty to detach)

- TUI surfaces: tree picker, OR free-text ID with validation.
- Tier: **P2**. Reparenting is structural; rare but high-impact.

#### Metadata (set/unset) — `--set-metadata key=value`, `--unset-metadata key`,
  `--metadata <json>`

- TUI surfaces: key=value table editor in detail view.
- Tier: **P2** at best. Metadata is mostly machine-written; user-driven
  edits are an escape hatch, not a daily flow.

#### Wisp lifecycle — `--ephemeral`, `--persistent`

- TUI surfaces: toggle in detail view.
- Tier: **P2**. Already partially exposed via wisp toggle (bt-9kdo done).

#### History flags — `--no-history`, `--history`

- Tier: **P2**. Niche, advanced.

### 1.3 Creation operations

#### Create — `bd create [title] [flags]`

- TUI surfaces: full-screen create modal with sequential or all-at-once form.
  Charm v2's `huh` form library is vendored (`vendor/charm.land/huh/v2/`),
  use it. Inline-create-from-list is also a pattern (lazygit-style).
- Inputs to expose in MVP modal: `--title`, `--type`, `--priority`,
  `--description`, `--labels`, `--parent` (when invoked from a child context).
- Tier: **P0**. Without create, the TUI is "edit-only" which is a worse
  product than read-only.
- Edge: bd's `--id` flag lets the user supply an explicit ID — required for
  cross-project paired beads (per MEMORY: `feedback_cross_project_bead_pairing.md`).
  Surface as an "advanced" field in the create modal.
- Edge: `--graph <json>` and `--file <md>` batch-create operations are
  power-user paths. P2; defer.
- Edge: `--validate` flag enforces required template sections per type.
  bt should default this on for human users — it's a quality gate.

#### Quick capture — `bd q "title"` (returns just the ID)

- TUI surfaces: a single hotkey (e.g. `Q`) opens a one-line input. No
  description, no parent, just title + (current view's filter as labels).
- Tier: **P1**. Fast capture is the lazydev / lazygit pattern (bt-z9ei).
  Distinct from `bd create` in that it minimizes friction.
- Edge: ergonomically the most valuable bind. Conflict-checking with `q`
  (quit) — see bt-tkhq research.

#### Create-form — `bd create-form` (interactive bd-side form)

- TUI surface: do not wrap. bt should *be* the form, not delegate to bd's
  ncurses-ish form. **Do not shell out to `create-form`** — it conflicts
  with bt's own TTY ownership.

### 1.4 Comment operations

#### Add comment — `bd comment <id> "text"` or `bd comments add`

- TUI surface: textarea modal in detail view, dedicated `c` keybind.
- Tier: **P1**. Comments are the public conversation layer; notes are private.
- Edge: use `--file` (tempfile) to avoid cp1252 corruption on Windows.
- Edge: `bd comments` (plural) has subcommands for view/list — read-only
  surface separate from this write path.

### 1.5 Dependency operations

#### Link / unlink — `bd link <a> <b> [--type ...]`, `bd dep add/remove`,
  `bd dep relate/unrelate`

- TUI surfaces: from detail view's "Depends on" / "Blocks" / "Related"
  sections, an Add affordance. Picker for target ID; type selector
  (blocks/tracks/related/parent-child/discovered-from).
- Tier: **P1**. Editing the graph is core to beads' identity.
- Edge: bd's `dep add` semantics are subtle: `bd dep add <blocked> <blocker>`
  vs `bd link <a> <b>` (b blocks a). bt's UX should not expose argv order
  to the user — present as "X depends on Y" and translate.
- Edge: cross-project deps need `bt --global` context. Validate ID against
  the global issue map (already loaded for graph).

#### Cycles — `bd dep cycles` (read), but cycle detection on add is a write-time concern

- TUI surface: confirm modal if a proposed link creates a cycle. bd may
  reject; bt should pre-check via `dep cycles` and warn pre-submit.
- Tier: **P2** (post-MVP; bd will reject anyway).

### 1.6 Lifecycle / structural operations

#### Promote — `bd promote <wisp-id> --reason=<text>`

- TUI surface: detail-view action when viewing a wisp. Reason optional.
- Tier: **P2**. Wisps are agent-emitted; humans rarely promote.

#### Duplicate — `bd duplicate <id> --of <canonical>`

- TUI surface: action in list/detail. Picker for canonical.
- Tier: **P2**.

#### Supersede — `bd supersede <id> --with <new>`

- Same shape as duplicate. Tier: **P2**.

#### Defer — `bd defer <id...> [--until <expr>]`

- TUI surface: keybind to defer (date prompt). Multi-select compatible.
- Tier: **P2**.

#### Undefer — `bd undefer <id...>`

- Inverse of above. **P2**.

#### Set-state — `bd set-state <id> <dim>=<val> [--reason=...]`

- TUI surface: state-dimension picker, then value picker.
- Tier: **P2**. Operational dimension (patrol/mode/health) — agent-driven
  more than human.

#### Delete — `bd delete <id...> [--force | --dry-run]`

- TUI surface: keybind + double-confirm modal showing what will be removed
  (deps, references). Per project rules: no destructive ops without
  explicit guard.
- Tier: **P2** if at all. Project rule from `AGENTS.md` Core Rules: "No file
  deletion without explicit written permission." That rule applies to
  files-on-disk, but the spirit (destructive ops require a high bar)
  carries to bead deletion. Recommendation: do not surface delete in TUI
  for v1; force users to drop to `bd delete` manually. **Decision needed
  (§3).**

### 1.7 Gates / async coordination

#### Gate operations — `bd gate list/check/resolve/...`

- TUI surface: dedicated gate view; resolve action with reason input.
- Tier: **P2**. Gates are agent-coordination machinery; the TUI mostly
  needs to *display* them, not write them. Bead bt-rbha already tracks
  the read-side surface.
- Edge: `bd merge-slot` (acquire/release/check) is closely related — same
  read-mostly story.

### 1.8 Tagging

#### `bd tag <id> <label>`, `bd label add <id> <label>`

- Equivalent to `update --add-label`. Single helper, single surface.

### 1.9 Triage / batch / hygiene

#### Batch — `bd batch` (stdin grammar, single transaction)

- TUI surface: bt's bulk operations (multi-select close, multi-select
  relabel) should compose into a `bd batch` invocation, not N individual
  `bd update` calls. See §2 Atomicity.
- Tier: **P1** as an *internal* mechanism for any multi-select TUI feature.
  Not surfaced as a user-visible "batch mode."

#### Find-duplicates / lint / preflight / doctor

- These are informational/audit. **Not write operations** despite "fix"
  framing. Out of writable-TUI scope.

### 1.10 Operations that should NOT be TUI-surfaced

Explicit non-list — these are bd surfaces bt should not wrap:

- `bd create-form` — TTY conflict.
- `bd edit` — opens `$EDITOR`, conflicts with bt's TTY. Project rule already
  forbids using `bd edit` (`AGENTS.md`: "Do NOT use bd edit").
- `bd dolt push/pull/start/stop/...` — server lifecycle, already partly
  managed in `internal/doltctl/`.
- `bd compact / flatten / gc / prune / purge` — destructive maintenance,
  manual-only.
- `bd init / bootstrap / setup / migrate / rename-prefix` — one-time
  configuration.
- `bd federation / repo / jira / linear / github / gitlab / notion / ado`
  — integrations. Out of scope for v1.
- `bd remember / forget / recall / memories` — agent memory, read-mostly
  for humans; if surfaced, read side only.
- `bd kv` — agent KV store; not a human surface.
- `bd ship` (cross-project capability publishing) — niche, advanced.
- `bd backup / restore` — admin-only, not TUI material.

---

## 2. Architectural concerns specific to shell-out writes

### 2.1 Latency

Each `bd` invocation has irreducible cost: process startup (~50ms on
Windows), Dolt server connect (~30ms even when warm), command execution
(~20-200ms depending on operation). Empirical baseline: 100ms is optimistic;
200-500ms is realistic for `bd update` with auto-commit on.

Implication: **synchronous shell-out from the Update loop is forbidden.**
Bubble Tea's Update must remain non-blocking. All mutations must be `tea.Cmd`s
that return a result message.

Strategies:
- Treat every mutation as async: dispatch `tea.Cmd`, render a "Saving..."
  spinner, accept the result message.
- Keep a per-issue write-pending flag in the model so the UI can dim or
  badge issues mid-write.
- Avoid chaining writes in the same user gesture (one keypress = one bd
  command, not three). When chaining is required, use `bd batch`.

### 2.2 Optimistic UI

Design tension: `bd` is the source of truth, but its result lands at most
5 seconds later (the Dolt poll interval). User experience expectation
is sub-second feedback.

Three options:
- **A. Pessimistic** — wait for `bd` to return success, then mutate the
  in-memory model. Feels slow but is honest. ~200-500ms perceived latency.
- **B. Optimistic + reconcile** — mutate the in-memory model immediately,
  fire `bd` async, on failure roll back with toast. Risk: divergent state
  between optimistic write and Dolt poll snapshot.
- **C. Hybrid: optimistic for trivial fields, pessimistic for structural** —
  status changes, label adds: optimistic. Reparenting, deletion, type
  changes: pessimistic. Defensible split.

Recommendation: start pessimistic + spinner. Move to C once UX feedback
warrants it. Going straight to B is a footgun — divergence bugs are hard.
**Decision needed (§3).**

### 2.3 Failure modes

bd command can fail for many reasons:
- **Validation error** (e.g., empty close reason on a project that
  enforces it via convention — though bd itself permits empty).
- **Conflict** (concurrent write changed the issue between bt's read and
  bt's write). bd has optimistic concurrency? Not directly visible — Dolt
  is the merge layer. Likely "last writer wins" within bd.
- **Dolt server down** — `bd dolt start` failed, or the server crashed.
  bt's `internal/doltctl/` already handles startup; need write-time
  detection.
- **bd binary not on PATH** — uncommon for bt's user base but possible.
  Currently `cmd/bt/root.go` already shells out for `dolt start` and
  expects bd; need a single discovery point and graceful error.
- **Bad input** — title too long, invalid status, invalid label.
- **Permission / readonly mode** — `bd --readonly` or `bd --sandbox`
  blocks writes.

Recovery patterns:
- All shell-outs return `(stdout, stderr, exitcode, err)`. `err != nil`
  triggers a toast + per-issue badge ("write failed").
- For transient failures (server down): retry with backoff in the bdcli
  layer, not the UI layer.
- For validation errors: parse stderr (best-effort), surface in the modal
  that initiated the write so the user can correct in place. **Don't
  dismiss the modal on failure.**
- For conflicts: refetch, show diff, re-prompt. Hard to do well — defer
  past v1.

### 2.4 Concurrency

Two scenarios:
- **Another bd session writes between bt's read and bt's write**. The
  poll interval is 5s; a human can absolutely change something via
  `bd update` from another shell mid-flight. bt's write of the *old*
  field value will silently overwrite. There is no way for bt to know
  this happened without re-reading first.
- **Another bt session is open** on the same database. Same problem,
  doubled. bt does not currently coordinate across its own sessions.

Mitigations:
- **Refetch-then-write** for any field-edit modal: when the modal opens,
  capture the current value; when the user submits, refetch via
  `bd show <id> --json`; if the field changed, prompt "field has changed
  externally, [O]verwrite / [R]eload / [C]ancel".
- For status/priority/label flips (single-keypress mutations), accept
  last-writer-wins. Cost of a refetch on every keystroke is unacceptable.
- For description/title (long-form edits), refetch is mandatory.

This is a known pattern (e.g., GitHub web UI's stale-edit warning).
**Decision needed (§3): is this fight worth picking in v1?**

### 2.5 Atomicity

A single user gesture often implies multiple bd commands:

- "Create child issue with labels and a parent link" =
  `bd create --parent <p> --labels a,b,c -t task ...`. This is *one* command.
  Good — bd already composes.
- "Close N issues with the same reason" = N invocations OR `bd batch`.
  Use batch.
- "Relabel: remove label X, add label Y on issue Z" = `bd update Z
  --remove-label X --add-label Y`. One command.
- "Set labels to exactly [a,b]" — one command via `--set-labels`. But
  setting to empty requires a `bd show` followed by N `--remove-label`
  invocations (the BDExecutor pattern from perles). Composite operation,
  not atomic in the wire sense.

Problematic compositions:
- **Create + dependency**: `bd create` accepts `--deps`, so atomic.
- **Create + comment**: two commands. If the second fails, the first
  succeeded — issue created without intended comment. Tolerable: comment
  is recoverable.
- **Bulk close + bulk relabel**: should be batched via `bd batch` (single
  transaction). bd's batch grammar covers close, update, dep add/remove,
  create. Use it.

Recommendation: the bdcli wrapper layer should expose composite operations
that *internally* select between a single bd call, a batch invocation, or
a fallback "best-effort sequence with rollback hints if any step fails."
The UI layer never composes raw bd args.

### 2.6 Undo

bd has no undo primitive. Dolt's commit log is the audit trail, but bt
cannot do a `git revert`-style operation — bd doesn't expose one. Options:

- **No undo, only confirm**: every destructive operation requires a
  confirmation modal. Lazygit's pattern. Pros: simple, safe. Cons: feels
  primitive.
- **Logical undo via inverse operation**: track the last N writes in a
  session ring buffer; "undo" replays the inverse (close -> reopen,
  add-label -> remove-label, set X to A having been B -> set to B). Pros:
  feels native. Cons: not all ops are reversible (delete is permanent;
  reasons get appended, not replaced).
- **History view + manual rollback**: just route the user to `bd history
  <id>` and let them see what changed. No bt-side undo. Honest.

Recommendation: **option (a) for v1.** Confirm every destructive op (close,
delete, reparent, supersede, duplicate, status->closed via update). No undo
infrastructure. Revisit when user reports calibrate. **Decision needed (§3).**

### 2.7 Drafts

Long-form edits (description, design, comment, close-reason) take time.
The user may switch views, lose focus, or accidentally hit Esc.

Options:
- **No draft**: Esc dismisses; content lost. Lazygit-ish but harsh.
- **In-memory per-issue draft**: model holds a `drafts map[id]map[field]string`;
  reopening the modal restores. Lost on bt restart.
- **Disk-persisted drafts** under `.bt/drafts/<id>-<field>.md`: survives
  restart, syncs across bt sessions on the same machine, recoverable.
- **Use `bd`'s `--body-file` mechanism end-to-end**: every edit writes to
  a tempfile in `.bt/tmp/edits/`; on submit, bd consumes it; on cancel,
  the file remains. User can manually recover.

Recommendation: in-memory drafts for v1, disk-persisted as a follow-up.
The tempfile approach is required anyway (UTF-8 corruption issue) — easy
to extend it into draft persistence.

### 2.8 Validation

Where does validation live?

- bd is source of truth — it will reject invalid input. bt should not
  reimplement bd's rules.
- However, **client-side validation prevents round-trip cost** for obvious
  errors (empty title, invalid priority value, label not in conventions).
- Pre-flight checks like "is this label in `.beads/conventions/labels.md`?"
  are bt-side because bd doesn't enforce label vocabulary itself.

Rule:
- bt validates **format** (priority is P0-P4; status is from `bd statuses`;
  type is from `bd types`).
- bt validates **project conventions** (labels from `.beads/conventions/`,
  required template sections from `bd lint`).
- bt does NOT validate **business rules** (cycle detection, parent-child
  consistency, gate satisfaction). Defer to bd.

Cache `bd statuses`, `bd types`, and the labels file at bt startup; refresh
on poll-detected change.

### 2.9 Multi-bead operations

Bulk close, bulk relabel, bulk-defer scenarios. Must use `bd batch`
(single transaction). Failure modes:

- **Whole batch fails** (one bad ID in the list, transaction rollback).
  All-or-nothing semantics, surface to user with the offending ID.
- **Partial UI selection** (50 issues selected, batch grammar limits) —
  bd batch grammar is one-line-per-op, no inherent limit, but stdin pipe
  has OS-dependent buffer sizes. Stream via stdin pipe, not args.
- **Cross-database bulk** in global mode — bd batch is per-database. bulk
  across projects means N batches, one per database. Spawn N processes
  in parallel? Sequential? **Decision needed (§3)**.

### 2.10 Audit trail (the actor / session question)

bd has two orthogonal identity dimensions:

- **Actor** (`BD_ACTOR` env, or `--actor` flag): who is the human / agent.
  Stored on every event.
- **Session** (`CLAUDE_SESSION_ID` env, or `--session` flag): which CC
  session originated the action. Set on `created_by_session`,
  `claimed_by_session`, `closed_by_session` columns.

bt's situation:
- `BD_ACTOR=sms` is set globally in PowerShell profile. Good.
- bt is run by a human. There is no CLAUDE_SESSION_ID. bd's session
  columns will be empty.
- bt-oiaj's notes argue for **dual attribution**: every mutation should
  carry both "the human" and "the tool" (bt). Options:
  - `BD_ACTOR=sms` + a tool tag in metadata or notes (e.g., append "[via bt]"
    to comments). Cleanest, no conventions invented.
  - `BD_ACTOR=sms@bt` — overloads actor with tool. Polluted across all
    other consumers.
  - Set a synthetic session id like `bt-<pid>-<timestamp>`. Misuses the
    session column (which exists for CC, not arbitrary tools).
  - Use `--actor=sms` per-call and rely on **a future bd `--via` flag**
    that bt would propose upstream.

Recommendation: in v1, do nothing special. `BD_ACTOR=sms` is preserved.
bt's dual-attribution requirement is genuine but **the cleanest path is
upstream**: file a bd issue requesting a `--client` or `--via` flag.
Until then, optionally add `(bt)` to comment text and close-reason text
where the user typed it themselves. **Decision needed (§3).**

### 2.11 Output parsing

bd's `--json` flag is the stable contract. Use it everywhere. Non-JSON
output is for humans and is not stable.

Concrete patterns:
- `bd create --json` returns a JSON object with the new issue. Parse and
  use the ID.
- `bd update <id> --json` returns the updated issue. Parse for echoed
  truth.
- `bd close --json` likewise.
- `bd q "title"` returns *just the ID* on stdout — special case.

Failure-channel strategy:
- stdout = JSON success, parse it.
- stderr = human-readable error, surface verbatim to the user (after
  trimming and de-ANSI-ing).
- exit code = boolean success/failure.

The bdcli wrapper must capture all three. Don't `cmd.Run()` and discard
stderr — perles' BDExecutor at line 50-58 shows the right pattern.

### 2.12 Working directory / global mode

bd discovers `.beads/` from CWD. bt running with `--global` may show
issues from many projects, but writes need the *project's* CWD context.

Three architectural patterns:
- **chdir before exec**: bdcli temporarily cds into the project's working
  directory before running bd. Race-prone (bt's other goroutines see the
  changed cwd). Hard in Go.
- **Per-process cwd via `cmd.Dir`**: `exec.Cmd` has a `Dir` field. Set
  per invocation. Safe, idiomatic. **This is what perles' BDExecutor does.**
- **bd `--db` flag**: bd accepts `--db <path>` for explicit database
  routing. Bypasses CWD discovery. Use in global mode.

Recommendation: `cmd.Dir` for project context + `--db` as a fallback.
Both are forms of "tell bd which database explicitly." Map every bt-loaded
issue to its source database path at load time; pass that path on write.
This unblocks bt-oiaj's stated dependency on bt-ssk7 (global mode).

---

## 3. Cross-cutting decisions that need to be made

> Phrased as questions with options. These will be made-wrong-by-default
> if not addressed before implementation begins.

### 3.1 Modal vs inline editing — pick a consistent pattern

- **Option A**: All edits go through full-screen modals (BQL pattern).
  Predictable, easy to test, plays well with `huh` v2.
- **Option B**: Inline editing where feasible (title in list, priority via
  hotkey), modals only for multi-line content (description, comment).
  More tactile, more lazygit-like.
- **Option C**: Per-field declared shape — meta the decision into a
  spec the implementation reads.

Recommendation: **B**. Matches bt-z9ei's lazydev framing and bt-oiaj's
"inline edit" Phase 4. Document the rule.

### 3.2 Optimistic vs pessimistic UI

See §2.2. Recommend **pessimistic + spinner for v1**, with a path to
hybrid.

### 3.3 Stale-edit / concurrency strategy

See §2.4. Recommend **refetch-on-modal-open, last-writer-wins for hotkey
mutations.** Defer the full conflict modal.

### 3.4 Undo

See §2.6. Recommend **confirm-only, no undo infrastructure.**

### 3.5 Delete in TUI?

See §1.6 delete entry. Recommend **no, drop to bd manually**. Spirit of
project's no-destructive-ops rule.

### 3.6 Gas Town actor / dual-attribution

See §2.10. Recommend **defer to upstream (file bd issue), use
`BD_ACTOR=sms` as-is in v1.**

### 3.7 Cross-database write routing

See §2.12. Recommend **`cmd.Dir` + `--db` fallback**, mapped per-issue
at load.

### 3.8 Where does the bdcli wrapper live?

bt-oiaj proposes `internal/bdcli/`. Confirm: yes, internal/ (not pkg/),
because the wire format is bt-private and unstable. Layered on top:
domain-typed adapters in `pkg/write/` or similar that the UI consumes.

### 3.9 What's the canonical way for the UI to dispatch a write?

Options:
- A `tea.Cmd` factory (`writeCmds.CloseIssue(id, reason)` returns `tea.Cmd`).
- A worker-pool pattern with a write queue (background goroutine reads
  from a channel; UI publishes intent).
- A "saga"-like pattern for multi-step writes.

Recommendation: **`tea.Cmd` factory for v1.** Bubble Tea's idiomatic
async pattern. Worker pool only if benchmarks show contention (e.g.,
Gas Town scenarios with bursts of writes).

### 3.10 Does the BQL filter follow the user into write-mode?

Concrete case: user filters to `priority<=P1`, presses `n` to create a
new issue. Should the create modal pre-fill `priority=P1`? Should it
inherit labels from the current filter?

Options:
- **No inheritance**: every create starts blank. Predictable, but loses
  context.
- **Soft inheritance**: prefill from current filter, user can clear.
  Matches "filter as default scope" intuition.

Recommendation: **soft inheritance**. Document.

### 3.11 Does writable-TUI redefine `/` (search)?

Currently `/` opens a search modal (read-only). Writable TUI introduces
`q` (quick-capture, lazygit convention) and `n` (new). `/` remains read.
But: in many editors `/` is read while edit mode reserves `:` for commands.
Should bt expose a command palette (`:close`, `:label add foo`) in
addition to keybinds?

Recommendation: **out of scope for v1**, but reserve `:` for future use.
bt-tkhq research feeds this.

### 3.12 What does Ctrl-C / Esc / q mean during a write?

Ctrl-C inside a modal — cancel the modal (do not quit bt). Esc — same.
`q` — context-sensitive (modal: cancel; otherwise quit). bt-tkhq covers
this; lock it before any write modal lands.

---

## 4. Re-scoped acceptance criteria for bt-oiaj

bt-oiaj's current acceptance criteria (verbatim):
> - Users can perform basic CRUD operations on beads from within bt
> - All mutations go through bd CLI, not direct Dolt writes
> - Mutations carry dual attribution (tool + human)
> - Poll refresh picks up changes made through bt

This is too vague to ship against. Recommend rewriting the umbrella as
follows.

### Proposed re-scope for bt-oiaj

**Definition of "writable TUI" for the purpose of closing this umbrella**:

bt has a writable TUI when ALL of the following hold:

1. **bdcli wrapper exists** at `internal/bdcli/` with typed wrappers for
   the operations listed below. No `exec.Command("bd", ...)` for writes
   anywhere outside this package. (Dolt-server lifecycle in
   `internal/doltctl/` is out of scope and stays separate.)
2. **Status hotkeys work in list and board**: claim (single-keystroke,
   `bd update --claim`), close (with reason modal, `bd close`), reopen
   (with reason modal, `bd reopen`).
3. **Create modal works**: title, type, priority, description, labels,
   parent fields. `bd create --json` parsed, new issue appears in the
   list within one poll cycle. Quick-capture variant via `bd q` available
   via separate hotkey.
4. **Inline edit works for**: title, priority, status, assignee. Each
   uses the chosen pattern (modal vs inline, decision §3.1).
5. **Multi-line edits work for**: description, design, acceptance,
   comment, append-notes. All go through tempfile (`--body-file`) to
   avoid Windows cp1252 corruption.
6. **Label add/remove works** with respect to project conventions
   (labels.md is read; users cannot invent new labels via the modal,
   only choose from the vocabulary).
7. **Failure handling**: every write modal stays open on bd error; the
   error text is shown inline; user can retry. Toasts dismiss on
   timer for non-modal mutations.
8. **Cross-database routing**: in `--global` mode, every write targets
   the correct database via `cmd.Dir` and/or `--db`. Verified via test.
9. **No undo, no stale-edit conflict resolution beyond refetch-on-modal-open
   warning.** These are out of scope, tracked as separate beads if needed.
10. **Help / discoverability**: `?` overlay enumerates write keybinds
    grouped separately from read keybinds. (Coordinates with bt-xavk.)
11. **All write keybinds are documented in a single source** referenced
    by the TUI tutorial.

**Out of scope for the bt-oiaj umbrella**:
- Bulk operations (multi-select close, bulk relabel) — defer to a child.
- Dependency graph editing — defer to a child.
- Gate / merge-slot writes — defer (bt-rbha territory).
- Delete / promote / supersede / duplicate / defer / set-state writes —
  separate beads each.
- Comment threading / editing existing comments / deleting comments —
  separate.
- Drafts persistence to disk — separate follow-up.
- Optimistic UI — separate follow-up after pessimistic baseline ships.
- Dual-attribution beyond `BD_ACTOR=sms` — separate, dependent on upstream.
- Command palette (`:`) — separate.

**Implied child beads** (the umbrella's structure):
- `bt-oiaj.1` — bdcli wrapper package.
- `bt-oiaj.2` — claim/close/reopen hotkeys.
- `bt-oiaj.3` — create modal.
- `bt-oiaj.4` — quick-capture (`q`).
- `bt-oiaj.5` — inline title/priority/status/assignee edit.
- `bt-oiaj.6` — long-form (description/design/comment) modals via tempfile.
- `bt-oiaj.7` — label picker write mode (with conventions enforcement).
- `bt-oiaj.8` — write-aware help overlay.
- `bt-oiaj.9` — cross-database write routing in --global.
- (do not file these beads here; the structural reorg should.)

---

## 5. Writable-TUI implications for existing beads

For each, the recommendation is a scope clarification, not a rewrite.
Filing of new beads is out of scope — these are notes for the structural
reorg.

### bt-oiaj — itself

Already addressed in §4. Should be re-described per the proposal above
when the structural reorg lands.

### bt-94a7 — Broad upstream audit

**Implication**: bt-94a7's Tier 1 says "is there a bt equivalent / wrapper
/ mention?" for each bd subcommand. This audit was framed during the
read-only era — it asked "should bt surface it?" without the writable-TUI
context. Now that writable is committed, the audit should explicitly
distinguish:
- Read-side bd surfaces (already covered).
- **Write-side bd surfaces** (close, update, create, comment, etc.) —
  audit asks "is there a TUI surface for this in v1 plan? if not,
  why?". This catalog (§1) is the input.
**Suggested action**: amend bt-94a7's description to require write-side
classification per command. Ideally, bt-94a7's audit includes this catalog
verbatim or by reference.

### bt-72l8.1 — Ghost-features audit

**Implication**: ghost-feature classification (working/stub/ghost/partial)
currently considers read-side observability. With write-side coming,
features like the sprint dashboard (whose **producer** would have to be
reborn as a writable concept) get murkier — a sprint-create write surface
would be a new feature, not a fix to a ghost. **Suggested action**: add
a fifth classification tag, "read-only-by-design", for features that are
consciously not getting a write surface (forecast, alerts, history). This
prevents bt-72l8.1 from re-opening already-decided write-scope questions.

### bt-72l8.1.1 — TUI ghost-features per-view audit

**Implication**: same as bt-72l8.1, plus this audit is the natural place
to verify that **every existing read view has a write-side decision recorded**
("write surface goes here / no write surface for this view"). **Suggested
action**: extend acceptance to require a write-decision row per ViewMode.

### bt-tkhq — Keybinding research

**Implication**: this bead already explicitly calls out CRUD as a forcing
function. No scope change needed, but its **outputs** are blocking inputs
for bt-oiaj's child beads (decision §3.1, §3.4, §3.12). **Suggested
action**: elevate priority from P2 to P1 — it's no longer optional research,
it's a prerequisite.

### bt-gf3d — Keybinding consistency overhaul (epic)

**Implication**: every binding in this epic must be re-evaluated against
the writable-TUI binding set. New bindings claimed by writable-TUI
(`q`, `n`, `c`, `e`, `1`-`4`, etc.) collide with existing ones (e.g.,
`x` is the export keybind per bt-n7i5). **Suggested action**: add an
explicit "reserve write-side keys before any non-write rebinding lands"
clause to the epic description.

### bt-xavk — Help system redesign

**Implication**: help system must distinguish read keybinds from write
keybinds visually. Acceptance criteria of bt-oiaj item 10 depends on this.
**Suggested action**: add a write/read column or section requirement to
bt-xavk's design.

### bt-s4b7 — Project navigation redesign

**Implication**: this bead ALREADY calls out the writable-TUI cwd
problem in its description (search "Working Directory Edge Cases" in the
bead). Decision §2.12 / §3.7 of this report should feed back into bt-s4b7
as a resolution. **Suggested action**: bt-s4b7's acceptance should
require a documented routing rule (cmd.Dir + --db) before any project
navigation UI changes.

### bt-ssk7 — Global mode federation

**Implication**: bt-ssk7's "Scope" line says "Read-only. Write path (CRUD
from TUI) deferred but kept in mind architecturally." That deferral is
the right framing, but bt-oiaj item 8 (cross-database write routing)
*depends* on bt-ssk7 being feature-complete enough that bt knows
which database each issue lives in. **Suggested action**: add a
"Provides" note to bt-ssk7 explicitly: "exposes per-issue source database
mapping for write routing (consumed by bt-oiaj.9)."

### bt-4fxz — bd audit / actor integration

**Implication**: this is the closest existing bead to the dual-attribution
question (§2.10 / §3.6). Currently scoped as a *read* of `bd audit`.
**Suggested action**: bt-4fxz should be split or augmented — its audit
side is read-only, but it should also propose the upstream bd `--via` flag
or whatever attribution mechanism bt needs for writes. Coordinate with
bd-side beads if any exist.

### bt-h5jz — `decision` type support

**Implication**: write-side "create issue" modal must include `decision`
as a type option once h5jz lands. **Suggested action**: ensure h5jz
acceptance covers the create-time path (it currently focuses on display).

### bt-rbha — Gate / human-label TUI surface

**Implication**: surfacing gate and `human` labels in the TUI is
read-side; gate *resolution* (bd gate resolve) is a write. **Suggested
action**: bt-rbha should explicitly defer gate-resolve writes to a child
bead — it's writable-TUI scope, not pure-display scope.

### bt-mbjg — Default-hide gates

**Implication**: same as bt-rbha — once we hide gates, "show me gates"
becomes a special filter. Combined with gate-resolve writes, it's the
"gates view" affordance.

### bt-ks0w — Mouse click support

**Implication**: writable-TUI introduces destructive ops where a stray
click is more dangerous than a stray keypress. **Suggested action**:
bt-ks0w should add "no destructive operations bound to mouse-only
gestures" to its acceptance.

### bt-9kdo — Wisp toggle (closed)

**Implication (already shipped)**: wisp toggle is read-side. The
write-side complement is `bd update --persistent` / `--ephemeral`, which
is the promote/demote path. Note for future: when promote write lands,
it should integrate with the wisp toggle's filter intuition.

### bt-4ew7 — Multi-select bead IDs

**Implication**: multi-select feeds bulk-operations. **Suggested action**:
bt-4ew7 should reference bd batch (§2.5 / §2.9) as the dispatch
mechanism for any future bulk-write feature, even though bt-4ew7 itself
is just copy.

### bt-19vp — History view focused dogfood

**Implication**: writable mutations create new history entries. bt-19vp's
dogfood should include "verify bt-side writes show up correctly in
history view" once writes ship. **Suggested action**: add a
post-writable acceptance row.

---

## 6. Risk register

Severity scale: 1 (low) - 5 (catastrophic).

| ID | Risk | Severity | Likelihood | Mitigation |
|----|------|---------:|-----------:|-----------|
| R1 | **Stale edit overwrites real work**: User edits description in modal; meanwhile, `bd update` from another session changes it; on submit, bt's old value wins. | 4 | 3 | Refetch-on-modal-open. Detect change, prompt. (§2.4) |
| R2 | **Dual-attribution missing**: Writes go to bd as `BD_ACTOR=sms`, indistinguishable from raw `bd` invocations. Audit/forensics later: who edited via TUI? Unknown. | 2 | 5 | Defer to upstream (bd `--via` flag). Optionally tag close-reasons / comments with "(via bt)" suffix. (§2.10) |
| R3 | **Windows UTF-8 corruption**: Non-ASCII content in titles/descriptions/comments mangled by cp1252 routing through bash command line. Already documented in MEMORY. | 4 | 5 | Always use `--body-file <tempfile>` for any text content. Never argv. (§1.2 description) |
| R4 | **Modal latency feels broken**: User presses close, sees nothing for 300ms, presses again, second invocation hits a closed issue and errors. | 3 | 4 | Spinner + disable resubmit + idempotent close. (§2.1) |
| R5 | **bd binary not on PATH or wrong version**: bt assumes a feature flag that bd-this-version doesn't expose. | 3 | 2 | Detect at startup (already partial via `internal/doltctl/`). Version-gate write features; degrade gracefully. |
| R6 | **Cross-database write goes to wrong DB**: In global mode, user closes bt-xxx but cwd is bd-yyy's project; close lands in bd database. | 5 | 3 | Per-issue source-database routing; `cmd.Dir`. (§2.12 / §3.7) Mandatory test. |
| R7 | **Convention drift**: Project requires structured close-reason; bt allows any text; closes ship without structure. | 2 | 4 | Read `.beads/conventions/reference.md`; offer template scaffold but don't enforce (project policy is human-discipline, not tool-enforcement). |
| R8 | **Atomic-batch failure is invisible**: 50-issue bulk close fails on issue 23; user sees "batch failed" and doesn't know which 22 succeeded vs which 28 didn't. | 3 | 3 | bd batch is transactional (all-or-nothing per `bd batch --help`). Surface "batch rolled back, no changes" honestly. (§2.5) |
| R9 | **Keybinding collision**: `x` was export (bt-n7i5); writable-TUI wants `x` for close (lazygit). Users muscle-memory clash. | 2 | 4 | Resolve in bt-tkhq + bt-gf3d before write keys land. (§5 bt-gf3d) |
| R10 | **Label invention**: User adds `urgent2` label via modal; bt accepts it; later audit reveals invalid label. | 2 | 3 | Read `.beads/conventions/labels.md`, restrict picker to known set. Power users can drop to `bd update` manually for new labels. (§2.8) |
| R11 | **Destructive op without confirmation**: User intends to defer, presses wrong key, deletes. | 5 | 2 | Confirm modal on every destructive op. Don't bind delete by default. (§3.5) |
| R12 | **Concurrent bt sessions diverge**: User has bt open on laptop and desktop; both write; last-writer-wins corrupts the user's mental model. | 3 | 2 | Out of scope for v1; document. Poll picks up changes within 5s. |
| R13 | **bd CLI breaking change between versions**: Upstream bd renames `--reason` to `--why`; bt's wrapper breaks silently. | 3 | 2 | Pin a minimum bd version; integration test in CI; bt-jov1 (upstream sync hook) catches earlier. |
| R14 | **Fork hazard from JSON output drift**: bd's `--json` output schema changes; bt's parser breaks. | 3 | 3 | Treat `bd --json` as a versioned contract; tolerate unknown fields; fail loud on missing required fields. |
| R15 | **Modal-trap UX**: Long-form edit modal eats keypresses bt expects to be navigation; user confused, presses Esc, loses content. | 3 | 4 | Drafts in memory (§2.7) + clear modal-mode indicator in footer. |
| R16 | **Tempfile leak**: `--body-file` writes to `.bt/tmp/edits/`; on crash, files accumulate. | 1 | 3 | Sweep on startup; document; gitignore. |
| R17 | **Race against poll**: Just-written change gets briefly overwritten by an in-flight poll's stale snapshot, then re-corrected on next poll. UI flickers. | 2 | 3 | Pessimistic UI: only render after bd confirms. (§2.2) |
| R18 | **Server lifecycle interaction**: bt-started server might receive a write from another `bd` invocation right when bt is shutting it down. | 2 | 2 | `btStartedServer` flag already prevents killing others' servers. Ensure shutdown drains pending writes. |
| R19 | **Cross-prefix write routing**: In global mode, user writes a bd-xxx issue from bt; bt routes correctly to beads project but BD_ACTOR / labels are bt's. | 2 | 3 | Per-database environment scoping in bdcli. |
| R20 | **Project-conventions divergence in global mode**: Two projects have different label vocabularies; bt's label picker should change as the user navigates between them. | 2 | 4 | Re-load conventions per active project. (§2.8) |
| R21 | **Quick-capture (`q`) shadows quit**: `q` is bd's quick-capture and lazygit's quit. Conflict. | 3 | 5 | Decide before any write keybind ships; bt-tkhq feeds. (§3.12) |
| R22 | **Dolt commit policy mismatch**: bd's `--dolt-auto-commit` defaults to `off`. Writes accumulate in working set; bt sees them via SQL but they're not committed. Pull/push won't carry them. | 3 | 3 | Either force `--dolt-auto-commit=on` for bt writes (safe default), or document the working-set behavior, or expose a "commit pending" affordance. **Decision needed.** Easy to forget; high impact for Gas Town federation. |
| R23 | **Read-only / sandbox mode silently breaks writes**: bd is started in `--readonly` or `--sandbox`; bt's writes fail with confusing errors. | 2 | 2 | Detect mode at startup; disable write keybinds and show indicator. |

---

## Appendix A — Quick reference: the bdcli surface (proposed)

Methods the wrapper should expose, derived from §1. Names are illustrative.

```
// Status / lifecycle
Claim(id) error
Close(id, reason string, force bool) error
Reopen(id, reason string) error
SetStatus(id, status string) error

// Field updates (each takes pointer-to-string for tristate set/clear/keep)
UpdateFields(id, opts UpdateOptions) error
AppendNotes(id, text string) error

// Labels
AddLabels(id, labels []string) error
RemoveLabels(id, labels []string) error
SetLabels(id, labels []string) error

// Comments
AddComment(id, text string) error

// Creation
Create(opts CreateOptions) (newID string, _ error)
QuickCapture(title string, opts QuickOptions) (newID string, _ error)

// Dependencies
LinkDep(from, to, depType string) error
UnlinkDep(from, to string) error

// Lifecycle structural (P2)
Promote(wispID, reason string) error
Duplicate(id, canonicalID string) error
Supersede(id, replacementID string) error
Defer(id string, until *string) error
Undefer(id string) error

// Bulk
Batch(ops []BatchOp) error
```

Every method takes a context, a database identifier (path or alias), and
returns a typed error category (validation / conflict / transport / unknown).

---

## Appendix B — Keys reserved by writable-TUI (proposal, not a binding)

> Inputs to bt-tkhq research, NOT a binding decision.

- Lower-case, list-context: claim/select/edit hotkeys. Likely candidates:
  Enter (claim/open detail), `c` (comment), `e` (edit), `n` (new),
  `q` (quick-capture — collides with quit), `r` (reopen — collides with
  refresh), `x` (close — collides with export bt-n7i5), `1`-`4` (priority).
- Upper-case: destructive variants (e.g., `D` for delete-with-confirm).
- Modifier-key: `Ctrl+S` save (modal submit), `Ctrl+Enter` (force submit).

Every collision listed above must be resolved by bt-tkhq before any
write key ships. Recommend that the structural reorg block any
write-key bead on bt-tkhq's outputs.

---

End of report.
