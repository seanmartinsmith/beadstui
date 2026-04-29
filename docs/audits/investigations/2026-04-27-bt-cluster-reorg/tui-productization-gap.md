# bt TUI Productization Gap Analysis

**Date:** 2026-04-27
**Author:** assistant (subagent), feeding into Sean's roadmap reorg
**Scope:** Compare bt against best-in-class TUIs (lazygit, k9s, gh dash, helix, lazyjj, superfile) and identify productization gaps for both read-side (today) and write-side (bt-oiaj future).

---

## 1. bt's current TUI surface

### View modes (one-of, exclusive)
Defined at `pkg/ui/model.go:78-94`:

- `ViewList` — default issue list (with optional split detail pane)
- `ViewBoard` — kanban (Open / In Progress / Blocked / Closed columns), swimlane modes
- `ViewGraph` — dependency graph
- `ViewTree` — hierarchical (parent/child)
- `ViewActionable` — execution plan / ready work
- `ViewHistory` — git correlation (commits ↔ beads)
- `ViewSprint` — sprint dashboard
- `ViewInsights` — metrics panel (PageRank, betweenness, etc.)
- `ViewFlowMatrix` — cross-label dependency flow
- `ViewLabelDashboard` — label health table
- `ViewAttention` — attention scores

### Modal types
Defined at `pkg/ui/model.go:102-117`:

`ModalQuitConfirm`, `ModalRecipePicker`, `ModalBQLQuery`, `ModalLabelPicker`, `ModalRepoPicker`, `ModalTimeTravelInput`, `ModalAgentPrompt`, `ModalTutorial`, `ModalCassSession`, `ModalUpdate`, `ModalAlerts`, `ModalLabelHealthDetail`, `ModalLabelDrilldown`, `ModalLabelGraphAnalysis`.

That's 14 modal types, plus 11 view modes, plus a focus state machine (`focusList`, `focusDetail`, `focusBoard`, ... ~20 states in `model_modes.go:217-265`).

### Top-level view-toggle keys (from list focus)
From `pkg/ui/model_update_input.go:803-1217`:

| Key | Action |
|---|---|
| `b` | toggle Board |
| `g` | toggle Graph |
| `a` | toggle Actionable |
| `E` | toggle Tree |
| `i` | toggle Insights |
| `h` | toggle History |
| `[` / `f3` | Label dashboard |
| `]` / `f4` | Attention |
| `f` | Flow matrix |
| `!` | Alerts modal |
| `:` | BQL query modal |
| `'` | Recipe picker |
| `l` | Label picker |
| `w` | Project picker (workspace mode) OR wisp toggle (single-project) |
| `W` | Quick toggle "this project" vs "all projects" |
| `t` / `T` | Time-travel (custom rev / HEAD~5) |
| `o` / `c` / `r` / `a` | Filter toggles (open / closed / ready / all) |
| `s` / `S` | Sort cycle / reverse |
| `R` | Apply triage recipe |
| `x` | Export markdown |
| `O` | Open beads.jsonl in $EDITOR |
| `C` | Copy issue to clipboard |
| `y` | Copy ID to clipboard |
| `p` | Toggle priority hints |
| `V` | Cass session preview modal |
| `U` | Self-update modal |
| `/` | List filter (Bubbles list) |
| `<` / `>` | Resize split pane |
| `tab` | Toggle list/detail focus |
| `?` / `F1` | Help overlay (with embedded tutorial via Space) |
| `q` / `esc` | Close current view / quit confirm |

Per-view keys overlap and shadow each other. E.g.:
- `s` in list = sort cycle; in board = swimlane cycle; in graph = swarm toggle
- `g` in list = toggle graph view; in tree = jump to top; in history = jump to graph; in board = "gg" combo
- `c` in list = filter closed; in history = cycle confidence threshold
- `o` in list = filter open; in tree = expand all; in history = open commit in browser
- `h` in list = toggle history; in tree = collapse parent; in graph view also has special meaning

### What's writable today
**Nothing.** TUI is 100% read-only, per product vision doc lines 67-72: "Zero `exec.Command("bd", ...)` calls in the UI package for writes." The `O` key opens `beads.jsonl` in `$EDITOR` (legacy JSONL workflow, predates Dolt-only beads architecture).

### What's stubbed / partial
- Writable TUI: bt-oiaj, design decided, zero implementation
- Sprint view: pattern-violating (method-on-Model not standalone), per product vision
- Workspace mode: plumbing exists, repo picker is a single confused widget (per resolved question 6 in vision)
- Help: full overlay exists, plus context_help.go with per-context summaries; tutorial behind Space inside help. Not "type-to-discover."
- Empty/error/loading states: ad-hoc — handled by `setStatus()` strings and `m.statusIsError` flag; no dedicated pattern.

---

## 2. Reference TUI patterns bt is missing

### From lazygit — panels-as-action-context

**Pattern:** lazygit has a fixed multi-panel layout (Status, Files, Branches, Commits, Stash, etc.). The currently-focused panel determines the available actions, displayed in a context-sensitive cheat sheet at the bottom. Tab cycles panels. Crucially, **every panel exposes verbs that operate on the selected row** — staging files, picking commits, dropping stashes — and those verbs are listed inline.

**bt today:** bt has a vaguely similar shape (list + detail split, with `tab` to switch focus), but it's not panel-based — it's view-modal. You don't have multiple useful panels in front of you simultaneously; you swap among them. The detail panel is a passive renderer of selected issue, not an action context with its own verbs.

**Why this matters for bt:** Once writes land, `bd update`, `bd close`, `bd comment`, `bd block`, `bd dep add` etc. all need surfaces. Stuffing them into the existing 60+ keybinding global namespace is going to break. lazygit's model — "verbs are scoped to the focused panel" — gives a clean home for write actions: the detail panel becomes a write-action panel, the dependency panel exposes link/unlink, the comments panel exposes add/edit/delete.

**Concrete gap:** bt has no concept of "the current selected issue's actions are X." Compare to lazygit's status bar: with a file focused, you see `space: stage  d: discard  e: edit  o: open  ...`. bt's footer has filter badges, project info, status messages — never "here's what you can do to the thing you're looking at."

### From k9s — universal command palette + jump-anywhere

**Pattern:** k9s uses `:` as a universal command palette. Type `:pods`, `:svc`, `:dp`, `:ns`, anything → jump there. The same `:` works for arbitrary Kubernetes resources, and tab-completion suggests what's available. There's no view-switching key matrix to memorize; there's one verb (`:`) and many nouns.

**bt today:** `:` opens BQL modal (`pkg/ui/model_update_input.go:1119-1125`), which is great for queries but is not a navigation primitive. bt has 11 view-toggle keys (`b`, `g`, `a`, `E`, `i`, `h`, `[`, `]`, `f`, `:`, `'`) plus modifiers, plus modals. Discovery is via the help screen.

**Why this matters for bt:** bt has more views than lazygit and roughly k9s-class breadth. The 11 view-toggle keys are at the edge of memorability; adding write actions, sprints, project-switching, settings, etc. will tip it. A `:` palette ("`:board`", "`:flow`", "`:close bt-mhcv`", "`:sprint q2`") gives bt a unified verb-vocabulary entry point that doesn't compete with single-letter mnemonics.

**Concrete gap:** No command palette. Help is a static reference card, not a "what can I do from here" search. `:` is consumed by BQL — which is itself a pseudo-command-palette for queries, but it doesn't navigate or invoke verbs. The natural fix is a unified palette where BQL is one citizen among many.

### From gh dash — per-section keybindings + section management

**Pattern:** gh dash lets you define multiple "sections" (saved queries) on a dashboard. Each section has its own context and own keybindings. You navigate sections like tabs, and each one is a saved view with persistent state.

**bt today:** Recipes (`'` modal) are the closest analog — saved filter+sort presets. But recipes are picked one at a time from a modal, not displayed as persistent navigable sections. Workspace mode + repo picker is also adjacent — pick a project filter — but again, not persistent sections.

**Why this matters for bt:** Daily-driver issue tracking means returning to the same 4-5 views: "my P0/P1 ready", "blocked", "in flight", "stale", "this sprint". gh dash's sections-as-tabs would give bt persistent-default-views behavior. Today, every session starts fresh with whatever the user remembers to apply.

**Concrete gap:** bt has no concept of "user's pinned dashboards." Recipes can stand in but they require explicit application each time. No saved-view tab strip.

### From helix — "type to filter actions" discoverability (space menus)

**Pattern:** Helix uses leader-key menus (`space` opens a popup with cheat-sheet entries; type to filter; commit). Same for `g` jumps, `m` marks, etc. The user never has to leave the keymap to see what's available.

**bt today:** bt has `?` / `F1` for help overlay; double-backtick for context help; no progressive popups. The user must know the key in advance OR open the help wall.

**Why this matters for bt:** With 60+ keybindings spread across 11 views, bt is exactly the case helix solves. A `space` (or `g` for "go", or `,` for "leader") leader popup that shows the 5-7 most relevant actions for the current focus, with type-to-filter, would be transformative for memorability — especially as writes triple the action count.

**Concrete gap:** There is no per-context popup. `context_help.go` has per-context content but it's a static modal, not an action-launcher — you can't act from it.

### From lazyjj — CLI-shell-out + reactive refresh, done well

**Pattern:** lazyjj is the closest architectural twin to bt's planned write path. It's a TUI for jj-vc that shells out to the `jj` CLI for every state change, then reloads from `jj log`. UX patterns lazyjj nails:
- **Per-action status line** that shows the actual `jj` command being run, so the user can learn the CLI by watching the TUI.
- **Confirmation prompts** for destructive actions are inline, not modal — a single line at the bottom asks `Y/n`.
- **Optimistic-then-confirm** state updates: after the shell-out succeeds, lazyjj reloads. If reload disagrees, lazyjj shows the diff.
- **Edit-in-$EDITOR** flow for descriptions: lazyjj suspends the TUI, opens $EDITOR, on save resumes and reloads.

**bt today:** Designed to do the same thing (per product vision §"What Needs to Change for CRUD"), zero implementation. `O` key in list (`model_keys.go:1003`) does an editor open of beads.jsonl, but that's the legacy JSONL flow, not a write-through.

**Concrete gap:** No `internal/bdcli/` package (called out in product-vision doc, line 95). No "show me the bd command" trace. No shell-out toast UX. No suspend-and-resume-$EDITOR pattern.

### From superfile — modern visual chrome (sidebars, breadcrumbs, picker UX)

**Pattern:** superfile sets a visual baseline: persistent left sidebar (pinned dirs), top breadcrumb showing current location, multiple panes side-by-side, modern picker modals with rounded borders + clear hierarchy. Color choices are calibrated for accessibility, and the empty/loading/error states are designed (not just text).

**bt today:** Tomorrow Night palette, rounded borders are partial (vision doc calls out "Two competing panel rendering approaches: hand-drawn box chars vs Lipgloss borders" line 56). No breadcrumb. No persistent sidebar.

**Why this matters for bt:** Once bt has writes + cross-project + sprints, "where am I" becomes a real navigation question. A breadcrumb (`global > bt > sprint:q2 > bt-mhcv`) and a slim left-rail with pinned recipes/sprints/projects is what daily-driver TUIs converge on.

**Concrete gap:** No breadcrumb. No left sidebar. Visual chrome is inconsistent (per the vision doc itself).

### From lazygit — undo + reflog

**Pattern:** lazygit has `z` for undo. Every git-mutating action gets logged and is reversible. Reflog is a first-class view.

**bt today:** No undo. No action history. If a write goes wrong (once writes land), the user has to manually `bd update --priority` it back.

**Concrete gap:** Beads has a native `events` table (per AGENTS.md "Beads architecture awareness"), so the data is there for an action log. The UI just doesn't surface it.

---

## 3. bt-specific productization gaps (independent of reference TUIs)

### Empty states
There's no designed empty state. Today: `cmd/bt/root.go:418-421` does `fmt.Println("No issues found. Create some with 'bd create'!")` and exits before the TUI even starts. If the user lands in the TUI with zero issues after filtering, they get an empty list with no guidance. No "you've filtered everything out, press `a` to show all" prompt. No "zero open P0s — nice" celebration. No "this project just got initialized, here's how to start" onboarding.

### Error states
Errors flow through `setStatusError()` → footer line. They auto-dismiss. There's no error log, no "show me what went wrong 30 seconds ago", no error escalation when something bad happened (failed Dolt connect, failed reload, failed export). Once writes land this gets actively dangerous: a failed `bd close` shouldn't just flash and disappear.

### Loading states
Background worker exists (`background_worker.go`). Snapshots are loaded async. But the UI doesn't have a calibrated loading vocabulary — sometimes a status string says "loading", sometimes a footer freshness indicator updates, sometimes nothing visible happens during a 2-second `bd` shell-out (which doesn't exist yet but will). No spinner component. No "this view is computing" placeholder.

### Help system completeness
- Static help (`?` / `F1`) — exists, has a tutorial behind `space`
- Context help (`pkg/ui/context_help.go`) — per-context summaries
- **What's missing:** "what does this key do RIGHT NOW given my current focus + modal + view-mode + filter state?" No which-key style popup. No per-key tooltip. No `?` after a prefix to show "what can come next." The 60+ keybinding namespace is humanly impossible to fully memorize, and bt isn't building it down — it's about to add writes which will double it.

### Onboarding
- AGENTS.md prompt modal exists for agent-friendliness onboarding
- Tutorial exists behind `?` → `space`
- **What's missing:** First-launch detection. No "is this the user's first bt session" hook. No detection of "user opened bt in a non-bd-init dir" (handled with an error message rather than an init wizard). No "we noticed you have 47 stale beads, want to triage?" nudges.

### Settings discoverability
- Theme overrides exist (3-layer YAML merge)
- Background mode toggle (`--background-mode`, env var, config file)
- BD_ACTOR via env
- **What's missing:** A settings view inside the TUI. Today, settings are config files + env vars + flags, with zero in-TUI discoverability. Want to switch theme? Edit YAML, restart. Want to change background-mode? Same. A `:set` palette or a settings modal would unify this.

### Undo / command history
- No undo (already noted above)
- BQL modal has up-arrow history (`model_keys.go:796-802`) — limited to BQL queries
- No general action history. No "last 10 things I did". No `Ctrl+R`-style recent-action search.

### Persistence of view state
- Recipe + filter applied via flags persist for that session only
- Window size, split ratio, last view — all reset on restart
- No "resume where I left off"

### Search ergonomics
- List filter (Bubbles `/`) — local
- BQL modal (`:`) — composable but separate
- Semantic search (`Ctrl+S` cycles fuzzy → semantic → hybrid) — orthogonal
- **Gap:** Three-search-modes-fighting-each-other UX. The user can't easily tell which search is active without reading the footer. No unified "search" verb that picks the right backend automatically.

### Status bar real-estate budget
The footer is doing a lot: filter badges, project name, freshness, status message, hints. There's no budget for "current selection actions" (which is what lazygit puts there). Once writes land, footer needs to either grow taller or specialize.

### Multi-select
- `repoPicker` and `labelPicker` have multi-select via `space`
- Main list does **not** — single selection only
- **Gap:** Mass-close, mass-relabel, mass-comment requires multi-select on the main list. Without it, every bulk operation is a `--robot-*` round-trip.

### Quoting / clipboard / share-out
- `y` copies ID to clipboard
- `C` copies issue summary to clipboard
- No "copy as bd command" (e.g., `bd close bt-mhcv --reason="..."`)
- No "copy as URL" (no concept of bead URLs yet)
- No "copy as markdown reference" for pasting into docs

### Refresh feedback
- Background poller silently updates
- No visual indicator of "you're looking at a 30-second-old state"
- No "just changed under you, here's the diff" toast

---

## 4. Writable-TUI specific concerns (bt-oiaj)

This is the section that matters most for the reorg, because every read-side gap above gets sharper with writes.

### Confirmation flows
**lazygit pattern:** Inline `Y/n` prompt at the bottom for destructive ops (drop stash, delete branch). Not modal. One key to confirm.
**lazyjj pattern:** Same, plus shows the actual command being run.
**bt today:** Only `ModalQuitConfirm` exists. There is no inline-confirm primitive.
**Gap:** Need a footer-line confirmation primitive that doesn't take focus away from the list. Modal-per-confirmation will be unbearable at write-cadence. The natural pattern: status line shows "close bt-mhcv? [y]es / [n]o / [e]dit reason" with single-key dispatch, no focus shift.

### Optimistic updates vs poll-and-reflect
**The problem:** bt's plan is shell-out-to-bd, then poll Dolt. Polling is on a 5-second interval (per ADR-002 / vision §). That's a long blank stare after a `c` press for "close."
**Two options:**
- **Pure poll-and-reflect:** wait for Dolt to confirm; safe but feels laggy
- **Optimistic:** mutate the local model immediately, show the new state, then reconcile when Dolt poll confirms (or revert + show error if it doesn't)
**Recommendation:** Hybrid — optimistic for status changes (close, claim, reopen) because the failure mode is rare and obvious; poll-only for create (you don't have an ID until bd assigns one anyway) and for edits with multiple fields.
**lazyjj reference:** Goes optimistic for status, blocking for descriptions (because they involve $EDITOR anyway).
**Concrete gap:** No optimistic-update plumbing in the model. `m.data.issues` is a poll-replaced slice; nothing in the architecture supports a "pending-not-yet-confirmed" overlay.

### Conflict resolution when poll picks up changes mid-edit
**The problem:** Multi-session bt + multi-agent bd. User opens bt-mhcv detail panel, starts typing a comment. Meanwhile, an agent in another session updates bt-mhcv's priority. Poll fires. What happens to the open editor?
**lazygit reference:** lazygit avoids this by keeping mutations atomic (one git command per action) and always reloading after.
**lazyjj reference:** Edits via $EDITOR are atomic — TUI is suspended during edit.
**Recommendation:** Lock-by-suspension for any modal that shells out. Inline edits should diff-on-save and warn on conflict. Don't try to merge.
**Concrete gap:** No model concept of "this issue is dirty / has uncommitted user input". No "issue changed under you, refresh?" prompt.

### Undo
Already noted. With writes, this goes from nice-to-have to mandatory. Beads has the `events` table; bt should query it after every shell-out and offer "undo last action" as a footer hint.

### Draft state
**lazygit pattern:** Stash works as draft state for working-tree changes. Commits-in-progress get a "WIP" prefix.
**Recommendation for bt:** Drafts for `bd create` modals — if the user opens the create-modal, types half a description, then escapes, save the draft. Restore on next create. Wisps in beads (ephemeral issues) are the right primitive for this — vision-aligned.
**Concrete gap:** Modal state is wiped on close. No draft persistence.

### Multi-bead operations
**lazygit pattern:** `space` to multi-select commits, then bulk operations (squash, drop, etc.).
**Recommendation for bt:** Add multi-select to main list (already noted). Then expose verbs that take multiple beads: mass close ("close 4 beads with reason X"), mass relabel, mass-priority-bump, mass-claim-as-blocked, mass-add-comment.
**Why this matters:** Triage workflow. Sean's morning routine (per the imagined scenario below) is at least 50% bulk operations.

### Where do write-verbs live in the keymap?
**The hardest design question of bt-oiaj.** Single-letter verbs are mostly exhausted by view-toggles and filters. Writes will need somewhere to go. Three options:
1. **Capital-letter convention:** `C` = create, `D` = delete, `B` = block — collides with existing (`C` already copies, `D` collides with detail-panel default in some views)
2. **Leader key:** `,c` = create, `,e` = edit, `,x` = close, `,k` = kill — no collisions, requires teaching
3. **Command palette only:** `:create`, `:close`, `:edit` — discoverable, slower

**Recommendation:** Hybrid — leader key (`,` or `space`) opens a popup of write verbs (helix-style); the most common (`c` for close on selected? `e` for edit? `n` for new?) get direct bindings only after the leader-popup gets steady-state usage and a pattern emerges.

### Visual feedback for the bd command run
**lazyjj reference:** Footer shows `> jj squash -r abc123` for ~2s after each action. Teaches the CLI by exposing it.
**Recommendation:** bt should do the same. Every shell-out flashes the actual `bd ...` command in the footer. This also gives the user a one-press "show me again so I can copy" affordance.

### $EDITOR shell-out for long-form fields
**Pattern (vim, lazygit, lazyjj all):** Multi-line text editing is unbearable in a TUI input field. Suspend the TUI, open $EDITOR, resume on save.
**bt today:** `O` opens beads.jsonl in $EDITOR (legacy). No mechanism for "edit this one field in $EDITOR."
**Recommendation:** New primitive — `EditFieldIn$EDITOR(initialText) string` — used for description, comment, close-reason. Anywhere multi-line text is needed.

### Agent-friendly write paths (the cross-cut)
bt has a strong robot-mode tradition (`--robot-*` flags). Writes should preserve this: every TUI write action should have an equivalent `bt --robot-*` form (or just document "use `bd close` directly"). This is more of a "don't paint yourself into a corner" gap than a UX gap.

---

## 5. Productization "north star" — a daily-driver scenario

Imagine Sean opens bt at 9am Monday. Coffee. He wants to:
1. See where things stand across his portfolio (bt, beads, cass, marketplace, dotfiles)
2. Triage P0/P1 beads
3. Find a specific bead about "Dolt audit" and read it
4. File a comment on it
5. Link two beads (the audit bead blocks something he discovered last night)
6. Close two beads that he finished but never closed
7. Queue up "this morning's batch" as a sprint or pinned recipe

### Today's bt walks him through this how?

**Step 1 — portfolio overview:** Launch `bt -g`. Get global mode list, all beads from all projects in a flat list. No portfolio-health dashboard (called out as bt-x cluster). Workflow rough-edge: he sees the same flat list whether he's working on bt or beads. He hits `]` to get attention scores OR `i` for insights, but those compute over the union, not per-project. He has to mentally project-filter.

**Step 2 — triage P0/P1:** Apply triage recipe via `'` modal. Now sees ranked list. Good. But — he can't multi-select to bulk-set priority, can't bulk-comment "deferring to next sprint", can't queue them.

**Step 3 — find the Dolt audit bead:** `/` for fuzzy, or `Ctrl+S` to flip to semantic, or `:` for BQL. Three searches. Probably uses `/` first, doesn't match, flips to semantic, finds it. Three keypresses he shouldn't have to learn. Modern: he should type a query and bt picks the best ranker.

**Step 4 — file a comment:** Today: impossible from TUI. He drops to a separate terminal, runs `bd comments add bt-mhcv "..."`, comes back to bt, hits `r` to refresh maybe? Doesn't even know if the refresh picked it up because the freshness indicator is subtle.

**Step 5 — link two beads:** Today: impossible from TUI. Out to terminal, `bd dep add bt-mhcv bt-foo --type=blocks`, back to bt.

**Step 6 — close two beads:** Today: impossible from TUI. Out to terminal x2, both with the close-format reason from his head/notes. Back to bt.

**Step 7 — queue a sprint:** Today: ad-hoc. He could try the sprint view (`pkg/ui/sprint_view.go`), but it's the pattern-violating one called out in the vision doc.

### Where today's bt falls short (summarized)

- No portfolio dashboard with per-project health → step 1 is rough
- No bulk operations on the list → step 2 is single-bead-at-a-time even when he has 10
- Three search modes, no auto-routing → step 3 is keypress-shopping
- Zero writes → steps 4, 5, 6 are entirely outside the TUI
- Sprint view is half-baked → step 7 is "go pin a recipe and pretend"

### What "nailed it" looks like

- `bt` opens to a portfolio dashboard (or to home project with a one-key flip to portfolio). Health is at-a-glance.
- Tab strip across the top has pinned dashboards (Sean's defaults: "P0/P1 ready", "blocked", "this week").
- Multi-select with `space` works on the main list; bulk verbs (close, relabel, prioritize, comment) operate on the selection.
- Single search verb (`/` or `:s`); ranker auto-selects.
- Write verbs live behind a leader (`,c` close, `,e` edit, `,n` new, `,d` link), discoverable via popup.
- Inline confirmations (`Y/n` in footer), not modals.
- Every shell-out shows the bd command run (lazyjj-style) and is undoable for ~30 seconds.
- $EDITOR opens for description/comment/close-reason; resume cleanly.
- Detail panel is an action context — when an issue is selected, the detail pane shows verbs (close, comment, link, claim) inline, not behind keybinds you have to memorize.
- Sprint view = a pinned multi-bead dashboard; "queue this batch" = a single command.

The morning takes 5 minutes inside bt instead of 20 minutes ping-ponging between bt and the terminal.

---

## 6. Prioritization

### Foundational (without this, the TUI feels broken — fix first)

These are the ones where the absence is felt every session, not just at the edges. They block the writable-TUI transition or make the read-only TUI feel undercooked.

1. **Empty / error / loading state vocabulary.** Today these are ad-hoc strings in the footer. Need designed components: empty placeholder, error toast with persistence, loading spinner. Without this, writes will land into a UX that can't communicate failure.
2. **Inline confirmation primitive.** Footer-line `Y/n` prompts. The whole writable-TUI plan depends on this; modal-per-confirmation will not survive contact with a triage session.
3. **Action-context model for the detail panel (lazygit pattern).** The detail pane needs to show "what can I do to this issue right now" verbs. Without this, bt-oiaj has nowhere to put write verbs that doesn't fight the existing keymap.
4. **Multi-select on the main list.** Daily triage is bulk by nature. Single-select-only forces every bulk op out of the TUI even after writes land.
5. **Shell-out trace + status pattern.** "Show the bd command, show success/failure clearly, surface duration." Foundation for the entire write path.
6. **Optimistic update plumbing in the model.** Without "pending mutations" overlaid on `m.data.issues`, writes feel like 5-second blanks.

### Workflow-completing (without this, common flows have rough edges)

These don't break the TUI but they make daily-driver use noticeably worse than the reference TUIs.

1. **Command palette (`:` repurposed or a new key).** Unified verb entry. BQL stays as one citizen. Write verbs slot in. Navigation slots in. Discovery solved at one entry point.
2. **Leader-key popup (helix-style which-key).** Per-context action menu with type-to-filter. Solves the 60+ keybinding memorability cliff for writes.
3. **Persistent saved-view tabs (gh dash sections).** Recipes-as-tabs, not recipes-via-modal. Pinned dashboards.
4. **Portfolio health dashboard (cross-project workflow).** Already in roadmap as bt-cluster. Step 1 of the morning scenario depends on it.
5. **Search auto-routing.** One verb, the system picks fuzzy/semantic/hybrid. Three modes is a leak of implementation through to UX.
6. **$EDITOR suspension for long-form fields.** Required for descriptions, comments, close-reasons. Anything multi-line.
7. **Undo (last-N actions, ~30s window).** With writes, this is no longer optional.
8. **Conflict-on-poll surfacing.** "Issue changed under you, refresh?"
9. **First-launch detection + onboarding nudge.** Especially "you're not in a bd-init'd dir" → init wizard or fallback-to-global.

### Polish (nice-to-have, ship later)

These improve feel and approachability but don't gate workflow completion.

1. **Breadcrumb navigation.** "global > bt > sprint:q2 > bt-mhcv". Helps once cross-project + sprints are mature.
2. **Persistent left-sidebar with pinned items** (recipes, sprints, projects).
3. **Visual chrome consistency.** Resolve the "two competing panel rendering approaches" called out in the vision doc.
4. **Settings view inside the TUI.** Theme picker, background-mode toggle, BD_ACTOR setting — without leaving the app.
5. **Resume-where-left-off.** Last view, last filter, last selection.
6. **"Copy as bd command" / "copy as markdown ref"** clipboard variants.
7. **First-class action log / reflog view.** Beads' events table makes this cheap; surface it as a tab.
8. **Drafts for create modals.** Save partial input on escape; restore on re-open. (Wisps fit naturally.)
9. **Celebration / zero-state design.** "No P0s open. Nice." vs blank list.
10. **Polish on the freshness indicator.** Make "this is N seconds stale" obvious.

---

## Cross-references for the roadmap reorg

### Files that own the surfaces discussed

- `cmd/bt/root.go` — CLI flag surface (170 flags, monolithic — flagged in vision doc as `bt-if3w`)
- `pkg/ui/model.go:78-94` — `ViewMode` enum (the place a "saved view tab" abstraction would slot in)
- `pkg/ui/model.go:102-117` — `ModalType` enum (the place inline-confirm and command-palette would slot in)
- `pkg/ui/model_keys.go` — per-view keymaps (the place leader-key popup would dispatch from)
- `pkg/ui/model_update_input.go:803-1219` — global key dispatch (the namespace pressure point)
- `pkg/ui/context_help.go` — already-present per-context help (the substrate for which-key popups)
- `pkg/ui/model_modes.go` — view enter/exit transitions
- (proposed but not yet existing) `internal/bdcli/` — write-path package called out in vision doc

### Beads / docs that align with these gaps

- `docs/brainstorms/2026-04-09-product-vision.md` — vision and resolved questions (esp. §"Vision: Main Board/List View" and §"Vision: Global Mode")
- `docs/brainstorms/2026-04-12-cross-project-management.md` — portfolio dashboard (foundational gap §3, workflow-completing #4)
- `docs/adr/002-stabilize-and-ship.md` Stream 7 — bt-oiaj writable TUI
- bt-s4b7 — project navigation UI (workflow-completing, ties into multi-project sidebar/breadcrumb)
- bt-xavk — help system redesign (already P1, aligns with leader-popup and command-palette work)
- bt-oiaj — writable TUI (every "Foundational" gap above is either input or output for this)
- bt-mhcv — Dolt-migration audit (P0, frames data-layer assumptions any write code must respect)

---

## Closing observation

bt's current shape is fundamentally "many specialized read views, glued by a single keymap." The reference TUIs that do this best (lazygit, k9s, helix) have all converged on three things bt doesn't have yet:
1. **A unified verb-entry primitive** (command palette / leader key) so the keymap doesn't have to be flat
2. **Action-context per pane** so verbs are scoped, not global
3. **Modern feedback patterns** (inline confirms, optimistic updates, undo, $EDITOR suspension) so the user trusts mutations

The writable-TUI transition (bt-oiaj) is the forcing function for all three. If the foundational gaps in section 6 are fixed before writes land, bt-oiaj becomes a feature delivery instead of a pile of UX archaeology. If they're not, the keymap will fight every new verb and the user experience of writes will be uneven.
