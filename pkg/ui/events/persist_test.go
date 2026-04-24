// pkg/ui/events/persist_test.go
package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPersist_RoundTrip writes events through a RingBuffer with a
// persist path set, then loads them back via LoadPersisted and verifies
// content fidelity.
func TestPersist_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	r := NewRingBuffer(50)
	r.SetPersistPath(path)

	now := time.Now().UTC()
	r.Append(Event{ID: "a", Kind: EventCreated, BeadID: "bt-1", Repo: "bt", Title: "alpha", At: now.Add(-2 * time.Hour)})
	r.AppendMany([]Event{
		{ID: "b", Kind: EventClosed, BeadID: "bt-2", Repo: "bt", Title: "beta", At: now.Add(-time.Hour)},
		{ID: "c", Kind: EventEdited, BeadID: "bt-3", Repo: "bt", Title: "gamma", At: now},
	})

	loaded, err := LoadPersisted(path, 24*time.Hour)
	if err != nil {
		t.Fatalf("LoadPersisted: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 events round-tripped, got %d", len(loaded))
	}
	if loaded[0].ID != "a" || loaded[2].ID != "c" {
		t.Errorf("order should match write order; got [%s,%s,%s]", loaded[0].ID, loaded[1].ID, loaded[2].ID)
	}
	if loaded[1].Kind != EventClosed {
		t.Errorf("Kind not preserved: got %v", loaded[1].Kind)
	}
}

// TestPersist_MaxAgeDropsOldEvents confirms LoadPersisted enforces the
// retention horizon so a long-lived JSONL doesn't replay ancient activity.
func TestPersist_MaxAgeDropsOldEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	r := NewRingBuffer(10)
	r.SetPersistPath(path)

	now := time.Now().UTC()
	r.Append(Event{ID: "old", Kind: EventCreated, BeadID: "bt-1", At: now.Add(-30 * 24 * time.Hour)})
	r.Append(Event{ID: "fresh", Kind: EventCreated, BeadID: "bt-2", At: now})

	loaded, err := LoadPersisted(path, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("LoadPersisted: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 event after age filter, got %d", len(loaded))
	}
	if loaded[0].ID != "fresh" {
		t.Errorf("expected the fresh event, got %s", loaded[0].ID)
	}
}

// TestPersist_HydrateRespectsCap mirrors the spec: hydrating more events
// than the ring's capacity drops the oldest, matching live Append semantics.
func TestPersist_HydrateRespectsCap(t *testing.T) {
	r := NewRingBuffer(3)
	now := time.Now().UTC()
	r.Hydrate([]Event{
		{ID: "1", At: now.Add(-4 * time.Minute)},
		{ID: "2", At: now.Add(-3 * time.Minute)},
		{ID: "3", At: now.Add(-2 * time.Minute)},
		{ID: "4", At: now.Add(-1 * time.Minute)},
		{ID: "5", At: now},
	})
	got := r.Snapshot()
	if len(got) != 3 {
		t.Fatalf("expected cap=3 to limit hydrate to 3 events, got %d", len(got))
	}
	if got[0].ID != "3" || got[2].ID != "5" {
		t.Errorf("oldest evicted; expected [3,4,5], got [%s,%s,%s]", got[0].ID, got[1].ID, got[2].ID)
	}
}

// TestPersist_CorruptLineSkippedNotFatal asserts that a bad JSON line in
// the middle of the file does not abort hydration of the surrounding
// well-formed events. Resilience matters more than strict parsing here.
func TestPersist_CorruptLineSkippedNotFatal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	now := time.Now().UTC()
	good1, _ := encodeEventLine(Event{ID: "a", Kind: EventCreated, BeadID: "bt-1", At: now})
	good2, _ := encodeEventLine(Event{ID: "b", Kind: EventClosed, BeadID: "bt-2", At: now})
	contents := good1 + "{not-valid-json\n" + good2

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	loaded, err := LoadPersisted(path, time.Hour)
	if err != nil {
		t.Fatalf("LoadPersisted should tolerate corrupt lines: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 well-formed events recovered, got %d", len(loaded))
	}
	if loaded[0].ID != "a" || loaded[1].ID != "b" {
		t.Errorf("good lines lost or reordered; got [%s,%s]", loaded[0].ID, loaded[1].ID)
	}
}

// TestPersist_MissingFileIsNotError asserts the boot path doesn't surface
// "file not found" when persistence is enabled on a fresh install.
func TestPersist_MissingFileIsNotError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-such-file.jsonl")

	loaded, err := LoadPersisted(path, time.Hour)
	if err != nil {
		t.Errorf("missing file should not error; got %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("missing file should yield empty slice; got %d", len(loaded))
	}
}

// TestPersist_NoOpWithoutPath confirms that calling Append on a buffer
// with no persist path configured does not crash and does not create
// any file. Opt-out path: SetPersistPath("") disables write-through.
func TestPersist_NoOpWithoutPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	r := NewRingBuffer(10)
	r.SetPersistPath(path)
	r.SetPersistPath("") // disable

	r.Append(Event{ID: "x", Kind: EventCreated, BeadID: "bt-1", At: time.Now().UTC()})

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("disabled persistence should not write any file; stat err = %v", err)
	}
	if r.Len() != 1 {
		t.Errorf("in-memory append should still work; Len = %d, want 1", r.Len())
	}
}

// encodeEventLine helper for tests: marshal an event the way the
// persister would write it.
func encodeEventLine(e Event) (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
