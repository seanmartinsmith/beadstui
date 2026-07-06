package ui

import (
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"

	tea "charm.land/bubbletea/v2"
)

func TestApplyFilterRespectsWorkspaceRepoFilter(t *testing.T) {
	issues := []model.Issue{
		{ID: "api-AUTH-1", Title: "API", Status: model.StatusOpen},
		{ID: "web-UI-1", Title: "Web", Status: model.StatusOpen},
	}

	m := NewModel(issues, nil, "", nil, nil)
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

// TestTreeViewRespectsActiveRepos asserts the tree view rebuilds from the
// activeRepos-filtered slice, matching the dogfood repro on bt-dcby.2:
// before the fix the tree showed all beads regardless of project filter.
func TestTreeViewRespectsActiveRepos(t *testing.T) {
	issues := []model.Issue{
		{ID: "api-AUTH-1", Title: "API auth", Status: model.StatusOpen},
		{ID: "api-AUTH-2", Title: "API token", Status: model.StatusOpen},
		{ID: "web-UI-1", Title: "Web UI", Status: model.StatusOpen},
	}

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	m.EnableWorkspaceMode(WorkspaceInfo{
		Enabled:      true,
		RepoCount:    2,
		RepoPrefixes: []string{"api-", "web-"},
	})

	// Enter tree mode so the helper actually rebuilds.
	m.mode = ViewTree
	m.activeRepos = map[string]bool{"api": true}
	m.rebuildTreeForCurrentFilter()

	if got := m.tree.NodeCount(); got != 2 {
		t.Fatalf("expected 2 visible tree nodes for api filter, got %d", got)
	}
	for i := 0; i < m.tree.NodeCount(); i++ {
		node := m.tree.flatList[i]
		if node == nil || node.Issue == nil {
			t.Fatalf("nil node/issue at index %d", i)
		}
		if got := IssueRepoKey(*node.Issue); got != "api" {
			t.Fatalf("non-api bead leaked into tree: id=%s repo=%s", node.Issue.ID, got)
		}
	}

	// Toggling activeRepos to nil should restore the full set on the next
	// applyFilter. This covers the user flow: open tree, toggle project filter
	// off via shortcut, expect tree to rebuild without re-pressing E.
	m.activeRepos = nil
	m.applyFilter()
	if got := m.tree.NodeCount(); got != 3 {
		t.Fatalf("expected 3 visible tree nodes after clearing activeRepos, got %d", got)
	}

	// Tightening to the web project should narrow the tree, again live.
	m.activeRepos = map[string]bool{"web": true}
	m.applyFilter()
	if got := m.tree.NodeCount(); got != 1 {
		t.Fatalf("expected 1 visible tree node after web filter, got %d", got)
	}
	if id := m.tree.flatList[0].Issue.ID; id != "web-UI-1" {
		t.Fatalf("expected web-UI-1, got %s", id)
	}
}

// TestRebuildTreeForCurrentFilterIsNoOpOutsideTreeMode asserts that the
// helper does not pay the build cost (or stomp tree state) when the active
// view is not the tree. Non-tree filter changes should not touch the tree.
func TestRebuildTreeForCurrentFilterIsNoOpOutsideTreeMode(t *testing.T) {
	issues := []model.Issue{
		{ID: "api-AUTH-1", Title: "API auth", Status: model.StatusOpen},
	}

	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)
	// mode defaults to ViewList; tree was never built.
	if m.tree.IsBuilt() {
		t.Fatalf("tree should not be built before any rebuild call")
	}

	m.rebuildTreeForCurrentFilter()
	if m.tree.IsBuilt() {
		t.Errorf("tree should remain unbuilt outside ViewTree mode")
	}
}
