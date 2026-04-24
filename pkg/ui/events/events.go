// pkg/ui/events/events.go
// Package events captures bead-activity events emitted by the Dolt snapshot
// diff pipeline. It holds the Event/EventKind/EventSource types, the
// RingBuffer, the Diff function, and the collapse helper.
package events

import (
	"fmt"
	"hash/fnv"
	"time"
)

// EventKind is the nature of a bead change captured as an event.
type EventKind int

const (
	EventCreated EventKind = iota
	EventEdited
	EventClosed
	EventCommented
	// EventBulk is synthesized when a single diff produces more than
	// bulkFloodThreshold underlying events (e.g., bd rename-prefix).
	// Represents "N beads changed (bulk operation)" as a single row.
	EventBulk
)

func (k EventKind) String() string {
	switch k {
	case EventCreated:
		return "created"
	case EventEdited:
		return "edited"
	case EventClosed:
		return "closed"
	case EventCommented:
		return "commented"
	case EventBulk:
		return "bulk"
	default:
		return fmt.Sprintf("EventKind(%d)", int(k))
	}
}

// EventSource is where an event originated. Only SourceDolt is emitted in v1;
// SourceCass is reserved for a future CASS live session stream integration.
type EventSource int

const (
	SourceDolt EventSource = iota
	SourceCass
)

func (s EventSource) String() string {
	switch s {
	case SourceDolt:
		return "dolt"
	case SourceCass:
		return "cass"
	default:
		return fmt.Sprintf("EventSource(%d)", int(s))
	}
}

// Event is one captured bead-activity record.
type Event struct {
	ID      string // stable hash of BeadID+Kind+At, for dedup/dismissal
	Kind    EventKind
	BeadID  string // "bt-1u3"
	Repo    string // prefix extracted from BeadID ("bt")
	Title   string // bead title at event time; frozen, not updated later
	Summary string // kind-dependent; see package doc
	Actor   string // Issue.Assignee if set, else ""
	At      time.Time
	// CommentAt is the CreatedAt of the comment that triggered this event.
	// Populated only for Kind == EventCommented; zero-value otherwise. Used by
	// the notifications-tab deep-link path (bt-46p6.16) to scroll the detail
	// viewport to the specific comment after jumping to the bead.
	CommentAt time.Time
	Source    EventSource
	Dismissed bool
}

// computeID derives a stable ID for an event from its BeadID, Kind, and
// emission time. Truncated fnv32 is enough for session-scoped dedup.
func computeID(beadID string, kind EventKind, at time.Time) string {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s|%d|%d", beadID, kind, at.UnixNano())
	return fmt.Sprintf("%08x", h.Sum32())
}

// repoFromBeadID returns the prefix portion of a bead ID (text before the
// first dash). Returns the full ID unchanged if no dash is present.
func repoFromBeadID(beadID string) string {
	for i, r := range beadID {
		if r == '-' {
			return beadID[:i]
		}
	}
	return beadID
}
