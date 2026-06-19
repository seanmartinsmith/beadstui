package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// epicCardFixture: one epic with 3 children (natural order .1 .2 .10) plus a
// standalone task, all visible under the "all" filter.
func epicCardFixture() []model.Issue {
	return []model.Issue{
		{ID: "ep1", Title: "Epic one", IssueType: model.TypeEpic, Status: model.StatusOpen},
		{ID: "ep1.1", Title: "Child one", Status: model.StatusClosed, Dependencies: pcDep("ep1.1", "ep1")},
		{ID: "ep1.2", Title: "Child two", Status: model.StatusInProgress, Dependencies: pcDep("ep1.2", "ep1")},
		{ID: "ep1.10", Title: "Child ten", Status: model.StatusOpen, Dependencies: pcDep("ep1.10", "ep1")},
		{ID: "task1", Title: "Lonely task", IssueType: model.TypeTask, Status: model.StatusOpen},
	}
}

// epicCardModel builds a fully-wired ViewList model with the "all" filter so
// every child is in the visible list (drill targets resolve).
func epicCardModel(issues []model.Issue) Model {
	m := NewModel(issues, nil, "", nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = nm.(Model)
	m.filter.currentFilter = "all"
	m.applyFilter()
	return m
}

func TestOpenEpicCard(t *testing.T) {
	m := epicCardModel(epicCardFixture())
	m.epicCardCursor = 5 // dirty state from a prior open
	m.openEpicCard("ep1")
	if m.activeModal != ModalEpicCard {
		t.Errorf("activeModal=%v, want ModalEpicCard", m.activeModal)
	}
	if m.epicCardID != "ep1" {
		t.Errorf("epicCardID=%q, want ep1", m.epicCardID)
	}
	if m.epicCardCursor != 0 {
		t.Errorf("epicCardCursor=%d, want 0 (reset on open)", m.epicCardCursor)
	}
}

func TestHandleEpicCardKeys_Navigation(t *testing.T) {
	m := epicCardModel(epicCardFixture())
	m.openEpicCard("ep1") // 3 children -> cursor range [0,2]

	m = m.handleEpicCardKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.epicCardCursor != 1 {
		t.Errorf("after j: cursor=%d, want 1", m.epicCardCursor)
	}
	m = m.handleEpicCardKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.epicCardCursor != 2 {
		t.Errorf("after 2nd j: cursor=%d, want 2", m.epicCardCursor)
	}
	// Clamp at the last child.
	m = m.handleEpicCardKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.epicCardCursor != 2 {
		t.Errorf("after 3rd j: cursor=%d, want 2 (clamped)", m.epicCardCursor)
	}
	// Up moves back and clamps at the top.
	m = m.handleEpicCardKeys(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m = m.handleEpicCardKeys(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m = m.handleEpicCardKeys(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.epicCardCursor != 0 {
		t.Errorf("after 3x k: cursor=%d, want 0 (clamped)", m.epicCardCursor)
	}
}

func TestHandleEpicCardKeys_Exit(t *testing.T) {
	m := epicCardModel(epicCardFixture())
	m.openEpicCard("ep1")
	m = m.handleEpicCardKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.activeModal != ModalNone {
		t.Errorf("esc should close the card, activeModal=%v", m.activeModal)
	}
}

func TestHandleEpicCardKeys_Drill(t *testing.T) {
	m := epicCardModel(epicCardFixture())
	m.openEpicCard("ep1")
	// cursor 0 = ep1.1 (children sorted .1 .2 .10).
	m = m.handleEpicCardKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.activeModal != ModalNone {
		t.Fatalf("drill should close the card, activeModal=%v", m.activeModal)
	}
	if m.focused != focusDetail {
		t.Errorf("drill should focus the detail pane, focused=%v", m.focused)
	}
	sel, ok := m.list.SelectedItem().(IssueItem)
	if !ok {
		t.Fatalf("selected item is not an IssueItem")
	}
	if sel.Issue.ID != "ep1.1" {
		t.Errorf("drill selected %q, want ep1.1", sel.Issue.ID)
	}
}

func TestListKeys_EpicCard_OpensOnEpic(t *testing.T) {
	m := epicCardModel(epicCardFixture())
	if !m.selectIssueByID("ep1") {
		t.Fatal("setup: ep1 not in visible list")
	}
	m = m.handleListKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if m.activeModal != ModalEpicCard {
		t.Fatalf("e on an epic should open the card, activeModal=%v", m.activeModal)
	}
	if m.epicCardID != "ep1" {
		t.Errorf("epicCardID=%q, want ep1", m.epicCardID)
	}
}

func TestListKeys_EpicCard_NoopOnNonEpic(t *testing.T) {
	m := epicCardModel(epicCardFixture())
	if !m.selectIssueByID("task1") {
		t.Fatal("setup: task1 not in visible list")
	}
	m = m.handleListKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if m.activeModal == ModalEpicCard {
		t.Fatal("e on a non-epic should not open the card")
	}
}

func TestRenderEpicCard_Basic(t *testing.T) {
	m := epicCardModel(epicCardFixture())
	m.openEpicCard("ep1")
	out := m.renderEpicCard()
	if !strings.Contains(out, "Epic ep1") {
		t.Error("card title should contain 'Epic ep1'")
	}
	if !strings.Contains(out, "ep1.1") {
		t.Error("card should list child ep1.1")
	}
	if !strings.Contains(out, "drill") {
		t.Error("card footer should mention drill")
	}
}

func TestRenderEpicCard_NoChildren(t *testing.T) {
	issues := []model.Issue{
		{ID: "solo", Title: "Childless epic", IssueType: model.TypeEpic, Status: model.StatusOpen},
	}
	m := epicCardModel(issues)
	m.openEpicCard("solo")
	out := m.renderEpicCard()
	if !strings.Contains(out, "No children") {
		t.Errorf("childless epic card should say 'No children', got:\n%s", out)
	}
}
