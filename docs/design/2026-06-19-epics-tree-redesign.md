# Epics overview redesign: full-sheet hierarchy tree (Tier-1)

Status: design / pre-implementation
Date: 2026-06-19
Related beads: bt-3ftfm.1 (this redesign - originating), bt-3ftfm (parent epic),
bt-gfxhz.3 (Tier-2 focus card - folded into the tree drill here), bt-ryi5z
(Phase-1 overview, superseded), bt-h97e (deep single-epic mermaid view - future,
out of scope), bt-3ftfm.2 (card ghost-cursor bug - separate, out of scope)

Supersedes the Tier-1 section of `docs/design/2026-06-19-epics-view.md`. The data
sourcing, status-filter override, and keybinding rationale in that doc still hold
and are referenced, not repeated.

## Why (the redesign trigger)

Phase 1 shipped the epics overview by inheriting the dead sprint dashboard's
render skeleton wholesale: a centered, `max-80`-wide bordered box
(`lipgloss.Place(Center)` + `min(80, width-4)` in `epics_view.go`
`renderEpicsOverview` ~:66/:134) wrapping a flat, sorted list of one-line epic
rows. Dogfooded 2026-06-19 against the real cross-project corpus (61 active
epics): *"not how i envisioned it - no tree view, not organized by project, not a
whole sheet, just a box in the middle."* The same instinct is in bt-h97e's
2026-05-08 comment (maintainer pressed `E` on an epic expecting its subtree, got
the global ~3236-bead tree: "no tree view for it? seems off/wrong").

This is a Tier-1 **redesign**, not a polish pass. The Phase-1 *data layer*
(`epicsOverviewRows`, `EpicRow`, `EpicStatusMode`, the status-filter override,
the scope/label sourcing in `refreshEpicsForCurrentFilter`) is correct and is
**kept**. Only the **render + interaction** layer is replaced.

## What ships (the four pillars, made concrete)

1. **Full-sheet, not a centered box.** Fill the terminal like Tree (`T`) /
   Graph (`g`) / Board (`b`): own the whole canvas, window + scroll the content.
   The `lipgloss.Place(Center)` + `min(80)` box is deleted.
2. **Organized by project.** Epics group under their project (ID prefix /
   `activeRepos`) as collapsible **swimlane headers** with a per-lane rollup.
3. **Tree-structured + visually rich.** Epic -> children as an indented tree
   (reusing the `tree.go` connector / windowing / expand-collapse pattern), each
   node carrying a status glyph + a **braille composition progress bar**.
4. **Filterable + scope-aware.** Inherits the Phase-1 sourcing: project scope
   (`w`), label filter (`f`), status mode (`s` = active/all/completed).

## Chosen visual register (user pick, 2026-06-19, from rendered braille mockups)

**Braille composition fill-bar** as the inline progress element (the btop
texture, not solid `█` blocks). The bar is always full-width; segments are
colored by child status composition:

- **done** -> green run (leftmost)
- **in-progress** -> yellow run
- **blocked** -> red run
- **open** -> grey track (the "unfilled" remainder)

The **filled** run (done/in-prog/blocked) renders as full-density braille
(`⣿`); the **track** (open) renders as a *low-density* braille glyph (a dim
mid-line such as `⠒`/`⠤`) AND dim-grey color. Using a different glyph density -
not color alone - for filled vs track means the boundary survives `ansi.Strip`,
so the render-harness `.txt` dumps (and non-color terminals) still show progress;
color then layers the composition on top in the `.ansi`/freeze view. So a
0%-done epic reads as an all-dim track bar, 100% as a full `⣿` green bar, and a
mixed epic shows full-density green|yellow|red runs then the dim track - one
glance = full composition. This subsumes Phase-1's separate `✓N ◐N ⊘N ○N` count
cluster (the counts move to an optional trailing summary; the bar carries the
signal).

A burn-up **sparkline** (per-epic completion trajectory from child `closed_at`,
also braille) was prototyped and is a **fast-follow** behind a `P`
progress-style toggle - NOT Tier-1 (it needs a per-epic time series and reads
near-empty for the many low-progress epics; cleaner as its own slice). Reserve
the `P` key; do not bind it yet.

## Architecture

### New model: `EpicsTreeModel` (the spine)

A purpose-built model in a **new** `pkg/ui/epics_tree.go`. It does NOT reuse the
global `TreeModel` (which builds from every issue's parent-child edges and
renders generic rows) - it builds an **epics-rooted, project-grouped** tree and
renders epic-specific rows (progress bars, swimlane headers). It mirrors
`tree.go`'s proven *windowing* approach (a flat visible-row list + manual
`[start,end)` clamp + cursor-follows-viewport) rather than entangling the shipped
global tree. The windowing math is simple enough (the Phase-1 box already does a
cursor-centered window at `epics_view.go` ~:88-119) that fresh, focused code is
cleaner than generalizing `TreeModel`; a later DRY pass can extract a shared
`viewportWindow` helper (follow-up bead, not Tier-1).

```go
// epicTreeRowKind tags each flattened row.
type epicTreeRowKind int
const (
    rowProjectHeader epicTreeRowKind = iota // swimlane header (collapsible)
    rowEpic                                 // an epic node (has a progress bar)
    rowChild                                // a non-epic child (status glyph + title)
)

// epicTreeRow is one rendered line in the flattened, expansion-aware list.
type epicTreeRow struct {
    kind     epicTreeRowKind
    depth    int          // indent level (project=0, epic under it=1, child=2, ...)
    project  string       // lane key (ID prefix); set on every row for the active header
    issue    *model.Issue // nil for rowProjectHeader
    counts   epicCounts   // rollup: done/total/inprog/blocked/open/atRisk (header + epic)
    lastKid  []bool       // per-ancestor "is last child" flags -> connector glyphs
    expanded bool         // header/epic: is it expanded?
    hasKids  bool         // epic: does it have children to expand?
}

type EpicsTreeModel struct {
    rows     []epicTreeRow      // flattened visible rows (post-expand)
    cursor   int                // index into rows
    offset   int                // first visible row (scroll)
    width    int
    height   int
    theme    Theme
    expanded map[string]bool    // key: project header ("proj:bt") or epic ID
    // statusMode/scope live on Model (Phase-1) and are passed into Build.
}
```

`epicCounts` is `EpicRow`'s count fields lifted to a small struct so a
project-header row can carry a lane rollup (sum of its epics) and an epic row can
carry its own.

### Build pipeline (pure, testable)

`Build(scoped []model.Issue, mode EpicStatusMode, now time.Time)`:

1. Reuse **`epicsOverviewRows(scoped, mode, now)`** (Phase-1, unchanged) to get
   the `[]EpicRow` set - this already applies the status-mode partition and
   counts children in full (the status-filter override). `scoped` is the
   workspace/label/wisp-prefiltered set from `refreshEpicsForCurrentFilter`.
2. **Root epics only**: drop epics that are themselves a parent-child child of
   another in-scope epic (they nest under their parent, matching `tree.go`'s
   "roots = no parent" rule). Real example: bt-19vp / bt-ph1z are epic-typed
   children of bt-ushd -> they appear only nested, not double-listed.
3. **Group** root epics by project key = `projectPrefix(epic.ID)` (ID up to the
   first `-`; reuse an existing prefix helper if present, else add one). Sort
   lanes by epic count desc (or name); within a lane sort by progress %
   ascending (Phase-1 default).
4. **Flatten** honoring expand state: for each lane emit a `rowProjectHeader`;
   if expanded, emit each epic as `rowEpic`; if an epic is expanded, recurse its
   `epicChildrenSorted` children as `rowChild` (a child that is itself an epic
   becomes a `rowEpic` with its own bar). Cycle-guard the recursion (a `visited`
   set, like `tree.go` `buildNode`).
5. Compute `lastKid`/connector flags during flatten.

Default expand state: **project headers expanded, epics collapsed** (so the
first paint is a scannable lane-of-epics overview; drilling expands an epic).

### Render (`View() string`) - full-bleed

Mirrors `tree.go` `View()`: NO `lipgloss.Place`. Compute `[start,end)` window
over `rows` around `cursor` (reuse the Phase-1 clamp), render each visible row,
clamp each line to `width` via `MaxWidth(width)`. A 1-line header
(`EPICS . <scope> . <mode>    N epics`) and a 1-line footer (key hints), the rest
is the windowed tree body. `↑ N more` / `↓ N more` indicators when scrolled.

Row renderers:
- **`rowProjectHeader`**: `▾ bt ────────…──── 8 active · 24% ────` - expand glyph,
  lane name, a horizontal rule filling the width, and the lane rollup (epic count
  + aggregate %). Collapsed (`▸`) hides the lane's epics.
- **`rowEpic`**: `<connectors> <expand-glyph> <ID> <braille-bar> <pct> <done/total> <⚠N?> <title>`.
  Title truncates to the remaining width (Phase-1 fixed-segment budget method).
  At-risk `⚠N` only when `AtRisk>0`.
- **`rowChild`**: `<connectors> <status-glyph> <ID> <title>`; if the child is an
  epic, append its own mini braille bar + `pct`. Closed children render faint
  (the Phase-1 `buildEpicProgressANSI` recede treatment).

Connectors (`├─ └─ │`) reuse `tree.go` `buildTreePrefix` logic (depth walk +
`lastKid` flags). Status glyphs reuse the theme status colors
(`done`=green-faint, `open`, `in_progress`, `blocked`).

### Braille bar (new `pkg/ui/braille.go`, TDD)

```go
// brailleCompositionBar renders a full-width braille bar whose runs encode
// child-status composition. width is the cell count; each cell is a braille
// glyph (2 sub-columns x 4 dot-rows). Done|InProgress|Blocked render at FULL
// density (⣿) in their theme color; Open renders at LOW density (a dim mid-line
// glyph) in grey. The density difference (not color alone) keeps the boundary
// legible after ansi.Strip. A cell straddling a boundary takes its left
// sub-column's category. Returns an ANSI/lipgloss string (route through the
// ANSI track, never Glamour - see bt-x5xc4).
func brailleCompositionBar(c epicCounts, width int, t Theme) string

// braillePlainBar: monochrome done/total fill (full-density filled run + dim
// low-density track), for the nested child-epic mini bar and anywhere
// composition is moot.
func braillePlainBar(done, total, width int) string
```

Dot-bit packing (verified against the real corpus during brainstorm):
`row0 col0=0x01 col1=0x08 / row1 0x02 0x10 / row2 0x04 0x20 / row3 0x40 0x80`,
base `U+2800`. Full cell = `U+28FF` (`⣿`). 2 horizontal sub-steps per cell.
Braille renders on the target terminal (Windows Terminal; btop4win++ proves the
font has the block).

### Folding the Tier-2 focus card into the tree drill (decided)

The full-sheet tree's own expand + drill subsumes the modal card as the *primary*
epic surface:

- **`⏎` / `→` / `l` on an epic** -> expand (and focus the subtree: move cursor to
  the first child). Collapsed->expanded; on an already-expanded epic, `←`/`h`
  collapses.
- **`⏎` on a child** -> open detail (the existing `selectIssueByID` +
  `focusDetailAfterJump` mechanism that `handleEpicCardKeys` already uses).
- **The modal card stays reachable** for a single-epic "zoom": bind a key
  (proposed **`v`** = view, free in the epics context) that opens the existing
  `ModalEpicCard` on the focused epic. `renderEpicCard` / `handleEpicCardKeys` /
  `buildEpicProgressANSI` are **unchanged** - `buildEpicProgressANSI` also still
  backs the detail-pane Epic Progress block (single source of truth, keep).

Net: `bt-gfxhz.3`'s deliverable is retained; the modal is demoted from the `⏎`
default to an optional `v` zoom. Record this on bt-gfxhz.3 (a comment, not a
reopen).

### Keybindings (extends Phase-1 `EpicsKeys`)

| Key | Action |
|---|---|
| `j`/`k` `↓`/`↑` | move cursor across the flat row list |
| `→`/`l` / `⏎` on epic or header | expand (focus subtree) |
| `←`/`h` | collapse (or jump to parent / lane header) |
| `⏎` on child | open detail (drill) |
| `z` | collapse all epics (back to lane overview) |
| `s` | cycle active / all / completed (Phase-1) |
| `f` | label filter (existing modal) |
| `w` | project picker (existing) |
| `v` | open the single-epic focus card (zoom) for the cursor epic |
| `P` | (reserved, not bound) progress-style toggle -> burn-up sparkline (fast-follow) |
| `esc` | back to list |

Every binding gets a non-empty `Help.Desc` (bt-ift6 key.Map convention;
cardinality tests enforce it).

## What changes vs Phase 1

| File | Change |
|---|---|
| `pkg/ui/epics_tree.go` | **new**: `EpicsTreeModel`, `epicTreeRow`, build/flatten/window/render |
| `pkg/ui/braille.go` | **new**: `brailleCompositionBar`, `braillePlainBar` |
| `pkg/ui/epics_view.go` | `renderEpicsOverview` rewritten to drive `EpicsTreeModel.View()` (delete `lipgloss.Place(Center)` + `min(80)`); `refreshEpicsForCurrentFilter` builds the tree model; `handleEpicsKeys` gains expand/collapse/drill/`v` |
| `pkg/ui/model.go` | replace `epicsRows`/`epicsCursor` with an `epicsTree EpicsTreeModel` field (statusMode stays); `epicsViewText` stays as the cached render |
| `pkg/ui/keys/epics.go` | add Expand/Collapse/CollapseAll/Card(`v`) bindings |
| `pkg/ui/render_harness_test.go` | add tree scenarios: grouped lanes, an expanded epic, scrunched 70x20, a multi-project sketch (inject 2 fixture lanes) |

`epicsOverviewRows` / `EpicRow` / `EpicStatusMode` / `epic_progress.go` /
`epic_card.go` / `buildEpicProgressANSI`: **unchanged**.

## Cross-project nuance (carried from Phase-1)

Parent-child deps store bare IDs; an epic's children are same-project in
practice. Project grouping keys on the ID prefix. When scoped to one project, the
overview shows one lane; cross-project / global scope shows all lanes. The `w`
picker and `W` home/all toggle drive scope as today.

## Testing

- `braille_test.go` (TDD first): `braillePlainBar` exact-glyph assertions at
  0/25/50/75/100% and odd widths; `brailleCompositionBar` segment boundaries
  (all-open -> all grey; all-done -> all green; mixed -> run lengths). Pure, no
  Model.
- `epics_tree_test.go`: `Build` produces correct row order (lanes -> epics ->
  children), root-epic dedup (nested child-epic not double-listed), expand/
  collapse flatten, cycle guard, connector `lastKid` flags, windowing clamp.
- `render_harness_test.go`: `BT_RENDER_DUMP=1` dumps for `epics_tree_100x32`,
  `epics_tree_expanded_120x40`, `epics_tree_70x20` (scrunched), and a
  multi-lane fixture. Read the `.txt` for layout; `.ansi` -> freeze for the
  braille/color check. **Visual sign-off against these dumps is the DONE gate.**
- Key cardinality test for the extended `EpicsKeys` (every binding has Help.Desc).
- Existing `epics_view_keys_test.go` / `epic_card_test.go` stay green (card
  unchanged; nav keys extended).

## Scope

**In:** Tier-1 overview as a full-sheet, project-grouped tree with the braille
composition bar; fold Tier-2 card into the tree drill (keep `v` zoom);
filterable + scope-aware; harness scenarios; responsive at 14-30 rows.

**Out (separate beads):** burn-up sparkline `P` toggle (fast-follow);
`bt-h97e` deep mermaid view; `bt-3ftfm.2` card ghost-cursor bug; the
`viewportWindow` DRY extraction; promoting Epics to lowercase `e`.

## Risks / watch-items

1. **Braille via the right render track.** The colored bar emits lipgloss ANSI;
   it MUST go through the renderSection ANSI track / direct string compositing,
   never Glamour's markdown path (bt-x5xc4 / bt-0qzp class of bug). The epics
   view composites strings directly (no Glamour), so this is satisfied by
   construction - keep it that way.
2. **Width budget at 70 cols.** Bar (10) + pct (4) + counts (6) + connectors +
   ID leave a thin title budget; truncate titles by *plain* width (Phase-1
   method) so lipgloss styling never overflows. Harness `epics_tree_70x20` is the
   gate.
3. **Root-epic dedup.** Forgetting step 2 double-lists nested child-epics. Test
   it explicitly (bt-19vp-under-bt-ushd shape).
4. **Composition-bar boundary color.** A cell straddling done/open picks one
   color; acceptable (sub-cell precision loss of <=1 cell). Document, don't
   over-engineer.
5. **Empty / childless epics.** `EpicsActive` already hides 0/0 epics; `all`
   mode shows them - render their bar as an all-grey track (0/0 -> 0%), no crash
   on divide-by-zero (guard `total==0`).

## Phasing (mirrors the originating plan)

This redesign is one plan: `docs/plans/2026-06-19-epics-tree-redesign.md`. The
burn-up sparkline and the `viewportWindow` extraction are separate follow-up
beads filed at close.
