package correlation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeJSONL writes one or more entries as JSON lines to path, creating parent dirs.
func writeJSONL(t *testing.T, path string, entries ...CorrelationFeedback) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	for _, e := range entries {
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
}

func TestFeedbackStore_LoadFromLegacyBeadsDir(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, ".beads", FeedbackFileName)
	writeJSONL(t, legacy, CorrelationFeedback{
		CommitSHA:    "abc123",
		BeadID:       "bt-001",
		FeedbackAt:   time.Now().UTC(),
		FeedbackBy:   "tester",
		Type:         FeedbackConfirm,
		OriginalConf: 0.8,
	})

	store := NewFeedbackStore(dir)
	if err := store.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	got, ok := store.Get("abc123", "bt-001")
	if !ok {
		t.Fatalf("expected legacy entry to be read, got nothing")
	}
	if got.Type != FeedbackConfirm {
		t.Errorf("type: got %q, want %q", got.Type, FeedbackConfirm)
	}

	// Sanity: the .bt/ file should not have been created merely by reading legacy.
	newPath := filepath.Join(dir, ".bt", FeedbackFileName)
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Errorf(".bt/ file should not exist after read-only Load; stat err = %v", err)
	}
}

func TestFeedbackStore_SaveWritesToBtDir(t *testing.T) {
	dir := t.TempDir()
	store := NewFeedbackStore(dir)

	if err := store.Confirm("sha1", "bt-002", "tester", 0.9, "looks right"); err != nil {
		t.Fatalf("Confirm: %v", err)
	}

	newPath := filepath.Join(dir, ".bt", FeedbackFileName)
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("expected new feedback file at %s: %v", newPath, err)
	}

	legacyPath := filepath.Join(dir, ".beads", FeedbackFileName)
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Errorf("legacy path should not be created on Save; stat err = %v", err)
	}
}

func TestFeedbackStore_NewLocationWinsOnKeyCollision(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	// Same key in both files; the .bt/ file has the newer "rejected" decision.
	legacyPath := filepath.Join(dir, ".beads", FeedbackFileName)
	newPath := filepath.Join(dir, ".bt", FeedbackFileName)

	writeJSONL(t, legacyPath, CorrelationFeedback{
		CommitSHA:    "sha2",
		BeadID:       "bt-003",
		FeedbackAt:   now.Add(-time.Hour),
		FeedbackBy:   "old",
		Type:         FeedbackConfirm,
		OriginalConf: 0.7,
	})
	writeJSONL(t, newPath, CorrelationFeedback{
		CommitSHA:    "sha2",
		BeadID:       "bt-003",
		FeedbackAt:   now,
		FeedbackBy:   "new",
		Type:         FeedbackReject,
		OriginalConf: 0.7,
	})

	store := NewFeedbackStore(dir)
	if err := store.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	got, ok := store.Get("sha2", "bt-003")
	if !ok {
		t.Fatalf("expected entry to be present")
	}
	if got.Type != FeedbackReject {
		t.Errorf("expected .bt/ value (reject) to win, got %q", got.Type)
	}
	if got.FeedbackBy != "new" {
		t.Errorf("expected feedback_by from .bt/, got %q", got.FeedbackBy)
	}
}

func TestFeedbackStore_LoadMergesDistinctKeys(t *testing.T) {
	dir := t.TempDir()

	legacyPath := filepath.Join(dir, ".beads", FeedbackFileName)
	newPath := filepath.Join(dir, ".bt", FeedbackFileName)

	writeJSONL(t, legacyPath, CorrelationFeedback{
		CommitSHA: "legacy-only", BeadID: "bt-A",
		Type: FeedbackConfirm, OriginalConf: 0.6,
	})
	writeJSONL(t, newPath, CorrelationFeedback{
		CommitSHA: "new-only", BeadID: "bt-B",
		Type: FeedbackReject, OriginalConf: 0.5,
	})

	store := NewFeedbackStore(dir)
	if err := store.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := store.Get("legacy-only", "bt-A"); !ok {
		t.Error("expected legacy-only entry to be loaded")
	}
	if _, ok := store.Get("new-only", "bt-B"); !ok {
		t.Error("expected new-only entry to be loaded")
	}
	if n := len(store.GetAll()); n != 2 {
		t.Errorf("expected 2 entries, got %d", n)
	}
}
