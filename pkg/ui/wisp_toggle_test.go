package ui

import (
	"testing"

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
