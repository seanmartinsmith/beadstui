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
// <id> --claim` (bd routes to whichever backend), and settlePendingWrites
// (bt-oiaj.13, generalized from settlePendingClaims) is hooked into BOTH
// reload consumers (handleSnapshotReady for the worker-driven embedded+server
// refresh, handleDataSourceReload for force refresh). This test proves the
// FULL pending -> settled flow rides the existing per-mode refresh machinery:
// a real worker (manifest watch for embedded, SQL poll for server) re-exports
// after the claim commits, its snapshot flows through the real
// handleSnapshotReady, and settlePendingWrites clears the pending marker.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/seanmartinsmith/beadstui/internal/bdexec"
	"github.com/seanmartinsmith/beadstui/internal/bdroute"
	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/workspace"
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

// isClaimed matches writeSettled's writeClaim predicate (settlePendingWrites):
// a claim moves the bead off open and/or sets an assignee.
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
	// that has both beads marked pending: settlePendingWrites must clear them.
	m := newSizedModel(t, []model.Issue{
		{ID: btBead, Title: "bt claim target", Status: model.StatusOpen, Priority: 1},
		{ID: extBead, Title: "external claim target", Status: model.StatusOpen, Priority: 2},
	}, 120, 32)
	m.pendingWrites[btBead] = pendingWrite{Kind: writeClaim, StartedAt: time.Now()}
	m.pendingWrites[extBead] = pendingWrite{Kind: writeClaim, StartedAt: time.Now()}

	updated, _ := m.Update(SnapshotReadyMsg{Snapshot: claimedSnap, SentAt: time.Now()})
	m = updated.(Model)

	if _, pending := m.pendingWrites[btBead]; pending {
		t.Errorf("bt-initiated claim %s did not settle through handleSnapshotReady", btBead)
	}
	if _, pending := m.pendingWrites[extBead]; pending {
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

// drainClaimResult executes cmd and returns the writeResultMsg it produces.
// confirmClaim batches the claim dispatch together with the spinner-tick cmd
// whenever the spinner isn't already active (the common case: the first claim
// in a session), so cmd() may yield a tea.BatchMsg ([]tea.Cmd, not yet
// executed) rather than the writeResultMsg directly - this unwraps that case
// by running each sub-command until it finds the one that produced it.
func drainClaimResult(t *testing.T, cmd tea.Cmd) writeResultMsg {
	t.Helper()
	msg := cmd()
	switch v := msg.(type) {
	case writeResultMsg:
		return v
	case tea.BatchMsg:
		for _, sub := range v {
			if sub == nil {
				continue
			}
			if res, ok := sub().(writeResultMsg); ok {
				return res
			}
		}
	}
	t.Fatalf("cmd() did not yield a writeResultMsg (got %T)", msg)
	return writeResultMsg{}
}

// firstOpenIssueViaWorker inits a throwaway bd project at root, creates one
// open task, and reads it back through the real BackgroundWorker/DataSource
// plumbing (not a hand-parsed `bd` CLI output) so the returned issue is
// exactly what bt's own read path would produce.
func firstOpenIssueViaWorker(t *testing.T, bdPath, root string) model.Issue {
	t.Helper()
	runBdUI(t, bdPath, root, "init")
	beadsDir := filepath.Join(root, ".beads")
	runBdUI(t, bdPath, root, "create", "route table target", "-t", "task", "-p", "1")

	src, err := datasource.DiscoverSource(datasource.DiscoveryOptions{BeadsDir: beadsDir})
	if err != nil {
		t.Fatalf("DiscoverSource(%s): %v", root, err)
	}

	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsDir:      beadsDir,
		DataSource:    &src,
		DebounceDelay: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker(%s): %v", root, err)
	}
	defer worker.Stop()
	if err := worker.Start(); err != nil {
		t.Fatalf("worker.Start(%s): %v", root, err)
	}
	worker.TriggerRefresh()
	if got := waitForIssueCount(t, worker, 1, 10*time.Second); got < 1 {
		t.Fatalf("initial snapshot at %s has %d issues, want >=1", root, got)
	}
	snap := worker.GetSnapshot()
	if snap == nil || len(snap.Issues) == 0 {
		t.Fatalf("empty snapshot at %s", root)
	}
	return snap.Issues[0]
}

// TestClaimLive_WorkspaceMultiProject is the bt-scc35 regression test: a
// workspace-mode route table must dispatch a claim to the CORRECT project
// checkout keyed by bead-ID prefix, across two DIFFERENT prefixes, and must
// refuse pre-flight (zero bd invocations) for a prefix the table doesn't
// know about. Throwaway projects + an isolated projects registry AND
// isolated settings.json - this test must never touch the developer's real
// ~/.bt files.
func TestClaimLive_WorkspaceMultiProject(t *testing.T) {
	if os.Getenv("BT_CLAIM_INTEGRATION") != "1" {
		t.Skip("set BT_CLAIM_INTEGRATION=1 to run bd-spawning claim integration tests")
	}
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("bd CLI not on PATH")
	}

	// bt-scc35 pollution-immunity requirement: even though this path never
	// reads the registry, isolate it (and settings.json) so a mistaken
	// future read can never reach the developer's real files.
	t.Setenv("BT_PROJECTS_REGISTRY_PATH", filepath.Join(t.TempDir(), "projects.json"))
	t.Setenv("BT_SETTINGS_PATH", filepath.Join(t.TempDir(), "settings.json"))

	root1 := t.TempDir()
	issue1 := firstOpenIssueViaWorker(t, bdPath, root1)
	prefix1, _, ok := strings.Cut(issue1.ID, "-")
	if !ok || prefix1 == "" {
		t.Fatalf("could not derive a prefix from issue ID %q", issue1.ID)
	}

	root2 := t.TempDir()
	issue2 := firstOpenIssueViaWorker(t, bdPath, root2)
	prefix2, _, ok := strings.Cut(issue2.ID, "-")
	if !ok || prefix2 == "" {
		t.Fatalf("could not derive a prefix from issue ID %q", issue2.ID)
	}
	if prefix1 == prefix2 {
		t.Fatalf("test setup collision: both throwaway projects assigned prefix %q; cannot exercise multi-prefix routing", prefix1)
	}

	table := bdroute.FromWorkspace([]workspace.LoadResult{
		{Prefix: prefix1, AbsPath: root1},
		{Prefix: prefix2, AbsPath: root2},
	})

	m := newSizedModelWithRoute(t, []model.Issue{issue1, issue2}, 120, 32, table)

	// Claim both beads - two DIFFERENT prefixes through the SAME route
	// table, each dispatching a REAL bd invocation (claimRunner is not
	// stubbed in this test).
	for _, iss := range []model.Issue{issue1, issue2} {
		m.claimTargetID = iss.ID
		updated, cmd := m.confirmClaim()
		m = updated
		if cmd == nil {
			t.Fatalf("confirmClaim(%s) returned a nil cmd; want a real claim dispatch", iss.ID)
		}
		res := drainClaimResult(t, cmd)
		if res.result.Err != nil || res.result.ExitCode != 0 {
			t.Fatalf("claim %s failed: exit=%d err=%v stderr=%q", iss.ID, res.result.ExitCode, res.result.Err, res.result.Stderr)
		}
	}

	// Verify both claims actually landed in their OWN (different) project
	// checkouts, proving the route table sent each claim to the right place.
	assertClaimedInProject(t, root1, issue1.ID)
	assertClaimedInProject(t, root2, issue2.ID)

	// Refusal on an unmappable third: a prefix the table has never heard of
	// must refuse pre-flight - no claim command, no pending state, no bd
	// invocation (no third project was even created).
	orphan := model.Issue{ID: "unmapped-zzz-1", Title: "orphan", Status: model.StatusOpen}
	m.data.issueMap[orphan.ID] = &orphan
	m.claimTargetID = orphan.ID
	updated, cmd := m.confirmClaim()
	m = updated
	if cmd != nil {
		t.Error("confirmClaim on an unmapped prefix must not dispatch a claim command")
	}
	if _, pending := m.pendingWrites[orphan.ID]; pending {
		t.Error("an unmappable claim must never enter the pending state")
	}
}

// TestClaimLive_SingleProjectUnchanged verifies confirmClaim's new
// Resolve()-based routing does not regress the plain single-project case:
// a bdroute.SingleProject table must still dispatch a real, successful claim.
func TestClaimLive_SingleProjectUnchanged(t *testing.T) {
	if os.Getenv("BT_CLAIM_INTEGRATION") != "1" {
		t.Skip("set BT_CLAIM_INTEGRATION=1 to run bd-spawning claim integration tests")
	}
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("bd CLI not on PATH")
	}

	t.Setenv("BT_PROJECTS_REGISTRY_PATH", filepath.Join(t.TempDir(), "projects.json"))
	t.Setenv("BT_SETTINGS_PATH", filepath.Join(t.TempDir(), "settings.json"))

	root := t.TempDir()
	issue := firstOpenIssueViaWorker(t, bdPath, root)

	m := newSizedModelWithRoute(t, []model.Issue{issue}, 120, 32, bdroute.SingleProject(root))
	m.claimTargetID = issue.ID
	updated, cmd := m.confirmClaim()
	m = updated
	if cmd == nil {
		t.Fatalf("confirmClaim(%s) returned a nil cmd; want a real claim dispatch", issue.ID)
	}
	res := drainClaimResult(t, cmd)
	if res.result.Err != nil || res.result.ExitCode != 0 {
		t.Fatalf("claim %s failed: exit=%d err=%v stderr=%q", issue.ID, res.result.ExitCode, res.result.Err, res.result.Stderr)
	}
	assertClaimedInProject(t, root, issue.ID)
}

// assertClaimedInProject re-reads dir's project through the real
// BackgroundWorker plumbing and fails the test unless id is claimed (status
// moved off open, or an assignee is set - the same predicate writeSettled's
// writeClaim branch uses).
func assertClaimedInProject(t *testing.T, dir, id string) {
	t.Helper()
	beadsDir := filepath.Join(dir, ".beads")
	src, err := datasource.DiscoverSource(datasource.DiscoveryOptions{BeadsDir: beadsDir})
	if err != nil {
		t.Fatalf("DiscoverSource(%s): %v", dir, err)
	}
	worker, err := NewBackgroundWorker(WorkerConfig{
		BeadsDir:      beadsDir,
		DataSource:    &src,
		DebounceDelay: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewBackgroundWorker(%s): %v", dir, err)
	}
	defer worker.Stop()
	if err := worker.Start(); err != nil {
		t.Fatalf("worker.Start(%s): %v", dir, err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		worker.TriggerRefresh()
		if snap := worker.GetSnapshot(); snap != nil {
			if is, ok := snapshotIssue(snap, id); ok && isClaimed(is) {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("issue %s in %s did not settle to a claimed state", id, dir)
}
