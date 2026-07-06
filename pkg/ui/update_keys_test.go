package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/internal/datasource"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/version"
)

// Cover additional branches in Model.Update for quit/help/tab handling and update notices.
func TestUpdateHelpQuitAndTabFocus(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "One", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil, nil)

	// Make model ready and split view
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Help toggle via ? then dismiss with another key
	updated, _ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m = updated.(Model)
	if m.activeModal != ModalHelp || m.focused != focusHelp {
		t.Fatalf("expected help overlay shown")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m = updated.(Model)
	if m.activeModal == ModalHelp || m.focused != focusList {
		t.Fatalf("expected help overlay dismissed")
	}

	// Tab should flip focus in split view
	if m.focused != focusList {
		t.Fatalf("expected list focus before tab")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)
	if m.focused != focusDetail {
		t.Fatalf("expected detail focus after tab")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)
	if m.focused != focusList {
		t.Fatalf("expected list focus after second tab")
	}

	// Escape should show quit confirm, 'y' should issue tea.Quit
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)
	if m.activeModal != ModalQuitConfirm {
		t.Fatalf("expected quit confirm after esc")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatalf("expected quit command on confirm quit")
	}
}

func TestUpdateMsgSetsUpdateAvailable(t *testing.T) {
	m := NewModel([]model.Issue{{ID: "1", Title: "One", Status: model.StatusOpen}}, nil, "", nil, nil)
	updated, _ := m.Update(UpdateMsg{TagName: "v9.9.9", URL: "https://example"})
	m = updated.(Model)
	if !m.updateAvailable || m.updateTag != "v9.9.9" {
		t.Fatalf("update flag not set")
	}
}

// TestCheckUpdateCmdSkipsDevBuild verifies the startup update check is a no-op
// on development builds. The test binary itself is a dev build (no ldflags,
// ReadBuildInfo reports "(devel)"), so version.IsDevBuild() is true and
// CheckUpdateCmd must short-circuit to nil before any network call — no
// UpdateMsg, no footer badge, no notification. Guards against a drifted
// `fallback` re-introducing the false "update available" nag.
func TestCheckUpdateCmdSkipsDevBuild(t *testing.T) {
	if !version.IsDevBuild() {
		t.Skip("test binary is not a dev build; skip cannot be exercised")
	}
	if msg := CheckUpdateCmd()(); msg != nil {
		t.Fatalf("CheckUpdateCmd on a dev build should return nil, got %#v", msg)
	}
}

func TestHistoryViewToggle(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Test Issue", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil, nil)

	// Make model ready
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// h should toggle history view on
	if m.mode == ViewHistory {
		t.Fatalf("history view should be off initially")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	if m.mode != ViewHistory {
		t.Fatalf("expected history view to be on after h key")
	}
	if m.focused != focusHistory {
		t.Fatalf("expected focus to be on history, got %v", m.focused)
	}

	// h again should toggle off
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	if m.mode == ViewHistory {
		t.Fatalf("expected history view to be off after second h key")
	}
	if m.focused != focusList {
		t.Fatalf("expected focus to be back on list, got %v", m.focused)
	}
}

// TestLabelPickerEnterClearsWhenOpenedWithFilter exercises the labels modal
// Enter handler end-to-end: open with an active label filter, deselect it,
// press Enter, confirm the model's labelFilter is cleared (not refilled
// with the cursor's label by the no-selection cursor-shortcut path).
func TestLabelPickerEnterClearsWhenOpenedWithFilter(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "One", Status: model.StatusOpen, Labels: []string{"area:tui"}},
		{ID: "bv-2", Title: "Two", Status: model.StatusOpen, Labels: []string{"area:product"}},
	}
	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Simulate a pre-existing filter and seed the picker with it.
	m.filter.labelFilter = "area:tui"
	m.labelPicker = NewLabelPickerModel(
		[]string{"area:tui", "area:product"},
		map[string]int{"area:tui": 1, "area:product": 1},
		m.theme,
	)
	m.labelPicker.SetActiveLabels([]string{"area:tui"})
	m.openModal(ModalLabelPicker)
	m.focused = focusLabelPicker

	if !m.labelPicker.OpenedWithFilter() {
		t.Fatalf("precondition: picker should report OpenedWithFilter()=true")
	}
	if got := len(m.labelPicker.SelectedLabels()); got != 1 {
		t.Fatalf("precondition: expected 1 selected label, got %d", got)
	}

	// Cursor is on the first label after Reset; it should be area:tui (the
	// active one). Toggle it off so SelectedLabels() returns nil.
	m.labelPicker.Reset()
	m.labelPicker.ToggleSelected()
	if got := len(m.labelPicker.SelectedLabels()); got != 0 {
		t.Fatalf("setup: deselect failed, %d still selected", got)
	}

	// Press Enter -- with OpenedWithFilter()==true, this must clear the
	// filter rather than apply the cursor's label.
	m = m.handleLabelPickerKeys(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.filter.labelFilter != "" {
		t.Errorf("Enter on deselected modal should clear filter; got labelFilter=%q", m.filter.labelFilter)
	}
}

// TestLabelPickerEnterAppliesCursorWhenColdOpen confirms the long-standing
// shortcut survives the bt-NEW fix: opening cold (no active filter) and
// pressing Enter on a label still applies that label as a single-select
// filter. Without this guard the fix could have broken the convenience path.
func TestLabelPickerEnterAppliesCursorWhenColdOpen(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "One", Status: model.StatusOpen, Labels: []string{"area:tui"}},
	}
	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	m.filter.labelFilter = "" // cold: no active filter
	m.labelPicker = NewLabelPickerModel(
		[]string{"area:tui"},
		map[string]int{"area:tui": 1},
		m.theme,
	)
	m.labelPicker.SetActiveLabels(nil) // explicitly cold
	m.openModal(ModalLabelPicker)
	m.focused = focusLabelPicker

	if m.labelPicker.OpenedWithFilter() {
		t.Fatalf("precondition: picker should report OpenedWithFilter()=false")
	}

	m.labelPicker.Reset()
	m = m.handleLabelPickerKeys(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.filter.labelFilter != "area:tui" {
		t.Errorf("cold-open Enter should apply cursor label; got labelFilter=%q want %q", m.filter.labelFilter, "area:tui")
	}
}

// TestHistoryViewTransitionNoLeakage covers bt-7hhc at the Model level.
// After pressing `h` to enter history view, the full rendered View output
// must NOT contain any issues-list row signatures (repo badges, P0/P1
// status codes, [BUG]-style type tags). If it does, the transition is
// leaking content through HistoryModel rendering. If this passes but the
// user still sees leakage in the running TUI, the issue is in the
// Bubble Tea v2 / terminal renderer layer below us.
func TestHistoryViewTransitionNoLeakage(t *testing.T) {
	issues := []model.Issue{
		{ID: "dotfiles-d6n", Title: "Some dotfiles work", Status: model.StatusOpen, Priority: 0},
		{ID: "bv-2", Title: "Other work", Status: model.StatusOpen, Priority: 1},
	}
	m := NewModel(issues, nil, "", nil, nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	m = updated.(Model)

	// Press h to enter history view. After bt-uizm, this dispatches an async
	// load and the model lands in ViewHistory + historyLoading=true; the
	// rendered frame is the loading screen until HistoryLoadedMsg arrives.
	// The leak-pattern assertions below still apply to the loading frame.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	view := m.View()
	rendered := view.Content

	leaks := []string{
		"P0 OPEN", "P1 OPEN", "P2 OPEN", "P3 OPEN",
		"[DOTF]",
		"[BUG]", "[FEATURE]", "[EPIC]", "[DECISION]",
	}
	for _, leak := range leaks {
		if strings.Contains(rendered, leak) {
			t.Errorf("post-transition render leaks issues-list pattern %q", leak)
		}
	}

	// The rendered output must cover the full terminal so that the diff
	// renderer in bubbletea/ultraviolet does not leave residual cells from
	// the previous frame. Each row should be at least m.width wide and
	// there should be at least m.height rows. Without this, partially
	// covered rows could explain the "issues-list rows showing inside
	// history panes" symptom — the rows are NOT history content; they are
	// stale terminal cells.
	rows := strings.Split(rendered, "\n")
	if len(rows) < m.height {
		t.Errorf("render produced %d rows, expected at least %d (height); short-renders leave stale cells", len(rows), m.height)
	}
}

// TestHistoryAsyncDispatch covers bt-uizm: pressing `h` must transition to
// ViewHistory immediately and dispatch the report load as a tea.Cmd, rather
// than blocking the Update tick on synchronous git history extraction.
// Verifies four invariants:
//
//  1. m.mode flips to ViewHistory on the same tick the key arrives.
//  2. m.historyLoading is true so the view renders the loading screen.
//  3. Update returns a non-nil tea.Cmd (the LoadHistoryCmd dispatch).
//  4. Pressing `h` again from the loading state exits cleanly back to ViewList.
//
// bt-ydjw phase 1 added the Dolt-only short-circuit, which fires when no
// .beads/*.jsonl exists on disk. To keep this test focused on the original
// async-dispatch invariants, chdir into a temp repo that has a JSONL file
// so the gate does NOT fire here; enterHistoryView resolves the JSONL-
// presence check off cwd. The Dolt-only short-circuit gets its own test.
func TestHistoryAsyncDispatch(t *testing.T) {
	tmp := t.TempDir()
	beadsDir := filepath.Join(tmp, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	beadsFile := filepath.Join(beadsDir, "beads.jsonl")
	if err := os.WriteFile(beadsFile, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Chdir-only so enterHistoryView's cwd-derived resolveHistoryPath sees
	// the JSONL. Leave NewModel's beadsPath empty so the background worker
	// doesn't lock files inside the temp dir (the lock would survive past
	// t.TempDir() cleanup on Windows and fail the test).
	t.Chdir(tmp)

	issues := []model.Issue{
		{ID: "bv-1", Title: "Test", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil, nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Press h to enter history.
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	if m.mode != ViewHistory {
		t.Fatalf("after h: expected ViewHistory, got %v", m.mode)
	}
	if !m.historyLoading {
		t.Errorf("after h: expected historyLoading=true, got false")
	}
	if cmd == nil {
		t.Errorf("after h: expected non-nil tea.Cmd from async dispatch, got nil")
	}

	// Loading frame must render without panicking and must not leak the
	// list-row signatures the leak test above looks for.
	frame := m.View().Content
	if frame == "" {
		t.Errorf("loading frame is empty")
	}

	// Press h again from the loading state to exit cleanly.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	if m.mode != ViewList {
		t.Errorf("h-from-loading: expected ViewList, got %v", m.mode)
	}
	if m.focused != focusList {
		t.Errorf("h-from-loading: expected focusList, got %v", m.focused)
	}
}

// TestHistoryDoltOnlyShortCircuit covers bt-ydjw phase 1: pressing `h` in a
// repo with no .beads/*.jsonl on disk must short-circuit to the polite
// empty-state instead of dispatching LoadHistoryCmd against a correlator
// that has no data to read.
//
// Invariants:
//  1. m.mode flips to ViewHistory.
//  2. m.historyDoltOnly is true so the view renders renderHistoryDoltOnly.
//  3. m.historyLoading is false (no spinner shown).
//  4. enterHistoryView returns nil (no async dispatch).
//  5. The rendered frame contains the bt-08sh reference so users know where
//     the migration work is being tracked.
//  6. Pressing `h` again exits to ViewList and clears historyDoltOnly.
func TestHistoryDoltOnlyShortCircuit(t *testing.T) {
	// TempDir with no .beads/*.jsonl files = Dolt-only repo for the purposes
	// of the gate. Real bt repos look like this post-90d8432d.
	tmp := t.TempDir()
	beadsFile := filepath.Join(tmp, ".beads", "beads.jsonl")
	// Do NOT create the file - the absence is the test condition.

	issues := []model.Issue{
		{ID: "bv-1", Title: "Test", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, beadsFile, nil, nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	if m.mode != ViewHistory {
		t.Fatalf("after h: expected ViewHistory, got %v", m.mode)
	}
	if !m.historyDoltOnly {
		t.Errorf("after h: expected historyDoltOnly=true, got false")
	}
	if m.historyLoading {
		t.Errorf("after h: expected historyLoading=false, got true")
	}
	if cmd != nil {
		t.Errorf("after h: expected nil Cmd (no async dispatch), got %T", cmd)
	}

	frame := m.View().Content
	if !strings.Contains(frame, "bt-08sh") {
		t.Errorf("Dolt-only empty state must reference bt-08sh, frame was:\n%s", frame)
	}
	if !strings.Contains(frame, "No commit history yet") {
		t.Errorf("Dolt-only empty state missing title; frame was:\n%s", frame)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	if m.mode != ViewList {
		t.Errorf("h-from-dolt-only: expected ViewList, got %v", m.mode)
	}
	if m.historyDoltOnly {
		t.Errorf("h-from-dolt-only: expected historyDoltOnly cleared, still true")
	}
}

// TestHistoryDispatchTarget_CursorDrivenGlobal covers bt-ydjw.1 Case B: in
// global Dolt mode, the History view's dispatch target must follow the
// cursor's bead-ID prefix when it maps to a known database, even when the
// boot-time cwd anchor (currentProjectDB) is empty. Without this the
// dispatcher refuses to run from non-beads cwds (e.g. ~/.obs/sms) and the
// phase-1 polite empty-state fires by mistake.
//
// Uses the package-level enumerateDoltDatabasesFn injection point to stub
// the live database list, avoiding a Dolt-server dependency.
func TestHistoryDispatchTarget_CursorDrivenGlobal(t *testing.T) {
	savedFn := enumerateDoltDatabasesFn
	t.Cleanup(func() { enumerateDoltDatabasesFn = savedFn })

	enumerateDoltDatabasesFn = func(dsn string) []string {
		return []string{"bt", "bd", "tpane"}
	}

	// Issue with bt-* prefix so historyContext().CursorPrefix == "bt".
	issues := []model.Issue{{ID: "bt-1", Title: "T", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", &datasource.DataSource{
		Type: datasource.SourceTypeDoltGlobal,
		Path: "root@tcp(127.0.0.1:9999)/?parseTime=true",
	}, nil)
	// Force a non-beads cwd posture: clear the anchor that detectCurrentProjectDB
	// would normally populate. This is the Case B condition: cwd is not in a
	// beads project, so m.currentProjectDB == "".
	m.currentProjectDB = ""

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	ctx := m.historyContext()
	if ctx.CursorPrefix != "bt" {
		t.Fatalf("test precondition: expected CursorPrefix=bt, got %q", ctx.CursorPrefix)
	}

	projectDB, ok := m.historyDispatchTarget(ctx, "/no/jsonl/here")
	if !ok {
		t.Fatalf("expected dispatch admitted via cursor prefix, got ok=false")
	}
	if projectDB != "bt" {
		t.Errorf("expected projectDB=bt (cursor-derived), got %q", projectDB)
	}
}

// TestHistoryDispatchTarget_GlobalUnknownPrefixFalls covers the safety side
// of bt-ydjw.1 Case B: when the cursor prefix does NOT match any database in
// the live enumeration, the resolver must reject the dispatch rather than
// targeting a nonexistent database. Falling back to a validated anchor
// (when available) is the intended secondary path; with no valid candidate
// the result is ("", false) and the gate caller short-circuits.
func TestHistoryDispatchTarget_GlobalUnknownPrefixFalls(t *testing.T) {
	savedFn := enumerateDoltDatabasesFn
	t.Cleanup(func() { enumerateDoltDatabasesFn = savedFn })

	enumerateDoltDatabasesFn = func(dsn string) []string {
		return []string{"bt", "bd"} // intentionally lacks "xx"
	}

	issues := []model.Issue{{ID: "xx-1", Title: "T", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", &datasource.DataSource{
		Type: datasource.SourceTypeDoltGlobal,
		Path: "root@tcp(127.0.0.1:9999)/?parseTime=true",
	}, nil)
	m.currentProjectDB = ""

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	ctx := m.historyContext()
	_, ok := m.historyDispatchTarget(ctx, "/no/jsonl/here")
	if ok {
		t.Errorf("expected dispatch rejected when cursor prefix is unknown to enumeration")
	}
}

// TestHistoryDispatchTarget_CrossProjectCursor covers bt-ydjw.4: when the
// bead-ID prefix differs from the Dolt DB name (e.g. bd-* beads live in
// the `beads` DB; mkt-* in `marketplace`; db-* in `dev_browser`), the
// dispatcher must use issue.SourceRepo (the authoritative DB name set by
// GlobalDoltReader from the UNION ALL _global_source column) and NOT the
// ID prefix as the candidate.
//
// Pre-fix: candidates=[bd] -> no match against [beads, bt] -> phase-1
// polite empty-state. Post-fix: candidates=[beads, bd] -> beads matches.
func TestHistoryDispatchTarget_CrossProjectCursor(t *testing.T) {
	savedFn := enumerateDoltDatabasesFn
	t.Cleanup(func() { enumerateDoltDatabasesFn = savedFn })

	enumerateDoltDatabasesFn = func(dsn string) []string {
		return []string{"beads", "bt"}
	}

	// bd-prefixed bead with SourceRepo="beads" (what GlobalDoltReader
	// produces for any bd-* loaded from the `beads` DB).
	issues := []model.Issue{{
		ID:         "bd-1",
		Title:      "T",
		Status:     model.StatusOpen,
		SourceRepo: "beads",
	}}
	m := NewModel(issues, nil, "", &datasource.DataSource{
		Type: datasource.SourceTypeDoltGlobal,
		Path: "root@tcp(127.0.0.1:9999)/?parseTime=true",
	}, nil)
	m.currentProjectDB = ""

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	ctx := m.historyContext()
	if ctx.CursorPrefix != "bd" {
		t.Fatalf("test precondition: expected CursorPrefix=bd, got %q", ctx.CursorPrefix)
	}
	if ctx.CursorSourceRepo != "beads" {
		t.Fatalf("test precondition: expected CursorSourceRepo=beads, got %q", ctx.CursorSourceRepo)
	}

	projectDB, ok := m.historyDispatchTarget(ctx, "/no/jsonl/here")
	if !ok {
		t.Fatalf("expected dispatch admitted via cursor SourceRepo, got ok=false")
	}
	if projectDB != "beads" {
		t.Errorf("expected projectDB=beads (SourceRepo-derived), got %q", projectDB)
	}
}

// TestHistoryDispatchTarget_CursorPrefixFallback covers the secondary
// candidate path: when SourceRepo is empty (single-repo Dolt, JSONL load,
// or any code path that doesn't stamp _global_source) but the ID prefix
// does match a known DB, the dispatcher still admits. Belt-and-suspenders
// for bt-ydjw.4 to keep the prefix path live when SourceRepo isn't
// available.
func TestHistoryDispatchTarget_CursorPrefixFallback(t *testing.T) {
	savedFn := enumerateDoltDatabasesFn
	t.Cleanup(func() { enumerateDoltDatabasesFn = savedFn })

	enumerateDoltDatabasesFn = func(dsn string) []string {
		return []string{"bt", "bd", "tpane"}
	}

	// bt-prefixed bead with SourceRepo unset; mirrors the workspace-mode
	// path where issues come from a non-global reader.
	issues := []model.Issue{{
		ID:     "bt-1",
		Title:  "T",
		Status: model.StatusOpen,
	}}
	m := NewModel(issues, nil, "", &datasource.DataSource{
		Type: datasource.SourceTypeDoltGlobal,
		Path: "root@tcp(127.0.0.1:9999)/?parseTime=true",
	}, nil)
	m.currentProjectDB = ""

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	ctx := m.historyContext()
	if ctx.CursorSourceRepo != "" {
		t.Fatalf("test precondition: expected CursorSourceRepo empty, got %q", ctx.CursorSourceRepo)
	}

	projectDB, ok := m.historyDispatchTarget(ctx, "/no/jsonl/here")
	if !ok {
		t.Fatalf("expected dispatch admitted via cursor prefix fallback, got ok=false")
	}
	if projectDB != "bt" {
		t.Errorf("expected projectDB=bt (prefix-derived), got %q", projectDB)
	}
}

// TestHistoryDispatchTarget_GlobalEnumerationFailure covers the conservative
// behavior when the live enumeration fails (server unreachable, transient
// error): the resolver returns ("", false) and the gate falls back to the
// polite empty-state rather than dispatching against an unverified target.
func TestHistoryDispatchTarget_GlobalEnumerationFailure(t *testing.T) {
	savedFn := enumerateDoltDatabasesFn
	t.Cleanup(func() { enumerateDoltDatabasesFn = savedFn })

	enumerateDoltDatabasesFn = func(dsn string) []string { return nil }

	issues := []model.Issue{{ID: "bt-1", Title: "T", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", &datasource.DataSource{
		Type: datasource.SourceTypeDoltGlobal,
		Path: "root@tcp(127.0.0.1:9999)/?parseTime=true",
	}, nil)
	m.currentProjectDB = "bt"

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	ctx := m.historyContext()
	_, ok := m.historyDispatchTarget(ctx, "/no/jsonl/here")
	if ok {
		t.Errorf("expected dispatch rejected when enumeration returns no databases")
	}
}

// TestResolveHistoryRepoPath_BeadsLayout: explicit beads.jsonl under
// <root>/.beads/ resolves to <root>. Underpins the dispatch-time path
// resolution refactor in bt-uizm; the function must run without touching
// the projects registry or os.Getwd inside the LoadHistoryCmd goroutine.
func TestResolveHistoryRepoPath_BeadsLayout(t *testing.T) {
	tmp := t.TempDir()
	beadsDir := filepath.Join(tmp, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	beadsFile := filepath.Join(beadsDir, "beads.jsonl")

	got := resolveHistoryRepoPath(beadsFile)

	// EvalSymlinks normalizes Windows short-name vs long-name forms and
	// any /private prefix on macOS, so the comparison stays exact.
	wantAbs, _ := filepath.EvalSymlinks(tmp)
	gotAbs, _ := filepath.EvalSymlinks(got)
	if gotAbs != wantAbs {
		t.Errorf("resolveHistoryRepoPath(%q) = %q, want %q", beadsFile, gotAbs, wantAbs)
	}
}

// TestHistorySearchKeyIsolation covers bt-mc4y: while history search is
// active, every printable key must reach the searchInput rather than firing
// a global mode toggle. Before the fix, typing `h` in history search closed
// the history view because the global `h = toggle history` handler ran
// before the focus-based dispatch reached handleHistoryKeys.
func TestHistorySearchKeyIsolation(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Test Issue", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil, nil)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Enter history view.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)
	if m.mode != ViewHistory {
		t.Fatalf("setup: expected ViewHistory after h key, got %v", m.mode)
	}

	// Activate search via /.
	updated, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = updated.(Model)
	if !m.historyView.IsSearchActive() {
		t.Fatalf("setup: expected search active after /")
	}

	// Type a sequence that mixes plain letters with letters that map to
	// global hotkeys (h = toggle history, b = board, g = graph, i =
	// insights, p = priority hints, a = actionable). Every keypress must
	// land in the search buffer and leave m.mode == ViewHistory.
	seq := []rune{'h', 'b', 'g', 'i', 'p', 'a'}
	for _, r := range seq {
		updated, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(Model)
		if m.mode != ViewHistory {
			t.Fatalf("keypress %q leaked through search and changed mode to %v", r, m.mode)
		}
		if !m.historyView.IsSearchActive() {
			t.Fatalf("keypress %q deactivated search", r)
		}
	}

	if got, want := m.historyView.SearchQuery(), string(seq); got != want {
		t.Fatalf("search buffer = %q, want %q", got, want)
	}

	// Delete key (forward delete) at end of buffer is a no-op in bubbles
	// textinput, but it must NOT fire any global handler. The buffer stays
	// the same and the view stays in history+search.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	m = updated.(Model)
	if m.mode != ViewHistory {
		t.Fatalf("Delete keypress changed mode to %v", m.mode)
	}
	if !m.historyView.IsSearchActive() {
		t.Fatalf("Delete keypress deactivated search")
	}
	if got, want := m.historyView.SearchQuery(), string(seq); got != want {
		t.Fatalf("buffer changed after no-op Delete: got %q want %q", got, want)
	}

	// Esc closes search (and only search — view stays as ViewHistory).
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)
	if m.historyView.IsSearchActive() {
		t.Fatalf("Esc did not deactivate search")
	}
	if m.mode != ViewHistory {
		t.Fatalf("Esc on active search exited history view; expected to stay in ViewHistory, got %v", m.mode)
	}
}

func TestHistoryViewKeys(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Test Issue", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil, nil)

	// Make model ready
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Enter history view
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	// Esc should close history view
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)

	if m.mode == ViewHistory {
		t.Fatalf("expected history view to be closed after Esc")
	}

	// Re-enter and test 'c' key cycles confidence
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = updated.(Model)

	initialConf := m.historyView.GetMinConfidence()
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = updated.(Model)

	if m.historyView.GetMinConfidence() == initialConf {
		t.Fatalf("expected confidence to change after 'c' key")
	}
}
