# AGENTS.md - beadstui

> Guidelines for AI agents working in this Go codebase. See `.claude/CLAUDE.md` for session workflow, ADR spine, and end-of-session protocol.

## Core Rules

1. **No file deletion** without explicit written permission. No exceptions.
2. **No destructive git** (`git reset --hard`, `git clean -fd`, `rm -rf`) without explicit approval. Use `git stash`/`git diff` first.
3. **No script-based code changes.** Make all edits manually or via parallel subagents.
4. **No file proliferation.** Edit existing files in place. No `_v2.go` / `_improved.go` variants. New files only for genuinely new functionality.
5. **Default branch is `main`.** Never reference `master` in code or docs.
6. **No backwards compatibility shims.** Early development, no users. Fix code directly.
7. **Verify after changes:** `go build ./...` and `go vet ./...` after any code change.

## Project Identity

- **Project**: beadstui (fork of beads_viewer, retargeted to upstream beads/Dolt)
- **Binary**: `bt`
- **Module**: `github.com/seanmartinsmith/beadstui`
- **Language**: Go 1.25+ (check `go.mod`)
- **TUI framework**: Charm Bracelet v1 (Bubble Tea, Lipgloss, Bubbles, Glamour)
- **Data backends**: JSONL, SQLite, Dolt (MySQL protocol)

## Key Directories

```
cmd/bt/              # CLI entry point (cobra)
pkg/ui/              # Bubble Tea model, update loop, views
pkg/analysis/        # Graph metrics, triage, planning
pkg/search/          # Hybrid semantic search (text + graph, FTS5)
pkg/model/           # Core data types
pkg/loader/          # JSONL parsing, bead loading
pkg/export/          # Static site export
pkg/agents/          # Agent detection (AGENTS.md filename hardcoded in 15 Go files)
pkg/correlation/     # Bead-to-commit correlation
pkg/watcher/         # Filesystem watching, daemon mode
internal/datasource/ # Data loading (JSONL, SQLite, Dolt)
internal/dolt/       # Dolt-specific reader
internal/models/     # Issue data structures
docs/adr/            # Architecture decision records (spine: 002-stabilize-and-ship.md)
tests/               # Cross-package integration / E2E tests
```

## Build & Test

```bash
go build ./...          # Build all
go test ./...           # All tests
go test ./... -race     # Race detector
go vet ./...            # Static analysis
go install ./cmd/bt/    # Install binary
```

## Key Design Constraints

- **Two-phase analysis**: Phase 1 (degree, topo sort, density) is instant. Phase 2 (PageRank, betweenness, HITS, eigenvector, cycles) runs async with 500ms timeout - check `status` flags in output.
- **Robot-first API**: All `--robot-*` flags emit deterministic JSON to stdout. Human TUI is secondary.
- **Elm architecture TUI** via bubbletea - all state transitions are message-based.
- **Pure-Go SQLite** (`modernc.org/sqlite`) for FTS5 search index - no CGO.
- **No raw prints in production** - TUI through lipgloss; robot mode outputs JSON to stdout; errors to stderr.
- **Error wrapping**: `fmt.Errorf("context: %w", err)` always.
- **Division safety**: Guard against divide-by-zero before computing averages/ratios.
- **Nil checks**: Check before dereferencing pointers, especially in graph traversal.
- **Browser safety**: All browser-opening functions gated by `BT_NO_BROWSER` / `BT_TEST_MODE` env vars.
- **Concurrency**: `sync.RWMutex` for shared state; capture channels before unlock to avoid races.

## Naming

- Binary: `bt`, Env vars: `BT_*`, CLI references: `bd` (beads CLI)
- Module: `github.com/seanmartinsmith/beadstui`, Data dir: `.bt/`
- AGENTS.md filename is hardcoded in `pkg/agents/` (15 Go files) - content can change, filename must stay.

## bt Robot Mode (for agents)

**CRITICAL: Use ONLY `--robot-*` flags. Bare `bt` launches an interactive TUI that blocks your session.**

```bash
bt --robot-triage                    # THE entry point: ranked recs, quick wins, blockers, health
bt --robot-next                      # Single top pick + claim command
bt --robot-plan                      # Parallel execution tracks
bt --robot-priority                  # Priority misalignment detection
bt --robot-insights                  # Full graph metrics
bt --robot-alerts                    # Stale issues, blocking cascades
bt --robot-diff --diff-since <ref>   # Changes since git ref
bt --robot-forecast <id|all>         # ETA predictions
bt --robot-suggest                   # Hygiene: duplicates, missing deps
```

Scoping: `--label <name>`, `--as-of <ref>`, `--recipe actionable|high-impact`

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:3216161c -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for persistent cross-session work. Use TaskCreate/TodoWrite for in-session execution tracking (3+ steps). They complement each other.
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for project-scoped knowledge. Use MEMORY.md for user preferences and cross-project patterns. They complement each other.

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->

## Workflow Formulas

This project has reusable workflow formulas at `~/.beads/formulas/`. Use them instead of ad-hoc execution.

```bash
bd formula list                    # See available formulas
bd mol pour <name> --var k=v       # Create persistent molecule (tracked work)
bd mol wisp <name> --var k=v       # Create ephemeral wisp (scratch/operational)
bd mol pour <name> --dry-run       # Preview what would be created
```

### Working through a molecule

Molecules create a parent issue with child steps linked by dependencies. `bd ready` surfaces the next unblocked step.

```bash
bd mol show <mol-id>               # See structure and steps
bd ready                           # Find next unblocked step
bd update <step-id> --claim        # Claim it
# ... do the work ...
bd close <step-id> --reason="..."  # Complete it, unblocks next step
bd mol burn <wisp-id>              # Clean up wisp when done (ephemeral only)
```

## Issue Tracking Conventions

- **Commit format**: `type(scope): description (bt-xxx)` - bead ref in parens when applicable
- **Creating issues**: Always include `--type`, `--priority`, `--labels`, `--description`. Valid labels: `.beads/conventions/labels.md`
- **Close format**: Summary, Change, Files, Verify, Risk, Notes
- Read `close_reason` before working a bead to avoid re-solving
- Only add blocking deps when work truly cannot start
