---
title: "Phase 1 dispatch template: parallel bug-bangout subagents"
type: dispatch
status: ready
date: 2026-04-27
parent: docs/plans/2026-04-27-bangout-arc.md
beads: bt-cl2m, bt-70cd, bt-nyjj, bt-foit
---

# Phase 1 dispatch template

Paste-ready Agent tool prompts for the Phase 1 quartet. Designed for one PM-style user session that dispatches 4 parallel worktree subagents, then aggregates.

## How to use

1. Run pre-flight (below) — single PM session, no subagents yet.
2. Dispatch the 4 subagents **in a single message with 4 parallel `Agent` tool calls**, each using `isolation: "worktree"`.
3. As each subagent returns, PM verifies, merges its branch into main, and pushes.
4. Run post-dispatch checklist.

Total expected wall time: 30-60 min for all 4 (vs ~2h sequential).

---

## Pre-flight (PM, single-thread)

Run these in the PM session before dispatching:

```bash
# Confirm clean working tree
git status -s

# Confirm bd state
bd list --status=in_progress
bd dolt status

# Pull latest
git pull --rebase
bd dolt pull

# Confirm tests are green BEFORE we start (so failures later are clearly ours)
go build ./... && go vet ./... && go test ./pkg/ui/
```

If any of those fail, fix or ask before dispatching.

---

## Dispatch (PM, parallel — single message, 4 Agent tool calls)

Send all four `Agent` tool calls in **one message** so they run concurrently. Each gets `isolation: "worktree"` so they work in isolated copies of the repo with their own git index.

### Subagent 1: bt-cl2m

```
description: bt-cl2m fix — modal-aware refresh
subagent_type: general-purpose
isolation: worktree
prompt: |
  You are a focused engineer fixing one bt bug. You have no prior chat context.

  ## Bug

  bt-cl2m (P2 bug): Background data refresh closes open modals in the TUI. When the
  user has a modal open (label picker, project picker, time-travel input, etc.) and
  the watcher fires a data refresh, the re-render closes the modal mid-interaction.
  Fix: guard refresh-driven re-renders so they don't fire while a modal is active.

  ## Project context (must read first)

  - C:\Users\sms\System\tools\bt\AGENTS.md — project conventions (especially core
    rules 1-7, build/test commands, no-deletion policy)
  - C:\Users\sms\System\tools\bt\pkg\ui\model.go (search for `activeModal` field
    and `ModalNone` const to understand modal state shape)
  - C:\Users\sms\System\tools\bt\pkg\ui\model_update_data.go (the refresh path)
  - C:\Users\sms\System\tools\bt\pkg\ui\model_update_input.go:1148 (existing usage
    of `m.activeModal != ModalNone` — copy this pattern)

  ## Implementation

  - Locate the refresh-driven re-render path in pkg/ui/model_update_data.go
  - Guard it with `m.activeModal != ModalNone` (NOT `m.modalActive()` — that
    method does NOT exist; if you reach for it, you're wrong)
  - If multiple refresh paths need the guard, factor a small helper (e.g.
    `m.shouldDeferRefresh()`) and call it consistently
  - Do NOT skip refresh entirely — defer it until modal closes (next user input
    triggers refresh) OR queue it. Pick the simpler path; do not over-engineer.

  ## Verify (mandatory per AGENTS.md rule 7)

  ```bash
  go build ./... && go vet ./... && go test ./pkg/ui/
  ```

  All three must be clean.

  ## Commit + report back

  1. Commit in your worktree with message format: `fix(tui): bt-cl2m guard refresh
     re-render when modal is active`
  2. Report back the branch name, files changed, and a one-paragraph summary of
     the approach. Do NOT push — PM will merge into main.

  ## Constraints

  - Do not delete any code without flagging it
  - Do not modify files outside the refresh-guard scope
  - If you discover the bug is not what's described, STOP and report — do not
    expand scope
  - Cap response at 300 words
```

### Subagent 2: bt-70cd

```
description: bt-70cd fix — robot subcommand exit code
subagent_type: general-purpose
isolation: worktree
prompt: |
  You are a focused engineer fixing one bt bug. You have no prior chat context.

  ## Bug

  bt-70cd (P2 bug): Unknown `bt robot` subcommand prints help to stdout and exits
  with code 0. This breaks pipes (agents that pipe `bt robot ...` into jq see help
  text mixed into their data) and breaks shell scripts that check $? for failure.
  Fix: configure cobra to write unknown-command errors to stderr and exit non-zero.

  ## Project context (must read first)

  - C:\Users\sms\System\tools\bt\AGENTS.md — project conventions
  - C:\Users\sms\System\tools\bt\cmd\bt\cobra_robot.go — the robot subcommand
    declarations
  - C:\Users\sms\System\tools\bt\cmd\bt\root.go — root command setup, cobra
    configuration (look for SilenceErrors, SilenceUsage, RunE patterns)

  ## Implementation

  - Configure cobra so unknown subcommands of `bt robot` write to stderr and
    return non-zero
  - Pattern: set `SilenceUsage: true` + `SilenceErrors: true` on the parent, then
    handle unknown commands explicitly OR use cobra's built-in unknown-command
    handling routed to stderr
  - The robot command tree is the most affected; verify the rest of the CLI still
    behaves correctly (don't break `bt --help` etc.)

  ## Verify (mandatory)

  ```bash
  go build ./... && go vet ./... && go test ./...

  # Behavioral checks
  bt robot bogus 2>/dev/null  # MUST be empty (error went to stderr)
  bt robot bogus; echo "exit=$?"  # MUST show non-zero exit
  bt --help  # MUST still work normally
  bt robot --help  # MUST still work normally
  ```

  All four behavioral checks must match expectation.

  ## Commit + report back

  1. Commit in your worktree: `fix(cli): bt-70cd unknown robot subcommand to
     stderr + non-zero exit`
  2. Report branch name, files changed, and the behavioral check outputs you saw.
  3. Do NOT push — PM will merge.

  ## Constraints

  - Don't break existing CLI behavior for valid commands
  - Don't modify the help text format itself, just the routing
  - Cap response at 300 words
```

### Subagent 3: bt-nyjj

```
description: bt-nyjj fix — git log error in non-git cwd
subagent_type: general-purpose
isolation: worktree
prompt: |
  You are a focused engineer fixing one bt bug. You have no prior chat context.

  ## Bug

  bt-nyjj (P2 bug, child of bt-19vp History view epic): When bt boots from a
  directory that is not inside a git repo (e.g. `cd ~ && bt`), the history view
  shows a red "git log failed" error banner. This is correct in that git did fail
  — but it shouldn't be presented as an error. The cwd just isn't a git repo;
  that's an expected condition, not a failure.

  Fix: detect "cwd is not in a git work tree" silently (no banner, history view
  shows a friendly empty-state). Reserve the red banner for actual git invocation
  failures (binary missing, permissions error, repo-corrupt, etc.).

  ## Detection mechanism

  Use `git rev-parse --is-inside-work-tree`:
  - Exit 0 + stdout "true" → inside a git repo, normal flow
  - Exit non-zero → not inside a repo, silent fallback (no error banner)

  Distinguish from real failures (e.g. `git` not on PATH) by checking the error
  type: missing binary or non-git-related error → red banner; "not a git
  repository" message → silent.

  ## Project context (must read first)

  - C:\Users\sms\System\tools\bt\AGENTS.md
  - pkg/correlation/ (search for `git log` invocations and the path that emits
    the "git log failed" banner)
  - pkg/ui/ (where the history view consumes correlation output and renders the
    banner)

  Trace from the user-facing red banner string back to its source. Then add the
  is-inside-work-tree probe upstream of the actual `git log` call.

  ## Implementation

  - Add a small helper (e.g. in pkg/correlation/) that returns
    `(insideRepo bool, err error)` based on `git rev-parse --is-inside-work-tree`
  - Caller checks this BEFORE running `git log`. If !insideRepo, return empty
    history with no error.
  - If the helper itself errors (binary missing, etc.), surface the real error.

  ## Verify (mandatory)

  ```bash
  go build ./... && go vet ./... && go test ./...

  # Behavioral checks
  cd ~ && bt  # navigate to history view — should see empty state, no red banner
  cd /path/to/git/repo && bt  # history view should still work normally
  ```

  ## Commit + report back

  1. Commit: `fix(history): bt-nyjj silent fallback when cwd is not in git repo`
  2. Report branch, files changed, behavioral check observations.
  3. Do NOT push — PM will merge.

  ## Constraints

  - Don't suppress real git errors; only the not-in-a-repo case is silent
  - Empty-state UX should match other empty states in bt (don't invent new style)
  - Cap response at 300 words
```

### Subagent 4: bt-foit

```
description: bt-foit fix — undocumented <> keys + label column alignment
subagent_type: general-purpose
isolation: worktree
prompt: |
  You are a focused engineer fixing one bt bug. You have no prior chat context.

  ## Bug

  bt-foit (P2 bug): Two related issues in the TUI list view:

  1. The `<` and `>` keys resize the list pane (vs detail pane) but are not
     documented in the help overlay or shortcuts sidebar. Users hit them
     accidentally and have no way to learn what they do.
  2. When the list pane is widened with `>`, the label column alignment breaks
     (columns drift, labels overflow, visual regression).

  Fix both.

  ## Project context (must read first)

  - C:\Users\sms\System\tools\bt\AGENTS.md
  - C:\Users\sms\System\tools\bt\docs\audit\keybindings-audit.md (reference for
    what's already documented vs not)
  - pkg/ui/model_keys.go (where < and > handlers live — grep for case "<", case ">")
  - pkg/ui/delegate.go (label column rendering — search for label-column width
    computation)
  - pkg/ui/model_view.go (help overlay content)
  - pkg/ui/shortcuts_sidebar.go (shortcuts sidebar content)

  ## Implementation

  ### Part 1: documentation

  - Add `<` and `>` (resize list pane) to the help overlay in pkg/ui/model_view.go
    (look for the existing pane-resize section or "Layout" section; if no such
    section, add it under a sensible heading)
  - Add the same to pkg/ui/shortcuts_sidebar.go
  - Match formatting of nearby entries (tone, casing, length)

  ### Part 2: alignment fix

  - Investigate the label column rendering in pkg/ui/delegate.go
  - When list pane width changes, the column-width math must rebalance — find
    the bug (likely a hardcoded width or stale cached width)
  - Fix without restructuring the delegate; minimal patch

  ## Verify (mandatory)

  ```bash
  go build ./... && go vet ./... && go test ./pkg/ui/

  # Behavioral checks
  bt  # in TUI:
  #   - press ?, confirm < and > are now documented
  #   - press > several times to widen list pane, verify label column stays
  #     aligned and labels render correctly
  #   - press < to revert
  ```

  ## Commit + report back

  1. Commit: `fix(tui): bt-foit document <> resize keys + fix label column
     alignment when list pane widens`
  2. Report branch, files changed, behavioral observations.
  3. Do NOT push — PM will merge.

  ## Constraints

  - Don't change the `<>` behavior itself, only document it and fix the
    alignment
  - Don't restructure the delegate or help overlay layout — minimal changes
  - Cap response at 300 words
```

---

## Post-dispatch (PM, sequential)

> **Updated 2026-04-27 with Phase 1 execution learnings.** The original `git merge --ff-only` sequence was wrong for parallel-from-baseline worktrees — only the first merge succeeds because the other branches share the same baseline ancestor (no fast-forward path). Use cherry-pick instead. See "Failure modes" below for full context.

After all 4 subagents return:

```bash
# 1. List the worktree branches each subagent created
git worktree list
git branch --list

# 2. CHECKOUT MAIN FIRST — subagent worktrees may have left a branch checked out
#    in the PM main worktree (Phase 1 learning: bt-foit's branch leaked).
git checkout main

# 3. Cherry-pick each subagent's commit into main (NOT ff-merge)
#    Why: subagent worktrees all branched from the same baseline main, so they're
#    siblings. ff-merge only works for the first one; the rest need cherry-pick.
git cherry-pick <branch-name-1>
git cherry-pick <branch-name-2>
git cherry-pick <branch-name-3>
git cherry-pick <branch-name-4>
# If files overlap (shouldn't with this quartet — verify before dispatch), expect
# conflicts. Resolve, `git cherry-pick --continue`.

# 4. Final verify on the merged main
go build ./... && go vet ./... && go test ./pkg/ui/

# 5. Push
git pull --rebase
git push
git status  # MUST show "up to date with origin"

# 6. Close beads
bd close bt-cl2m --reason "Summary: ... Change: ... Files: ... Verify: ... Risk: ... Notes: ..."
bd close bt-70cd --reason "..."
bd close bt-nyjj --reason "..."
bd close bt-foit --reason "..."
bd dolt push

# 7. Update CHANGELOG.md with a Phase 1 entry covering all 4 ships
# 8. Update ADR-002 Stream 6 recent completions

# 9. Clean up subagent worktrees + branches
#    Worktrees may stay locked-and-present after subagent return (Phase 1 learning).
#    Manual unlock + remove may be required. Then delete the branches.
git worktree list
git worktree unlock <worktree-path>  # if locked
git worktree remove <worktree-path>
git branch -D worktree-agent-<hex1> worktree-agent-<hex2> ...
git branch -D fix/bt-foit-pane-resize-docs-and-alignment  # any explicit branch names too
```

---

## Failure modes to watch for

> **Updated 2026-04-27 with Phase 1 execution observations.** Marked items below were observed on the first run.

| Symptom | Likely cause | Action |
|---|---|---|
| Subagent expands scope ("while I was in there I noticed...") | Prompt didn't constrain enough | Reject the diff, re-dispatch with tighter scope language |
| Two subagents both touch `pkg/ui/delegate.go` | bt-foit + something unexpected | Should not happen with this quartet, but if it does — sequence them, don't merge in parallel |
| Subagent reports "tests pass" but `go test` fails on PM merge | Subagent skipped a test, used `-run` filter, or has stale state | Run the full suite on PM side; reject and re-dispatch |
| Subagent fails to commit ("nothing staged") | Subagent forgot or hit an editor block | Have subagent paste their diff in the report; PM stages + commits |
| **OBSERVED: `--ff-only` merge fails after the first one** | Sibling branches from same baseline — only one can fast-forward | Use `git cherry-pick <branch>` for the second through Nth branches. Updated post-dispatch sequence above. |
| **OBSERVED: subagent branch leaked into PM main worktree** | bt-foit's branch was checked out in the main worktree even after subagent returned | `git checkout main` BEFORE attempting any merges. Add this as the first step. |
| **OBSERVED: worktrees stay locked and present after subagent return** | Auto-cleanup didn't fire for 3 of 4 worktrees | Manual `git worktree unlock` + `git worktree remove` + `git branch -D` per leftover. Add to cleanup checklist. |
| **OBSERVED: subagent reports `go install` blocked by safety hook** | Hook restricts `go install` for subagents | PM runs `go install` from main session after merge to do behavioral verify. Plan around this — subagent verifies via `go test`, PM verifies via behavioral run. |
| **OBSERVED: subagent gently corrects the dispatch prompt** | Prompt asserted something about the codebase that was wrong (e.g. "method X doesn't exist" when it does) | Subagent's correction is data; update the dispatch template to reflect new reality. Phase 1 example: cl2m subagent added `m.modalActive()` helper, so future beads can use it directly. |
| **OBSERVED: pre-existing test failures surface as subagent reports** | Subagents run tests on baseline main + their changes; failures present on baseline get reported | Confirm against `git stash && go test` on a clean main. If pre-existing, file a follow-up bead BEFORE proceeding with merges (Phase 1 example: bt-bnji filed for `TestDetectSigils_LinearScaling` flake). Don't ignore — they'll re-surface in every future dispatch session. |

---

## Success looks like

- 4 commits on main (one per bug, with `(bt-XXXX)` ref in subject)
- 4 closed beads with proper close-reason format (Summary/Change/Files/Verify/Risk/Notes)
- `git status` clean and up-to-date with origin
- CHANGELOG + ADR-002 updated
- Wall time ~30-60 minutes vs ~2 hours sequential
- Phase 2 ready to start in next session

After this, move to Phase 2 (bt-krwp design conversation) — no subagent dispatch, just you + me.
