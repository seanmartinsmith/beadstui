package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestHandleMouseClick_NoModalRequired(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200
	m.height = 40
	m.activeModal = ModalHelp
	msg := tea.MouseClickMsg{X: 50, Y: 10, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)
	if got.focused != m.focused {
		t.Fatalf("focus should not change when modal is open")
	}
}

func TestHandleMouseClick_SplitViewSwitchesFocus(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	// Force a known listInnerWidth so we can reason about boundary.
	m.list.SetSize(60, 30)
	m.focused = focusList

	// Click to the right of the list boundary → should focus detail.
	rightClick := tea.MouseClickMsg{X: 150, Y: 5, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(rightClick)
	if got.focused != focusDetail {
		t.Fatalf("expected focusDetail after click on detail pane, got %v", got.focused)
	}

	// Click on the left side → should focus list.
	leftClick := tea.MouseClickMsg{X: 2, Y: 5, Button: tea.MouseLeft}
	got.focused = focusDetail
	got, _ = got.handleMouseClick(leftClick)
	if got.focused != focusList {
		t.Fatalf("expected focusList after click on list pane, got %v", got.focused)
	}
}

func TestHandleMouseClick_RightButtonIgnored(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.focused = focusList

	msg := tea.MouseClickMsg{X: 150, Y: 5, Button: tea.MouseRight}
	got, _ := m.handleMouseClick(msg)
	if got.focused != focusList {
		t.Fatalf("right-click should not change focus, got %v", got.focused)
	}
}

func TestHandleMouseClick_FooterIgnored(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewList
	m.isSplitView = true
	m.focused = focusList

	// Click on the footer row (last line) → should be a no-op.
	msg := tea.MouseClickMsg{X: 150, Y: 39, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)
	if got.focused != focusList {
		t.Fatalf("footer click should not change focus, got %v", got.focused)
	}
}

func TestHandleMouseClick_NonListModeIgnored(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200
	m.height = 40
	m.mode = ViewBoard // not ViewList
	m.isSplitView = true
	m.focused = focusBoard

	msg := tea.MouseClickMsg{X: 150, Y: 5, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)
	if got.focused != focusBoard {
		t.Fatalf("non-list mode should not change focus, got %v", got.focused)
	}
}
