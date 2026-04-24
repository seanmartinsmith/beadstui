package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// mkSnapshot builds a minimal DataSnapshot sufficient for the
// handleSnapshotReady path to swap pointers and run the diff.
func mkSnapshot(issues []model.Issue) *DataSnapshot {
	return &DataSnapshot{
		Issues:      issues,
		CreatedAt:   time.Now(),
		Phase2Ready: true,
	}
}

func TestHandleSnapshotReady_EmitsCreateEvent(t *testing.T) {
	// Bootstrap a Model with a prior snapshot and feed a new snapshot
	// that adds one bead. Verify the ring buffer captures a create event.
	initial := []model.Issue{{ID: "bt-1", Title: "alpha", Status: model.StatusOpen}}
	m := NewModel(initial, nil, "", nil)
	m.data.snapshot = mkSnapshot(initial)

	next := []model.Issue{
		{ID: "bt-1", Title: "alpha", Status: model.StatusOpen},
		{ID: "portal-9", Title: "beta", Status: model.StatusOpen},
	}
	msg := SnapshotReadyMsg{Snapshot: mkSnapshot(next)}
	modelAny, _ := m.Update(msg)
	m2 := modelAny.(Model)

	got := m2.events.Snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 event emitted, got %d", len(got))
	}
	if got[0].Kind != events.EventCreated || got[0].BeadID != "portal-9" {
		t.Errorf("unexpected event: %+v", got[0])
	}
}

func TestHandleSnapshotReady_SkipsInTimeTravel(t *testing.T) {
	initial := []model.Issue{{ID: "bt-1", Title: "alpha", Status: model.StatusOpen}}
	m := NewModel(initial, nil, "", nil)
	m.data.snapshot = mkSnapshot(initial)
	m.timeTravelMode = true // active time-travel must suppress emission

	next := []model.Issue{
		{ID: "bt-1", Title: "alpha", Status: model.StatusOpen},
		{ID: "portal-9", Title: "beta", Status: model.StatusOpen},
	}
	msg := SnapshotReadyMsg{Snapshot: mkSnapshot(next)}
	modelAny, _ := m.Update(msg)
	m2 := modelAny.(Model)

	if n := len(m2.events.Snapshot()); n != 0 {
		t.Fatalf("time-travel must not emit events, got %d", n)
	}
}

func TestHandleSnapshotReady_SkipsOnBootstrap(t *testing.T) {
	// First snapshot with a nil prior must not emit creates for every
	// existing bead — that would flood the ring on startup.
	m := NewModel(nil, nil, "", nil)
	m.data.snapshot = nil // bootstrap path
	m.data.snapshotInitPending = true

	next := []model.Issue{
		{ID: "bt-1", Title: "alpha", Status: model.StatusOpen},
		{ID: "bt-2", Title: "beta", Status: model.StatusOpen},
	}
	msg := SnapshotReadyMsg{Snapshot: mkSnapshot(next)}
	modelAny, _ := m.Update(msg)
	m2 := modelAny.(Model)

	if n := len(m2.events.Snapshot()); n != 0 {
		t.Fatalf("bootstrap snapshot must not emit events, got %d", n)
	}
}

// Silence unused-import warning on tea during tests that do not inspect cmds.
var _ = tea.Batch

// TestHandleSnapshotReady_WorkspaceFilterHonorsSourceRepo guards bt-ci7b:
// when the workspace activeRepos filter is keyed by the workspace DB name
// (e.g. "marketplace") but the issue's bead-ID prefix is something else
// (e.g. "mkt-xxx"), the handleSnapshotReady filter loop must consult
// IssueRepoKey (which honors issue.SourceRepo) instead of ID-derived
// item.RepoPrefix. Pre-fix the list went empty on every refresh; post-fix
// the issue stays visible.
func TestHandleSnapshotReady_WorkspaceFilterHonorsSourceRepo(t *testing.T) {
	// Initial snapshot establishes prior state so we hit the refresh path,
	// not the bootstrap branch.
	prior := []model.Issue{{
		ID: "mkt-foo", Title: "alpha", Status: model.StatusOpen,
		SourceRepo: "marketplace",
	}}
	m := NewModel(prior, nil, "", nil)
	m.workspaceMode = true
	m.activeRepos = map[string]bool{"marketplace": true}

	priorSnap := mkSnapshot(prior)
	priorSnap.ListItems = buildListItems(prior, nil)
	m.data.snapshot = priorSnap

	// Refresh: same single issue, but rebuilt as a fresh snapshot. Pre-fix
	// the filter loop computed strings.ToLower(item.RepoPrefix) = "mkt"
	// and missed the activeRepos["marketplace"] entry, dropping the item.
	next := []model.Issue{{
		ID: "mkt-foo", Title: "alpha", Status: model.StatusOpen,
		SourceRepo: "marketplace",
	}}
	nextSnap := mkSnapshot(next)
	nextSnap.ListItems = buildListItems(next, nil)
	msg := SnapshotReadyMsg{Snapshot: nextSnap}

	modelAny, _ := m.Update(msg)
	m2 := modelAny.(Model)

	visible := m2.list.VisibleItems()
	if len(visible) != 1 {
		t.Fatalf("workspace filter should keep the marketplace bead visible after refresh; got %d items", len(visible))
	}
	if item, ok := visible[0].(IssueItem); !ok || item.Issue.ID != "mkt-foo" {
		t.Errorf("unexpected visible item: %+v", visible[0])
	}
}

// TestHandleSnapshotReady_WorkspaceFilterAlsoRespectsIDPrefix is the
// degenerate case: when SourceRepo is empty and the workspace activeRepos
// key matches the ID prefix directly, behavior must stay correct. Prevents
// the fix from regressing the simple case.
func TestHandleSnapshotReady_WorkspaceFilterAlsoRespectsIDPrefix(t *testing.T) {
	prior := []model.Issue{{
		ID: "bt-foo", Title: "alpha", Status: model.StatusOpen,
		// SourceRepo deliberately empty — exercises the ID-prefix fallback.
	}}
	m := NewModel(prior, nil, "", nil)
	m.workspaceMode = true
	m.activeRepos = map[string]bool{"bt": true}

	priorSnap := mkSnapshot(prior)
	priorSnap.ListItems = buildListItems(prior, nil)
	m.data.snapshot = priorSnap

	next := []model.Issue{{
		ID: "bt-foo", Title: "alpha", Status: model.StatusOpen,
	}}
	nextSnap := mkSnapshot(next)
	nextSnap.ListItems = buildListItems(next, nil)
	modelAny, _ := m.Update(SnapshotReadyMsg{Snapshot: nextSnap})
	m2 := modelAny.(Model)

	if got := len(m2.list.VisibleItems()); got != 1 {
		t.Fatalf("ID-prefix-only filter should still match; got %d items", got)
	}
}
