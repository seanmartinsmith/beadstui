package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// Tests that verify splitViewListChromeHeight() matches the Y where the first
// list item actually renders in the full View() output. The existing
// TestHandleMouseClick_RowMathMatchesChrome asks the implementation for the
// expected Y and then clicks there — so the formula and the assertion move
// together. These tests render the view, find the item's real Y by scanning
// the rendered bytes, and compare. Catches drift in bubbles-list phantom
// behavior, panel chrome, or pill rendering that the self-consistent test
// would miss (bt-ej61).

func stripANSI(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !isCSITerm(s[j]) {
				j++
			}
			i = j
			continue
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

func isCSITerm(b byte) bool { return b >= 0x40 && b <= 0x7e }

func findRenderedItemY(rendered, needle string) int {
	for i, line := range strings.Split(rendered, "\n") {
		if strings.Contains(stripANSI(line), needle) {
			return i
		}
	}
	return -1
}

func mouseTestModel(n int, w, h, listW, listH int) Model {
	var issues []model.Issue
	for i := 0; i < n; i++ {
		issues = append(issues, model.Issue{
			ID:     "bd-x" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)),
			Title:  "title",
			Status: model.StatusOpen,
		})
	}
	m := NewModel(issues, nil, "", nil)
	m.width = w
	m.height = h
	m.mode = ViewList
	m.isSplitView = true
	m.list.SetSize(listW, listH)
	m.focused = focusList
	m.ready = true
	return m
}

// Default split view: no pill, no filtering, wide pane.
func TestMouseClick_FormulaMatchesRender_Default(t *testing.T) {
	m := mouseTestModel(3, 200, 40, 60, 30)
	formulaY := m.splitViewListChromeHeight()
	actualY := findRenderedItemY(m.View().Content, "bd-xaa")
	if formulaY != actualY {
		t.Errorf("chrome height drifted: formula=%d actual=%d", formulaY, actualY)
	}
}

// Workspace mode (REPO badges on each row): header layout doesn't change row count.
func TestMouseClick_FormulaMatchesRender_WorkspaceMode(t *testing.T) {
	m := mouseTestModel(3, 200, 40, 60, 30)
	m.workspaceMode = true
	formulaY := m.splitViewListChromeHeight()
	actualY := findRenderedItemY(m.View().Content, "bd-xaa")
	if formulaY != actualY {
		t.Errorf("chrome height drifted in workspace mode: formula=%d actual=%d", formulaY, actualY)
	}
}

// FilterApplied state: the search pill adds a row above the header.
func TestMouseClick_FormulaMatchesRender_WithPill(t *testing.T) {
	m := mouseTestModel(3, 200, 40, 60, 30)
	m.list.SetFilterText("xaa")
	formulaY := m.splitViewListChromeHeight()
	actualY := findRenderedItemY(m.View().Content, "bd-xaa")
	if formulaY != actualY {
		t.Errorf("chrome height drifted with pill: formula=%d actual=%d", formulaY, actualY)
	}
}

// Paginated list, still on page 0: click on rendered Y+2 must select index 2.
func TestMouseClick_ResolvesThirdRowOnFirstPage(t *testing.T) {
	m := mouseTestModel(200, 200, 40, 60, 10)
	firstY := findRenderedItemY(m.View().Content, "bd-xaa")
	clicked, _ := m.handleMouseClick(tea.MouseClickMsg{
		X: 10, Y: firstY + 2, Button: tea.MouseLeft,
	})
	if got := clicked.list.Index(); got != 2 {
		t.Errorf("click at rendered Y=%d expected index 2, got %d", firstY+2, got)
	}
}

// Scrolled past page 0: click on first visible rendered row must select the
// first-visible index (page * perPage), not index 0.
func TestMouseClick_ResolvesFirstRowAcrossPages(t *testing.T) {
	m := mouseTestModel(200, 200, 40, 60, 10)
	m.list.Select(25)
	content := m.View().Content
	firstY := findRenderedItemY(content, "bd-x")
	clicked, _ := m.handleMouseClick(tea.MouseClickMsg{
		X: 10, Y: firstY, Button: tea.MouseLeft,
	})
	expected := m.list.Paginator.Page * m.list.Paginator.PerPage
	if got := clicked.list.Index(); got != expected {
		t.Errorf("click at page-%d first visible Y=%d: expected index %d, got %d",
			m.list.Paginator.Page, firstY, expected, got)
	}
}
