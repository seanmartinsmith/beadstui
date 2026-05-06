# TUI bead-edit patterns (Charm v2)

Engineering reference for in-place editing of bead fields from the TUI
(priority, status, type, assignee, labels). Informs bt-88qn (priority
cycle design) and the broader Stream 7 (CRUD-from-TUI) work.

Verified against `charm.land/bubbles/v2@v2.1.0` (the v2 module path
post-rebrand; same code as `github.com/charmbracelet/bubbles/v2`).

## Module path note

The project is on the `charm.land/*` v2 module path post-rebrand, kept
in lockstep with the `github.com/charmbracelet/*` mirror. Web links
below cite GitHub; installed source for grepping lives at
`C:/Users/sms/go/pkg/mod/charm.land/...`.

## Idiomatic patterns inventory

### 1. Delegate `UpdateFunc` for row-context keys (cleanest enum cycle)

Bubbles `list.Model` exposes `DefaultDelegate.UpdateFunc` as the canonical
extension point. The list's `handleBrowsing` calls
`m.delegate.Update(msg, m)` after its own switch
(`bubbles/v2/list/list.go:904`), so the delegate sees all unconsumed
keys, can read `m.SelectedItem()`, and can mutate items via
`m.SetItem(index, newItem)`, `m.RemoveItem`, and `m.NewStatusMessage`.
Charm's own `examples/list-fancy/delegate.go` follows exactly this shape.

```go
d := list.NewDefaultDelegate()
d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
    if i, ok := m.SelectedItem().(IssueItem); ok {
        if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, keys.cyclePrio) {
            next := nextPriority(i.Issue.Priority)
            return tea.Batch(
                m.NewStatusMessage(fmt.Sprintf("Priority: %s -> %s", i.Issue.Priority, next)),
                cyclePriorityCmd(i.Issue.ID, next),
            )
        }
    }
    return nil
}
```

This is the natural home for cycle-style keybinds (Pattern A / B).

### 2. Mode flag on the parent model + key preemption (for inline edit mode)

`list.Model` has no built-in concept of "row edit mode."
`handleBrowsing` always consumes `j/k/g/G/h/l//`
(`list.go:865-901`). Two ways to preempt:

1. **Intercept in the parent's `Update` before delegating to
   `list.Update`.** When `m.priorityEditMode` is on, handle
   `j/k/Enter/Esc/P` yourself and return without calling
   `list.Update`. This is what `bql_modal.go` and `label_picker.go`
   already do via the existing `m.activeModal` switch.
2. **Disable list keybindings while editing**:
   `m.list.KeyMap.CursorUp.SetEnabled(false)` etc., then re-enable on
   commit/cancel. More surgical; lets paging via PgUp/PgDn and help
   keep working. `key.Binding.SetEnabled` is the supported toggle
   (see `list-fancy/delegate.go` calling
   `keys.remove.SetEnabled(false)`). For inline edit mode, disable
   `CursorUp/CursorDown/PrevPage/NextPage/GoToStart/GoToEnd/Filter/Quit/ClearFilter`.

Visual indicator is purely render-time: pass
`m.priorityEditMode` and `m.priorityEditValue` into the delegate, and
in `Render` swap the priority badge for a bracketed/underlined version
when `index == m.Index() && d.PriorityEditMode`.

### 3. Modal-overlay picker (already precedented in this project)

The project already ships three of these: `pkg/ui/label_picker.go`,
`pkg/ui/recipe_picker.go`, `pkg/ui/repo_picker.go`, plus
`pkg/ui/bql_modal.go`. AGENTS.md prescribes
`OverlayCenterDimBackdrop` from `pkg/ui/panel.go` for modal
compositing. A priority picker would be a 5-row trivial version of
`LabelPickerModel` without the textinput - up/down + Enter/Esc + render.

### 4. Per-row anchored popover

There is no first-class anchored popover in Bubbles or Lipgloss v2.
You can fake it with `lipgloss.Place` at computed coordinates, but
there is no off-the-shelf "balloon next to row N" primitive. Skip;
not idiomatic.

### 5. `huh.Form` as embedded sub-model

`huh.Form` implements `tea.Model` (`form.go:528`) and
`WithWidth/WithHeight` make it sizable, so a
`huh.NewSelect[string]().Options(...)` form can embed inside an
overlay. For one enum field this is heavyweight - focus management,
validation, theme overhead. For a multi-step CRUD flow ("create new
bead" with status + assignee + labels in one ceremony), it becomes
the right tool. For bt-88qn alone, overkill.

### 6. Inline `textinput` swap (free-text fields)

The `bubbles/list` package itself uses this pattern internally - its
filter UI is a `textinput.Model` swapped into the header when
`FilterState() == Filtering` (`list.go:216-218, 911-948`). Same
shape applies for assignee free-text: keep `textinput.Model` on the
parent model, focus/blur on enter/exit, render it inside the row
cell from your delegate when
`d.AssigneeEditMode && index == m.Index()`.

### 7. `list.Model` built-ins for mutation

- `m.SetItem(i, newItem)` - mutate one row in place. Use after a
  successful `bd update` round-trip so the TUI reflects the new
  value without a full reload.
- `m.NewStatusMessage(s)` - shows a transient line in the list footer
  for `m.StatusMessageLifetime` (default 1s). Perfect for
  "Priority: p2 -> p3."
- `m.SetDelegate(d)` - swap delegate at runtime. Useful only if you
  want a fully separate "edit-mode delegate" rather than a flag on
  one delegate; usually not worth the swap overhead.

## Pattern fit for bt-88qn options

| Option | Mechanism | Idiomatic? | Awkward bits |
|---|---|---|---|
| **A** (`P` cycles forward) | Delegate `UpdateFunc` matches `P`, computes next priority, fires `bd update` cmd, calls `m.SetItem` on success, `m.NewStatusMessage` for confirmation. | Yes - mirrors `list-fancy` directly. | Forward-only wrap; user must traverse 3-4 states to go backward. No fat-finger ambiguity (capital `P` distinct from lowercase `p` for hints). |
| **B** (`P` fwd, sibling back) | Same as A, two bindings. | Yes. | Modifier conventions inconsistent across terminals. `Ctrl+P` collides with terminal/tmux defaults on some setups. Pick a *letter* sibling (e.g. `P` forward / `O` backward, or `>` / `<` modeled on git/vim) to avoid modifier hell. |
| **C** (`P` opens a 5-option modal) | Mirror `LabelPickerModel`. Add `m.priorityPicker` field, gate render via `m.activeModal`, dim backdrop with `OverlayCenterDimBackdrop`. | Yes - matches existing project conventions exactly. | One more modal in a modal-heavy TUI. Highest discoverability and lowest fat-finger risk. |
| **D** (`P` toggles row into edit mode, j/k cycles) | `m.priorityEditMode bool` + `m.priorityEditValue Priority` on parent. Intercept keys in parent `Update` before list. Disable several list keymap bindings while editing, re-enable on commit/cancel. Pass mode flag into `IssueDelegate.Render` for the visual indicator. | Buildable but **no Charm precedent** for "row enters edit mode." Inventing the pattern. | Three real traps: (1) `Esc` already maps to `ClearFilter` and `Quit` in `list.KeyMap` (`keys.go:65-67, 91-93`) - must disable both during edit; (2) `j/k` collision with `CursorDown/CursorUp` - must disable; (3) the existing project pattern of forwarding synthetic `KeyPressMsg`s to clear filter state means `list.FilterState() != Unfiltered` precondition has to be checked before entering edit mode, or you fight the filter UI. |

## Reusability across all five edit fields

This is the deciding lens.

| Field | Cardinality | Best pattern |
|---|---|---|
| Priority | 5 enum | A, C, or D |
| Status | ~4 enum (open / in_progress / blocked / closed) | Same as priority |
| Type | ~5 enum (bug / feature / task / chore / decision) | Same as priority |
| Assignee | free text (or fuzzy from known set) | inline `textinput` swap (pattern 6) - **not** a cycle |
| Labels | multi-select from large set | modal picker (existing `LabelPickerModel`) - **not** a cycle |

**Cycle-style (A / B) breaks at assignee and labels.** A cycle through
every possible assignee is nonsense; labels are multi-select. So if
you ship A for priority, you still need a modal picker for labels and
a textinput swap for assignee - **three disjoint paradigms**.

**Modal-picker (C) generalizes cleanly.** Same
`OverlayCenterDimBackdrop` shape works for priority (5 rows), status
(4 rows), type (5 rows), and assignee (textinput-on-top + filtered
list - exactly what `LabelPickerModel` already is). Labels uses the
existing `LabelPickerModel`. **One paradigm covers all five fields.**

**Inline-edit-mode (D) generalizes for enums but not for text.** D
works for priority/status/type. For assignee you still need a
textinput swap (different paradigm). For labels you still need a
multi-select modal (different paradigm). D buys you one mode for
three of five fields and orphans the other two.

## Gotchas in `list.Model` for mode-switching

Verified against `charm.land/bubbles/v2@v2.1.0/list/list.go`:

- **`Esc` is overloaded**: `KeyMap.ClearFilter` (`keys.go:65`) and
  `KeyMap.Quit` (`keys.go:91`) both bind `esc`. The list's
  `handleBrowsing` checks `ClearFilter` first (`list.go:859`). For
  Pattern D you must `SetEnabled(false)` on both during edit, or
  `Esc` will exit the program / clear the filter instead of
  cancelling your edit.
- **`q` quits**: Default `KeyMap.Quit` includes `q`. If priority
  cycle ever uses lowercase letters, watch this.
- **`/` enters filter mode** (`list.go:883-894`), which sets
  `filterState = Filtering` and routes ALL subsequent keys through
  `handleFiltering` (`list.go:911-985`). The delegate's
  `UpdateFunc` is **not called** during filtering ("All messages in
  the list's update loop will pass through here except when the user
  is setting a filter" - `list.go:56-57`). So Pattern A/B keybinds
  via delegate **do not fire while the user is filtering** - which is
  actually fine; you don't want priority cycling during filter
  typing. But test it.
- **`FilterState != Unfiltered` is sticky**: `FilterApplied` persists
  after the user accepts the filter (`list.go:938`). Browsing keys
  still work, delegate `UpdateFunc` still fires. Pattern A/B/D all
  work in filter-applied mode.
- **`m.SetItem(index, item)` re-renders** but does not move the
  cursor. Safe to call from a `tea.Cmd` returning a
  `priorityUpdatedMsg` after `bd update` succeeds.
- **`NewStatusMessage` lifetime is 1s default** (`list.go:240`). For
  audit trails ("Priority: p2 -> p3, undo with Z") that may be too
  short; bump `m.StatusMessageLifetime = 3 * time.Second` or wire a
  separate notification ring (the project already has
  `pkg/ui/events/ring.go`).

The synthetic-keypress workarounds existing in this codebase
(visible in `coverage_extra_test.go:171`, `context_test.go:200`) are
the smell of this filter-state stickiness; tests use
`tea.KeyPressMsg{Code: 'l', Text: "l"}` to drive modals around it.
Pattern C lives outside the list entirely (modal layer), so it
sidesteps every one of these traps. Pattern D fights all of them.

## Recommendation

**Ship pattern C (modal picker)** as the canonical edit path for all
bead fields. Reasons:

1. Single paradigm covering priority + status + type + assignee +
   labels.
2. Code already proven in this codebase (`label_picker.go`,
   `recipe_picker.go`, `repo_picker.go`, `bql_modal.go`).
3. Avoids every `list.Model` keymap collision.
4. Matches the project's mature modal compositing convention
   (`OverlayCenterDimBackdrop`, AGENTS.md).
5. Highest discoverability and lowest fat-finger risk.

The one cost: friction. Modal-per-edit is more clicks than a single
key cycle. The project already accepts this for labels; same trade
applies to priority/status/type.

### Hybrid A + C worth considering

For keyboard-fast users on enum fields, a delegate `UpdateFunc`
cycle (`P` forward / `O` backward) on top of the modal picker gives
both affordances backed by the same `bd update` write path:

- Cycle keys for fast inline edits (priority/status/type only - the
  three enum fields).
- Modal picker (`Enter` or `e` on row) for discoverable safe edits
  on all five fields (incl. assignee free-text and labels
  multi-select).

This is what gum-style ecosystem tools do (gum's `choose` is the
modal; gum users still bind keys in their wrappers). Two affordances,
one write path, both proven patterns. Recommended if A's "first
ship" velocity is appealing and you want to avoid re-architecting
when assignee / labels editing lands.

### Patterns rejected

- **Pattern B** has terminal-portability tax (modifier keys) for
  marginal gain over A.
- **Pattern D** invents a UX Charm doesn't bless and requires
  fighting the list keymap on three fronts (j/k, Esc, q). Not
  recommended unless you are willing to own that maintenance.

## Files in this project relevant to picking and implementing

- `pkg/ui/delegate.go` - existing `IssueDelegate`; add `UpdateFunc`
  here for A/B, or `PriorityEditMode` field for D.
- `pkg/ui/label_picker.go` - template for C.
- `pkg/ui/repo_picker.go` - simpler template for C (no textinput).
- `pkg/ui/bql_modal.go` - modal-with-textinput precedent.
- `pkg/ui/model.go` - where the `list.Model` lives, how
  `FilterState` is checked.
- `pkg/ui/model_keys.go` - parent-level key dispatch (where to
  intercept for D).
- `pkg/ui/panel.go` - `OverlayCenterDimBackdrop` (per AGENTS.md
  modal convention).
- `pkg/ui/events/ring.go` - if status messages need to outlive the
  1s list status.

## Installed Charm v2 source (read-only reference)

- `charm.land/bubbles/v2@v2.1.0/list/list.go`
  (lines 40-61 ItemDelegate, 819-908 Update flow, 667-680
  NewStatusMessage)
- `charm.land/bubbles/v2@v2.1.0/list/keys.go` (full KeyMap;
  Esc/q overloads)
- `charm.land/bubbles/v2@v2.1.0/list/defaultitem.go`
  (lines 86-141 DefaultDelegate / UpdateFunc shape)
- `charm.land/huh/v2@v2.0.3/field_select.go` and `form.go` (if you
  want huh-as-sub-model later)
- `charm.land/bubbles/v2@v2.1.0/textinput/textinput.go` (for
  assignee swap pattern)

## External sources

- [bubbletea/examples/list-fancy/delegate.go](https://github.com/charmbracelet/bubbletea/blob/main/examples/list-fancy/delegate.go)
  - canonical delegate `UpdateFunc` example
- [bubbles/list package docs](https://pkg.go.dev/github.com/charmbracelet/bubbles/list)
  - `ItemDelegate`, `KeyMap`, `NewStatusMessage`
- [bubbletea v2 UPGRADE_GUIDE_V2.md](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md)
  - declarative View, `tea.KeyPressMsg`
- [bubbles examples/textinput - swap pattern](https://github.com/charmbracelet/bubbletea/tree/main/examples/textinput)
- [charmbracelet/huh - embedding huh.Form in tea](https://github.com/charmbracelet/huh/discussions/98)
- [bubbles discussion #160 - textinput value capture](https://github.com/charmbracelet/bubbles/discussions/160)
- [bubbles list source on GitHub](https://github.com/charmbracelet/bubbles/blob/master/list/list.go)
