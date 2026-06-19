package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// TestHistoryFileTreeFocusNoGlobalLeak (bt-s2xpy, acceptance #5): when the
// history file tree has focus, global view-switch letters (g/b/i/a) must act
// in-pane / be inert, NOT switch views. Before the bt-s2xpy dispatcher guard
// these leaked because the global view-switch switch ran before
// handleHistoryKeys.
func TestHistoryFileTreeFocusNoGlobalLeak(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Test 1", Status: model.StatusOpen, Priority: 0},
		{ID: "bv-2", Title: "Test 2", Status: model.StatusOpen, Priority: 1},
	}
	m := NewModel(issues, nil, "", nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	m = updated.(Model)

	report := createTestHistoryReportWithFiles()
	m.historyView = NewHistoryModel(report, m.theme)
	m.historyView.SetSize(180, 49)
	m.mode = ViewHistory
	m.focused = focusHistory
	m.historyLoading = false

	m.historyView.ToggleFileTree()
	m.historyView.SetFileTreeFocus(true)

	if !m.historyView.FileTreeHasFocus() {
		t.Fatal("setup failed: file tree should have focus")
	}
	if m.historyView.IsSearchActive() {
		t.Fatal("setup failed: search should NOT be active (file-tree sub-state only)")
	}

	for _, k := range []rune{'g', 'b', 'i', 'a'} {
		m.mode = ViewHistory
		m.focused = focusHistory
		m.historyView.SetFileTreeFocus(true)

		updated, _ = m.Update(tea.KeyPressMsg{Code: k, Text: string(k)})
		m = updated.(Model)

		if m.mode != ViewHistory {
			t.Errorf("LEAK: pressing %q in history file-tree focus switched view to %v (expected ViewHistory)", string(k), m.mode)
		}
	}
}

// TestBoardSearchModeNoGlobalLeak (bt-s2xpy, acceptance #2): in board search
// mode, typing letters (g/a/i) must append to the query, NOT fire global
// view-switch bindings, and must stay in search mode.
func TestBoardSearchModeNoGlobalLeak(t *testing.T) {
	issues := []model.Issue{
		{ID: "auth-1", Title: "Auth thing", Status: model.StatusOpen, Priority: 0},
		{ID: "bug-2", Title: "Bug thing", Status: model.StatusInProgress, Priority: 1},
	}
	m := NewModel(issues, nil, "", nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = updated.(Model)
	if m.mode != ViewBoard {
		t.Fatalf("setup failed: expected ViewBoard after 'b', got %v", m.mode)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = updated.(Model)
	if !m.board.IsSearchMode() {
		t.Fatalf("setup failed: board should be in search mode after '/'")
	}

	for _, k := range []rune{'g', 'a', 'i'} {
		updated, _ = m.Update(tea.KeyPressMsg{Code: k, Text: string(k)})
		m = updated.(Model)

		if m.mode != ViewBoard {
			t.Errorf("LEAK: typing %q in board search switched view to %v (expected ViewBoard)", string(k), m.mode)
		}
		if !m.board.IsSearchMode() {
			t.Errorf("LEAK: typing %q in board search dropped out of search mode", string(k))
		}
	}
}
