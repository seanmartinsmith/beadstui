package ui

// Tests for the long-form field edits (bt-oiaj.6, Slice C): the textarea
// modal for description/design/comment/append-notes/acceptance, built on
// the same generic pending/settled write machinery as claim.go/field_edit.go
// (bt-oiaj.13/.5). Helpers (newSizedModel, newSizedModelWithRoute,
// stubClaimRunner, mustSelectTarget, drainCmd) are shared with claim_test.go
// / field_edit_test.go in this package.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/seanmartinsmith/beadstui/internal/bdexec"
	"github.com/seanmartinsmith/beadstui/internal/bdroute"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// longformTestIssues seeds a target bead with non-empty content on every
// long-form field so prefill, dirty-check, and notes-append target
// computation all have something meaningful to work against.
func longformTestIssues() []model.Issue {
	return []model.Issue{
		{
			ID: "zz-target", Title: "Original title", Status: model.StatusOpen, Priority: 2,
			Description:        "Original description.",
			Design:             "Original design.",
			AcceptanceCriteria: "Original acceptance.",
			Notes:              "Original notes.",
		},
		{ID: "zz-other", Title: "Another bead", Status: model.StatusOpen, Priority: 1},
	}
}

// chdirTemp switches the working directory to a fresh t.TempDir() for the
// duration of the test, restoring the original cwd on cleanup. Longform's
// tempfile writes are cwd-relative (.bt/tmp/edits/...) - any test that
// exercises a tempfile field must isolate itself here rather than writing
// into the real repo checkout's .bt/ dir (same discipline as
// coverage_extra_test.go's openInEditor tests).
func chdirTemp(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldCwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("os.Chdir(%s): %v", tmp, err)
	}
	return tmp
}

// ---------------------------------------------------------------------------
// Transport classification (fork #9 regression guard).
// ---------------------------------------------------------------------------

func TestLongformFieldSpecs_TransportAssignments(t *testing.T) {
	cases := []struct {
		field    string
		wantTr   longformTransport
		wantFlag string
	}{
		{"description", longformTempfile, "--body-file"},
		{"design", longformTempfile, "--design-file"},
		{"comment", longformTempfile, "-f"},
		{"acceptance", longformInline, ""},
		{"notes", longformInline, ""},
	}
	for _, tc := range cases {
		spec, ok := longformFieldSpecs[tc.field]
		if !ok {
			t.Fatalf("longformFieldSpecs missing field %q", tc.field)
		}
		if spec.Transport != tc.wantTr {
			t.Errorf("%s: Transport = %v, want %v", tc.field, spec.Transport, tc.wantTr)
		}
		if spec.FileFlag != tc.wantFlag {
			t.Errorf("%s: FileFlag = %q, want %q", tc.field, spec.FileFlag, tc.wantFlag)
		}
	}
}

// ---------------------------------------------------------------------------
// fieldValue / writeSettled (claim.go) - the known-issue fix plus the new
// settle cases Slice C adds.
// ---------------------------------------------------------------------------

func TestFieldValue_LongformFields(t *testing.T) {
	iss := &model.Issue{Description: "d", Design: "g", AcceptanceCriteria: "a", Notes: "n"}
	cases := map[string]string{"description": "d", "design": "g", "acceptance": "a", "notes": "n"}
	for field, want := range cases {
		if got := fieldValue(iss, field); got != want {
			t.Errorf("fieldValue(iss, %q) = %q, want %q", field, got, want)
		}
	}
}

// TestWriteSettled_CommentSettlesOnNextReloadUnconditionally pins the
// explicit third predicate case (plan Slice C step 5): a comment
// pendingWrite (Target == "") settles on the very next reload regardless of
// the issue's current state - there is no scalar field to target-compare.
func TestWriteSettled_CommentSettlesOnNextReloadUnconditionally(t *testing.T) {
	iss := &model.Issue{ID: "zz-target", Title: "T", Status: model.StatusOpen}
	pw := pendingWrite{Kind: writeFieldEdit, Field: "comment", Target: ""}
	if !writeSettled(pw, iss) {
		t.Error("a comment pendingWrite must settle on the next reload unconditionally")
	}
}

// ---------------------------------------------------------------------------
// Hub dispatch + prefill.
// ---------------------------------------------------------------------------

func TestFieldSelectKeys_LongformAcceleratorsOpenTextareaModal(t *testing.T) {
	cases := []struct {
		key   string
		field string
	}{
		{"d", "description"},
		{"g", "design"},
		{"c", "comment"},
		{"n", "notes"},
		{"A", "acceptance"},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			m := newSizedModel(t, longformTestIssues(), 120, 32)
			mustSelectTarget(t, &m)
			m.requestFieldEdit()
			m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: rune(tc.key[0]), Text: tc.key})
			m = m2
			if m.activeModal != ModalLongformEdit {
				t.Fatalf("key %q -> activeModal = %v, want ModalLongformEdit", tc.key, m.activeModal)
			}
			if m.longformEdit.field != tc.field {
				t.Errorf("longformEdit.field = %q, want %q", m.longformEdit.field, tc.field)
			}
			if m.focused != focusLongformEdit {
				t.Errorf("focused = %v, want focusLongformEdit", m.focused)
			}
		})
	}
}

// TestOpenFieldPickerOrInput_LongformPrefill verifies description/design/
// acceptance prefill from the bead's current value (full-replace fields),
// while comment/notes open empty (add-only - prefilling from the current
// value would invite committing a duplicate of it).
func TestOpenFieldPickerOrInput_LongformPrefill(t *testing.T) {
	cases := []struct {
		field string
		key   string
		want  string
	}{
		{"description", "d", "Original description."},
		{"design", "g", "Original design."},
		{"acceptance", "A", "Original acceptance."},
		{"comment", "c", ""},
		{"notes", "n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			m := newSizedModel(t, longformTestIssues(), 120, 32)
			mustSelectTarget(t, &m)
			m.requestFieldEdit()
			m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: rune(tc.key[0]), Text: tc.key})
			m = m2
			if got := m.longformEdit.textarea.Value(); got != tc.want {
				t.Errorf("prefill = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Commit path: transport-specific argv + settle target.
// ---------------------------------------------------------------------------

func TestCommitLongformEdit_DescriptionTempfileArgv(t *testing.T) {
	chdirTemp(t)
	got := stubClaimRunner(t, bdexec.Result{ExitCode: 0})
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	m.longformEdit.textarea.SetValue("Original description. changed")
	m2, cmd := m.handleLongformEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m = m2
	if cmd == nil {
		t.Fatal("commit returned nil cmd")
	}
	drainCmd(cmd)
	if len(*got) != 4 || (*got)[0] != "update" || (*got)[1] != "zz-target" || (*got)[2] != "--body-file" {
		t.Fatalf("commit argv = %v, want [update zz-target --body-file <path>]", *got)
	}
	path := (*got)[3]
	if !strings.Contains(filepath.ToSlash(path), "/.bt/tmp/edits/") {
		t.Errorf("tempfile path = %q, want it under .bt/tmp/edits/", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("tempfile not readable: %v", err)
	}
	if string(content) != "Original description. changed" {
		t.Errorf("tempfile content = %q, want %q", content, "Original description. changed")
	}
	pw := m.pendingWrites["zz-target"]
	if pw.Field != "description" || pw.Target != "Original description. changed" {
		t.Errorf("pendingWrite = %+v, want Field=description Target=%q", pw, "Original description. changed")
	}
}

func TestCommitLongformEdit_DesignTempfileArgv(t *testing.T) {
	chdirTemp(t)
	got := stubClaimRunner(t, bdexec.Result{ExitCode: 0})
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'g', Text: "g"})
	m = m2
	m.longformEdit.textarea.SetValue("New design notes")
	m2, cmd := m.handleLongformEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m = m2
	drainCmd(cmd)
	if len(*got) != 4 || (*got)[0] != "update" || (*got)[1] != "zz-target" || (*got)[2] != "--design-file" {
		t.Fatalf("commit argv = %v, want [update zz-target --design-file <path>]", *got)
	}
	content, err := os.ReadFile((*got)[3])
	if err != nil {
		t.Fatalf("tempfile not readable: %v", err)
	}
	if string(content) != "New design notes" {
		t.Errorf("tempfile content = %q, want %q", content, "New design notes")
	}
}

// TestCommitLongformEdit_CommentTempfileArgv pins fork #9's "NOT bd comment
// --file" pin: comment argv is `comments add <id> -f <file>`, a different
// subcommand shape than the generic update flow the other tempfile fields
// use, with Target == "" (no scalar to target-compare).
func TestCommitLongformEdit_CommentTempfileArgv(t *testing.T) {
	chdirTemp(t)
	got := stubClaimRunner(t, bdexec.Result{ExitCode: 0})
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = m2
	m.longformEdit.textarea.SetValue("looks good to me")
	m2, cmd := m.handleLongformEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m = m2
	drainCmd(cmd)
	if len(*got) != 5 || (*got)[0] != "comments" || (*got)[1] != "add" || (*got)[2] != "zz-target" || (*got)[3] != "-f" {
		t.Fatalf("commit argv = %v, want [comments add zz-target -f <path>]", *got)
	}
	content, err := os.ReadFile((*got)[4])
	if err != nil {
		t.Fatalf("tempfile not readable: %v", err)
	}
	if string(content) != "looks good to me" {
		t.Errorf("tempfile content = %q, want %q", content, "looks good to me")
	}
	pw := m.pendingWrites["zz-target"]
	if pw.Field != "comment" || pw.Target != "" {
		t.Errorf("pendingWrite = %+v, want Field=comment Target=\"\"", pw)
	}
}

// TestCommitLongformEdit_AcceptanceInlineArgv pins fork #9's "no
// --acceptance-file exists" resolution: acceptance goes INLINE, no tempfile
// at all.
func TestCommitLongformEdit_AcceptanceInlineArgv(t *testing.T) {
	got := stubClaimRunner(t, bdexec.Result{ExitCode: 0})
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'A', Text: "A"})
	m = m2
	m.longformEdit.textarea.SetValue("New acceptance criteria")
	m2, cmd := m.handleLongformEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m = m2
	if cmd == nil {
		t.Fatal("commit returned nil cmd")
	}
	drainCmd(cmd)
	want := []string{"update", "zz-target", "--acceptance", "New acceptance criteria"}
	if !slices.Equal(*got, want) {
		t.Errorf("commit argv = %v, want %v (INLINE, no tempfile)", *got, want)
	}
	pw := m.pendingWrites["zz-target"]
	if pw.Field != "acceptance" || pw.Target != "New acceptance criteria" {
		t.Errorf("pendingWrite = %+v, want Field=acceptance Target=%q", pw, "New acceptance criteria")
	}
}

// TestCommitLongformEdit_NotesAppendTarget verifies the notes settle-target
// mirrors bd's documented "append with a newline separator" behavior
// (`bd update --help`), for both an empty and a non-empty original value.
func TestCommitLongformEdit_NotesAppendTarget(t *testing.T) {
	cases := []struct {
		name       string
		origNotes  string
		typed      string
		wantTarget string
	}{
		{"empty original", "", "first note", "first note"},
		{"non-empty original", "Existing note.", "second note", "Existing note.\nsecond note"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stubClaimRunner(t, bdexec.Result{ExitCode: 0})
			issues := []model.Issue{{ID: "zz-target", Title: "T", Status: model.StatusOpen, Notes: tc.origNotes}}
			m := newSizedModel(t, issues, 120, 32)
			mustSelectTarget(t, &m)
			m.requestFieldEdit()
			m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'n', Text: "n"})
			m = m2
			m.longformEdit.textarea.SetValue(tc.typed)
			m2, cmd := m.handleLongformEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
			m = m2
			drainCmd(cmd)
			want := []string{"update", "zz-target", "--append-notes", tc.typed}
			if !slices.Equal(*got, want) {
				t.Errorf("commit argv = %v, want %v (INLINE)", *got, want)
			}
			pw := m.pendingWrites["zz-target"]
			if pw.Target != tc.wantTarget {
				t.Errorf("pendingWrite.Target = %q, want %q", pw.Target, tc.wantTarget)
			}
		})
	}
}

// TestCommitLongformEdit_RefusesWhenUnmappable mirrors
// TestCommitFieldEdit_RefusesWhenUnmappable (field_edit_test.go): the
// write-routing Consumers contract applies identically to long-form fields.
func TestCommitLongformEdit_RefusesWhenUnmappable(t *testing.T) {
	invoked := stubClaimRunner(t, bdexec.Result{ExitCode: 0})
	m := newSizedModelWithRoute(t, longformTestIssues(), 120, 32, bdroute.FromWorkspace(nil))
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'A', Text: "A"})
	m = m2
	m.longformEdit.textarea.SetValue("New acceptance")
	m2, cmd := m.handleLongformEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m = m2
	if cmd != nil {
		t.Fatal("a refused write must not dispatch a cmd")
	}
	if len(*invoked) != 0 {
		t.Errorf("executor must not be invoked on a pre-flight refusal; got argv %v", *invoked)
	}
	if _, pending := m.pendingWrites["zz-target"]; pending {
		t.Error("a refused write must never enter the pending state")
	}
	if m.statusSeverity != SeverityFailure {
		t.Errorf("statusSeverity = %v, want SeverityFailure", m.statusSeverity)
	}
}

// ---------------------------------------------------------------------------
// Dirty-guard (tkhq #3 Variant A): arm, disarm on keypress, discard on
// double-Esc; a clean buffer backs out immediately with no arming at all.
// ---------------------------------------------------------------------------

func TestLongformDirtyGuard_CleanBufferBacksOutImmediately(t *testing.T) {
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2

	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m2
	if m.activeModal != ModalFieldSelect {
		t.Fatalf("esc on clean buffer: activeModal = %v, want ModalFieldSelect (immediate back)", m.activeModal)
	}
}

func TestLongformDirtyGuard_ArmDisarmDiscard(t *testing.T) {
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	m.longformEdit.textarea.SetValue("Original description. changed")

	// First esc on a dirty buffer arms the guard and stays open.
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m2
	if m.activeModal != ModalLongformEdit {
		t.Fatalf("first esc on dirty buffer: activeModal = %v, want ModalLongformEdit (armed, stays open)", m.activeModal)
	}
	if !m.longformEdit.escArmed {
		t.Error("first esc on a dirty buffer should arm the guard")
	}
	if !strings.Contains(m.statusMsg, "esc again to discard") {
		t.Errorf("notice = %q, want the discard hint", m.statusMsg)
	}

	// A non-esc key disarms without closing the modal.
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: 'z', Text: "z"})
	m = m2
	if m.longformEdit.escArmed {
		t.Error("a non-esc key should disarm the guard")
	}
	if m.activeModal != ModalLongformEdit {
		t.Fatalf("a disarming keypress should not close the modal; activeModal = %v", m.activeModal)
	}

	// Re-arm, then a second esc within the window discards (rapid esc-esc).
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m2
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m2
	if m.activeModal != ModalFieldSelect {
		t.Fatalf("second esc within the window: activeModal = %v, want ModalFieldSelect (discarded)", m.activeModal)
	}
}

// ---------------------------------------------------------------------------
// $EDITOR escalation (tkhq Q5) - tempfile write + failure paths tested at
// the unit level; the actual tea.ExecProcess cmd is never invoked (it would
// try to run a real editor process outside a running Program).
// ---------------------------------------------------------------------------

func TestEscalateLongformEditor_NoEditorSet(t *testing.T) {
	origEditor, origVisual := os.Getenv("EDITOR"), os.Getenv("VISUAL")
	t.Cleanup(func() {
		_ = os.Setenv("EDITOR", origEditor)
		_ = os.Setenv("VISUAL", origVisual)
	})
	_ = os.Unsetenv("EDITOR")
	_ = os.Unsetenv("VISUAL")

	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	m2, cmd := m.handleLongformEditKeys(tea.KeyPressMsg{Code: 'E', Text: "E"})
	m = m2
	if cmd != nil {
		t.Error("escalate with no $EDITOR/$VISUAL must not dispatch a cmd")
	}
	if m.statusSeverity != SeverityFailure || !strings.Contains(m.statusMsg, "$EDITOR") {
		t.Errorf("expected a failure toast naming $EDITOR, got severity=%v msg=%q", m.statusSeverity, m.statusMsg)
	}
}

func TestEscalateLongformEditor_WritesTempfileWhenEditorSet(t *testing.T) {
	tmpRoot := chdirTemp(t)

	origEditor := os.Getenv("EDITOR")
	t.Cleanup(func() { _ = os.Setenv("EDITOR", origEditor) })
	_ = os.Setenv("EDITOR", "some-editor-binary-that-does-not-need-to-exist")

	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	m.longformEdit.textarea.SetValue("buffer at escalation time")

	m2, cmd := m.handleLongformEditKeys(tea.KeyPressMsg{Code: 'E', Text: "E"})
	m = m2
	if cmd == nil {
		t.Fatal("escalate with $EDITOR set should return a non-nil cmd")
	}
	path := filepath.Join(tmpRoot, ".bt", "tmp", "edits", strconv.Itoa(os.Getpid()), "zz-target-description.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("tempfile not written before escalation: %v", err)
	}
	if string(content) != "buffer at escalation time" {
		t.Errorf("tempfile content = %q, want %q", content, "buffer at escalation time")
	}
}

func TestHandleLongformEditorFinished_ReloadsBuffer(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "edited.md")
	if err := os.WriteFile(path, []byte("edited in $EDITOR"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	m.longformEdit.escArmed = true // proves the reload also clears stale arm state.

	m2, cmd := m.handleLongformEditorFinished(longformEditorFinishedMsg{path: path, err: nil})
	m = m2
	if cmd != nil {
		t.Error("reload should not dispatch a further cmd")
	}
	if got := m.longformEdit.textarea.Value(); got != "edited in $EDITOR" {
		t.Errorf("textarea value = %q, want %q", got, "edited in $EDITOR")
	}
	if m.longformEdit.escArmed {
		t.Error("reload should clear the dirty-guard arm state")
	}
}

func TestHandleLongformEditorFinished_EditorErrorSurfacesFailureToast(t *testing.T) {
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	original := m.longformEdit.textarea.Value()

	m2, _ = m.handleLongformEditorFinished(longformEditorFinishedMsg{path: "/does/not/matter", err: fmt.Errorf("boom")})
	m = m2
	if m.statusSeverity != SeverityFailure {
		t.Errorf("statusSeverity = %v, want SeverityFailure", m.statusSeverity)
	}
	if got := m.longformEdit.textarea.Value(); got != original {
		t.Errorf("buffer should be unchanged on editor error, got %q", got)
	}
}

func TestHandleLongformEditorFinished_DropsStaleReloadIfModalClosed(t *testing.T) {
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	m.closeModal() // simulate having left the modal while $EDITOR was running.

	m2, cmd := m.handleLongformEditorFinished(longformEditorFinishedMsg{path: "/does/not/matter", err: nil})
	m = m2
	if cmd != nil {
		t.Error("a stale reload should not dispatch a cmd")
	}
	if m.statusSeverity == SeverityFailure {
		t.Error("a stale reload should be a silent no-op, not surface a toast")
	}
}

// ---------------------------------------------------------------------------
// Tempfile lifecycle: sweep + pid-liveness probe.
// ---------------------------------------------------------------------------

// TestSweepStaleLongformSessionDirs verifies the sweep preserves this
// session's own dir AND any other dir whose pid is still alive (the user
// routinely runs bt from more than one terminal against the same project),
// removing only dirs whose pid is verifiably dead.
func TestSweepStaleLongformSessionDirs(t *testing.T) {
	tmpRoot := chdirTemp(t)

	base := filepath.Join(tmpRoot, ".bt", "tmp", "edits")
	mine := strconv.Itoa(os.Getpid())
	otherRunning := strconv.Itoa(os.Getppid())
	stale := "999999999" // not a real pid
	notPidShaped := "scratch"

	for _, dir := range []string{mine, otherRunning, stale, notPidShaped} {
		if err := os.MkdirAll(filepath.Join(base, dir), 0o755); err != nil {
			t.Fatalf("setup mkdir %s: %v", dir, err)
		}
	}

	sweepStaleLongformSessionDirs()

	for _, dir := range []string{mine, otherRunning, notPidShaped} {
		if _, err := os.Stat(filepath.Join(base, dir)); err != nil {
			t.Errorf("dir %s should survive the sweep, stat err: %v", dir, err)
		}
	}
	if _, err := os.Stat(filepath.Join(base, stale)); !os.IsNotExist(err) {
		t.Errorf("stale dir %s should have been swept, stat err = %v", stale, err)
	}
}

func TestPidIsRunning(t *testing.T) {
	if !pidIsRunning(os.Getpid()) {
		t.Error("pidIsRunning(own pid) should be true")
	}
	if !pidIsRunning(os.Getppid()) {
		t.Error("pidIsRunning(parent pid) should be true")
	}
	if pidIsRunning(999999999) {
		t.Error("pidIsRunning(a clearly-invalid pid) should be false")
	}
}

// ---------------------------------------------------------------------------
// Session-memory draft cache (controller-ratified fix on the Slice C
// dirty-guard concern; bt-oiaj.6 acceptance criterion "Esc cancels with
// draft preservation in memory (so re-opening modal restores draft)").
// ---------------------------------------------------------------------------

// drainCmdMsgs recursively executes cmd (unwrapping tea.BatchMsg like
// drainCmd) and returns every leaf message produced, so a test can route a
// writeResultMsg through handleWriteResult after a stubbed commit.
func drainCmdMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	var out []tea.Msg
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			out = append(out, drainCmdMsgs(c)...)
		}
		return out
	}
	return append(out, msg)
}

// commitAndHandleResult drives a ctrl+s commit through the stubbed executor
// and routes the resulting writeResultMsg through handleWriteResult,
// returning the updated model. Fails the test if no writeResultMsg was
// produced.
func commitAndHandleResult(t *testing.T, m Model) Model {
	t.Helper()
	m2, cmd := m.handleLongformEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m = m2
	if cmd == nil {
		t.Fatal("commit returned nil cmd")
	}
	found := false
	for _, msg := range drainCmdMsgs(cmd) {
		if wr, ok := msg.(writeResultMsg); ok {
			m2, _ := m.handleWriteResult(wr)
			m = m2
			found = true
		}
	}
	if !found {
		t.Fatal("commit produced no writeResultMsg")
	}
	return m
}

// TestLongformDraftCache_DiscardThenReopenRestoresDraft: (a) an Esc-Esc
// discard keeps the buffer in the session cache; reopening the same field
// prefills the draft (not the field value), and the restored draft reads
// dirty - Esc arms the guard instead of backing out.
func TestLongformDraftCache_DiscardThenReopenRestoresDraft(t *testing.T) {
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	m.longformEdit.textarea.SetValue("half-finished draft")

	// Esc-Esc discard.
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m2
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m2
	if m.activeModal != ModalFieldSelect {
		t.Fatalf("activeModal = %v, want ModalFieldSelect after discard", m.activeModal)
	}

	// Reopen the same field: prefill = the draft.
	m2, _ = m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	if got := m.longformEdit.textarea.Value(); got != "half-finished draft" {
		t.Fatalf("reopen prefill = %q, want the discarded draft", got)
	}
	// The restored draft reads dirty vs the field value: Esc must arm, not
	// close (the controller-pinned semantics: original stays the field value).
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m2
	if m.activeModal != ModalLongformEdit {
		t.Error("esc on a restored draft should arm the guard, not close the modal")
	}
	if !m.longformEdit.escArmed {
		t.Error("esc on a restored draft should arm the dirty guard")
	}
}

// TestLongformDraftCache_SuccessfulWriteClearsDraft: (b) a commit whose
// write succeeds (exit 0) clears the cache entry - reopening prefills the
// current field value, not the pre-commit draft.
func TestLongformDraftCache_SuccessfulWriteClearsDraft(t *testing.T) {
	chdirTemp(t)
	stubClaimRunner(t, bdexec.Result{ExitCode: 0})
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	m.longformEdit.textarea.SetValue("Original description. v2")

	m = commitAndHandleResult(t, m)

	// The success path keeps the row pending until the watcher reload
	// settles it; simulate that settle so requestFieldEdit doesn't refuse
	// on the double-dispatch guard.
	delete(m.pendingWrites, "zz-target")

	m.requestFieldEdit()
	m2, _ = m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	if got := m.longformEdit.textarea.Value(); got != "Original description." {
		t.Errorf("reopen after successful write prefill = %q, want the field value (draft cleared)", got)
	}
}

// TestLongformDraftCache_FailedWriteKeepsDraft: (c) a commit whose write
// FAILS (exit 1) keeps the cache entry - reopening restores the attempted
// text so nothing long-form is ever lost. Uses acceptance (inline
// transport) so no tempfile/cwd isolation is needed.
func TestLongformDraftCache_FailedWriteKeepsDraft(t *testing.T) {
	stubClaimRunner(t, bdexec.Result{ExitCode: 1, Stderr: "Error: something broke"})
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'A', Text: "A"})
	m = m2
	m.longformEdit.textarea.SetValue("attempted acceptance text")

	m = commitAndHandleResult(t, m)
	// The failure path already cleared pendingWrites (handleWriteResult),
	// so reopening needs no manual settle.
	if _, pending := m.pendingWrites["zz-target"]; pending {
		t.Fatal("setup: failed write should have left pending state")
	}

	m.requestFieldEdit()
	m2, _ = m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'A', Text: "A"})
	m = m2
	if got := m.longformEdit.textarea.Value(); got != "attempted acceptance text" {
		t.Errorf("reopen after failed write prefill = %q, want the attempted text (draft kept)", got)
	}
}

// TestLongformDraftCache_RefusedWriteKeepsDraft: the pre-flight refusal
// variant of (c) - a Resolve error produces NO writeResultMsg at all, and
// the draft stashed at commit time must survive for the reopen.
func TestLongformDraftCache_RefusedWriteKeepsDraft(t *testing.T) {
	stubClaimRunner(t, bdexec.Result{ExitCode: 0})
	m := newSizedModelWithRoute(t, longformTestIssues(), 120, 32, bdroute.FromWorkspace(nil))
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'A', Text: "A"})
	m = m2
	m.longformEdit.textarea.SetValue("refused attempt")

	m2, cmd := m.handleLongformEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m = m2
	if cmd != nil {
		t.Fatal("a refused write must not dispatch a cmd")
	}

	m.requestFieldEdit()
	m2, _ = m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'A', Text: "A"})
	m = m2
	if got := m.longformEdit.textarea.Value(); got != "refused attempt" {
		t.Errorf("reopen after refusal prefill = %q, want the attempted text (draft kept)", got)
	}
}

// TestLongformDraftCache_PerIssueFieldIsolation: (d) the cache is keyed by
// (id, field) - a draft on one bead's description leaks into neither
// another field on the same bead nor the same field on another bead.
func TestLongformDraftCache_PerIssueFieldIsolation(t *testing.T) {
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	m.longformEdit.textarea.SetValue("zz-target description draft")
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m2
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m2

	// Same bead, different field: design must prefill its own field value.
	m2, _ = m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'g', Text: "g"})
	m = m2
	if got := m.longformEdit.textarea.Value(); got != "Original design." {
		t.Errorf("design prefill = %q, want the design field value (no cross-field leak)", got)
	}
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc}) // clean -> back to hub
	m = m2
	m2, _ = m.handleFieldSelectKeys(tea.KeyPressMsg{Code: tea.KeyEsc}) // cancel the flow
	m = m2

	// Different bead, same field: description must prefill zz-other's own
	// (empty) description.
	if !m.selectIssueByID("zz-other") {
		t.Fatal("setup: zz-other not in visible list")
	}
	m.requestFieldEdit()
	m2, _ = m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	if got := m.longformEdit.textarea.Value(); got != "" {
		t.Errorf("zz-other description prefill = %q, want empty (no cross-bead leak)", got)
	}

	// And the original draft is still there for its own (id, field).
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc}) // clean -> hub
	m = m2
	m2, _ = m.handleFieldSelectKeys(tea.KeyPressMsg{Code: tea.KeyEsc}) // cancel
	m = m2
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ = m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	if got := m.longformEdit.textarea.Value(); got != "zz-target description draft" {
		t.Errorf("zz-target description reopen = %q, want the stashed draft", got)
	}
}

// TestLongformDraftCache_CleanCloseClearsStaleDraft pins the symmetric
// completion of the ratified semantics: reopening a cached draft and
// manually reverting the buffer to the field value, then closing clean,
// retires the entry - the NEXT open prefills the field value again instead
// of resurrecting the reverted draft.
func TestLongformDraftCache_CleanCloseClearsStaleDraft(t *testing.T) {
	m := newSizedModel(t, longformTestIssues(), 120, 32)
	mustSelectTarget(t, &m)
	m.requestFieldEdit()
	m2, _ := m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	m.longformEdit.textarea.SetValue("draft to be reverted")
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m2
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = m2

	// Reopen (draft restored), manually revert to the field value, close.
	m2, _ = m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	if got := m.longformEdit.textarea.Value(); got != "draft to be reverted" {
		t.Fatalf("setup: reopen prefill = %q, want the draft", got)
	}
	m.longformEdit.textarea.SetValue("Original description.")
	m2, _ = m.handleLongformEditKeys(tea.KeyPressMsg{Code: tea.KeyEsc}) // clean -> immediate back
	m = m2
	if m.activeModal != ModalFieldSelect {
		t.Fatalf("clean esc should back out immediately; activeModal = %v", m.activeModal)
	}

	// Next open prefills the field value - the stale draft is gone.
	m2, _ = m.handleFieldSelectKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = m2
	if got := m.longformEdit.textarea.Value(); got != "Original description." {
		t.Errorf("reopen after clean revert = %q, want the field value (stale draft cleared)", got)
	}
}

// ---------------------------------------------------------------------------
// Full teatest keypath - real Bubble Tea event loop, stubbed executor.
// ---------------------------------------------------------------------------

// TestLongformEditKeypath_DescriptionTeatest exercises description end to
// end: open the hub, open the description textarea, type additional text,
// commit via ctrl+s, and assert the executor saw a --body-file argv whose
// tempfile path is readable and holds the typed content.
func TestLongformEditKeypath_DescriptionTeatest(t *testing.T) {
	chdirTemp(t)

	invoked := make(chan []string, 1)
	orig := claimRunner
	claimRunner = func(ctx context.Context, dir string, args ...string) bdexec.Result {
		invoked <- append([]string{}, args...)
		return bdexec.Result{Args: append([]string{"bd"}, args...), ExitCode: 0}
	}
	t.Cleanup(func() { claimRunner = orig })

	issues := []model.Issue{
		{ID: "zz-target", Title: "Original title", Status: model.StatusOpen, Description: "Original description."},
	}
	m := NewModel(issues, nil, "", nil, bdroute.SingleProject(t.TempDir()))
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 32))

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("zz-target"))
	}, teatest.WithDuration(8*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'e', Text: "e"})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Edit field"))
	}, teatest.WithDuration(5*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'd', Text: "d"})
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("Edit Description"))
	}, teatest.WithDuration(5*time.Second))

	for _, r := range " xyz" {
		tm.Send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	tm.Send(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})

	select {
	case args := <-invoked:
		if len(args) != 4 || args[0] != "update" || args[1] != "zz-target" || args[2] != "--body-file" {
			t.Fatalf("executor argv = %v, want [update zz-target --body-file <path>]", args)
		}
		path := args[3]
		if !strings.Contains(filepath.ToSlash(path), "/.bt/tmp/edits/") {
			t.Errorf("tempfile path = %q, want it under .bt/tmp/edits/", path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("tempfile not readable: %v", err)
		}
		if string(content) != "Original description. xyz" {
			t.Errorf("tempfile content = %q, want %q", content, "Original description. xyz")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("field-edit executor was not invoked after commit")
	}

	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(15*time.Second))
}
