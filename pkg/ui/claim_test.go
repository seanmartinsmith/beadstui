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
	msg := writeCmd(bdroute.WriteTarget{Dir: "some/dir"}, "zz-target", writeClaim, "", []string{"update", "zz-target", "--claim"})()
	res, ok := msg.(writeResultMsg)
	if !ok {
		t.Fatalf("writeCmd msg type = %T, want writeResultMsg", msg)
	}
	if res.id != "zz-target" {
		t.Errorf("result id = %q, want zz-target", res.id)
	}
	if res.kind != writeClaim {
		t.Errorf("result kind = %v, want writeClaim", res.kind)
	}
	want := []string{"update", "zz-target", "--claim"}
	if !slices.Equal(*got, want) {
		t.Errorf("claim argv = %v, want %v", *got, want)
	}
}

// TestClaimCmd_GlobalTargetAppendsFlag verifies the WriteTarget.Global branch
// (bt-scc35 follow-up wiring): writeCmd appends --global and runs with no
// checkout directory, rather than requiring Dir. Resolve does not produce a
// Global target yet (beads_global is refused pre-flight), so this exercises
// the writeCmd branch directly via the executor stub.
func TestClaimCmd_GlobalTargetAppendsFlag(t *testing.T) {
	var gotDir string
	got := stubClaimRunnerCapturingDir(t, bdexec.Result{ExitCode: 0}, &gotDir)
	msg := writeCmd(bdroute.WriteTarget{Global: true}, "zz-target", writeClaim, "", []string{"update", "zz-target", "--claim"})()
	if _, ok := msg.(writeResultMsg); !ok {
		t.Fatalf("writeCmd msg type = %T, want writeResultMsg", msg)
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
	if _, pending := m.pendingWrites["zz-target"]; !pending {
		t.Fatalf("zz-target not marked pending: %v", m.pendingWrites)
	}
	if m.activeModal != ModalNone {
		t.Errorf("modal still open after confirm: %v", m.activeModal)
	}
	// The pending row swaps its selection caret for the spinner frame, so the
	// current frame glyph must appear in the rendered list.
	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, claimSpinnerFrame(m.writeSpinnerIdx)) {
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
	if len(m.pendingWrites) != 0 {
		t.Errorf("cancel should not mark anything pending: %v", m.pendingWrites)
	}
	if len(*got) != 0 {
		t.Errorf("cancel should not invoke the executor; got %v", *got)
	}
}

// TestRequestClaim_RefusesWhenAlreadyPending verifies the v1 double-dispatch
// guard (bt-oiaj.13 step 1): a second write request on a row that already
// has a pending write is refused with a notice, not queued.
func TestRequestClaim_RefusesWhenAlreadyPending(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.pendingWrites["zz-target"] = pendingWrite{Kind: writeClaim, StartedAt: time.Now()}
	m.requestClaim()
	if m.activeModal != ModalNone {
		t.Errorf("activeModal = %v, want ModalNone (refused, not opened)", m.activeModal)
	}
	if m.statusSeverity != SeverityNotice {
		t.Errorf("statusSeverity = %v, want SeverityNotice", m.statusSeverity)
	}
	if !strings.Contains(m.statusMsg, "write already pending for zz-target") {
		t.Errorf("notice = %q, want it to name the pending write", m.statusMsg)
	}
}

func TestHandleClaimResult_SuccessKeepsPending(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.pendingWrites["zz-target"] = pendingWrite{Kind: writeClaim, StartedAt: time.Now()}
	m2, _ := m.handleWriteResult(writeResultMsg{
		id:     "zz-target",
		kind:   writeClaim,
		result: bdexec.Result{Args: []string{"bd", "update", "zz-target", "--claim"}, ExitCode: 0},
	})
	m = m2
	if _, pending := m.pendingWrites["zz-target"]; !pending {
		t.Error("success must keep the row pending until the watcher settles it")
	}
	if m.statusSeverity != SeveritySuccess {
		t.Errorf("statusSeverity = %v, want SeveritySuccess", m.statusSeverity)
	}
}

func TestHandleClaimResult_FailureClearsPendingWithStderr(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.pendingWrites["zz-target"] = pendingWrite{Kind: writeClaim, StartedAt: time.Now()}
	m2, _ := m.handleWriteResult(writeResultMsg{
		id:   "zz-target",
		kind: writeClaim,
		result: bdexec.Result{
			Args:     []string{"bd", "update", "zz-target", "--claim"},
			ExitCode: 1,
			Stderr:   "error: issue zz-target not found\n",
		},
	})
	m = m2
	if _, pending := m.pendingWrites["zz-target"]; pending {
		t.Error("failure must clear the pending marker")
	}
	if m.statusSeverity != SeverityFailure {
		t.Errorf("statusSeverity = %v, want SeverityFailure", m.statusSeverity)
	}
	if !strings.Contains(m.statusMsg, "not found") {
		t.Errorf("failure toast should preserve bd stderr; got %q", m.statusMsg)
	}
}

func TestSettlePendingWrites_ClaimHeuristic(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.pendingWrites["zz-target"] = pendingWrite{Kind: writeClaim, StartedAt: time.Now()}
	m.pendingWrites["zz-other"] = pendingWrite{Kind: writeClaim, StartedAt: time.Now()}

	// zz-target now reflects the claim (in_progress + assignee); zz-other is
	// still open + unassigned and must stay pending.
	m.data.issueMap["zz-target"].Status = model.StatusInProgress
	m.data.issueMap["zz-target"].Assignee = "sms"

	m.settlePendingWrites()

	if _, pending := m.pendingWrites["zz-target"]; pending {
		t.Error("claimed bead should have settled out of pending")
	}
	if _, pending := m.pendingWrites["zz-other"]; !pending {
		t.Error("still-open bead must remain pending")
	}
}

// TestSettlePendingWrites_FieldEditTargetHit exercises the writeFieldEdit
// branch (bt-oiaj.13 step 3, fork #3): unlike claim, a field edit settles by
// exact target-compare of the named field, not a status/assignee heuristic.
// No field-edit caller exists yet (bt-oiaj.5) - this proves the generalized
// mechanism is correct ahead of that consumer.
func TestSettlePendingWrites_FieldEditTargetHit(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.pendingWrites["zz-target"] = pendingWrite{
		Kind: writeFieldEdit, Field: "title", Target: "New title", StartedAt: time.Now(),
	}
	m.data.issueMap["zz-target"].Title = "New title"

	m.settlePendingWrites()

	if _, pending := m.pendingWrites["zz-target"]; pending {
		t.Error("field edit matching its target should have settled out of pending")
	}
}

// TestSettlePendingWrites_FieldEditTargetMiss verifies a field edit stays
// pending when the reloaded value does not (yet) match the captured target.
func TestSettlePendingWrites_FieldEditTargetMiss(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.pendingWrites["zz-target"] = pendingWrite{
		Kind: writeFieldEdit, Field: "title", Target: "New title", StartedAt: time.Now(),
	}
	// issueMap still has the original title ("Claim target bead") - no match.

	m.settlePendingWrites()

	if _, pending := m.pendingWrites["zz-target"]; !pending {
		t.Error("field edit not yet matching its target must remain pending")
	}
}

// TestSettlePendingWrites_TimeoutAnnunciator forces a stuck pending write
// (StartedAt far in the past, content never matches) through a settle pass
// and verifies the discrepancy annunciator fires: the marker clears, a
// Failure-severity toast names the id/field, and the events ring records it
// (bt-oiaj.13 step 4 - never silent stale state).
func TestSettlePendingWrites_TimeoutAnnunciator(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.pendingWrites["zz-target"] = pendingWrite{
		Kind: writeFieldEdit, Field: "title", Target: "unreachable value",
		StartedAt: time.Now().Add(-writeSettleTimeout - time.Second),
	}
	eventsBefore := m.events.Len()

	m.settlePendingWrites()

	if _, pending := m.pendingWrites["zz-target"]; pending {
		t.Error("timed-out write must clear the pending marker, not stay stuck forever")
	}
	if m.statusSeverity != SeverityFailure {
		t.Errorf("statusSeverity = %v, want SeverityFailure (no dedicated Warning severity exists)", m.statusSeverity)
	}
	if !strings.Contains(m.statusMsg, "zz-target") || !strings.Contains(m.statusMsg, "title") || !strings.Contains(m.statusMsg, "45s") {
		t.Errorf("timeout toast = %q, want it to name the id, field, and 45s window", m.statusMsg)
	}
	if m.events.Len() != eventsBefore+1 {
		t.Errorf("events ring len = %d, want %d (timeout must record an entry)", m.events.Len(), eventsBefore+1)
	}
}

func TestWriteSpinnerTick_SelfCancels(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)

	// No pending writes: the tick clears the active flag and stops re-arming.
	m.writeSpinnerActive = true
	m2, cmd := m.handleWriteSpinnerTick()
	m = m2
	if m.writeSpinnerActive {
		t.Error("spinner should deactivate with no pending writes")
	}
	if cmd != nil {
		t.Error("spinner should not re-arm with no pending writes")
	}

	// With a pending write: the tick advances and re-arms.
	m.pendingWrites["zz-target"] = pendingWrite{Kind: writeClaim, StartedAt: time.Now()}
	before := m.writeSpinnerIdx
	m2, cmd = m.handleWriteSpinnerTick()
	m = m2
	if m.writeSpinnerIdx != before+1 {
		t.Errorf("spinner idx = %d, want %d", m.writeSpinnerIdx, before+1)
	}
	if cmd == nil {
		t.Error("spinner should re-arm while writes are pending")
	}
}

// TestRenderClaimConfirm_PredictsNotClaimable_Closed verifies the bt-55n3s
// outcome prediction (bt-oiaj.13 step 5): a closed bead's confirm modal warns
// it is not claimable, purely from loaded state (no bd spawn).
func TestRenderClaimConfirm_PredictsNotClaimable_Closed(t *testing.T) {
	issues := claimTestIssues()
	issues[0].Status = model.StatusClosed
	m := newSizedModel(t, issues, 120, 32)
	// Closing zz-target moves it to the end of the open-first sort
	// (replaceIssues); select it explicitly rather than relying on the
	// default index-0 selection.
	if !m.selectIssueByID("zz-target") {
		t.Fatal("zz-target not found in the list after closing")
	}
	m.requestClaim()

	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "not claimable: status closed") {
		t.Errorf("confirm modal missing closed-status prediction; view:\n%s", out)
	}
}

// TestRenderClaimConfirm_PredictsNotClaimable_Tombstone mirrors the closed-bead
// case above: field_edit.go's status-picker fence (~line 480) treats closed
// AND tombstone as terminal/destructive, so the claim prediction must match
// for symmetry. Defensive-only in practice - the datasource filters
// tombstones from every load - but the sibling surface defends anyway.
func TestRenderClaimConfirm_PredictsNotClaimable_Tombstone(t *testing.T) {
	issues := claimTestIssues()
	issues[0].Status = model.StatusTombstone
	m := newSizedModel(t, issues, 120, 32)
	if !m.selectIssueByID("zz-target") {
		t.Fatal("zz-target not found in the list after tombstoning")
	}
	m.requestClaim()

	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "not claimable: status tombstone") {
		t.Errorf("confirm modal missing tombstone-status prediction; view:\n%s", out)
	}
}

// TestRenderClaimConfirm_PredictsAssignedToOther verifies the assigned-bead
// branch of the bt-55n3s matrix: bd will refuse unless the actor matches.
func TestRenderClaimConfirm_PredictsAssignedToOther(t *testing.T) {
	issues := claimTestIssues()
	issues[0].Assignee = "sms"
	m := newSizedModel(t, issues, 120, 32)
	if !m.selectIssueByID("zz-target") {
		t.Fatal("zz-target not found in the list")
	}
	m.requestClaim()

	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "assigned to sms") {
		t.Errorf("confirm modal missing assignee prediction; view:\n%s", out)
	}
}

// TestRenderClaimConfirm_PredictionScrunchedNoOverflow verifies the confirm
// modal (now up to 4 lines with the prediction row) still renders cleanly at
// the user's routine scrunched terminal size (~50x16 - project norm per
// tui-dev-rendering-sop.md), rather than only at the 120x32 the other tests
// use.
func TestRenderClaimConfirm_PredictionScrunchedNoOverflow(t *testing.T) {
	issues := claimTestIssues()
	issues[0].Assignee = "sms"
	m := newSizedModel(t, issues, 50, 16)
	if !m.selectIssueByID("zz-target") {
		t.Fatal("zz-target not found in the list")
	}
	m.requestClaim()

	out := ansi.Strip(m.View().Content)
	if !strings.Contains(out, "Claim?") {
		t.Errorf("confirm modal not rendered at scrunched size; view:\n%s", out)
	}
	if !strings.Contains(out, "assigned to sms") {
		t.Errorf("prediction line missing at scrunched size; view:\n%s", out)
	}
}

// TestRenderClaimConfirm_NoWarningForOpenUnassigned is the control: the
// common case (open + unassigned) must render no warning line at all.
func TestRenderClaimConfirm_NoWarningForOpenUnassigned(t *testing.T) {
	m := newSizedModel(t, claimTestIssues(), 120, 32)
	m.requestClaim()

	out := ansi.Strip(m.View().Content)
	if strings.Contains(out, "not claimable") || strings.Contains(out, "assigned to") {
		t.Errorf("open+unassigned bead should render no prediction warning; view:\n%s", out)
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

	if _, pending := fm.pendingWrites["zz-target"]; pending {
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
