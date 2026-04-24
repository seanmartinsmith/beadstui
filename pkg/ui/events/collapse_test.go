// pkg/ui/events/collapse_test.go
package events

import (
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func mkEventAt(beadID string, kind EventKind, at time.Time) Event {
	return Event{
		ID:     computeID(beadID, kind, at),
		Kind:   kind,
		BeadID: beadID,
		Repo:   model.ExtractRepoPrefix(beadID),
		At:     at,
		Source: SourceDolt,
	}
}

func TestCollapseForTicker_Empty(t *testing.T) {
	got := CollapseForTicker(nil, 30*time.Second)
	if len(got) != 0 {
		t.Fatalf("empty input -> empty output; got %d", len(got))
	}
}

func TestCollapseForTicker_SingleEvent(t *testing.T) {
	now := time.Now()
	in := []Event{mkEventAt("bt-1", EventEdited, now)}
	got := CollapseForTicker(in, 30*time.Second)
	if len(got) != 1 {
		t.Fatalf("single event -> single output; got %d", len(got))
	}
}

func TestCollapseForTicker_SameBeadSameKindWithinWindow(t *testing.T) {
	now := time.Now()
	in := []Event{
		mkEventAt("bt-1", EventEdited, now.Add(-20*time.Second)),
		mkEventAt("bt-1", EventEdited, now.Add(-10*time.Second)),
		mkEventAt("bt-1", EventEdited, now),
	}
	got := CollapseForTicker(in, 30*time.Second)
	if len(got) != 1 {
		t.Fatalf("3 same-bead same-kind events within window -> 1 output; got %d", len(got))
	}
	if !got[0].At.Equal(now) {
		t.Errorf("kept timestamp %v, want most recent %v", got[0].At, now)
	}
	if got[0].Summary != "+ 3 fields" {
		t.Errorf("collapsed Summary = %q, want aggregate phrasing", got[0].Summary)
	}
}

func TestCollapseForTicker_SameBeadSameKindOutsideWindow(t *testing.T) {
	now := time.Now()
	in := []Event{
		mkEventAt("bt-1", EventEdited, now.Add(-60*time.Second)),
		mkEventAt("bt-1", EventEdited, now),
	}
	got := CollapseForTicker(in, 30*time.Second)
	if len(got) != 2 {
		t.Fatalf("events outside window stay separate; got %d", len(got))
	}
}

func TestCollapseForTicker_DifferentKindsNeverCollapse(t *testing.T) {
	now := time.Now()
	in := []Event{
		mkEventAt("bt-1", EventEdited, now.Add(-5*time.Second)),
		mkEventAt("bt-1", EventCommented, now),
	}
	got := CollapseForTicker(in, 30*time.Second)
	if len(got) != 2 {
		t.Fatalf("different kinds on same bead stay separate; got %d", len(got))
	}
}

func TestCollapseForTicker_DifferentBeadsNeverCollapse(t *testing.T) {
	now := time.Now()
	in := []Event{
		mkEventAt("bt-1", EventEdited, now.Add(-5*time.Second)),
		mkEventAt("bt-2", EventEdited, now),
	}
	got := CollapseForTicker(in, 30*time.Second)
	if len(got) != 2 {
		t.Fatalf("different beads stay separate; got %d", len(got))
	}
}
