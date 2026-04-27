# Attack Surface Map — beadstui (bt)

**Date:** 2026-04-27
**Methodology:** Static analysis, file enumeration, exec-site grep

## Entry Points

### CLI / Robot subcommands (cobra)

```
bt                          → TUI launch (interactive)
bt --robot-triage           → ranked recommendations
bt --robot-next             → top pick
bt --robot-plan             → execution tracks
bt --robot-priority         → priority misalignment
bt --robot-insights         → graph metrics
bt --robot-alerts           → stale issues
bt --robot-graph            → graph export
bt --robot-search "<query>" → semantic search
bt --robot-bql "<query>"    → BQL execution
bt --robot-history <id>     → history
bt --robot-suggest          → hygiene
bt --robot-diff             → since-ref diffs
bt --robot-metrics          → graph metrics
bt --robot-recipes          → recipe list
bt --robot-schema           → schema dump
bt --robot-docs             → doc dump
bt --robot-drift            → drift detection
bt --robot-sprint <list|show>
bt --robot-labels <health|flow|attention>
bt --robot-files <beads|hotspots|relations>
bt --robot-correlation      → correlation
bt --robot-pairs            → pair detection
bt --robot-list / --robot-list-compact

bt agents <check|add|remove|update>   → agent file management
bt baseline <save|info|check>         → baseline operations
bt export                             → static site / GitHub / Cloudflare
bt tail                               → headless event stream
bt update                             → self-update
bt check-update                       → version check
bt rollback                           → rollback to previous version
bt version                            → version info
```

All `--robot-*` accept piping/redirection; some accept user-controlled flags (`--bql`, `--label`, `--as-of`, `--diff-since`).

### Files Read at Startup (auto-discovery)

| File | Read by | Format | Trust |
|------|---------|--------|-------|
| `.beads/config.yaml` | `cmd/bt/root.go:842` | YAML | repo |
| `.beads/issues.jsonl` | `pkg/loader/` (legacy path) | JSONL | repo |
| `.beads/dolt/` | `internal/datasource/` (Dolt server) | Dolt repo | repo |
| `.bt/hooks.yaml` | `pkg/hooks/config.go:114` | YAML, executes shell | repo |
| `.bt/recipes/*.yaml` | `pkg/recipe/loader.go` | YAML | repo |
| `.bt/themes/*.yaml` | `pkg/ui/theme_loader.go` | YAML | repo |
| `.bt/workspace.yaml` | `pkg/workspace/types.go` | YAML | repo |
| `.bt/drift.yaml` | `pkg/drift/config.go:115` | YAML | repo |
| `$HOME/.bt/agents.json` | `pkg/agents/prefs.go` | JSON | user |
| `AGENTS.md` etc. | `pkg/agents/file.go` | Markdown | repo |

### Files Written

- `.bt/profile/*` (CPU/heap profiles when `BT_PROFILE` set)
- `.bt/baseline/*.json` (baseline snapshots)
- `.bt/search/index.db` (SQLite FTS5)
- `.bt/events.json` (notification ring buffer persistence)
- `.bt/temp/*` (description staging via `os.CreateTemp`)
- Static site export targets (user-chosen)
- Self-update binary at install path

### Network Egress

| Target | Where | Auth |
|--------|-------|------|
| api.github.com / objects.githubusercontent.com | `pkg/updater/updater.go` | unauth (public release) |
| Cloudflare API meta endpoint | `pkg/export/cloudflare.go:431` (curl shell-out, 10s timeout) | unauth |

No inbound listeners. No long-lived sockets except short-lived MySQL connection to local Dolt.

### Process Boundary (shell-outs)

| Binary | Location count | Quoting |
|--------|---------------|---------|
| `git` | 16 sites in pkg/correlation, pkg/baseline, pkg/export, cmd/bt/burndown.go | argv (no shell) |
| `bd` | `cmd/bt/root.go:865` (one site, `dolt start`) | argv |
| `gh` | 11 sites in pkg/export/github.go | argv |
| `wrangler` | 8 sites in pkg/export/cloudflare.go | argv |
| `npm` | 1 site (`-g wrangler` install) | argv |
| `brew` | 1 site (`install gh`) | argv |
| `curl` | 1 site (Cloudflare meta) | argv |
| `cmd /c start` (Windows) | export/github.go, export/cloudflare.go | **shell metacharacter risk** |
| `rundll32` (Windows) | model_export.go | argv |
| `open` (macOS) / `xdg-open` (Linux) | export/cloudflare.go | argv |
| `$EDITOR` / `$VISUAL` | model_editor.go | allowlisted |
| `npm install -g wrangler` | export/cloudflare.go:154 | argv |
| `wasm-pack` | cmd/bt/pages.go:701 | argv (path resolved) |

### Environment Variables Read

```
BT_DOLT_PORT, BT_DOLT_USER, BT_DOLT_PASSWORD (proposed in prior audit)
BT_PROFILE
BT_DEBUG
BT_NO_BROWSER
BT_TEST_MODE
BT_OUTPUT_SHAPE
BT_DOLT_PATH
BT_BD_PATH
BD_ACTOR (passed through to bd)
EDITOR, VISUAL
TERM, NO_COLOR, FORCE_COLOR
HOME, USERPROFILE, APPDATA, LOCALAPPDATA
PATH (inherited)
```

### TUI Input Surfaces

- Keyboard input → tea Update (no injection target since msgs are typed)
- Mouse input → click coords + button (numeric)
- Resize msg → window dimensions (numeric)
- Search input → BQL parser
- Editor return → text → bd create/update via shell-out

## Data Flows

### Repo → bt → Display (read path)

```
.beads/dolt/  ──MySQL TCP─→  internal/datasource/dolt.go  ──→  pkg/model.Issue
                                                                    │
.beads/issues.jsonl ──JSONL─→  pkg/loader/                          │
                                                                    ▼
                                                            pkg/ui (TUI render)
                                                            pkg/view (projections)
                                                            cmd/bt/robot_* (stdout)
```

Trust crosses at the parse boundary. Every parser is typed-struct (no interface{}).

### User input → Issue mutation (write path, indirect)

```
TUI → editor → bd create/update CLI → Dolt
TUI → bd close --reason → Dolt
```

bt itself does not write to issues — it shells out. Adversary's leverage is on what bt passes as argv to `bd`.

### Git → Correlation → Output

```
.git/  ──git rev-list/log/show─→  pkg/correlation  ──→  Issue.Correlations[]  ──→  TUI/robot
```

Adversary surface is git refs/SHAs that bt feeds to git as arguments.

### Self-Update

```
GitHub releases  ──HTTPS─→  pkg/updater/updater.go
                            ├── verify SHA256
                            ├── strip auth on cross-host redirect
                            ├── extract via filepath.Base (zip-slip safe)
                            └── replace install binary atomically (rename + chmod)
```

## Abuse Paths

### Path 1 — Repo-level arbitrary code execution
1. Attacker publishes a project with `.bt/hooks.yaml`
2. Victim clones, opens with `bt`, hooks fire on first event
3. **RCE as victim user**

Mitigation: by-design. Could be hardened with first-run consent.

### Path 2 — Browser metachar injection (Windows)
1. Crafted issue title / Cloudflare project name with `&`, `|`, `>`
2. User triggers "open in browser" flow
3. **Command following metachar runs**

Mitigation: replace `cmd /c start` with `rundll32 url.dll,FileProtocolHandler`.

### Path 3 — Git argument injection
1. Beads JSONL contains a "SHA" of `--upload-pack=evil.sh` or similar
2. Correlation analyzer passes to `git log` / `git rev-list`
3. **git interprets as flag**

Mitigation: validate hex; use `--end-of-options` consistently.

### Path 4 — BQL DoS
1. Attacker convinces victim to run `bt --bql "<deeply nested query>"`
2. Parser recurses, stack grows
3. **process crash**

Mitigation: depth limit + IN-list cap.

### Path 5 — Self-update subversion
1. GitHub release tampered with (or attacker compromises CDN)
2. SHA256 mismatch caught → install aborts
3. **fails closed**

Strong; only residual risks are oversized tarball entries (DoS, prior finding).

### Path 6 — Information disclosure via debug mode
1. User runs with `BT_DEBUG=1` and shares output
2. Output may include DSN, file paths, query parameters

Mitigation: redact secrets in debug output.

### Path 7 — Editor injection (LATERAL)
1. `EDITOR=sh -c "evil"` or `EDITOR=evil-editor`
2. bt rejects via allowlist
3. **fails closed**

Strong defense.

## Coverage Plan for Loop

The 30-iteration loop will cover:
1. Regression on all 20 prior findings (iter 1–10) — confirm fixed/recurring/still-present
2. New surfaces (iter 11–25) — `pkg/tail`, `pkg/view`, `pkg/ui/events`, `pkg/drift` projects, robot cobra surface, baseline cmd
3. Cross-cutting concerns (iter 26–30) — supply chain, CI workflows, schema validation, profiler endpoints, debug-mode leaks
