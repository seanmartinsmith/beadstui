# STRIDE Threat Model — beadstui (bt)

**Date:** 2026-04-27 00:39 UTC
**Scope:** Entire codebase (~115k lines Go, 25 packages)
**Methodology:** STRIDE per asset × trust boundary

## Application Profile

`bt` is a local-first TUI/CLI dashboard over a beads issue tracker. The binary runs as the local user, reads/writes files in `.beads/` (Dolt repo) and `.bt/` (local cache), shells out to `git`/`bd`/`gh`/`wrangler` for various operations, opens URLs in a browser, and offers an opt-in self-update path that fetches a signed binary from GitHub.

Two surfaces are reachable without local user interaction:
1. **Repository content** — anything checked into a project that bt opens (`.beads/`, `.bt/hooks.yaml`, `.bt/recipes/`, `.bt/themes/`, beads issue payloads, git refs/SHAs).
2. **Self-updater** — HTTPS round-trip to GitHub with SHA256 verification.

There is no server, no network listener, no inbound socket, no auth backend, no persisted user data beyond the local working tree. Threats are dominated by:
- Malicious or untrusted repository content executing as the local user
- Crafted issue/git data leaking secrets or DoSing the tool
- The self-updater being subverted

## Assets

| Asset | Storage | Sensitivity |
|------|---------|-------------|
| Beads issue data | `.beads/dolt/` (Dolt repo, MySQL via 127.0.0.1:3307) | Project metadata; potentially private |
| Local bt cache | `.bt/` (SQLite FTS5 index, baselines, recipes) | Project-derived, low |
| Hook config | `.bt/hooks.yaml` | High — executes commands |
| Recipe config | `.bt/recipes/*.yaml` | Medium — drives queries |
| Theme files | `.bt/themes/*.yaml` | Low |
| Workspace config | `.bt/workspace.yaml` | Low |
| Drift config | `.bt/drift.yaml` | Low |
| GitHub OAuth token | `gh` CLI keyring (delegated, not stored by bt) | High — but not held by bt |
| Cloudflare API token | `wrangler` CLI (delegated) | High — but not held by bt |
| Self-update binary | Downloaded from GitHub releases, SHA256-verified | Critical — replaces bt binary |
| Browser URL | Constructed from issue/export data, passed to OS handler | Low — but injection target |
| Editor invocation | `$EDITOR`/`$VISUAL`, allowlisted | Medium — RCE if bypass |

## Trust Boundaries

```
[1] Untrusted repo (cloned)  ←→  bt process
[2] bt process               ←→  shell (git/bd/gh/wrangler)
[3] bt process               ←→  Dolt server (127.0.0.1:3307, root, no password)
[4] bt process               ←→  GitHub HTTPS (releases endpoint)
[5] bt process               ←→  Browser handler (OS shell/rundll32/open/xdg-open)
[6] bt process               ←→  $EDITOR child
[7] bt process               ←→  Filesystem (RW under cwd; HOME for prefs)
[8] bt process               ←→  Clipboard (write-only)
```

The most consequential boundary is **[1]** — bt opens any project the user `cd`s into and starts processing config files immediately. A repo can land arbitrary YAML, JSONL, and (via beads) git SHAs into bt's input stream before the user clicks anything.

## STRIDE Matrix (per asset class)

### Spoofing

| Source | Threat | Mitigation in code |
|--------|--------|---------------------|
| Self-update binary | Malicious release impersonates GitHub | SHA256 verify; auth header stripped on cross-host redirects |
| Dolt connection | No auth — `root` with no password | 127.0.0.1 only; bind audit needed |
| GitHub API caller | gh token theft | Delegated to `gh` CLI, bt never touches token |
| `bd` binary | PATH-resolved name, could be hijacked | Inherited PATH (no custom resolution) |

### Tampering

| Source | Threat | Mitigation |
|--------|--------|------------|
| `.bt/hooks.yaml` | Repo ships malicious hook | YAML typed-struct, but **command IS executed** by design |
| Beads issue JSONL | Crafted SHA / ID / metadata | Some validation (extractor.go regex-quote), other paths unvalidated |
| BQL query | Injection via interpolation | SQLBuilder dead path interpolates digits; runtime executor pure-Go |
| Git refs from beads | Crafted ref → arg injection to git | Some `--end-of-options` use, inconsistent |
| Issue text → SQL | Direct interpolation | Most paths parameterized, one DB-name path string-escaped |
| Theme YAML | Crafted theme content | Typed-struct decode |
| Cache files | `.bt/` modified between runs | Treated as untrusted; rebuilt on schema mismatch |

### Repudiation

| Source | Threat | Mitigation |
|--------|--------|------------|
| Local actions | No audit log | bt writes back via `bd` CLI, which has its own actor metadata; bt itself is a viewer plus shell-out |
| Hook runs | Hook executions not logged | `pkg/hooks/executor.go` returns output but doesn't audit |
| Self-update | Rollback exists; install/rollback events to stderr | Installed binary path tracked |

### Information Disclosure

| Source | Threat | Mitigation |
|--------|--------|------------|
| Error messages | DSN/path leak in TUI status | shortError truncates, but DSN string still constructed |
| Logs / debug | Sensitive data in `BT_DEBUG` output | Debug mode prints raw queries, including parameters |
| Clipboard | Copies search commands, not secrets | Write-only, low-risk content |
| Crash dumps | Goroutine traces to terminal | Standard Go behavior; no panic encoding of secrets |
| Static export | `pkg/export/` — public site of issues | User-driven; bt warns about public exposure but does not redact |
| Profiler endpoints | `BT_PROFILE` enables CPU/heap profiling | Writes profiles to `.bt/profile/`, not exposed over network |

### Denial of Service

| Source | Threat | Mitigation |
|--------|--------|------------|
| BQL query | Unbounded recursion, IN list | Depth/IN cap status: see findings |
| JSONL parser | Oversize line | 10MB per-line cap |
| Self-update tarball | Oversize entry | size check + io.Copy without LimitReader (prior finding) |
| Snapshot diff | Many issues → many events | EventBulk marker + per-diff cap of 100 events (bt-nexz) |
| Watcher | Filesystem flood | fsnotify rate-limited via debounce timers |
| Dolt query | UNION-ALL across many DBs | bounded by SHOW DATABASES count |
| Tail stream | Long-running `bt tail` | Goroutine with ctx cancel |
| Goroutine leak | TUI Update funcs spawning unbounded | `pkg/ui` has worker registry / cancel patterns |

### Elevation of Privilege

| Source | Threat | Mitigation |
|--------|--------|------------|
| Hooks | repo → arbitrary command | by-design (like git hooks); user must opt in to a project |
| Editor invocation | $EDITOR command-line injection | strict allowlist (well-implemented per prior audit) |
| Browser open | URL → cmd /c start metachar interpretation | mixed: some paths use rundll32, others use cmd start (prior medium) |
| Self-update | Binary swap | SHA256 + GitHub-only host pin |
| Path traversal | Repo specifies `../foo` paths | filepath.Clean, filepath.Base on extraction |

## Top Threat Scenarios

1. **Cloned-repo arbitrary code execution via hooks** — opening any repo with `.bt/hooks.yaml` runs commands. Same threat model as `direnv`/`git hooks`.
2. **Git argument injection via crafted bead SHAs** — beads issue payload influences `git log`/`git rev-list` arguments. Most paths still inconsistent on `--end-of-options` placement.
3. **Browser URL injection on Windows** — `cmd /c start` paths (if any remain) interpret `&`, `|`, `>`.
4. **BQL recursion DoS** — `bt --bql "not not not …"` hangs or OOMs the process.
5. **Dolt local server exposure** — `127.0.0.1:3307` with `root`/no-password is fine on a single-user workstation but risky on a multi-user host. No bind-address audit in setup code.
6. **Self-updater tampering** — already strong (SHA256 + host pin); tarball size limit still missing per prior audit.

## Notes Specific to This Audit (Δ vs 2026-04-09)

Significant code added since prior audit:
- `pkg/tail/` (new) — `bt tail` headless stream of bead events
- `pkg/view/` (new) — projection schemas for robot output (`CompactIssue`, pair/portfolio/ref records)
- `pkg/drift/` (modified) — alert taxonomy + project scope
- `pkg/ui/events/` (new) — event ring buffer, persistence (`bt-6ool` Part A)
- `cmd/bt/cobra_*` (rewritten) — robot subcommand surface significantly expanded
- `pkg/agents/` — author detection now first-class on Issue model

This audit re-checks all prior findings (regression), then sweeps the new surfaces with iteration capacity reserved for them.
