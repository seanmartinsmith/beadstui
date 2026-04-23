// pkg/ui/events/ring_test.go
package events

import (
	"testing"
	"time"
)

func mkEvent(id string, kind EventKind) Event {
	at := time.Now()
	return Event{
		ID:     computeID(id, kind, at),
		Kind:   kind,
		BeadID: id,
		Repo:   repoFromBeadID(id),
		At:     at,
		Source: SourceDolt,
	}
}

func TestRingBuffer_AppendAndSnapshot(t *testing.T) {
	r := NewRingBuffer(10)
	r.Append(mkEvent("bt-1", EventCreated))
	r.Append(mkEvent("bt-2", EventEdited))
	got := r.Snapshot()
	if len(got) != 2 {
		t.Fatalf("Snapshot len = %d, want 2", len(got))
	}
	if got[0].BeadID != "bt-1" || got[1].BeadID != "bt-2" {
		t.Errorf("Snapshot order wrong: %v", got)
	}
}

func TestRingBuffer_CapacityEviction(t *testing.T) {
	r := NewRingBuffer(3)
	for i := 0; i < 5; i++ {
		r.Append(mkEvent("bt-x", EventCreated))
	}
	got := r.Snapshot()
	if len(got) != 3 {
		t.Fatalf("Snapshot len after overflow = %d, want 3 (cap)", len(got))
	}
}

func TestRingBuffer_UnreadCount(t *testing.T) {
	r := NewRingBuffer(10)
	r.Append(mkEvent("bt-1", EventCreated))
	r.Append(mkEvent("bt-2", EventEdited))
	r.Append(mkEvent("bt-3", EventClosed))
	if n := r.UnreadCount(); n != 3 {
		t.Fatalf("UnreadCount = %d, want 3", n)
	}
	// Dismiss one
	snap := r.Snapshot()
	r.Dismiss(snap[1].ID)
	if n := r.UnreadCount(); n != 2 {
		t.Fatalf("UnreadCount after dismiss = %d, want 2", n)
	}
}

func TestRingBuffer_DismissIdempotent(t *testing.T) {
	r := NewRingBuffer(10)
	r.Append(mkEvent("bt-1", EventCreated))
	id := r.Snapshot()[0].ID
	r.Dismiss(id)
	r.Dismiss(id) // second dismiss should be no-op
	if n := r.UnreadCount(); n != 0 {
		t.Fatalf("UnreadCount after double-dismiss = %d, want 0", n)
	}
}

func TestRingBuffer_DismissUnknownID(t *testing.T) {
	r := NewRingBuffer(10)
	r.Append(mkEvent("bt-1", EventCreated))
	r.Dismiss("nonexistent") // must not panic or change state
	if n := r.UnreadCount(); n != 1 {
		t.Fatalf("UnreadCount after dismiss of unknown id = %d, want 1", n)
	}
}

func TestRingBuffer_DismissAll(t *testing.T) {
	r := NewRingBuffer(10)
	for i := 0; i < 4; i++ {
		r.Append(mkEvent("bt-x", EventCreated))
	}
	r.DismissAll()
	if n := r.UnreadCount(); n != 0 {
		t.Fatalf("UnreadCount after DismissAll = %d, want 0", n)
	}
	if len(r.Snapshot()) != 4 {
		t.Fatalf("events should still exist after DismissAll (retention)")
	}
}
