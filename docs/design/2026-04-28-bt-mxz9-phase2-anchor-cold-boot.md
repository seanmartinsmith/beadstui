# bt-mxz9 Phase 2: Anchor + Cold-Boot via `bd -C`

<!-- Related: bt-mxz9, bd-mxz9, bt-fd3k, bt-oiaj, bt-uahv -->

**Date:** 2026-04-28
**Status:** Design
**Author:** sms via Claude Code
**Bead:** bt-mxz9 (P2, area:cli,data)

## Context

Phase 1 (shipped 2026-04-28) added a TCP liveness probe to `DiscoverSharedServer` so a stale port file fails fast with a clear error instead of routing into the misleading start-and-retry cascade. It did not, however, make cold boot from `~` actually work. That requires bt to be able to start the shared Dolt server from a non-workspace cwd.

PR #3442 (merged upstream 2026-04-27) added `bd -C <path>` (git-style chdir before any `.beads/` discovery). This is the upstream-blessed escape hatch: bt's process never chdirs, every subprocess targets a project via `-C`. Phase 2 wires this into the cold-boot path.

The ambient question this answers: when bt has to do something on disk and there's no obvious project context (no cwd inside a project, no issue in scope), what project does it act from?

## Decisions

### 1. Settings split: global vs. per-project

bt has two persistence scopes, not one. Phase 2 establishes both.

**Global (per-user) settings** — readable from `~`, contains things that aren't tied to any single project:

- Path: `~/.bt/settings.json` (resolved via `os.UserHomeDir()` + `/.bt/settings.json`).
  - Cross-platform: `C:\Users\sms\.bt\settings.json` on Windows, `~/.bt/settings.json` on macOS/Linux.
  - Mirrors the existing `~/.beads/` pattern for shared-server config — accessible, single canonical location, no platform-specific path variance.
- Created on first write; nothing breaks if absent.
- Initial fields: `anchor_project` (string, absolute path).
- Future fields owned by this scope: `project_paths` (map dbname→path, for bt-oiaj per-issue write resolution), theme preference, keybinding overrides.

**Per-project settings** — readable only when bt is run inside a project:

- Path: `<project-root>/.bt/settings.json`
- Per existing bt-uahv split: bt-only state, gitignored.
- Initial fields owned by this scope: none added in Phase 2.
- Future fields: tree-state, last-active-filter, per-project search index defaults.

### Why the split

Anchor and project-paths are fundamentally per-user, not per-project. Storing them in a project's `.bt/` would mean either (a) the anchor is unreadable from `~` (defeats the purpose) or (b) we lookup a "primary" project to read settings from, which is the chicken-and-egg the anchor is trying to solve.

bt-fd3k's Phase 2 was scoped against per-project `.bt/settings.json` per the bt-uahv split. With Phase 2 of bt-mxz9 establishing the global file, bt-fd3k Phase 2 should classify each future setting into one scope or the other:

- TUI internal state (tree expand state, last filter): per-project.
- User preferences (theme, keybindings, default search mode): global.
- bt-oiaj `project_paths`, bt-mxz9 `anchor_project`: global.

This implies a sibling comment on bt-fd3k flagging the split.

### Why JSON

The other session's earlier framing left the format open ("settings.json or .yaml"). Choosing JSON:

- bt already parses JSON elsewhere (data layer, robot output, `metadata` blob).
- No YAML parser dependency added.
- Settings file is read by humans rarely; JSON's verbosity isn't a UX issue at this scale.
- Trivially extensible — adding a field is one line, no cross-format migration.

### 2. Anchor lifecycle

**Source-of-truth precedence (highest first):**

1. `BT_ANCHOR_PROJECT` env var — explicit override, never persisted, never auto-modified.
2. `anchor_project` field in global settings.json — persisted, auto-managed.
3. None — fall back to "no anchor available" error UX.

**When the persisted anchor gets written:**

- On any successful boot from inside a beads project where `os.Getwd()` resolves to a directory containing `.beads/` (or has a `.beads/` ancestor walk hit).
- **Rule: latest-cwd-wins.** Each successful inside-project boot overwrites the anchor with the current project. The anchor naturally tracks wherever you've recently been working — no "stuck on a throwaway project from your first boot" failure mode that would otherwise require a UI or manual file edit to fix.
- The `BT_ANCHOR_PROJECT` env var still overrides; bt's auto-write does not touch the env.
- Future configurability: bt-fd3k Phase 1 audit may enumerate this auto-update behavior as a per-user setting (`auto_anchor_on_boot: true | false`). For Phase 2 it's hardcoded to true (auto-update). If a user wants stable-infrastructure semantics, the workaround until bt-fd3k surfaces it is to `export BT_ANCHOR_PROJECT=...` in their shell profile.

**When the persisted anchor gets cleared:**

- On cold-boot path: if `bd -C $anchor dolt start` fails with an "anchor path is no longer a beads project" shape (specifically: bd reports no `.beads/` at the path), bt blanks the field and surfaces a clear message: `"anchor at <path> is no longer a beads project; cd into a project once to re-anchor, or set BT_ANCHOR_PROJECT manually."`
- Bt does NOT clear the anchor on transient failures (network, server process crash) — only when bd indicates the path itself is invalid.

### 3. Cold-boot flow

Replaces the existing logic at `cmd/bt/root.go:215-241`. The new sequence:

```
DiscoverSharedServer:
  port file present + listener live  → return host, port → load global mode
  port file present + no listener    → "stale port file" error → enter cold-boot path
  port file absent                   → "not configured" error → enter cold-boot path

Cold-boot path (when DiscoverSharedServer fails):
  resolve anchor:
    BT_ANCHOR_PROJECT env set              → use it
    settings.json anchor_project set       → use it
    neither set                            → emit "no anchor available; cd into any
                                             beads project once to set one, or export
                                             BT_ANCHOR_PROJECT" and exit non-zero (no
                                             JSONL fallback noise)

  shellout: bd -C <anchor> dolt start
    success → wait for port file (poll 50ms intervals, max 2s) → DiscoverSharedServer
              → load global mode
    failure with "no .beads/ at path" pattern → blank persisted anchor, emit clear
                                                "anchor invalid" message, exit non-zero
    failure other (e.g. bd not on PATH, port already in use)
            → emit bd's stderr verbatim wrapped with context, exit non-zero
```

Removes the existing `BEADS_DOLT_SHARED_SERVER=1 bd dolt start` shellout from `cmd/bt/root.go:855` — that pattern never worked from non-workspace cwd (verified in bd-mxz9 grounding) and the new `bd -C` path supersedes it.

The "fall back to local project" branch when both global and local fail also goes away in this flow. If the user is in a non-workspace dir and global fails, the answer is "fix global" (re-anchor), not "silently degrade to JSONL loader" (which is dead-code-leaking the bt-6am7 false-flag error). When bt is run from inside a project AND global fails, local-project mode is still a valid fallback — but the gating moves to "are we inside a project?" rather than "did global fail?".

### 4. The `BT_TEST_MODE` interaction

`DiscoverSharedServer` already short-circuits with an error when `BT_TEST_MODE=1` so e2e tests don't accidentally hit the developer's shared server. The cold-boot path should also respect this: if `BT_TEST_MODE=1`, do not invoke `bd -C $anchor dolt start` (it would touch the user's real shared server). Current behavior — fall through to local-project loader for the test fixtures — is preserved, gated on `BT_TEST_MODE` rather than on global-failure.

### 5. What this does NOT do

- Per-issue write cwd resolution (bt-oiaj Phase 1 territory). Phase 2 establishes the settings file but does not populate `project_paths`.
- Settings UI (bt-fd3k Phase 3). Anchor is editable only via env var or by editing the file.
- The full bt-6am7 cleanup. Phase 2 removes the JSONL-loader leak from the cold-boot fallback specifically; if any other path still routes into the legacy loader on bare-Dolt installs, that's a separate cleanup.
- bd-mxz9 upstream patch. Per the 2026-04-28 status comment on bd-mxz9, that's now optional. If we want it later for cleanliness, fine — but Phase 2 doesn't depend on it.

## Implementation sketch

### Files

- `internal/settings/global.go` (NEW) — package owning the global settings file. Schema struct, read/write/atomic-replace, default location resolution via `os.UserHomeDir() + "/.bt/settings.json"`. ~80 lines.
- `internal/settings/global_test.go` (NEW) — unit tests for read defaults, round-trip, atomic-replace semantics, env override precedence helper. ~120 lines.
- `cmd/bt/root.go` — replace the existing fallback chain at `:215-241` with the new cold-boot flow described above. Remove the `BEADS_DOLT_SHARED_SERVER=1 bd dolt start` shellout at `:855`. Add the `bd -C $anchor dolt start` invocation. Add the post-start port-file polling loop.
- `cmd/bt/root.go` — add the "successful inside-project boot writes anchor" hook (sticky-on-first-set rule).
- `internal/datasource/global_dolt.go` — no changes (Phase 1 is enough).

### Settings file schema (initial)

```json
{
  "anchor_project": "C:/Users/sms/System/tools/bt"
}
```

That's it for Phase 2. Future fields appended without migration:

```json
{
  "anchor_project": "C:/Users/sms/System/tools/bt",
  "project_paths": {
    "bt": "C:/Users/sms/System/tools/bt",
    "cass": "C:/Users/sms/System/tools/cass"
  },
  "theme": "tomorrow-night",
  "default_search_mode": "hybrid"
}
```

### Atomic-replace semantics

Settings writes use the standard tempfile + rename pattern (`os.CreateTemp` in same dir → `os.Rename`). Avoids partial writes if bt is killed mid-write. Single-author per file (bt itself), no inter-process locking needed.

## Verification plan

### Unit tests

- `internal/settings/global_test.go`:
  - Read missing file → returns zero-value struct, no error.
  - Read+write round-trip preserves fields exactly.
  - Atomic replace: write succeeds even when target file already exists.
  - `Anchor()` resolution: env var trumps file; file used if env empty; both empty returns empty.

### Integration (manual or scripted)

- **Cold-boot from `~` with anchor set, dead server.** Stop shared server, leave port file behind, `cd ~ && bt robot triage`. Expected: bt detects stale port, runs `bd -C $anchor dolt start`, polls for port file, loads global mode. No misleading messages. No JSONL leak. Exit 0.
- **Cold-boot from `~` with no anchor set.** Delete settings.json (or blank the field). `cd ~ && bt robot triage`. Expected: clear "no anchor available" message, exit non-zero, no cascade.
- **Anchor pointing at deleted dir.** Set anchor_project to a non-existent path. Expected: bd reports no `.beads/` at path, bt blanks the field, surfaces clear message, exit non-zero. Subsequent boot from inside any project re-populates.
- **Boot from inside project with anchor unset.** Expected: anchor gets written to the cwd's project path. Boot from `~` afterward works without further setup.
- **Boot from inside project with anchor already set to different project.** Expected: anchor overwritten with the cwd's project path (latest-cwd-wins).
- **`BT_ANCHOR_PROJECT` env var set.** Expected: env path used regardless of file contents. No file write triggered by env-only usage (env doesn't trigger the auto-update path).
- **`BT_TEST_MODE=1`.** Expected: cold-boot path does NOT invoke `bd -C` against any real anchor; falls through to local-project loader for fixtures (current behavior preserved).

### Live verification on this machine

- Boot from `~` with shared server alive: should stay clean (no regressions from Phase 1 baseline).
- Stop shared server (kill PID), leave port file: cold-boot should now actually work via anchor.
- All of the above without breaking the existing project-dir boot path.

## Open questions for implementation

1. **Port file polling interval/timeout after `bd -C $anchor dolt start`.** Sketch says 50ms × 40 = 2s max. Probably fine but worth confirming against bd's typical startup time. If bd takes >2s to write the port file, bt should surface that as a separate "server starting too slowly" error rather than silently failing.
2. **Stderr handling for the `bd -C` shellout.** When bd succeeds, do we discard its stderr or emit it at debug level? Probably discard — successful starts are common, no signal in the stderr.
3. **Anchor recording on `bt --workspace` mode.** The existing workspace flag (`-w`) loads from a JSON workspace config rather than auto-discovery. Should that path also write the anchor? Probably no — workspace mode is an explicit non-default usage, the user knows what they're doing. Anchor only writes from the auto-global path in `cmd/bt/root.go:215+`.
4. **"Inside a project" detection for the auto-write trigger.** Should be the same logic as `loader.GetBeadsDir("")` (cwd ancestor walk for `.beads/`). Keep it shared so we don't drift from bd's discovery shape.

## Backward compatibility

- Users with no settings.json: file is created (or stays absent if the cold-boot path never triggers). No setup required.
- Users with shared server running normally: zero behavioral change, the cold-boot path never runs.
- Users with `BT_GLOBAL_DOLT_PORT` env override: zero change — that path bypasses the liveness check and the anchor entirely.

## Out of scope for Phase 2

- bt-oiaj Phase 1 (`internal/bdcli/` wrapper + `project_paths` map population). Will reuse the settings file infrastructure landed here.
- bt-fd3k Phase 2 (full enumeration of configurables, persistence layer for TUI settings). Should adopt the same settings file shape (global vs. per-project split).
- bt-6am7 (full JSONL-loader cleanup beyond what Phase 2's fallback rewrite removes).
- bd-mxz9 upstream patch.
- TUI surface for editing the anchor (settings screen — bt-fd3k Phase 3).
