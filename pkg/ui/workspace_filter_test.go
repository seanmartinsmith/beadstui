package ui

import (
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"

	tea "github.com/charmbracelet/bubbletea"
)

func TestApplyFilterRespectsWorkspaceRepoFilter(t *testing.T) {
	issues := []model.Issue{
		{ID: "api-AUTH-1", Title: "API", Status: model.StatusOpen},
		{ID: "web-UI-1", Title: "Web", Status: model.StatusOpen},
	}

	m := NewModel(issues, nil, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"api-", "web-"},
	})

	// Filter to api only
	m.activeRepos = map[string]bool{"api": true}
	m.applyFilter()

	if got := len(m.list.Items()); got != 1 {
		t.Fatalf("expected 1 visible item after repo filter, got %d", got)
	}
	item, ok := m.list.Items()[0].(IssueItem)
	if !ok {
		t.Fatalf("expected IssueItem")
	}
	if item.Issue.ID != "api-AUTH-1" {
		t.Fatalf("expected api issue, got %s", item.Issue.ID)
	}

	// Clear repo filter (nil = all repos)
	m.activeRepos = nil
	m.applyFilter()
	if got := len(m.list.Items()); got != 2 {
		t.Fatalf("expected 2 visible items with no repo filter, got %d", got)
	}
}
