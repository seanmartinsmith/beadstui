package ui

// claim.go implements the claim-first vertical write slice (bt-oiaj.10): the
// first bt write to ship end to end. A keybind on the selected bead opens a
// k9s-style confirm (no free-text input anywhere - Windows cp1252 safety),
// which shells out `bd update <id> --claim` through the bdexec executor. The
// row enters a pending (spinner) state immediately; the existing
// content-comparing manifest watcher's reload settles it exactly as for an
// external bd write (bt-chbqq).
//
// bt-oiaj.13 generalizes the pending/settled machinery introduced here into a
// write-kind-agnostic mechanism (pendingWrite / writeCmd / writeResultMsg)
// with a 45s timeout discrepancy annunciator, and claim migrates onto it as
// the reference consumer. Field edits (bt-oiaj.5) and long-form modals
// (bt-oiaj.6) are the next consumers of the SAME mechanism - see
// docs/plans/2026-07-07-bt-edits-wave-oiaj13-5-6.md. Deliberately still out
// of scope here: the always-on receipts pane (bt-oiaj.11), formal --readonly
// gating (bt-oiaj.12), and the write-actor identity convention (bt-oiaj.14).

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/seanmartinsmith/beadstui/internal/bdexec"
	"github.com/seanmartinsmith/beadstui/internal/bdroute"
	"github.com/seanmartinsmith/beadstui/pkg/debug"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// writeKind identifies which operation a pendingWrite/writeResultMsg
// represents. Each kind carries its own settle predicate (writeSettled) and
// its own toast wording (writeKindVerb) - see bt-oiaj.13 fork #3: claim
// cannot target-compare assignee (bt can't predict the actor string bd will
// resolve), so it keeps its shipped heuristic verbatim instead of unifying
// with the field-edit target-compare predicate.
type writeKind int

const (
	writeClaim writeKind = iota
	writeFieldEdit
)

// pendingWrite tracks one in-flight write (bt-oiaj.13). Field/Target are
// empty for writeClaim (fork #3, above); a writeFieldEdit entry (bt-oiaj.5/.6
// callers, not yet wired in this slice) carries the field name and the
// canonical string form of the value the write is expected to settle to.
//
// v1 simplification: one pending write per issue, keyed by ID. A second
// write request on a row that already has a pending write is refused with a
// notice rather than queued - see requestClaim.
type pendingWrite struct {
	Kind      writeKind
	Field     string
	Target    string
	StartedAt time.Time
}

// label names pw for the timeout annunciator's toast copy (settlePendingWrites).
func (pw pendingWrite) label() string {
	if pw.Field != "" {
		return pw.Field
	}
	if pw.Kind == writeClaim {
		return "claim"
	}
	return "write"
}

// writeResultMsg carries the outcome of a bd write shell-out back onto the
// event loop. Generalizes claimResultMsg (bt-oiaj.10) to carry the write's
// kind and field so handleWriteResult can build kind-appropriate toast copy
// without consulting the pendingWrites map.
type writeResultMsg struct {
	id     string
	kind   writeKind
	field  string
	result bdexec.Result
}

// writeSpinnerTickMsg advances the pending-write row spinner. It self-cancels
// once no writes are pending (handleWriteSpinnerTick).
type writeSpinnerTickMsg struct{}

// claimRunner is the seam the executor runs through. Production wires it to
// bdexec.Run; tests swap in a stub so the keypath is exercised without
// spawning bd. Already write-kind-agnostic (it just runs bd with the argv
// it's given): claim is the only caller in this slice, but bt-oiaj.5/.6 field
// edits share this exact seam rather than inventing their own.
var claimRunner = func(ctx context.Context, dir string, args ...string) bdexec.Result {
	return bdexec.Run(ctx, dir, args...)
}

// writeSpinnerInterval paces the pending-row spinner. Matches the worker
// spinner cadence so concurrent indicators animate in lockstep.
const writeSpinnerInterval = 150 * time.Millisecond

// writeSettleTimeout is how long a pending write may go unconfirmed before
// settlePendingWrites surfaces a discrepancy annunciator and clears the
// marker (bt-oiaj.13: never silent stale state).
const writeSettleTimeout = 45 * time.Second

// claimSpinnerFrame returns the current 1-cell braille frame. It reuses the
// worker spinner frames (already 1 column wide) so it drops into the
// delegate's 2-cell selection slot without shifting the row layout.
func claimSpinnerFrame(idx int) string {
	return workerSpinnerFrames[idx%len(workerSpinnerFrames)]
}

func writeSpinnerTickCmd() tea.Cmd {
	return tea.Tick(writeSpinnerInterval, func(time.Time) tea.Msg {
		return writeSpinnerTickMsg{}
	})
}

// writeCmd shells out a bd write command against target and reports the
// result via writeResultMsg. Generalizes claimCmd (bt-oiaj.10): claim passes
// args = ["update", id, "--claim"], kind = writeClaim, field = ""; future
// field-edit callers (bt-oiaj.5/.6) pass their own argv/kind/field. A Global
// target routes via `bd --global` instead of a checkout directory
// (WriteTarget.Global; the follow-up bt-scc35 designed but did not implement
// full `bd --global` write dispatch - Resolve currently refuses beads_global
// before a Global target can reach here, but the branch is wired and
// unit-tested so that follow-up is a routing-table change only). The command
// builder is intentionally tiny; the canonical bd command-builder package
// (bt-s5zgk.1) is a later extraction from this live write.
func writeCmd(target bdroute.WriteTarget, id string, kind writeKind, field string, args []string) tea.Cmd {
	return func() tea.Msg {
		dir := target.Dir
		finalArgs := args
		if target.Global {
			finalArgs = append(append([]string{}, args...), "--global")
			dir = ""
		}
		res := claimRunner(context.Background(), dir, finalArgs...)
		return writeResultMsg{id: id, kind: kind, field: field, result: res}
	}
}

// requestClaim opens the confirm modal for the currently selected bead. It is
// a no-op with a notice when nothing is selected or a write is already
// pending for that bead (double-dispatch guard; v1 simplification - one
// pending write per issue, see pendingWrite doc).
func (m *Model) requestClaim() {
	sel, ok := m.list.SelectedItem().(IssueItem)
	if !ok {
		m.setNotice("No issue selected")
		return
	}
	if _, pending := m.pendingWrites[sel.Issue.ID]; pending {
		m.setNotice(fmt.Sprintf("write already pending for %s", sel.Issue.ID))
		return
	}
	m.claimTargetID = sel.Issue.ID
	m.claimTargetTitle = sel.Issue.Title
	m.openModal(ModalClaimConfirm)
	m.focused = focusClaimConfirm
}

// confirmClaim fires the claim for the confirm target, marks the row pending,
// and starts the spinner. Called when the user accepts the confirm modal.
//
// Routing is a pre-flight step on the main goroutine (bt-scc35): Resolve is
// consulted BEFORE any bd invocation, and a non-nil error is a refusal - the
// row never enters pending, the executor is never dispatched, and the user
// sees an actionable failure toast via the existing setFailure path.
func (m Model) confirmClaim() (Model, tea.Cmd) {
	id := m.claimTargetID
	m.claimTargetID = ""
	m.claimTargetTitle = ""
	m.closeModal()
	m.focused = focusList
	if id == "" {
		return m, nil
	}

	iss, ok := m.data.issueMap[id]
	if !ok {
		m.setFailure(fmt.Sprintf("Claim %s refused: issue not found in loaded data", id))
		return m, nil
	}

	target, err := m.routeTable.Resolve(*iss)
	if err != nil {
		m.setFailure(err.Error())
		return m, nil
	}

	if m.pendingWrites == nil {
		m.pendingWrites = make(map[string]pendingWrite)
	}
	m.pendingWrites[id] = pendingWrite{Kind: writeClaim, StartedAt: time.Now()}
	m.updateListDelegate()
	m.setNotice(fmt.Sprintf("Claiming %s...", id))

	cmds := []tea.Cmd{writeCmd(target, id, writeClaim, "", []string{"update", id, "--claim"})}
	if !m.writeSpinnerActive {
		m.writeSpinnerActive = true
		cmds = append(cmds, writeSpinnerTickCmd())
	}
	return m, tea.Batch(cmds...)
}

// cancelClaim closes the confirm modal without firing (any key but y/enter).
func (m *Model) cancelClaim() {
	m.claimTargetID = ""
	m.claimTargetTitle = ""
	m.closeModal()
	m.focused = focusList
	m.setNotice("Claim cancelled")
}

// writeKindVerb returns the past-tense and present-tense verbs bt uses in
// status/failure toast copy for kind. Claim keeps its established
// "Claimed"/"Claim" wording (bt-55n3s toast taxonomy: claim copy is
// deliberately distinct from generic write copy); field-edit wording is
// bt-oiaj.5/.6's to refine when the first field-edit caller lands.
func writeKindVerb(kind writeKind) (past, present string) {
	if kind == writeClaim {
		return "Claimed", "Claim"
	}
	return "Updated", "Update"
}

// handleWriteResult records the shell-out trace and surfaces success/failure.
// Generalizes handleClaimResult (bt-oiaj.10) to any write kind. On failure
// the row leaves pending immediately with bd's stderr preserved; on success
// the row stays pending until the manifest watcher's reload settles it
// (settlePendingWrites). The always-on receipts pane is bt-oiaj.11; the
// timeout + discrepancy annunciator are implemented in settlePendingWrites
// (bt-oiaj.13).
func (m Model) handleWriteResult(msg writeResultMsg) (Model, tea.Cmd) {
	res := msg.result
	// Observable trace: exact argv + exit code (pre-receipts, gated on BT_DEBUG).
	debug.Log("write: %s -> exit %d (%v)", res.Argv(), res.ExitCode, res.Duration)

	if res.Err != nil || res.ExitCode != 0 {
		delete(m.pendingWrites, msg.id)
		m.updateListDelegate()
		m.setFailure(writeFailureMessage(msg.id, msg.kind, res))
		return m, nil
	}
	past, _ := writeKindVerb(msg.kind)
	m.setStatus(fmt.Sprintf("%s %s; awaiting refresh", past, msg.id))
	// A successful long-form write retires the session draft for this
	// (issue, field): reopening the editor must show the (soon-to-reload)
	// new field value, not the stale pre-commit draft (bt-oiaj.6 draft
	// cache - see longform_edit.go). No-op for claim and for the
	// single-line fields, which never populate the cache.
	if msg.kind == writeFieldEdit {
		m.clearLongformDraft(msg.id, msg.field)
	}
	return m, nil
}

// writeFailureMessage builds a one-line failure toast, preferring the last
// non-empty line of bd stderr so the user sees bd's own reason verbatim.
// Generalizes claimFailureMessage (bt-oiaj.10).
func writeFailureMessage(id string, kind writeKind, res bdexec.Result) string {
	detail := lastNonEmptyLine(res.Stderr)
	if detail == "" {
		detail = lastNonEmptyLine(res.Stdout)
	}
	if detail == "" && res.Err != nil {
		detail = res.Err.Error()
	}
	if detail == "" {
		detail = fmt.Sprintf("exit %d", res.ExitCode)
	}
	_, present := writeKindVerb(kind)
	return fmt.Sprintf("%s %s failed: %s", present, id, detail)
}

// lastNonEmptyLine returns the last non-blank line of s, trimmed.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}

// handleWriteSpinnerTick advances the pending-row spinner, re-arming itself
// while writes remain pending and self-cancelling otherwise. Generalizes
// handleClaimSpinnerTick (bt-oiaj.10).
func (m Model) handleWriteSpinnerTick() (Model, tea.Cmd) {
	if len(m.pendingWrites) == 0 {
		m.writeSpinnerActive = false
		return m, nil
	}
	m.writeSpinnerIdx++
	m.updateListDelegate()
	return m, writeSpinnerTickCmd()
}

// pendingWriteIDs derives a presence-only view of pendingWrites for the list
// delegate, which only needs to know WHETHER a row has a pending write (not
// its kind/field) to swap in the spinner glyph (IssueDelegate.PendingClaims).
func (m *Model) pendingWriteIDs() map[string]bool {
	if len(m.pendingWrites) == 0 {
		return nil
	}
	ids := make(map[string]bool, len(m.pendingWrites))
	for id := range m.pendingWrites {
		ids[id] = true
	}
	return ids
}

// writeSettled reports whether iss's current (reloaded) state confirms pw,
// per its Kind's op-specific predicate (bt-oiaj.13 fork #3):
//
//   - writeClaim keeps the shipped heuristic verbatim (bt-oiaj.10): a claim
//     moves the bead off open and/or sets an assignee. bt cannot predict the
//     actor string bd will resolve, so claim cannot target-compare assignee.
//   - writeFieldEdit target-compares the named field's canonical string
//     (captured at write time as pw.Target) against its current value - EXCEPT
//     the "comment" field (bt-oiaj.6/Slice C step 5), which has no scalar
//     Issue field to compare against (bd comments add appends to a list, not
//     a replaceable value). Comment callers always register Target == "" and
//     settle on the very next reload after a successful write, unconditionally
//     - there is nothing more precise to check without re-reading bd's
//     comment list. This is a third, explicit predicate case (not the
//     fieldValue default-"" fallthrough the comment above this used to rely
//     on implicitly - see fieldValue's doc comment for why that was wrong).
func writeSettled(pw pendingWrite, iss *model.Issue) bool {
	switch pw.Kind {
	case writeClaim:
		return iss.Status != model.StatusOpen || iss.Assignee != ""
	case writeFieldEdit:
		if pw.Field == "comment" {
			return true
		}
		return fieldValue(iss, pw.Field) == pw.Target
	default:
		return false
	}
}

// fieldValue returns the canonical string form of the named field on iss, for
// settle-time target-compare. bt-oiaj.5 wires status/priority/title/assignee;
// bt-oiaj.6 (Slice C) adds description/design/acceptance/notes - all five are
// full-replace fields where the write's Target is the new value verbatim, so
// an exact string compare against the reloaded field is meaningful. "comment"
// deliberately has NO case here: bd comments add appends to a list rather
// than replacing a scalar, so there is no canonical string to target-compare
// against - see writeSettled's explicit third predicate case instead, which
// settles unconditionally on the next reload rather than consulting this
// function's default branch.
//
// CORRECTION (bt-oiaj.6 review): this doc comment previously claimed unknown
// field names "can only settle via the writeSettleTimeout path" - that was
// wrong. The default branch below returns "", so any caller that registered a
// pendingWrite with an unmapped Field AND an empty Target (""=="") would have
// settled on the very first reload, not the 45s timeout - the timeout only
// fires for an unmapped field with a NON-empty Target. This never surfaced as
// a bug because every writeFieldEdit caller before this slice used one of the
// four mapped fields below. Slice C's comment field is exactly the "unmapped
// field, empty Target" shape the old comment mis-described - which is why it
// now gets an EXPLICIT case in writeSettled instead of being left to fall
// through here implicitly. Any future field-edit caller adding a new field
// name should add a real case below, not rely on this "" fallthrough.
func fieldValue(iss *model.Issue, field string) string {
	switch field {
	case "status":
		return string(iss.Status)
	case "priority":
		return strconv.Itoa(iss.Priority)
	case "title":
		return iss.Title
	case "assignee":
		return iss.Assignee
	case "description":
		return iss.Description
	case "design":
		return iss.Design
	case "acceptance":
		return iss.AcceptanceCriteria
	case "notes":
		return iss.Notes
	default:
		return ""
	}
}

// settlePendingWrites clears pending-write markers for issues whose reloaded
// state confirms the write (writeSettled), and surfaces a discrepancy
// annunciator for any write that has not settled within writeSettleTimeout -
// including one whose issue is missing from the reloaded set entirely (e.g.
// filtered out of a cross-project scope) - so no write is ever left silently
// pending forever (bt-oiaj.13: never silent stale state). Generalizes
// settlePendingClaims (bt-oiaj.10); called from the same two reload sites in
// model_update_data.go (handleSnapshotReady, handleDataSourceReload).
func (m *Model) settlePendingWrites() {
	if len(m.pendingWrites) == 0 {
		return
	}
	changed := false
	now := time.Now()
	for id, pw := range m.pendingWrites {
		if iss, ok := m.data.issueMap[id]; ok && writeSettled(pw, iss) {
			delete(m.pendingWrites, id)
			changed = true
			continue
		}
		if now.Sub(pw.StartedAt) >= writeSettleTimeout {
			delete(m.pendingWrites, id)
			changed = true
			// No dedicated "warning" StatusSeverity exists (only
			// Success/Notice/Failure/Degraded - see model_footer.go); Failure
			// is the closest fit (auto-dismisses, records an events ring
			// entry) and is reused here per the plan's fallback (noted in
			// the PR description).
			m.setFailure(fmt.Sprintf(
				"write to %s (%s) not confirmed after %s - refresh or check bd",
				id, pw.label(), writeSettleTimeout))
		}
	}
	if changed {
		m.updateListDelegate()
	}
}

// predictClaimOutcome returns a WARN-only outcome line for the claim confirm
// modal, built entirely from already-loaded issue state (bt-55n3s's
// empirical claim matrix) - zero bd spawns, never a refusal. Empty string
// means no warning (the common case: open + unassigned). bd remains the
// actual source of truth (don't-trust-verify); y/enter always proceeds
// regardless of this line.
func predictClaimOutcome(iss *model.Issue) string {
	if iss == nil {
		return ""
	}
	switch iss.Status {
	case model.StatusClosed, model.StatusBlocked:
		return fmt.Sprintf("not claimable: status %s", iss.Status)
	}
	if iss.Assignee != "" {
		return fmt.Sprintf("assigned to %s - bd will refuse unless that's your actor", iss.Assignee)
	}
	return ""
}

// renderClaimConfirm returns the claim-confirm modal panel. View() composites
// it via OverlayCenterDimBackdrop so the backdrop dims uniformly with the other
// modals (tui-modal-compositing.md). Mirrors renderQuitConfirm.
func (m Model) renderClaimConfirm() string {
	t := m.theme

	textStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
	keyStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	idStyle := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(t.Warning)

	// Cap the panel to the terminal so it never overflows at scrunched widths
	// (the user routinely runs 50-70 columns). Reserve a margin for borders +
	// the overlay's centering.
	maxInner := m.width - 8
	if maxInner < 16 {
		maxInner = 16
	}
	titleMax := maxInner
	if titleMax > 48 {
		titleMax = 48
	}

	lineWhat := textStyle.Render("Claim ") + idStyle.Render(m.claimTargetID)
	lineTitle := textStyle.Render(truncateRunesHelper(m.claimTargetTitle, titleMax, "..."))
	lineConfirm := keyStyle.Render("y") + textStyle.Render("/") + keyStyle.Render("enter") +
		textStyle.Render(" confirm    ") + keyStyle.Render("esc") + textStyle.Render(" cancel")

	lines := []string{lineWhat, lineTitle}
	// Outcome prediction (bt-55n3s matrix, bt-oiaj.13 step 5): WARN only,
	// from data bt already holds in m.data.issueMap - never a refusal, never
	// a bd spawn. Empty prediction adds no line.
	if prediction := predictClaimOutcome(m.data.issueMap[m.claimTargetID]); prediction != "" {
		lines = append(lines, warnStyle.Render(truncateRunesHelper(prediction, titleMax, "...")))
	}
	lines = append(lines, lineConfirm)

	innerW := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > innerW {
			innerW = w
		}
	}
	if innerW > maxInner {
		innerW = maxInner
	}

	const sidePad = 4
	panelWidth := innerW + 2 + sidePad
	pad := strings.Repeat(" ", sidePad/2)
	var body strings.Builder
	for i, l := range lines {
		if i > 0 {
			body.WriteString("\n")
		}
		body.WriteString(pad + centerLine(l, innerW) + pad)
	}
	content := "\n" + body.String() + "\n"

	return RenderTitledPanel(content, PanelOpts{
		Title:       "Claim?",
		Width:       panelWidth,
		CenterTitle: true,
		BorderColor: t.Primary,
		TitleColor:  t.Primary,
		Focused:     true,
	})
}
