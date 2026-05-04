package projects

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// makeGitDir creates a directory with a fake .git subdirectory and returns
// the parent directory path. Used to satisfy pathLooksLikeGitRepo.
func makeGitDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("makeGitDir: %v", err)
	}
	return dir
}

// TestStamp_NewEntry verifies that stamping an empty (nil) registry
// produces an entry with the correct path and a LastSeen equal to the
// time passed in.
func TestStamp_NewEntry(t *testing.T) {
	now := time.Now()
	r := Stamp(nil, "bt", "/some/path", "", now)

	entry, ok := r["bt"]
	if !ok {
		t.Fatal("expected entry for prefix 'bt', got none")
	}
	if entry.Path != "/some/path" {
		t.Errorf("path: got %q, want %q", entry.Path, "/some/path")
	}
	if entry.Subdir != "" {
		t.Errorf("Subdir: got %q, want %q", entry.Subdir, "")
	}
	if !entry.LastSeen.Equal(now) {
		t.Errorf("LastSeen: got %v, want %v", entry.LastSeen, now)
	}
}

// TestStamp_OverwritePath verifies that stamping an existing prefix with
// a different path overwrites the path field.
func TestStamp_OverwritePath(t *testing.T) {
	now := time.Now()
	r := Stamp(nil, "bt", "/old/path", "", now)
	r = Stamp(r, "bt", "/new/path", "", now)

	entry := r["bt"]
	if entry.Path != "/new/path" {
		t.Errorf("path: got %q, want %q", entry.Path, "/new/path")
	}
}

// TestStamp_RefreshLastSeen verifies that stamping the same prefix with a
// later time advances LastSeen to the new value.
func TestStamp_RefreshLastSeen(t *testing.T) {
	t1 := time.Now()
	t2 := t1.Add(5 * time.Minute)

	r := Stamp(nil, "bt", "/some/path", "", t1)
	r = Stamp(r, "bt", "/some/path", "", t2)

	entry := r["bt"]
	if !entry.LastSeen.Equal(t2) {
		t.Errorf("LastSeen: got %v, want %v", entry.LastSeen, t2)
	}
}

// TestLookup_Missing verifies that looking up an unknown prefix on an
// empty registry returns a zero Entry and false.
func TestLookup_Missing(t *testing.T) {
	r := Registry{}
	entry, ok := Lookup(r, "bt")
	if ok {
		t.Fatal("expected ok=false for missing prefix, got true")
	}
	if entry.Path != "" || !entry.LastSeen.IsZero() {
		t.Errorf("expected zero Entry, got %+v", entry)
	}
}

// TestLookup_Valid verifies that a stamped entry pointing to a real git
// directory is found and returns (entry, true).
func TestLookup_Valid(t *testing.T) {
	dir := makeGitDir(t)
	now := time.Now()
	r := Stamp(nil, "bt", dir, "", now)

	entry, ok := Lookup(r, "bt")
	if !ok {
		t.Fatal("expected ok=true for valid entry, got false")
	}
	if entry.Path != dir {
		t.Errorf("path: got %q, want %q", entry.Path, dir)
	}
}

// TestLookup_StalePathMissing verifies that when the recorded path no
// longer exists on disk, Lookup returns (entry, false) with the stale
// Entry still populated so callers can name the missing path.
func TestLookup_StalePathMissing(t *testing.T) {
	dir := makeGitDir(t)
	now := time.Now()
	r := Stamp(nil, "bt", dir, "", now)

	// Remove the whole directory to make the path stale.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	entry, ok := Lookup(r, "bt")
	if ok {
		t.Fatal("expected ok=false for stale path, got true")
	}
	// Entry must still carry the stale path so callers can surface it.
	if entry.Path != dir {
		t.Errorf("stale entry path: got %q, want %q", entry.Path, dir)
	}
}

// TestLookup_StaleNotGitRepo verifies that when the directory exists but
// no longer contains a .git entry, Lookup returns (entry, false).
func TestLookup_StaleNotGitRepo(t *testing.T) {
	dir := makeGitDir(t)
	now := time.Now()
	r := Stamp(nil, "bt", dir, "", now)

	// Remove only the .git entry; leave the directory itself intact.
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("RemoveAll .git: %v", err)
	}

	entry, ok := Lookup(r, "bt")
	if ok {
		t.Fatal("expected ok=false when .git is gone, got true")
	}
	if entry.Path != dir {
		t.Errorf("stale entry path: got %q, want %q", entry.Path, dir)
	}
}

// TestSaveLoadRoundTrip verifies that Save followed by Load produces an
// identical Registry. It also covers Subdir round-trip: "bt" stamps with
// empty subdir (omitempty must suppress the key on disk), "bd" stamps with
// a non-empty subdir (key must be present and round-trip cleanly).
func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")

	t1 := time.Now().UTC()
	t2 := t1.Add(7 * time.Minute)

	r := Stamp(nil, "bt", "/path/to/bt", "", t1)
	r = Stamp(r, "bd", "/path/to/bd", "services/billing", t2)

	if err := Save(path, r); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("len: got %d, want 2", len(loaded))
	}

	for _, prefix := range []string{"bt", "bd"} {
		orig, ok := r[prefix]
		if !ok {
			t.Fatalf("original missing prefix %q", prefix)
		}
		got, ok := loaded[prefix]
		if !ok {
			t.Fatalf("loaded missing prefix %q", prefix)
		}
		if got.Path != orig.Path {
			t.Errorf("[%s] path: got %q, want %q", prefix, got.Path, orig.Path)
		}
		if got.Subdir != orig.Subdir {
			t.Errorf("[%s] Subdir: got %q, want %q", prefix, got.Subdir, orig.Subdir)
		}
		// time.Equal handles the monotonic-clock reading being stripped on
		// JSON round-trip; nanosecond resolution survives RFC3339Nano encoding.
		if !got.LastSeen.Equal(orig.LastSeen) {
			t.Errorf("[%s] LastSeen: got %v, want %v", prefix, got.LastSeen, orig.LastSeen)
		}
	}

	// Verify on-disk JSON key presence via raw unmarshal.
	// "bt" has empty Subdir: omitempty must suppress the "subdir" key.
	// "bd" has non-empty Subdir: "subdir" key must be present.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var rawMap map[string]map[string]any
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		t.Fatalf("raw unmarshal: %v", err)
	}
	if _, hasSubdir := rawMap["bt"]["subdir"]; hasSubdir {
		t.Error("bt entry: expected no 'subdir' key (omitempty), but found one")
	}
	if v, hasSubdir := rawMap["bd"]["subdir"]; !hasSubdir {
		t.Error("bd entry: expected 'subdir' key to be present, but not found")
	} else if v != "services/billing" {
		t.Errorf("bd entry: subdir value got %q, want %q", v, "services/billing")
	}
	// Confirm the raw bytes contain the expected subdir value as a
	// cheap regression guard against omitempty being accidentally removed.
	if !bytes.Contains(raw, []byte(`"services/billing"`)) {
		t.Error("raw JSON missing expected subdir value \"services/billing\"")
	}
}

// TestLoad_Corrupt verifies that a file containing garbage bytes is
// treated as an empty registry (not an error).
func TestLoad_Corrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")

	if err := os.WriteFile(path, []byte("this is not valid json }{"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	r, err := Load(path)
	if err != nil {
		t.Fatalf("expected nil error for corrupt file, got: %v", err)
	}
	if len(r) != 0 {
		t.Errorf("expected empty registry, got %d entries", len(r))
	}
}

// TestSave_AtomicTempRename verifies that after a successful Save, no
// leftover .tmp-* files remain in the target directory. A temp-file
// leak would surface here.
func TestSave_AtomicTempRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projects.json")

	r := Stamp(nil, "bt", "/some/path", "", time.Now())
	if err := Save(path, r); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	for _, e := range entries {
		if matched, _ := filepath.Match("projects.json.tmp-*", e.Name()); matched {
			t.Errorf("leftover temp file found: %s", e.Name())
		}
	}

	// Confirm the target file itself is present.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("target file missing after Save: %v", err)
	}
}

// TestLookupAndValidate_EnvOverride verifies that BT_PROJECTS_REGISTRY_PATH
// overrides the default path and that LookupAndValidate reads from it.
func TestLookupAndValidate_EnvOverride(t *testing.T) {
	repoDir := makeGitDir(t)

	registryDir := t.TempDir()
	registryPath := filepath.Join(registryDir, "projects.json")

	t.Setenv("BT_PROJECTS_REGISTRY_PATH", registryPath)

	r := Stamp(nil, "testpfx", repoDir, "", time.Now())
	if err := Save(registryPath, r); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, ok := LookupAndValidate("testpfx")
	if !ok {
		t.Fatal("LookupAndValidate: expected ok=true, got false")
	}
	if got != repoDir {
		t.Errorf("path: got %q, want %q", got, repoDir)
	}
}

// TestLoad_Missing verifies that loading a nonexistent file returns an
// empty registry and nil error (first-launch normalcy).
func TestLoad_Missing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does_not_exist.json")

	r, err := Load(path)
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if len(r) != 0 {
		t.Errorf("expected empty registry, got %d entries", len(r))
	}
}

// TestStamp_WithSubdir verifies that a non-empty subdir is stored and
// retrieved correctly. This covers the forward-compat monorepo path
// where .beads/ lives below the git toplevel.
func TestStamp_WithSubdir(t *testing.T) {
	now := time.Now()
	r := Stamp(nil, "mono", "/repo/root", "services/billing", now)

	entry, ok := r["mono"]
	if !ok {
		t.Fatal("expected entry for prefix 'mono', got none")
	}
	if entry.Path != "/repo/root" {
		t.Errorf("Path: got %q, want %q", entry.Path, "/repo/root")
	}
	if entry.Subdir != "services/billing" {
		t.Errorf("Subdir: got %q, want %q", entry.Subdir, "services/billing")
	}
	if !entry.LastSeen.Equal(now) {
		t.Errorf("LastSeen: got %v, want %v", entry.LastSeen, now)
	}
}
