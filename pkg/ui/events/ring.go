// pkg/ui/events/ring.go
package events

import (
	"sync"

	"github.com/seanmartinsmith/beadstui/pkg/debug"
)

// RingBuffer is a session-scoped fixed-capacity store for Events. It is
// safe for concurrent use: writes are serialized, reads return a copy.
// Capacity evictions drop the oldest event when Append would exceed cap.
//
// When SetPersistPath has been called with a non-empty path, every
// Append/AppendMany also writes the new events as JSONL to that file
// (bt-6ool Part A). Hydrate replays a previously persisted slice into
// the buffer at boot.
type RingBuffer struct {
	mu        sync.RWMutex
	events    []Event
	cap       int
	persister *filePersister
}

// NewRingBuffer returns a RingBuffer with the given maximum capacity.
// A capacity <= 0 is treated as 1 to avoid divide-by-zero/empty-store edge
// cases; callers should pass a sensible value (see DefaultCapacity).
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &RingBuffer{
		events: make([]Event, 0, capacity),
		cap:    capacity,
	}
}

// DefaultCapacity is the session-scoped event retention used by callers
// that do not have a specific capacity requirement. Matches the spec.
const DefaultCapacity = 500

// SetPersistPath enables write-through JSONL persistence at the given
// file path. Pass "" to disable. The directory is created on first
// write. Hydrate is the companion read-side; call it BEFORE the live
// pipeline starts emitting events to avoid double-counting.
func (r *RingBuffer) SetPersistPath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if path == "" {
		r.persister = nil
		return
	}
	r.persister = &filePersister{path: path}
}

// Hydrate inserts pre-loaded events into the buffer in order, respecting
// capacity (oldest dropped when over). Intended to be called once at
// boot from a LoadPersisted result. Does NOT trigger persistence
// write-through — these events are already on disk.
func (r *RingBuffer) Hydrate(events []Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range events {
		if len(r.events) >= r.cap {
			r.events = r.events[1:]
		}
		r.events = append(r.events, e)
	}
}

// Append adds an event to the buffer. If the buffer is at capacity, the
// oldest event is evicted to make room. When persistence is configured,
// the event is also written through to disk; write failures are logged
// via pkg/debug and do not propagate (the in-memory ring stays the
// source of truth for the live session).
func (r *RingBuffer) Append(e Event) {
	r.mu.Lock()
	persister := r.persister
	if len(r.events) >= r.cap {
		r.events = r.events[1:]
	}
	r.events = append(r.events, e)
	r.mu.Unlock()

	if persister != nil {
		if err := persister.appendOne(e); err != nil {
			debug.Log("events.RingBuffer.Append persist failed: %v", err)
		}
	}
}

// AppendMany is a convenience for bulk-appending a slice of events.
// Evictions are applied per-event, so a large slice can flush older
// state. Persistence is batched: one disk write covers the whole slice.
func (r *RingBuffer) AppendMany(events []Event) {
	r.mu.Lock()
	persister := r.persister
	for _, e := range events {
		if len(r.events) >= r.cap {
			r.events = r.events[1:]
		}
		r.events = append(r.events, e)
	}
	r.mu.Unlock()

	if persister != nil && len(events) > 0 {
		if err := persister.appendMany(events); err != nil {
			debug.Log("events.RingBuffer.AppendMany persist failed: %v", err)
		}
	}
}

// Snapshot returns a copy of the current events slice, oldest-first.
// Callers may freely mutate the returned slice; it does not alias the
// internal buffer.
func (r *RingBuffer) Snapshot() []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

// UnreadCount returns the number of non-dismissed events currently in
// the buffer. This is what the footer count badge renders.
func (r *RingBuffer) UnreadCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for i := range r.events {
		if !r.events[i].Dismissed {
			n++
		}
	}
	return n
}

// Dismiss marks the event with the given ID as dismissed. No-op if the
// ID is not found. Idempotent — re-dismissing a dismissed event is safe.
func (r *RingBuffer) Dismiss(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.events {
		if r.events[i].ID == id {
			r.events[i].Dismissed = true
			return
		}
	}
}

// DismissAll marks every event currently in the buffer as dismissed.
// The events themselves remain — retention is unchanged.
func (r *RingBuffer) DismissAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.events {
		r.events[i].Dismissed = true
	}
}

// Len returns the number of events currently in the buffer (dismissed
// or not). Useful for debugging and diagnostics.
func (r *RingBuffer) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.events)
}
