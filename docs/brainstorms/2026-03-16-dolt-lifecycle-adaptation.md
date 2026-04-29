---
date: 2026-03-16
topic: dolt-lifecycle-adaptation
---

# Dolt Lifecycle Adaptation + Ecosystem Vision

## What We're Building

beadstui (bt) is the TUI layer for the beads/Gas Town/Wasteland ecosystem. It should work for:

1. **Solo beads users** - individual developers using beads for issue tracking
2. **Gas Town operators** - monitoring 20-160 concurrent agents working on beads
3. **Wasteland participants** - visibility into cross-project bounties and reputation

Jeffrey Emanuel's beads_viewer provides the scaffold. The codebase is messy with dead code and unfinished features, but the bones are solid - Bubble Tea architecture, Dolt reader, dependency graph rendering. The work is to refine, not rewrite.

## Why This Brainstorm

Beads v0.59-0.61 fundamentally changed the Dolt server lifecycle:

- **v0.59**: Daemon infrastructure removed. `bd` is purely CLI-driven.
- **v0.60**: Ephemeral port allocation replaces hash-derived ports. Auto-start/stop per `bd` command. `DoltStore.Close()` stops the server. Shared server mode added (port 3308).
- **v0.61**: Port resolution chain fixes, credential storage changes.

This broke bt's assumption of a persistent Dolt server to poll against.

## The Ecosystem Stack

```
Wasteland  ->  separate DB (~/.local/share/wasteland/), Dolt CLI + DoltHub API, no server
Gas Town   ->  port 3307, persistent server, town-level orchestration
Beads      ->  per-project ephemeral port (or shared 3308), project-level issues
beadstui   ->  connects to beads' Dolt as read client, shells out to bd for writes
```

No port conflicts between layers. Each uses separate databases and ports.

## Chosen Approach: Hybrid (Approach C)

**Reads**: Direct SQL against Dolt server (current polling model, plus dolt_diff tables for history)
**Writes**: Shell out to `bd` CLI (create, update, close, etc.)
**Server lifecycle**: bt manages startup/shutdown awareness, not the server itself

### Startup Sequence

1. Read `.beads/dolt-server.port` - check if server already running (TCP dial)
2. If running (Gas Town user, or bd just ran): connect as client. Done.
3. If not running: shell out to `bd dolt start`, read port file, connect
4. Track whether bt started the server (`btStartedServer bool`)

### Shutdown Sequence

1. Close SQL connection pool
2. If `btStartedServer == true`: shell out to `bd dolt stop`
3. If bt did NOT start the server: leave it alone (don't kill Gas Town's server)

### Polling (Reads)

- Keep current model: `MAX(updated_at)` every 5s via direct SQL
- Drop `.beads/dolt-server.activity` keepalive writes (idle monitor is dead)
- DoltVerifiedMsg on every successful poll (freshness tracking unchanged)
- Exponential backoff on failure (unchanged)

### Writes (New - Interactive Editing)

- Shell out to `bd create`, `bd close`, `bd update` for all mutations
- Parse JSON output for confirmation / error handling
- After successful write, poll will pick up the change within 5s
- No direct SQL writes ever - bd owns data integrity and business logic

### Port Discovery (Updated)

Priority order (matching beads upstream):
1. `.beads/dolt-server.port` file (ephemeral, per-project)
2. `BEADS_DOLT_SERVER_PORT` env var
3. `BT_DOLT_PORT` env var (bt-specific override)
4. Shared server mode check -> port 3308
5. Fallback: 3307 (Gas Town default / legacy)

### Wasteland Visibility (Future)

- Separate data source: query `~/.local/share/wasteland/` Dolt database via CLI or direct Dolt read
- Surface "wanted" items related to the current project
- Show reputation/stamps for the current rig
- This is additive - doesn't affect the core beads Dolt connection

## Key Decisions

- **bt never writes to Dolt directly**: bd CLI is the stable write API. This insulates us from upstream schema changes in business logic (validation, hooks, commit semantics).
- **bt manages server awareness, not lifecycle**: we call `bd dolt start`/`bd dolt stop`, we don't run `dolt sql-server` ourselves. beads owns that complexity.
- **btStartedServer flag prevents killing others' servers**: critical for Gas Town coexistence.
- **Drop idle monitor keepalive**: the `.beads/dolt-server.activity` file has no consumer since v0.59.
- **Wasteland is a separate data source**: never mixed with beads Dolt connection.

## What This Doesn't Cover

- Codebase audit (planned separately - 8 domain teams, see docs/archive/plans/2026-03-12-codebase-audit-plan.md)
- Specific UI changes for interactive editing (needs its own brainstorm)
- Claude Code tasks viewer integration (parked for later)
- Release strategy / when to cut v0.0.1

## Resolved Questions

1. **Auto-start visibility**: Show status message ("Starting Dolt server...") so users know what's happening.
2. **Gas Town/Gas City indicator**: Yes, detect and show when connected to an existing Gas Town/Gas City server.
3. **Wasteland visibility**: Deferred, but tracked. Likely shell out to `wl` CLI (same principle as using `bd` for writes - let the tool own its data layer). See bt-wasteland-visibility in beads.
4. **Reconnect aggressiveness**: Current exponential backoff (5s -> 2min cap) is fine as starting point. Refine during implementation.

## Ecosystem Notes

- **steveyegge/beads** remains upstream for beads
- **gastownhall** org is the ecosystem hub (wasteland, gascity, marketplace, overwatch, etc.)
- **gastownhall/beads** is a fork of steveyegge/beads
- **gastownhall/gascity** is the new name/rebuild of Gas Town ("Orchestration-builder SDK for multi-agent coding workflows")
- Gas Town used port 3307, Gas City likely similar
- Other TUIs (bdui, lazybeads, tasc) appear inactive/broken post-Dolt migration - bt is likely the only maintained TUI

## Next Steps

-> `/ce:plan` for implementation of the Dolt connection changes (immediate fix)
-> Codebase audit (session A) to understand the scaffold before building on it
-> Separate brainstorm for interactive editing UX
