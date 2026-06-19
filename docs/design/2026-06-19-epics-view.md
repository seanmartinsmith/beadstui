# Epics view: overview page + focus card (repurposed from the dead sprint view)

Status: design / pre-implementation
Date: 2026-06-19
Related beads: bt-ryi5z (sprint view unreachable - originating), bt-gfxhz.3 (per-child status pills), bt-h97e (dedicated deep epic view, future)

## Why

The `ViewSprint` dashboard shipped with the original beads_viewer fork import
(`454784c9`, tagged `bv-161`) and was **dead on arrival in bt**: no global key
ever set the mode (fixed in this session under bt-ryi5z), and its only data
source - `.beads/sprints.jsonl` - is never written by bt or bd, and isn't read
at all in Dolt mode (the loader was gated on `beadsPath != ""`, which is empty
for Dolt/workspace/global). "Sprint" is not a beads-native concept; bd has no
sprint or milestone primitive.

But the render skeleton (a scrollable list of items, each with a progress bar +
status counts + at-risk flag + drill) is useful - and it maps almost 1:1 onto a
concept bd *does* have natively: the **epic** (a parent bead + its
parent-child children). `bd epic status` already computes epic completion.

bt currently surfaces epic progress three ways - list rows carry
`EpicDone/EpicTotal`, the detail pane renders an "Epic Progress" block
(`model_filter.go`), and `bt-h97e` plans a deep single-epic view (mermaid +
drill, blocked on the mermaid substrate). What's missing is a place to see
**all epics at once**, and a **focused single-epic surface** better than the
markdown-styled, buried detail-pane embed. This design fills both by repurposing
the sprint scaffolding, and retires the sprint feature.

## The shape: a 3-tier epic stack

| Tier | Surface | Scope | Status |
|---|---|---|---|
| 1 | **Epics overview** (new view) | all epics | this design |
| 2 | **Epic focus card** (new modal) | one epic | this design |
| 3 | `bt-h97e` deep epic view (mermaid, group ops) | one epic, deep | future, unchanged |

Tier 1 is the index; tier 2 is the focused drill; tier 3 is the eventual deep
map. They compose - they are not three copies of the same thing.

## Tier 1: Epics overview (new `ViewEpics`)

A new full-screen view listing every epic as a row: progress bar,
`checkmark/in-progress/blocked/open` counts, and an at-risk flag (>=1 stale
in-progress child). `enter` on a row opens the tier-2 focus card.

### Data source - the key payoff

The overview is a **projection over `m.filteredIssuesForActiveView()`**
(`model_filter.go`), the canonical filtered set that already folds in:

- **project scope** (`activeRepos`) - single-project vs global/cross-project
- **label filter**
- **status filter** (with the override below)
- **BQL / recipe / wisp visibility**
- **sort** (`SortMode` - creation-date, progress, etc.)

So the project picker (`w`), `W` home/all toggle, label filters, and sort all
compose **for free**. The overview follows the established non-list-view
pattern: a `refreshEpicsForCurrentFilter()` helper (mirroring
`refreshBoardAndGraphForCurrentFilter` / `rebuildTreeForCurrentFilter`) called
on view-switch, on filter change (from `applyFilter`/`applyRecipe`), and on data
reload. It reads the filtered set, partitions epics via `epicProgress()` /
`epicChildrenSorted()`, and rebuilds the rows.

### Status filter override (the one thing that does NOT inherit cleanly)

A progress bar needs *closed* children (the "done" portion). If the overview
naively reads a `status=open` filtered set, every bar reads ~0% because closed
children were filtered out. So for this view the status filter is **reinterpreted
as which epics to list**, while **children are always counted in full** for the
bar:

- **active** (default) - epics with >=1 non-closed child
- **all** - every epic
- **completed** - epics whose children are all closed

### Sort

Reuse the existing `SortMode` comparators, applied to epic rows (by progress %,
creation date, etc.). Default sort: TBD at implementation (candidates: most
at-risk first, or lowest progress first).

### Cross-project nuance

Parent-child deps store bare issue IDs with no project prefix
(`model.Dependency.DependsOnID`). In practice an epic's children are same-project.
When scoped to one project, children outside scope are not counted - progress
reflects the current scope, which is the correct behavior. Document this; no
special handling.

## Tier 2: Epic focus card (new modal)

A centered modal overlay (compositing like `ModalAlerts` /
`OverlayCenterDimBackdrop` per `docs/design/tui-modal-compositing.md`), opened:

1. `enter` on a row in the overview (tier 1), and
2. on an epic in the list / detail page (a focus key when the cursor is on an
   epic).

Renders: the epic's children as **lipgloss status pills** (status + priority +
ID + title), progress, at-risk children, and drill-into-a-child. The pill
renderer **is `bt-gfxhz.3`'s deliverable** - build it once and share it between
this modal and the existing detail-pane Epic Progress block (single source of
truth, closes bt-gfxhz.3). Children come from `epicChildrenSorted()`
(natural-numeric order already implemented).

## Salvage vs strip

**Keep / repurpose:**
- The `ViewSprint` ViewMode plumbing -> rename to `ViewEpics` (+ `focusEpics`,
  `ContextEpics`, help/focus-restore cases).
- The render skeleton: progress bar, status counts, at-risk logic, child list.
- `epicProgress()` / `epicChildrenSorted()` (already exist, unchanged).
- The Dolt-mode load-path fix's `GetBeadsDir` pattern is no longer needed once
  the file source is gone (see strip).

**Strip (needs explicit go-ahead - file deletions):**
- `pkg/model` `Sprint` struct + methods.
- `pkg/loader/sprint.go` (+ `sprint_test.go`).
- `cmd/bt/robot_sprint.go` + the `robot sprint` cobra commands; the
  `sprints.jsonl` reload blocks in `model_update_data.go`.
- The **burndown + dates** UI - epics have no time window, and a burndown needs
  one. (Epic velocity-over-time, if ever wanted, is a separate event-history
  feature, not this.)
- `cmd/bt/burndown.go` sprint pieces.
- The demo `.beads/sprints.jsonl` (untracked, delete after).

**Replace:** `bt robot sprint` -> `bt robot epics` = a robot `bd epic status`
(epics with progress, for agent consumption), if we want robot parity.

## Keybindings (deferred to implementation)

Working default: **`E` = Epics overview** (mnemonic), **Tree moves `E` -> `P`**
(zero-cost: `P` frees up when Epics takes `E`; nothing shadowed). Tier-2 focus
card opens on `enter` from the overview and a focus key from the list/detail.
Alternative considered: Tree -> `T` (perfect "T for Tree" mnemonic) at the cost
of list's `T` = time-travel-+HEAD~5. Final pick at implementation per the
`bt-h97e` precedent.

## Testing

- Render-harness scenarios (`render_harness_test.go`) for the overview at full
  and scrunched sizes, and the focus card. (Sprint scenario already added this
  session - repoint to epics.)
- Key/cardinality tests for the new `EpicsKeys` map (matching the bt-ift6
  key.Map convention - every binding has a non-empty `Help.Desc`).
- Helper tests already cover `epicProgress` / `epicChildrenSorted`.
- Status-filter-override test: a `status=open` view still shows full progress
  (closed children counted).

## Phasing

1. Strip sprint (Sprint struct, loader, robot, burndown, sprints.jsonl path);
   rename `ViewSprint` -> `ViewEpics`; repoint the render to epics + the filtered
   set; wire `E` (Epics) and move Tree to `P`. Ships tier 1.
2. Tier 2 focus card + shared pill renderer (closes bt-gfxhz.3); `enter` from
   overview + focus key from list/detail.
3. `bt robot epics` (optional robot parity).
4. (Later, separate) `bt-h97e` deep view layers on top.

## Open / deferred

- Exact keys (above).
- Default sort for the overview.
- Whether the detail-pane Epic Progress embed is later replaced by the focus
  card or kept as the inline at-a-glance form (lean: keep inline, share the
  renderer).
