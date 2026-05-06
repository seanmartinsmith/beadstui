package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// TestHistoryClickAt_MissTargets covers the noPane sentinel paths.
func TestHistoryClickAt_MissTargets(t *testing.T) {
	report := createRichHistoryReport()
	h := NewHistoryModel(report, DefaultTheme())
	h.SetSize(180, 40)

	// Out-of-bounds coordinates always miss.
	cases := []struct {
		name string
		x, y int
	}{
		{"x<0", -1, 5},
		{"y<0", 5, -1},
		{"x>=width", 200, 5},
		{"y>=height", 5, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hit := h.ClickAt(tc.x, tc.y)
			if hit.Pane != noPane {
				t.Errorf("expected noPane for %s, got pane=%d", tc.name, hit.Pane)
			}
			if hit.HasItem {
				t.Errorf("expected HasItem=false for %s", tc.name)
			}
		})
	}

	// Header rows return noPane regardless of x.
	hit := h.ClickAt(50, 0)
	if hit.Pane != noPane {
		t.Errorf("header row click: expected noPane, got pane=%d", hit.Pane)
	}
}

// TestHistoryClickAt_BeadsListPane: clicks on rows of the BEADS pane in
// the wide bead-mode layout map to histories[] indices respecting scroll.
// Mirrors TestLabelPickerItemAtPanelY's row-mapping shape.
func TestHistoryClickAt_BeadsListPane(t *testing.T) {
	report := createRichHistoryReport()
	h := NewHistoryModel(report, DefaultTheme())
	h.SetSize(180, 40)

	// Compute the header height up front -- the panel block starts the row
	// after the header. Click on (panelTop + 1) hits content row 0.
	headerRows := historyHeaderRows(&h)
	contentRow0Y := headerRows + 1

	// Wide bead-mode list pane occupies x in [0, 180*0.20) = [0, 36).
	// Click well inside that range so we don't hit a border column.
	clickX := 5

	hit := h.ClickAt(clickX, contentRow0Y)
	if hit.Pane != historyFocusList {
		t.Fatalf("expected historyFocusList, got pane=%d", hit.Pane)
	}
	if !hit.HasItem {
		t.Fatalf("expected HasItem=true, got false (item=%d)", hit.Item)
	}
	if hit.Item != 0 {
		t.Errorf("expected item=0 (first visible bead), got %d", hit.Item)
	}

	// Click on row 1 -> bead at scrollOffset+1.
	hit = h.ClickAt(clickX, contentRow0Y+1)
	if !hit.HasItem || hit.Item != 1 {
		t.Errorf("row 1: expected item=1, got HasItem=%v item=%d", hit.HasItem, hit.Item)
	}
}

// TestHistoryClickAt_CommitsMiddlePane: clicks on rows of the COMMITS pane
// map to commits[] indices on the currently-selected bead.
func TestHistoryClickAt_CommitsMiddlePane(t *testing.T) {
	report := createRichHistoryReport()
	h := NewHistoryModel(report, DefaultTheme())
	h.SetSize(180, 40)

	headerRows := historyHeaderRows(&h)
	contentRow0Y := headerRows + 1

	// Wide bead-mode middle (commits) pane: starts at x = 180*(0.20+0.22) = ~75.
	// Click somewhere inside the commits pane, not on its border column.
	clickX := 90

	hit := h.ClickAt(clickX, contentRow0Y)
	if hit.Pane != historyFocusMiddle {
		t.Fatalf("expected historyFocusMiddle, got pane=%d", hit.Pane)
	}
	if !hit.HasItem {
		t.Fatalf("expected HasItem=true, got false")
	}
	if hit.Item != 0 {
		t.Errorf("commit row 0: expected item=0, got %d", hit.Item)
	}

	// Commit row 5 with no scroll = commits[5].
	hit = h.ClickAt(clickX, contentRow0Y+5)
	if !hit.HasItem || hit.Item != 5 {
		t.Errorf("commit row 5: expected item=5, got HasItem=%v item=%d", hit.HasItem, hit.Item)
	}
}

// TestHistoryClickAt_DetailPaneFocusOnly: clicks inside the COMMIT DETAILS
// pane never set HasItem -- per the bt-y3ip acceptance, the detail pane is
// focus-only (Tab-equivalent) on click.
func TestHistoryClickAt_DetailPaneFocusOnly(t *testing.T) {
	report := createRichHistoryReport()
	h := NewHistoryModel(report, DefaultTheme())
	h.SetSize(180, 40)

	headerRows := historyHeaderRows(&h)
	contentRow0Y := headerRows + 1

	// Wide bead-mode detail pane starts at x = 180*(0.20+0.22+0.25) = ~120.
	clickX := 150

	hit := h.ClickAt(clickX, contentRow0Y+3)
	if hit.Pane != historyFocusDetail {
		t.Fatalf("expected historyFocusDetail, got pane=%d", hit.Pane)
	}
	if hit.HasItem {
		t.Errorf("detail pane click: expected HasItem=false (focus-only), got true item=%d", hit.Item)
	}
}

// TestHistoryClickAt_BorderRowsFocusOnly: clicks on top/bottom border rows
// resolve to the pane (focus-only) but never set HasItem.
func TestHistoryClickAt_BorderRowsFocusOnly(t *testing.T) {
	report := createRichHistoryReport()
	h := NewHistoryModel(report, DefaultTheme())
	h.SetSize(180, 40)

	headerRows := historyHeaderRows(&h)
	topBorderY := headerRows
	bottomBorderY := h.height - 1

	hit := h.ClickAt(5, topBorderY)
	if hit.Pane != historyFocusList {
		t.Errorf("top border list pane: expected historyFocusList, got pane=%d", hit.Pane)
	}
	if hit.HasItem {
		t.Errorf("top border: expected HasItem=false, got true")
	}

	hit = h.ClickAt(5, bottomBorderY)
	if hit.Pane != historyFocusList {
		t.Errorf("bottom border list pane: expected historyFocusList, got pane=%d", hit.Pane)
	}
	if hit.HasItem {
		t.Errorf("bottom border: expected HasItem=false, got true")
	}
}

// TestHistoryClickAt_TimelineGapWideBead: in wide bead mode the timeline
// pane is supplementary and not focusable. Clicks there return noPane.
func TestHistoryClickAt_TimelineGapWideBead(t *testing.T) {
	report := createRichHistoryReport()
	h := NewHistoryModel(report, DefaultTheme())
	h.SetSize(180, 40)

	headerRows := historyHeaderRows(&h)
	// Timeline pane in wide bead mode: x in [180*0.20, 180*(0.20+0.22)) = [36, 75).
	clickX := 50

	hit := h.ClickAt(clickX, headerRows+2)
	if hit.Pane != noPane {
		t.Errorf("timeline pane click: expected noPane (timeline isn't focusable), got pane=%d", hit.Pane)
	}
}

// TestHistoryClickAt_StandardLayout: 100-150 col layout has 3 panes
// (no timeline). Verify the boundaries.
func TestHistoryClickAt_StandardLayout(t *testing.T) {
	report := createRichHistoryReport()
	h := NewHistoryModel(report, DefaultTheme())
	h.SetSize(120, 40) // standard layout (100..150)

	headerRows := historyHeaderRows(&h)
	contentY := headerRows + 1

	// Standard 30/35/35 split at width 120: list = 36, middle = 42.
	cases := []struct {
		x    int
		want historyFocus
	}{
		{0, historyFocusList},
		{20, historyFocusList},
		{50, historyFocusMiddle},
		{100, historyFocusDetail},
	}
	for _, tc := range cases {
		hit := h.ClickAt(tc.x, contentY)
		if hit.Pane != tc.want {
			t.Errorf("x=%d: expected pane=%d, got pane=%d", tc.x, tc.want, hit.Pane)
		}
	}
}

// TestHistoryMouseClickRoutesToHistoryView: end-to-end check that a
// MouseClickMsg in ViewHistory updates the inner historyView focus and
// selection through Model.handleMouseClick.
func TestHistoryMouseClickRoutesToHistoryView(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Test", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	m = updated.(Model)

	// Prime the history view with a rich fixture and force ViewHistory.
	report := createRichHistoryReport()
	m.historyView = NewHistoryModel(report, m.theme)
	m.historyView.SetSize(180, 49)
	m.mode = ViewHistory
	m.focused = focusHistory
	m.historyLoading = false

	// Click on commits pane (wide bead mode middle starts ~x=75); pick a
	// content row 2 below the panel top border.
	headerRows := historyHeaderRows(&m.historyView)
	clickY := headerRows + 1 + 2
	clickX := 90

	updated, _ = m.Update(tea.MouseClickMsg{X: clickX, Y: clickY, Button: tea.MouseLeft})
	m = updated.(Model)

	if m.historyView.focused != historyFocusMiddle {
		t.Errorf("after click in middle pane: expected historyFocusMiddle, got %d", m.historyView.focused)
	}
	if m.historyView.selectedCommit != 2 {
		t.Errorf("after click on commit row 2: expected selectedCommit=2, got %d", m.historyView.selectedCommit)
	}
}

// TestHistoryMouseWheelRoutesPerPane: wheel events with X coordinates over
// different panes scroll those panes' state, not just the global bead list.
// Wheel-on-detail must scroll detailScrollOffset; wheel-on-middle (bead
// mode) must move the commit cursor; wheel-on-list moves the bead cursor.
func TestHistoryMouseWheelRoutesPerPane(t *testing.T) {
	issues := []model.Issue{{ID: "bv-1", Title: "Test", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	m = updated.(Model)

	report := createRichHistoryReport()
	m.historyView = NewHistoryModel(report, m.theme)
	m.historyView.SetSize(180, 49)
	m.mode = ViewHistory
	m.focused = focusHistory
	m.historyLoading = false

	// Wheel down on the BEADS pane -> bead cursor advances.
	startBead := m.historyView.selectedBead
	updated, _ = m.Update(tea.MouseWheelMsg{X: 5, Y: 10, Button: tea.MouseWheelDown})
	m = updated.(Model)
	if m.historyView.selectedBead != startBead+1 {
		t.Errorf("wheel-down on list: expected bead cursor to advance, got %d->%d", startBead, m.historyView.selectedBead)
	}

	// Wheel down on the COMMITS pane -> commit cursor advances.
	startCommit := m.historyView.selectedCommit
	updated, _ = m.Update(tea.MouseWheelMsg{X: 90, Y: 10, Button: tea.MouseWheelDown})
	m = updated.(Model)
	if m.historyView.selectedCommit != startCommit+1 {
		t.Errorf("wheel-down on middle (bead mode): expected commit cursor to advance, got %d->%d", startCommit, m.historyView.selectedCommit)
	}

	// Wheel down on the DETAIL pane -> detailScrollOffset advances. Need a
	// tall enough content list for scroll to be possible -- the rich fixture
	// has 12 commits per bead so the offset can grow.
	startOffset := m.historyView.detailScrollOffset
	updated, _ = m.Update(tea.MouseWheelMsg{X: 150, Y: 10, Button: tea.MouseWheelDown})
	m = updated.(Model)
	if m.historyView.detailScrollOffset != startOffset+1 {
		t.Errorf("wheel-down on detail: expected detailScrollOffset to advance, got %d->%d", startOffset, m.historyView.detailScrollOffset)
	}
}

// historyHeaderRows is a small helper to compute the rendered header height
// once and reuse across the table tests above. Mirrors the same calculation
// done inside ClickAt.
func historyHeaderRows(h *HistoryModel) int {
	header := h.renderHeader()
	rows := strings.Count(header, "\n") + 1
	if rows < 1 {
		rows = 1
	}
	return rows
}
