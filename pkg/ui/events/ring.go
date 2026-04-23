// pkg/ui/events/ring.go
package events

import "sync"

// RingBuffer is a session-scoped fixed-capacity store for Events. It is
// safe for concurrent use: writes are serialized, reads return a copy.
// Capacity evictions drop the oldest event when Append would exceed cap.
type RingBuffer struct {
	mu     sync.RWMutex
	events []Event
	cap    int
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

// Append adds an event to the buffer. If the buffer is at capacity, the
// oldest event is evicted to make room.
func (r *RingBuffer) Append(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) >= r.cap {
		r.events = r.events[1:]
	}
	r.events = append(r.events, e)
}

// AppendMany is a convenience for bulk-appending a slice of events.
// Evictions are applied per-event, so a large slice can flush older state.
func (r *RingBuffer) AppendMany(events []Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range events {
		if len(r.events) >= r.cap {
			r.events = r.events[1:]
		}
		r.events = append(r.events, e)
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
