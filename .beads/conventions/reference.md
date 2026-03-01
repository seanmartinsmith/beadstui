# Beads Quality Reference

Detailed conventions for this project. AGENTS.md has the summary; this file
has the depth.

## Close Outcome Template

```
Summary: <one sentence>
Change: <what changed>
Files: <paths>
Verify: <how verified>
Risk: <if any>
Notes: <optional gotchas>
```

**Formatting:** Use blank lines between sections. No semicolons.

**Good example:**

```
Summary: Fixed silent fallback to stale SQLite when Dolt server is down

Change: Added ErrDoltRequired sentinel and RequireDolt gate in DiscoverSources().
When metadata.json says backend: dolt, discovery skips SQLite/JSONL entirely.
If Dolt unreachable, returns clear error instead of falling back to stale data.

Files: internal/datasource/source.go, internal/datasource/load.go,
internal/datasource/source_test.go

Verify: go test ./internal/datasource/... (34 tests pass).
Stop Dolt, run bt - get clear error message instead of stale data.

Risk: None - legacy SQLite/JSONL path preserved for non-Dolt projects.

Notes: Upstream beads v0.56+ removed SQLite/JSONL entirely.
The beads.db file is a dead migration artifact.
```

**Bad examples:**
- "Done" - no context, unsearchable
- "Fixed it" - no details for future agents
- "See commit abc123" - commits get lost, close reasons persist

## Creating Issues

Always include: `--type`, `--priority`, `--labels`, `--description`.

Priority: 0 (critical) to 4 (backlog). Use numbers, not words.
Search before creating: `bd search "query"`

Labels must come from `.beads/conventions/labels.md`. Do not invent new labels.

## Dependencies

- Only add blocking deps when work truly cannot start
- Default to NOT adding blocking deps
- Use `bd dep relate <a> <b>` for connected but parallel work
- Never use `blocks` to mean "belongs to epic" - use parent-child

## Session Discipline

- `bd show <id>` and read close_reason before starting work
- Check `bd list --status=in_progress` at session start
- Use `bd comments add <id> "..."` for session notes
- Close beads before committing
- Run `bd dolt push` before ending session (if remote configured)

## Beads + Ephemeral Tasks

Beads for persistence across sessions. Tasks for within-session execution.

- **Beads** own the "what" - the work item, its context, decisions, and outcome
- **Tasks** (Claude Code TaskCreate/TaskUpdate) own the "how" - implementation
  steps within a single session

**Pattern:**
1. Create a bead for the work item (or claim an existing one)
2. Use Tasks to break down implementation steps (> 3 steps)
3. Mark tasks completed as you go
4. Close the bead when the work ships

**When to create a bead:** Would it show up in a changelog? Will you need
context in a future session? Is there a decision worth recording?

**When to use tasks only:** Purely mechanical within-session work where the
bead already captures the intent (e.g., "fix 5 stale references" - one bead,
5 tasks).

## Connecting Plans, Docs, and Commits

- In docs: `<!-- Related: bv-xxx -->` near the top
- In beads: `bd comments add <id> "Doc: path/to/doc.md"`
- In commits: `type(scope): description (bv-xxx)` when the commit directly addresses a bead
- In plans: Reference bead IDs the plan addresses
