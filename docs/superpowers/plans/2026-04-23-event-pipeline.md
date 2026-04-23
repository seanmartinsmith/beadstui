# Event Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the event capture pipeline that diffs Dolt snapshots and produces structured `Event` records in a session-scoped ring buffer. This is the data layer for the notification center and footer ticker. No UI changes in this plan.

**Architecture:** New `pkg/ui/events/` package owns the types, ring buffer, diff function, and collapse helper. `Model` holds a `*events.RingBuffer`. The existing `handleSnapshotReady` path in `pkg/ui/model_update_data.go` captures the prior snapshot (already does at `oldSnapshot := m.data.snapshot`) and invokes `events.Diff(oldIssues, newIssues)` on every swap, skipping when time-travel mode is active or the prior snapshot was nil. Emissions are capped at 100 events per diff to prevent bulk floods from flushing history.

**Tech Stack:** Go 1.25+, `sync.RWMutex` for ring concurrency, `fnv` std hash for event IDs, no external deps.

**Scope exclusions (per bt-d5wr spec):** no UI changes (no footer, no modal, no keybindings). Actor filtering, CASS source emission, and cross-session persistence are deferred. This plan produces a tested data layer that later beads consume.

---

### Task 1: Scaffold `events` package with core types

**Files:**
- Create: `pkg/ui/events/events.go`
- Create: `pkg/ui/events/events_test.go`

- [ ] **Step 1.1: Write the failing test for EventKind String()**

```go
// pkg/ui/events/events_test.go
package events

import "testing"

func TestEventKindString(t *testing.T) {
	cases := []struct {
		kind EventKind
		want string
	}{
		{EventCreated, "created"},
		{EventEdited, "edited"},
		{EventClosed, "closed"},
		{EventCommented, "commented"},
	}
	for _, c := range cases {
		if got := c.kind.String(); got != c.want {
			t.Errorf("EventKind(%d).String() = %q, want %q", c.kind, got, c.want)
		}
	}
}

func TestEventSourceString(t *testing.T) {
	if SourceDolt.String() != "dolt" {
		t.Errorf("SourceDolt.String() = %q, want %q", SourceDolt.String(), "dolt")
	}
	if SourceCass.String() != "cass" {
		t.Errorf("SourceCass.String() = %q, want %q", SourceCass.String(), "cass")
	}
}
```

- [ ] **Step 1.2: Run the test and verify it fails**

Run: `go test ./pkg/ui/events/ -run TestEventKindString -v`
Expected: compile error — package does not exist yet.

- [ ] **Step 1.3: Create the events.go file with types**

```go
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
	ID        string      // stable hash of BeadID+Kind+At, for dedup/dismissal
	Kind      EventKind
	BeadID    string      // "bt-1u3"
	Repo      string      // prefix extracted from BeadID ("bt")
	Title     string      // bead title at event time; frozen, not updated later
	Summary   string      // kind-dependent; see package doc
	Actor     string      // Issue.Assignee if set, else ""
	At        time.Time
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
```

- [ ] **Step 1.4: Run the test and verify it passes**

Run: `go test ./pkg/ui/events/ -run TestEventKindString -v`
Expected: PASS on both TestEventKindString and TestEventSourceString.

- [ ] **Step 1.5: Commit**

```bash
git add pkg/ui/events/events.go pkg/ui/events/events_test.go
git commit -m "feat(events): scaffold events package with Event/EventKind/EventSource types"
```

---

### Task 2: Ring buffer data structure

**Files:**
- Create: `pkg/ui/events/ring.go`
- Create: `pkg/ui/events/ring_test.go`

- [ ] **Step 2.1: Write the failing tests for RingBuffer**

```go
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
```

- [ ] **Step 2.2: Run tests and verify they fail**

Run: `go test ./pkg/ui/events/ -run TestRingBuffer -v`
Expected: FAIL — `NewRingBuffer` undefined.

- [ ] **Step 2.3: Create ring.go with RingBuffer implementation**

```go
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
```

- [ ] **Step 2.4: Run tests and verify they pass**

Run: `go test ./pkg/ui/events/ -run TestRingBuffer -v`
Expected: PASS for all six TestRingBuffer_* tests.

- [ ] **Step 2.5: Commit**

```bash
git add pkg/ui/events/ring.go pkg/ui/events/ring_test.go
git commit -m "feat(events): add RingBuffer for session-scoped event retention"
```

---

### Task 3: Snapshot diff — created / closed detection

**Files:**
- Create: `pkg/ui/events/diff.go`
- Create: `pkg/ui/events/diff_test.go`

- [ ] **Step 3.1: Write the failing test for created/closed detection**

```go
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

func TestDiff_NoChange(t *testing.T) {
	issue := mkIssue("bt-1", "Alpha", model.StatusOpen)
	events := Diff([]model.Issue{issue}, []model.Issue{issue}, time.Now(), SourceDolt)
	if len(events) != 0 {
		t.Fatalf("Diff with no changes should emit 0 events, got %d", len(events))
	}
}
```

- [ ] **Step 3.2: Run tests and verify they fail**

Run: `go test ./pkg/ui/events/ -run TestDiff -v`
Expected: FAIL — `Diff` undefined.

- [ ] **Step 3.3: Create diff.go with the skeleton and created/closed logic**

```go
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
```

- [ ] **Step 3.4: Run tests and verify they pass**

Run: `go test ./pkg/ui/events/ -run TestDiff -v`
Expected: PASS on all five TestDiff_* tests from 3.1.

- [ ] **Step 3.5: Commit**

```bash
git add pkg/ui/events/diff.go pkg/ui/events/diff_test.go
git commit -m "feat(events): Diff emits EventCreated and EventClosed on snapshot compare"
```

---

### Task 4: Snapshot diff — edited detection with field summary

**Files:**
- Modify: `pkg/ui/events/diff.go`
- Modify: `pkg/ui/events/diff_test.go`

- [ ] **Step 4.1: Add failing tests for edit detection**

Append to `pkg/ui/events/diff_test.go`:

```go
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
```

- [ ] **Step 4.2: Run tests and verify they fail**

Run: `go test ./pkg/ui/events/ -run TestDiff_Edited -v`
Expected: FAIL on four of the five new tests (TestDiff_EditedIgnoresUpdatedAt may pass since the stub returns false).

- [ ] **Step 4.3: Implement editSummary in diff.go**

Replace the stub `editSummary` in `pkg/ui/events/diff.go` with:

```go
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
```

Also add `"fmt"` and `"strings"` to the imports in `diff.go` if not already present.

- [ ] **Step 4.4: Run tests and verify they pass**

Run: `go test ./pkg/ui/events/ -v`
Expected: PASS on all TestDiff_Edited_* cases plus all previously passing tests.

- [ ] **Step 4.5: Commit**

```bash
git add pkg/ui/events/diff.go pkg/ui/events/diff_test.go
git commit -m "feat(events): detect edited fields with named/aggregated Summary"
```

---

### Task 5: Snapshot diff — commented detection

**Files:**
- Modify: `pkg/ui/events/diff.go`
- Modify: `pkg/ui/events/diff_test.go`

- [ ] **Step 5.1: Add failing tests for comment detection**

Append to `pkg/ui/events/diff_test.go`:

```go
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
	if events[0].Summary[len(events[0].Summary)-1] != '…' && !hasEllipsisSuffix(events[0].Summary) {
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
```

- [ ] **Step 5.2: Run tests and verify they fail**

Run: `go test ./pkg/ui/events/ -run TestDiff -v`
Expected: FAIL on the three new tests (stub `newCommentedEvent` returns zero value).

- [ ] **Step 5.3: Implement newCommentedEvent in diff.go**

Replace the stub `newCommentedEvent` in `pkg/ui/events/diff.go` with:

```go
// commentSummaryLimit is the maximum rune length of a comment-derived
// Summary string. Longer comments are truncated with a trailing "...".
const commentSummaryLimit = 80

// newCommentedEvent builds an EventCommented from the latest new comment
// on the bead. oldIssue is used to determine which comments are new; the
// summary derives from the most recently added one.
func newCommentedEvent(oldIssue, newIssue model.Issue, at time.Time, source EventSource) Event {
	summary := ""
	if len(newIssue.Comments) > 0 {
		latest := newIssue.Comments[len(newIssue.Comments)-1]
		if latest != nil {
			summary = truncateForSummary(latest.Text, commentSummaryLimit)
		}
	}
	return Event{
		ID:      computeID(newIssue.ID, EventCommented, at),
		Kind:    EventCommented,
		BeadID:  newIssue.ID,
		Repo:    repoFromBeadID(newIssue.ID),
		Title:   newIssue.Title,
		Summary: summary,
		Actor:   newIssue.Assignee,
		At:      at,
		Source:  source,
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
```

- [ ] **Step 5.4: Run tests and verify they pass**

Run: `go test ./pkg/ui/events/ -v`
Expected: PASS on TestDiff_NewComment, TestDiff_CommentTextTruncatedAt80, TestDiff_MultipleNewComments_UsesLatest, and all prior tests.

- [ ] **Step 5.5: Commit**

```bash
git add pkg/ui/events/diff.go pkg/ui/events/diff_test.go
git commit -m "feat(events): detect new comments and truncate Summary at 80 runes"
```

---

### Task 6: Bulk-flood cap emits a single EventBulk marker

**Files:**
- Modify: `pkg/ui/events/diff.go`
- Modify: `pkg/ui/events/diff_test.go`

- [ ] **Step 6.1: Add failing tests for the bulk cap**

Append to `pkg/ui/events/diff_test.go`:

```go
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
```

Add `"fmt"` to the imports of `diff_test.go` if not already present.

- [ ] **Step 6.2: Run tests and verify they fail**

Run: `go test ./pkg/ui/events/ -run TestDiff_AboveBulk -v`
Expected: FAIL — current `bulkSummary` returns empty string.

- [ ] **Step 6.3: Implement bulkSummary**

Replace the stub `bulkSummary` in `pkg/ui/events/diff.go` with:

```go
// bulkSummary renders the Summary string for an EventBulk marker that
// collapses many underlying events emitted by one diff.
func bulkSummary(n int) string {
	return fmt.Sprintf("%d beads changed (bulk operation)", n)
}
```

- [ ] **Step 6.4: Run tests and verify they pass**

Run: `go test ./pkg/ui/events/ -v`
Expected: PASS on TestDiff_BelowBulkThresholdEmitsIndividual and TestDiff_AboveBulkThresholdEmitsBulkMarker plus all prior tests.

- [ ] **Step 6.5: Commit**

```bash
git add pkg/ui/events/diff.go pkg/ui/events/diff_test.go
git commit -m "feat(events): cap per-diff emission at 100 events with EventBulk marker"
```

---

### Task 7: Collapse-for-ticker helper

**Files:**
- Create: `pkg/ui/events/collapse.go`
- Create: `pkg/ui/events/collapse_test.go`

- [ ] **Step 7.1: Write failing tests for collapse**

```go
// pkg/ui/events/collapse_test.go
package events

import (
	"testing"
	"time"
)

func mkEventAt(beadID string, kind EventKind, at time.Time) Event {
	return Event{
		ID:     computeID(beadID, kind, at),
		Kind:   kind,
		BeadID: beadID,
		Repo:   repoFromBeadID(beadID),
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
```

- [ ] **Step 7.2: Run tests and verify they fail**

Run: `go test ./pkg/ui/events/ -run TestCollapseForTicker -v`
Expected: FAIL — `CollapseForTicker` undefined.

- [ ] **Step 7.3: Create collapse.go**

```go
// pkg/ui/events/collapse.go
package events

import (
	"fmt"
	"sort"
	"time"
)

// CollapseForTicker returns a view of events where runs of the same
// BeadID + same Kind within the given window fold into their most
// recent event. The aggregated Summary reads "+ N fields" where N is
// the count of collapsed events (the original Summary is discarded
// because it is ambiguous in the collapsed case — the modal shows
// the raw events for full detail).
//
// Input ordering is preserved within non-collapsed groups. Output is
// ordered oldest-first.
//
// This is a pure function — the input slice is not modified, and
// dismissed events are NOT filtered out (callers filter before calling
// if they want to hide dismissed events).
func CollapseForTicker(events []Event, window time.Duration) []Event {
	if len(events) == 0 {
		return nil
	}

	// Sort a copy by (BeadID, Kind, At) so we can scan groups in order.
	sorted := make([]Event, len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].BeadID != sorted[j].BeadID {
			return sorted[i].BeadID < sorted[j].BeadID
		}
		if sorted[i].Kind != sorted[j].Kind {
			return sorted[i].Kind < sorted[j].Kind
		}
		return sorted[i].At.Before(sorted[j].At)
	})

	var out []Event
	i := 0
	for i < len(sorted) {
		start := i
		// Advance while BeadID, Kind match AND the event is within
		// `window` of the group's first event.
		for i+1 < len(sorted) &&
			sorted[i+1].BeadID == sorted[start].BeadID &&
			sorted[i+1].Kind == sorted[start].Kind &&
			sorted[i+1].At.Sub(sorted[start].At) <= window {
			i++
		}
		if i == start {
			out = append(out, sorted[start])
		} else {
			// Emit the most recent event with aggregated Summary.
			latest := sorted[i]
			latest.Summary = fmt.Sprintf("+ %d fields", i-start+1)
			out = append(out, latest)
		}
		i++
	}

	// Return output in chronological order (oldest-first).
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].At.Before(out[j].At)
	})
	return out
}
```

- [ ] **Step 7.4: Run tests and verify they pass**

Run: `go test ./pkg/ui/events/ -v`
Expected: PASS on all six TestCollapseForTicker_* cases plus every prior events-package test.

- [ ] **Step 7.5: Commit**

```bash
git add pkg/ui/events/collapse.go pkg/ui/events/collapse_test.go
git commit -m "feat(events): CollapseForTicker folds same-bead same-kind runs within window"
```

---

### Task 8: Attach RingBuffer to Model

**Files:**
- Modify: `pkg/ui/model.go`

- [ ] **Step 8.1: Add the events field to Model**

Open `pkg/ui/model.go` and locate the status-message fields near line 530 (the block that includes `statusMsg`, `statusIsError`, etc.). Immediately AFTER that block, add:

```go
	// Activity event ring buffer (bt-d5wr). Populated by handleSnapshotReady
	// via events.Diff; consumed by the footer ticker + count badge and the
	// notification center modal (both implemented in later beads). Session-
	// scoped; not persisted across bt restarts.
	events *events.RingBuffer
```

Add `"github.com/seanmartinsmith/beadstui/pkg/ui/events"` to the import block of `model.go` if not already present.

- [ ] **Step 8.2: Initialize the ring buffer in NewModel**

Locate the `return Model{...}` construction inside `NewModel` (around line 1055 in the current tree). Add a new field in the struct literal:

```go
		events:                 events.NewRingBuffer(events.DefaultCapacity),
```

Place it alongside other initializer lines; alignment of the `:` is not required by go fmt.

- [ ] **Step 8.3: Verify build and existing tests still pass**

Run: `go build ./... && go vet ./... && go test ./pkg/ui/ -count=1`
Expected: build succeeds, vet clean, all pkg/ui tests pass (no new tests yet — Task 9 adds integration tests).

- [ ] **Step 8.4: Commit**

```bash
git add pkg/ui/model.go
git commit -m "feat(ui): attach events.RingBuffer to Model"
```

---

### Task 9: Emit events from handleSnapshotReady

**Files:**
- Modify: `pkg/ui/model_update_data.go`
- Create: `pkg/ui/snapshot_events_test.go`

- [ ] **Step 9.1: Write a failing integration test**

Create `pkg/ui/snapshot_events_test.go`:

```go
package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// mkSnapshot builds a minimal DataSnapshot sufficient for the
// handleSnapshotReady path to swap pointers and run the diff.
func mkSnapshot(issues []model.Issue) *DataSnapshot {
	return &DataSnapshot{
		Issues:     issues,
		CreatedAt:  time.Now(),
		Phase2Ready: true,
	}
}

func TestHandleSnapshotReady_EmitsCreateEvent(t *testing.T) {
	// Bootstrap a Model with a prior snapshot and feed a new snapshot
	// that adds one bead. Verify the ring buffer captures a create event.
	initial := []model.Issue{{ID: "bt-1", Title: "alpha", Status: model.StatusOpen}}
	m := NewModel(initial, nil, "", nil)
	m.data.snapshot = mkSnapshot(initial)

	next := []model.Issue{
		{ID: "bt-1", Title: "alpha", Status: model.StatusOpen},
		{ID: "portal-9", Title: "beta", Status: model.StatusOpen},
	}
	msg := SnapshotReadyMsg{Snapshot: mkSnapshot(next)}
	modelAny, _ := m.Update(msg)
	m2 := modelAny.(Model)

	got := m2.events.Snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 event emitted, got %d", len(got))
	}
	if got[0].Kind != events.EventCreated || got[0].BeadID != "portal-9" {
		t.Errorf("unexpected event: %+v", got[0])
	}
}

func TestHandleSnapshotReady_SkipsInTimeTravel(t *testing.T) {
	initial := []model.Issue{{ID: "bt-1", Title: "alpha", Status: model.StatusOpen}}
	m := NewModel(initial, nil, "", nil)
	m.data.snapshot = mkSnapshot(initial)
	m.timeTravelMode = true // active time-travel must suppress emission

	next := []model.Issue{
		{ID: "bt-1", Title: "alpha", Status: model.StatusOpen},
		{ID: "portal-9", Title: "beta", Status: model.StatusOpen},
	}
	msg := SnapshotReadyMsg{Snapshot: mkSnapshot(next)}
	modelAny, _ := m.Update(msg)
	m2 := modelAny.(Model)

	if n := len(m2.events.Snapshot()); n != 0 {
		t.Fatalf("time-travel must not emit events, got %d", n)
	}
}

func TestHandleSnapshotReady_SkipsOnBootstrap(t *testing.T) {
	// First snapshot with a nil prior must not emit creates for every
	// existing bead — that would flood the ring on startup.
	m := NewModel(nil, nil, "", nil)
	m.data.snapshot = nil // bootstrap path
	m.data.snapshotInitPending = true

	next := []model.Issue{
		{ID: "bt-1", Title: "alpha", Status: model.StatusOpen},
		{ID: "bt-2", Title: "beta", Status: model.StatusOpen},
	}
	msg := SnapshotReadyMsg{Snapshot: mkSnapshot(next)}
	modelAny, _ := m.Update(msg)
	m2 := modelAny.(Model)

	if n := len(m2.events.Snapshot()); n != 0 {
		t.Fatalf("bootstrap snapshot must not emit events, got %d", n)
	}
}

// Silence unused-import warning on tea during tests that do not inspect cmds.
var _ = tea.Batch
```

- [ ] **Step 9.2: Run tests and verify they fail**

Run: `go test ./pkg/ui/ -run TestHandleSnapshotReady_Emits -v`
Expected: FAIL — events are not yet emitted from `handleSnapshotReady`.

- [ ] **Step 9.3: Wire emission into handleSnapshotReady**

Open `pkg/ui/model_update_data.go`. The `handleSnapshotReady` function starts near line 23 and has this approximate shape:

```
func (m Model) handleSnapshotReady(msg SnapshotReadyMsg) (Model, tea.Cmd) {
    ... nil check ...
    firstSnapshot := m.data.snapshotInitPending && m.data.snapshot == nil
    m.data.snapshotInitPending = false
    ... clearAttentionOverlay ...
    ... time-travel reset: m.timeTravelMode = false ...  ← must capture entry state BEFORE this
    ... selected/board selection ...
    oldSnapshot := m.data.snapshot
    m.data.snapshot = msg.Snapshot                       ← pointer swap
    ... latency recording ...
    ... downstream: list update, phase2 wait, etc. ...
```

Make two edits:

**Edit 1** — immediately AFTER `m.data.snapshotInitPending = false`, capture the entry time-travel state:

```go
	wasTimeTravel := m.timeTravelMode
```

This line is required because the function clears `m.timeTravelMode` a few lines later; without capturing it first, the downstream guard cannot tell whether this handler call began in time-travel mode.

**Edit 2** — immediately AFTER the pointer swap block (the `m.data.snapshot = msg.Snapshot` line and its subsequent latency-recording `if` block), insert:

```go
	// bt-d5wr: emit activity events from the snapshot diff.
	// Gated on: (a) not the bootstrap snapshot (no prior to diff against),
	// (b) oldSnapshot is non-nil, (c) ring buffer is initialized,
	// (d) this handler call did NOT begin in time-travel mode (diffing a
	// historical snapshot against a live one produces spurious events).
	if !firstSnapshot && oldSnapshot != nil && m.events != nil && !wasTimeTravel {
		diff := events.Diff(oldSnapshot.Issues, msg.Snapshot.Issues, time.Now(), events.SourceDolt)
		if len(diff) > 0 {
			m.events.AppendMany(diff)
		}
	}
```

Add `"github.com/seanmartinsmith/beadstui/pkg/ui/events"` to the import block of `model_update_data.go`.

- [ ] **Step 9.4: Run tests and verify they pass**

Run: `go test ./pkg/ui/ -run TestHandleSnapshotReady_Emits -v`
Expected: PASS on TestHandleSnapshotReady_EmitsCreateEvent, TestHandleSnapshotReady_SkipsInTimeTravel, TestHandleSnapshotReady_SkipsOnBootstrap.

- [ ] **Step 9.5: Run the full pkg/ui test suite to check for regressions**

Run: `go test ./pkg/ui/ -count=1 -timeout 180s`
Expected: PASS. If anything else regresses, STOP and inspect — this integration is the riskiest step.

- [ ] **Step 9.6: Commit**

```bash
git add pkg/ui/model_update_data.go pkg/ui/snapshot_events_test.go
git commit -m "feat(ui): emit activity events from handleSnapshotReady snapshot diff"
```

---

### Task 10: Final verification

**Files:** none to modify.

- [ ] **Step 10.1: Full build and vet**

Run: `go build ./... && go vet ./...`
Expected: clean exit, no output.

- [ ] **Step 10.2: Full events package test with race detector**

Run: `go test ./pkg/ui/events/ -race -count=1 -timeout 60s -v`
Expected: PASS on all tests with no race warnings.

- [ ] **Step 10.3: Full ui package test**

Run: `go test ./pkg/ui/ -count=1 -timeout 180s`
Expected: PASS.

- [ ] **Step 10.4: Install the binary**

Run: `go install ./cmd/bt/`
Expected: clean exit.

- [ ] **Step 10.5: Smoke-test against real data**

Run: `bt --global` in one terminal. In another terminal, edit a bead: `bd update bt-some-id --priority P1`. Wait up to 5s for the poll cycle.

Expected: no visible change in the UI (this bead does not ship UI surfaces). But the process stays responsive — no panics, no freezes. The event ring is populated internally; the next bead (footer redesign) will surface it.

Kill `bt` when satisfied.

- [ ] **Step 10.6: Final commit and push**

Nothing more to commit. Push all work:

```bash
git pull --rebase
bd dolt push
git push
git status
```

Expected: `git status` says "up to date with origin".

---

## Post-implementation notes

**Close the implementation bead** (whichever ID is assigned for this plan) with the standard bt-d5wr-derived close template referencing this plan file.

**Do NOT close bt-d5wr** — that bead covers the entire three-bead cluster; close it only after all three implementation beads land.

**Close bt-spzz as duplicate** when this event pipeline ships — its acceptance criteria are subsumed. Reference: bt-d5wr spec section 5, "Closures triggered by this design".

**Next plan**: footer redesign bead. Depends on this event pipeline existing. Starts when this plan is merged.
