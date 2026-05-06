# AGENTS.md - beadstui

Project-canonical guidelines for AI agents working in this Go codebase.
Root `CLAUDE.md` imports this file via `@AGENTS.md`; it loads on every
session, so keep it tight.

## Core Rules

1. **No file deletion** without explicit written permission. No exceptions.
2. **No destructive git** (`git reset --hard`, `git clean -fd`, `rm -rf`) without explicit approval. Use `git stash`/`git diff` first.
3. **No script-based code changes.** Make all edits manually or via parallel subagents.
4. **No file proliferation.** Edit existing files in place. No `_v2.go` / `_improved.go` variants. New files only for genuinely new functionality.
5. **Default branch is `main`.** Never reference `master` in code or docs.
6. **No backwards compatibility shims.** Early development, no users. Fix code directly.
7. **Verify after changes:** `go build ./...` and `go vet ./...` after any code change.
8. **bd writes via bash command line corrupt non-ASCII** (Windows bash routes through cp1252). For ANY non-ASCII content (em-dashes, smart quotes, Unicode), use `bd close --reason-file <path>`, `bd update --body-file <path>`, `bd create --body-file <path>`, `bd comments add -f <path>`. Inline strings via `--reason=`/`-d` corrupt em-dashes to `â€"`. Default to ASCII (hyphens, straight quotes) when writing inline; switch to file-based for anything richer.

## Project Identity

- **Project**: beadstui (fork of beads_viewer, retargeted to upstream beads/Dolt)
- **Binary**: `bt`
- **Module**: `github.com/seanmartinsmith/beadstui`
- **Language**: Go 1.25+ (check `go.mod`)
- **TUI framework**: Charm Bracelet v2 (Bubble Tea, Lipgloss, Bubbles, Glamour)
- **Data backend**: Dolt (MySQL protocol). See [docs/design/beads-data-layer.md](docs/design/beads-data-layer.md) before touching the data layer, correlations, sprints, session columns, or git-history-derived features.

## Key Directories

```
cmd/bt/              # CLI entry point (cobra)
pkg/ui/              # Bubble Tea model, update loop, views
pkg/analysis/        # Graph metrics, triage, planning
pkg/search/          # Hybrid search: hash-based semantic embeddings + lexical boost; custom .bvvi vector index under .bt/semantic/
pkg/model/           # Core data types
pkg/view/            # CompactIssue projections (robot output)
pkg/loader/          # JSONL parsing, bead loading
pkg/export/          # Static site export
pkg/agents/          # Agent detection (filename pinned by agents.AgentsFileName constant)
pkg/correlation/     # Bead-to-commit correlation
pkg/watcher/         # Filesystem watching, daemon mode
internal/datasource/ # Data loading (Dolt, JSONL fallback)
internal/doltctl/    # Dolt server lifecycle
docs/                # See docs/README.md for layout map and decision tree
tests/               # Cross-package integration / E2E tests
```

## Build & Test

```bash
go build ./...          # Build all
go test ./...           # All tests
go test ./... -race     # Race detector
go vet ./...            # Static analysis
go install ./cmd/bt/    # Install binary (run after every build; bt is invoked from PATH)
```

## Scratch Conventions

| What | Where | Tracked? |
|---|---|---|
| Beads-context scratch (descriptions, close reasons, comments, audit findings) | `.beads/tmp/` | gitignored |
| General temp / non-beads scratch | `_tmp/` | gitignored |
| bt runtime caches (semantic index, baselines) | `.bt/` | partially gitignored |
| Per-agent worktrees | `.claude/worktrees/` | gitignored |

`.beads/tmp/` is for content that becomes part of bead state (drafts loaded with `--body-file`, comment text via `bd comments add -f`). `_tmp/` is everything else.

## Key Design Constraints

- **Two-phase analysis**: Phase 1 (degree, topo sort, density) is instant. Phase 2 (PageRank, betweenness, HITS, eigenvector, cycles) runs async with 500ms timeout - check `status` flags in output.
- **Robot-first API**: All `bt robot <subcmd>` invocations emit deterministic JSON to stdout. Human TUI is secondary.
- **Elm architecture TUI** via bubbletea - all state transitions are message-based.
- **No raw prints in production** - TUI through lipgloss; robot mode outputs JSON to stdout; errors to stderr.
- **Error wrapping**: `fmt.Errorf("context: %w", err)` always.
- **Division safety**: Guard against divide-by-zero before computing averages/ratios.
- **Nil checks**: Check before dereferencing pointers, especially in graph traversal.
- **Browser safety**: All browser-opening functions gated by `BT_NO_BROWSER` / `BT_TEST_MODE` env vars.
- **Concurrency**: `sync.RWMutex` for shared state; capture channels before unlock to avoid races.
- **Pure-Go SQLite** (`modernc.org/sqlite`, no CGO) is used only by the SQLite **export** artifact (`pkg/export/sqlite_export.go`). There is no SQLite at runtime. bt's own search index is a custom binary format (`.bvvi`) under `.bt/semantic/`.

## TUI modal compositing

Modal overlays use `OverlayCenterDimBackdrop` (in `pkg/ui/panel.go`) for
the dimmed-backdrop pop-up effect. Non-modal overlays use the non-dim
`OverlayCenter`. Step-by-step for adding a new modal:
[docs/design/tui-modal-compositing.md](docs/design/tui-modal-compositing.md).

## Naming

- Binary: `bt`, Env vars: `BT_*`, CLI references: `bd` (beads CLI)
- Module: `github.com/seanmartinsmith/beadstui`, Data dir: `.bt/`
- AGENTS.md filename is pinned via the `agents.AgentsFileName` constant in `pkg/agents/file.go`. Content can change; filename must stay.

## bt Robot Mode (for agents)

**CRITICAL: Use ONLY `bt robot <subcmd>` invocations. Bare `bt` launches an interactive TUI that blocks your session.**

Primary entry point: `bt robot triage` (ranked recs, quick wins,
blockers, health). Full reference with per-subcommand output shapes
and flags: [docs/robot/README.md](docs/robot/README.md). List
subcommands: `bt robot --help`. Common scoping flags: `--label <name>`,
`--as-of <ref>`, `--recipe actionable|high-impact`.

## Docs structure

Canonical doc map and "where does this go?" decision tree:
[docs/README.md](docs/README.md).

<!-- BEGIN BEADS INTEGRATION v:4 profile:full -->
## Issue Tracking

> This project uses [beads](https://github.com/gastownhall/beads) for task tracking.

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

### Session Rules

- Read close_reason before working a bead to avoid re-solving
- Check for abandoned work: `bd list --status=in_progress`
- Use `bd human <id>` for issues needing human decision
- Close beads before committing
- Don't invent labels - use `.beads/conventions/labels.md`
- Do NOT use `bd edit` - it opens $EDITOR. Use `bd update <id> --field "value"`
- Beads for cross-session persistence; tasks for within-session execution

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
