package ui

// claim.go implements the claim-first vertical write slice (bt-oiaj.10): the
// first bt write to ship end to end. A keybind on the selected bead opens a
// k9s-style confirm (no free-text input anywhere - Windows cp1252 safety),
// which shells out `bd update <id> --claim` through the bdexec executor. The
// row enters a pending (spinner) state immediately; the existing
// content-comparing manifest watcher's reload settles it exactly as for an
// external bd write (bt-chbqq).
//
// Deliberately out of scope here (boundary beads): the always-on receipts pane
// (bt-oiaj.11), formal --readonly gating (bt-oiaj.12), full pending/settled
// semantics with a timeout + discrepancy annunciator (bt-oiaj.13), and the
// write-actor identity convention (bt-oiaj.14). This slice ships spinner-only
// pending and a debug-log trace of the argv + exit code.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/seanmartinsmith/beadstui/internal/bdexec"
	"github.com/seanmartinsmith/beadstui/pkg/debug"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/projects"
)

// claimResultMsg carries the outcome of a `bd update <id> --claim` shell-out
// back onto the event loop.
type claimResultMsg struct {
	id     string
	result bdexec.Result
}

// claimSpinnerTickMsg advances the pending-claim row spinner. It self-cancels
// once no claims are pending (handleClaimSpinnerTick).
type claimSpinnerTickMsg struct{}

// claimRunner is the seam the executor runs through. Production wires it to
// bdexec.Run; tests swap in a stub so the keypath is exercised without
// spawning bd.
var claimRunner = func(ctx context.Context, dir string, args ...string) bdexec.Result {
	return bdexec.Run(ctx, dir, args...)
}

// claimSpinnerInterval paces the pending-row spinner. Matches the worker
// spinner cadence so concurrent indicators animate in lockstep.
const claimSpinnerInterval = 150 * time.Millisecond

// claimSpinnerFrame returns the current 1-cell braille frame. It reuses the
// worker spinner frames (already 1 column wide) so it drops into the
// delegate's 2-cell selection slot without shifting the row layout.
func claimSpinnerFrame(idx int) string {
	return workerSpinnerFrames[idx%len(workerSpinnerFrames)]
}

func claimSpinnerTickCmd() tea.Cmd {
	return tea.Tick(claimSpinnerInterval, func(time.Time) tea.Msg {
		return claimSpinnerTickMsg{}
	})
}

// claimCmd shells out `bd update <id> --claim` in dir and reports the result.
// The command builder is intentionally tiny; the canonical bd command-builder
// package (bt-s5zgk.1) is a later extraction from this live write.
func claimCmd(dir, id string) tea.Cmd {
	return func() tea.Msg {
		res := claimRunner(context.Background(), dir, "update", id, "--claim")
		return claimResultMsg{id: id, result: res}
	}
}

// requestClaim opens the confirm modal for the currently selected bead. It is a
// no-op with a notice when nothing is selected or a claim is already pending
// for that bead (double-dispatch guard).
func (m *Model) requestClaim() {
	sel, ok := m.list.SelectedItem().(IssueItem)
	if !ok {
		m.setNotice("No issue selected")
		return
	}
	if m.pendingClaims[sel.Issue.ID] {
		m.setNotice(fmt.Sprintf("Claim already pending for %s", sel.Issue.ID))
		return
	}
	m.claimTargetID = sel.Issue.ID
	m.claimTargetTitle = sel.Issue.Title
	m.openModal(ModalClaimConfirm)
	m.focused = focusClaimConfirm
}

// confirmClaim fires the claim for the confirm target, marks the row pending,
// and starts the spinner. Called when the user accepts the confirm modal.
func (m Model) confirmClaim() (Model, tea.Cmd) {
	id := m.claimTargetID
	m.claimTargetID = ""
	m.claimTargetTitle = ""
	m.closeModal()
	m.focused = focusList
	if id == "" {
		return m, nil
	}

	// Resolve the target project directory on the main goroutine (registry
	// lookup + os.Getwd are main-goroutine reads, mirroring resolveHistoryPath).
	dir := m.resolveClaimDir(id)

	if m.pendingClaims == nil {
		m.pendingClaims = make(map[string]bool)
	}
	m.pendingClaims[id] = true
	m.updateListDelegate()
	m.setNotice(fmt.Sprintf("Claiming %s...", id))

	cmds := []tea.Cmd{claimCmd(dir, id)}
	if !m.claimSpinnerActive {
		m.claimSpinnerActive = true
		cmds = append(cmds, claimSpinnerTickCmd())
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

// resolveClaimDir picks the directory the bd claim runs in. Prefer the bead's
// project via the registry (so claim works in workspace/global mode), then the
// bt working directory, then the process cwd. Mirrors resolveHistoryPath.
func (m Model) resolveClaimDir(id string) string {
	if prefix, _, ok := strings.Cut(id, "-"); ok && prefix != "" {
		if path, ok := projects.LookupAndValidate(prefix); ok {
			return path
		}
	}
	if m.workDir != "" {
		return m.workDir
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// handleClaimResult records the shell-out trace and surfaces success/failure.
// On failure the row leaves pending immediately with bd's stderr preserved; on
// success the row stays pending until the manifest watcher's reload settles it
// (settlePendingClaims). The always-on receipts pane is bt-oiaj.11; the timeout
// + discrepancy annunciator are bt-oiaj.13.
func (m Model) handleClaimResult(msg claimResultMsg) (Model, tea.Cmd) {
	res := msg.result
	// Observable trace: exact argv + exit code (pre-receipts, gated on BT_DEBUG).
	debug.Log("claim: %s -> exit %d (%v)", res.Argv(), res.ExitCode, res.Duration)

	if res.Err != nil || res.ExitCode != 0 {
		delete(m.pendingClaims, msg.id)
		m.updateListDelegate()
		m.setFailure(claimFailureMessage(msg.id, res))
		return m, nil
	}
	m.setStatus(fmt.Sprintf("Claimed %s; awaiting refresh", msg.id))
	return m, nil
}

// claimFailureMessage builds a one-line failure toast, preferring the last
// non-empty line of bd stderr so the user sees bd's own reason verbatim.
func claimFailureMessage(id string, res bdexec.Result) string {
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
	return fmt.Sprintf("Claim %s failed: %s", id, detail)
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

// handleClaimSpinnerTick advances the pending-row spinner, re-arming itself
// while claims remain pending and self-cancelling otherwise.
func (m Model) handleClaimSpinnerTick() (Model, tea.Cmd) {
	if len(m.pendingClaims) == 0 {
		m.claimSpinnerActive = false
		return m, nil
	}
	m.claimSpinnerIdx++
	m.updateListDelegate()
	return m, claimSpinnerTickCmd()
}

// settlePendingClaims clears pending-claim markers for beads whose reloaded
// state now reflects the claim (status moved off open, or an assignee is set).
// The content-comparing manifest watcher is the authoritative confirmation
// channel (bt-chbqq); this is the v1 spinner-pending settle. Beads not present
// in the reloaded set (e.g. filtered out of a cross-project scope) are left
// pending. Full settle semantics (timeout, discrepancy annunciator) are
// bt-oiaj.13.
func (m *Model) settlePendingClaims() {
	if len(m.pendingClaims) == 0 {
		return
	}
	changed := false
	for id := range m.pendingClaims {
		iss, ok := m.data.issueMap[id]
		if !ok {
			continue
		}
		if iss.Status != model.StatusOpen || iss.Assignee != "" {
			delete(m.pendingClaims, id)
			changed = true
		}
	}
	if changed {
		m.updateListDelegate()
	}
}

// renderClaimConfirm returns the claim-confirm modal panel. View() composites
// it via OverlayCenterDimBackdrop so the backdrop dims uniformly with the other
// modals (tui-modal-compositing.md). Mirrors renderQuitConfirm.
func (m Model) renderClaimConfirm() string {
	t := m.theme

	textStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
	keyStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	idStyle := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)

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

	innerW := lipgloss.Width(lineConfirm)
	for _, l := range []string{lineWhat, lineTitle} {
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
	body := pad + centerLine(lineWhat, innerW) + pad + "\n" +
		pad + centerLine(lineTitle, innerW) + pad + "\n" +
		pad + centerLine(lineConfirm, innerW) + pad
	content := "\n" + body + "\n"

	return RenderTitledPanel(content, PanelOpts{
		Title:       "Claim?",
		Width:       panelWidth,
		CenterTitle: true,
		BorderColor: t.Primary,
		TitleColor:  t.Primary,
		Focused:     true,
	})
}
