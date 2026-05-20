package correlation

// Dolt-native extractor introduced per bt-592c (Option C: detect + dispatch).
//
// Reads bead lifecycle events directly from upstream beads' `events` and
// `wisp_events` tables instead of from a JSONL file's git history.
// BeadEvent.CommitSHA is always empty for events sourced this way: upstream
// records no link from events to git commits, and 592c rejected heuristic
// correlation as too lossy.
//
// Mirrors the public Extract signature of the JSONL+git-diff Extractor in
// extractor.go so Correlator (bt-08sh.4) can dispatch to either
// implementation transparently.

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DoltExtractor extracts bead lifecycle events from a running Dolt SQL
// server, reading the events and wisp_events tables maintained by upstream
// beads.
type DoltExtractor struct {
	db *sql.DB
}

// NewDoltExtractor returns an extractor that reads from the given already-open
// Dolt SQL connection.
//
// The connection is borrowed, not owned: callers (typically the TUI's
// long-lived *datasource.DoltReader) retain responsibility for its lifecycle.
// This deliberately does not open a second Dolt connection - 592c calls for
// reusing the existing one.
func NewDoltExtractor(db *sql.DB) *DoltExtractor {
	return &DoltExtractor{db: db}
}

// Extract reads bead lifecycle events from the events and wisp_events tables,
// applying the filters in opts. Events are returned in chronological order
// (oldest first), matching the JSONL Extractor's contract.
func (e *DoltExtractor) Extract(opts ExtractOptions) ([]BeadEvent, error) {
	if e.db == nil {
		return nil, fmt.Errorf("dolt extractor: nil database handle")
	}

	query, args := buildEventsQuery(opts)

	rows, err := e.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []BeadEvent
	for rows.Next() {
		var (
			issueID, eventType                 string
			actor, oldValue, newValue, comment sql.NullString
			createdAt                          time.Time
		)
		if err := rows.Scan(&issueID, &eventType, &actor, &oldValue, &newValue, &comment, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning event row: %w", err)
		}

		author := actor.String
		var authorEmail string
		if strings.Contains(author, "@") {
			authorEmail = author
		}

		events = append(events, BeadEvent{
			BeadID:      issueID,
			EventType:   mapDoltEventType(eventType, oldValue.String, newValue.String),
			Timestamp:   createdAt,
			CommitSHA:   "", // intentional per 592c: no event-to-commit link upstream
			CommitMsg:   comment.String,
			Author:      author,
			AuthorEmail: authorEmail,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating event rows: %w", err)
	}

	return events, nil
}

// ExtractForBead is a convenience wrapper that filters Extract to a single
// bead ID. Mirrors Extractor.ExtractForBead.
func (e *DoltExtractor) ExtractForBead(beadID string, opts ExtractOptions) ([]BeadEvent, error) {
	opts.BeadID = beadID
	return e.Extract(opts)
}

// buildEventsQuery builds a UNION ALL over events and wisp_events with shared
// filters applied to each half. Mirrors upstream's GetAllEventsSinceInTx
// pattern in internal/storage/issueops/events.go.
//
// LIMIT is applied to the combined, ordered result so callers see the N
// oldest events overall when opts.Limit > 0.
func buildEventsQuery(opts ExtractOptions) (string, []any) {
	where, halfArgs := buildEventsWhere(opts)

	const base = "SELECT issue_id, event_type, actor, old_value, new_value, comment, created_at FROM "
	eventsHalf := base + "events" + where
	wispHalf := base + "wisp_events" + where

	query := "(" + eventsHalf + ") UNION ALL (" + wispHalf + ") ORDER BY created_at ASC"
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	args := make([]any, 0, len(halfArgs)*2)
	args = append(args, halfArgs...)
	args = append(args, halfArgs...)
	return query, args
}

// buildEventsWhere builds the WHERE clause and arg list shared by both halves
// of the events UNION wisp_events query.
func buildEventsWhere(opts ExtractOptions) (string, []any) {
	var clauses []string
	var args []any

	if opts.BeadID != "" {
		clauses = append(clauses, "issue_id = ?")
		args = append(args, opts.BeadID)
	}
	if opts.Since != nil {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, *opts.Since)
	}
	if opts.Until != nil {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, *opts.Until)
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// mapDoltEventType translates upstream's event_type (with old/new value when
// relevant) into bt's EventType taxonomy per bt-592c.
//
// Upstream event_type vocabulary: created, updated, status_changed, commented,
// closed, reopened, dependency_added, dependency_removed, label_added,
// label_removed, compacted.
//
// bt's EventType vocabulary: created, claimed, closed, reopened, modified.
//
// Mapping:
//   - created                                -> EventCreated
//   - status_changed (new=in_progress)       -> EventClaimed
//   - status_changed (new=closed)            -> EventClosed
//   - status_changed (new=open, old=closed)  -> EventReopened
//   - everything else                        -> EventModified
//
// The reopened branch matches determineStatusEvent in extractor.go: a
// transition into "open" only counts as reopened if it came from "closed";
// any other origin lands in EventModified.
//
// Standalone "closed" and "reopened" event_types from the upstream enum fall
// through to EventModified under this mapping. If upstream emits those as
// distinct events (not as status_changed), bt-08sh.5's fixture tests will
// surface the gap and the mapping can expand.
func mapDoltEventType(eventType, oldValue, newValue string) EventType {
	switch eventType {
	case "created":
		return EventCreated
	case "status_changed":
		switch newValue {
		case "in_progress":
			return EventClaimed
		case "closed":
			return EventClosed
		case "open":
			if oldValue == "closed" {
				return EventReopened
			}
			return EventModified
		default:
			return EventModified
		}
	default:
		return EventModified
	}
}
