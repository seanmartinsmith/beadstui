# beadstui

## Session Start - READ THIS FIRST

Before doing ANY work in this project, read the active ADR:

```
docs/adr/001-btui-fork-takeover.md
```

This is the spine document for the fork takeover. It tracks:
- What decisions have been made vs what's still open
- Which work streams exist and their dependencies
- What's already been implemented
- Where the last session left off (check the Changelog at the bottom)

**Do not start implementation without checking the ADR.** If the user asks you to do something, orient against the ADR first to understand where it fits.

After completing significant work, update the ADR's Changelog section and any relevant status fields.

## What This Is

Fork of Jeffrey Emanuel's beads_viewer, retargeted to upstream beads (Go/Dolt) instead of beads_rust.

**Naming**: Project is `beadstui`, binary is `bt`, module is `github.com/seanmartinsmith/beadstui`.

## Architecture

- **Language**: Go (1.25+)
- **TUI framework**: Charm Bracelet (Bubble Tea, Lipgloss, Bubbles, Glamour)
- **Data backends**: JSONL, SQLite, Dolt (MySQL protocol)
- **Binary**: `bt`
- **Module**: `github.com/seanmartinsmith/beadstui`

### Key Directories

- `cmd/bt/` - main entry point
- `pkg/ui/` - Bubble Tea model, update loop, views
- `internal/datasource/` - data loading (JSONL, SQLite, Dolt)
- `internal/dolt/` - Dolt-specific reader
- `internal/models/` - issue data structures
- `docs/adr/` - architecture decision records

## Workflow Conventions

### Cross-Session Persistence
- **ADRs** (`docs/adr/`) for architectural decisions and living project tracking
- **Beads issues** for work items that span sessions
- **Commits** reference relevant beads issue IDs when applicable

### Within-Session Tracking
- Use Claude Code tasks (TaskCreate/TaskUpdate) for anything > 3 steps
- Mark tasks in_progress before starting, completed when done

### Planning Flow
1. Check ADR-001 for current state and open questions
2. Read relevant plan docs before implementing
3. Flag anything that contradicts the ADR or plan - don't silently adapt

## End-of-Session Protocol

Before ending a session where significant work was done:

1. **Update ADR-001 Changelog** - add a dated entry summarizing what was done
2. **Update stream statuses** - if a stream's status changed, reflect it in the ADR
3. **Record new open questions** - anything discovered that needs a decision
4. **Link new plans** - if a plan doc was created, add it to the Related Plans section
5. **Update auto-memory** - if project state changed materially, update MEMORY.md

If the session is ending abruptly (context limits, user stopping), at minimum do step 1 - a changelog entry is the bare minimum handoff.

## Build & Test

```bash
go build ./cmd/bt/          # build
go test ./...               # all tests
go install ./cmd/bt/        # install binary
```

## Naming

Rename complete (Stream 2). The codebase uses:
- Binary: `bt`
- Env vars: `BT_*`
- CLI references: `bd` (the beads CLI)
- Module: `github.com/seanmartinsmith/beadstui`
- Data dir: `.bt/`

Note: AGENTS.md filename is hardcoded in `pkg/agents/` (15 Go files) - content was rewritten but filename must stay `AGENTS.md`.
