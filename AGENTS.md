# AGENTS.md - beadstui

> Canonical guidelines for AI agents working in this Go codebase. Single source of truth for project conventions, session workflow, and ADR spine. Root `CLAUDE.md` is a thin shim that imports this file via `@AGENTS.md`.

## Session Start - READ THIS FIRST

Before doing ANY work in this project, read the active ADR:

```
docs/adr/002-stabilize-and-ship.md
```

This is the spine document. It tracks:
- What decisions have been made vs what's still open
- Which work streams exist and their status
- Audit reports that inform each stream
- Open design decisions that block implementation

**Do not start implementation without checking the ADR.** If the user asks you to do something, orient against the ADR first to understand where it fits.

After completing significant work, update `CHANGELOG.md` and any relevant ADR-002 stream statuses.

> **Note on artifact locations**: bead descriptions filed in the 2026-04-27 cluster reorg reference scratch paths under `.bt/tmp/` (gitignored, project-scoped scratch convention). Durable canonical copies of those artifacts live at `docs/plans/2026-04-27-bt-cluster-reorg-proposal.md` and `docs/audit/2026-04-27-{bt-cluster-map,bd-surface-map,tui-productization-gap,writable-tui-design-surface}.md`. If a bead reference under `.bt/tmp/` is missing on a fresh checkout, read the corresponding `docs/` file with the same trailing name.

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
- **TUI framework**: Charm Bracelet v2 (Bubble Tea, Lipgloss, Bubbles, Glamour) — migration shipped via bt-ykqq / bt-k5zs / bt-zt9q, 2026-04-10
- **Data backend**: Dolt (MySQL protocol). Beads is Dolt-only since v1.0.1 (March 2026); JSONL/SQLite paths in bt code are pre-migration legacy. See "Beads architecture awareness" section below before touching the data layer.

## Key Directories

```
cmd/bt/              # CLI entry point (cobra)
pkg/ui/              # Bubble Tea model, update loop, views
pkg/analysis/        # Graph metrics, triage, planning
pkg/search/          # Hybrid search: hash-based semantic embeddings + lexical boost; custom .bvvi vector index under .bt/semantic/
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

## Beads architecture awareness (verified 2026-04-25)

Beads-the-tool (`bd`) migrated to Dolt-only storage in v1.0.1 (March 2026). Some bt code and bt beads predate this migration and assume the older JSONL-backed layout. **Before scoping or implementing any bead that touches data layer, correlations, sprints, session columns, or git-history-derived features, verify against current beads architecture rather than assumed prior state.** A systematic audit of all open bt beads against this reality is tracked in **bt-mhcv (P0)**.

### Current beads architecture

- **Storage**: Dolt is the only backend. JSONL export is opt-in for portability, not the system of record. The Dolt server data lives in `.beads/dolt/`.
- **Session columns**: `created_by_session`, `claimed_by_session`, `closed_by_session` are first-class columns on the `issues` table (upstream `0033_add_session_columns.up.sql`; Phase 1a merged 2026-04-24 via bd-34v). **NOT** sourced from the `metadata` JSON blob — that pattern is now stale code (bt-5hl9 tracks the bt-side migration).
- **Events**: Beads has a native `events` table (`Storage.GetEvents` at `internal/storage/storage.go:76`) with columns `id, issue_id, event_type, actor, old_value, new_value, comment, created_at`. This is the upstream primitive for bead-event audit trails.
- **History**: `bd history <id>` queries `dolt_history_issues` for per-commit issue snapshots (with full session columns per snapshot). Note: bd-3gb tracks an empty-result `--json` bug being PR'd upstream.
- **Sprints**: NOT a beads concept upstream — no `sprints` table or subcommand. Any sprint-related code in bt is a bt-only feature (tracked in bt-z5jj — rebuild against Dolt or retire).
- **Correlations**: NOT a beads concept upstream. Purely bt's domain (tracked in bt-08sh — migrate from JSONL+git-diff witness to `dolt_log` + `dolt_history_issues`).
- **Data dirs**: `.beads/` is shared with bd's Dolt server + bd metadata. `.bt/` is bt-only cache (baseline, semantic search index). The split is partly accidental and being canonicalized in bt-uahv.

### Stale-assumption checklist

When scoping or auditing any bt bead, ask:

- [ ] Does it assume `.beads/<project>.jsonl` exists? (Dolt-only installs don't produce one.)
- [ ] Does it assume `.beads/sprints.jsonl` exists? (Beads doesn't produce one — sprints aren't upstream.)
- [ ] Does it read session columns from the `metadata` blob? (Should read direct columns; bt-5hl9 tracks the migration.)
- [ ] Does it expect `--global` to fail for any single-ID lookup? (bt-vhn2 was misframed this way — actual root cause was the correlator, not routing.)
- [ ] Does its acceptance criteria reference pre-Dolt invariants? (Likely needs rescoping.)

If suspect, leave a comment with the recon finding rather than diving in. Cross-reference bt-mhcv for the systematic audit.

### Related beads

- **bt-mhcv** (P0) — systematic audit of all open bt beads
- **bt-08sh** (P2) — correlator Dolt migration
- **bt-z5jj** (P3) — sprint feature decision
- **bt-uahv** (P3) — `.beads/` vs `.bt/` canonical split
- **bt-5hl9** (P2) — CompactIssue session column migration
- **bd-3gb** (in beads repo) — bd history `--json` empty-result bug

## Key Design Constraints

- **Two-phase analysis**: Phase 1 (degree, topo sort, density) is instant. Phase 2 (PageRank, betweenness, HITS, eigenvector, cycles) runs async with 500ms timeout - check `status` flags in output.
- **Robot-first API**: All `--robot-*` flags emit deterministic JSON to stdout. Human TUI is secondary.
- **Elm architecture TUI** via bubbletea - all state transitions are message-based.
- **Pure-Go SQLite** (`modernc.org/sqlite`, no CGO) is used only by the SQLite **export** artifact (`pkg/export/sqlite_export.go`) — that output DB has FTS5 + materialized views. There is no SQLite at runtime. bt's own search index is a custom binary format (`.bvvi`) under `.bt/semantic/`, written by `pkg/search/vector_index.go`.
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

<!-- BEGIN BEADS INTEGRATION v:4 profile:full -->
## Issue Tracking

> This project uses [beads](https://github.com/steveyegge/beads) for task tracking.

**MANDATORY**: Read these files before creating, updating, or closing any issue:
1. `.beads/conventions/reference.md` - issue lifecycle, field triggers, close format
2. `.beads/conventions/labels.md` - valid label taxonomy

### Code Change Policy

Create a bead for anything changelog-worthy, any deliberate project activity, or any work persisting between sessions. Even single-session cleanup gets a bead if it's intentional project work. Skip beads only for truly trivial changes (typo fix, dep bump, lint).

### Commit Format

`type(scope): description (bt-xxx)` - bead ref in parens when applicable.

Scope maps to area labels: `cli`, `tui`, `bql`, `data`, `export`, `graph`, `search`, etc.

### Quick Reference

| Action | Command |
|---|---|
| Find work | `bd ready` |
| Read before work | `bd show <id>` |
| Claim | `bd update <id> --claim` |
| Complete | `bd close <id> --reason="..."` |
| Flag for human | `bd human <id>` |
| Session notes | `bd comments add <id> "..."` |
| Search | `bd search "query"` |
| Sync | `bd dolt push` |

### Creating Issues

Always include: `--type`, `--priority`, `--labels`, `--description`.
Use `--notes`, `--design`, `--acceptance` when trigger conditions apply
(see reference.md for triggers and decision tests).
Valid labels: `.beads/conventions/labels.md`

### Close Outcome Format

Use **literal newlines with blank lines** between fields:

```bash
bd close <id> --reason="Summary: ...

Change: ...

Files: ...

Verify: ...

Risk: ...

Notes: ..."
```

### Beads + Tasks

Beads for cross-session persistence. Tasks for within-session execution.

### Session Rules

- Read close_reason before working a bead to avoid re-solving
- Check for abandoned work: `bd list --status=in_progress`
- Use `bd human <id>` for issues needing human decision
- Close beads before committing
- Don't invent labels - use `.beads/conventions/labels.md`
- Do NOT use `bd edit` - it opens $EDITOR. Use `bd update <id> --field "value"`

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

## Planning Flow

1. Check ADR-002 for current state and open questions
2. Read relevant plan docs and audit reports before implementing
3. Flag anything that contradicts the ADR or plan — don't silently adapt

## End-of-Session Protocol (documentation side)

The BEADS INTEGRATION block above mandates the push mechanics. This section layers documentation updates on top.

Before ending a session where significant work was done:

1. **Update `CHANGELOG.md`** — add a session entry summarizing what was done
2. **Update ADR-002 stream statuses** — if a stream's status changed, reflect it
3. **Record new open questions** — anything discovered that needs a decision
4. **Update auto-memory** — if project state changed materially, update `~/.claude/projects/<this-project>/memory/MEMORY.md`

If the session is ending abruptly (context limits, user stopping), at minimum do step 1 — a changelog entry is the bare minimum handoff.

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
