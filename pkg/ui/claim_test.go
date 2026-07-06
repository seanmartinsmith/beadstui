package ui

// Tests for the claim-first vertical write slice (bt-oiaj.10): the keypath
// (keypress -> confirm -> executor -> pending render) plus the result and
// settle handlers. The executor runs through the claimRunner seam, stubbed here
// so no bd process is spawned. The live bd end-to-end path is in
// claim_integration_test.go (gated on BT_CLAIM_INTEGRATION=1).

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/seanmartinsmith/beadstui/internal/bdexec"
	"github.com/seanmartinsmith/beadstui/internal/bdroute"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func claimTestIssues() []model.Issue {
	// Synthetic "zz" prefix; the executor is stubbed regardless of routing.
	return []model.Issue{
		{ID: "zz-target", Title: "Claim target bead", Status: model.StatusOpen, Priority: 0},
		{ID: "zz-other", Title: "Another open bead", Status: model.StatusOpen, Priority: 1},
	}
}

// newSizedModel builds a sized Model with a working single-project route
// table (bt-scc35): every issue ID resolves to the same tempdir, regardless
// of prefix, so pre-bt-scc35 tests that don't care about routing keep working
// unchanged. Tests that DO care about routing (refusal, workspace/global
// mapping) use newSizedModelWithRoute directly.
func newSizedModel(t *testing.T, issues []model.Issue, w, h int) Model {
	t.Helper()
	return newSizedModelWithRoute(t, issues, w, h, bdroute.SingleProject(t.TempDir()))
}

func newSizedModelWithRoute(t *testing.T, issues []model.Issue, w, h int, table *bdroute.Table) Model {
	t.Helper()
	m := NewModel(issues, nil, "", nil, table)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return updated.(Model)
}

// stubClaimRunner swaps in a runner that records the argv and returns res,
// restoring the original on cleanup.
func stubClaimRunner(t *testing.T, res bdexec.Result) *[]string {
	t.Helper()
	var got []string
	orig := claimRunner
	claimRunner = func(ctx context.Context, dir string, args ...string) bdexec.Result {
		got = append([]string{}, args...)
		return res
	}
	t.Cleanup(func() { claimRunner = orig })
	return &got
}

// stubClaimRunnerCapturingDir is stubClaimRunner plus capturing the dir
// argument, so a WriteTarget.Global branch (which passes "" instead of a
// checkout path) can be asserted directly.
func stubClaimRunnerCapturingDir(t *testing.T, res bdexec.Result, gotDir *string) *[]string {
	t.Helper()
	var got []string
	orig := claimRunner
	claimRunner = func(ctx context.Context, dir string, args ...string) bdexec.Result {
		*gotDir = dir
		got = append([]string{}, args...)
		return res
	}
	t.Cleanup(func() { claimRunner = orig })
	return &got
}

func TestClaimCmd_BuildsClaimArgv(t *testing.T) {
	got := stubClaimRunner(t, bdexec.Result{ExitCode: 0})
	msg := claimCmd(bdroute.WriteTarget{Dir: "some/dir"}, "zz-target")()
	res, ok := msg.(claimResultMsg)
	if !ok {
		t.Fatalf("claimCmd msg type = %T, want claimResultMsg", msg)
	}
	if res.id != "zz-target" {
		t.Errorf("result id = %q, want zz-target", res.id)
	}
	want := []string{"update", "zz-target", "--claim"}
	if !slices.Equal(*got, want) {
		t.Errorf("claim argv = %v, want %v", *got, want)
	}
}

// TestClaimCmd_GlobalTargetAppendsFlag verifies the WriteTarget.Global branch
// (bt-scc35 follow-up wiring): claimCmd appends --global and runs with no
// checkout directory, rather than requiring Dir. Resolve does not produce a
// Global target yet (beads_global is refused pre-flight), so this exercises
// the claimCmd branch directly via the executor stub.
func TestClaimCmd_GlobalTargetAppendsFlag(t *testing.T) {
	var gotDir string
	got := stubClaimRunnerCapturingDir(t, bdexec.Result{ExitCode: 0}, &gotDir)
	msg := claimCmd(bdroute.WriteTarget{Global: true}, "zz-target")()
	if _, ok := msg.(claimResultMsg); !ok {
		t.Fatalf("claimCmd msg type = %T, want claimResultMsg", msg)
	}
	want := []string{"update", "zz-target", "--claim", "--global"}
	if !slices.Equal(*got, want) {
		t.Errorf("claim argv = %v, want %v", *got, want)
	}
	if gotDir != "" {
		t.Errorf("global target dir = %q, want empty (no checkout needed)", gotDir)
	}
}

func TestRequestClaim_OpensConfirmForSelected(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.requestClaim()
	if m.activeModal != ModalClaimConfirm {
		t.Fatalf("activeModal = %v, want ModalClaimConfirm", m.activeModal)
	}
	if m.claimTargetID != "zz-target" {
		t.Errorf("claimTargetID = %q, want zz-target", m.claimTargetID)
	}
	if m.focused != focusClaimConfirm {
		t.Errorf("focused = %v, want focusClaimConfirm", m.focused)
	}
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Claim?") || !strings.Contains(out, "zz-target") {
		t.Errorf("confirm modal not rendered; view:\n%s", out)
	}
}

func TestConfirmClaim_MarksPendingAndRendersSpinner(t *testing.T) {
	stubClaimRunner(t, bdexec.Result{ExitCode: 0})
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.requestClaim()
	m2, cmd := m.confirmClaim()
	m = m2
	if cmd == nil {
		t.Fatal("confirmClaim returned nil cmd (expected claim dispatch)")
	}
	if !m.pendingClaims["zz-target"] {
		t.Fatalf("zz-target not marked pending: %v", m.pendingClaims)
	}
	if m.activeModal != ModalNone {
		t.Errorf("modal still open after confirm: %v", m.activeModal)
	}
	// The pending row swaps its selection caret for the spinner frame, so the
	// current frame glyph must appear in the rendered list.
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, claimSpinnerFrame(m.claimSpinnerIdx)) {
		t.Errorf("pending spinner not rendered in list; view:\n%s", out)
	}
}

func TestCancelClaim_ClosesWithoutDispatch(t *testing.T) {
	got := stubClaimRunner(t, bdexec.Result{ExitCode: 0})
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.requestClaim()
	m.cancelClaim()
	if m.activeModal != ModalNone {
		t.Errorf("modal still open after cancel: %v", m.activeModal)
	}
	if len(m.pendingClaims) != 0 {
		t.Errorf("cancel should not mark anything pending: %v", m.pendingClaims)
	}
	if len(*got) != 0 {
		t.Errorf("cancel should not invoke the executor; got %v", *got)
	}
}

func TestHandleClaimResult_SuccessKeepsPending(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.pendingClaims["zz-target"] = true
	m2, _ := m.handleClaimResult(claimResultMsg{
		id:     "zz-target",
		result: bdexec.Result{Args: []string{"bd", "update", "zz-target", "--claim"}, ExitCode: 0},
	})
	m = m2
	if !m.pendingClaims["zz-target"] {
		t.Error("success must keep the row pending until the watcher settles it")
	}
	if m.statusSeverity != SeveritySuccess {
		t.Errorf("statusSeverity = %v, want SeveritySuccess", m.statusSeverity)
	}
}

func TestHandleClaimResult_FailureClearsPendingWithStderr(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.pendingClaims["zz-target"] = true
	m2, _ := m.handleClaimResult(claimResultMsg{
		id: "zz-target",
		result: bdexec.Result{
			Args:     []string{"bd", "update", "zz-target", "--claim"},
			ExitCode: 1,
			Stderr:   "error: issue zz-target not found\n",
		},
	})
	m = m2
	if m.pendingClaims["zz-target"] {
		t.Error("failure must clear the pending marker")
	}
	if m.statusSeverity != SeverityFailure {
		t.Errorf("statusSeverity = %v, want SeverityFailure", m.statusSeverity)
	}
	if !strings.Contains(m.statusMsg, "not found") {
		t.Errorf("failure toast should preserve bd stderr; got %q", m.statusMsg)
	}
}

func TestSettlePendingClaims(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.pendingClaims["zz-target"] = true
	m.pendingClaims["zz-other"] = true

	// zz-target now reflects the claim (in_progress + assignee); zz-other is
	// still open + unassigned and must stay pending.
	m.data.issueMap["zz-target"].Status = model.StatusInProgress
	m.data.issueMap["zz-target"].Assignee = "sms"

	m.settlePendingClaims()

	if m.pendingClaims["zz-target"] {
		t.Error("claimed bead should have settled out of pending")
	}
	if !m.pendingClaims["zz-other"] {
		t.Error("still-open bead must remain pending")
	}
}

func TestClaimSpinnerTick_SelfCancels(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)

	// No pending claims: the tick clears the active flag and stops re-arming.
	m.claimSpinnerActive = true
	m2, cmd := m.handleClaimSpinnerTick()
	m = m2
	if m.claimSpinnerActive {
		t.Error("spinner should deactivate with no pending claims")
	}
	if cmd != nil {
		t.Error("spinner should not re-arm with no pending claims")
	}

	// With a pending claim: the tick advances and re-arms.
	m.pendingClaims["zz-target"] = true
	before := m.claimSpinnerIdx
	m2, cmd = m.handleClaimSpinnerTick()
	m = m2
	if m.claimSpinnerIdx != before+1 {
		t.Errorf("spinner idx = %d, want %d", m.claimSpinnerIdx, before+1)
	}
	if cmd == nil {
		t.Error("spinner should re-arm while claims are pending")
	}
}

// TestClaimKeypath_Teatest drives the full slice through the real Bubble Tea
// event loop: press the claim key -> confirm -> the executor is invoked with
// the exact claim argv -> the pending row renders. The executor is stubbed so
// no bd is spawned.
func TestClaimKeypath_Teatest(t *testing.T) {
	invoked := make(chan []string, 1)
	orig := claimRunner
	claimRunner = func(ctx context.Context, dir string, args ...string) bdexec.Result {
		invoked <- append([]string{}, args...)
		return bdexec.Result{Args: append([]string{"bd"}, args...), ExitCode: 0}
	}
	t.Cleanup(func() { claimRunner = orig })

	m := NewModel(claimTestIssues(), nil, "", nil, bdroute.SingleProject(t.TempDir()))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 32))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("zz-target"))
	}, teatest.WithDuration(8*time.Second))

	// Open the confirm modal.
	tm.Send(tea.KeyPressMsg{Code: 'm', Text: "m"})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Claim?"))
	}, teatest.WithDuration(5*time.Second))

	// Accept it.
	tm.Send(tea.KeyPressMsg{Code: 'y', Text: "y"})

	select {
	case args := <-invoked:
		want := []string{"update", "zz-target", "--claim"}
		if !slices.Equal(args, want) {
			t.Fatalf("executor argv = %v, want %v", args, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("claim executor was not invoked after confirm")
	}

	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(15*time.Second))
}

// TestClaimKeypath_RefusalTeatest drives the same real event loop as
// TestClaimKeypath_Teatest, but with a route table that cannot map the
// selected bead (an empty workspace mapping). Resolve must refuse BEFORE any
// bd invocation (bt-scc35): the executor stub records zero invocations, and
// the refusal surfaces as a failure toast rather than a pending row.
func TestClaimKeypath_RefusalTeatest(t *testing.T) {
	invoked := make(chan []string, 1)
	orig := claimRunner
	claimRunner = func(ctx context.Context, dir string, args ...string) bdexec.Result {
		invoked <- append([]string{}, args...)
		return bdexec.Result{Args: append([]string{"bd"}, args...), ExitCode: 0}
	}
	t.Cleanup(func() { claimRunner = orig })

	// An empty workspace table maps no prefixes at all - any claim in this
	// model is unmappable.
	m := NewModel(claimTestIssues(), nil, "", nil, bdroute.FromWorkspace(nil))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 32))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("zz-target"))
	}, teatest.WithDuration(8*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'm', Text: "m"})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Claim?"))
	}, teatest.WithDuration(5*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'y', Text: "y"})

	tm.Quit()
	final := tm.FinalModel(t, teatest.WithFinalTimeout(15*time.Second))
	fm, ok := final.(Model)
	if !ok {
		t.Fatalf("final model type = %T, want Model", final)
	}

	if fm.pendingClaims["zz-target"] {
		t.Error("a refused claim must never enter the pending state")
	}
	if fm.statusSeverity != SeverityFailure {
		t.Errorf("statusSeverity = %v, want SeverityFailure", fm.statusSeverity)
	}
	if !strings.Contains(fm.statusMsg, "no workspace mapping") {
		t.Errorf("refusal toast = %q, want it to name the missing workspace mapping", fm.statusMsg)
	}

	select {
	case args := <-invoked:
		t.Fatalf("claim executor was invoked despite a pre-flight refusal; argv=%v", args)
	default:
		// Expected: zero invocations.
	}
}
