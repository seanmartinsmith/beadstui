# Write routing: issue → writable bd target

<!-- Related: bt-scc35, bt-2pk38, bt-oiaj.9, bt-fg7wa -->

Status: shipped (slice 1: bdroute foundation + claim consumer, 2026-07-06).
Decision bead: bt-2pk38. Delivered by bt-scc35; amends bt-oiaj.9 (see its
2026-07-06 amendment comment).

## Problem

bt's first live write (the claim slice, bt-oiaj.10) misrouted in
workspace/global mode: `resolveClaimDir` parsed the bead-ID prefix and
consulted the user-global prefix registry (`~/.bt/projects.json`,
pkg/projects) FIRST in every mode. The registry is a launch-stamped cache
keyed by bead-ID prefix, polluted by non-isolated benches (bt-fg7wa), and
any git repo path passes its validation — so `bd update bt-sj8zw --claim`
ran in a bench temp dir and bd reported "no issue found matching". Verified
live: `"bt"` → bench temp dir, `"bd"` → a deleted job tmp dir,
`"mkt"/"lil"/"dotfiles"` → portfolio. Even single-project claims were
misroutable.

The structural fault, not just the pollution: a prefix-keyed cache stamped
at whatever-launch-happened-last is the wrong source for deciding where a
MUTATION lands. Writing to the wrong database is R6, the highest-severity
risk in the writable-TUI risk register.

## Design: one resolver, three mapping sources

Package `internal/bdroute`: a route table built at LAUNCH (load time),
consulted at WRITE time. `Resolve(issue) → (WriteTarget, error)`; a non-nil
error is a pre-flight refusal — no bd invocation of any kind, actionable
message surfaced in the failure toast.

```go
type WriteTarget struct {
    Dir    string // project checkout for cmd.Dir (bd -C equivalent)
    Global bool   // route via `bd --global` (beads_global; no checkout needed)
}
```

| Mode | Source key | Mapping source | Trust model |
|---|---|---|---|
| single-project | (none) | the launch `workDir`, always | The project you launched in IS the target. Registry never consulted. Fixes the latent single-project misroute. |
| workspace (`.bt/workspace.yaml`) | load-assigned repo Prefix | `LoadResult.AbsPath` (pkg/workspace/loader.go) threaded into the table | User-authored config = authoritative. No identity check — Prefix ≠ dolt_database legitimately (e.g. lil/lil_sto); a wrong path is a config error surfaced by bd's own error. |
| global (shared server) | `issue.SourceRepo` (= DB name, set at load in internal/datasource/global_dolt.go) | `~/.bt/settings.json` `project_paths[dbname]` | Mapping is a cache → **identity proof at write time** (below). Missing mapping → refuse. `beads_global` → `WriteTarget{Global: true}` is the designed contract; slice 1 refuses with an actionable message (`--global` routing is a follow-up). |

The prefix registry (`~/.bt/projects.json`) is **demoted to
History-view/git-log use only**. The write path never consults it. This
kills the polluted-registry class for writes regardless of bt-fg7wa's
timeline.

## Identity proof for global mode (no bd spawn)

Before a global-mode write, bt reads `<dir>/.beads/metadata.json` fresh (at
write time, not launch) and requires:

1. `dolt_database == issue.SourceRepo`
2. `dolt_mode` is server/shared — the checkout routes to the same shared
   server bt read the issue from, not an embedded clone.

This is deliberately stronger than a pre-flight `bd show` verification
read: a wholesale clone of a project would PASS a bd read (bd finds the
issue in the clone) and silently misroute the write into the clone's
embedded DB. The metadata check proves *database identity* — if the config
routes to shared-server DB X, the write lands in the same DB bt read from,
by construction. Mismatch or missing metadata → refusal toast naming the
dir and the reason. Zero bd invocations on any refusal path.

The metadata primitive is two small file reads (`dolt_database`,
`dolt_mode`, `project_id` from metadata.json) — the same one
`detectProjectDBAt` (cmd/bt/root.go) uses for boot-time DB discovery,
shared via bdroute's exported helper.

## settings.json `project_paths`

- `settings.Global` (internal/settings) carries
  `ProjectPaths map[string]string` (dbname → abs path), atomic save as
  before.
- Auto-stamped on successful **cwd-mode** boots only, keyed by
  `dolt_database` read from the project's own metadata.json — never by
  inferred bead prefix (prefix-inference stamping is exactly how `"bt"` got
  clobbered in the registry). Absent metadata → skip silently.
- Manual edits are always possible: `~/.bt/settings.json` is plain JSON;
  add/fix entries under `project_paths` by hand if a machine has checkouts
  bt has never been launched from.
- Stale path at write time → the identity proof catches it → refusal with
  "relaunch bt from <project> once, or edit ~/.bt/settings.json".
- Test/bench isolation: `BT_SETTINGS_PATH` overrides the settings file path
  (mirror of `BT_PROJECTS_REGISTRY_PATH`). Any test touching settings must
  set it; the real user file is never read or written by the suite.

## Composition contracts

- **bdexec** (unchanged in slice 1): routing resolves `WriteTarget`, bdexec
  runs in `target.Dir`. Adding `Dir` to `bdexec.Result` — so receipts show
  where the command ran — is a one-field addition deferred to bt-oiaj.11.
- **bdcmd (bt-s5zgk.1, future)**: the builder composes argv (`--global`
  when `target.Global`; `--actor` when bt-oiaj.14 decides policy); bdroute
  stays argv-agnostic.
- **BD_ACTOR**: v1 inherits the environment untouched (bt-oiaj.10 decision;
  matches what raw CLI bd does today with the profile-wide BD_ACTOR). The
  per-DB seam is bd's own `--actor` flag plus bd's per-project config
  discovery via `-C` — policy deferred to bt-oiaj.14; bdroute carries no
  env-overlay seam in v1.

## Consumers (binding contract)

Every write path — current and future — MUST call `Resolve(issue)` before
any mutation and treat a resolver error as a pre-flight refusal with the
claim slice's UX: failure toast with the resolver's message, pending state
cleared, zero bd invocations. The claim path (pkg/ui/claim.go) is the
reference implementation.

For the inline-edit consumers (bt-oiaj.5 single-line fields, bt-oiaj.6
long-form modals), additionally:

- **Single-line fields** (title, assignee): in-TUI textinput → argv. This
  is Unicode-safe from bt (see cp1252 note below).
- **Multi-line fields** (description/design/acceptance/notes/comments):
  tempfile + `--body-file` family regardless — for multiline/size
  robustness, not codepage fear. Tempfiles under `.bt/tmp/edits/` per
  bt-oiaj.6.
- **$EDITOR**: escalation only (Shift+E) via `tea.ExecProcess`, per the
  tkhq-ratified hybrid (bt-oiaj decision table 2026-05-19, resolution #5).
- Field-edit UI per docs/design/tui-bead-edit-patterns.md (modal picker
  canon + optional cycle keys).

### cp1252: a property of the bash layer, not bt's write path

The historical non-ASCII corruption (memory: bd writes via bash command
line) is a property of the cp1252-encoded bash command-line layer — agents
driving bd through Windows bash. It does not apply to bt-initiated writes:
Go's `exec.Command` passes argv as UTF-16 to `CreateProcessW`, and bd (Go)
decodes `os.Args` from UTF-16 — no codepage is involved end-to-end. The
claim slice's "no free-text anywhere" posture was slice-scoping caution,
not a constraint on bt itself.

## Slice map

| Piece | Status | Where |
|---|---|---|
| bdroute table + Resolve + identity proof | shipped (bt-scc35) | internal/bdroute |
| settings `project_paths` + `BT_SETTINGS_PATH` + cwd-boot stamp | shipped (bt-scc35) | internal/settings, cmd/bt/root.go |
| Claim consumer | shipped (bt-scc35) | pkg/ui/claim.go |
| Registry demotion (write path) | shipped (bt-scc35) | History view keeps registry |
| Test/bench registry isolation | shipped (bt-fg7wa) | test-side TestMain guards |
| `beads_global` `--global` routing | follow-up | Resolve refuses with actionable message today |
| `bdexec.Result.Dir` receipts | bt-oiaj.11 | one-field addition |
| Actor policy (`--actor`) | bt-oiaj.14 | bdroute stays argv-agnostic |
| Inline-edit consumers | bt-oiaj.5 / bt-oiaj.6 | contract bound above, implementation deferred |
