# Install + first-time setup

This is the full walkthrough. The [README](../README.md) shows the short version.

bt sits on top of [beads](https://github.com/gastownhall/beads), which uses [Dolt](https://www.dolthub.com/) as its storage backend. Three things have to be in place before `bt` will do anything useful:

1. The `bt` binary is on your `PATH`.
2. The `bd` CLI is installed and on your `PATH`.
3. You're in a directory with a `.beads/` subdirectory (created by `bd init`).

## 1. Install bt

### Pre-built binaries (no Go toolchain required)

Download the archive for your platform from the [latest release](https://github.com/seanmartinsmith/beadstui/releases/latest):

- `bt_X.Y.Z_darwin_arm64.tar.gz` — Apple Silicon
- `bt_X.Y.Z_darwin_amd64.tar.gz` — Intel Mac
- `bt_X.Y.Z_linux_arm64.tar.gz` / `_amd64.tar.gz` — Linux
- `bt_X.Y.Z_windows_amd64.tar.gz` — Windows

Verify against `checksums.txt` (also on the release page), extract, and move `bt` somewhere on your `PATH` (e.g. `/usr/local/bin/bt` or `~/bin/bt` on Unix; `%USERPROFILE%\bin\bt.exe` on Windows).

### From source

Requires Go 1.25+. Verify with `go version`.

```bash
go install github.com/seanmartinsmith/beadstui/cmd/bt@latest
```

This installs `bt` to `$(go env GOBIN)` (or `$(go env GOPATH)/bin` if `GOBIN` is unset). Make sure that directory is on your `PATH`.

To build from a clone instead:

```bash
git clone https://github.com/seanmartinsmith/beadstui.git
cd beadstui
go build -o bt ./cmd/bt/
```

Verify:

```bash
bt version
# bt v0.1.0  (or v0.1.0-dev for a local build)
```

## 2. Install bd (the beads CLI)

bt does not bundle `bd`. You need a working `bd` installation. Follow the [beads install guide](https://github.com/gastownhall/beads#install) — pre-built binaries are available there too.

Verify:

```bash
bd version
```

## 3. Initialize beads in a project

Three paths depending on where you're starting.

### a. You already have a project with `.beads/`

Just run bt from the project root:

```bash
cd ~/projects/my-thing
bt
```

bt auto-discovers `.beads/`, starts an embedded Dolt server if one isn't running, and opens the TUI.

### b. Starting from scratch

```bash
mkdir my-project && cd my-project
bd init                                    # creates .beads/ and a Dolt database
bd create -t task -p P2 -d "first issue"   # so the TUI has something to render
bt
```

`bd init --help` covers prefix customization (`--prefix`), stealth mode (`--stealth` to keep `.beads/` out of git), and external server mode. Defaults are reasonable for most cases.

### c. Trying bt against a public beads project

Clone any project that uses beads, then run bt from inside it:

```bash
git clone <some-beads-project>
cd <some-beads-project>
bt
```

If the project hasn't pushed its Dolt database to the repo, you may see an empty list — that's a project-level decision, not a bt problem.

## Verification

A working install should pass all three:

```bash
bt version                # prints the version
bt robot triage           # prints valid JSON to stdout
bt                        # opens the TUI; press q to quit
```

`bt robot triage` is a good sanity check because it exercises the full data path (Dolt connection, graph computation, JSON serialization) without needing the TUI to render.

## Common failures

### "no .beads directory found" / "failed to read beads directory"

You're not in a project with beads initialized. Either:
- `cd` into the right directory, or
- Run `bd init` to create one in the current directory.

### Dolt port already in use

bt and bd auto-start an embedded Dolt server on a default port. If something else is on that port (another bt session, a previous Dolt that didn't shut down cleanly, an unrelated MySQL):

```bash
# Pick a different port for this session
export BEADS_DOLT_SERVER_PORT=3344
bt
```

`BEADS_DOLT_SERVER_PORT` is the canonical override (used by both bd and bt). `BT_DOLT_PORT` is a bt-only fallback if you need bt and bd to talk to *different* servers.

To check what's holding a port:
- macOS / Linux: `lsof -i :PORT`
- Windows (PowerShell): `Get-NetTCPConnection -LocalPort PORT`

### bt exits immediately with no TUI

Almost always a Dolt connection failure. Run with verbose logging:

```bash
BT_LOG_LEVEL=debug bt 2>bt.log
# inspect bt.log for the connection error
```

Common causes: stale Dolt server holding the port (kill it or change ports per above); permission issue on `.beads/` (check directory ownership); a half-completed `bd init` (delete `.beads/` and re-run).

### bt opens but shows zero issues

Three things to check:
- Did anyone create issues? Try `bd list --all` from the same directory.
- Is a filter applied in the TUI? Look at the bottom status bar; press `f`/`p`/`t` to clear filters or `Esc` to reset.
- Are you in the right project root? `bt` keys off the nearest `.beads/` directory walking upward; if you're in a parent or sibling, you may be reading the wrong database.

### `bt: command not found`

The binary isn't on your `PATH`. After `go install`, run `go env GOBIN` and `go env GOPATH` to find where Go put it, then add that directory to `PATH`. After downloading a release archive, you have to extract `bt` and place it somewhere on `PATH` yourself — the archive doesn't install it for you.

## Where to go from here

- [README](../README.md) — feature tour and key bindings.
- [AGENTS.md](../AGENTS.md) — project conventions, robot mode API, commit format.
- [docs/design/testing.md](design/testing.md) — running the test suite, fixtures, coverage thresholds.
- [CONTRIBUTING.md](../CONTRIBUTING.md) — PR workflow and maintainer posture.

If something here is wrong or missing, [open an issue](https://github.com/seanmartinsmith/beadstui/issues/new).
