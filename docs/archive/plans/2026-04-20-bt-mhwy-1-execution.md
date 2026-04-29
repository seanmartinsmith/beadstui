# Execution Handoff: bt-mhwy.1 compact output (via prereqs)

> Copied into repo by the executing session so future sessions can find the plan
> in-tree, not just in the user's `~/.claude/plans/` scratch. The authoritative
> implementation reference is the design spec at
> `docs/design/2026-04-20-bt-mhwy-1-compact-output.md`.

**Scope**: MULTI-SESSION. Three phases, each ends in a stable working state:
1. `bt-uc6k` — schema-drift audit (P2, task)
2. `bt-mhwy.0` — column catchup (P0, task) — blocked by bt-uc6k
3. `bt-mhwy.1` — compact output (P0, feature) — blocked by bt-mhwy.0

**Coordination**: sequential. Each bead blocks the next (`bd dep` graph wired).

**Per-phase handoff**: At the end of each phase, if remaining phases are
unstarted, produce a fresh handoff summary (what was done + SHAs + remaining
work + paste-ready resume prompt). Comment on the next phase's bead, and
append here if useful.

## Brainstorm session

`4c4046f0-1429-46eb-800f-abce77a44871` (bt workspace, 2026-04-20) — origin of
the three beads + design spec. Readable via `cass transcript`.

## Critical path

```
bt-uc6k  (P2, audit)           →   produces drift report
   ↓ blocks
bt-mhwy.0  (P0, column catchup) →   adds metadata + closed_by_session columns
   ↓ blocks
bt-mhwy.1  (P0, compact output) →   ships the feature per design spec
   ↓ blocks (existing children)
bt-mhwy.2 ... bt-mhwy.6         →   downstream work, not in scope here
```

## Orientation reads

1. `docs/design/2026-04-20-bt-mhwy-1-compact-output.md` — authoritative spec
2. `.beads/conventions/reference.md` — bead quality / close template
3. `bd show bt-mhwy` / `bt-uc6k` / `bt-mhwy.0` / `bt-mhwy.1`
4. `pkg/model/types.go` (10–47) — `Issue` struct
5. `internal/datasource/columns.go` — `IssuesColumns` constant
6. `internal/datasource/dolt.go` (60–100, 265–320) — scan in `LoadIssuesFiltered`
7. `internal/datasource/global_dolt.go` (~335) — `scanIssue` mirror
8. `cmd/bt/robot_list.go` — pattern for a robot subcommand
9. `cmd/bt/robot_output.go` — `RobotEnvelope` struct
10. `cmd/bt/cobra_robot.go` — persistent-flag registration

## Phase 1 — bt-uc6k (schema-drift audit, P2)

### Steps

1. Claim: `bd update bt-uc6k --status=in_progress --session $CLAUDE_SESSION_ID --set-metadata claimed_by_session=$CLAUDE_SESSION_ID action=claimed`
2. Read upstream schema under `~/System/tools/beads/internal/storage/schema/migrations/`
   (all migrations, latest is authoritative — in particular 0001 base and any
   migration touching issues/labels/dependencies/comments).
3. Read bt's current view:
   - `internal/datasource/columns.go`
   - `internal/datasource/dolt.go` (the `SELECT … FROM labels`, `… FROM dependencies`,
     `… FROM comments` blocks)
4. Produce `docs/audit/YYYY-MM-DD-bt-beads-schema-drift.md` with:
   - Missing columns on `issues` (known starting points: `metadata`, `closed_by_session`)
   - Missing columns on `labels`, `dependencies`, `comments`
   - Any rename / type change
   - Recommended scope for bt-mhwy.0
5. Commit the report.
6. Close: `bd close bt-uc6k --session $CLAUDE_SESSION_ID` using the
   Summary / Change / Files / Verify / Risk / Notes template.

### Acceptance

- Report exists at the path above with all required sections
- Report identifies scope input for bt-mhwy.0

## Phase 2 — bt-mhwy.0 (column catchup, P0)

### Steps

1. Claim: `bd update bt-mhwy.0 --status=in_progress --session $CLAUDE_SESSION_ID --set-metadata claimed_by_session=$CLAUDE_SESSION_ID action=claimed`
2. Scope follows the audit report. At minimum:
   - `metadata` (JSON column on `issues`)
   - `closed_by_session` (first-class, migration 034)
3. Update `internal/datasource/columns.go` `IssuesColumns`.
4. Update `internal/datasource/dolt.go` `LoadIssuesFiltered` scan.
5. Update `internal/datasource/global_dolt.go` `scanIssue` mirror.
6. Update `pkg/model/types.go` `Issue` struct + `Clone()` deep-copy.
7. Backwards compat via defensive scan (`sql.NullString` pattern).
8. Tests in `internal/datasource/dolt_test.go` — metadata parsing +
   missing-column fallback.
9. Layered commits; `go test ./...` and `go install ./cmd/bt/` must pass.
10. Close: `bd close bt-mhwy.0 --session $CLAUDE_SESSION_ID`.

### Acceptance

- All audited missing columns readable in both readers
- `Issue` has corresponding typed fields; `metadata` deserialized to Go-native map
- Backwards-compat graceful; no crash on older schemas
- Tests green; new tests cover metadata + fallback

## Phase 3 — bt-mhwy.1 (compact output, P0)

Ship exactly per design spec. Four layered commits:

1. `pkg/view/` new package — `CompactIssue`, `CompactAll`, tests, fixtures,
   schema file, `doc.go` with pattern conventions.
2. Bellwether integration in `robot list`:
   - Persistent flags `--shape` / `--format` on `robot` (`cmd/bt/cobra_robot.go`)
   - `BT_OUTPUT_SHAPE` env var precedence
   - `cmd/bt/robot_compact_flag.go` registration
   - `robot_ctx.go`: `shape` field + `projectIssues` + `schemaFor`
   - Envelope `Schema string` field (omitempty)
   - `robot list`: `Issues any`, call `projectIssues`, set envelope schema
   - Golden + contract tests; `--full` byte-identical vs pre-change
3. Remaining 16 subcommands (mechanical bulk): `triage`, `next`, `insights`,
   `plan`, `priority`, `alerts`, `search`, `suggest`, `diff`, `drift`,
   `blocker-chain`, `impact-network`, `causality`, `related`, `impact`, `orphans`.
   Golden + contract tests per subcommand.
4. Docs: `CHANGELOG.md` flag default flip; `docs/adr/002-stabilize-and-ship.md`
   status update if relevant; `robot schema` / `robot docs` mention envelope
   `schema` field.

### Locked decisions (from spec)

- `pkg/view/` (NOT `pkg/model/`).
- `CompactAll([]Issue) []CompactIssue` free function (visible reverse-map dep).
- `--shape` + `--format` orthogonal; `BT_OUTPUT_SHAPE` env var.
- Envelope `Schema` is `omitempty` → absent in `--full` → byte-identical.
- Session-ID fields from `metadata.<name>` today; first-class columns later.
- `--shape` does NOT apply to Alert / Correlation / Decision in v1.

### Acceptance

- `bt robot list --global` compact by default, ≥80% byte drop vs full on 100 issues
- `--full` restores byte-identical pre-change output
- Envelope `"schema": "compact.v1"` present in compact, absent in full
- Compact output contains none of: `description`, `design`, `acceptance_criteria`,
  `notes`, `comments`, `close_reason`
- All 17 subcommands support both modes; golden + contract tests per
- TOON round-trip test per subcommand (catches `any`-typed edge cases)

## Verify (after Phase 3)

```bash
go build ./cmd/bt/ && go install ./cmd/bt/
bt robot list --global | jq '.schema'               # "compact.v1"
bt robot list --global | jq '.issues[0] | keys'     # no description/design/etc.
bt robot list --global --full | jq '.schema'        # null (omitempty)
bt robot list --global | wc -c
bt robot list --global --full | wc -c               # ≥80% drop
go test ./...
```

## Gotchas

### bd / beads
- `bd create --id=<id> --parent=<id>` doesn't combine — create then `bd update --parent`.
- `BD_ACTOR=sms` set in PowerShell profile, don't override.
- `$CLAUDE_SESSION_ID` available as env var; pass `--session` + `--set-metadata
  created_by_session|claimed_by_session` on mutations.

### bt specifics
- Binary `bt`, CLI references `bd`, data dir `.bt/`, env prefix `BT_*`.
- Run `go install ./cmd/bt/` after every `go build ./cmd/bt/`.
- TUI uses Charm Bracelet v2 (Bubble Tea).

### Bead hygiene
- Exactly one `area:*` label per issue.
- Close template: Summary / Change / Files / Verify / Risk / Notes.
- Use `bd remember` for project learnings; no MEMORY.md files.
- Use `bd` for task tracking (no TodoWrite / TaskCreate).

## End-of-session protocol

1. Update `CHANGELOG.md` with a session entry
2. Update `docs/adr/002-stabilize-and-ship.md` if a stream status changed
3. `git pull --rebase && bd dolt push && git push`
4. Verify `git status` shows `up to date with origin`
5. Publish CASS summary via `cass-live:cass-publish`
