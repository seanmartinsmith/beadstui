// pkg/ui/events/diff.go
package events

import (
	"fmt"
	"strings"
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

// fieldCheck pairs a human-readable field name with a comparator.
// Order matters: Summary lists names in declaration order.
type fieldCheck struct {
	name    string
	changed func(a, b model.Issue) bool
}

// editableFields enumerates the fields whose mutation produces an
// EventEdited. UpdatedAt is intentionally excluded (it mutates on every
// write and would drown legitimate edits). Add new fields here as the
// Issue model grows.
var editableFields = []fieldCheck{
	{"title", func(a, b model.Issue) bool { return a.Title != b.Title }},
	{"priority", func(a, b model.Issue) bool { return a.Priority != b.Priority }},
	{"assignee", func(a, b model.Issue) bool { return a.Assignee != b.Assignee }},
	{"status", func(a, b model.Issue) bool { return a.Status != b.Status }},
	{"description", func(a, b model.Issue) bool { return a.Description != b.Description }},
	{"design", func(a, b model.Issue) bool { return a.Design != b.Design }},
	{"acceptance", func(a, b model.Issue) bool { return a.AcceptanceCriteria != b.AcceptanceCriteria }},
	{"notes", func(a, b model.Issue) bool { return a.Notes != b.Notes }},
	{"type", func(a, b model.Issue) bool { return a.IssueType != b.IssueType }},
	{"labels", func(a, b model.Issue) bool { return !stringSliceEqual(a.Labels, b.Labels) }},
	{"due_date", func(a, b model.Issue) bool { return !timePtrEqual(a.DueDate, b.DueDate) }},
	{"close_reason", func(a, b model.Issue) bool { return !stringPtrEqual(a.CloseReason, b.CloseReason) }},
	{"external_ref", func(a, b model.Issue) bool { return !stringPtrEqual(a.ExternalRef, b.ExternalRef) }},
}

// editSummary returns a human-readable summary of the fields that
// changed between old and new, plus a bool indicating whether any
// change was detected. When <=3 fields changed, the summary names
// them in declaration order; when >3, it aggregates to "+ N fields".
func editSummary(old model.Issue, new model.Issue) (string, bool) {
	var changed []string
	for _, f := range editableFields {
		if f.changed(old, new) {
			changed = append(changed, f.name)
		}
	}
	if len(changed) == 0 {
		return "", false
	}
	if len(changed) > 3 {
		return fmt.Sprintf("+ %d fields", len(changed)), true
	}
	parts := make([]string, len(changed))
	for i, n := range changed {
		parts[i] = "+ " + n
	}
	return strings.Join(parts, ", "), true
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func timePtrEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// commentSummaryLimit is the maximum rune length of a comment-derived
// Summary string. Longer comments are truncated with a trailing "...".
const commentSummaryLimit = 80

// newCommentedEvent builds an EventCommented from the latest new comment
// on the bead. oldIssue is used to determine which comments are new; the
// summary derives from the most recently added one.
func newCommentedEvent(oldIssue, newIssue model.Issue, at time.Time, source EventSource) Event {
	summary := ""
	var commentAt time.Time
	if len(newIssue.Comments) > 0 {
		latest := newIssue.Comments[len(newIssue.Comments)-1]
		if latest != nil {
			summary = truncateForSummary(latest.Text, commentSummaryLimit)
			commentAt = latest.CreatedAt
		}
	}
	return Event{
		ID:        computeID(newIssue.ID, EventCommented, at),
		Kind:      EventCommented,
		BeadID:    newIssue.ID,
		Repo:      repoFromBeadID(newIssue.ID),
		Title:     newIssue.Title,
		Summary:   summary,
		Actor:     newIssue.Assignee,
		At:        at,
		CommentAt: commentAt,
		Source:    source,
	}
}

// truncateForSummary shortens s to at most n runes, appending "..."
// when truncation occurs. Returns s unchanged if already within n.
func truncateForSummary(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 3 {
		return string(runes[:n])
	}
	return string(runes[:n-3]) + "..."
}

// bulkSummary renders the Summary string for an EventBulk marker that
// collapses many underlying events emitted by one diff.
func bulkSummary(n int) string {
	return fmt.Sprintf("%d beads changed (bulk operation)", n)
}
