package correlation

// Tests for the Dolt-native extractor introduced in bt-08sh.1 and the
// dispatch branching introduced in bt-08sh.4. The fixture uses pure-Go
// modernc.org/sqlite in :memory: mode -- no external Dolt dependency.
//
// Fixture schema mirrors only the columns DoltExtractor reads from upstream's
// events / wisp_events tables (issue_id, event_type, actor, old_value,
// new_value, comment, created_at). To add a new event-shape test:
//
//  1. Call newDoltEventsFixture(t) for a clean *sql.DB with both tables.
//  2. insertEvent(t, db, "events", ...) -- "events" or "wisp_events".
//  3. Construct NewDoltExtractor(db) and call Extract / ExtractForBead.
//
// SQLite stores time.Time as TEXT (RFC3339-ish per modernc's formatTime) and
// parses it back on scan. The DoltExtractor's parameter binding uses
// time.Time values directly; no test-side formatting required.

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// newDoltEventsFixture returns an open in-memory SQLite handle with empty
// events and wisp_events tables matching the subset of upstream beads'
// schema that DoltExtractor reads. The handle is closed via t.Cleanup.
func newDoltEventsFixture(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Schema mirrors upstream's 0005_create_events.up.sql and 0021 mirror for
	// wisp_events. We only declare the columns DoltExtractor reads, plus an
	// auto-incrementing id so each insert gets a unique row.
	for _, table := range []string{"events", "wisp_events"} {
		_, err := db.Exec(`
			CREATE TABLE ` + table + ` (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				issue_id   TEXT NOT NULL,
				event_type TEXT NOT NULL,
				actor      TEXT,
				old_value  TEXT,
				new_value  TEXT,
				comment    TEXT,
				created_at TIMESTAMP NOT NULL
			)
		`)
		if err != nil {
			t.Fatalf("create %s: %v", table, err)
		}
	}
	return db
}

// insertEvent inserts one row into the chosen table. Pass empty strings for
// nullable columns the test doesn't care about; SQLite will store them as
// "" rather than NULL, which is fine since DoltExtractor reads via
// sql.NullString and treats both as zero-value strings.
func insertEvent(t *testing.T, db *sql.DB, table, issueID, eventType, actor, oldValue, newValue, comment string, createdAt time.Time) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO `+table+` (issue_id, event_type, actor, old_value, new_value, comment, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		issueID, eventType, actor, oldValue, newValue, comment, createdAt,
	)
	if err != nil {
		t.Fatalf("insert %s row: %v", table, err)
	}
}

// TestDoltExtractor_BasicEvents verifies the happy path for the Dolt-native
// extractor: three lifecycle events for one bead are returned in chronological
// order, mapped to bt's EventType taxonomy, with empty CommitSHA per 592c.
func TestDoltExtractor_BasicEvents(t *testing.T) {
	db := newDoltEventsFixture(t)

	t0 := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	// Insert out of chronological order to prove ORDER BY does the work.
	insertEvent(t, db, "events", "bt-1", "status_changed", "sms", "open", "closed", "", t0.Add(2*time.Hour))
	insertEvent(t, db, "events", "bt-1", "created", "sms", "", "", "initial", t0)
	insertEvent(t, db, "events", "bt-1", "status_changed", "sms", "open", "in_progress", "", t0.Add(1*time.Hour))

	events, err := NewDoltExtractor(db).Extract(ExtractOptions{})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}

	want := []struct {
		eventType EventType
		ts        time.Time
	}{
		{EventCreated, t0},
		{EventClaimed, t0.Add(1 * time.Hour)},
		{EventClosed, t0.Add(2 * time.Hour)},
	}
	for i, w := range want {
		if events[i].EventType != w.eventType {
			t.Errorf("events[%d].EventType = %q, want %q", i, events[i].EventType, w.eventType)
		}
		if !events[i].Timestamp.Equal(w.ts) {
			t.Errorf("events[%d].Timestamp = %v, want %v", i, events[i].Timestamp, w.ts)
		}
		if events[i].CommitSHA != "" {
			t.Errorf("events[%d].CommitSHA = %q, want empty (no event-to-commit link upstream per 592c)", i, events[i].CommitSHA)
		}
		if events[i].BeadID != "bt-1" {
			t.Errorf("events[%d].BeadID = %q, want bt-1", i, events[i].BeadID)
		}
	}
}

// TestDoltExtractor_StatusChangeMapping exercises mapDoltEventType through
// the public Extract interface, covering each documented branch:
//
//   - status_changed new=in_progress   -> EventClaimed
//   - status_changed new=closed        -> EventClosed
//   - status_changed new=open old=closed -> EventReopened
//   - status_changed new=open old=other  -> EventModified (fallback)
//   - status_changed new=other         -> EventModified (fallback)
//   - created                          -> EventCreated
//   - updated / commented / etc.       -> EventModified (default arm)
//
// One row uses wisp_events to confirm the UNION ALL half is wired in -- if
// someone deletes the wisp_events half of buildEventsQuery, the wispRow case
// below disappears from the result and this test fails.
func TestDoltExtractor_StatusChangeMapping(t *testing.T) {
	db := newDoltEventsFixture(t)

	t0 := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		table      string
		eventType  string
		oldValue   string
		newValue   string
		wantMapped EventType
	}{
		{"created", "events", "created", "", "", EventCreated},
		{"status_changed -> claimed", "events", "status_changed", "open", "in_progress", EventClaimed},
		{"status_changed -> closed", "events", "status_changed", "in_progress", "closed", EventClosed},
		{"status_changed -> reopened (closed->open)", "events", "status_changed", "closed", "open", EventReopened},
		{"status_changed open from non-closed origin", "events", "status_changed", "in_progress", "open", EventModified},
		{"status_changed unknown new_value", "events", "status_changed", "open", "blocked", EventModified},
		{"updated", "events", "updated", "", "", EventModified},
		{"commented", "events", "commented", "", "", EventModified},
		{"wisp_events: status_changed -> claimed", "wisp_events", "status_changed", "open", "in_progress", EventClaimed},
	}

	// Insert in order so ORDER BY created_at ASC matches the cases slice.
	for i, c := range cases {
		insertEvent(t, db, c.table, "bt-1", c.eventType, "sms", c.oldValue, c.newValue, "", t0.Add(time.Duration(i)*time.Minute))
	}

	events, err := NewDoltExtractor(db).Extract(ExtractOptions{})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(events) != len(cases) {
		t.Fatalf("len(events) = %d, want %d", len(events), len(cases))
	}

	for i, c := range cases {
		if events[i].EventType != c.wantMapped {
			t.Errorf("case %q: events[%d].EventType = %q, want %q", c.name, i, events[i].EventType, c.wantMapped)
		}
	}
}

// TestDoltExtractor_BeadIDFilter verifies ExtractOptions.BeadID restricts the
// result set to one issue. Also covers ExtractForBead, which is a thin wrapper
// that forwards opts.BeadID to Extract.
func TestDoltExtractor_BeadIDFilter(t *testing.T) {
	db := newDoltEventsFixture(t)

	t0 := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	insertEvent(t, db, "events", "bt-1", "created", "sms", "", "", "", t0)
	insertEvent(t, db, "events", "bt-2", "created", "sms", "", "", "", t0.Add(1*time.Minute))
	insertEvent(t, db, "events", "bt-1", "status_changed", "sms", "open", "closed", "", t0.Add(2*time.Minute))
	insertEvent(t, db, "wisp_events", "bt-2", "status_changed", "sms", "open", "closed", "", t0.Add(3*time.Minute))

	t.Run("Extract with BeadID", func(t *testing.T) {
		events, err := NewDoltExtractor(db).Extract(ExtractOptions{BeadID: "bt-1"})
		if err != nil {
			t.Fatalf("Extract returned error: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("len(events) = %d, want 2 (only bt-1)", len(events))
		}
		for i, e := range events {
			if e.BeadID != "bt-1" {
				t.Errorf("events[%d].BeadID = %q, want bt-1", i, e.BeadID)
			}
		}
	})

	t.Run("ExtractForBead routes through Extract", func(t *testing.T) {
		events, err := NewDoltExtractor(db).ExtractForBead("bt-2", ExtractOptions{})
		if err != nil {
			t.Fatalf("ExtractForBead returned error: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("len(events) = %d, want 2 (only bt-2, including wisp half)", len(events))
		}
		for i, e := range events {
			if e.BeadID != "bt-2" {
				t.Errorf("events[%d].BeadID = %q, want bt-2", i, e.BeadID)
			}
		}
	})
}

// TestDoltExtractor_TimeWindow verifies Since and Until in ExtractOptions
// are honored with inclusive bounds. The buildEventsWhere helper translates
// these to "created_at >= ?" and "created_at <= ?", so the boundary events
// should appear in the result.
func TestDoltExtractor_TimeWindow(t *testing.T) {
	db := newDoltEventsFixture(t)

	t0 := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	tMid := t0.Add(1 * time.Hour)
	tEnd := t0.Add(2 * time.Hour)

	insertEvent(t, db, "events", "bt-1", "created", "sms", "", "", "before", t0.Add(-1*time.Hour))
	insertEvent(t, db, "events", "bt-1", "updated", "sms", "", "", "at-since", tMid)
	insertEvent(t, db, "events", "bt-1", "updated", "sms", "", "", "mid", tMid.Add(30*time.Minute))
	insertEvent(t, db, "events", "bt-1", "updated", "sms", "", "", "at-until", tEnd)
	insertEvent(t, db, "events", "bt-1", "status_changed", "sms", "open", "closed", "after", tEnd.Add(1*time.Hour))

	since := tMid
	until := tEnd
	events, err := NewDoltExtractor(db).Extract(ExtractOptions{Since: &since, Until: &until})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	// Want: at-since, mid, at-until (3 events). Both boundaries inclusive.
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3 (boundaries inclusive)", len(events))
	}
	wantComments := []string{"at-since", "mid", "at-until"}
	for i, w := range wantComments {
		if events[i].CommitMsg != w {
			t.Errorf("events[%d].CommitMsg = %q, want %q", i, events[i].CommitMsg, w)
		}
	}
}

// TestDoltExtractor_NilDB confirms the entry-point nil-handle guard in the
// extractor: callers that construct via NewDoltExtractor(nil) get a clean
// error instead of a nil-deref panic. Mirrors bt-08sh.1's "Extra polish
// beyond the spec" note about this guard.
func TestDoltExtractor_NilDB(t *testing.T) {
	e := NewDoltExtractor(nil)
	_, err := e.Extract(ExtractOptions{})
	if err == nil {
		t.Fatal("Extract on nil-db extractor should error, got nil")
	}
}
