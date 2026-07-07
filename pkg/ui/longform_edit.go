package ui

// longform_edit.go implements the long-form field edits (bt-oiaj.6, Slice
// C): a full-screen textarea modal for description, design, comment
// (add-only), append-notes (add-only), and acceptance criteria - the last
// five entries in the field-select hub (field_edit.go's fieldEditEntries).
// Built on the same generic pending/settled write machinery as claim.go/
// field_edit.go (bt-oiaj.13/.5): commitLongformEdit below builds the argv/
// target tuple per field and hands off to the EXISTING commitFieldEdit
// (field_edit.go) verbatim - no new Resolve/pendingWrite plumbing here.
//
// Markdown preview is DEFERRED (bt-oiaj.6 body's "polish" note) - the
// textarea is plain-text editing only in this slice.
//
// Transport per field (fork #9, verified against installed bd 1.0.5 - see
// docs/design/write-routing.md's Consumers section, amended alongside this
// file):
//
//	description -> tempfile + --body-file
//	design      -> tempfile + --design-file
//	comment     -> tempfile + `bd comments add <id> -f <file>` (author flag
//	               unused - bt-oiaj.14's seam)
//	acceptance  -> INLINE --acceptance argv (no --acceptance-file exists)
//	notes       -> INLINE --append-notes argv
import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// longformDirtyGuardWindow is the tkhq #3 "Variant A" ratified discard
// window (bt-tkhq comment, 2026-05-19): Esc on a dirty buffer arms this
// window; a second Esc before it elapses discards; any other key disarms
// early. Matches Slice A's claim-confirm precedent of documenting timing
// constants at the point of use.
const longformDirtyGuardWindow = 3 * time.Second

// ---------------------------------------------------------------------------
// Transport classification (fork #9) - single source of truth for
// commitLongformEdit's argv shape, also asserted directly by
// TestLongformFieldSpecs_TransportAssignments (regression guard against the
// "hunt for --acceptance-file" trap the plan calls out).
// ---------------------------------------------------------------------------

type longformTransport int

const (
	longformTempfile longformTransport = iota
	longformInline
)

// longformFieldSpec describes one long-form field's transport and (for
// tempfile fields) the bd flag that takes the file path. FileFlag is unused
// for inline fields and for "comment", whose argv shape (`comments add <id>
// -f <file>`) differs from the generic `update <id> <flag> <file>` the other
// tempfile fields share - see commitLongformEdit.
type longformFieldSpec struct {
	Transport longformTransport
	FileFlag  string
}

var longformFieldSpecs = map[string]longformFieldSpec{
	"description": {Transport: longformTempfile, FileFlag: "--body-file"},
	"design":      {Transport: longformTempfile, FileFlag: "--design-file"},
	"comment":     {Transport: longformTempfile, FileFlag: "-f"},
	"acceptance":  {Transport: longformInline},
	"notes":       {Transport: longformInline},
}

// ---------------------------------------------------------------------------
// Session-memory draft cache (bt-oiaj.6 acceptance criterion: "Esc cancels
// with draft preservation in memory (so re-opening modal restores draft)").
// Keyed by (issue, field), memory only - NO disk persistence (fence).
// Controller-ratified semantics (Slice C fix, 2026-07-07):
//
//   - Modal closes with a dirty buffer (Esc-Esc discard OR commit) -> stash.
//   - Reopen for the same (id, field) -> prefill from the cache; `original`
//     stays the field value, so the buffer reads dirty and the Esc guard
//     arms normally. Cache miss -> prefill from the field value as usual.
//   - writeResultMsg SUCCESS (exit 0) for that (id, field) -> clear the
//     entry (handleWriteResult, claim.go): after a successful edit,
//     reopening must show the new field value, not a stale draft.
//   - Pre-flight refusal (Resolve error) or bd failure (exit != 0) -> entry
//     stays; reopening restores the attempted text, nothing long-form is
//     ever lost.
//   - Modal closes CLEAN (buffer == field-value baseline) -> entry cleared:
//     the user has (re)landed exactly on the field's value - e.g. reopened
//     a cached draft and manually reverted it - so restoring that stale
//     draft on the next open would be wrong.
// ---------------------------------------------------------------------------

// longformDraftKey identifies one entry in Model.longformDrafts.
type longformDraftKey struct {
	ID    string
	Field string
}

// stashLongformDraft records the open modal's buffer in the session draft
// cache when it differs from its open-time baseline (dirty). A clean buffer
// is never stashed - there is nothing worth preserving.
func (m *Model) stashLongformDraft() {
	if !m.longformEdit.dirty() {
		return
	}
	if m.longformDrafts == nil {
		m.longformDrafts = make(map[longformDraftKey]string)
	}
	key := longformDraftKey{ID: m.fieldEditTargetID, Field: m.longformEdit.field}
	m.longformDrafts[key] = m.longformEdit.textarea.Value()
}

// clearLongformDraft retires the session draft for (id, field). Safe on a
// nil map and a missing key.
func (m *Model) clearLongformDraft(id, field string) {
	delete(m.longformDrafts, longformDraftKey{ID: id, Field: field})
}

// ---------------------------------------------------------------------------
// Textarea modal.
// ---------------------------------------------------------------------------

// LongformEditModal is the full-screen textarea sub-modal for the five
// long-form fields. original is the dirty-check baseline captured at open
// time (the bead's current value for description/design/acceptance; empty
// for comment/notes, which are add-only - see NewLongformEditModal). On a
// draft-cache hit (openLongformEditModal) the BUFFER is prefilled from the
// cached draft while original stays the field value - deliberately, so the
// restored draft reads dirty and the Esc guard arms normally.
type LongformEditModal struct {
	field    string
	label    string
	textarea textarea.Model
	original string

	// Dirty-guard state (tkhq #3 Variant A). escArmed/armedAt back
	// handleLongformEscape's arm/discard state machine.
	escArmed bool
	armedAt  time.Time

	theme         Theme
	width, height int
}

// NewLongformEditModal creates a fresh long-form edit modal for field,
// prefilled with current. label is both the internal "<label>:" line above
// the textarea and (via panelTitle) the basis for the panel's border title.
func NewLongformEditModal(field, label, current string, theme Theme) LongformEditModal {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.SetValue(current)
	ta.CursorEnd()
	return LongformEditModal{field: field, label: label, textarea: ta, original: current, theme: theme}
}

// Focus activates the textarea's cursor/blink.
func (m *LongformEditModal) Focus() tea.Cmd { return m.textarea.Focus() }

// dirty reports whether the buffer differs from its open-time baseline -
// the Esc dirty-guard's only gate (handleLongformEscape). Committing (ctrl+s)
// is NOT gated on dirty - re-submitting an unchanged value is a harmless,
// idempotent bd write, same posture as Slice B's enum pickers (re-picking the
// current status still fires a write).
func (m LongformEditModal) dirty() bool {
	return m.textarea.Value() != m.original
}

// panelTitle returns the modal's border title. Comment/notes get verbs that
// reflect their add-only semantics ("Add"/"Append") rather than "Edit" -
// wording that would wrongly imply an existing comment or the full notes
// history is being replaced (out of scope: no comment edit/delete/threading,
// and --append-notes appends rather than replaces).
func (m LongformEditModal) panelTitle() string {
	switch m.field {
	case "comment":
		return "Add Comment"
	case "notes":
		return "Append Notes"
	default:
		return "Edit " + m.label
	}
}

// panelDims computes the outer panel box (width, height) from the layout
// budget SetSize was given. Occupies most of the frame (bt-oiaj.6 body's
// "full-screen textarea modal") while leaving a margin so the dimmed
// backdrop reads as a frame around it, matching repo_picker.go's sizing
// convention (computeBoxWidth/visibleCount).
func (m LongformEditModal) panelDims() (w, h int) {
	w = int(float64(m.width) * 0.9)
	if w < 40 {
		w = 40
	}
	if maxW := m.width - 2; w > maxW {
		w = maxW
	}
	h = int(float64(m.height) * 0.85)
	if h < 8 {
		h = 8
	}
	if maxH := m.height; h > maxH {
		h = maxH
	}
	return w, h
}

// SetSize updates the modal's layout budget and re-flows the inner textarea.
// Mirrors FieldPickerModal/FieldInputModal: called once at open time only
// (Slice B's established convention - these sub-modals don't re-flow on a
// live WindowSizeMsg while open; see model_update_input.go's
// handleWindowSize, which does not touch any field-edit modal).
func (m *LongformEditModal) SetSize(w, h int) {
	m.width, m.height = w, h
	pw, ph := m.panelDims()
	taWidth := pw - 4 // borders(2) + side breathing(2)
	if taWidth < 10 {
		taWidth = 10
	}
	taHeight := ph - 5 // borders(2) + label line(1) + hint line(1) + blank(1)
	if taHeight < 3 {
		taHeight = 3
	}
	m.textarea.SetWidth(taWidth)
	m.textarea.SetHeight(taHeight)
}

// Update forwards msg to the textarea. Mirrors FieldInputModal.Update -
// handleLongformEditKeys (below) intercepts Commit/Escalate/Cancel before
// any other key reaches here.
func (m LongformEditModal) Update(msg tea.Msg) (LongformEditModal, tea.Cmd) {
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// View renders the textarea modal. Bare content only
// (tui-modal-compositing.md step 1) - composited via OverlayCenterDimBackdrop
// in Model.View().
func (m LongformEditModal) View() string {
	t := m.theme
	pw, ph := m.panelDims()

	labelStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)

	lines := []string{
		labelStyle.Render(m.label + ":"),
		m.textarea.View(),
		"",
		hintStyle.Render("ctrl+s commit  E $EDITOR  esc back/discard"),
	}
	content := strings.Join(lines, "\n")

	return RenderTitledPanel(content, PanelOpts{
		Title:       m.panelTitle(),
		Width:       pw,
		Height:      ph,
		CenterTitle: true,
		BorderColor: t.Primary,
		TitleColor:  t.Primary,
		Focused:     true,
	})
}

// ---------------------------------------------------------------------------
// Trigger / commit (mirrors openFieldPickerOrInput/commitFieldEdit -
// field_edit.go).
// ---------------------------------------------------------------------------

// openLongformEditModal opens the textarea modal for field, prefilled with
// current - or, on a session draft-cache hit for (fieldEditTargetID, field),
// with the cached draft (original stays = current so the restored draft
// reads dirty and the Esc guard arms; see the draft-cache section above).
// Sweeps stale session tempfile dirs on the first call in this bt process
// (sweepStaleLongformSessionDirsOnce) - see the tempfile-lifecycle section
// below for why the sweep is lazy rather than a root.go startup hook.
func (m Model) openLongformEditModal(field, label, current string) (Model, tea.Cmd) {
	sweepStaleLongformSessionDirsOnce()
	m.longformEdit = NewLongformEditModal(field, label, current, m.theme)
	if draft, ok := m.longformDrafts[longformDraftKey{ID: m.fieldEditTargetID, Field: field}]; ok {
		m.longformEdit.textarea.SetValue(draft)
		m.longformEdit.textarea.CursorEnd()
	}
	m.longformEdit.SetSize(m.width, m.height-1)
	m.openModal(ModalLongformEdit)
	m.focused = focusLongformEdit
	return m, m.longformEdit.Focus()
}

// handleLongformEditKeys dispatches keys for the textarea sub-modal. Esc is
// checked first (its dirty-guard state machine needs to run even when
// "armed" from a prior Esc); every other key disarms the guard (tkhq #3:
// "any other key resets the armed state") before falling through to
// Commit/Escalate or the textarea's own key handling.
func (m Model) handleLongformEditKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	kk := m.keys.LongformEdit
	if key.Matches(msg, kk.Cancel) {
		return m.handleLongformEscape()
	}
	m.longformEdit.escArmed = false
	switch {
	case key.Matches(msg, kk.Commit):
		return m.commitLongformEdit()
	case key.Matches(msg, kk.Escalate):
		return m.escalateLongformEditor()
	default:
		var cmd tea.Cmd
		m.longformEdit, cmd = m.longformEdit.Update(msg)
		return m, cmd
	}
}

// handleLongformEscape implements the tkhq #3 Variant A dirty-guard: a clean
// buffer backs out immediately (nothing to lose, matches FieldPickerKeys/
// FieldInputKeys' plain Esc); a dirty buffer arms a 3s discard window on the
// first Esc (footer hint via setNotice) and discards on a second Esc within
// that window. Rapid Esc-Esc (both presses inside the window) discards in
// one gesture, per tkhq's ratified wording.
func (m Model) handleLongformEscape() (Model, tea.Cmd) {
	if !m.longformEdit.dirty() {
		// Clean close also retires any cached draft for this (id, field):
		// the buffer sits exactly on the field's value - e.g. the user
		// reopened a cached draft and manually reverted it - so restoring
		// that stale draft on the next open would be wrong (draft-cache
		// section above).
		m.clearLongformDraft(m.fieldEditTargetID, m.longformEdit.field)
		m.backToFieldSelect()
		return m, nil
	}
	if m.longformEdit.escArmed && time.Since(m.longformEdit.armedAt) < longformDirtyGuardWindow {
		m.longformEdit.escArmed = false
		// Discard closes the modal but keeps the draft for the session
		// (bt-oiaj.6 acceptance: re-opening the modal restores the draft).
		m.stashLongformDraft()
		m.backToFieldSelect()
		m.setNotice("edit discarded (draft kept for this session)")
		return m, nil
	}
	m.longformEdit.escArmed = true
	m.longformEdit.armedAt = time.Now()
	m.setNotice("unsaved - esc again to discard")
	return m, nil
}

// commitLongformEdit builds the argv/target tuple for the current field and
// delegates to commitFieldEdit (field_edit.go) - the SAME Resolve pre-flight
// + pendingWrite registration + writeCmd dispatch every field edit shares
// (Slice B step 5's shared commit path). Only the argv shape and the
// settle-compare Target differ per field/transport.
func (m Model) commitLongformEdit() (Model, tea.Cmd) {
	field := m.longformEdit.field
	value := m.longformEdit.textarea.Value()
	id := m.fieldEditTargetID

	spec, ok := longformFieldSpecs[field]
	if !ok {
		// Defensive: the hub only ever opens a field present in
		// longformFieldSpecs (field_edit.go's five new cases). Not reachable
		// through the UI, but refuse cleanly rather than panic if it ever is.
		m.setFailure(fmt.Sprintf("Edit refused: unknown long-form field %q", field))
		m.fieldEditTargetID = ""
		m.closeModal()
		m.focused = focusList
		return m, nil
	}

	var args []string
	var target string

	if spec.Transport == longformTempfile {
		path, err := writeLongformTempFile(id, field, value)
		if err != nil {
			m.setFailure(err.Error())
			return m, nil
		}
		if field == "comment" {
			// `bd comments add <id> -f <file>` - a different subcommand
			// shape than the generic `update <id> <flag> <file>` the other
			// tempfile fields share (fork #9's "NOT bd comment --file" pin).
			// The author flag is deliberately unused - bt-oiaj.14's seam.
			args = []string{"comments", "add", id, spec.FileFlag, path}
			target = "" // no scalar Issue field to compare - writeSettled's
			// explicit third predicate case (claim.go) settles on the next
			// reload instead (Slice C step 5).
		} else {
			args = []string{"update", id, spec.FileFlag, path}
			target = value // full-replace field: the new value IS the target.
		}
	} else {
		switch field {
		case "acceptance":
			args = []string{"update", id, "--acceptance", value}
			target = value
		case "notes":
			args = []string{"update", id, "--append-notes", value}
			// --append-notes appends with a newline separator (bd update
			// --help). Mirror that here so the settle-compare target matches
			// what bd is documented to produce; if bd's actual separator
			// ever differs, the write still succeeds and surfaces via the
			// 45s discrepancy annunciator rather than silently misreporting
			// success (bt-oiaj.13: never silent stale state).
			prev := ""
			if iss, ok := m.data.issueMap[id]; ok {
				prev = iss.Notes
			}
			if prev == "" {
				target = value
			} else {
				target = prev + "\n" + value
			}
		}
	}

	// Stash the buffer before dispatch (draft-cache section above): on
	// SUCCESS handleWriteResult clears the entry; on pre-flight refusal
	// (commitFieldEdit's Resolve error - no writeResultMsg ever fires) or bd
	// failure (exit != 0) it stays, so reopening restores the attempted text
	// and nothing long-form is ever lost.
	m.stashLongformDraft()
	return m.commitFieldEdit(field, target, args)
}

// escalateLongformEditor writes the current buffer to the session tempfile
// and shells out to $EDITOR/$VISUAL via tea.ExecProcess (tkhq Q5's ratified
// hybrid: uppercase E inside a long-form edit modal only - never bound
// globally; see keys/field_edit.go's LongformEditKeys.Escalate doc). This is
// a DIFFERENT code path from model_editor.go's openInEditor: that function
// backgrounds a GUI app on the whole beads.jsonl file and explicitly refuses
// terminal editors (vim/nano) because a non-blocking background process
// would fight bt for the terminal. tea.ExecProcess is bt's own terminal
// release/restore (Program.exec), the standard bubbletea $EDITOR pattern -
// blocking, so terminal editors work fine here.
func (m Model) escalateLongformEditor() (Model, tea.Cmd) {
	editorRaw := os.Getenv("EDITOR")
	if editorRaw == "" {
		editorRaw = os.Getenv("VISUAL")
	}
	if editorRaw == "" {
		m.setFailure("no $EDITOR or $VISUAL set - can't escalate")
		return m, nil
	}
	editorArgs, err := parseCommandLine(editorRaw)
	if err != nil {
		m.setFailure(fmt.Sprintf("invalid $EDITOR/$VISUAL: %v", err))
		return m, nil
	}
	if len(editorArgs) == 0 {
		m.setFailure("invalid $EDITOR/$VISUAL: empty command")
		return m, nil
	}

	id := m.fieldEditTargetID
	field := m.longformEdit.field
	path, err := writeLongformTempFile(id, field, m.longformEdit.textarea.Value())
	if err != nil {
		m.setFailure(err.Error())
		return m, nil
	}

	cmdArgs := append(append([]string{}, editorArgs[1:]...), path)
	c := exec.Command(editorArgs[0], cmdArgs...)
	cb := func(execErr error) tea.Msg {
		return longformEditorFinishedMsg{path: path, err: execErr}
	}
	return m, tea.ExecProcess(c, cb)
}

// longformEditorFinishedMsg carries the result of an $EDITOR escalation
// (escalateLongformEditor) back onto the event loop.
type longformEditorFinishedMsg struct {
	path string
	err  error
}

// handleLongformEditorFinished reloads the textarea buffer from the
// tempfile $EDITOR was pointed at. If the modal was closed while the editor
// was running (Esc-Esc discard, or a commit via some other path), the reload
// is dropped rather than resurrecting a stale buffer into whatever is open
// now.
func (m Model) handleLongformEditorFinished(msg longformEditorFinishedMsg) (Model, tea.Cmd) {
	if m.activeModal != ModalLongformEdit {
		return m, nil
	}
	if msg.err != nil {
		m.setFailure(fmt.Sprintf("$EDITOR exited with an error: %v", msg.err))
		return m, nil
	}
	content, err := os.ReadFile(msg.path)
	if err != nil {
		m.setFailure(fmt.Sprintf("could not reload buffer from %s: %v", msg.path, err))
		return m, nil
	}
	m.longformEdit.textarea.SetValue(string(content))
	m.longformEdit.textarea.CursorEnd()
	m.longformEdit.escArmed = false
	return m, nil
}

// ---------------------------------------------------------------------------
// Tempfile lifecycle (.bt/tmp/edits/<session-pid>/<id>-<field>.md).
//
// Paths are resolved to ABSOLUTE before being written into argv: bd runs
// with cmd.Dir = the write target's checkout (writeCmd, claim.go), which in
// workspace/multi-repo mode can differ from bt's own launch cwd. A relative
// --body-file path would resolve against the WRONG directory in that case
// (bd would look for it inside the checkout, not bt's cwd) - the tempfile
// itself always lives under bt's own launch cwd's .bt/ (matching every
// other .bt/* convention in this project), but the path handed to bd must
// be absolute so it's reachable regardless of which directory bd is
// actually running in.
//
// Sweep: the plan allows a root.go startup hook only if a trivial insertion
// point exists there. runRootTUI has several early-return branches (
// --search, --diff-since, --profile-startup, --export-md, --as-of) before
// the TUI program ever starts, each of which would need the same sweep call
// repeated (or hoisted above all of them, growing this slice's footprint
// into cmd/bt/ for a TUI-only feature - the field-select hub these fields
// live in never opens outside the TUI anyway). A lazy sweep on the first
// long-form-modal open keeps this entirely within pkg/ui (the plan's Code
// Organization fence) and needs no new insertion point at all - noted here
// per the plan's "otherwise sweep lazily... and note it" instruction.
// ---------------------------------------------------------------------------

var longformSweepOnce sync.Once

// sweepStaleLongformSessionDirsOnce runs sweepStaleLongformSessionDirs
// exactly once per bt process, triggered by the first long-form-modal open
// (openLongformEditModal).
func sweepStaleLongformSessionDirsOnce() {
	longformSweepOnce.Do(sweepStaleLongformSessionDirs)
}

// longformEditsBaseDir returns the absolute path to .bt/tmp/edits under bt's
// launch cwd.
func longformEditsBaseDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve cwd for edit tempfile dir: %w", err)
	}
	return filepath.Join(cwd, ".bt", "tmp", "edits"), nil
}

// longformSessionDir returns this bt process's own tempfile subdir
// (<session-pid> = os.Getpid(), per the plan's path spec).
func longformSessionDir() (string, error) {
	base, err := longformEditsBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, strconv.Itoa(os.Getpid())), nil
}

// writeLongformTempFile writes content to this session's tempfile for
// (id, field), creating the session dir on demand, and returns the absolute
// path.
func writeLongformTempFile(id, field, content string) (string, error) {
	dir, err := longformSessionDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create edit tempfile dir: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.md", id, field))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write edit tempfile: %w", err)
	}
	return path, nil
}

// sweepStaleLongformSessionDirs removes .bt/tmp/edits/<pid> subdirs left
// behind by bt sessions that are no longer running. It does NOT remove
// every dir but this process's own PID: the user routinely runs bt from
// more than one terminal against the same project, so a concurrent
// session's tempfile dir must survive - only dirs whose PID is verifiably
// dead are removed (pidIsRunning). Errors reading the base dir (most
// commonly: it doesn't exist yet) are silently ignored - this is best-effort
// housekeeping, not a correctness requirement of any write.
func sweepStaleLongformSessionDirs() {
	base, err := longformEditsBaseDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	mine := os.Getpid()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue // not a pid-shaped dir name - leave anything unrecognized alone.
		}
		if pid == mine || pidIsRunning(pid) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(base, e.Name()))
	}
}

// pidIsRunning reports whether pid identifies a live OS process.
//
// The two platforms need genuinely different probes:
//   - Windows: os.FindProcess opens a real OpenProcess handle and returns an
//     error precisely when the pid doesn't exist - err == nil IS "running".
//   - POSIX: os.FindProcess essentially always succeeds regardless of
//     whether the pid exists (Go's exec_unix.go findProcess falls back to a
//     bare PID-holding Process on any pidfd lookup error); Signal(0) is the
//     real existence probe there, with no side effects.
//
// Any genuinely uncertain case fails OPEN (assumes running) rather than
// risking deletion of a live session's in-flight draft.
func pidIsRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if runtime.GOOS == "windows" {
		return err == nil
	}
	if err != nil {
		return true
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
