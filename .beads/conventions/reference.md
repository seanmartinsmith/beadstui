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

**Why each field matters:**

| Field | Purpose |
|---|---|
| Summary | Search target - what `bd search` finds |
| Change | Scope - what was modified, blast radius |
| Files | Navigation - where to look |
| Verify | Confidence - how to confirm fix holds |
| Risk | Landmines - what could break |
| Notes | Gotchas - the 30-minute discovery saved for next agent |

**Formatting:** Use **literal newlines with blank lines** between sections in --reason.

**Good example:**
```bash
bd close bt-8frs --reason="Summary: Fixed URL injection in browser-open on Windows

Change: Replaced cmd /c start with rundll32 url.dll,FileProtocolHandler in github.go and cloudflare.go, matching safe pattern already in model_export.go

Files: pkg/export/github.go:610, pkg/export/cloudflare.go:347

Verify: go build ./cmd/bt/ succeeds. URLs with & or | metacharacters no longer escape to shell.

Risk: None - rundll32 pattern already proven in model_export.go

Notes: See security/260409-2034-stride-owasp-full-audit/findings.md Finding 1"
```

**Minimum close reason (the floor):** Summary + Change + Files. Verify, Risk,
Notes added for non-trivial changes.

## Creating Issues

Always include: `--type`, `--priority`, `--labels`, `--description`.
Search before creating: `bd search "query"`

Labels must come from `.beads/conventions/labels.md`. Do not invent new labels.

## Description Structure by Type

### Bug

```markdown
## Bug
<what's broken, observed behavior vs expected>

## Steps to Reproduce (if known)
<numbered steps>

## Root Cause (if known)
<why it happens, file paths, line numbers>

## Acceptance Criteria
<what "fixed" looks like>
```

**Decision test:** Can another agent reproduce this from the description alone?

### Feature

```markdown
## Why
<why this matters, what problem it solves>

## What
<what needs to happen, concrete deliverables>

## Scope (if applicable)
<boundaries, what's in/out, design decisions>

## Acceptance Criteria
<what "done" looks like>
```

**Decision test:** Could two agents disagree on what to build?

### Task

```markdown
## Why
<why this work needs doing>

## What
<concrete deliverables or steps>

## Acceptance Criteria
<what "done" looks like>
```

### Epic

```markdown
## Why
<why this body of work matters>

## What
<what the epic encompasses, child work streams>

## Success Criteria
<what "done" looks like for the whole epic>
```

### Chore

Same as Task. For non-user-facing maintenance work.

## Priority Semantics

This is a CLI/TUI tool in active development (pre-release, single maintainer).

| Priority | Meaning | Example |
|---|---|---|
| P0 | Data loss or blocks all work | Dolt corruption, bt won't start |
| P1 | Core command/view broken | TUI crashes, robot mode outputs garbage |
| P2 | Meaningful improvement or confusing behavior | Missing feature, UX bug, performance |
| P3 | Minor friction or polish | Visual tweaks, better error messages |
| P4 | Backlog / someday-maybe | Research spikes, nice-to-haves |

## Create Fields Beyond --description

| Field | Trigger | Decision Test |
|---|---|---|
| `--description` | Always | -- |
| `--notes` | Discovered something non-obvious | "Would a future agent hit this surprise?" |
| `--design` | Chose between alternatives | "Is there a 'why not the other way?' question?" |
| `--acceptance` | Done-when isn't obvious from title | "Could two agents disagree on completeness?" |
| `--labels` | Always | Use `.beads/conventions/labels.md` |

## Enriching Beads During Work

| Moment | Action | Instead of |
|---|---|---|
| Made a design decision | `bd update <id> --design "chose X because Y"` | Commenting |
| Found an edge case | `bd update <id> --append-notes "edge case: Z"` | Commenting |
| Realized done criteria changed | `bd update <id> --acceptance "new criteria"` | Commenting |
| Created a related doc | `bd update <id> --spec-id "docs/..."` | Commenting |

## When to Use Comments vs Fields

> **Decision test:** Would this information still be useful if all comments were
> deleted? If yes - it belongs in a structured field, not a comment.

Comments are right for: session handoffs, questions/blockers, progress checkpoints, external references, post-close addenda.

## Dependencies

- Only add blocking deps when work truly cannot start
- Default to NOT adding blocking deps
- Use `bd dep relate <a> <b>` for connected but parallel work
- Never use `blocks` to mean "belongs to epic" - use parent-child

## Session Discipline

- `bd show <id>` and read close_reason before starting work
- Check `bd list --status=in_progress` at session start
- Use `bd human <id>` for issues needing human decision (don't invent ad-hoc patterns)
- Close beads before committing
- Run `bd dolt push` before ending session

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

**When to create a bead:** Would it show up in a changelog? Is it a deliberate
project activity? Will you need context in a future session? Is there a decision
worth recording? Even single-session cleanup (like a bead quality pass) gets a
bead if it's a deliberate project activity.

**When to skip a bead:** Truly trivial - typo in one file, dependency version bump,
formatting/lint fix. If you wouldn't mention it in a standup, it doesn't need a bead.

## Connecting Plans, Docs, and Commits

- In docs: `<!-- Related: bt-xxx -->` near the top
- In beads: `bd comments add <id> "Doc: path/to/doc.md"`
- In commits: `type(scope): description (bt-xxx)` when the commit directly addresses a bead
- In plans: Reference bead IDs the plan addresses
