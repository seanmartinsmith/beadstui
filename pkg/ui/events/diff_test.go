// pkg/ui/events/diff_test.go
package events

import (
	"fmt"
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

func TestDiff_ReopenIsEdit(t *testing.T) {
	// Status transition closed -> open is an edit, not a special kind.
	prior := []model.Issue{mkIssue("bt-1", "Alpha", model.StatusClosed)}
	next := []model.Issue{mkIssue("bt-1", "Alpha", model.StatusOpen)}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 1 {
		t.Fatalf("Diff should emit 1 event, got %d", len(events))
	}
	if events[0].Kind != EventEdited {
		t.Errorf("Kind = %v, want EventEdited for reopen", events[0].Kind)
	}
}

func TestDiff_EditedSingleField(t *testing.T) {
	prior := []model.Issue{mkIssue("bt-1", "Alpha", model.StatusOpen)}
	next := []model.Issue{mkIssue("bt-1", "Alpha renamed", model.StatusOpen)}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 1 || events[0].Kind != EventEdited {
		t.Fatalf("want 1 EventEdited, got %v", events)
	}
	if events[0].Summary != "+ title" {
		t.Errorf("Summary = %q, want %q", events[0].Summary, "+ title")
	}
}

func TestDiff_EditedTwoFields(t *testing.T) {
	prior := []model.Issue{{ID: "bt-1", Title: "Alpha", Priority: 2, Status: model.StatusOpen}}
	next := []model.Issue{{ID: "bt-1", Title: "Alpha v2", Priority: 1, Status: model.StatusOpen}}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 1 || events[0].Kind != EventEdited {
		t.Fatalf("want 1 EventEdited, got %v", events)
	}
	if events[0].Summary != "+ title, + priority" {
		t.Errorf("Summary = %q, want %q", events[0].Summary, "+ title, + priority")
	}
}

func TestDiff_EditedThreeFields(t *testing.T) {
	prior := []model.Issue{{ID: "bt-1", Title: "A", Priority: 2, Assignee: "", Status: model.StatusOpen}}
	next := []model.Issue{{ID: "bt-1", Title: "A v2", Priority: 1, Assignee: "sms", Status: model.StatusOpen}}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	// Order follows editableFields declaration order; assert exactly.
	if events[0].Summary != "+ title, + priority, + assignee" {
		t.Errorf("Summary = %q, want exact field list for 3", events[0].Summary)
	}
}

func TestDiff_EditedFourPlusFieldsAggregates(t *testing.T) {
	prior := []model.Issue{{ID: "bt-1", Title: "A", Description: "old", Priority: 2, Assignee: "", Status: model.StatusOpen}}
	next := []model.Issue{{ID: "bt-1", Title: "B", Description: "new", Priority: 1, Assignee: "sms", Status: model.StatusOpen, Labels: []string{"area:tui"}}}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].Summary != "+ 5 fields" {
		t.Errorf("Summary = %q, want %q", events[0].Summary, "+ 5 fields")
	}
}

func TestDiff_EditedIgnoresUpdatedAt(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	prior := []model.Issue{{ID: "bt-1", Title: "A", Status: model.StatusOpen, UpdatedAt: t0}}
	next := []model.Issue{{ID: "bt-1", Title: "A", Status: model.StatusOpen, UpdatedAt: t1}}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 0 {
		t.Fatalf("UpdatedAt-only change should not emit an event, got %d", len(events))
	}
}

func TestDiff_NewComment(t *testing.T) {
	prior := []model.Issue{{ID: "bt-1", Title: "A", Status: model.StatusOpen}}
	next := []model.Issue{{
		ID: "bt-1", Title: "A", Status: model.StatusOpen,
		Comments: []*model.Comment{{ID: "c1", Text: "Index rebuild finished", Author: "sms"}},
	}}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 1 || events[0].Kind != EventCommented {
		t.Fatalf("want 1 EventCommented, got %v", events)
	}
	if events[0].Summary != "Index rebuild finished" {
		t.Errorf("Summary = %q, want comment text", events[0].Summary)
	}
}

func TestDiff_CommentTextTruncatedAt80(t *testing.T) {
	long := "This is a very long comment that exceeds the 80-character summary truncation threshold set by the spec for ticker readability."
	prior := []model.Issue{{ID: "bt-1", Title: "A", Status: model.StatusOpen}}
	next := []model.Issue{{
		ID: "bt-1", Title: "A", Status: model.StatusOpen,
		Comments: []*model.Comment{{ID: "c1", Text: long, Author: "sms"}},
	}}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 1 || events[0].Kind != EventCommented {
		t.Fatalf("want 1 EventCommented, got %v", events)
	}
	if len(events[0].Summary) > 80 {
		t.Errorf("Summary len = %d, want <= 80", len(events[0].Summary))
	}
	runes := []rune(events[0].Summary)
	if runes[len(runes)-1] != '…' && !hasEllipsisSuffix(events[0].Summary) {
		// Any suffix indicating truncation is fine; assert it is not the full text.
		if events[0].Summary == long {
			t.Errorf("Summary was not truncated: %q", events[0].Summary)
		}
	}
}

func hasEllipsisSuffix(s string) bool {
	return len(s) >= 3 && s[len(s)-3:] == "..."
}

func TestDiff_MultipleNewComments_UsesLatest(t *testing.T) {
	// Two comments added since last poll — Summary should reflect the
	// most recently added one (last element of Comments).
	prior := []model.Issue{{ID: "bt-1", Title: "A", Status: model.StatusOpen}}
	next := []model.Issue{{
		ID: "bt-1", Title: "A", Status: model.StatusOpen,
		Comments: []*model.Comment{
			{ID: "c1", Text: "first", Author: "sms"},
			{ID: "c2", Text: "second", Author: "sms"},
		},
	}}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 1 || events[0].Kind != EventCommented {
		t.Fatalf("want 1 EventCommented, got %v", events)
	}
	if events[0].Summary != "second" {
		t.Errorf("Summary = %q, want %q (latest comment)", events[0].Summary, "second")
	}
}

func TestDiff_BelowBulkThresholdEmitsIndividual(t *testing.T) {
	// 50 new beads — below the 100-event threshold. All emit as individual EventCreated.
	var prior, next []model.Issue
	for i := 0; i < 50; i++ {
		next = append(next, mkIssue(
			fmt.Sprintf("bt-%d", i),
			fmt.Sprintf("Bead %d", i),
			model.StatusOpen,
		))
	}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 50 {
		t.Fatalf("50 new beads should emit 50 events, got %d", len(events))
	}
	for _, e := range events {
		if e.Kind != EventCreated {
			t.Errorf("unexpected kind %v, want EventCreated", e.Kind)
		}
	}
}

func TestDiff_AboveBulkThresholdEmitsBulkMarker(t *testing.T) {
	// 150 new beads — above the 100-event threshold. Collapses to a single EventBulk.
	var prior, next []model.Issue
	for i := 0; i < 150; i++ {
		next = append(next, mkIssue(
			fmt.Sprintf("bt-%d", i),
			fmt.Sprintf("Bead %d", i),
			model.StatusOpen,
		))
	}
	events := Diff(prior, next, time.Now(), SourceDolt)
	if len(events) != 1 {
		t.Fatalf("150 new beads above threshold should emit 1 EventBulk, got %d events", len(events))
	}
	if events[0].Kind != EventBulk {
		t.Errorf("Kind = %v, want EventBulk", events[0].Kind)
	}
	if events[0].Summary != "150 beads changed (bulk operation)" {
		t.Errorf("Summary = %q, want exact bulk phrasing", events[0].Summary)
	}
}
