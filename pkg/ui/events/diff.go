// pkg/ui/events/diff.go
package events

import (
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// bulkFloodThreshold caps per-diff emission. When a single diff produces
// more than this many underlying events (e.g., bd rename-prefix on 3000
// beads), emit a single EventBulk marker instead. Prevents one migration
// from flushing all useful history out of the ring buffer.
const bulkFloodThreshold = 100

// Diff compares two snapshots of issues and returns the events that
// represent the change between them. `at` is the emission time stamped
// onto every event. `source` is the EventSource (SourceDolt for the
// live poll path).
//
// A nil `prior` is treated as "no prior state" — every issue in `next`
// becomes an EventCreated. Callers in the live poll path should NOT
// invoke Diff on the very first snapshot; use nil only when bootstrapping
// test fixtures.
//
// If the diff would produce more than bulkFloodThreshold events, the
// underlying events are discarded and a single EventBulk event is
// returned with Summary = "N beads changed (bulk operation)".
func Diff(prior, next []model.Issue, at time.Time, source EventSource) []Event {
	priorByID := indexByID(prior)
	nextByID := indexByID(next)

	var events []Event
	for id, newIssue := range nextByID {
		oldIssue, existed := priorByID[id]
		if !existed {
			events = append(events, newCreatedEvent(newIssue, at, source))
			continue
		}
		// Status transition from open-family -> closed is an explicit close.
		if !oldIssue.Status.IsClosed() && newIssue.Status.IsClosed() {
			events = append(events, newClosedEvent(newIssue, at, source))
			continue
		}
		// Commented: comment count increased.
		if len(newIssue.Comments) > len(oldIssue.Comments) {
			events = append(events, newCommentedEvent(oldIssue, newIssue, at, source))
			continue
		}
		// Edited: any field other than UpdatedAt changed.
		if summary, changed := editSummary(oldIssue, newIssue); changed {
			events = append(events, newEditedEvent(newIssue, summary, at, source))
		}
	}

	if len(events) > bulkFloodThreshold {
		return []Event{{
			ID:      computeID("<bulk>", EventBulk, at),
			Kind:    EventBulk,
			BeadID:  "",
			Repo:    "",
			Title:   "",
			Summary: bulkSummary(len(events)),
			At:      at,
			Source:  source,
		}}
	}
	return events
}

// indexByID returns a lookup map keyed by Issue.ID. Issues are shallow-
// copied into values so callers may freely mutate the snapshots later
// without corrupting the map (the snapshot layer does not modify issues
// in place, but defensive copy is cheap).
func indexByID(issues []model.Issue) map[string]model.Issue {
	out := make(map[string]model.Issue, len(issues))
	for i := range issues {
		out[issues[i].ID] = issues[i]
	}
	return out
}

func newCreatedEvent(issue model.Issue, at time.Time, source EventSource) Event {
	return Event{
		ID:      computeID(issue.ID, EventCreated, at),
		Kind:    EventCreated,
		BeadID:  issue.ID,
		Repo:    repoFromBeadID(issue.ID),
		Title:   issue.Title,
		Summary: issue.Title,
		Actor:   issue.Assignee,
		At:      at,
		Source:  source,
	}
}

func newClosedEvent(issue model.Issue, at time.Time, source EventSource) Event {
	return Event{
		ID:      computeID(issue.ID, EventClosed, at),
		Kind:    EventClosed,
		BeadID:  issue.ID,
		Repo:    repoFromBeadID(issue.ID),
		Title:   issue.Title,
		Summary: issue.Title,
		Actor:   issue.Assignee,
		At:      at,
		Source:  source,
	}
}

func newEditedEvent(issue model.Issue, summary string, at time.Time, source EventSource) Event {
	return Event{
		ID:      computeID(issue.ID, EventEdited, at),
		Kind:    EventEdited,
		BeadID:  issue.ID,
		Repo:    repoFromBeadID(issue.ID),
		Title:   issue.Title,
		Summary: summary,
		Actor:   issue.Assignee,
		At:      at,
		Source:  source,
	}
}

// editSummary and newCommentedEvent implemented in subsequent tasks.
// Stub placeholders live here only to keep the file compilable once
// the test suite for created/closed runs.
func editSummary(_ model.Issue, _ model.Issue) (string, bool) {
	return "", false
}

func newCommentedEvent(_, _ model.Issue, _ time.Time, _ EventSource) Event {
	return Event{}
}

func bulkSummary(n int) string {
	return "" // implemented in Task 6
}
