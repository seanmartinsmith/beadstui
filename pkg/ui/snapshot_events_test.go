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
