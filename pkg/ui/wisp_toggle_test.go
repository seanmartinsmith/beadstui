package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func TestWispToggle(t *testing.T) {
	t.Run("default hides wisps", func(t *testing.T) {
		m := Model{}
		if m.showWisps {
			t.Error("showWisps should default to false")
		}
	})

	t.Run("toggle flips state", func(t *testing.T) {
		m := Model{}
		m.showWisps = !m.showWisps
		if !m.showWisps {
			t.Error("first toggle should enable wisps")
		}
		m.showWisps = !m.showWisps
		if m.showWisps {
			t.Error("second toggle should disable wisps")
		}
	})
}

func TestWispFiltering(t *testing.T) {
	boolTrue := true
	boolFalse := false

	normalIssue := model.Issue{ID: "bt-001", Title: "Normal", Status: model.StatusOpen}
	wispIssue := model.Issue{ID: "bt-002", Title: "Wisp", Status: model.StatusOpen, Ephemeral: &boolTrue}
	nonWispExplicit := model.Issue{ID: "bt-003", Title: "Explicit Non-Wisp", Status: model.StatusOpen, Ephemeral: &boolFalse}

	t.Run("wisps hidden by default", func(t *testing.T) {
		showWisps := false
		issues := []model.Issue{normalIssue, wispIssue, nonWispExplicit}
		var visible []model.Issue
		for _, issue := range issues {
			if !showWisps && issue.Ephemeral != nil && *issue.Ephemeral {
				continue
			}
			visible = append(visible, issue)
		}
		if len(visible) != 2 {
			t.Errorf("expected 2 visible issues, got %d", len(visible))
		}
		for _, v := range visible {
			if v.ID == "bt-002" {
				t.Error("wisp issue should be hidden")
			}
		}
	})

	t.Run("wisps visible when toggled", func(t *testing.T) {
		showWisps := true
		issues := []model.Issue{normalIssue, wispIssue, nonWispExplicit}
		var visible []model.Issue
		for _, issue := range issues {
			if !showWisps && issue.Ephemeral != nil && *issue.Ephemeral {
				continue
			}
			visible = append(visible, issue)
		}
		if len(visible) != 3 {
			t.Errorf("expected 3 visible issues, got %d", len(visible))
		}
	})

	t.Run("non-ephemeral unaffected", func(t *testing.T) {
		showWisps := false
		issues := []model.Issue{normalIssue, nonWispExplicit}
		var visible []model.Issue
		for _, issue := range issues {
			if !showWisps && issue.Ephemeral != nil && *issue.Ephemeral {
				continue
			}
			visible = append(visible, issue)
		}
		if len(visible) != 2 {
			t.Errorf("expected 2 visible issues, got %d", len(visible))
		}
	})
}

// TestWispToggleDispatch_WorkspaceMode covers bt-8jds: in workspace/global
// mode, `w` is claimed by the project picker (ProjectsOrWisps), so wisp
// visibility must be reachable via a different key. Ctrl+W was chosen since
// `W` is already the live WorkspaceHomeAll ("home / all projects") binding
// in this exact mode - this test also regression-guards that collision.
func TestWispToggleDispatch_WorkspaceMode(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-001", Title: "Normal", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil, nil)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	m.workspaceMode = true
	m.availableRepos = []string{"bt", "sym"}

	t.Run("ctrl+w toggles wisps, no modal opens", func(t *testing.T) {
		if m.showWisps {
			t.Fatalf("precondition: showWisps should start false")
		}
		modelAny, _ := m.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
		got := modelAny.(Model)
		if !got.showWisps {
			t.Error("ctrl+w should enable wisps in workspace mode")
		}
		if got.activeModal == ModalRepoPicker {
			t.Error("ctrl+w must not open the project picker")
		}

		modelAny, _ = got.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
		got = modelAny.(Model)
		if got.showWisps {
			t.Error("second ctrl+w should disable wisps again")
		}
	})

	t.Run("plain w still opens the project picker, not wisp toggle", func(t *testing.T) {
		before := m.showWisps
		modelAny, _ := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
		got := modelAny.(Model)
		if got.activeModal != ModalRepoPicker {
			t.Errorf("plain 'w' in workspace mode should open the project picker, got activeModal=%v", got.activeModal)
		}
		if got.showWisps != before {
			t.Error("plain 'w' in workspace mode must not touch wisp visibility")
		}
	})

	t.Run("shift-W still does home/all projects toggle, not wisp toggle", func(t *testing.T) {
		m.currentProjectDB = "bt"
		before := m.showWisps
		modelAny, _ := m.Update(tea.KeyPressMsg{Code: 'W', Text: "W"})
		got := modelAny.(Model)
		if got.activeModal == ModalRepoPicker {
			t.Error("shift-W must not open the project picker")
		}
		if got.showWisps != before {
			t.Error("shift-W (WorkspaceHomeAll) must not touch wisp visibility")
		}
		if got.activeRepos == nil {
			t.Error("shift-W should have filtered activeRepos to the home project")
		}
	})
}

// TestWispToggleDispatch_SingleProjectMode covers the regression guard: the
// existing single-project `w` wisp toggle must be unaffected by adding
// ctrl+w, and ctrl+w works there too as a consistent alias.
func TestWispToggleDispatch_SingleProjectMode(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-001", Title: "Normal", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil, nil)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	// workspaceMode left false: single-project mode.

	// Model.Update has a value receiver, so state must be threaded through
	// the returned value explicitly rather than relying on outer-scope
	// mutation across subtests.
	t.Run("plain w still toggles wisps", func(t *testing.T) {
		modelAny, _ := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
		got := modelAny.(Model)
		if !got.showWisps {
			t.Error("'w' should still toggle wisps on in single-project mode")
		}
		m = got
	})

	t.Run("ctrl+w also toggles wisps", func(t *testing.T) {
		modelAny, _ := m.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
		got := modelAny.(Model)
		if got.showWisps {
			t.Error("ctrl+w should toggle wisps off after the prior 'w' toggled it on")
		}
	})
}
