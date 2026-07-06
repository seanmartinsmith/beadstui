package datasource

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// writeEmbeddedFixture creates a minimal on-disk embedded project (metadata.json
// + a manifest file with the given content) rooted at t.TempDir(), returning
// the repo root, .beads dir, and the manifest path so tests can mutate it to
// simulate a commit without spawning real bd.
func writeEmbeddedFixture(t *testing.T, dbName string, manifestContent string) (root, beadsDir, manifestPath string) {
	t.Helper()
	root = t.TempDir()
	beadsDir = filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beadsDir: %v", err)
	}
	meta := `{"database":"dolt","backend":"dolt","dolt_mode":"embedded","dolt_database":"` + dbName + `"}`
	writeMetadata(t, beadsDir, meta)

	manifestDir := filepath.Join(beadsDir, "embeddeddolt", dbName, ".dolt", "noms")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}
	manifestPath = filepath.Join(manifestDir, "manifest")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return root, beadsDir, manifestPath
}

func TestComputeEmbeddedCacheKey(t *testing.T) {
	t.Run("embedded with manifest", func(t *testing.T) {
		_, beadsDir, _ := writeEmbeddedFixture(t, "embtest", "root-hash-v1")
		key, ok := computeEmbeddedCacheKey(beadsDir)
		if !ok {
			t.Fatal("computeEmbeddedCacheKey ok = false, want true")
		}
		if key.dbName != "embtest" {
			t.Errorf("dbName = %q, want embtest", key.dbName)
		}
		var zero [32]byte
		if key.manifestHash == zero {
			t.Error("manifestHash is zero, want a real sha256 of the manifest content")
		}
	})

	t.Run("non-embedded metadata", func(t *testing.T) {
		beadsDir := t.TempDir()
		writeMetadata(t, beadsDir, serverMetadata)
		if _, ok := computeEmbeddedCacheKey(beadsDir); ok {
			t.Error("computeEmbeddedCacheKey ok = true for server mode, want false")
		}
	})

	t.Run("embedded but manifest missing (fresh project)", func(t *testing.T) {
		beadsDir := t.TempDir()
		writeMetadata(t, beadsDir, embeddedMetadata)
		// No embeddeddolt/<db>/.dolt/noms/manifest written.
		if _, ok := computeEmbeddedCacheKey(beadsDir); ok {
			t.Error("computeEmbeddedCacheKey ok = true with no manifest file, want false (nothing stable to key on)")
		}
	})

	t.Run("different manifest content produces different hash", func(t *testing.T) {
		_, beadsDir1, _ := writeEmbeddedFixture(t, "embtest", "root-hash-v1")
		key1, ok := computeEmbeddedCacheKey(beadsDir1)
		if !ok {
			t.Fatal("key1 not ok")
		}
		_, beadsDir2, _ := writeEmbeddedFixture(t, "embtest", "root-hash-v2")
		key2, ok := computeEmbeddedCacheKey(beadsDir2)
		if !ok {
			t.Fatal("key2 not ok")
		}
		if key1.manifestHash == key2.manifestHash {
			t.Error("different manifest content hashed to the same key")
		}
	})
}

func TestEmbeddedSnapshotCachePath_Sanitizes(t *testing.T) {
	root := t.TempDir()
	path := embeddedSnapshotCachePath(root, "weird/name:with*chars")
	wantDir := filepath.Join(root, ".bt", "snapshot-cache")
	if filepath.Dir(path) != wantDir {
		t.Errorf("cache path dir = %q, want %q", filepath.Dir(path), wantDir)
	}
	if filepath.Base(path) != "embedded-weird_name_with_chars.snap" {
		t.Errorf("cache path base = %q", filepath.Base(path))
	}

	// Two different repo roots must never collide on the same cache file,
	// even with an identical db name - per-DB keying is anchored on the
	// repoRoot-scoped .bt/ directory, not just the file basename.
	other := t.TempDir()
	otherPath := embeddedSnapshotCachePath(other, "weird/name:with*chars")
	if otherPath == path {
		t.Error("cache paths for two different repo roots collided")
	}
}

func TestEmbeddedSnapshotCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.snap")
	key := embeddedCacheKey{dbName: "embtest", manifestHash: [32]byte{1, 2, 3}}
	issues := []model.Issue{
		{ID: "embtest-1", Title: "Alpha", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeTask, Labels: []string{"area:data"}},
		{ID: "embtest-2", Title: "Beta", Status: model.StatusClosed, Priority: 2, IssueType: model.TypeBug},
	}

	if err := writeEmbeddedSnapshot(path, key, issues); err != nil {
		t.Fatalf("writeEmbeddedSnapshot: %v", err)
	}

	got, hit := readEmbeddedSnapshot(path, key)
	if !hit {
		t.Fatal("readEmbeddedSnapshot: hit = false, want true")
	}
	if len(got) != 2 || got[0].ID != "embtest-1" || got[1].ID != "embtest-2" {
		t.Errorf("round-tripped issues mismatch: %+v", got)
	}
	if len(got[0].Labels) != 1 || got[0].Labels[0] != "area:data" {
		t.Errorf("labels lost in round trip: %+v", got[0].Labels)
	}
}

func TestEmbeddedSnapshotCache_HashMismatchIsMiss(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.snap")
	written := embeddedCacheKey{dbName: "embtest", manifestHash: [32]byte{1}}
	if err := writeEmbeddedSnapshot(path, written, []model.Issue{{ID: "x-1", Title: "T", Status: model.StatusOpen, IssueType: model.TypeTask}}); err != nil {
		t.Fatalf("write: %v", err)
	}

	wanted := embeddedCacheKey{dbName: "embtest", manifestHash: [32]byte{2}} // different hash
	if _, hit := readEmbeddedSnapshot(path, wanted); hit {
		t.Error("readEmbeddedSnapshot hit = true for a changed manifest hash, want false (must never serve stale data)")
	}
}

func TestEmbeddedSnapshotCache_DBNameMismatchIsMiss(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.snap")
	written := embeddedCacheKey{dbName: "project-a", manifestHash: [32]byte{9}}
	if err := writeEmbeddedSnapshot(path, written, []model.Issue{{ID: "a-1", Title: "T", Status: model.StatusOpen, IssueType: model.TypeTask}}); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Same hash, different DB - must never cross-serve project A's snapshot
	// for project B (per-DB keying requirement).
	wanted := embeddedCacheKey{dbName: "project-b", manifestHash: [32]byte{9}}
	if _, hit := readEmbeddedSnapshot(path, wanted); hit {
		t.Error("readEmbeddedSnapshot hit = true across different DB names, want false")
	}
}

func TestEmbeddedSnapshotCache_MissingFileIsMiss(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.snap")
	if _, hit := readEmbeddedSnapshot(path, embeddedCacheKey{}); hit {
		t.Error("readEmbeddedSnapshot hit = true for a missing file, want false")
	}
}

func TestEmbeddedSnapshotCache_CorruptOrForeignFileIsMiss(t *testing.T) {
	dir := t.TempDir()

	t.Run("truncated header", func(t *testing.T) {
		path := filepath.Join(dir, "truncated.snap")
		if err := os.WriteFile(path, []byte("BT"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, hit := readEmbeddedSnapshot(path, embeddedCacheKey{}); hit {
			t.Error("hit = true for a truncated header, want false")
		}
	})

	t.Run("wrong magic", func(t *testing.T) {
		path := filepath.Join(dir, "wrong-magic.snap")
		if err := os.WriteFile(path, []byte("XXXX\x01\x00\x00\x00"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, hit := readEmbeddedSnapshot(path, embeddedCacheKey{}); hit {
			t.Error("hit = true for wrong magic, want false")
		}
	})

	t.Run("future version", func(t *testing.T) {
		// Write via the real encoder, then flip the version byte so a
		// future schema change is guaranteed to invalidate cleanly rather
		// than misdecoding old bytes as the new layout.
		path := filepath.Join(dir, "future-version.snap")
		key := embeddedCacheKey{dbName: "x", manifestHash: [32]byte{1}}
		if err := writeEmbeddedSnapshot(path, key, []model.Issue{{ID: "x-1", Title: "T", Status: model.StatusOpen, IssueType: model.TypeTask}}); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		// Version is the uint16 immediately after the 4-byte magic (little-endian).
		data[4] = 0xFF
		data[5] = 0xFF
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, hit := readEmbeddedSnapshot(path, key); hit {
			t.Error("hit = true for an unrecognized future version, want false")
		}
	})
}

func TestEmbeddedSnapshotCache_EmptyIssuesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.snap")
	key := embeddedCacheKey{dbName: "embtest", manifestHash: [32]byte{7}}
	if err := writeEmbeddedSnapshot(path, key, nil); err != nil {
		t.Fatalf("write empty snapshot: %v", err)
	}
	got, hit := readEmbeddedSnapshot(path, key)
	if !hit {
		t.Fatal("hit = false for an empty-but-valid snapshot, want true")
	}
	if len(got) != 0 {
		t.Errorf("got %d issues, want 0", len(got))
	}
}

// TestEmbeddedReader_CacheHitSkipsExport exercises the full LoadIssues()
// wrapping via an injected runExport (no real bd needed): a first load
// exports and populates the cache; a second load with the manifest
// unchanged must hit the cache and NOT call runExport again; mutating the
// manifest (simulating a real commit) must force a third, fresh export
// rather than serving the now-stale cached snapshot (bt-xdah0 staleness
// requirement).
func TestEmbeddedReader_CacheHitSkipsExport(t *testing.T) {
	root, _, manifestPath := writeEmbeddedFixture(t, "embtest", "root-hash-v1")

	calls := 0
	r := &EmbeddedReader{
		repoRoot:     root,
		lookPath:     func(string) (string, error) { return "bd", nil },
		maxRetries:   0,
		retryBackoff: time.Millisecond,
		sleep:        func(time.Duration) {},
		runExport: func(_ context.Context, _, _ string) ([]byte, []byte, error) {
			calls++
			return []byte(`{"id":"embtest-1","title":"T","status":"open","priority":1,"issue_type":"task"}`), nil, nil
		},
	}

	issues1, err := r.LoadIssues()
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if len(issues1) != 1 {
		t.Fatalf("first load: got %d issues, want 1", len(issues1))
	}
	if calls != 1 {
		t.Fatalf("expected 1 export call after first load, got %d", calls)
	}

	issues2, err := r.LoadIssues()
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if calls != 1 {
		t.Errorf("manifest unchanged: expected export NOT re-run, but calls=%d", calls)
	}
	if len(issues2) != 1 || issues2[0].ID != "embtest-1" {
		t.Errorf("cached issues mismatch: %+v", issues2)
	}

	// Simulate a real Dolt commit: the manifest's bytes change.
	if err := os.WriteFile(manifestPath, []byte("root-hash-v2"), 0o644); err != nil {
		t.Fatalf("mutate manifest: %v", err)
	}

	issues3, err := r.LoadIssues()
	if err != nil {
		t.Fatalf("third load: %v", err)
	}
	if calls != 2 {
		t.Fatalf("manifest changed: expected a fresh export (calls=2), got calls=%d", calls)
	}
	if len(issues3) != 1 || issues3[0].ID != "embtest-1" {
		t.Errorf("post-change issues mismatch: %+v", issues3)
	}
}
