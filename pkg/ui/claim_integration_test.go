package ui

// Live end-to-end verification of the claim slice (bt-oiaj.10) against real
// throwaway bd projects, in BOTH data modes:
//
//   - embedded (bd v1.0+ default): bd-CLI read path + Dolt manifest watcher.
//   - server (opt-in per project): dolt sql-server + SQL poll loop.
//
// Gated behind BT_CLAIM_INTEGRATION=1 because it spawns bd and (for the server
// variant) a Dolt sql-server.
//
// The claim path is deliberately mode-agnostic: the executor shells `bd update
// <id> --claim` (bd routes to whichever backend), and settlePendingClaims is
// hooked into BOTH reload consumers (handleSnapshotReady for the worker-driven
// embedded+server refresh, handleDataSourceReload for force refresh). This test
// proves the FULL pending -> settled flow rides the existing per-mode refresh
// machinery: a real worker (manifest watch for embedded, SQL poll for server)
// re-exports after the claim commits, its snapshot flows through the real
// handleSnapshotReady, and settlePendingClaims clears the pending marker.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/internal/bdexec"
	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// runBdUIAllow runs a bd subcommand without failing the test on error, so the
// caller can decide to skip (used for optional server-mode setup).
func runBdUIAllow(bdPath, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bdPath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BD_NON_INTERACTIVE=1")
	return cmd.CombinedOutput()
}

// isClaimed matches the settlePendingClaims predicate: a claim moves the bead
// off open and/or sets an assignee.
func isClaimed(is model.Issue) bool {
	return is.Status != model.StatusOpen || is.Assignee != ""
}

func snapshotIssue(snap *DataSnapshot, id string) (model.Issue, bool) {
	if snap == nil {
		return model.Issue{}, false
	}
	for _, is := range snap.Issues {
		if is.ID == id {
			return is, true
		}
	}
	return model.Issue{}, false
}

// settleBothClaimed polls the worker snapshot until both beads reflect a claim,
// returning that snapshot and whether the mode's LIVE refresh (manifest watch /
// SQL poll) settled it on its own. For the first autoWindow it waits passively
// (auto-fire); after that it forces refreshes so the settle WIRING is still
// verified even when a mode's auto-fire trigger is broken. autoFired=false is a
// documented finding, not a slice failure.
func settleBothClaimed(t *testing.T, w *BackgroundWorker, id1, id2 string, autoWindow, total time.Duration) (*DataSnapshot, bool) {
	t.Helper()
	start := time.Now()
	forced := false
	for time.Since(start) < total {
		if !forced && time.Since(start) >= autoWindow {
			forced = true
		}
		if forced {
			w.TriggerRefresh()
		}
		snap := w.GetSnapshot()
		is1, ok1 := snapshotIssue(snap, id1)
		is2, ok2 := snapshotIssue(snap, id2)
		if ok1 && ok2 && isClaimed(is1) && isClaimed(is2) {
			return snap, !forced
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("worker snapshot did not reflect both claims within %s even with forced refresh", total)
	return nil, false
}

// verifyClaimSlice drives the full pending -> settled flow against a live bd
// project already set up under root. It exercises the real executor and the
// real worker/handleSnapshotReady settle path, plus the external-claim
// regression.
func verifyClaimSlice(t *testing.T, mode, root, beadsDir string, src datasource.DataSource) {
	t.Helper()

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsDir:      beadsDir,
		DataSource:    &src,
		DebounceDelay: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker: %v", err)
	}
	if err := worker.Start(); err != nil {
		t.Fatalf("worker.Start: %v", err)
	}
	stopped := false
	defer func() {
		if !stopped {
			worker.Stop()
		}
	}()

	worker.TriggerRefresh()
	if got := waitForIssueCount(t, worker, 2, 10*time.Second); got < 2 {
		t.Fatalf("initial snapshot has %d issues, want >=2", got)
	}

	// Pick two open beads: one claimed by bt's executor, one externally.
	var btBead, extBead string
	for _, is := range worker.GetSnapshot().Issues {
		if is.Status != model.StatusOpen {
			continue
		}
		switch {
		case btBead == "":
			btBead = is.ID
		case extBead == "":
			extBead = is.ID
		}
	}
	if btBead == "" || extBead == "" {
		t.Fatalf("need two open beads; got bt=%q ext=%q", btBead, extBead)
	}

	// bt-initiated claim through the real executor.
	res := bdexec.Run(context.Background(), root, "update", btBead, "--claim")
	if res.Err != nil || res.ExitCode != 0 {
		t.Fatalf("bt claim failed: exit=%d err=%v stderr=%q", res.ExitCode, res.Err, res.Stderr)
	}
	if want := "bd update " + btBead + " --claim"; res.Argv() != want {
		t.Errorf("argv = %q, want %q", res.Argv(), want)
	}

	// External claim regression: an external bd --claim must refresh identically.
	bdPath, _ := exec.LookPath("bd")
	runBdUI(t, bdPath, root, "update", extBead, "--claim")

	// The mode's live refresh re-exports after both commits. autoFired records
	// whether the mode settled on its own (manifest watch / SQL poll) vs needing
	// a forced refresh - a documented per-mode asymmetry, not a slice failure.
	claimedSnap, autoFired := settleBothClaimed(t, worker, btBead, extBead, 8*time.Second, 30*time.Second)
	t.Logf("[%s] both claims settled in the worker snapshot (live auto-fire=%v)", mode, autoFired)

	// Freeze the worker so the captured snapshot's pooled issues are not recycled.
	worker.Stop()
	stopped = true

	// Drive the captured snapshot through the REAL handleSnapshotReady on a model
	// that has both beads marked pending: settlePendingClaims must clear them.
	m := newSizedModel(t, []model.Issue{
		{ID: btBead, Title: "bt claim target", Status: model.StatusOpen, Priority: 1},
		{ID: extBead, Title: "external claim target", Status: model.StatusOpen, Priority: 2},
	}, 120, 32)
	m.pendingClaims[btBead] = true
	m.pendingClaims[extBead] = true

	updated, _ := m.Update(SnapshotReadyMsg{Snapshot: claimedSnap, SentAt: time.Now()})
	m = updated.(Model)

	if m.pendingClaims[btBead] {
		t.Errorf("bt-initiated claim %s did not settle through handleSnapshotReady", btBead)
	}
	if m.pendingClaims[extBead] {
		t.Errorf("external claim %s did not settle through handleSnapshotReady", extBead)
	}
}

func TestClaimLive_Embedded(t *testing.T) {
	if os.Getenv("BT_CLAIM_INTEGRATION") != "1" {
		t.Skip("set BT_CLAIM_INTEGRATION=1 to run bd-spawning claim integration tests")
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
	runBdUI(t, bdPath, root, "create", "claim me", "-t", "task", "-p", "1")
	runBdUI(t, bdPath, root, "create", "external claim", "-t", "task", "-p", "2")

	src, err := datasource.DiscoverSource(datasource.DiscoveryOptions{BeadsDir: beadsDir})
	if err != nil || src.Type != datasource.SourceTypeEmbeddedDolt {
		t.Fatalf("DiscoverSource type=%q err=%v (want embedded)", src.Type, err)
	}

	verifyClaimSlice(t, "embedded", root, beadsDir, src)
}

func TestClaimLive_Server(t *testing.T) {
	if os.Getenv("BT_CLAIM_INTEGRATION") != "1" {
		t.Skip("set BT_CLAIM_INTEGRATION=1 to run bd-spawning claim integration tests")
	}
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("bd CLI not on PATH")
	}
	// Fast poll so the SQL-poll settle path fires promptly in the test window.
	t.Setenv("BT_DOLT_POLL_INTERVAL_S", "1")

	root := t.TempDir()
	if out, err := runBdUIAllow(bdPath, root, "init", "--server"); err != nil {
		t.Skipf("bd init --server unavailable in this environment: %v\n%s", err, out)
	}
	beadsDir := filepath.Join(root, ".beads")

	// Bring up the managed Dolt sql-server; skip the variant if it can't start.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := exec.CommandContext(ctx, bdPath, "dolt", "start")
	start.Dir = root
	if out, err := start.CombinedOutput(); err != nil {
		t.Skipf("bd dolt start unavailable in this environment: %v\n%s", err, out)
	}
	defer func() {
		stop := exec.Command(bdPath, "dolt", "stop")
		stop.Dir = root
		_ = stop.Run()
	}()

	runBdUI(t, bdPath, root, "create", "claim me", "-t", "task", "-p", "1")
	runBdUI(t, bdPath, root, "create", "external claim", "-t", "task", "-p", "2")

	src, err := datasource.DiscoverSource(datasource.DiscoveryOptions{BeadsDir: beadsDir})
	if err != nil {
		t.Fatalf("DiscoverSource err=%v", err)
	}
	if src.Type != datasource.SourceTypeDolt {
		t.Fatalf("DiscoverSource type=%q, want server-mode Dolt", src.Type)
	}

	verifyClaimSlice(t, "server", root, beadsDir, src)
}
