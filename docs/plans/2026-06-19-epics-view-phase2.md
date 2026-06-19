# Epics View - Phase 2 Implementation Plan (focus card + shared pill renderer)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land **tier 2** of the epics stack - a centered **epic focus-card modal** drilled into from the overview (`enter`) and from the list/detail (a focus key) - and the **shared lipgloss status-pill renderer** that backs both the card and the existing detail-pane Epic Progress block. The shared renderer is `bt-gfxhz.3`'s deliverable; Phase 2 closes it.

**Spec:** `docs/design/2026-06-19-epics-view.md` -> "Tier 2: Epic focus card" (read it first).
**Phase 1 plan (landed):** `docs/plans/2026-06-19-epics-view.md`.
**Modal pattern:** `docs/design/tui-modal-compositing.md` (OverlayCenterDimBackdrop).

**Tech Stack:** Go 1.25+, Charm Bracelet v2 (Bubble Tea / Lipgloss), `charm.land/bubbles/v2/key` key.Map dispatch.

## The shape (decided in the spec)

```
Overview row  --enter-->  ┌─ Epic bt-evuf ───────────────┐
List/detail   --e------>  │ 3 / 7 complete (42%)  ⚠1      │
 (on an epic)             │                              │
                          │  ▸ [DONE] P1 bt-evuf.1 — ...  │  j/k move
                          │    [PROG] P0 bt-evuf.2 — ...  │  ⏎ drill into child
                          │    [OPEN] P2 bt-evuf.3 — ...  │  esc / E close
                          └──────────────────────────────┘
```

The per-child pill rows ARE the shared renderer. The detail-pane Epic Progress block (currently markdown via Glamour) re-renders through the same function with no cursor.

## Architecture (the key decisions)

1. **One renderer, two call sites.** `buildEpicProgressANSI(epic, allIssues, selectedIdx, width)` returns the progress-summary line + per-child status-pill rows as pre-rendered lipgloss (ANSI). It is the single source of truth for both:
   - the **detail pane** (`model_filter.go`), via `addANSI(...)` on the `renderSection` two-track path (`selectedIdx = -1`, no cursor) - this is the markdown->lipgloss migration that closes **bt-gfxhz.3**;
   - the **focus card** (`selectedIdx = m.epicCardCursor`, cursor highlight on the drillable child).
   This mirrors the established `buildPropertyBlockANSI` / `buildGraphAnalysisANSI` siblings: the `### Epic Progress` H3 stays in the Glamour (md) track so it styles consistently with adjacent headings; only the styled body moves to the ANSI track (the bt-x5xc4 trap class - ANSI cannot pass through Glamour's chroma code-fence path).

2. **Pills reuse existing primitives.** `RenderStatusBadge(string(status))` (styles.go) and `RenderPriorityBadge(priority)` (styles.go) already produce the fg/bg chips. Closed children render `Faint(true)` (recede) instead of strikethrough. No new color tables.

3. **Card is a Model-method modal, not a struct.** The card reads `m.data.issues` to resolve children and reuses `buildEpicProgressANSI`, so a `renderEpicCard()` Model method (mirroring `renderAlertsPanel` / `renderTimeTravelPrompt` / `renderQuitConfirm`) is lower-friction than a separate struct with `SetSize`/data-sync plumbing. State on Model: `epicCardID string`, `epicCardCursor int`. Compositing still follows the modal doc exactly: a fall-through `case ModalEpicCard:` in the `View()` switch + an `OverlayCenterDimBackdrop(body, m.renderEpicCard(), m.width, m.height-1)` block at the bottom.

4. **Drill reuses the jump path.** `enter` on a child calls `selectIssueByID(childID)` + `focusDetailAfterJump()` + `closeModal()` - the exact mechanism the alerts modal uses (`model_filter.go:119`, `:149`).

5. **Entry keys.** `enter` from the overview (replaces the Phase-1 stub in `handleEpicsKeys`). From the list/detail, **lowercase `e`** (new `ListNormalKeys.EpicCard`, gated on cursor-on-an-epic) - free in the list keymap, and a clean mnemonic pair with global `E` (overview) vs `e` (this epic's card). This resolves the spec's "key TBD at implementation" open item.

## Global Constraints

- `go build ./...` and `go vet ./...` pass after every task (AGENTS.md rule 7); `go install ./cmd/bt/` after every successful build.
- No file deletion (none needed this phase).
- No `bd` non-ASCII via inline strings; use `-f`/`--*-file`.
- key.Map convention (bt-ift6): every `key.Binding` has a non-empty `Help.Desc`; cardinality tests catch dropped bindings.
- Commits: `type(scope): description (bt-gfxhz.3)`, scope `tui`.
- TUI render check: `BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump` then read `_tmp/render/*.txt`.

## Roadblocks (read before starting)

1. **ANSI through Glamour is the whole reason the renderSection split exists** (bt-x5xc4). `buildEpicProgressANSI` MUST be routed via `addANSI` (placeholder + `spliceSections`), never `addMD`. If you `addMD` a lipgloss string, chroma strips the ESC bytes and leaves literal `[2m` garbage. The detail block keeps `addMD("### Epic Progress\n")` for the heading and switches only the body to `addANSI`.
2. **`addANSI` rejects empty content.** `buildEpicProgressANSI` returns "" for a childless epic (`total == 0`); guard the detail call so the section (and its H3) is skipped when empty, matching the existing `if total > 0` gate.
3. **Title width / overflow.** The old markdown wrapped long titles via Glamour; spliced ANSI does NOT wrap. Truncate child titles to the available `width` (use `truncateString`, as `renderEpicRow` does). Detail pane passes its content width; the card passes its inner box width.
4. **Cursor clamp.** `epicCardCursor` must clamp to `len(children)-1` on open and on every move; a childless or single-child epic must not panic or scroll off.
5. **Modal key dispatch precedence.** The card's key handler must early-return at the top of the input handler (like the `if m.activeModal == ModalAlerts` block, `model_update_input.go:239`) BEFORE the global view-switch and focus dispatch, or `E`/`e`/`j` leak to the background view.
6. **`e` must not already be live in `handleListKeys`.** ADR-004 makes the keymap the source of truth and `e` is absent from `ListNormalKeys`, but grep `handleListKeys` for a raw `case "e"` before wiring (Task 4).

## File Structure

- `pkg/ui/epic_progress.go` - **new**: `buildEpicProgressANSI(epic model.Issue, allIssues []model.Issue, selectedIdx, width int) string` + `epicProgressSummaryLine` helper. (Keeping it out of `helpers_epics.go` so the overview-projection helpers and the detail/card renderer stay separately testable; `helpers_epics.go` is pure data, this is pure render.)
- `pkg/ui/epic_progress_test.go` - **new**: pill parity + ANSI-present + no-`~~` tests (the bt-gfxhz.3 acceptance test).
- `pkg/ui/model_filter.go` - migrate the Epic Progress block (~1030-1069) from per-status markdown to `addMD(heading)` + `addANSI(buildEpicProgressANSI(item, m.data.issues, -1, width))`.
- `pkg/ui/model.go` - add `ModalEpicCard` to the `ModalType` enum; add `epicCardID string` + `epicCardCursor int` fields; add `openEpicCard(id string)` helper.
- `pkg/ui/epic_card.go` - **new**: `renderEpicCard() string` (panel via `RenderTitledPanel`, body via `buildEpicProgressANSI`, footer hint) + `handleEpicCardKeys(msg) (Model, tea.Cmd)`.
- `pkg/ui/epic_card_test.go` - **new**: open/cursor/drill/close behavior.
- `pkg/ui/keys/epic_card.go` - **new**: `EpicCardKeys` map (Up/Down/Open(drill)/Exit) + ShortHelp/FullHelp.
- `pkg/ui/keys/list.go` - add `EpicCard` binding (`e`, "epic card") to `ListNormalKeys` + ShortHelp/FullHelp.
- `pkg/ui/keys/keys.go` (or wherever the keymap aggregate lives) - add `EpicCard EpicCardKeys` to the keys struct + constructor.
- `pkg/ui/epics_view.go` - replace the `handleEpicsKeys` `Open` stub (lines 230-232) with `m.openEpicCard(...)`.
- `pkg/ui/model_update_input.go` - add the `if m.activeModal == ModalEpicCard { return m.handleEpicCardKeys(msg) }` early-return; wire `EpicCard` (e) in the focusList (and focusDetail) dispatch when the current item is an epic.
- `pkg/ui/model_view.go` - `case ModalEpicCard:` fall-through in the switch + `OverlayCenterDimBackdrop` block.
- `pkg/ui/context.go` / `pkg/ui/model_modes.go` - add `ModalEpicCard` to `modalKeyMap` / focus-name where the other modals are listed (footer help + cardinality).
- `pkg/ui/render_harness_test.go` - add `epic_card_*` scenarios (full + scrunched).

---

### Task 1: Shared pill renderer `buildEpicProgressANSI` (TDD) - closes bt-gfxhz.3

**Files:** Create `pkg/ui/epic_progress.go`, `pkg/ui/epic_progress_test.go`.

**Interface produced:**
```go
// buildEpicProgressANSI renders an epic's progress summary line plus one
// status-pill row per child (natural-numeric order via epicChildrenSorted).
// selectedIdx highlights one child row (the focus card's cursor); pass -1 for
// the static detail-pane embed. width truncates child titles (0 = no truncate).
// Returns "" when the epic has no children, so callers can skip the section.
func buildEpicProgressANSI(epic model.Issue, allIssues []model.Issue, selectedIdx, width int) string
```

Row shape: `<cursor><statusPill> <prioPill> <id> — <title>`. Closed rows use `Faint(true)`. Cursor is `▸ ` on the selected row, `  ` otherwise.

- [ ] **Step 1: Write the failing test** `epic_progress_test.go`:
  - fixture: an epic + 3 children (closed / in_progress / open) via parent_child deps.
  - assert output contains an ESC byte (`\x1b`) - i.e. it is lipgloss, not markdown.
  - assert output contains NO `~~` literal and NO markdown `**` (parity: the old strikethrough/bold are gone).
  - assert child IDs appear in natural-numeric order (`.1` before `.2` before `.10`).
  - assert the summary contains `1 / 3` and `33%`.
  - assert `selectedIdx=1` puts `▸` on the second row only.
  - assert a childless epic returns `""`.
- [ ] **Step 2: Run, verify FAIL** - `go test ./pkg/ui -run TestBuildEpicProgressANSI`.
- [ ] **Step 3: Implement** in `epic_progress.go`. Reuse `epicChildrenSorted`, `RenderStatusBadge(string(child.Status))`, `RenderPriorityBadge(child.Priority)`, `truncateString`, `lipgloss.JoinVertical`. Closed children: wrap the row in `Faint(true)`. Compute `done/total` over the full child set (mirrors `epicProgress`).
- [ ] **Step 4: Run, verify PASS**.
- [ ] **Step 5: Commit** `feat(tui): shared lipgloss epic-progress pill renderer (bt-gfxhz.3)`.

---

### Task 2: Migrate the detail-pane Epic Progress block to the shared renderer

**Files:** Modify `pkg/ui/model_filter.go` (the `item.IssueType == model.TypeEpic` block, ~1030-1069).

- [ ] **Step 1:** Replace the per-status markdown loop with:
  ```go
  if item.IssueType == model.TypeEpic {
      body := buildEpicProgressANSI(item, m.data.issues, -1, detailWidth)
      if body != "" {
          addMD("### Epic Progress\n")
          addANSI(body)
      }
  }
  ```
  `detailWidth`: use the same width the detail render already wraps to (find the Glamour wrap width in `updateViewportContent`; if not readily threaded, use `m.viewport.Width()` minus the panel padding). The summary line moves INTO `buildEpicProgressANSI`, so drop the old `**N / M** children complete` md line.
- [ ] **Step 2:** Confirm `statusGlyph` is still used elsewhere; if the detail block was its only caller, leave it (pre-existing, may be reused by the card) - do not delete (AGENTS.md rule 1; pre-existing-dead-code: mention, don't delete).
- [ ] **Step 3:** `BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump`; read `_tmp/render/detail_epic_90x28.txt` - children render as pills, no literal `~~`/`[2m`, no overflow at width.
- [ ] **Step 4:** `go build ./... && go vet ./... && go install ./cmd/bt/`.
- [ ] **Step 5:** Update/extend the detail render test that asserted the old markdown (search for `~~` or "Epic Progress" assertions) to assert ESC bytes + no `~~`.
- [ ] **Step 6: Commit** `refactor(tui): detail Epic Progress uses shared pill renderer (bt-gfxhz.3)`.

---

### Task 3: `EpicCardKeys` map + Model state + open helper

**Files:** Create `pkg/ui/keys/epic_card.go`; modify `pkg/ui/model.go`, the keys aggregate.

**Interfaces produced:** `keys.EpicCardKeys` (Up/Down/Open/Exit), `m.keys.EpicCard`, `ModalEpicCard`, `m.epicCardID`, `m.epicCardCursor`, `m.openEpicCard(id)`.

- [ ] **Step 1:** `keys/epic_card.go`: `EpicCardKeys{Up, Down, Open, Exit}` mirroring `EpicsKeys` (arrows-primary + vim; `Open` = enter "drill into child"; `Exit` = "esc" / "E" "close card"). ShortHelp + FullHelp with non-empty Help.Desc.
- [ ] **Step 2:** Add `EpicCard EpicCardKeys` to the keys aggregate struct + its constructor (`NewEpicCardKeys()`).
- [ ] **Step 3:** `model.go`: add `ModalEpicCard` to the `ModalType` iota; add `epicCardID string` + `epicCardCursor int` fields; add:
  ```go
  func (m *Model) openEpicCard(id string) {
      m.epicCardID = id
      m.epicCardCursor = 0
      m.openModal(ModalEpicCard)
  }
  ```
- [ ] **Step 4:** `go build ./... && go vet ./...`.
- [ ] **Step 5: Commit** `feat(tui): epic focus-card keymap + modal state (bt-gfxhz.3)`.

---

### Task 4: The focus card - render + key handler + entry points

**Files:** Create `pkg/ui/epic_card.go`; modify `pkg/ui/epics_view.go`, `pkg/ui/model_update_input.go`, `pkg/ui/model_view.go`, `pkg/ui/keys/list.go`, `pkg/ui/context.go`/`model_modes.go`.

- [ ] **Step 1:** `epic_card.go` `renderEpicCard()`:
  - resolve `epic := m.data.issueMap[m.epicCardID]`; guard nil -> empty.
  - children := `epicChildrenSorted(epic.ID, m.data.issues)`.
  - box width ~ `min(70, m.width-6)`; inner width for title truncation.
  - body := `buildEpicProgressANSI(*epic, m.data.issues, m.epicCardCursor, innerWidth)`.
  - footer hint: `"j/k move · ⏎ drill · esc close"`.
  - wrap via `RenderTitledPanel(content, PanelOpts{Title: "Epic " + epic.ID, Width: boxWidth, Height: boxHeight, Focused: true})` (same primitive the recipe picker uses).
- [ ] **Step 2:** `epic_card.go` `handleEpicCardKeys(msg) (Model, tea.Cmd)`:
  - children := `epicChildrenSorted(m.epicCardID, m.data.issues)`.
  - `Up`/`Down`: move + clamp `epicCardCursor` to `[0, len(children)-1]`.
  - `Open`: if `len(children) > 0`, `id := children[m.epicCardCursor].ID`; `m.closeModal()`; `m.selectIssueByID(id)`; `m.focusDetailAfterJump()`.
  - `Exit`: `m.closeModal()`; restore `m.focused`/`m.mode` to whatever opened it (track the opener, or default to list).
- [ ] **Step 3:** `epics_view.go`: replace the `Open` stub (lines 230-232) with `if m.epicsCursor < len(m.epicsRows) { m.openEpicCard(m.epicsRows[m.epicsCursor].Epic.ID) }`.
- [ ] **Step 4:** `keys/list.go`: add `EpicCard` binding (`e`, "epic card") to `ListNormalKeys`, ShortHelp, FullHelp. Grep `handleListKeys` for a raw `case "e"` first.
- [ ] **Step 5:** `model_update_input.go`:
  - early-return block near the other modal blocks: `if m.activeModal == ModalEpicCard { nm, cmd := m.handleEpicCardKeys(msg); return nm, cmd }`.
  - in `handleListKeys` (and the detail handler), on `key.Matches(msg, m.keys.EpicCard)`: resolve the current item; if it's `model.TypeEpic`, `m.openEpicCard(item.ID)`; else `m.setStatus("Not an epic")` (or fall through).
- [ ] **Step 6:** `model_view.go`: add `case ModalEpicCard:` fall-through in the switch (line ~100) + `if m.activeModal == ModalEpicCard { body = OverlayCenterDimBackdrop(body, m.renderEpicCard(), m.width, m.height-1) }` in the overlay block (~line 232).
- [ ] **Step 7:** `context.go` / `model_modes.go` / footer `modalKeyMap`: register `ModalEpicCard` so the footer help + modal cardinality tests see it (mirror `ModalAlerts` entries).
- [ ] **Step 8:** `go build ./... && go vet ./... && go install ./cmd/bt/`.
- [ ] **Step 9: Commit** `feat(tui): epic focus-card modal + drill (bt-gfxhz.3)`.

---

### Task 5: Render harness + key/cardinality tests

**Files:** Modify `pkg/ui/render_harness_test.go`; create `pkg/ui/epic_card_test.go`.

- [ ] **Step 1:** `render_harness_test.go`: add an `epicCard` setup closure (`m.openEpicCard("bt-evuf")` after building the epics fixture with the stale child) and scenarios `{"epic_card_100x32", ...}` + `{"epic_card_70x20", ...}` (scrunched).
- [ ] **Step 2:** `BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump`; read both - card centered, dimmed backdrop visible (judge dim in `.ansi`/PNG), pills render, cursor on row 0, no overflow at 70 wide.
- [ ] **Step 3:** `epic_card_test.go`:
  - `openEpicCard` sets `ModalEpicCard` + cursor 0.
  - j/k move + clamp (no move past ends).
  - enter on a child closes the modal and selects that issue (`selectIssueByID` returns true / cursor lands).
  - esc closes the modal.
  - the `e` key on a list epic opens the card; on a non-epic does not.
- [ ] **Step 4:** Key cardinality: extend the existing `EpicsKeys`/modal cardinality test to include `EpicCardKeys` (every binding has Help.Desc; the modalKeyMap has a `ModalEpicCard` entry).
- [ ] **Step 5:** `go test ./pkg/ui/... ./pkg/ui/keys/...`.
- [ ] **Step 6: Commit** `test(tui): epic focus-card render + key coverage (bt-gfxhz.3)`.

---

### Task 6: Close out

- [ ] **Step 1:** `go build ./... && go vet ./... && go test ./... && go install ./cmd/bt/` - all green.
- [ ] **Step 2:** Close `bt-gfxhz.3` (Summary/Change/Files/Verify/Risk/Notes); note the shared renderer is the single source of truth for both surfaces.
- [ ] **Step 3:** Update parent `bt-3ftfm`: Phase 2 done; Phase 3 (`bt robot epics`) remains. Clear the stale "Push hold" note (Phase 1 is pushed; HEAD == origin/main).
- [ ] **Step 4:** CHANGELOG.md entry (Phase 2) + ADR-002 stream status if applicable.
- [ ] **Step 5:** Integrate the worktree branch to main; push.

---

## Self-review notes

- Spec coverage: tier-2 card (Tasks 3-5), shared pill renderer reused by both card and detail (Tasks 1-2 = bt-gfxhz.3), enter-from-overview + `e`-from-list/detail entry keys (Task 4), modal compositing via OverlayCenterDimBackdrop (Task 4 Step 6), drill-into-child (Task 4 Step 2). Tier 3 (`bt-h97e`) untouched.
- bt-gfxhz.3 acceptance mapped: visual parity for status mapping (Task 1/2), natural-sort preserved (`epicChildrenSorted`, Task 1 test), ANSI-present + no-`~~` test (Task 1 Step 1, Task 2 Step 5).
- Open decision resolved in-plan: list/detail open key = `e` (free, mnemonic pair with `E`). If the e/E pairing reads as too subtle in dogfood, the fallback is a capital like `F` - one-line change, noted not gold-plated.
- Carried risk: detail-pane wrap width threading (Task 2 Step 1) - read `updateViewportContent` to source the exact width rather than guessing.
