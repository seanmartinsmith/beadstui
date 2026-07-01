package datasource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeMetadata(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write metadata.json: %v", err)
	}
}

const embeddedMetadata = `{"database":"dolt","backend":"dolt","dolt_mode":"embedded","dolt_database":"embtest","project_id":"c0331338"}`
const serverMetadata = `{"database":"dolt","backend":"dolt","dolt_mode":"server","dolt_database":"bt","project_id":"6fd231df"}`

func TestReadEmbeddedConfig(t *testing.T) {
	t.Run("embedded", func(t *testing.T) {
		dir := t.TempDir()
		writeMetadata(t, dir, embeddedMetadata)
		db, ok := ReadEmbeddedConfig(dir)
		if !ok {
			t.Fatal("ReadEmbeddedConfig ok = false, want true")
		}
		if db != "embtest" {
			t.Errorf("db = %q, want embtest", db)
		}
	})

	t.Run("server mode is not embedded", func(t *testing.T) {
		dir := t.TempDir()
		writeMetadata(t, dir, serverMetadata)
		if _, ok := ReadEmbeddedConfig(dir); ok {
			t.Error("ReadEmbeddedConfig ok = true for server mode, want false")
		}
	})

	t.Run("missing metadata is not embedded", func(t *testing.T) {
		dir := t.TempDir()
		if _, ok := ReadEmbeddedConfig(dir); ok {
			t.Error("ReadEmbeddedConfig ok = true for missing metadata, want false")
		}
	})
}

// TestDiscoverSource_EmbeddedRoutesToBdCLI is the mode-detection guard
// (bt-ij71a): an embedded project must resolve to SourceTypeEmbeddedDolt and
// must NOT return ErrDoltRequired or otherwise fall through to the server
// attach path (which would try to reach / start a Dolt server on 3307 and
// deadlock the concurrent bd CLI - bt-qrt2u). No server is running against the
// temp dir, so a fall-through would surface as ErrDoltRequired.
func TestDiscoverSource_EmbeddedRoutesToBdCLI(t *testing.T) {
	dir := t.TempDir()
	writeMetadata(t, dir, embeddedMetadata)

	src, err := DiscoverSource(DiscoveryOptions{BeadsDir: dir})
	if err != nil {
		t.Fatalf("DiscoverSource error = %v, want nil (embedded must not require a server)", err)
	}
	if src.Type != SourceTypeEmbeddedDolt {
		t.Fatalf("src.Type = %q, want %q", src.Type, SourceTypeEmbeddedDolt)
	}
	// Path must point at the repo root (parent of .beads) so the reader can
	// run `bd export` there.
	wantRoot := filepath.Dir(dir)
	if src.Path != wantRoot {
		t.Errorf("src.Path = %q, want repo root %q", src.Path, wantRoot)
	}
}

func TestNewEmbeddedReader_Validation(t *testing.T) {
	if _, err := NewEmbeddedReader(DataSource{Type: SourceTypeJSONLFallback, Path: "x"}); err == nil {
		t.Error("expected error for non-embedded source type")
	}
	if _, err := NewEmbeddedReader(DataSource{Type: SourceTypeEmbeddedDolt, Path: ""}); err == nil {
		t.Error("expected error for empty repo root")
	}
}

// TestEmbeddedReader_BdNotOnPath asserts the clear error when the bd CLI is
// missing (an edge case called out in bt-ij71a). Uses an injected lookPath so
// the test is deterministic regardless of the host PATH.
func TestEmbeddedReader_BdNotOnPath(t *testing.T) {
	r, err := NewEmbeddedReader(DataSource{Type: SourceTypeEmbeddedDolt, Path: t.TempDir()})
	if err != nil {
		t.Fatalf("NewEmbeddedReader: %v", err)
	}
	r.lookPath = func(string) (string, error) { return "", errors.New("executable file not found in %PATH%") }
	if _, err := r.LoadIssues(); err == nil {
		t.Fatal("LoadIssues succeeded with bd absent, want error")
	} else if !strings.Contains(err.Error(), "bd") {
		t.Errorf("error should name the bd CLI, got: %v", err)
	}
}

// TestEmbeddedReader_RetriesTransientFailure locks the retry that keeps reads
// robust against the Windows delete-pending race when `bd export` collides
// with a concurrent `bd create` commit (observed in the live-refresh
// integration test). Export fails twice, then succeeds.
func TestEmbeddedReader_RetriesTransientFailure(t *testing.T) {
	calls := 0
	r := &EmbeddedReader{
		repoRoot:     "repo",
		lookPath:     func(string) (string, error) { return "bd", nil },
		maxRetries:   3,
		retryBackoff: time.Millisecond,
		sleep:        func(time.Duration) {},
		runExport: func(_ context.Context, _, _ string) ([]byte, []byte, error) {
			calls++
			if calls < 3 {
				return nil, []byte("a non close operation ... delete pending"), errors.New("exit status 1")
			}
			return []byte(`{"id":"x-1","title":"T","status":"open","priority":1,"issue_type":"task"}`), nil, nil
		},
	}
	issues, err := r.LoadIssues()
	if err != nil {
		t.Fatalf("LoadIssues after transient failures: %v", err)
	}
	if calls != 3 {
		t.Errorf("export called %d times, want 3 (2 transient failures + 1 success)", calls)
	}
	if len(issues) != 1 {
		t.Errorf("got %d issues, want 1", len(issues))
	}
}

// TestEmbeddedReader_ExhaustsRetries asserts a persistent failure surfaces an
// error (with the attempt count) rather than looping forever.
func TestEmbeddedReader_ExhaustsRetries(t *testing.T) {
	calls := 0
	r := &EmbeddedReader{
		repoRoot:     "repo",
		lookPath:     func(string) (string, error) { return "bd", nil },
		maxRetries:   2,
		retryBackoff: time.Millisecond,
		sleep:        func(time.Duration) {},
		runExport: func(_ context.Context, _, _ string) ([]byte, []byte, error) {
			calls++
			return nil, []byte("still pending"), errors.New("exit status 1")
		},
	}
	if _, err := r.LoadIssues(); err == nil {
		t.Fatal("expected error after exhausting retries")
	} else if !strings.Contains(err.Error(), "attempts") {
		t.Errorf("error should mention attempts: %v", err)
	}
	if calls != 3 { // maxRetries=2 => attempts 0,1,2
		t.Errorf("export called %d times, want 3 (maxRetries+1)", calls)
	}
}

func TestEmbeddedManifestPath(t *testing.T) {
	dir := t.TempDir()
	writeMetadata(t, dir, embeddedMetadata)
	got := EmbeddedManifestPath(dir)
	want := filepath.Join(dir, "embeddeddolt", "embtest", ".dolt", "noms", "manifest")
	if got != want {
		t.Errorf("EmbeddedManifestPath = %q, want %q", got, want)
	}

	// Non-embedded projects have no manifest to watch.
	serverDir := t.TempDir()
	writeMetadata(t, serverDir, serverMetadata)
	if p := EmbeddedManifestPath(serverDir); p != "" {
		t.Errorf("EmbeddedManifestPath for server mode = %q, want empty", p)
	}
}
