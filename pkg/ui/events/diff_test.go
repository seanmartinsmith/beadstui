// pkg/ui/events/diff_test.go
package events

import (
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func mkIssue(id, title string, status model.Status) model.Issue {
	return model.Issue{
		ID:        id,
		Title:     title,
		Status:    status,
		UpdatedAt: time.Now(),
	}
}

func TestDiff_EmptyPrior(t *testing.T) {
	// First snapshot: everything is "created" since prior is empty.
	// But per spec, we only emit creates when prior snapshot was non-nil,
	// so a bootstrapping caller should not call Diff with a nil prior.
	// Still, calling with an empty slice must not panic.
	next := []model.Issue{mkIssue("bt-1", "Alpha", model.StatusOpen)}
	events := Diff(nil, next, time.Now(), SourceDolt)
	if len(events) != 1 || events[0].Kind != EventCreated {
		t.Fatalf("Diff(empty, [1 new]) should emit 1 EventCreated, got %v", events)
	}
}

func TestDiff_Created(t *testing.T) {
	prior := []model.Issue{mkIssue("bt-1", "Alpha", model.StatusOpen)}
	next := []model.Issue{
		mkIssue("bt-1", "Alpha", model.StatusOpen),
		mkIssue("portal-9", "Beta", model.StatusOpen),
	}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 1 {
		t.Fatalf("Diff should emit 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Kind != EventCreated {
		t.Errorf("Kind = %v, want EventCreated", e.Kind)
	}
	if e.BeadID != "portal-9" {
		t.Errorf("BeadID = %q, want portal-9", e.BeadID)
	}
	if e.Repo != "portal" {
		t.Errorf("Repo = %q, want portal", e.Repo)
	}
	if e.Title != "Beta" {
		t.Errorf("Title = %q, want Beta", e.Title)
	}
	if e.Summary != "Beta" {
		t.Errorf("Summary = %q, want Beta (title for created)", e.Summary)
	}
}

func TestDiff_Closed(t *testing.T) {
	prior := []model.Issue{mkIssue("bt-1", "Alpha", model.StatusOpen)}
	next := []model.Issue{mkIssue("bt-1", "Alpha", model.StatusClosed)}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 1 {
		t.Fatalf("Diff should emit 1 event, got %d", len(events))
	}
	if events[0].Kind != EventClosed {
		t.Errorf("Kind = %v, want EventClosed", events[0].Kind)
	}
	if events[0].BeadID != "bt-1" {
		t.Errorf("BeadID = %q, want bt-1", events[0].BeadID)
	}
}

func TestDiff_NoChange(t *testing.T) {
	issue := mkIssue("bt-1", "Alpha", model.StatusOpen)
	events := Diff([]model.Issue{issue}, []model.Issue{issue}, time.Now(), SourceDolt)
	if len(events) != 0 {
		t.Fatalf("Diff with no changes should emit 0 events, got %d", len(events))
	}
}
