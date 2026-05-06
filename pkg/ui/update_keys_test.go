package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// Cover additional branches in Model.Update for quit/help/tab handling and update notices.
func TestUpdateHelpQuitAndTabFocus(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "One", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)

	// Make model ready and split view
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Help toggle via ? then dismiss with another key
	updated, _ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m = updated.(Model)
	if m.activeModal != ModalHelp || m.focused != focusHelp {
		t.Fatalf("expected help overlay shown")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m = updated.(Model)
	if m.activeModal == ModalHelp || m.focused != focusList {
		t.Fatalf("expected help overlay dismissed")
	}

	// Tab should flip focus in split view
	if m.focused != focusList {
		t.Fatalf("expected list focus before tab")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)
	if m.focused != focusDetail {
		t.Fatalf("expected detail focus after tab")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)
	if m.focused != focusList {
		t.Fatalf("expected list focus after second tab")
	}

	// Escape should show quit confirm, 'y' should issue tea.Quit
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)
	if m.activeModal != ModalQuitConfirm {
		t.Fatalf("expected quit confirm after esc")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatalf("expected quit command on confirm quit")
	}
}

func TestUpdateMsgSetsUpdateAvailable(t *testing.T) {
	m := NewModel([]model.Issue{{ID: "1", Title: "One", Status: model.StatusOpen}}, nil, "", nil)
	updated, _ := m.Update(UpdateMsg{TagName: "v9.9.9", URL: "https://example"})
	m = updated.(Model)
	if !m.updateAvailable || m.updateTag != "v9.9.9" {
		t.Fatalf("update flag not set")
	}
}

func TestHistoryViewToggle(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Test Issue", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)

	// Make model ready
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// h should toggle history view on
	if m.mode == ViewHistory {
		t.Fatalf("history view should be off initially")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	if m.mode != ViewHistory {
		t.Fatalf("expected history view to be on after h key")
	}
	if m.focused != focusHistory {
		t.Fatalf("expected focus to be on history, got %v", m.focused)
	}

	// h again should toggle off
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	if m.mode == ViewHistory {
		t.Fatalf("expected history view to be off after second h key")
	}
	if m.focused != focusList {
		t.Fatalf("expected focus to be back on list, got %v", m.focused)
	}
}

// TestHistoryViewTransitionNoLeakage covers bt-7hhc at the Model level.
// After pressing `h` to enter history view, the full rendered View output
// must NOT contain any issues-list row signatures (repo badges, P0/P1
// status codes, [BUG]-style type tags). If it does, the transition is
// leaking content through HistoryModel rendering. If this passes but the
// user still sees leakage in the running TUI, the issue is in the
// Bubble Tea v2 / terminal renderer layer below us.
func TestHistoryViewTransitionNoLeakage(t *testing.T) {
	issues := []model.Issue{
		{ID: "dotfiles-d6n", Title: "Some dotfiles work", Status: model.StatusOpen, Priority: 0},
		{ID: "bv-2", Title: "Other work", Status: model.StatusOpen, Priority: 1},
	}
	m := NewModel(issues, nil, "", nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	m = updated.(Model)

	// Press h to enter history view (this synchronously runs enterHistoryView
	// which may fail without a real git repo — that's fine; we only need to
	// verify the rendered output of whatever state we land in is clean).
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	view := m.View()
	rendered := view.Content

	leaks := []string{
		"P0 OPEN", "P1 OPEN", "P2 OPEN", "P3 OPEN",
		"[DOTF]",
		"[BUG]", "[FEATURE]", "[EPIC]", "[DECISION]",
	}
	for _, leak := range leaks {
		if strings.Contains(rendered, leak) {
			t.Errorf("post-transition render leaks issues-list pattern %q", leak)
		}
	}

	// The rendered output must cover the full terminal so that the diff
	// renderer in bubbletea/ultraviolet does not leave residual cells from
	// the previous frame. Each row should be at least m.width wide and
	// there should be at least m.height rows. Without this, partially
	// covered rows could explain the "issues-list rows showing inside
	// history panes" symptom — the rows are NOT history content; they are
	// stale terminal cells.
	rows := strings.Split(rendered, "\n")
	if len(rows) < m.height {
		t.Errorf("render produced %d rows, expected at least %d (height); short-renders leave stale cells", len(rows), m.height)
	}
}

// TestHistorySearchKeyIsolation covers bt-mc4y: while history search is
// active, every printable key must reach the searchInput rather than firing
// a global mode toggle. Before the fix, typing `h` in history search closed
// the history view because the global `h = toggle history` handler ran
// before the focus-based dispatch reached handleHistoryKeys.
func TestHistorySearchKeyIsolation(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Test Issue", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Enter history view.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)
	if m.mode != ViewHistory {
		t.Fatalf("setup: expected ViewHistory after h key, got %v", m.mode)
	}

	// Activate search via /.
	updated, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = updated.(Model)
	if !m.historyView.IsSearchActive() {
		t.Fatalf("setup: expected search active after /")
	}

	// Type a sequence that mixes plain letters with letters that map to
	// global hotkeys (h = toggle history, b = board, g = graph, i =
	// insights, p = priority hints, a = actionable). Every keypress must
	// land in the search buffer and leave m.mode == ViewHistory.
	seq := []rune{'h', 'b', 'g', 'i', 'p', 'a'}
	for _, r := range seq {
		updated, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(Model)
		if m.mode != ViewHistory {
			t.Fatalf("keypress %q leaked through search and changed mode to %v", r, m.mode)
		}
		if !m.historyView.IsSearchActive() {
			t.Fatalf("keypress %q deactivated search", r)
		}
	}

	if got, want := m.historyView.SearchQuery(), string(seq); got != want {
		t.Fatalf("search buffer = %q, want %q", got, want)
	}

	// Delete key (forward delete) at end of buffer is a no-op in bubbles
	// textinput, but it must NOT fire any global handler. The buffer stays
	// the same and the view stays in history+search.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	m = updated.(Model)
	if m.mode != ViewHistory {
		t.Fatalf("Delete keypress changed mode to %v", m.mode)
	}
	if !m.historyView.IsSearchActive() {
		t.Fatalf("Delete keypress deactivated search")
	}
	if got, want := m.historyView.SearchQuery(), string(seq); got != want {
		t.Fatalf("buffer changed after no-op Delete: got %q want %q", got, want)
	}

	// Esc closes search (and only search — view stays as ViewHistory).
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)
	if m.historyView.IsSearchActive() {
		t.Fatalf("Esc did not deactivate search")
	}
	if m.mode != ViewHistory {
		t.Fatalf("Esc on active search exited history view; expected to stay in ViewHistory, got %v", m.mode)
	}
}

func TestHistoryViewKeys(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Test Issue", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)

	// Make model ready
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Enter history view
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	// Esc should close history view
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)

	if m.mode == ViewHistory {
		t.Fatalf("expected history view to be closed after Esc")
	}

	// Re-enter and test 'c' key cycles confidence
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	initialConf := m.historyView.GetMinConfidence()
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updated.(Model)

	if m.historyView.GetMinConfidence() == initialConf {
		t.Fatalf("expected confidence to change after 'c' key")
	}
}
