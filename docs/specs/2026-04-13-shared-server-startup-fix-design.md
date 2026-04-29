# Design: Shared Dolt Server Startup Fix

**Date:** 2026-04-13
**Beads:** bt-zsy8, bt-4ckk (root cause is same - startup sequencing)
**Cross-project:** bd-zsy8 (beads-side lifecycle investigation, findings complete)
**Epic:** bd-mh6 (cross-project beads OS)
**Status:** Approved design, ready for implementation plan

## Problem

When bt boots and the shared Dolt server port file exists (`~/.beads/shared-server/dolt-server.port`) but the server isn't running, bt's auto-global path fails and falls through to a local project fallback that creates cascading bugs:

1. **Ghost Dolt process** - `doltctl.EnsureServer()` runs `bd dolt start` which starts a second dolt process. The shared server may already own the port (or starts later), so the ghost process loads the full DB engine (~470MB) but can't bind, sitting idle.
2. **Wrong datasource type** - bt ends up with `SourceTypeDolt` (per-project) instead of `SourceTypeDoltGlobal`. The poll loop uses the wrong reader type.
3. **Staleness indicator** - The misconfigured poll loop either silently fails or doesn't properly verify, causing the STALE indicator to appear after 2 minutes.

### Root cause trace

```
root.go:211  DiscoverSharedServer() → finds port file (3308) → OK
root.go:213  NewGlobalDataSource(host, port) → builds global DSN
root.go:214  LoadFromSource(globalSource) → NewGlobalDoltReader → db.Ping() → REFUSED
root.go:222  Warning printed, fall through (appCtx.selectedSource stays nil)
root.go:228  len(appCtx.issues) == 0, so local fallback runs
root.go:229  LoadIssuesWithSource("") → DiscoverSources → finds .beads/ Dolt config
root.go:230  Returns ErrDoltRequired
root.go:235  doltctl.EnsureServer() → runs bd dolt start → STARTS GHOST PROCESS
root.go:240  LoadIssuesWithSource("") retries → connects to port 3308 → loads issues
root.go:249  appCtx.selectedSource = SourceTypeDolt (WRONG - should be DoltGlobal)
root.go:433  NewModel gets SourceTypeDolt → poll loop uses doltPollOnce not globalDoltPollOnce
```

### Confirmed by testing

- Killed both dolt processes, ran `bd dolt start`, restarted bt → no staleness, proper global mode
- Two dolt.exe processes visible in btop when bug triggers (8MB shared server + 470MB ghost)
- `netstat` showed only one LISTENING on 3308, zero ESTABLISHED connections from bt during staleness

## Solution: Approach A - Auto-global starts the server itself

When auto-global finds the port file but the server is down, start the shared server by shelling out to `bd dolt start` and retry the global connection.

### New flow

```
DiscoverSharedServer() finds port file
  → LoadFromSource(global) fails (server down)
  → Shell out to `bd dolt start` (bd knows the project's server mode from its config)
  → Retry LoadFromSource(global)
  → If succeeds: SourceTypeDoltGlobal (correct), poll loop works
  → If still fails: print warning + fall through to local (safety net)
```

### Why this approach

- **Fixes both bugs** (staleness + ghost process) with one change in one location
- **Aligns with beads architecture** - bd-zsy8 investigation confirmed: "Don't manage the server yourself. Shell out to `bd`, let auto-start handle it."
- **Forward-compatible with CRUD** - bt will shell out to `bd` for writes anyway, so `bd` availability is already a dependency
- **Matches bd's lifecycle model** - auto-starts on demand, never auto-stops. Server persists across commands. No ownership complexity.

### What changes

**`cmd/bt/root.go:221-225`** - Replace the warning-and-fallthrough with a start-and-retry helper:

```go
// BEFORE (broken):
} else {
    if !envRobot {
        fmt.Fprintf(os.Stderr, "Warning: shared Dolt server found but load failed (%v)...\n", loadErr)
    }
}

// AFTER (fixed):
} else {
    // Shared server configured but not running - start it
    if !envRobot {
        fmt.Fprintln(os.Stderr, "Starting shared Dolt server...")
    }
    if startErr := startSharedDoltServer(); startErr == nil {
        // Retry global load
        retryResult, retryErr := datasource.LoadFromSource(globalSource)
        if retryErr == nil {
            appCtx.issues = retryResult
            appCtx.selectedSource = &globalSource
            appCtx.beadsPath = ""
            appCtx.workspaceInfo = buildWorkspaceInfoFromIssues(retryResult)
            appCtx.currentProjectDB = detectCurrentProjectDB()
        } else if !envRobot {
            fmt.Fprintf(os.Stderr, "Warning: started server but load still failed (%v), falling back to local\n", retryErr)
        }
    } else if !envRobot {
        fmt.Fprintf(os.Stderr, "Warning: could not start shared server (%v), falling back to local\n", startErr)
    }
}
```

**New helper function `startSharedDoltServer()`:**

```go
func startSharedDoltServer() error {
    bdPath, err := exec.LookPath("bd")
    if err != nil {
        return fmt.Errorf("bd CLI not found")
    }
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, bdPath, "dolt", "start")
    // Force shared server mode regardless of cwd - we know this is a shared
    // server setup because DiscoverSharedServer() found the port file.
    cmd.Env = append(os.Environ(), "BEADS_DOLT_SHARED_SERVER=1")
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("bd dolt start failed: %w\nOutput: %s", err, string(out))
    }
    // Wait for server to be ready (TCP dial retry)
    host, port, _ := datasource.DiscoverSharedServer()
    addr := fmt.Sprintf("%s:%d", host, port)
    deadline := time.Now().Add(10 * time.Second)
    for time.Now().Before(deadline) {
        conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
        if err == nil {
            conn.Close()
            return nil
        }
        time.Sleep(500 * time.Millisecond)
    }
    return fmt.Errorf("server started but not reachable after 10s")
}
```

### What doesn't change

- **Users without shared server** - `DiscoverSharedServer()` returns error (no port file), entire auto-global block is skipped. Local project path works exactly as before.
- **`--global` flag** - Still hard-errors if shared server is unreachable. Explicit intent means explicit failure.
- **Poll loop, background worker, datasource types** - No changes needed. They already work correctly when given the right datasource type at startup.
- **`doltctl.EnsureServer()`** - Still used in the local fallback path (line 235). Not modified.

### Working directory for `bd dolt start`

`bd dolt start` resolves server mode from the current project's `.beads/metadata.json`. If bt is launched from a non-beads directory (e.g., `~`), `bd` won't find a project config. The helper sets `BEADS_DOLT_SHARED_SERVER=1` on the subprocess environment to force shared mode regardless of cwd. This is safe because we only reach this code path when `DiscoverSharedServer()` already found the port file - confirming a shared server setup exists.

### Ghost process prevention

By starting the server in the auto-global path (before falling through to local), we avoid the scenario where `doltctl.EnsureServer` at line 235 starts a competing dolt process on the same port.

## Design decisions and rationale

### Why not validate the port file before trusting it?
Deleting files managed by `bd` feels wrong - bt shouldn't clean up beads state. And it would put bt in per-project mode instead of global, losing the multi-project view.

### Why not detect shared mode after local start?
More complex (two connection paths to reach the same result), still creates the ghost process, and adds switching logic that doesn't exist today.

### Why shell out to `bd` instead of starting dolt directly?
`bd` knows the project's server mode, data directory, port, and all the lifecycle management. bt reimplementing any of that creates a maintenance burden and divergence risk. The beads investigation explicitly recommends: "Don't manage the server yourself."

### Per-project vs global: clean separation
Per-project and shared servers cannot coexist for the same beadsDir (beads enforces this via `ResolveServerMode()`). bt's auto-global only triggers when the shared server port file exists. Users on local-only projects are completely unaffected. This is the right boundary - bt doesn't need to bridge the two modes.

### Local-to-shared migration
Not part of this spec. Filed as bt-bv3a. When bt detects a local-only project, it could suggest `bd init --shared-server` to the user, but that's a separate feature.

## Related work (not in scope)

| Bead | What | Why not in scope |
|------|------|-----------------|
| bt-6l2c | Poll loop dies after ~10 min in proper global mode | Different bug - poll loop works on restart, dies over time. Separate investigation. |
| bt-c69c | Expose bd human/gate commands in TUI | CRUD story - depends on understanding bd human (advisory label) vs bd gate await:human (blocking). Beads session investigated this. |
| bt-zdae | Status bar messages persist until manual interaction | UX polish, unrelated to server lifecycle |
| bt-d8ty | Scroll acceleration on issues list | UX polish, unrelated |
| bt-bv3a | Document local-to-shared migration path | Follow-up, depends on this fix shipping first |

## Cross-project context (from beads session bd-zsy8)

Server lifecycle findings that informed this design:

- **Auto-starts on demand, never auto-stops.** Server persists across commands. `cleanupStateFiles()` removes port/PID on stop. `IsRunning()` detects and cleans stale files on crash.
- **`BEADS_DOLT_SHARED_SERVER=1`** forces shared mode. Without it, `bd dolt start` resolves mode from the cwd's `.beads/metadata.json`.
- **Key files in beads:** `internal/doltserver/doltserver.go` (lifecycle), `internal/doltserver/servermode.go` (mode resolution), `internal/storage/dolt/store.go:972-1010` (auto-start trigger).

## Testing

1. **Kill shared server, run bt** - should see "Starting shared Dolt server...", then normal global mode, no staleness
2. **No port file, run bt** - should skip auto-global entirely, load local project normally
3. **`bt --global` with server down** - should hard-error (unchanged behavior)
4. **bd not installed** - should fall through to local with warning (safety net)
5. **Server start succeeds but load still fails** - should fall through to local with warning
