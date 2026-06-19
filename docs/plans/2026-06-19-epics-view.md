# Epics View - Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repurpose the dead `ViewSprint` dashboard into a working **Epics overview** view (`ViewEpics`) - an all-epics list with progress/status/at-risk, sourced from real bd data - and strip the sprint scaffolding.

**Architecture:** `ViewSprint` is renamed to `ViewEpics` so the existing ViewMode plumbing, focus state, help/context wiring, and render skeleton are reused, not rebuilt. The overview is a **projection over `m.filteredIssuesForActiveView()`** (the canonical filtered/scoped/sorted set), so project scope, label filter, and sort inherit for free. Order of operations keeps the build green at every step: rename first, repoint the render to epic data, reshuffle keys, then delete the now-dead sprint code last.

**Tech Stack:** Go 1.25+, Charm Bracelet v2 (Bubble Tea / Lipgloss), `charm.land/bubbles/v2/key` key.Map dispatch.

**Spec:** `docs/design/2026-06-19-epics-view.md` (read it first).

## Global Constraints

- `go build ./...` and `go vet ./...` must pass after every task (AGENTS.md rule 7).
- `go install ./cmd/bt/` after every successful build (user runs `bt` from PATH).
- No file deletion without the deletions being explicitly listed here (they are, Task 7) - confirm with the user before Task 7 if unsure.
- No `bd` non-ASCII via inline strings; use `-f`/`--*-file` for rich content.
- key.Map convention (bt-ift6): every `key.Binding` has a non-empty `Help.Desc`; cardinality tests catch dropped bindings.
- Commits: `type(scope): description (bt-<id>)`, scope `tui`.
- TUI rendering verification: `BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump` then read `_tmp/render/*.txt` (see `docs/design/` SOP / memory).

## Roadblocks the implementer will hit (read before starting)

1. **The rename fans out.** `ViewSprint`/`focusSprint`/`ContextSprint`/`sprintViewText`/`m.sprints`/`m.selectedSprint` appear across ~10 files (model.go, model_view.go, model_modes.go, context.go, help_keys.go, model_update_input.go, model_update_data.go, sprint_view.go, context_test.go, sprint_view_keys_test.go). Use `git grep -n` to find every site; the build will tell you what you missed.
2. **Status filter vs progress.** The overview's status filter must NOT filter children out of the progress bar - `epicProgress()` must run over the *full* child set, not the status-filtered set. This is the one place the "projection over filteredIssuesForActiveView" rule is overridden (Task 4).
3. **Keybinding dispatch order.** Global view-switch keys fire in the dispatcher switch (model_update_input.go ~864-1296) BEFORE per-view focus handlers. Moving Tree to a global `T` retires list's local `T`; confirm no test asserts list `T` behavior (Task 3).
4. **Strip last.** Do NOT delete `Sprint`/`loader/sprint.go`/`robot_sprint.go` until the epics render no longer references them (Task 7), or the build breaks mid-way.
5. **`filteredIssuesForActiveView()` is the integration seam** (model_filter.go ~272-321). It already applies scope/label/status/sort. Read it before Task 4.

---

## File Structure

- `pkg/ui/model.go` - rename `ViewSprint`->`ViewEpics`, `focusSprint`->`focusEpics`; replace `sprints []model.Sprint`/`selectedSprint`/`sprintViewText` fields with `epicsViewText string` (+ overview state struct).
- `pkg/ui/epics_view.go` - **new** (renamed from `sprint_view.go`): `renderEpicsOverview()`, `handleEpicsKeys()`, the epic-row/progress/at-risk render helpers.
- `pkg/ui/keys/epics.go` - **new**: `EpicsKeys` key.Map (navigation within the overview).
- `pkg/ui/keys/global.go` - `Epics` binding on `E`; `Tree` moves to `T`; remove `Sprint`.
- `pkg/ui/keys/list.go` - remove the `T` (time-travel HEAD~5) binding (retired).
- `pkg/ui/model_update_input.go` - global dispatch: `Epics`(E)/`Tree`(T) cases; esc/q cascades for `ViewEpics`; focus dispatch `focusEpics`->`handleEpicsKeys`.
- `pkg/ui/model_view.go` - `ViewEpics` render case.
- `pkg/ui/model_modes.go`, `context.go`, `help_keys.go` - rename sprint->epics cases.
- `pkg/ui/model_update_data.go` - replace the `sprints.jsonl` reload block with `refreshEpicsForCurrentFilter()`.
- `pkg/ui/helpers_epics.go` - **new**: `epicsOverviewRows(filtered, statusMode)` pure function (testable without a Model).
- `pkg/ui/render_harness_test.go` - repoint the sprint scenarios to epics.
- **Delete (Task 7):** `pkg/model` Sprint struct + methods, `pkg/loader/sprint.go`(+test), `cmd/bt/robot_sprint.go`, sprint pieces of `cmd/bt/burndown.go` + `cmd/bt/cobra_robot.go`, `.beads/sprints.jsonl`.

---

### Task 1: Rename ViewSprint -> ViewEpics (mechanical, build stays green)

**Files:** Modify every site of `ViewSprint`/`focusSprint`/`ContextSprint` (find with `git grep -n "ViewSprint\|focusSprint\|ContextSprint"`). Keep `sprint_view.go` and its data for now (renamed types still reference `m.sprints` - that's fine until Task 4).

**Interfaces produced:** `ViewEpics ViewMode`, `focusEpics focus`, `ContextEpics` - the constants every later task references.

- [ ] **Step 1:** `git grep -n "ViewSprint\|focusSprint\|ContextSprint" pkg/ui` - list all sites.
- [ ] **Step 2:** Rename the constant declarations in `model.go` (`ViewSprint`->`ViewEpics` line ~93, `focusSprint`->`focusEpics` line ~72) and `context.go` (`ContextSprint`->`ContextEpics`).
- [ ] **Step 3:** Update all references (model_view.go:180, model_modes.go:408/447, context.go:102, help_keys.go:30, model_update_input.go:1370, the global dispatch Sprint case, context_test.go, sprint_view_keys_test.go).
- [ ] **Step 4:** `go build ./... && go vet ./...` - Expected: PASS (pure rename).
- [ ] **Step 5:** `go test ./pkg/ui/...` - Expected: PASS.
- [ ] **Step 6:** Commit: `refactor(tui): rename ViewSprint to ViewEpics (bt-ryi5z)`

---

### Task 2: Pure overview-rows function (TDD, no Model)

**Files:** Create `pkg/ui/helpers_epics.go`, `pkg/ui/helpers_epics_test.go`.

**Interfaces produced:**
```go
type EpicStatusMode int
const ( EpicsActive EpicStatusMode = iota; EpicsAll; EpicsCompleted )

type EpicRow struct {
    Epic       model.Issue
    Done, Total int        // progress, children ALWAYS counted in full
    InProgress, Blocked, Open int
    AtRisk     int          // children in_progress with >=3d no update
}

// epicsOverviewRows partitions epics out of `all` (the full issue set, NOT
// status-filtered), computes per-epic counts via parent-child deps, and keeps
// only epics matching statusMode. `all` should already be scope/label filtered.
func epicsOverviewRows(all []model.Issue, statusMode EpicStatusMode, now time.Time) []EpicRow
```

- [ ] **Step 1: Write the failing test** in `helpers_epics_test.go`:
```go
func TestEpicsOverviewRows(t *testing.T) {
    now := time.Now()
    ago := func(d time.Duration) time.Time { return now.Add(-d) }
    pc := func(child, parent string) []*model.Dependency {
        return []*model.Dependency{{IssueID: child, DependsOnID: parent, Type: model.DepParentChild}}
    }
    all := []model.Issue{
        {ID: "ep1", IssueType: model.TypeEpic, Status: model.StatusOpen},
        {ID: "ep1.a", Status: model.StatusClosed, Dependencies: pc("ep1.a", "ep1")},
        {ID: "ep1.b", Status: model.StatusInProgress, UpdatedAt: ago(5 * 24 * time.Hour), Dependencies: pc("ep1.b", "ep1")},
        {ID: "ep1.c", Status: model.StatusOpen, Dependencies: pc("ep1.c", "ep1")},
        {ID: "ep2", IssueType: model.TypeEpic, Status: model.StatusClosed}, // all children done
        {ID: "ep2.a", Status: model.StatusClosed, Dependencies: pc("ep2.a", "ep2")},
    }
    rows := epicsOverviewRows(all, EpicsActive, now)
    if len(rows) != 1 || rows[0].Epic.ID != "ep1" {
        t.Fatalf("EpicsActive want [ep1], got %v", rows)
    }
    r := rows[0]
    if r.Done != 1 || r.Total != 3 { t.Errorf("progress = %d/%d, want 1/3", r.Done, r.Total) }
    if r.AtRisk != 1 { t.Errorf("at-risk = %d, want 1 (ep1.b stale)", r.AtRisk) }
    if all2 := epicsOverviewRows(all, EpicsAll, now); len(all2) != 2 {
        t.Errorf("EpicsAll want 2 epics, got %d", len(all2))
    }
    if comp := epicsOverviewRows(all, EpicsCompleted, now); len(comp) != 1 || comp[0].Epic.ID != "ep2" {
        t.Errorf("EpicsCompleted want [ep2], got %v", comp)
    }
}
```
- [ ] **Step 2: Run, verify FAIL** - `go test ./pkg/ui -run TestEpicsOverviewRows` - Expected: FAIL (undefined: epicsOverviewRows).
- [ ] **Step 3: Implement** `epicsOverviewRows` in `helpers_epics.go`. Reuse `epicChildrenSorted`/`isClosedLikeStatus` (already in package). Count children fully; `EpicsActive` keeps epics with >=1 non-closed child, `EpicsCompleted` keeps epics whose children are all closed (and Total>0), `EpicsAll` keeps every `TypeEpic`. At-risk: child `StatusInProgress` && `now.Sub(child.UpdatedAt) >= 3*24h`.
- [ ] **Step 4: Run, verify PASS** - `go test ./pkg/ui -run TestEpicsOverviewRows` - Expected: PASS.
- [ ] **Step 5: Commit** `feat(tui): epicsOverviewRows projection helper (bt-ryi5z)`

---

### Task 3: Keybinding reshuffle - E=Epics, T=Tree, retire list T

**Files:** Modify `pkg/ui/keys/global.go`, `pkg/ui/keys/list.go`, create `pkg/ui/keys/epics.go`, modify `pkg/ui/model_update_input.go`.

**Interfaces produced:** `m.keys.Global.Epics` (E), `m.keys.Global.Tree` (T), `EpicsKeys` map.

- [ ] **Step 1:** `global.go`: rename the `Sprint` field/binding -> `Epics` with `WithKeys("E")`, `WithHelp("E","epics")`. Change `Tree` binding from `WithKeys("E")` to `WithKeys("T")`, `WithHelp("T","tree")`. Update `FullHelp` Views column (replace `k.Sprint`/keep `k.Tree`).
- [ ] **Step 2:** `list.go`: remove the `TimeTravelHead5` binding (`WithKeys("T")`, line ~157 area) and its `FullHelp`/handler reference (retired). Keep lowercase `t` time-travel.
- [ ] **Step 3:** `model_update_input.go`: in the global dispatch, the existing Sprint case (the `key.Matches(msg, m.keys.Global.Sprint)` block) becomes `m.keys.Global.Epics` and sets `m.mode = ViewEpics; m.focused = focusEpics; m.epicsViewText = m.renderEpicsOverview()`. Add/confirm the `m.keys.Global.Tree` case toggles `ViewTree`. Update the `ViewSprint`->`ViewEpics` branches in the q and esc cascades (renamed in Task 1).
- [ ] **Step 4:** Create `pkg/ui/keys/epics.go` `EpicsKeys` map: `Up`(k/up), `Down`(j/down), `Open`(enter, "open epic"), `CycleStatus`(s, "active/all/completed"), `Back`(esc). Each with non-empty Help.Desc.
- [ ] **Step 5:** `go build ./... && go vet ./...` - Expected: PASS.
- [ ] **Step 6:** Run existing key tests `go test ./pkg/ui/keys/... ./pkg/ui -run 'Key|Help'` - Expected: PASS (fix any test asserting old `E`=tree or list `T`).
- [ ] **Step 7:** Commit `feat(tui): E=epics, T=tree keybindings; retire list T (bt-ryi5z)`

---

### Task 4: Wire the overview into the Model (data + render)

**Files:** Modify `pkg/ui/model.go` (state), `pkg/ui/epics_view.go` (rename from sprint_view.go; `renderEpicsOverview`, `handleEpicsKeys`), `pkg/ui/model_view.go` (ViewEpics case), `pkg/ui/model_update_data.go` (refresh).

**Interfaces consumed:** `epicsOverviewRows` (Task 2), `filteredIssuesForActiveView()` (existing), `EpicsKeys` (Task 3).
**Interfaces produced:** `m.renderEpicsOverview() string`, `m.refreshEpicsForCurrentFilter()`, `m.epicsViewText`, `m.epicsCursor int`, `m.epicsStatusMode EpicStatusMode`.

- [ ] **Step 1:** `model.go`: replace `sprints`/`selectedSprint`/`sprintViewText` fields with `epicsViewText string`, `epicsRows []EpicRow`, `epicsCursor int`, `epicsStatusMode EpicStatusMode`.
- [ ] **Step 2:** `git mv pkg/ui/sprint_view.go pkg/ui/epics_view.go`. Rewrite `renderSprintDashboard` -> `renderEpicsOverview()`: header "Epics (<mode>)", then for each `EpicRow` a line with progress bar (reuse the existing bar code), `checkmark/in-progress/blocked/open` counts, and an at-risk marker when `AtRisk>0`. Cursor highlight on `epicsCursor`. Keep the box/`lipgloss.Place` wrapper. **Drop** the dates/days-remaining/burndown blocks (epics have no window).
- [ ] **Step 3:** Rewrite `handleSprintKeys` -> `handleEpicsKeys` using `EpicsKeys`: j/k move `epicsCursor` (clamp to len(rows)); `s` cycles `epicsStatusMode` then `refreshEpicsForCurrentFilter()`; enter -> (Phase 2 focus card; for now no-op or status msg); esc -> `ViewList`.
- [ ] **Step 4:** `model_update_data.go`: replace BOTH `sprints.jsonl` reload blocks with a call to `m.refreshEpicsForCurrentFilter()` (only when `m.mode == ViewEpics`). Implement it: `rows := epicsOverviewRows(m.filteredIssuesForActiveView(), m.epicsStatusMode, time.Now()); sort rows by Done/Total asc (progress %); m.epicsRows = rows; m.epicsViewText = m.renderEpicsOverview()`.
  - **Roadblock note:** pass `m.filteredIssuesForActiveView()` (scope+label+sort) but `epicsOverviewRows` counts children fully regardless of the status filter - that is the intended override (see spec). If `filteredIssuesForActiveView` drops closed issues under a status=open list filter, epic progress would undercount; verify by reading that function - if it excludes closed, source the rows from `m.data.issues` filtered by `activeRepos`+label only (use `workspacePrefilter`), NOT the status-filtered set.
- [ ] **Step 5:** `model_view.go`: `case ViewEpics: body = m.epicsViewText` (already there post-rename; confirm).
- [ ] **Step 6:** `model_update_input.go`: the Epics(E) case sets up cursor/mode and calls `refreshEpicsForCurrentFilter()`. `applyFilter`/`applyRecipe` also call it (mirror `refreshBoardAndGraphForCurrentFilter`).
- [ ] **Step 7:** `go build ./... && go vet ./... && go install ./cmd/bt/` - Expected: PASS.
- [ ] **Step 8:** Commit `feat(tui): epics overview view sourced from filtered set (bt-ryi5z)`

---

### Task 5: Render harness + visual check

**Files:** Modify `pkg/ui/render_harness_test.go` (repoint the `sprintView` setup to build epics from `harnessIssues` - the fixture already has `bt-evuf` epic with child `bt-evuf.1`).

- [ ] **Step 1:** Rename the `sprintView` setup closure -> `epicsView`; set `m.mode=ViewEpics; m.focused=focusEpics; m.refreshEpicsForCurrentFilter()`. Rename scenarios `sprint_*` -> `epics_*`.
- [ ] **Step 2:** `BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump` then read `_tmp/render/epics_100x32.txt` and `epics_70x20.txt`.
- [ ] **Step 3:** Verify: rows render, progress bars correct, no box-overflow at 70 wide (FIX the bead-title wrap that broke the sprint render - truncate titles to fit `innerWidth`). Add a viewport/scroll if rows exceed height (the sprint render clipped with no scroll - do not repeat).
- [ ] **Step 4:** Commit `test(tui): epics overview render harness scenarios (bt-ryi5z)`

---

### Task 6: Default sort + status-filter cycle polish

- [ ] **Step 1:** Confirm default sort is progress % ascending (least-complete first) in `refreshEpicsForCurrentFilter`. Add a test asserting row order for a 2-epic fixture (20% before 80%).
- [ ] **Step 2:** Confirm `s` cycles active->all->completed and the header label updates; add a key test.
- [ ] **Step 3:** `go test ./pkg/ui/...` - Expected: PASS.
- [ ] **Step 4:** Commit `feat(tui): epics overview default progress sort + status cycle (bt-ryi5z)`

---

### Task 7: Strip the sprint scaffolding (DELETIONS - confirm with user first)

**Files - DELETE:** `pkg/loader/sprint.go`, `pkg/loader/sprint_test.go`, `cmd/bt/robot_sprint.go`, `.beads/sprints.jsonl`. **Modify (remove sprint code):** `pkg/model/types.go` (Sprint struct + Validate/IsActive), `cmd/bt/cobra_robot.go` (robotSprint* commands + AddCommand), `cmd/bt/burndown.go` (sprint-only helpers), any remaining `loader.SprintsFileName`/`LoadSprints` references.

- [ ] **Step 1:** `git grep -rn "Sprint\|sprints\.jsonl\|LoadSprints\|robotSprint" pkg cmd | grep -v epics` - confirm nothing in the live epics path references these.
- [ ] **Step 2:** Get explicit user go-ahead for the deletions (AGENTS.md rule 1).
- [ ] **Step 3:** Delete the files and remove the sprint code from the modify-list files.
- [ ] **Step 4:** `go build ./... && go vet ./... && go test ./... && go install ./cmd/bt/` - Expected: PASS.
- [ ] **Step 5:** Commit `refactor: strip dead sprint feature, superseded by epics view (bt-ryi5z)`
- [ ] **Step 6:** Close `bt-ryi5z` with the outcome; note `bt-gfxhz.3` and Phase 2 as follow-ons.

---

## Phases 2-3 (separate plans, after Phase 1 lands)

- **Phase 2 - Epic focus card (modal):** `enter` on an overview row + a focus key on an epic in list/detail. Shared lipgloss status-pill renderer for children = `bt-gfxhz.3`'s deliverable, used by both the modal and the detail-pane Epic Progress embed. Modal compositing per `docs/design/tui-modal-compositing.md`. Write this plan after Phase 1, when `EpicRow`/`epics_view.go` exist to build against.
- **Phase 3 - `bt robot epics`:** robot `bd epic status` (epics + progress) replacing the removed `bt robot sprint`.

## Beads

- Epic: "Epics view (repurpose sprint scaffolding)" - parent, resolves the direction of `bt-ryi5z`.
- Children: T1-rename, T2-rows-helper, T3-keys, T4-wire, T5-harness, T6-sort, T7-strip (Phase 1); plus Phase-2 and Phase-3 child beads.
- Links: `bt-ryi5z` (closed by T7), `bt-gfxhz.3` (delivered in Phase 2), `bt-h97e` (complemented, unchanged).

## Self-review notes

- Spec coverage: 3-tier design (Phase 1 = tier 1; tiers 2/3 = Phases 2/3), data source (Task 4), status override (Tasks 2+4), keys (Task 3), strip (Task 7), sort (Task 6), robot epics (Phase 3) - all mapped.
- Open risk carried into execution: the `filteredIssuesForActiveView` closed-children question (Task 4 Step 4) - the implementer must read that function and branch accordingly; flagged inline rather than guessed.
