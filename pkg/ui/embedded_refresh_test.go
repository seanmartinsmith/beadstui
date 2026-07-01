package ui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/internal/datasource"
)

// runBdUI runs a bd subcommand in dir, non-interactively, failing on error.
func runBdUI(t *testing.T, bdPath, dir string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bdPath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BD_NON_INTERACTIVE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bd %v failed: %v\noutput: %s", args, err, out)
	}
}

// TestBackgroundWorker_EmbeddedLiveRefresh proves the "stay live as beads
// change" requirement for embedded mode (bt-ij71a): the worker watches the
// Dolt storage manifest and re-runs `bd export` when a concurrent `bd create`
// mutates it, atomically swapping in the new dataset. Gated behind
// BT_EMBEDDED_INTEGRATION=1 (spawns real bd).
func TestBackgroundWorker_EmbeddedLiveRefresh(t *testing.T) {
	if os.Getenv("BT_EMBEDDED_INTEGRATION") != "1" {
		t.Skip("set BT_EMBEDDED_INTEGRATION=1 to run bd-spawning embedded integration tests")
	}
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("bd CLI not on PATH")
	}

	root := t.TempDir()
	runBdUI(t, bdPath, root, "init")
	beadsDir := filepath.Join(root, ".beads")
	if _, ok := datasource.ReadEmbeddedConfig(beadsDir); !ok {
		t.Fatalf("bd init did not produce an embedded project")
	}
	runBdUI(t, bdPath, root, "create", "seed", "-t", "task", "-p", "1")

	src, err := datasource.DiscoverSource(datasource.DiscoveryOptions{BeadsDir: beadsDir})
	if err != nil || src.Type != datasource.SourceTypeEmbeddedDolt {
		t.Fatalf("DiscoverSource type=%q err=%v", src.Type, err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsDir:      beadsDir,
		DataSource:    &src,
		DebounceDelay: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker: %v", err)
	}
	defer worker.Stop()

	// Embedded must get an event-driven manifest watcher (not the SQL poll).
	if worker.WatcherChanged() == nil {
		t.Fatal("embedded worker has no watcher; live refresh would not fire on bd writes")
	}

	if err := worker.Start(); err != nil {
		t.Fatalf("worker.Start: %v", err)
	}

	// Initial load.
	worker.TriggerRefresh()
	initial := waitForIssueCount(t, worker, 1, 5*time.Second)
	if initial < 1 {
		t.Fatalf("initial snapshot has %d issues, want >=1", initial)
	}

	// Concurrent write mutates the manifest; the watcher must fire and the
	// worker must re-export, WITHOUT any explicit TriggerRefresh call here.
	runBdUI(t, bdPath, root, "create", "live-one", "-t", "task", "-p", "2")

	got := waitForIssueCount(t, worker, initial+1, 10*time.Second)
	if got < initial+1 {
		t.Fatalf("live refresh did not pick up the new bead: snapshot has %d issues, want >=%d", got, initial+1)
	}
}

// waitForIssueCount polls the worker snapshot until it holds at least `want`
// issues or the timeout elapses, returning the last observed count.
func waitForIssueCount(t *testing.T, w *BackgroundWorker, want int, timeout time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(timeout)
	last := 0
	for time.Now().Before(deadline) {
		if snap := w.GetSnapshot(); snap != nil {
			last = len(snap.Issues)
			if last >= want {
				return last
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return last
}
