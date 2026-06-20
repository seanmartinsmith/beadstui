# Epics overview redesign - Full-sheet hierarchy tree (Tier-1) - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: use superpowers:test-driven-development
> per task (write the failing test, watch it fail, implement, watch it pass).
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Phase-1 epics overview (a centered `max-80` box with a flat
epic-row list) with a **full-sheet, project-grouped hierarchy tree**: epic ->
children as an indented tree, each node carrying a **braille composition progress
bar**, grouped under collapsible project swimlanes, filterable + scope-aware. Fold
the Tier-2 focus card into the tree drill (keep a `v` zoom). Closes **bt-3ftfm.1**.

**Spec (read first):** `docs/design/2026-06-19-epics-tree-redesign.md`.
**Supersedes:** the Tier-1 render in `docs/design/2026-06-19-epics-view.md`. The
Phase-1 *data layer* is kept unchanged.

**Tech Stack:** Go 1.25+, Charm Bracelet v2 (Bubble Tea / Lipgloss),
`charm.land/bubbles/v2/key` key.Map dispatch.

## Orientation (targeted reads - do NOT read these files wholesale)

Substrate to copy (mapped during brainstorm; line numbers approximate):
- `pkg/ui/tree.go` - the windowing/connector model to mirror (NOT reuse directly):
  - `buildTreePrefix` ~:626-657 - connector glyphs (`├─ └─ │`) from a depth walk +
    is-last-child flags. **Copy this logic** for epic/child rows.
  - `visibleRange` ~:922-951 + `ensureCursorVisible` ~:1060-1094 - `[start,end)`
    window + cursor-follows-viewport. **Mirror the approach** (the Phase-1 box
    already has a simpler version at `epics_view.go` ~:88-119).
  - `View` ~:477-522 - full-bleed pattern: NO `lipgloss.Place`, render windowed
    lines, `MaxWidth(width)` per line. **Mirror.**
  - `buildNode` ~:371-419 - cycle detection (`visited` map). **Copy the guard.**
- `pkg/ui/epics_view.go` - the file being rewritten:
  - `renderEpicsOverview` ~:63-141 - **delete** the `lipgloss.Place(Center)` +
    `min(80, width-4)` box (~:66, ~:134); drive `EpicsTreeModel.View()` instead.
  - `refreshEpicsForCurrentFilter` ~:27-56 - **keep** the sourcing
    (workspacePrefilter + label/wisp filter + `epicsOverviewRows`); repoint the
    tail to build the tree model.
  - `renderEpicRow` ~:147-209 - **superseded** by the tree row renderers (the
    fixed-segment title-budget method ~:158-164 is the pattern to reuse).
  - `handleEpicsKeys` ~:213-240 - **extend** (expand/collapse/drill/`v`).
- `pkg/ui/helpers_epics.go` - `epicsOverviewRows` ~:75, `EpicRow` ~:65,
  `EpicStatusMode` ~:13, `epicProgressFraction` ~:55. **Unchanged**; reuse.
- `pkg/ui/epic_card.go` - `renderEpicCard`, `handleEpicCardKeys`,
  `openEpicCard`/`ModalEpicCard`. **Unchanged**; reached via the new `v` key.
- `pkg/ui/epic_progress.go` - `buildEpicProgressANSI`. **Unchanged** (detail pane +
  the `v` card still use it).
- `pkg/ui/model_view.go` ~:183 - `case ViewEpics: body = m.epicsViewText`. Unchanged.
- Helpers to reuse: `epicChildrenSorted`, `isClosedLikeStatus`, `truncateString`,
  theme status colors (`t.Open`/`t.Feature`/`t.Blocked`/`ColorMuted`). Check for an
  existing project-prefix helper (graph.go clusters by project) before adding one.

## Global Constraints

- `go build ./...` and `go vet ./...` pass after **every task** (AGENTS.md rule 7).
- `go install ./cmd/bt/` after every successful build (user runs `bt` from PATH).
- TUI verification: `BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump`, then
  read `_tmp/render/epics_tree_*.txt` (layout) and freeze the `.ansi` (braille +
  color). Per the tui-dev rendering SOP.
- key.Map convention (bt-ift6): every `key.Binding` has a non-empty `Help.Desc`;
  cardinality tests enforce it.
- No `bd` non-ASCII via inline strings; use `-f`/`--*-file`.
- Commits: `type(scope): description (bt-3ftfm.1)`, scope `tui`.
- No file deletion beyond what this plan lists. (None planned - the Phase-1 epics
  files are rewritten in place, not deleted.)

## Roadblocks (read before starting)

1. **Braille must not go through Glamour.** The colored bar emits lipgloss ANSI;
   the epics view composites strings directly (no Glamour), so route it through
   the direct string path - never `addMD`. Mirrors bt-x5xc4 / bt-0qzp.
2. **Root-epic dedup.** Epics that are parent-child children of another in-scope
   epic must nest, not double-list (bt-19vp / bt-ph1z under bt-ushd). Test it.
3. **Status-filter override is already handled** by `epicsOverviewRows` - source
   the tree from the workspace/label-prefiltered set (NOT the status-filtered
   list), exactly as `refreshEpicsForCurrentFilter` does today. Don't re-derive.
4. **Width budget at 70 cols.** Truncate titles by *plain* width (the
   `renderEpicRow` fixed-segment method), so lipgloss styling never overflows.
5. **Divide-by-zero.** Childless epics (`all` mode) -> guard `total==0` in the bar.

---

### Task 1: Braille bars (TDD, pure - no Model)

**Files:** create `pkg/ui/braille.go`, `pkg/ui/braille_test.go`.

**Interfaces produced:** `braillePlainBar(done, total, width int) string`,
`brailleCompositionBar(c epicCounts, width int, t Theme) string`, `epicCounts`
struct (`Done, Total, InProgress, Blocked, Open, AtRisk int`).

- [ ] **Step 1 (RED):** `braille_test.go` - assert exact glyphs for
  `braillePlainBar`: 0% -> all dim-track glyph; 100% width=10 -> ten `⣿`; 50%
  width=10 -> five `⣿` + five track; an odd fraction exercising the 2-sub-step
  cell (e.g. done=1,total=4,width=4 -> one `⣿` + a half cell + track). Assert
  `brailleCompositionBar` run lengths: all-open -> all track; all-done -> all
  `⣿`; mixed (done=2,inprog=1,blocked=1,open=4,width=8) -> 2 done + 1 ip + 1 bl +
  4 track cells (verify via `ansi.Strip` glyph counts, color via substring).
- [ ] **Step 2:** Run `go test ./pkg/ui -run TestBraille` - expect FAIL (undefined).
- [ ] **Step 3 (GREEN):** Implement the dot-bit packer (spec "Braille bar"):
  base `U+2800`, dot map `r0:0x01/0x08 r1:0x02/0x10 r2:0x04/0x20 r3:0x40/0x80`,
  full cell `⣿`. Filled run = full-density `⣿` (composition: theme-colored per
  segment); track = a low-density glyph (`⠒`/`⠤`) in `ColorMuted`. 2 horizontal
  sub-steps/cell. Guard `total==0`. Composition cell straddling a boundary takes
  its left sub-column's category.
- [ ] **Step 4:** Run - expect PASS.
- [ ] **Step 5:** `go build ./... && go vet ./...` - PASS.
- [ ] **Step 6:** Commit `feat(tui): braille progress bars for epics tree (bt-3ftfm.1)`.

---

### Task 2: EpicsTreeModel build + flatten (TDD, pure)

**Files:** create `pkg/ui/epics_tree.go`, `pkg/ui/epics_tree_test.go`.

**Interfaces produced:** `EpicsTreeModel`, `epicTreeRow`, `epicTreeRowKind`
(`rowProjectHeader`/`rowEpic`/`rowChild`), `projectPrefix(id string) string` (if
no existing helper), and:
```go
func (e *EpicsTreeModel) Build(scoped []model.Issue, mode EpicStatusMode, now time.Time)
func (e *EpicsTreeModel) rows() []epicTreeRow   // flattened, expansion-aware
```

- [ ] **Step 1 (RED):** `epics_tree_test.go` with a fixture: 2 projects, an epic
  with mixed-status children, a **nested child-epic** (epic B is a parent-child
  child of epic A), a childless epic. Assert:
  - lanes appear as `rowProjectHeader`, grouped by prefix, in count-desc order;
  - within a lane, epics sort by progress % asc;
  - **root-epic dedup**: epic B appears only nested under A, never as a top row;
  - default expand state: headers expanded, epics collapsed -> rows = headers +
    epics, no child rows until an epic is expanded;
  - after `expand(epicA.ID)`: A's children appear as `rowChild` (and nested epic
    B as a `rowEpic` with `hasKids`); `lastKid` flags correct for connectors;
  - cycle guard: a self/loop parent-child edge does not infinite-loop.
- [ ] **Step 2:** Run - expect FAIL.
- [ ] **Step 3 (GREEN):** Implement `Build` per spec "Build pipeline": reuse
  `epicsOverviewRows(scoped, mode, now)` for the `[]EpicRow`; drop nested
  child-epics from roots; group by `projectPrefix`; flatten with expand state +
  cycle guard + `lastKid`. `expand`/`collapse`/`collapseAll`/`toggle` mutate the
  `expanded` map and re-flatten.
- [ ] **Step 4:** Run - expect PASS.
- [ ] **Step 5:** `go build ./... && go vet ./...` - PASS.
- [ ] **Step 6:** Commit `feat(tui): EpicsTreeModel build + flatten (bt-3ftfm.1)`.

---

### Task 3: EpicsTreeModel render (full-bleed View)

**Files:** modify `pkg/ui/epics_tree.go`.

**Interfaces produced:** `func (e *EpicsTreeModel) SetSize(w, h int)`,
`func (e *EpicsTreeModel) View() string`, cursor/window helpers
(`moveCursor`, `window() (start, end int)`).

- [ ] **Step 1:** Implement `View()`: 1-line header (`EPICS · <scope> · <mode>
  N epics`), windowed body over `rows()` (mirror `visibleRange`/`ensureCursorVisible`),
  1-line footer hints, `↑/↓ N more` indicators. Per-row renderers:
  - `rowProjectHeader`: expand glyph + lane name + width-filling rule + rollup
    (`N active · P%`).
  - `rowEpic`: connectors + expand glyph + ID + `brailleCompositionBar` + pct +
    `done/total` + `⚠N`(if>0) + title (truncate by plain width).
  - `rowChild`: connectors + status glyph + ID + title; nested epic child also
    gets a `braillePlainBar` + pct; closed children render faint.
  - NO `lipgloss.Place`; `MaxWidth(e.width)` per line.
- [ ] **Step 2:** No new unit test here (covered by the harness in Task 6);
  `go build ./... && go vet ./...` - PASS.
- [ ] **Step 3:** Commit `feat(tui): EpicsTreeModel full-bleed render (bt-3ftfm.1)`.

---

### Task 4: Wire into Model + delete the box

**Files:** modify `pkg/ui/model.go`, `pkg/ui/epics_view.go`,
`pkg/ui/model_update_data.go` (if it calls the refresh).

- [ ] **Step 1:** `model.go` - replace `epicsRows []EpicRow` + `epicsCursor int`
  with `epicsTree EpicsTreeModel`. Keep `epicsStatusMode`, `epicsViewText`.
  Initialize `epicsTree.expanded` map in `NewModel`.
- [ ] **Step 2:** `epics_view.go` - `refreshEpicsForCurrentFilter`: after building
  `scoped`, call `m.epicsTree.Build(scoped, m.epicsStatusMode, time.Now())`,
  `m.epicsTree.SetSize(m.bodyWidth(), m.height-1)` (match the Tree/Graph dims),
  then `m.epicsViewText = m.epicsTree.View()`. **Delete** `renderEpicsOverview`'s
  box (the `lipgloss.Place` + `min(80)` + `boxStyle`) and `renderEpicRow`
  (superseded). Keep a thin `renderEpicsOverview` only if `model_view.go` still
  calls it - else point `ViewEpics` straight at `m.epicsViewText` (already does).
- [ ] **Step 3:** Ensure size changes re-render: on `WindowSizeMsg` while
  `ViewEpics`, call the refresh (mirror how Tree handles resize).
- [ ] **Step 4:** `go build ./... && go vet ./... && go install ./cmd/bt/` - PASS.
- [ ] **Step 5:** Commit `feat(tui): drive epics overview from EpicsTreeModel; drop centered box (bt-3ftfm.1)`.

---

### Task 5: Keys - expand / collapse / drill / card-zoom

**Files:** modify `pkg/ui/keys/epics.go`, `pkg/ui/epics_view.go` (`handleEpicsKeys`),
`pkg/ui/keys/epics_*_test.go` (cardinality).

- [ ] **Step 1:** `keys/epics.go` - add `Expand`(`l`/`right`/`enter`),
  `Collapse`(`h`/`left`), `CollapseAll`(`z`), `Card`(`v`). Keep `Up`/`Down`/
  `CycleStatus`/`Open`/`Exit`. Every binding a non-empty `Help.Desc`. Verify `v`
  is free in the epics context.
- [ ] **Step 2:** `handleEpicsKeys` - dispatch via `key.Matches`:
  - Down/Up -> `m.epicsTree.moveCursor(±1)`, re-render.
  - Expand/Open on a `rowEpic` or `rowProjectHeader` -> expand + focus subtree;
    Open on a `rowChild` -> `selectIssueByID` + `focusDetailAfterJump` (drill).
  - Collapse -> collapse node / jump to parent.
  - CollapseAll (`z`) -> collapse all epics.
  - Card (`v`) -> `m.openEpicCard(<cursor epic id>)` (works on epic or child's
    parent epic).
  - CycleStatus (`s`) -> Phase-1 behavior + rebuild. Exit (`esc`) -> ViewList.
  - Re-render `m.epicsViewText` after mutations.
- [ ] **Step 3:** Cardinality/help test for the extended `EpicsKeys`.
- [ ] **Step 4:** `go build ./... && go vet ./... && go test ./pkg/ui/... && go install ./cmd/bt/` - PASS.
- [ ] **Step 5:** Commit `feat(tui): epics tree expand/collapse/drill + v zoom (bt-3ftfm.1)`.

---

### Task 6: Render harness + VISUAL sign-off (the DONE gate)

**Files:** modify `pkg/ui/render_harness_test.go`.

- [ ] **Step 1:** Replace/extend the `epicsView` setup: build the tree, expand
  one epic (bt-evuf). Add a **multi-lane** fixture (inject a 2nd-project epic +
  children so swimlanes show >1 lane). Scenarios:
  `epics_tree_100x32`, `epics_tree_expanded_120x40`, `epics_tree_70x20`
  (scrunched), `epics_tree_multilane_120x40`.
- [ ] **Step 2:** `BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump`; read
  each `_tmp/render/epics_tree_*.txt`. Verify: full-bleed (no centered box),
  lanes + connectors render, braille bars present, no overflow at 70 wide,
  cursor + `↑/↓ more` correct.
- [ ] **Step 3:** Freeze the `.ansi` -> PNG for the braille + composition color
  check (per SOP Tier 3a). **Hand the dumps/PNGs to the user for visual sign-off
  - this is the acceptance gate; iterate on layout until approved.**
- [ ] **Step 4:** Commit `test(tui): epics tree render harness scenarios (bt-3ftfm.1)`.

---

### Task 7: Close-out

- [ ] **Step 1:** Record the Tier-2 fold decision on **bt-gfxhz.3** (a comment via
  `bd comments add -f`: card retained, demoted from `⏎` default to the `v` zoom;
  `buildEpicProgressANSI` still single source of truth). Do NOT reopen it.
- [ ] **Step 2:** File follow-up beads: (a) burn-up sparkline `P` progress-style
  toggle (area:tui, ux); (b) `viewportWindow` DRY extraction shared by
  `tree.go` + `epics_tree.go` (area:tui, refactor). Link both to bt-3ftfm.
- [ ] **Step 3:** `go build ./... && go vet ./... && go test ./... -race && go install ./cmd/bt/` - all PASS.
- [ ] **Step 4:** Update `CHANGELOG.md` (session-indexed entry) + ADR-002 stream
  status if applicable.
- [ ] **Step 5:** Close **bt-3ftfm.1** with the Summary/Change/Files/Verify/Risk/
  Notes format (`bd close --reason-file`).
- [ ] **Step 6:** Session-completion protocol (AGENTS.md): `git pull --rebase`,
  `bd dolt push`, `git push`, confirm `git status` clean. (If on a worktree
  branch, merge/PR to `main` per the user's finish-branch preference first.)

## Self-review notes

- Spec coverage: full-bleed (T4), project grouping (T2/T3), braille composition
  bar (T1/T3), tree drill + folded card (T5), filter/scope (inherited, T4),
  harness + visual gate (T6), close-out + follow-ups (T7) - all mapped.
- Green at every step: T1 pure, T2 pure, T3 render-only, T4 swaps the wiring,
  T5 keys, T6 tests. No mid-task broken build.
- Carried risk: the `bodyWidth()`/resize seam (T4 Step 3) - confirm how Tree
  re-renders on `WindowSizeMsg` and mirror it; flagged, not guessed.
- Out of scope (separate beads): sparkline `P` toggle, `viewportWindow` extract,
  bt-h97e deep view, bt-3ftfm.2 ghost-cursor bug.
