# Help-surface split (`?` global / `;` view) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split bt's two keybind help surfaces by content — `?` overlay shows the GLOBAL map only (responsive, scrolling, task-headed); `;` sidebar shows the ACTIVE-VIEW map only (with an empty-view fallback) — fixing the "everything at once" wall and the dead `helpScroll` so `?` is usable at 14-30 rows.

**Architecture:** Both surfaces already consume the same per-view `key.Map` `FullHelp()` source (bt-ift6). This plan re-scopes WHICH maps populate each surface and wires the unread scroll. `?` (`renderHelpOverlay`, `pkg/ui/model_view.go`) drops its 10-map `panels[]` and renders `m.keys.Global.FullHelp()` under task headers, in responsive columns, windowed by `m.helpScroll`. `;` (`sidebarHelpGroups`, `pkg/ui/model_footer.go` + `ShortcutsSidebar.View`, `pkg/ui/shortcuts_sidebar.go`) drops the Global prefix it composes in, gaining an empty-view fallback. Symmetric one-line cross-reference footers tie them together. No `??` layer.

**Tech Stack:** Go 1.25+, Charm Bracelet v2 (Bubble Tea, Lipgloss, Bubbles `key`/`help`). Tier-1 render harness (`render_harness_test.go`, `BT_RENDER_DUMP=1`).

## Global Constraints

- **Single-source-of-truth invariant (preserve):** `?` consumes `m.keys.Global.FullHelp()`; `;` consumes `viewSpecificKeyMap().FullHelp()`. No literal key/desc string tables may be reintroduced. (spec: "Single-source-of-truth invariant")
- **No `??` layer.** Explicitly dropped. (spec: "What this supersedes")
- **Verify after every code change:** `go build ./...` and `go vet ./...` must pass. (AGENTS.md Core Rule 7)
- **Install after build:** `go install ./cmd/bt/` after a green build (user runs `bt` from PATH).
- **Edits manual or via subagents only** — no script-based code changes. (AGENTS.md Core Rule 3)
- **ASCII in inline strings;** any non-ASCII (em-dash, arrows) only via `--body-file` / file-based bd writes. Use `-` and straight quotes in code/UI copy by default. (AGENTS.md Core Rule 8)
- **Scrunched-first testing:** the render harness `modal_help_70x20` case is the bt-dx7k repro. `?` must fit and be non-clipping across 70x20, 50x14, 30x20, 120x40. (spec: "Testing")
- **Hold the push.** Maintainer dogfoods before anything hits public main. Do NOT `git push` or `bd dolt push`. Commit locally on branch `worktree-bt-dx7k-help-surface` only.
- **Worktree:** all work happens in `.claude/worktrees/bt-dx7k-help-surface` on branch `worktree-bt-dx7k-help-surface` (already set up; the spec is committed there at 0d590dc9).
- **Charm TUI:** invoke the `charm-tui-design` skill before writing any TUI rendering code (it informs the responsive-layout / lipgloss work in Tasks 2-3, 6-8).

---

## Orientation (read these before starting)

- `pkg/ui/model_view.go:653` — `renderHelpOverlay()` (pointer receiver). Builds 10 per-view panels + status legend, river-packs them, ends with `lipgloss.Place(..., Center, Center, ...)` which **centers and clips** oversized content. This is the function rewritten in Tasks 1-4.
- `pkg/ui/model_view.go:93` — `View()` is a **value** receiver (`func (m Model) View() tea.View`). Consequence: scroll clamping inside the render is **display-only** (the copy's mutation does not persist). The handler must clamp the persisted value (Task 3).
- `pkg/ui/model_view.go:140` — `case ModalHelp: body = m.renderHelpOverlay()`. The overlay is the full-screen `body`; it is NOT composited via `OverlayCenterDimBackdrop` (it is a full-screen takeover). Keep it that way.
- `pkg/ui/keys/global.go:206` — `GlobalKeys.FullHelp()` returns 4 groups in fixed order: `[0]`=Help&Chrome, `[1]`=Views, `[2]`=Workspace, `[3]`=Actions. These are the groups Task 1 labels and reorders (essentials-first).
- `pkg/ui/model_footer.go:775` — `sidebarHelpGroups()` returns `Global.FullHelp() ++ viewSpecificKeyMap().FullHelp()` (or modal-only when a modal owns the sidebar). Task 6 drops the Global prefix.
- `pkg/ui/model_footer.go:704` — `viewSpecificKeyMap()` returns the per-view map, or `nil` for Attention/LabelDashboard (the empty-view case Task 7 handles).
- `pkg/ui/shortcuts_sidebar.go:85` — `ShortcutsSidebar.View()` renders `s.groups` as a vertical key+desc list with a `; hide` / `ctrl+j/k scroll %` footer. Tasks 7-8 add the empty fallback and the `? for global` cross-ref.
- `pkg/ui/render_harness_test.go:302` — `modal_help_70x20` / `modal_help_120x40` scenarios. Task 9 extends the scrunched matrix.

### Tests that encode OLD behavior and MUST be rewritten

These currently pass against the 10-map `?` and Global++view `;`. They will fail after re-scoping and are rewritten in the tasks below (TDD red → green):

- `pkg/ui/coverage_extra_test.go:1434` `TestHelpOverlay_ConsumesKeyMapBindings` — asserts `"epic card"`/`"cass session"` (ListNormal descs) appear in `?`. → rewrite to global-only scoping (Task 1). NOTE: do NOT reuse `"cass session"` as a marker — that binding is stale (the `cass` tool was renamed to `sym`; the feature silently no-ops and the modal suggests a dead `cass search` command; flagged separately as a tracking bead). Use `"cycle sort"` / `"epic card"` as the view-specific markers instead.
- `pkg/ui/coverage_extra_test.go:1402` `TestRenderHelpOverlay_ResponsiveLayout` — asserts the title `"Keyboard Shortcuts"`. → update to the new title (Task 1).
- `pkg/ui/coverage_extra_test.go:1303` `TestHelpOverlayScroll` (render section, lines 1372-1387) — asserts title `"Keyboard Shortcuts"` + Tutorial hint. → update strings + extend with a real scroll-window assertion (Task 3). The handler-mutation assertions (j→1, k→0, etc.) stay valid as-is.
- `pkg/ui/shortcuts_sidebar_test.go:68` `TestSidebarHelpGroups_NonModalComposesGlobalAndView` — asserts `board` (global) AND `cycle sort` (list) appear. → rewrite to view-only: `board` absent, `cycle sort` present (Task 6). The modal-only test at `:90` stays valid.

---

## File Structure

No new production files. Changes are localized to:

- `pkg/ui/model_view.go` — rewrite `renderHelpOverlay()`; add small helpers (`helpOverlayPanels`, `helpOverlayBodyLines`, `helpOverlayColumns`, `helpOverlayAvailBody`, `helpScrollMax`). All help-overlay logic stays in this file (it already owns the function).
- `pkg/ui/help_keys.go` — `handleHelpKeys()` gains handler-side scroll clamping.
- `pkg/ui/model_footer.go` — `sidebarHelpGroups()` drops the Global prefix.
- `pkg/ui/shortcuts_sidebar.go` — `View()` gains the empty-view fallback + `? for global` cross-ref footer.
- `pkg/ui/render_harness_test.go` — extend the scrunched `modal_help_*` matrix (test fixture, no assertions).
- Test files: `pkg/ui/coverage_extra_test.go`, `pkg/ui/shortcuts_sidebar_test.go` — rewrite the four old-behavior tests; add new scoping/scroll/fallback tests.

---

## Task 1: `?` overlay → Global-only, task-headed, essentials-first

**Goal:** `renderHelpOverlay()` renders `m.keys.Global.FullHelp()` only — four task-headed panels in essentials-first order (Views, Actions, Workspace, Chrome) plus the status-glyph legend. Drop the 10-map `panels[]`. Layout/scroll come in Tasks 2-3; this task establishes the content and the panel-builder helpers, keeping the existing river-pack + `lipgloss.Place` tail so the function still compiles and renders.

**Files:**
- Modify: `pkg/ui/model_view.go:653-874` (`renderHelpOverlay`)
- Modify (test): `pkg/ui/coverage_extra_test.go:1434` (`TestHelpOverlay_ConsumesKeyMapBindings`), `:1402` (`TestRenderHelpOverlay_ResponsiveLayout`)

**Interfaces:**
- Produces: `func (m Model) helpOverlayPanels() []string` — the rendered task-group panels (essentials-first) + status legend, in display order. Consumed by Task 2's `helpOverlayBodyLines`.
- Consumes: `m.keys.Global.FullHelp() [][]key.Binding` (order: Help&Chrome, Views, Workspace, Actions).

- [ ] **Step 1: Write the failing scoping test** (rewrite `TestHelpOverlay_ConsumesKeyMapBindings`)

Replace the body of `TestHelpOverlay_ConsumesKeyMapBindings` in `pkg/ui/coverage_extra_test.go` with the new global-only contract:

```go
// TestHelpOverlay_ConsumesKeyMapBindings verifies the ? overlay is scoped to the
// Global map only (bt-dx7k): global binding descs appear; view-specific descs
// (ListNormal's "epic card" / "cycle sort") do not. Source is still the key.Map
// FullHelp(), never literal tables.
func TestHelpOverlay_ConsumesKeyMapBindings(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.mode = ViewList // a populated view, to prove its keys are still excluded
	m.width, m.height = 180, 60
	m.openModal(ModalHelp)
	m.focused = focusHelp

	out := m.renderHelpOverlay()

	// Global descs must be present.
	for _, want := range []string{"board", "graph", "refresh", "label picker"} {
		if !strings.Contains(out, want) {
			t.Errorf("? overlay missing global binding desc %q", want)
		}
	}
	// View-specific descs (ListNormal) must NOT be present — ? is global-only.
	for _, absent := range []string{"epic card", "cycle sort"} {
		if strings.Contains(out, absent) {
			t.Errorf("? overlay leaked view-specific desc %q (should be global-only)", absent)
		}
	}
}
```

- [ ] **Step 2: Update the title assertion** in `TestRenderHelpOverlay_ResponsiveLayout` (`pkg/ui/coverage_extra_test.go:1422`)

Change the assertion string from `"Keyboard Shortcuts"` to the new title:

```go
				if !strings.Contains(out, "Global Shortcuts") {
					t.Fatalf("help overlay should contain title at width=%d", tc.width)
				}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./pkg/ui -run 'TestHelpOverlay_ConsumesKeyMapBindings|TestRenderHelpOverlay_ResponsiveLayout' -v`
Expected: FAIL — `"epic card"`/`"cycle sort"` still present (10-map overlay); title still `"Keyboard Shortcuts"`.

- [ ] **Step 4: Rewrite `renderHelpOverlay` to global-only with task headers**

In `pkg/ui/model_view.go`, replace the `panels := []string{ ... 10 entries ... }` block (lines ~779-790) and the title block (lines ~852-863) so the overlay sources only the Global map under essentials-first task headers. Keep `renderRowsPanel`, `bindingGroups`, `statusLegend`, the river-pack loop, and the `lipgloss.Place` tail for now (Tasks 2-3 replace the layout/place).

Replace the 10-element `panels` literal with:

```go
	// Global map → essentials-first, task-headed panels (bt-dx7k). FullHelp()
	// returns 4 fixed groups: [0]Help&Chrome [1]Views [2]Workspace [3]Actions.
	// Display order puts the most-used (view switching) first so the top of the
	// overlay is useful before any scroll. Headers label the existing groups;
	// the grouping is not restructured. Header strings are a dogfood tuning point.
	g := m.keys.Global.FullHelp()
	taskOrder := []struct {
		title    string
		colorIdx int
		bindings [][]key.Binding
	}{
		{"SWITCH VIEWS", 0, [][]key.Binding{g[1]}},
		{"DO THINGS", 1, [][]key.Binding{g[3]}},
		{"WORKSPACE", 2, [][]key.Binding{g[2]}},
		{"CHROME", 3, [][]key.Binding{g[0]}},
	}
	var panels []string
	for _, tg := range taskOrder {
		if p := renderRowsPanel(tg.title, "", tg.colorIdx, bindingGroups(tg.bindings)); p != "" {
			panels = append(panels, p)
		}
	}
```

(Leave the `statusLegend` definition and its `renderRowsPanel("Status Indicators", ...)` append in the river-pack tail unchanged.)

Update the title block (lines ~852-863) to name the global scope:

```go
	header := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("Global Shortcuts"),
		subtitleStyle.Render("; for this screen  -  ? or Esc to close"),
	)
```

Note: `renderRowsPanel`'s first call arg is the title and its second is an `icon` string — pass `""` for the icon (task headers carry no emoji; keeps them legible at narrow width). If `renderRowsPanel` requires a non-empty icon for spacing, leave the existing signature and pass a single space.

- [ ] **Step 5: Build + vet, then run the tests to verify they pass**

Run: `go build ./... && go vet ./... && go test ./pkg/ui -run 'TestHelpOverlay_ConsumesKeyMapBindings|TestRenderHelpOverlay_ResponsiveLayout' -v`
Expected: PASS.

- [ ] **Step 6: Run the help tests to scope collateral breakage**

Run: `go test ./pkg/ui -run 'Help' -v`
Expected: `TestHelpOverlayScroll` may FAIL on its render-section title/Tutorial assertions (lines 1377/1385) — that is expected and fixed in Task 3. All non-`TestHelpOverlayScroll` help tests PASS. If any OTHER test fails, stop and investigate.

- [ ] **Step 7: Commit**

```bash
git add pkg/ui/model_view.go pkg/ui/coverage_extra_test.go
git commit -m "fix(tui): scope ? overlay to Global map only, task-headed (bt-dx7k)"
```

---

## Task 2: `?` overlay → responsive columns (4 / 2 / 1 by width)

**Goal:** Replace the greedy river-pack with explicit column count from width: 4 columns at width ≥120, 2 at 80-120, 1 below. Essentials-first order means the 1-column case shows SWITCH VIEWS first. Factor the panel grid into `helpOverlayBodyLines()` returning flat terminal lines (consumed by Task 3's scroll window).

**Files:**
- Modify: `pkg/ui/model_view.go` (`renderHelpOverlay` body; add `helpOverlayColumns`, `helpOverlayPanels`, `helpOverlayBodyLines`)
- Modify (test): `pkg/ui/coverage_extra_test.go` (add `TestHelpOverlayColumns`)

**Interfaces:**
- Produces: `func helpOverlayColumns(width int) int` — returns 4/2/1.
- Produces: `func (m Model) helpOverlayBodyLines() []string` — panels laid into the responsive grid, flattened to lines (pre-scroll). Consumed by Task 3 (`renderHelpOverlay` window + `helpScrollMax`).
- Consumes: `helpOverlayPanels() []string` from Task 1 (extract the Task-1 panel-build loop + status legend into this method).

- [ ] **Step 1: Write the failing column-count test**

Add to `pkg/ui/coverage_extra_test.go`:

```go
func TestHelpOverlayColumns(t *testing.T) {
	cases := []struct {
		width, want int
	}{
		{130, 4}, {120, 4},
		{119, 2}, {100, 2}, {80, 2},
		{79, 1}, {50, 1}, {30, 1},
	}
	for _, c := range cases {
		if got := helpOverlayColumns(c.width); got != c.want {
			t.Errorf("helpOverlayColumns(%d) = %d, want %d", c.width, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./pkg/ui -run TestHelpOverlayColumns -v`
Expected: FAIL with "undefined: helpOverlayColumns".

- [ ] **Step 3: Add `helpOverlayColumns` and extract the panel/grid helpers**

In `pkg/ui/model_view.go`, add the column helper near `renderHelpOverlay`:

```go
// helpOverlayColumns returns the task-panel column count for the ? overlay at the
// given width (bt-dx7k responsive levers): 4 wide, 2 medium, 1 narrow.
func helpOverlayColumns(width int) int {
	switch {
	case width >= 120:
		return 4
	case width >= 80:
		return 2
	default:
		return 1
	}
}
```

Extract the Task-1 panel-build loop and the status legend into `helpOverlayPanels`, and add `helpOverlayBodyLines` that lays them into the grid. Move the `renderRowsPanel`/`bindingGroups` closures and `colors`/`statusLegend` data out of `renderHelpOverlay` into `helpOverlayPanels` (they are no longer needed by the outer function). Note both methods need value receivers so the handler (Task 3) can call them:

```go
// helpOverlayPanels builds the global task-group panels (essentials-first) plus
// the status-glyph legend, in display order, for the ? overlay (bt-dx7k).
func (m Model) helpOverlayPanels() []string {
	t := m.theme
	colors := []color.Color{ColorInfo, ColorSuccess, ColorWarning, ColorPrimary}
	type helpRow struct{ left, right string }

	renderRowsPanel := func(title, icon string, colorIdx int, groups [][]helpRow) string {
		// ... unchanged body moved verbatim from renderHelpOverlay (panel sizing,
		// left-text/right-token alignment, RenderTitledPanel) ...
	}
	bindingGroups := func(groups [][]key.Binding) [][]helpRow {
		// ... unchanged body moved verbatim from renderHelpOverlay ...
	}

	g := m.keys.Global.FullHelp()
	taskOrder := []struct {
		title    string
		colorIdx int
		bindings [][]key.Binding
	}{
		{"SWITCH VIEWS", 0, [][]key.Binding{g[1]}},
		{"DO THINGS", 1, [][]key.Binding{g[3]}},
		{"WORKSPACE", 2, [][]key.Binding{g[2]}},
		{"CHROME", 3, [][]key.Binding{g[0]}},
	}
	var panels []string
	for _, tg := range taskOrder {
		if p := renderRowsPanel(tg.title, "", tg.colorIdx, bindingGroups(tg.bindings)); p != "" {
			panels = append(panels, p)
		}
	}

	// Status-glyph legend: the one panel with no key.Map source. Preserve the
	// original glyph right-tokens from model_view.go:794-802 verbatim (they are
	// existing UI Unicode written via the file editor, safe to keep).
	statusLegend := [][]helpRow{{
		{left: "Phase 2 metrics computing", right: "<metrics glyph>"},
		// ... remaining 6 rows verbatim from the original statusLegend ...
	}}
	if p := renderRowsPanel("Status Indicators", "", 0, statusLegend); p != "" {
		panels = append(panels, p)
	}
	return panels
}

// helpOverlayBodyLines lays the task panels into the responsive grid and returns
// the flat terminal lines (pre-scroll), so the caller can window them (bt-dx7k).
func (m Model) helpOverlayBodyLines() []string {
	panels := m.helpOverlayPanels()
	if len(panels) == 0 {
		return nil
	}
	cols := helpOverlayColumns(m.width)
	gap := strings.Repeat(" ", 3)
	var rows []string
	for i := 0; i < len(panels); i += cols {
		end := i + cols
		if end > len(panels) {
			end = len(panels)
		}
		row := panels[i]
		for _, p := range panels[i+1 : end] {
			row = lipgloss.JoinHorizontal(lipgloss.Top, row, gap, p)
		}
		rows = append(rows, row)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return strings.Split(body, "\n")
}
```

Then slim `renderHelpOverlay` to consume `helpOverlayBodyLines()` for the body (keep the `lipgloss.Place(..., Center, Center, ...)` tail until Task 3):

```go
func (m *Model) renderHelpOverlay() string {
	t := m.theme
	bodyLines := m.helpOverlayBodyLines()
	body := strings.Join(bodyLines, "\n")

	titleStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	subtitleStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)
	header := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("Global Shortcuts"),
		subtitleStyle.Render("; for this screen  -  ? or Esc to close"),
	)
	fullContent := lipgloss.JoinVertical(lipgloss.Center, header, "", body)
	return lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, fullContent)
}
```

- [ ] **Step 4: Run column test + build/vet to verify pass**

Run: `go build ./... && go vet ./... && go test ./pkg/ui -run TestHelpOverlayColumns -v`
Expected: PASS.

- [ ] **Step 5: Verify non-clipping at scrunched widths via the harness**

Run: `BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump`
Then Read `_tmp/render/modal_help_70x20.txt` and `_tmp/render/modal_help_120x40.txt`. Confirm: 70x20 shows 1 column (panels stacked), no line exceeds 70 cols, content is not horizontally truncated. 120x40 shows 4 columns side by side.
Expected: at 70-wide, single-column stack (will overflow vertically — that is the scroll case fixed in Task 3, not a failure here).

- [ ] **Step 6: Commit**

```bash
git add pkg/ui/model_view.go pkg/ui/coverage_extra_test.go
git commit -m "fix(tui): responsive 4/2/1 columns for ? overlay (bt-dx7k)"
```

---

## Task 3: `?` overlay → live scroll (the dead-`helpScroll` fix)

**Goal:** Window `helpOverlayBodyLines()` by `m.helpScroll`, clamped to content height; top-align the content (centering caused the clip). Clamp the persisted `helpScroll` in the handler (because `View()` is a value receiver, render-side clamp is display-only) so `G` lands exactly at the bottom and `j`/`ctrl+d` stop at the end.

**Files:**
- Modify: `pkg/ui/model_view.go` (`renderHelpOverlay` tail; add `helpOverlayAvailBody`, `helpScrollMax`)
- Modify: `pkg/ui/help_keys.go:53` (`handleHelpKeys` — clamp after mutation)
- Modify (test): `pkg/ui/coverage_extra_test.go:1303` (`TestHelpOverlayScroll` render section); add `TestHelpOverlayScroll_WindowsContent`

**Interfaces:**
- Produces: `func (m Model) helpOverlayAvailBody() int` — scrollable body height (screen minus header/footer/spacer chrome).
- Produces: `func (m Model) helpScrollMax() int` — `max(0, len(helpOverlayBodyLines()) - helpOverlayAvailBody())`.
- Consumes: `helpOverlayBodyLines()` (Task 2), `m.helpScroll int` (`model.go:618`).

- [ ] **Step 1: Write the failing scroll-window test**

Add to `pkg/ui/coverage_extra_test.go`:

```go
// TestHelpOverlayScroll_WindowsContent verifies helpScroll actually pans the ?
// overlay body (the dead-scroll fix, bt-dx7k): at a height that forces overflow,
// scrolling changes which lines render, and helpScrollMax clamps the bottom.
func TestHelpOverlayScroll_WindowsContent(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.width, m.height = 60, 16 // narrow + short -> 1 column, overflows
	m.openModal(ModalHelp)
	m.focused = focusHelp

	max := m.helpScrollMax()
	if max <= 0 {
		t.Fatalf("expected content to overflow at 60x16 (helpScrollMax>0), got %d", max)
	}

	m.helpScroll = 0
	top := m.renderHelpOverlay()
	m.helpScroll = max
	bottom := m.renderHelpOverlay()
	if top == bottom {
		t.Fatalf("scrolling to bottom did not change the rendered window")
	}

	// Over-scroll is clamped (display): scroll past max renders same as max.
	m.helpScroll = max + 50
	over := m.renderHelpOverlay()
	if over != bottom {
		t.Fatalf("over-scroll should clamp to the bottom window")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./pkg/ui -run TestHelpOverlayScroll_WindowsContent -v`
Expected: FAIL with "undefined: helpScrollMax".

- [ ] **Step 3: Add the scroll helpers and window the render**

In `pkg/ui/model_view.go` add:

```go
// helpOverlayChrome is the fixed rows the ? overlay reserves outside the
// scrollable body: title + subtitle (2), a blank spacer (1), the footer (1).
const helpOverlayChrome = 4

// helpOverlayAvailBody returns the scrollable body height for the ? overlay.
func (m Model) helpOverlayAvailBody() int {
	avail := m.height - 1 - helpOverlayChrome
	if avail < 1 {
		avail = 1
	}
	return avail
}

// helpScrollMax is the maximum helpScroll offset for the current dimensions.
func (m Model) helpScrollMax() int {
	max := len(m.helpOverlayBodyLines()) - m.helpOverlayAvailBody()
	if max < 0 {
		max = 0
	}
	return max
}
```

Replace `renderHelpOverlay`'s body/place tail (from Task 2) with windowing + top-align:

```go
func (m *Model) renderHelpOverlay() string {
	t := m.theme
	bodyLines := m.helpOverlayBodyLines()

	avail := m.helpOverlayAvailBody()
	maxScroll := len(bodyLines) - avail
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.helpScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	end := scroll + avail
	if end > len(bodyLines) {
		end = len(bodyLines)
	}
	window := bodyLines
	if len(bodyLines) > 0 {
		window = bodyLines[scroll:end]
	}
	body := strings.Join(window, "\n")

	titleStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	subtitleStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)
	header := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("Global Shortcuts"),
		subtitleStyle.Render("; for this screen  -  ? or Esc to close"),
	)

	// Footer scroll indicator when content overflows (Task 4 enriches the footer).
	footer := ""
	if maxScroll > 0 {
		pct := scroll * 100 / maxScroll
		footer = subtitleStyle.Render(fmt.Sprintf("j/k scroll  %d%%", pct))
	}

	fullContent := lipgloss.JoinVertical(lipgloss.Center, header, "", body, footer)
	// Top-align vertically: centering clipped oversized content (the bt-dx7k bug).
	return lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Top, fullContent)
}
```

- [ ] **Step 4: Clamp `helpScroll` in the handler**

In `pkg/ui/help_keys.go`, replace the scroll cases in `handleHelpKeys` so each clamps against `helpScrollMax()` (and `G` lands exactly at the bottom instead of the `999` sentinel):

```go
	switch msg.String() {
	case "j", "down":
		m.helpScroll++
		if max := m.helpScrollMax(); m.helpScroll > max {
			m.helpScroll = max
		}
	case "k", "up":
		if m.helpScroll > 0 {
			m.helpScroll--
		}
	case "ctrl+d":
		m.helpScroll += 10
		if max := m.helpScrollMax(); m.helpScroll > max {
			m.helpScroll = max
		}
	case "ctrl+u":
		m.helpScroll -= 10
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
	case "home", "g":
		m.helpScroll = 0
	case "G", "end":
		m.helpScroll = m.helpScrollMax()
```

(Leave the `q`/`esc`/`?`/`f1`, `space`, and `default` cases unchanged.)

- [ ] **Step 5: Update the `TestHelpOverlayScroll` render-section assertions**

In `pkg/ui/coverage_extra_test.go`, the render section (lines ~1376-1387) asserts the old title/hints. Update to the new contract. Replace:

```go
	out := m.renderHelpOverlay()
	if !strings.Contains(out, "Keyboard Shortcuts") {
		t.Fatalf("help overlay should render shortcuts")
	}
	// Should show close hint
	if !strings.Contains(out, "close") && !strings.Contains(out, "Esc") {
		t.Fatalf("help overlay should show close hint")
	}
	// Should show tutorial hint (bv-0trk)
	if !strings.Contains(out, "Tutorial") {
		t.Fatalf("help overlay should show Tutorial hint")
	}
```

with:

```go
	out := m.renderHelpOverlay()
	if !strings.Contains(out, "Global Shortcuts") {
		t.Fatalf("help overlay should render the global title")
	}
	// Should show close + cross-reference hints.
	if !strings.Contains(out, "Esc") || !strings.Contains(out, ";") {
		t.Fatalf("help overlay should show close hint and ; cross-reference")
	}
```

Also: the `G` handler assertion at line ~1349-1352 expects `m.helpScroll < 10` to fail (i.e. `>10`). With the new clamp, after `G` the value is `helpScrollMax()` which may be small. Replace it with a clamp-aware check:

```go
	// Test end -> bottom (clamped to content, not a 999 sentinel)
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if m.helpScroll != m.helpScrollMax() {
		t.Fatalf("expected helpScroll=helpScrollMax after G, got %d (max %d)", m.helpScroll, m.helpScrollMax())
	}
```

And the `ctrl+d -> 10` assertion (line ~1330-1333) assumes no clamp; at 80x20 `helpScrollMax()` may be `< 10`. Make it clamp-aware:

```go
	// Test page down (clamped to content)
	m.helpScroll = 0
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	wantDown := 10
	if max := m.helpScrollMax(); wantDown > max {
		wantDown = max
	}
	if m.helpScroll != wantDown {
		t.Fatalf("expected helpScroll=%d after ctrl+d, got %d", wantDown, m.helpScroll)
	}
```

- [ ] **Step 6: Run the scroll tests + build/vet**

Run: `go build ./... && go vet ./... && go test ./pkg/ui -run 'TestHelpOverlayScroll' -v`
Expected: PASS (`TestHelpOverlayScroll` and `TestHelpOverlayScroll_WindowsContent`).

- [ ] **Step 7: Verify scroll visually at the repro size**

Run: `BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump`
Read `_tmp/render/modal_help_70x20.txt`: confirm the overlay top-aligns, the header + first panel(s) are visible and non-clipped, and a `j/k scroll 0%` footer shows.
Expected: no top/bottom clipping; footer scroll indicator present.

- [ ] **Step 8: Commit**

```bash
git add pkg/ui/model_view.go pkg/ui/help_keys.go pkg/ui/coverage_extra_test.go
git commit -m "fix(tui): wire helpScroll into ? overlay render + handler clamp (bt-dx7k)"
```

---

## Task 4: `?` overlay → footer cross-reference + hints

**Goal:** Make the `?` footer a single line carrying the cross-reference to `;`, the close hint, and the scroll indicator (when scrollable). Symmetric counterpart to `;`'s footer (Task 8).

**Files:**
- Modify: `pkg/ui/model_view.go` (`renderHelpOverlay` footer)
- Modify (test): `pkg/ui/coverage_extra_test.go` (add `TestHelpOverlay_CrossRefFooter`)

**Interfaces:** none new (footer string only).

- [ ] **Step 1: Write the failing footer test**

Add to `pkg/ui/coverage_extra_test.go`:

```go
// TestHelpOverlay_CrossRefFooter verifies the ? overlay footer carries the
// one-line cross-reference to the ; sidebar plus a close hint (bt-dx7k).
func TestHelpOverlay_CrossRefFooter(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.width, m.height = 120, 40
	m.openModal(ModalHelp)
	m.focused = focusHelp

	out := m.renderHelpOverlay()
	if !strings.Contains(out, ";") {
		t.Errorf("? footer should cross-reference the ; sidebar")
	}
	if !strings.Contains(out, "Esc") {
		t.Errorf("? footer should show a close hint")
	}
}
```

- [ ] **Step 2: Run to verify it passes or fails**

Run: `go test ./pkg/ui -run TestHelpOverlay_CrossRefFooter -v`
Expected: likely PASS already (the subtitle from Task 3 contains `;` and `Esc`). If PASS, proceed to Step 3 to consolidate cross-ref + scroll into ONE footer line so the subtitle stays a pure title-clarifier. If FAIL, Step 3 fixes it.

- [ ] **Step 3: Consolidate the footer line**

In `renderHelpOverlay`, fold the cross-ref + close + scroll into the single footer line, and drop the subtitle:

```go
	header := titleStyle.Render("Global Shortcuts")

	hint := "; for this screen  -  ? or Esc to close"
	if maxScroll > 0 {
		pct := scroll * 100 / maxScroll
		hint = fmt.Sprintf("%s  -  j/k scroll %d%%", hint, pct)
	}
	footer := subtitleStyle.Render(hint)
```

Then `helpOverlayChrome` becomes 3 (title 1 + spacer 1 + footer 1). Update the constant:

```go
const helpOverlayChrome = 3
```

- [ ] **Step 4: Run footer + scroll tests + build/vet**

Run: `go build ./... && go vet ./... && go test ./pkg/ui -run 'TestHelpOverlay_CrossRefFooter|TestHelpOverlayScroll' -v`
Expected: PASS. (The scroll test recomputes `helpScrollMax()` from the new chrome, so it stays consistent.)

- [ ] **Step 5: Commit**

```bash
git add pkg/ui/model_view.go pkg/ui/coverage_extra_test.go
git commit -m "fix(tui): single-line cross-ref footer for ? overlay (bt-dx7k)"
```

---

## Task 5: Full `?` regression sweep + scrunched harness gate

**Goal:** Confirm the `?` rewrite (Tasks 1-4) is green across the whole `pkg/ui` package and visually correct at the maintainer's sizes before touching `;`.

**Files:**
- Modify: `pkg/ui/render_harness_test.go:302-303` (extend the `modal_help_*` matrix)

- [ ] **Step 1: Add scrunched `?` harness scenarios**

In `pkg/ui/render_harness_test.go`, extend the modal scenarios (after line 303):

```go
		{"modal_help_50x14", 50, 14, func(m *Model) { m.openModal(ModalHelp) }}, // bt-dx7k hard gate
		{"modal_help_30x20", 30, 20, func(m *Model) { m.openModal(ModalHelp) }}, // bt-dx7k 1-col scroll
```

- [ ] **Step 2: Run the full ui package**

Run: `go test ./pkg/ui`
Expected: PASS (no remaining references to the 10-map overlay or old titles).

- [ ] **Step 3: Dump + read the scrunched matrix**

Run: `BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump`
Read each of: `_tmp/render/modal_help_50x14.txt`, `modal_help_30x20.txt`, `modal_help_70x20.txt`, `modal_help_120x40.txt`.
Confirm for each: no line wider than the terminal width; header visible; no vertical clipping of the visible window; scroll footer present when overflowing; 4-col at 120, 1-col at ≤79.

- [ ] **Step 4: Live dogfood the `?` overlay**

Suggest the user run `! bt` at a scrunched window (~70x20 and ~30x20). Press `?`, scroll with `j`/`k`/`G`/`g`, dismiss with `Esc`. Confirm usability and non-clipping.
Note: do not block on this if running headless — the harness dumps are the gate; the live run is the maintainer's dogfood (held pre-push).

- [ ] **Step 5: Commit**

```bash
git add pkg/ui/render_harness_test.go
git commit -m "test(tui): scrunched ? overlay harness scenarios (bt-dx7k)"
```

---

## Task 6: `;` sidebar → view-only scoping

**Goal:** `sidebarHelpGroups()` returns the active view's `FullHelp()` only (drop the Global prefix). Modal behavior (modal-only) is unchanged.

**Files:**
- Modify: `pkg/ui/model_footer.go:775` (`sidebarHelpGroups`)
- Modify (test): `pkg/ui/shortcuts_sidebar_test.go:68` (`TestSidebarHelpGroups_NonModalComposesGlobalAndView`)

**Interfaces:**
- Modifies: `func (m Model) sidebarHelpGroups() [][]key.Binding` — non-modal path now returns `viewSpecificKeyMap().FullHelp()` only (or empty when nil).

- [ ] **Step 1: Rewrite the scoping test to view-only**

Replace `TestSidebarHelpGroups_NonModalComposesGlobalAndView` in `pkg/ui/shortcuts_sidebar_test.go`:

```go
// TestSidebarHelpGroups_NonModalShowsViewOnly verifies the ; sidebar shows the
// active view's bindings ONLY (bt-dx7k) - the Global prefix is dropped (it now
// lives on the ? overlay). In List view, "cycle sort" (ListNormal) is present
// and "board" (Global) is absent.
func TestSidebarHelpGroups_NonModalShowsViewOnly(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.mode = ViewList
	m.activeModal = ModalNone

	descs := map[string]bool{}
	for _, g := range m.sidebarHelpGroups() {
		for _, b := range g {
			descs[b.Help().Desc] = true
		}
	}
	if descs["board"] {
		t.Errorf("global binding (board) must NOT appear in the view-only ; sidebar")
	}
	if !descs["cycle sort"] {
		t.Errorf("expected a list binding (cycle sort) in the ; sidebar")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./pkg/ui -run TestSidebarHelpGroups -v`
Expected: FAIL — `board` still present (Global prefix still composed in).

- [ ] **Step 3: Drop the Global prefix in `sidebarHelpGroups`**

In `pkg/ui/model_footer.go`, change the non-modal branch (lines ~782-786) from composing Global++view to view-only:

```go
func (m Model) sidebarHelpGroups() [][]key.Binding {
	if m.activeModal != ModalNone {
		if km := m.modalKeyMap(); km != nil {
			return km.FullHelp()
		}
		return nil
	}
	// View-only (bt-dx7k): the Global map now lives on the ? overlay; ; shows
	// just the active view's actions. nil view map -> empty (the sidebar renders
	// an empty-view fallback; see ShortcutsSidebar.View).
	if km := m.viewSpecificKeyMap(); km != nil {
		return km.FullHelp()
	}
	return nil
}
```

Update the doc comment above the function (lines ~768-774) to describe view-only scoping (drop the "composes the Global map's groups with the active view's" sentence).

- [ ] **Step 4: Run test + build/vet to verify pass**

Run: `go build ./... && go vet ./... && go test ./pkg/ui -run TestSidebarHelpGroups -v`
Expected: PASS (`TestSidebarHelpGroups_NonModalShowsViewOnly` and the unchanged `TestSidebarHelpGroups_ModalShowsModalOnly`).

- [ ] **Step 5: Commit**

```bash
git add pkg/ui/model_footer.go pkg/ui/shortcuts_sidebar_test.go
git commit -m "fix(tui): scope ; sidebar to active-view map only (bt-dx7k)"
```

---

## Task 7: `;` sidebar → empty-view fallback

**Goal:** When the active view has no view-specific map (Attention, LabelDashboard → `sidebarHelpGroups()` empty), the sidebar renders a fallback line directing to `?`, instead of an empty panel.

**Files:**
- Modify: `pkg/ui/shortcuts_sidebar.go:85` (`View()`)
- Modify (test): `pkg/ui/shortcuts_sidebar_test.go` (add `TestShortcutsSidebar_EmptyViewFallback`)

**Interfaces:** none new (View renders fallback text when `s.groups` yields no rows).

- [ ] **Step 1: Write the failing fallback test**

Add to `pkg/ui/shortcuts_sidebar_test.go`:

```go
// TestShortcutsSidebar_EmptyViewFallback verifies the ; sidebar shows a fallback
// directing to ? when the active view has no view-specific bindings (bt-dx7k).
func TestShortcutsSidebar_EmptyViewFallback(t *testing.T) {
	sidebar := NewShortcutsSidebar(sidebarTestTheme())
	sidebar.SetSize(34, 20)
	sidebar.SetBindings(nil) // empty-view (e.g. Attention / LabelDashboard)

	view := sidebar.View()
	if !strings.Contains(view, "?") {
		t.Errorf("empty-view sidebar should direct to ? for global\n%s", view)
	}
	if !strings.Contains(strings.ToLower(view), "no actions") {
		t.Errorf("empty-view sidebar should state there are no view actions\n%s", view)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./pkg/ui -run TestShortcutsSidebar_EmptyViewFallback -v`
Expected: FAIL — empty groups render an empty body, no fallback text.

- [ ] **Step 3: Add the fallback in `View()`**

In `pkg/ui/shortcuts_sidebar.go`, after building `fullContent` from the groups (line ~133), substitute the fallback when nothing rendered:

```go
	fullContent := strings.TrimRight(sb.String(), "\n")
	if strings.TrimSpace(fullContent) == "" {
		fullContent = dimStyle.Render("no actions here") + "\n" +
			dimStyle.Render("press ? for global")
	}
	lines := strings.Split(fullContent, "\n")
```

- [ ] **Step 4: Run test + build/vet to verify pass**

Run: `go build ./... && go vet ./... && go test ./pkg/ui -run TestShortcutsSidebar_EmptyViewFallback -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/ui/shortcuts_sidebar.go pkg/ui/shortcuts_sidebar_test.go
git commit -m "fix(tui): empty-view fallback for ; sidebar (bt-dx7k)"
```

---

## Task 8: `;` sidebar → footer cross-reference to `?`

**Goal:** The sidebar footer carries a one-line cross-reference to the `?` overlay (symmetric with Task 4), without losing the existing scroll-% / hide hint.

**Files:**
- Modify: `pkg/ui/shortcuts_sidebar.go:159-168` (footer in `View()`)
- Modify (test): `pkg/ui/shortcuts_sidebar_test.go` (add `TestShortcutsSidebar_CrossRefFooter`)

**Interfaces:** none new.

- [ ] **Step 1: Write the failing cross-ref test**

Add to `pkg/ui/shortcuts_sidebar_test.go`:

```go
// TestShortcutsSidebar_CrossRefFooter verifies the ; sidebar footer cross-
// references the ? overlay (bt-dx7k), symmetric with the ? footer.
func TestShortcutsSidebar_CrossRefFooter(t *testing.T) {
	sidebar := NewShortcutsSidebar(sidebarTestTheme())
	sidebar.SetSize(34, 20)
	sidebar.SetBindings([][]key.Binding{
		{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open detail"))},
	})

	view := sidebar.View()
	if !strings.Contains(view, "?") {
		t.Errorf("; sidebar footer should cross-reference ? for global\n%s", view)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./pkg/ui -run TestShortcutsSidebar_CrossRefFooter -v`
Expected: FAIL — current footer is `; hide` or `ctrl+j/k scroll %`, no `?`.

- [ ] **Step 3: Add the cross-ref to the footer**

In `pkg/ui/shortcuts_sidebar.go`, update the footer block (lines ~159-168):

```go
	var footer string
	if totalLines > availableHeight {
		scrollPercent := 0
		if maxScroll > 0 {
			scrollPercent = s.scrollOffset * 100 / maxScroll
		}
		footer = dimStyle.Render(fmt.Sprintf("ctrl+j/k %d%%  -  ? global", scrollPercent))
	} else {
		footer = dimStyle.Render("? global  -  ; hide")
	}
```

- [ ] **Step 4: Run test + build/vet to verify pass**

Run: `go build ./... && go vet ./... && go test ./pkg/ui -run 'TestShortcutsSidebar' -v`
Expected: PASS (cross-ref test + the existing sidebar tests — confirm `TestShortcutsSidebarView` and width tests still pass with the longer footer).

- [ ] **Step 5: Commit**

```bash
git add pkg/ui/shortcuts_sidebar.go pkg/ui/shortcuts_sidebar_test.go
git commit -m "fix(tui): ; sidebar footer cross-references ? overlay (bt-dx7k)"
```

---

## Task 9: Whole-package gate, install, dogfood, bead close

**Goal:** Final verification across the package and binary; capture the dogfood evidence; close `bt-dx7k`. Push is HELD.

**Files:** none (verification + bead bookkeeping).

- [ ] **Step 1: Full build, vet, race-tested package run**

Run: `go build ./... && go vet ./... && go test ./pkg/ui -race`
Expected: PASS.

- [ ] **Step 2: Whole-repo test**

Run: `go test ./...`
Expected: PASS. If any non-ui package references the help surfaces (unlikely), fix and re-run.

- [ ] **Step 3: Install the binary**

Run: `go install ./cmd/bt/`
Expected: success (user runs `bt` from PATH).

- [ ] **Step 4: Dump the full scrunched matrix for the dogfood record**

Run: `BT_RENDER_DUMP=1 go test ./pkg/ui -run TestRenderDump`
Read `_tmp/render/modal_help_{30x20,50x14,70x20,120x40}.txt`. Confirm the acceptance criteria (spec "Acceptance"):
- `?` usable + non-clipping at narrow widths/short heights.
- `?` shows global only, responsive, working scroll.
- `;` shows active-view only with empty-view fallback (add a `sidebar_attention_*` harness scenario if no Attention/LabelDashboard sidebar dump exists).
- Both surfaces carry the one-line cross-ref.

- [ ] **Step 5: Live dogfood (maintainer)**

Suggest the user run `! bt` at ~70x20 and ~30x20: press `?` (global, scroll, dismiss), press `;` in List (view actions, `? global` footer), press `;` in Attention/LabelDashboard (empty-view fallback). This is the dogfood-before-public-main gate; the push stays held until the maintainer signs off.

- [ ] **Step 6: Close the bead** (held; do NOT push)

```bash
bd update bt-dx7k --claim   # if not already in_progress
# After maintainer dogfood sign-off:
bd close bt-dx7k --reason-file .beads/tmp/bt-dx7k-close.txt
```

Close reason (write to `.beads/tmp/bt-dx7k-close.txt`, ASCII) following the project template: Summary / Change / Files / Verify / Risk / Notes. Note the adjacent `bt-yvr4g` (per-view map gap audit) now pairs with the view-only `;`.

- [ ] **Step 7: HOLD.** Do not `git push` or `bd dolt push`. Report the branch state and the dogfood dumps; wait for the maintainer.

---

## Self-Review

**Spec coverage** (against `docs/design/2026-06-22-bt-dx7k-help-surface-split.md`):
- `?` = Global only → Task 1. ✓
- Task-oriented headers (label existing 4 groups) → Task 1 (SWITCH VIEWS / DO THINGS / WORKSPACE / CHROME, essentials-first). ✓
- Status-glyph legend retained → Task 2 (`helpOverlayPanels` appends it). ✓
- Responsive columns 4/2/1 → Task 2. ✓
- Working scroll (wire dead `helpScroll`) → Task 3. ✓
- Essentials-first ordering → Task 1 (Views first). ✓
- `;` = active-view only (drop Global prefix) → Task 6. ✓
- `;` empty-view fallback → Task 7. ✓
- Symmetric one-line cross-ref footers → Task 4 (`?`) + Task 8 (`;`). ✓
- No `??` layer → nothing adds one. ✓
- Single-source-of-truth (key.Map FullHelp, no literal tables) → preserved; only the status legend keeps literal rows (it has no key.Map, as today). ✓
- Testing: harness scrunched matrix (70x20 repro extended to 50x14/30x20) + scoping + scroll-window assertions + live run → Tasks 3, 5, 9. ✓

**Placeholder scan:** No "TBD"/"handle edge cases"/"similar to Task N". Code shown for every code step. The two large closures moved in Task 2 are explicitly "moved verbatim from `renderHelpOverlay`" with source lines cited (they are unchanged — copying them is mechanical, not re-specified). The status-legend rows in Task 2 say "verbatim from the original" with the source line range, because the glyph right-tokens are existing UI Unicode that must be preserved exactly.

**Type consistency:** `helpOverlayPanels() []string` (Task 2) ← consumed by `helpOverlayBodyLines() []string` (Task 2) ← consumed by `renderHelpOverlay` window + `helpScrollMax()` (Task 3). `helpOverlayColumns(int) int` (Task 2). `helpOverlayAvailBody() int` / `helpScrollMax() int` (Task 3) use `helpOverlayChrome` const (set to 4 in Task 3, corrected to 3 in Task 4 when the subtitle line is removed). `sidebarHelpGroups() [][]key.Binding` signature unchanged (Task 6). All names consistent across tasks.

**Known follow-ups (out of scope):**
- CRUD write/read key distinction (bt-oiaj), L1 contextual-hint redesign (bt-d5wr), run-context surfacing (bt-npxi) — stay in bt-xavk.
- `"cass session"` binding (`V`, `keys/list.go:162`) is stale (cass→sym rename; silent no-op; modal suggests dead `cass search`) — tracked as a separate bead, NOT addressed here. It is excluded from all test markers in this plan.
- Header label strings and exact width thresholds are dogfood tuning points, not fixed contracts.
