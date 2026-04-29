---
title: "feat: Adapt Dolt connection for beads v0.59+ lifecycle"
type: feat
status: active
date: 2026-03-16
origin: docs/brainstorms/2026-03-16-dolt-lifecycle-adaptation.md
beads: bt-07jp
---

# Adapt Dolt Connection for beads v0.59+ Lifecycle

## Overview

Beads v0.59-0.61 removed the persistent Dolt server daemon. `bd` now auto-starts/stops Dolt per command with ephemeral ports. bt's polling model assumes a persistent server and is currently broken. This plan implements Approach C (hybrid): bt detects or starts a persistent Dolt server on launch, reads via direct SQL, writes via `bd` CLI, and only stops the server on exit if bt started it.

## Problem Statement / Motivation

bt connects to Dolt via a persistent MySQL connection pool and polls `MAX(updated_at)` every 5s. This worked when beads maintained a long-running Dolt server with an idle monitor sidecar. As of beads v0.59:

- The daemon and idle monitor were fully removed
- Each `bd` command auto-starts Dolt, uses it, then `DoltStore.Close()` stops it
- Ports are ephemeral (OS-assigned), not hash-derived
- The `.beads/dolt-server.activity` keepalive file has no consumer

Result: bt launches, finds no server, prints "Start it with: bd dolt start" and exits. The keepalive writes are dead code.

The fix is straightforward: bt should start a Dolt server if one isn't running, and stop it when bt exits (if bt was the one that started it). `bd dolt start` still exists and is the officially supported way for external tools to run a persistent server.

(see brainstorm: docs/brainstorms/2026-03-16-dolt-lifecycle-adaptation.md)

## Proposed Solution

### Phase 1: Server Lifecycle Management (new module)

Create `internal/doltctl/doltctl.go` - a small module that encapsulates Dolt server detection, startup, and shutdown.

**Why a separate module**: This logic doesn't belong in `datasource/` (which handles data reading) or `pkg/ui/` (which handles display). It's infrastructure that sits between them.

```go
// internal/doltctl/doltctl.go
package doltctl

// ServerState tracks whether bt started the Dolt server and owns its lifecycle.
// Guarded by mu since poll loop goroutine and main goroutine both access it.
type ServerState struct {
    mu             sync.Mutex
    Port           int
    StartedByBT   bool    // true if bt ran `bd dolt start`
    ServerPID      int     // Dolt server PID from bd dolt start output
    BeadsDir       string  // path to .beads/
}

// EnsureServer detects or starts a Dolt server.
// Returns ServerState describing what happened.
func EnsureServer(beadsDir string) (*ServerState, error) { ... }

// StopIfOwned stops the Dolt server only if bt started it.
func (s *ServerState) StopIfOwned() error { ... }
```

**EnsureServer flow:**

```
0. Check bd is available: exec.LookPath("bd")
   - If not found: return error "bd CLI not found - install beads first"

1. Resolve port via datasource.ReadDoltConfig(beadsDir)
   - Single source of truth for port resolution (see Phase 4)

2. TCP dial 127.0.0.1:<port> (500ms timeout)

3. If listening:
   - Return ServerState{Port: port, StartedByBT: false}

4. If not listening:
   - Print "Starting Dolt server..." to stderr
   - Run: bd dolt start (capture stdout/stderr)
   - Parse output for PID and port (format: "Dolt server started (PID XXXXX, port YYYYY)")
   - If bd dolt start fails: return error with bd's stderr
   - Verify port file appeared, read it
   - TCP dial to confirm server is ready (retry up to 10s, 500ms intervals)
   - Return ServerState{Port: port, StartedByBT: true, ServerPID: parsed}
```

**StopIfOwned flow:**

```
1. If !StartedByBT: return nil (not our server)

2. Read .beads/dolt-server.pid
   - If PID file is gone or PID doesn't match ServerPID: return nil (someone else took over)

3. Run: bd dolt stop (capture output, 5s timeout)
   - If fails: log warning, don't block exit
```

**Multi-instance safety** (addresses SpecFlow Gap 1):

The PID check in StopIfOwned is the key: bt records the Dolt server's PID at startup. On shutdown, it re-reads `.beads/dolt-server.pid`. If the PID has changed (another bt instance restarted Dolt, or an external process started a new server), bt does NOT stop it. If the PID matches, bt is still the "owner" and stops it.

For the edge case where instance A starts Dolt and instance B attaches, then A quits:
- A's StopIfOwned sees PID matches -> stops Dolt
- B's poll loop detects disconnect -> triggers auto-reconnect (see below)
- B runs EnsureServer, starts a new server, resumes polling

True reference counting adds complexity for a rare scenario. Auto-reconnect handles it cleanly without needing shared state between bt instances.

### Phase 2: Wire Into Startup (cmd/bt/main.go)

Replace the current hard-exit on `ErrDoltRequired` (line 1354) with the EnsureServer flow.

```
Current (main.go:1350-1361):
  result, err := datasource.LoadIssuesWithSource("")
  if errors.Is(err, datasource.ErrDoltRequired) {
      fmt.Fprintln(os.Stderr, "Dolt server not running. Start it with: bd dolt start")
      os.Exit(1)
  }

New:
  result, err := datasource.LoadIssuesWithSource("")
  if errors.Is(err, datasource.ErrDoltRequired) {
      // beadsDir is already resolved earlier via loader.GetBeadsDir("")
      serverState, startErr := doltctl.EnsureServer(beadsDir)
      if startErr != nil {
          fmt.Fprintf(os.Stderr, "Failed to start Dolt server: %v\n", startErr)
          os.Exit(1)
      }
      // Retry data load now that server is running
      result, err = datasource.LoadIssuesWithSource("")
      if err != nil {
          fmt.Fprintf(os.Stderr, "Dolt connected but failed to load issues: %v\n", err)
          serverState.StopIfOwned()
          os.Exit(1)
      }
      // Pass serverState to Model for cleanup on exit
  }
```

### Phase 3: Wire Into Shutdown (pkg/ui/model.go)

Add `doltServer *doltctl.ServerState` field to Model. In `Model.Stop()`:

```go
func (m *Model) Stop() {
    if m.backgroundWorker != nil { m.backgroundWorker.Stop() }
    if m.watcher != nil { m.watcher.Stop() }
    if m.instanceLock != nil { m.instanceLock.Release() }
    // NEW: stop Dolt server if bt started it
    if m.doltServer != nil {
        if err := m.doltServer.StopIfOwned(); err != nil {
            log.Printf("WARN: failed to stop Dolt server: %v", err)
        }
    }
}
```

Also register in the signal handler (main.go:4963-4984) so SIGTERM/crash paths still clean up.

### Phase 4: Update Port Discovery (internal/datasource/metadata.go)

Update `ReadDoltConfig()` to be the single source of truth for port resolution. Both the datasource layer and `doltctl.EnsureServer` call this function - no duplicate logic.

```
Current priority:
  1. .beads/dolt/config.yaml listener.port
  2. .beads/dolt-server.port file (overrides)
  3. Default: 3307

New priority:
  1. BEADS_DOLT_SERVER_PORT env var (highest - matches beads upstream)
  2. BT_DOLT_PORT env var (bt-specific override)
  3. .beads/dolt-server.port file
  4. .beads/dolt/config.yaml listener.port
  5. Default: 3307
```

Note: shared server mode (port 3308) is omitted. If `BEADS_DOLT_SHARED_SERVER=1` is set, `bd dolt start` handles that internally and writes the correct port to the port file. bt doesn't need to special-case it.

### Phase 5: Auto-Reconnect in Poll Loop (pkg/ui/background_worker.go)

Wire EnsureServer into the poll loop's failure path so bt self-heals when the Dolt server dies mid-session.

The BackgroundWorker gets a mutable `*doltctl.ServerState` pointer (shared with Model for shutdown awareness). Since the poll loop goroutine and Model.Stop() can both access ServerState, guard mutations with a `sync.Mutex` on the ServerState struct.

**On consecutive poll failures:**
1. After 3 failures (~15s at base interval): show "Reconnecting..." in status bar via `DoltConnectionStatusMsg`
2. Call `doltctl.EnsureServer()` - detects server is down, starts a new one
3. If succeeds: update ServerState (bt now owns the new server), reset backoff, resume polling, show "Reconnected"
4. If fails: continue exponential backoff, retry EnsureServer every 30s
5. After 5 failed EnsureServer attempts (~2.5min): show "Dolt unavailable" and stop retrying automatically. User can press R to retry (bt-ztrz keybind).

This handles all mid-session death scenarios:
- External server dies -> bt starts its own -> resumes
- bt's own server crashes -> bt starts a new one -> resumes
- Another bt instance kills the server -> this instance starts a fresh one -> resumes
- Dolt is genuinely broken -> bt gives up and tells the user

### Phase 6: Drop Dead Code

Remove the idle monitor keepalive that has no consumer since beads v0.59:

- `pkg/ui/background_worker.go`: Remove `touchDoltActivity()` method (lines 2022-2028)
- `pkg/ui/background_worker.go`: Remove `w.touchDoltActivity()` call in `doltPollOnce` (line 2043)
- Keep the `beadsDir` field on WorkerConfig (still needed for port file path in future reconnect scenarios)

### Phase 7: Database Identity Check

After connecting, verify we're talking to the right Dolt database (addresses SpecFlow Gap 11):

```go
// In datasource.NewDoltReader() or discoverDoltSources()
rows, err := db.Query("SHOW TABLES LIKE 'issues'")
if err != nil || !rows.Next() {
    return nil, fmt.Errorf("connected to port %d but no 'issues' table found - wrong database?", port)
}
```

This prevents silently connecting to Gas Town's server or another MySQL service on the expected port.

## Technical Considerations

**beads won't kill bt's server**: `DoltStore.Close()` only stops servers that it auto-started via `EnsureRunning`. When an agent's `bd` command finds bt's server already running (port file exists, TCP dial succeeds), it connects as a client and `Close()` leaves it alone. This is the same pattern that protects Gas Town's server. (see brainstorm: Approach C rationale)

**Orphaned servers on bt crash** (SpecFlow Gap 2): If bt is SIGKILLed, `StopIfOwned` never runs. The Dolt server stays alive. This is acceptable because:
- beads' `KillStaleServers` (scoped to repo) cleans orphans
- Next `bd dolt start` is idempotent - detects the existing server
- Next bt launch detects the running server and attaches (StartedByBT=false)
- The server consumes minimal resources when idle

**Rapid restart race** (SpecFlow Gap 7): If bt quits and `bd dolt stop` is still executing when bt relaunches, the new bt will either find the server still up (connects) or find it down (starts a new one). Both paths work.

**Beads version compatibility** (SpecFlow Gap 16): We target beads v0.59+ only. Pre-v0.59 users who still have persistent servers will see bt detect their running server and attach (StartedByBT=false). No conflict.

**`bd dolt start` output parsing**: The output format is stable: `"Dolt server started (PID %d, port %d)"`. We parse this for the PID. If parsing fails, fall back to reading `.beads/dolt-server.pid` and `.beads/dolt-server.port` files.

## System-Wide Impact

- **Interaction graph**: Startup gains a subprocess call (`bd dolt start`). Shutdown gains another (`bd dolt stop`). Poll loop gains auto-reconnect via EnsureServer. `DoltConnectionStatusMsg` gains a "Reconnecting" semantic (existing message type, new status text).
- **Error propagation**: `bd dolt start` failure surfaces as a startup error before the TUI launches. `bd dolt stop` failure is logged but doesn't block exit.
- **State lifecycle risks**: The `btStartedServer` flag is in-memory only. If bt crashes, no persistent state is corrupted. The Dolt server just stays alive (harmless).
- **API surface parity**: No CLI or robot-mode changes needed. Robot mode doesn't use the poll loop.

## Acceptance Criteria

### Functional Requirements

- [ ] bt auto-starts Dolt when no server is running, with visible "Starting Dolt server..." message
- [ ] bt connects to an existing Dolt server without trying to start a new one
- [ ] bt stops Dolt on exit only if bt started it (PID verification)
- [ ] bt does NOT stop Dolt if another process started it (Gas Town, another bt instance, manual `bd dolt start`)
- [ ] Port discovery follows the 5-level priority chain (env vars > port file > config > default)
- [ ] bt fails with clear error if `bd` CLI is not on PATH
- [ ] `.beads/dolt-server.activity` keepalive writes are removed
- [ ] Database identity check prevents connecting to wrong service
- [ ] Agent `bd` commands don't kill bt's server (beads' internal "who started it" tracking)

### Non-Functional Requirements

- [ ] Startup with server already running: < 1s additional latency
- [ ] Startup with auto-start: < 15s (includes Dolt boot time)
- [ ] Shutdown with stop: < 5s
- [ ] No orphaned processes under normal operation (graceful quit, SIGTERM)

### Testing

- [ ] `go test ./internal/doltctl/...` - unit tests for port discovery, output parsing
- [ ] Manual test: bt with no server running -> auto-starts -> shows data
- [ ] Manual test: bt with server already running -> attaches -> shows data -> quit doesn't kill server
- [ ] Manual test: bt auto-starts -> quit -> server stops
- [ ] Manual test: two bt instances -> first quits -> second auto-reconnects (starts new server)
- [ ] Manual test: bt running + `bd list` in another terminal -> both work, server survives

## Dependencies & Risks

- **Dependency**: `bd dolt start` command must exist and produce a persistent server (confirmed in beads v0.61)
- **Risk**: beads could change `bd dolt start` output format - mitigated by fallback to PID/port files
- **Risk**: beads could remove `bd dolt start` entirely - low probability, it's the official external tool path
- **Risk**: Multi-instance PID check has a race window - acceptable for v1, can add reference counting later

## Implementation Order

1. `internal/datasource/metadata.go` - update port priority chain (single source of truth)
2. `internal/doltctl/doltctl.go` - new module (EnsureServer calls ReadDoltConfig, StopIfOwned, bd availability check)
3. `cmd/bt/main.go` - replace hard-exit with EnsureServer + retry
4. `pkg/ui/model.go` - add doltServer field, wire into Stop()
5. `pkg/ui/background_worker.go` - auto-reconnect via EnsureServer on poll failures
6. `pkg/ui/background_worker.go` - remove touchDoltActivity
7. `internal/datasource/dolt.go` or `source.go` - add database identity check
8. Tests + manual verification

## Sources & References

- **Origin brainstorm:** [docs/brainstorms/2026-03-16-dolt-lifecycle-adaptation.md](docs/brainstorms/2026-03-16-dolt-lifecycle-adaptation.md) - key decisions: Approach C hybrid, bd for writes, server awareness not lifecycle ownership, drop keepalive, wasteland separate data source
- **ADR spine:** [docs/adr/001-btui-fork-takeover.md](docs/adr/001-btui-fork-takeover.md) - Stream 1 Dolt verification context
- **Beads issue:** bt-07jp (P1)
- **Related beads issues:** bt-tebr (auto-start, blocked by bt-07jp), bt-ztrz (manual reconnect keybind)
- **Beads changelog:** v0.59 (daemon removed), v0.60 (ephemeral ports, DoltStore.Close stops auto-started servers), v0.61 (port resolution fixes)
- **Key upstream code:** `internal/doltserver/doltserver.go` in beads - EnsureRunning, Start, port file management
- **Ecosystem context:** gastownhall org, Gas City (gascity) on port 3307, Wasteland on separate DB
