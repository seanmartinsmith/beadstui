package ui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/recipe"
	"github.com/seanmartinsmith/beadstui/pkg/watcher"
)

// Basic helpers and tiny behaviours that were previously uncovered.
func TestIssueItemBasicsAndPrefix(t *testing.T) {
	issue := model.Issue{
		ID:        "api-AUTH-123",
		Title:     "Auth plumbing",
		Status:    model.StatusOpen,
		IssueType: model.TypeFeature,
		Assignee:  "alice",
		Labels:    []string{"backend", "security"},
	}
	item := IssueItem{Issue: issue, RepoPrefix: ExtractRepoPrefix(issue.ID)}
	// item.ComputeFilterValue() // Removed call to undefined method

	if got := item.Title(); got != issue.Title {
		t.Fatalf("Title() = %s, want %s", got, issue.Title)
	}
	desc := item.Description()
	if !strings.Contains(desc, issue.ID) || !strings.Contains(desc, string(issue.Status)) {
		t.Fatalf("Description() missing pieces: %s", desc)
	}
	filter := item.FilterValue()
	for _, want := range []string{issue.Title, issue.ID, "backend", "security", "api"} {
		if !strings.Contains(filter, want) {
			t.Fatalf("FilterValue missing %q in %q", want, filter)
		}
	}
	if got := ExtractRepoPrefix("web:UI-9"); got != "web" {
		t.Fatalf("ExtractRepoPrefix wrong for colon sep: %s", got)
	}
	if got := ExtractRepoPrefix("noprefix"); got != "" {
		t.Fatalf("ExtractRepoPrefix expected empty, got %s", got)
	}
}

func TestRecipePickerIndexesAndCounts(t *testing.T) {
	loader := recipe.NewLoader()
	if err := loader.Load(); err != nil {
		t.Skipf("recipes not available: %v", err)
	}
	picker := NewRecipePickerModel(loader.List(), DefaultTheme())
	if picker.SelectedIndex() != 0 {
		t.Fatalf("initial SelectedIndex = %d, want 0", picker.SelectedIndex())
	}
	picker.MoveDown()
	if picker.SelectedIndex() == 0 {
		t.Fatalf("MoveDown did not change selection")
	}
	if picker.RecipeCount() != len(loader.List()) {
		t.Fatalf("RecipeCount mismatch")
	}
}

func TestRenderSubtleDivider(t *testing.T) {
	if out := RenderSubtleDivider(10); len(strings.TrimSpace(out)) == 0 {
		t.Fatalf("RenderSubtleDivider returned empty output")
	}
}

func TestParseCommandLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "simple",
			input: "code",
			want:  []string{"code"},
		},
		{
			name:  "args",
			input: "code --wait",
			want:  []string{"code", "--wait"},
		},
		{
			name:  "double_quoted_path",
			input: "\"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code\" --wait",
			want:  []string{"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code", "--wait"},
		},
		{
			name:  "single_quoted_arg",
			input: "open -a 'Visual Studio Code'",
			want:  []string{"open", "-a", "Visual Studio Code"},
		},
		{
			name:  "escaped_space",
			input: "open Visual\\ Studio",
			want:  []string{"open", "Visual Studio"},
		},
		{
			name:  "windows_path_in_quotes_preserves_backslashes",
			input: "\"C:\\Program Files\\VS Code\\Code.exe\" --wait",
			want:  []string{"C:\\Program Files\\VS Code\\Code.exe", "--wait"},
		},
		{
			name:    "unterminated_single_quote",
			input:   "open 'oops",
			wantErr: true,
		},
		{
			name:    "unterminated_double_quote",
			input:   "open \"oops",
			wantErr: true,
		},
		{
			name:    "trailing_escape",
			input:   "open \\",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCommandLine(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (got=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseCommandLine(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestHandleListKeysFiltersAndTimeTravelPrompt(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "One", Status: model.StatusOpen},
		{ID: "2", Title: "Two", Status: model.StatusOpen},
		{ID: "3", Title: "Three", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	m.height = 30
	m.width = 80
	m.focused = focusList
	m.isSplitView = false

	m = m.handleListKeys(tea.KeyPressMsg{Code: 'o', Text: "o"})
	if m.filter.currentFilter != "open" {
		t.Fatalf("expected filter 'open', got %s", m.filter.currentFilter)
	}
	m = m.handleListKeys(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if m.filter.currentFilter != "closed" {
		t.Fatalf("expected filter 'closed', got %s", m.filter.currentFilter)
	}
	m = m.handleListKeys(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if m.filter.currentFilter != "ready" {
		t.Fatalf("expected filter 'ready', got %s", m.filter.currentFilter)
	}

	// Paging up/down
	m.list.Select(0)
	m = m.handleListKeys(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if m.list.Index() == 0 {
		t.Fatalf("ctrl+d should move selection down")
	}
	m = m.handleListKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if m.list.Index() != 0 {
		t.Fatalf("ctrl+u should move selection up")
	}

	// Enter should flip showDetails in mobile view
	m.showDetails = false
	m = m.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.showDetails {
		t.Fatalf("enter should show details when not split view")
	}

	// Time-travel prompt toggling
	m.timeTravelMode = false
	m = m.handleListKeys(tea.KeyPressMsg{Code: 't', Text: "t"})
	if m.activeModal != ModalTimeTravelInput || m.focused != focusTimeTravelInput {
		t.Fatalf("time-travel prompt not activated")
	}
	// Cancel via Esc to avoid git dependency
	m = m.handleTimeTravelInputKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.activeModal == ModalTimeTravelInput {
		t.Fatalf("prompt should close on esc")
	}
	if m.focused != focusList {
		t.Fatalf("focus should return to list after esc")
	}
}

// TestHandleListKeysSortCycle verifies s/S forward/reverse sort cycling (bt-ktcr).
// S previously applied the triage recipe; it now mirrors bt's alerts-modal
// convention of s forward / S reverse. Triage moved to R.
func TestHandleListKeysSortCycle(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "One", Status: model.StatusOpen},
		{ID: "2", Title: "Two", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	m.height = 30
	m.width = 80
	m.focused = focusList
	m.isSplitView = false

	if m.filter.sortMode != SortDefault {
		t.Fatalf("initial sort mode should be SortDefault, got %v", m.filter.sortMode)
	}

	// Forward: s advances the mode
	m = m.handleListKeys(tea.KeyPressMsg{Code: 's', Text: "s"})
	forwardOne := m.filter.sortMode
	if forwardOne == SortDefault {
		t.Fatalf("s should advance sort mode")
	}

	// Forward again: lands on a different mode (or wraps cleanly)
	m = m.handleListKeys(tea.KeyPressMsg{Code: 's', Text: "s"})
	forwardTwo := m.filter.sortMode
	if forwardTwo == forwardOne {
		t.Fatalf("second s should advance again, got same mode %v", forwardTwo)
	}

	// Reverse: S brings us back to forwardOne
	m = m.handleListKeys(tea.KeyPressMsg{Code: 'S', Text: "S"})
	if m.filter.sortMode != forwardOne {
		t.Fatalf("S should reverse to previous mode %v, got %v", forwardOne, m.filter.sortMode)
	}

	// Reverse again: back to SortDefault
	m = m.handleListKeys(tea.KeyPressMsg{Code: 'S', Text: "S"})
	if m.filter.sortMode != SortDefault {
		t.Fatalf("S should reverse to SortDefault, got %v", m.filter.sortMode)
	}

	// Reverse past zero: wraps to last mode
	m = m.handleListKeys(tea.KeyPressMsg{Code: 'S', Text: "S"})
	if m.filter.sortMode != numSortModes-1 {
		t.Fatalf("S at SortDefault should wrap to %v, got %v", numSortModes-1, m.filter.sortMode)
	}
}

// TestHandleListKeysTriageRecipe verifies R applies the triage recipe (bt-ktcr).
// The recipe loader is nil-safe; if no triage recipe is loaded in the test
// environment, R is a no-op rather than a panic.
func TestHandleListKeysTriageRecipe(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "One", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	m.height = 30
	m.width = 80
	m.focused = focusList
	m.isSplitView = false

	// R must not panic regardless of whether a triage recipe is registered.
	m = m.handleListKeys(tea.KeyPressMsg{Code: 'R', Text: "R"})

	if r := m.filter.recipeLoader.Get("triage"); r != nil && m.filter.activeRecipe == nil {
		t.Fatalf("R should set activeRecipe when triage recipe exists")
	}
}

func TestClassifyEditorCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantBase string
		wantKind editorCommandKind
	}{
		{
			name:     "empty",
			args:     nil,
			wantBase: "",
			wantKind: editorCommandEmpty,
		},
		{
			name:     "terminal_editor",
			args:     []string{"VIM"},
			wantBase: "vim",
			wantKind: editorCommandTerminal,
		},
		{
			name:     "forbidden_shell_bash",
			args:     []string{"bash", "-lc", "echo hi"},
			wantBase: "bash",
			wantKind: editorCommandForbidden,
		},
		{
			name:     "forbidden_shell_pwsh_exe",
			args:     []string{"pwsh.exe", "-NoProfile"},
			wantBase: "pwsh",
			wantKind: editorCommandForbidden,
		},
		{
			name:     "gui_editor",
			args:     []string{"code", "--reuse-window"},
			wantBase: "code",
			wantKind: editorCommandOK,
		},
		{
			name:     "windows_path_gui_editor",
			args:     []string{`C:\Program Files\VS Code\Code.exe`, "--wait"},
			wantBase: "code",
			wantKind: editorCommandOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBase, gotKind := classifyEditorCommand(tt.args)
			if gotBase != tt.wantBase || gotKind != tt.wantKind {
				t.Fatalf("classifyEditorCommand(%v) = (%q, %v), want (%q, %v)", tt.args, gotBase, gotKind, tt.wantBase, tt.wantKind)
			}
		})
	}
}

func TestAllowlistedGUIEditorKindForBase(t *testing.T) {
	tests := []struct {
		base string
		want allowlistedGUIEditorKind
	}{
		{base: "code", want: allowlistedGUIEditorCode},
		{base: "code-insiders", want: allowlistedGUIEditorCodeInsiders},
		{base: "cursor", want: allowlistedGUIEditorCursor},
		{base: "xdg-open", want: allowlistedGUIEditorXdgOpen},
		{base: "notepad", want: allowlistedGUIEditorNotepad},
		{base: "open", want: allowlistedGUIEditorOpenText},
		{base: "unknown", want: allowlistedGUIEditorUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.base, func(t *testing.T) {
			if got := allowlistedGUIEditorKindForBase(tt.base); got != tt.want {
				t.Fatalf("allowlistedGUIEditorKindForBase(%q)=%v, want %v", tt.base, got, tt.want)
			}
		})
	}
}

func TestViewTogglesGraphBoardInsightsActionable(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Alpha", Status: model.StatusOpen},
		{ID: "B", Title: "Beta", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	// Prime layout so width/height are non-zero
	_, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})

	// Graph toggle
	modelAny, _ := m.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	m = modelAny.(Model)
	if m.mode != ViewGraph || m.focused != focusGraph {
		t.Fatalf("graph view not activated")
	}

	// Board toggle
	modelAny, _ = m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = modelAny.(Model)
	if m.mode != ViewBoard || m.focused != focusBoard {
		t.Fatalf("board view not activated")
	}

	// Insights toggle
	modelAny, _ = m.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	m = modelAny.(Model)
	if m.mode != ViewInsights || m.focused != focusInsights {
		t.Fatalf("insights not focused after toggle")
	}

	// Actionable toggle
	modelAny, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = modelAny.(Model)
	if m.mode != ViewActionable || m.focused != focusActionable {
		t.Fatalf("actionable view not activated")
	}

	// Priority hints toggle
	modelAny, _ = m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = modelAny.(Model)
	if !m.ac.showPriorityHints {
		t.Fatalf("priority hints should toggle on with 'p'")
	}

	// Recipe picker toggle (' key)
	modelAny, _ = m.Update(tea.KeyPressMsg{Code: '\'', Text: "'"})
	m = modelAny.(Model)
	if m.activeModal != ModalRecipePicker || m.focused != focusRecipePicker {
		t.Fatalf("recipe picker not opened correctly")
	}
}

func TestHandleGraphBoardActionableKeys(t *testing.T) {
	issues := []model.Issue{
		{ID: "X", Title: "Cross", Status: model.StatusOpen},
		{ID: "Y", Title: "Why", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	m.width, m.height = 120, 30

	// Focus graph and exercise navigation + enter selection logic
	m.mode = ViewGraph
	m.focused = focusGraph
	// force select first node then enter to sync list
	m.graphView.MoveDown()
	m = m.handleGraphKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode == ViewGraph {
		t.Fatalf("enter should exit graph view")
	}

	// Focus board navigation paths
	m.mode = ViewBoard
	m.focused = focusBoard
	m = m.handleBoardKeys(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = m.handleBoardKeys(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = m.handleBoardKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = m.handleBoardKeys(tea.KeyPressMsg{Code: 'k', Text: "k"})
	// Navigate back to Open column (with items) - Status mode shows all columns (bv-tf6j)
	m.board.JumpToFirstColumn()
	// Enter should exit board when selection exists
	m.board.MoveToTop()
	m = m.handleBoardKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode == ViewBoard {
		t.Fatalf("enter should exit board view")
	}

	// Actionable view enter selects matching issue in list
	plan := analysis.ExecutionPlan{
		Tracks: []analysis.ExecutionTrack{{
			TrackID: "t1",
			Items:   []analysis.PlanItem{{ID: "X", Title: "Cross", Status: "open"}},
		}},
		TotalActionable: 1,
	}
	m.mode = ViewActionable
	m.focused = focusActionable
	m.actionableView = NewActionableModel(plan, m.theme)
	m = m.handleActionableKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode == ViewActionable {
		t.Fatalf("enter should exit actionable view")
	}
}

func TestHandleRecipePickerAndInsightsKeys(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "One", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	m.width, m.height = 100, 20

	// Seed insights with a selected item
	ins := analysis.Insights{
		Bottlenecks: []analysis.InsightItem{{ID: "1", Value: 1}},
		Stats:       m.data.analysis,
	}
	m.insightsPanel = NewInsightsModel(ins, m.data.issueMap, m.theme)
	m.focused = focusInsights
	m = m.handleInsightsKeys(tea.KeyPressMsg{Code: 'l', Text: "l"})
	m = m.handleInsightsKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	m = m.handleInsightsKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m = m.handleInsightsKeys(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Recipe picker escape path
	m.openModal(ModalRecipePicker)
	m.focused = focusRecipePicker
	m = m.handleRecipePickerKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = m.handleRecipePickerKeys(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m = m.handleRecipePickerKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.activeModal == ModalRecipePicker {
		t.Fatalf("recipe picker should close on esc")
	}

	// Enter applies selection
	m.openModal(ModalRecipePicker)
	m.focused = focusRecipePicker
	m = m.handleRecipePickerKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.filter.activeRecipe == nil || m.activeModal == ModalRecipePicker {
		t.Fatalf("enter should apply recipe and close picker")
	}
}

func TestWaitForPhase2CmdCompletes(t *testing.T) {
	stats := analysis.NewGraphStatsForTest(
		map[string]float64{"A": 1},
		map[string]float64{"A": 1},
		map[string]float64{"A": 1},
		map[string]float64{"A": 1},
		map[string]float64{"A": 1},
		map[string]float64{"A": 1},
		map[string]int{"A": 0},
		map[string]int{"A": 0},
		nil,
		0,
		nil,
	)
	cmd := WaitForPhase2Cmd(stats)
	if msg := cmd(); msg == nil {
		t.Fatalf("expected Phase2ReadyMsg")
	}
}

func TestDiffStatusAndExitTimeTravel(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.timeTravelMode = true
	m.newIssueIDs = map[string]bool{"N": true}
	m.closedIssueIDs = map[string]bool{"C": true}
	m.modifiedIssueIDs = map[string]bool{"M": true}

	if got := m.getDiffStatus("N"); got != DiffStatusNew {
		t.Fatalf("expected DiffStatusNew, got %v", got)
	}
	if got := m.getDiffStatus("C"); got != DiffStatusClosed {
		t.Fatalf("expected DiffStatusClosed, got %v", got)
	}
	if got := m.getDiffStatus("M"); got != DiffStatusModified {
		t.Fatalf("expected DiffStatusModified, got %v", got)
	}
	if got := m.getDiffStatus("X"); got != DiffStatusNone {
		t.Fatalf("expected DiffStatusNone, got %v", got)
	}

	// exit path should clear maps and status
	m.exitTimeTravelMode()
	if m.timeTravelMode || m.newIssueIDs != nil || m.closedIssueIDs != nil || m.modifiedIssueIDs != nil {
		t.Fatalf("exitTimeTravelMode should clear state")
	}
	if m.statusMsg == "" {
		t.Fatalf("exitTimeTravelMode should set status message")
	}
}

func TestRenderFooterStatusAndBadges(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200 // wide enough to avoid width-based badge dropping (bt-m9te)

	// status message branch
	m.statusMsg = "Saved"
	m.statusSeverity = SeveritySuccess
	footer := m.renderFooter()
	if !strings.Contains(footer, "Saved") {
		t.Fatalf("footer should include status message")
	}
	if !strings.Contains(footer, "✓") {
		t.Fatalf("footer should include success icon")
	}

	// badges branch
	m.statusMsg = ""
	m.filter.currentFilter = "ready"
	m.ac.countOpen, m.ac.countReady, m.ac.countBlocked, m.ac.countClosed = 1, 2, 3, 4
	m.updateAvailable = true
	m.updateTag = "v9.9.9"
	m.workspaceMode = true
	m.workspaceSummary = "2 projects"
	footer = m.renderFooter()
	for _, expect := range []string{"READY", "◉", "⭐", "📦"} {
		if !strings.Contains(footer, expect) {
			t.Fatalf("footer missing %s: %s", expect, footer)
		}
	}
}

func TestRenderFooter_FreshnessIndicatorLevels(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 140
	m.filter.currentFilter = "all"

	m.data.backgroundWorker = &BackgroundWorker{}
	m.data.snapshot = &DataSnapshot{CreatedAt: time.Now()}

	// Fresh (<30s): no indicator
	out := m.renderFooter()
	if strings.Contains(out, "⚠") || strings.Contains(out, "STALE") || strings.Contains(out, "✗") {
		t.Fatalf("expected no freshness indicator when fresh, got: %q", out)
	}

	// Warn (>=30s)
	m.data.snapshot.CreatedAt = time.Now().Add(-45 * time.Second)
	out = m.renderFooter()
	if !strings.Contains(out, "⚠") || strings.Contains(out, "STALE") {
		t.Fatalf("expected warning freshness indicator, got: %q", out)
	}

	// Stale (>=2m)
	m.data.snapshot.CreatedAt = time.Now().Add(-3 * time.Minute)
	out = m.renderFooter()
	if !strings.Contains(out, "STALE") {
		t.Fatalf("expected stale freshness indicator, got: %q", out)
	}

	// Error (>=3 consecutive errors)
	m.data.backgroundWorker.lastError = &WorkerError{
		Phase:   "load",
		Time:    time.Now().Add(-5 * time.Second),
		Retries: 3,
	}
	out = m.renderFooter()
	if !strings.Contains(out, "✗") || !strings.Contains(out, "3x") {
		t.Fatalf("expected error freshness indicator, got: %q", out)
	}
}

func TestRenderFooter_DoltVerifiedPreventsStale(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 140
	m.filter.currentFilter = "all"

	m.data.backgroundWorker = &BackgroundWorker{}
	// Snapshot was built 5 minutes ago (would normally show STALE)
	m.data.snapshot = &DataSnapshot{CreatedAt: time.Now().Add(-5 * time.Minute)}

	// Without Dolt verification, this should show STALE
	out := m.renderFooter()
	if !strings.Contains(out, "STALE") {
		t.Fatalf("expected STALE without Dolt verification, got: %q", out)
	}

	// With recent Dolt verification (bt-3ynd), STALE should be suppressed
	m.lastDoltVerified = time.Now().Add(-3 * time.Second)
	out = m.renderFooter()
	if strings.Contains(out, "STALE") || strings.Contains(out, "⚠") {
		t.Fatalf("expected no staleness indicator with recent Dolt verification, got: %q", out)
	}

	// If Dolt verification itself is old, show STALE based on verification age
	m.lastDoltVerified = time.Now().Add(-3 * time.Minute)
	out = m.renderFooter()
	if !strings.Contains(out, "STALE") {
		t.Fatalf("expected STALE when Dolt verification is old, got: %q", out)
	}
}

func TestView_LoadingScreen_TransitionsOnFirstSnapshotOrError(t *testing.T) {
	issues := []model.Issue{{
		ID:        "L-1",
		Title:     "Loading Test",
		Status:    model.StatusOpen,
		Priority:  1,
		IssueType: model.TypeTask,
		CreatedAt: time.Now(),
	}}

	m := NewModel(issues, nil, "", nil)
	m.width, m.height = 120, 30
	m.data.backgroundWorker = &BackgroundWorker{state: WorkerProcessing}
	m.data.snapshot = nil
	m.data.snapshotInitPending = true

	if out := m.View().Content; !strings.Contains(out, "Loading beads") {
		t.Fatalf("expected loading screen before first snapshot, got: %q", out)
	}

	// Error should exit the loading screen (we already have initial data).
	modelAny, _ := m.Update(SnapshotErrorMsg{Err: errors.New("boom"), Recoverable: true})
	mErr := modelAny.(Model)
	if out := mErr.View().Content; strings.Contains(out, "Loading beads") {
		t.Fatalf("expected loading screen to clear on error, got: %q", out)
	}

	// Snapshot should exit the loading screen.
	m.data.snapshotInitPending = true
	snap := NewSnapshotBuilder(issues).Build()
	modelAny, _ = m.Update(SnapshotReadyMsg{Snapshot: snap})
	mOK := modelAny.(Model)
	if out := mOK.View().Content; strings.Contains(out, "Loading beads") {
		t.Fatalf("expected loading screen to clear on first snapshot, got: %q", out)
	}
}

func TestRenderFooter_ShowsPhase2ProgressBadge(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 200 // wide enough to avoid width-based badge dropping (bt-m9te)
	m.data.snapshot = &DataSnapshot{Phase2Ready: false}

	out := m.renderFooter()
	if !strings.Contains(out, "metrics") {
		t.Fatalf("expected phase 2 progress badge, got: %q", out)
	}
}

func TestRenderFooter_ShowsWorkerHealthIndicators(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 140
	m.filter.currentFilter = "all"
	m.data.snapshot = &DataSnapshot{CreatedAt: time.Now()}

	m.data.backgroundWorker = &BackgroundWorker{
		started:          true,
		heartbeatTimeout: time.Second,
		lastHeartbeat:    time.Now().Add(-2 * time.Second),
	}
	out := m.renderFooter()
	if !strings.Contains(out, "unresponsive") {
		t.Fatalf("expected worker unresponsive indicator, got: %q", out)
	}

	m.data.backgroundWorker = &BackgroundWorker{
		started:          true,
		heartbeatTimeout: time.Second,
		lastHeartbeat:    time.Now(),
		recoveryCount:    2,
	}
	out = m.renderFooter()
	if !strings.Contains(out, "recovered x2") {
		t.Fatalf("expected worker recovered indicator, got: %q", out)
	}
}

func TestExportToMarkdownSmoke(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	issues := []model.Issue{{
		ID:        "R-1",
		Title:     "Report me",
		Status:    model.StatusOpen,
		Priority:  1,
		IssueType: model.TypeTask,
		CreatedAt: time.Now(),
	}}
	m := NewModel(issues, nil, "", nil)
	m.exportToMarkdown()

	files, _ := os.ReadDir(".")
	if len(files) == 0 {
		t.Fatalf("expected export file to be written")
	}
	if m.statusSeverity >= SeverityFailure {
		t.Fatalf("exportToMarkdown should succeed, got error status")
	}
}

func TestGraphConnectorDown(t *testing.T) {
	theme := DefaultTheme()
	g := &GraphModel{theme: theme}

	if out := g.renderConnectorDown(0, 20, theme); out != "" {
		t.Fatalf("count 0 should return empty string")
	}
	if out := g.renderConnectorDown(1, 10, theme); !strings.Contains(out, "▼") {
		t.Fatalf("single connector missing arrow")
	}
	if out := g.renderConnectorDown(3, 20, theme); !strings.Contains(out, "┼") {
		t.Fatalf("multi connector should include fan pattern, got %q", out)
	}
}

func TestCopyIssueToClipboardNoSelection(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.copyIssueToClipboard()
	if m.statusSeverity < SeverityNotice || !strings.Contains(m.statusMsg, "No issue selected") {
		t.Fatalf("expected notice status for missing selection")
	}
}

func TestOpenInEditorTerminalEditorGuard(t *testing.T) {
	tmp := t.TempDir()
	oldCwd, _ := os.Getwd()
	defer os.Chdir(oldCwd)
	_ = os.MkdirAll(filepath.Join(tmp, ".beads"), 0755)
	_ = os.WriteFile(filepath.Join(tmp, ".beads", "beads.jsonl"), []byte("{}"), 0644)
	_ = os.Chdir(tmp)

	origEditor := os.Getenv("EDITOR")
	defer os.Setenv("EDITOR", origEditor)
	_ = os.Setenv("EDITOR", "vim") // triggers terminal-editor guard, no exec

	m := NewModel(nil, nil, "", nil)
	m.openInEditor()
	if m.statusSeverity < SeverityFailure || !strings.Contains(m.statusMsg, "terminal editor") {
		t.Fatalf("expected terminal editor warning, got %q", m.statusMsg)
	}
}

func TestOpenInEditorWithArguments(t *testing.T) {
	// Test that EDITOR with arguments (e.g., "cursor -w") works correctly
	// This tests the fix for GitHub issue #47
	if runtime.GOOS == "windows" {
		t.Skip("shell execution test unreliable on Windows CI")
	}
	tmp := t.TempDir()
	oldCwd, _ := os.Getwd()
	defer os.Chdir(oldCwd)
	_ = os.MkdirAll(filepath.Join(tmp, ".beads"), 0755)
	_ = os.WriteFile(filepath.Join(tmp, ".beads", "beads.jsonl"), []byte("{}"), 0644)
	_ = os.Chdir(tmp)

	origEditor := os.Getenv("EDITOR")
	defer os.Setenv("EDITOR", origEditor)

	// Test with EDITOR containing arguments - "true" is a POSIX command that just exits 0
	// Using "true --" simulates EDITOR with arguments like "cursor -w"
	_ = os.Setenv("EDITOR", "true --")

	m := NewModel(nil, nil, "", nil)
	m.openInEditor()
	// Should succeed - the shell should parse "true --" correctly
	if m.statusSeverity >= SeverityFailure {
		t.Fatalf("expected success with EDITOR containing arguments, got error: %q", m.statusMsg)
	}
	if !strings.Contains(m.statusMsg, "Opened in") {
		t.Fatalf("expected 'Opened in' message, got %q", m.statusMsg)
	}

	// Also test terminal editor detection with arguments (e.g., "vim -u NONE")
	_ = os.Setenv("EDITOR", "vim -u NONE")
	m2 := NewModel(nil, nil, "", nil)
	m2.openInEditor()
	if m2.statusSeverity < SeverityFailure || !strings.Contains(m2.statusMsg, "terminal editor") {
		t.Fatalf("expected terminal editor warning for 'vim -u NONE', got %q", m2.statusMsg)
	}
}

func TestGraphPageDownEmpty(t *testing.T) {
	g := NewGraphModel(nil, nil, DefaultTheme())
	g.PageDown() // len=0 branch
}

func TestRenderFooterErrorStatus(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 40
	m.statusMsg = "boom"
	m.statusSeverity = SeverityFailure
	out := m.renderFooter()
	if !strings.Contains(out, "boom") {
		t.Fatalf("footer should show error status")
	}
	if !strings.Contains(out, "✗") {
		t.Fatalf("footer should show error icon")
	}
}

func TestRenderFooter_CombinedIndicators(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "beads.jsonl")
	if err := os.WriteFile(f, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	w, err := watcher.NewWatcher(
		f,
		watcher.WithForcePoll(true),
		watcher.WithPollInterval(10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("watcher: %v", err)
	}
	if err := w.Start(); err != nil {
		t.Fatalf("watcher start: %v", err)
	}
	t.Cleanup(func() { w.Stop() })

	m := NewModel(nil, nil, "", nil)
	m.width = 300 // wide enough to avoid width-based badge dropping (bt-m9te);
	// bumped from 200 in bt-ift6.2 because ListNormalKeys.ShortHelp() expanded the
	// L1 hint chain (~80 chars), squeezing tier-1 badges (phase2 "metrics", watcher
	// "polling") out at 200. The footer compression rule itself is unchanged.
	m.filter.currentFilter = "ready"
	m.ac.countOpen, m.ac.countReady, m.ac.countBlocked, m.ac.countClosed = 1, 2, 3, 4
	m.updateAvailable = true
	m.updateTag = "v9.9.9"
	m.data.snapshot = &DataSnapshot{CreatedAt: time.Now(), Phase2Ready: false}
	m.data.backgroundWorker = &BackgroundWorker{
		started:          true,
		state:            WorkerIdle,
		lastHeartbeat:    time.Now(),
		heartbeatTimeout: 5 * time.Second,
		recoveryCount:    2,
		watcher:          w,
	}

	out := m.renderFooter()
	for _, expect := range []string{"READY", "metrics", "recovered x2", "polling", "⭐"} {
		if !strings.Contains(out, expect) {
			t.Fatalf("footer missing %q: %q", expect, out)
		}
	}
}

func TestRenderSplitAndListViews(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Alpha", Status: model.StatusOpen},
		{ID: "2", Title: "Beta", Status: model.StatusClosed},
	}
	m := NewModel(issues, nil, "", nil)

	// Prime layout into split view
	modelAny, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 40})
	m = modelAny.(Model)
	m.isSplitView = true
	out := m.renderSplitView()
	if !strings.Contains(out, "Alpha") || !strings.Contains(out, "Beta") {
		t.Fatalf("renderSplitView missing issue titles: %s", out)
	}

	// Mobile/list-only view path
	m.isSplitView = false
	m.showDetails = false
	m.ready = true
	m.width = 90
	m.height = 30
	listOut := m.renderListWithHeader()
	if !strings.Contains(listOut, "Alpha") {
		t.Fatalf("renderListWithHeader missing content: %s", listOut)
	}
}

func TestInitAndStopNoWatcher(t *testing.T) {
	m := NewModel([]model.Issue{{ID: "1", Title: "x", Status: model.StatusOpen}}, nil, "", nil)
	if cmd := m.Init(); cmd == nil {
		t.Fatalf("Init should return a command batch")
	}
	// Stop should be safe when watcher is nil
	m.Stop()

	// Stop with real watcher
	tmp := t.TempDir()
	f := filepath.Join(tmp, "beads.jsonl")
	_ = os.WriteFile(f, []byte("{}"), 0o644)
	w, err := watcher.NewWatcher(f, watcher.WithForcePoll(true), watcher.WithPollInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("watcher create: %v", err)
	}
	_ = w.Start()
	m.data.watcher = w
	m.Stop()
	if w.IsStarted() {
		t.Fatalf("watcher should be stopped")
	}
}

func TestBoardAndInsightsExtraKeys(t *testing.T) {
	issues := []model.Issue{{ID: "1", Title: "One", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", nil)
	m.width, m.height = 120, 30

	// Board page up/down coverage
	m.mode = ViewBoard
	m.focused = focusBoard
	m = m.handleBoardKeys(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	m = m.handleBoardKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	m = m.handleBoardKeys(tea.KeyPressMsg{Code: tea.KeyHome})
	m = m.handleBoardKeys(tea.KeyPressMsg{Code: tea.KeyEnd})

	// Insights escape and tab navigation
	m.focused = focusInsights
	m = m.handleInsightsKeys(tea.KeyPressMsg{Code: 'h', Text: "h"})
	m = m.handleInsightsKeys(tea.KeyPressMsg{Code: tea.KeyTab})
	m = m.handleInsightsKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.focused != focusList {
		t.Fatalf("Esc should return focus to list")
	}

	// Time-travel input enter path (will fail gracefully without git)
	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	m.openModal(ModalTimeTravelInput)
	m.focused = focusTimeTravelInput
	m.timeTravelInput.SetValue("HEAD~1")
	m = m.handleTimeTravelInputKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.statusSeverity < SeverityFailure && m.statusMsg == "" {
		t.Fatalf("expected status message after attempting time-travel without git")
	}
}

func TestOpenInEditorMissingAndGUI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("openInEditor GUI path is unreliable on headless Windows CI")
	}
	// Missing beads file branch
	tmp := t.TempDir()
	origWD, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	_ = os.Chdir(tmp)

	m := NewModel([]model.Issue{{ID: "1", Title: "x", Status: model.StatusOpen}}, nil, "", nil)
	m.openInEditor()
	if m.statusSeverity < SeverityFailure || !strings.Contains(m.statusMsg, "No .beads") {
		t.Fatalf("expected missing beads error, got %q", m.statusMsg)
	}

	// Success branch with GUI-ish editor
	beadsDir := filepath.Join(tmp, ".beads")
	_ = os.Mkdir(beadsDir, 0o755)
	_ = os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(`{}`), 0o644)

	origEditor := os.Getenv("EDITOR")
	t.Cleanup(func() { _ = os.Setenv("EDITOR", origEditor) })
	_ = os.Setenv("EDITOR", "true") // present on POSIX; not in terminal editor block

	m.openInEditor()
	if m.statusSeverity >= SeverityFailure || !strings.Contains(m.statusMsg, "Opened in") {
		t.Fatalf("expected success opening editor, got %q", m.statusMsg)
	}
}

func TestExportToMarkdownCreatesFile(t *testing.T) {
	tmp := t.TempDir()
	origWD, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	_ = os.Chdir(tmp)

	issues := []model.Issue{{ID: "1", Title: "Alpha", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", nil)
	filename := m.generateExportFilename()

	m.exportToMarkdown()

	if _, err := os.Stat(filepath.Join(tmp, filename)); err != nil {
		t.Fatalf("expected export file to exist: %v", err)
	}
	if m.statusSeverity >= SeverityFailure {
		t.Fatalf("export should succeed, got error %q", m.statusMsg)
	}
}

func TestWatchFileCmdDetectsChange(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "file.txt")
	_ = os.WriteFile(file, []byte("hi"), 0o644)

	w, err := watcher.NewWatcher(file,
		watcher.WithForcePoll(true),
		watcher.WithPollInterval(20*time.Millisecond),
		watcher.WithDebounceDuration(10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	if err := w.Start(); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Stop()

	// Modify the file after a short delay
	go func() {
		time.Sleep(40 * time.Millisecond)
		_ = os.WriteFile(file, []byte("bye"), 0o644)
	}()

	cmd := WatchFileCmd(w)
	msg := cmd()
	if _, ok := msg.(FileChangedMsg); !ok {
		t.Fatalf("expected FileChangedMsg, got %T", msg)
	}
}

func TestRenderFooterVariantsAndDiffStatus(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "One", Status: model.StatusOpen},
		{ID: "2", Title: "Two", Status: model.StatusClosed},
	}
	m := NewModel(issues, nil, "", nil)
	m.width = 200 // wide enough to avoid width-based badge dropping (bt-m9te)
	m.height = 20
	m.ready = true

	// Status message branch
	m.statusMsg = "All good"
	m.statusSeverity = SeveritySuccess
	out := m.renderFooter()
	if !strings.Contains(out, "All good") {
		t.Fatalf("status footer missing message: %s", out)
	}

	// Time-travel + update + workspace branch
	m.statusMsg = "" // disable status override
	m.timeTravelMode = true
	m.timeTravelSince = "HEAD~1"
	m.timeTravelDiff = &analysis.SnapshotDiff{
		Summary: analysis.DiffSummary{
			IssuesAdded:    2,
			IssuesClosed:   1,
			IssuesModified: 3,
		},
	}
	m.updateAvailable = true
	m.updateTag = "v9.9.9"
	m.workspaceMode = true
	m.workspaceSummary = "2 projects"
	m.ac.countOpen = 1
	m.ac.countReady = 1
	m.ac.countBlocked = 0
	m.ac.countClosed = 1

	out = m.renderFooter()
	for _, want := range []string{"⏱", "v9.9.9", "2 projects"} {
		if !strings.Contains(out, want) {
			t.Fatalf("footer missing %q in %q", want, out)
		}
	}

	// Diff status mapping
	m.newIssueIDs = map[string]bool{"n": true}
	m.closedIssueIDs = map[string]bool{"c": true}
	m.modifiedIssueIDs = map[string]bool{"m": true}
	m.timeTravelMode = true
	if got := m.getDiffStatus("n"); got != DiffStatusNew {
		t.Fatalf("new diff status mismatch: %v", got)
	}
	if got := m.getDiffStatus("c"); got != DiffStatusClosed {
		t.Fatalf("closed diff status mismatch: %v", got)
	}
	if got := m.getDiffStatus("m"); got != DiffStatusModified {
		t.Fatalf("modified diff status mismatch: %v", got)
	}
	if got := m.getDiffStatus("z"); got != DiffStatusNone {
		t.Fatalf("none diff status mismatch: %v", got)
	}
}

func TestGraphRenderBlocksAndDependents(t *testing.T) {
	issues := []model.Issue{
		{ID: "EGO", Title: "Center", Status: model.StatusOpen},
	}
	ins := analysis.Insights{Stats: analysis.NewGraphStatsForTest(
		nil, nil, nil, nil, nil, nil,
		map[string]int{"EGO": 0}, map[string]int{"EGO": 0},
		nil, 0, nil,
	)}
	g := NewGraphModel(issues, &ins, DefaultTheme())

	blockers := []string{"B1", "B2", "B3", "B4", "B5", "B6"}
	dependents := []string{"D1", "D2", "D3"}
	blockOut := g.renderBlockersVisual(blockers, 80, g.theme)
	blockStripped := ansi.Strip(blockOut)
	if !strings.Contains(blockStripped, "+1") || !strings.Contains(blockStripped, "more") {
		t.Fatalf("blockers visual should include +N more badge, got: %q", blockStripped)
	}
	depOut := g.renderDependentsVisual(dependents, 80, g.theme)
	depStripped := ansi.Strip(depOut)
	if !strings.Contains(depStripped, "D1") || !strings.Contains(depStripped, "D3") {
		t.Fatalf("dependents visual missing entries: %s", depStripped)
	}
}

func TestViewVariantsCoverBranches(t *testing.T) {
	issues := []model.Issue{{ID: "1", Title: "One", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", nil)
	m.ready = true
	m.width, m.height = 120, 30

	// Quit confirm
	m.openModal(ModalQuitConfirm)
	_ = m.View()

	// Time-travel prompt
	m.openModal(ModalTimeTravelInput)
	_ = m.View()

	// Recipe picker
	m.openModal(ModalRecipePicker)
	_ = m.View()

	// Help
	m.openModal(ModalHelp)
	_ = m.View()

	// Insights view
	m.closeModal()
	m.mode = ViewInsights
	m.focused = focusInsights
	_ = m.View()

	// Graph view
	m.mode = ViewGraph
	m.focused = focusGraph
	_ = m.View()

	// Board view
	m.mode = ViewBoard
	_ = m.View()

	// Actionable view
	m.mode = ViewActionable
	_ = m.View()

	// Split view
	m.mode = ViewList
	m.isSplitView = true
	_ = m.View()
}

func TestUpdateMouseAndResize(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "One", Status: model.StatusOpen},
		{ID: "2", Title: "Two", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)

	// Window size
	_, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})

	// Mouse wheel up/down in list focus
	m.focused = focusList
	_, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	_, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})

	// Switch focus and scroll other components
	m.focused = focusDetail
	_, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m.focused = focusInsights
	_, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	m.focused = focusBoard
	_, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	m.focused = focusGraph
	_, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m.focused = focusActionable
	_, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
}

func TestOverlaysAndWorkspaceHelpers(t *testing.T) {
	issues := []model.Issue{
		{ID: "W-1", Title: "Workspace", Status: model.StatusOpen},
	}
	m := NewModel(issues, nil, "", nil)
	if updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30}); updated != nil {
		m = updated.(Model)
	}

	// Workspace state
	m.EnableWorkspaceMode(WorkspaceInfo{Enabled: true, RepoCount: 2, RepoPrefixes: []string{"api", "web"}})
	if !m.IsWorkspaceMode() {
		t.Fatalf("workspace mode should be enabled")
	}

	// Quit confirm overlay (bt-yly4: title is "Quit?", not "Quit bt?")
	m.openModal(ModalQuitConfirm)
	if !strings.Contains(m.View().Content, "Quit?") {
		t.Fatalf("quit overlay should render")
	}
	m.closeModal()

	// Help overlay
	m.openModal(ModalHelp)
	if !strings.Contains(m.View().Content, "shortcuts") {
		t.Fatalf("help overlay should render")
	}
	m.closeModal()

	// Time-travel prompt render path (no git calls)
	m.openModal(ModalTimeTravelInput)
	m.timeTravelInput.SetValue("HEAD~1")
	if out := m.renderTimeTravelPrompt(); !strings.Contains(out, "Time-Travel Mode") {
		t.Fatalf("time-travel prompt text missing")
	}
	m.closeModal()

	// Export filename helper (no filesystem writes)
	name := m.generateExportFilename()
	if !strings.HasPrefix(name, "beads_report_") || !strings.HasSuffix(name, ".md") {
		t.Fatalf("generateExportFilename unexpected: %s", name)
	}
}

func TestGraphIconsAndTruncation(t *testing.T) {
	if getTypeIcon(model.TypeBug) == "" || getPriorityIcon(1) == "" {
		t.Fatalf("graph icons should not be empty")
	}
	if got := smartTruncateID("very_long_identifier_with_parts", 8); len([]rune(got)) > 8 {
		t.Fatalf("smartTruncateID should respect max length, got %s", got)
	}
	if smartTruncateID("id", 0) != "" {
		t.Fatalf("smartTruncateID should return empty when maxLen<=0")
	}
}

func TestHelpOverlayScroll(t *testing.T) {
	issues := []model.Issue{{ID: "1", Title: "One", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", nil)
	m.width, m.height = 80, 20 // Small terminal to force scroll
	m.openModal(ModalHelp)
	m.focused = focusHelp
	m.helpScroll = 0

	// Test scroll down
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.helpScroll != 1 {
		t.Fatalf("expected helpScroll=1 after j, got %d", m.helpScroll)
	}

	// Test scroll up
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.helpScroll != 0 {
		t.Fatalf("expected helpScroll=0 after k, got %d", m.helpScroll)
	}

	// Test scroll up at top (should stay at 0)
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.helpScroll != 0 {
		t.Fatalf("expected helpScroll=0 at top, got %d", m.helpScroll)
	}

	// Test page down (clamped to content)
	m.helpScroll = 0
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	wantDown := 10
	if max := m.helpScrollMax(); wantDown > max {
		wantDown = max
	}
	if m.helpScroll != wantDown {
		t.Fatalf("expected helpScroll=%d after ctrl+d, got %d", wantDown, m.helpScroll)
	}

	// Test page up
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if m.helpScroll != 0 {
		t.Fatalf("expected helpScroll=0 after ctrl+u, got %d", m.helpScroll)
	}

	// Test home
	m.helpScroll = 5
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if m.helpScroll != 0 {
		t.Fatalf("expected helpScroll=0 after g, got %d", m.helpScroll)
	}

	// Test end -> bottom (clamped to content, not a 999 sentinel)
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if m.helpScroll != m.helpScrollMax() {
		t.Fatalf("expected helpScroll=helpScrollMax after G, got %d (max %d)", m.helpScroll, m.helpScrollMax())
	}

	// Test q closes help
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if m.activeModal == ModalHelp {
		t.Fatalf("expected modal closed after q")
	}
	if m.helpScroll != 0 {
		t.Fatalf("expected helpScroll=0 after closing, got %d", m.helpScroll)
	}

	// Test any other key closes help
	m.openModal(ModalHelp)
	m.focused = focusHelp
	m.helpScroll = 5
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if m.activeModal == ModalHelp {
		t.Fatalf("expected modal closed after x")
	}

	// Test render help overlay
	m.openModal(ModalHelp)
	m.focused = focusHelp
	m.helpScroll = 0
	out := m.renderHelpOverlay()
	if !strings.Contains(out, "shortcuts") {
		t.Fatalf("help overlay should render the global title")
	}
	// Should show close + cross-reference hints.
	if !strings.Contains(out, "Esc") || !strings.Contains(out, ";") {
		t.Fatalf("help overlay should show close hint and ; cross-reference")
	}

	// Test Space key closes help for tutorial entry (bv-0trk)
	m.openModal(ModalHelp)
	m.focused = focusHelp
	m.helpScroll = 5
	m = m.handleHelpKeys(tea.KeyPressMsg{Code: ' ', Text: " "})
	if m.activeModal == ModalHelp {
		t.Fatalf("expected modal closed after Space")
	}
	if m.helpScroll != 0 {
		t.Fatalf("expected helpScroll=0 after Space, got %d", m.helpScroll)
	}
}

func TestRenderHelpOverlay_ResponsiveLayout(t *testing.T) {
	// Test that help overlay renders without panic at various widths (bt-aog1)
	widths := []struct {
		name  string
		width int
	}{
		{"wide", 200},
		{"medium", 100},
		{"narrow", 60},
		{"tiny", 40},
	}

	for _, tc := range widths {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(nil, nil, "", nil)
			m.width = tc.width
			m.height = 40
			m.openModal(ModalHelp)
			m.focused = focusHelp
			out := m.renderHelpOverlay()
			if !strings.Contains(out, "shortcuts") {
				t.Fatalf("help overlay should contain title at width=%d", tc.width)
			}
		})
	}
}

// TestHelpOverlay_ConsumesKeyMapBindings verifies the ? overlay is scoped to the
// Global map only (bt-dx7k): global binding descs appear; view-specific descs
// (ListNormal's "epic card" / "cycle sort") do not. Source is still the key.Map
// FullHelp(), never literal tables.
func TestHelpOverlay_ConsumesKeyMapBindings(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.mode = ViewList // a populated view, to prove its keys are still excluded
	m.width, m.height = 180, 60
	m.openModal(ModalHelp)
	m.focused = focusHelp

	out := m.renderHelpOverlay()

	// Global descs must be present.
	for _, want := range []string{"board", "graph", "refresh", "label picker"} {
		if !strings.Contains(out, want) {
			t.Errorf("? overlay missing global binding desc %q", want)
		}
	}
	// View-specific descs (ListNormal) must NOT be present -- ? is global-only.
	for _, absent := range []string{"epic card", "cycle sort"} {
		if strings.Contains(out, absent) {
			t.Errorf("? overlay leaked view-specific desc %q (should be global-only)", absent)
		}
	}
}

func TestHelpOverlayColumns(t *testing.T) {
	cases := []struct {
		width, want int
	}{
		{130, 4}, {120, 4},
		{119, 2}, {100, 2}, {80, 2},
		{79, 1}, {50, 1}, {30, 1},
	}
	for _, c := range cases {
		if got := helpOverlayColumns(c.width); got != c.want {
			t.Errorf("helpOverlayColumns(%d) = %d, want %d", c.width, got, c.want)
		}
	}
}

// TestHelpOverlayScroll_WindowsContent verifies help overlay behaviour at a
// narrow short height (bt-dx7k.1 tier-flip adaptation). At 60x16 the full body
// overflows so renderHelpOverlay returns the non-scrolling mini card. The scroll
// handler math (helpScrollMax) still holds — the overflow count is valid — but
// the render surface is now the mini, not the full sheet. The "scrolling changes
// the render" premise no longer applies to this size class.
func TestHelpOverlayScroll_WindowsContent(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.width, m.height = 60, 16 // narrow + short -> 1 column, overflows -> mini tier
	m.openModal(ModalHelp)
	m.focused = focusHelp

	// The overflow math is still valid: bodyLines > avail at this size.
	max := m.helpScrollMax()
	if max <= 0 {
		t.Fatalf("expected content to overflow at 60x16 (helpScrollMax>0), got %d", max)
	}

	// At overflow size, renderHelpOverlay returns the mini card (non-scrolling).
	m.helpScroll = 0
	out := m.renderHelpOverlay()
	if !strings.Contains(out, "↓ expand") {
		t.Fatalf("expected mini card at 60x16 (missing '↓ expand' nudge): %s", out)
	}
	// Full-sheet group headers must NOT appear in the mini.
	for _, absent := range []string{"SWITCH VIEWS", "WORKSPACE", "CHROME"} {
		if strings.Contains(out, absent) {
			t.Fatalf("mini at 60x16 unexpectedly shows full-sheet header %q", absent)
		}
	}

	// The mini renders the same card regardless of helpScroll (non-scrolling).
	// Verify over-scroll doesn't panic and still returns the mini.
	m.helpScroll = max + 50
	over := m.renderHelpOverlay()
	if !strings.Contains(over, "↓ expand") {
		t.Fatalf("over-scroll should still show mini card at 60x16: %s", over)
	}
}

// TestHelpOverlay_CrossRefFooter verifies the ? overlay footer carries the
// one-line cross-reference to the ; sidebar plus a close hint (bt-dx7k).
func TestHelpOverlay_CrossRefFooter(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.width, m.height = 120, 40
	m.openModal(ModalHelp)
	m.focused = focusHelp

	out := m.renderHelpOverlay()
	if !strings.Contains(out, ";") {
		t.Errorf("? footer should cross-reference the ; sidebar")
	}
	if !strings.Contains(out, "Esc") {
		t.Errorf("? footer should show a close hint")
	}
}

// TestHelpOverlayMini_ShownWhenShort verifies the mini card renders (not the
// full sheet) when the terminal is too short for the full body (bt-dx7k.1 Task 2).
// Descs are sourced from GlobalKeys.Board.Help() etc. (single-source, no literals).
// "label" matches the LabelPicker desc "label picker"; the task spec used "labels"
// as a shorthand for the label-entry slot.
func TestHelpOverlayMini_ShownWhenShort(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.width, m.height = 80, 12
	m.openModal(ModalHelp)
	m.focused = focusHelp
	out := m.renderHelpOverlay()

	// Mini descs must be present (sourced from GlobalKeys bindings).
	for _, want := range []string{"board", "search", "label", "help"} {
		if !strings.Contains(out, want) {
			t.Errorf("mini card at 80x12 missing desc %q: %s", want, out)
		}
	}
	// Nudge and per-view pointer.
	if !strings.Contains(out, "↓ expand") {
		t.Errorf("mini card at 80x12 missing '↓ expand' nudge: %s", out)
	}
	if !strings.Contains(out, "; per-view") {
		t.Errorf("mini card at 80x12 missing '; per-view' hint: %s", out)
	}
	// Full-sheet-only group headers must NOT appear.
	for _, absent := range []string{"WORKSPACE", "CHROME"} {
		if strings.Contains(out, absent) {
			t.Errorf("mini card at 80x12 leaked full-sheet header %q", absent)
		}
	}
}

// TestHelpOverlayMini_HiddenWhenTall verifies the full sheet renders (not the
// mini) when the terminal is tall enough for the full body (bt-dx7k.1 Task 2).
func TestHelpOverlayMini_HiddenWhenTall(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.width, m.height = 120, 40
	m.openModal(ModalHelp)
	m.focused = focusHelp
	out := m.renderHelpOverlay()

	// At 120x40 the full sheet fits; the mini nudge must NOT appear.
	if strings.Contains(out, "↓ expand") {
		t.Errorf("full-sheet at 120x40 should NOT contain '↓ expand' mini nudge")
	}
	// Full-sheet group headers must be present.
	for _, want := range []string{"SWITCH VIEWS", "CHROME"} {
		if !strings.Contains(out, want) {
			t.Errorf("full-sheet at 120x40 missing group header %q", want)
		}
	}
}

// TestHelpOverlay_FullSheetVerticallyCentered verifies the full sheet floats
// vertically centered (not pinned to the top) when there is spare height
// (bt-dx7k.1 dogfood). The tier selector guarantees the full sheet only renders
// when it fits, so centering cannot clip.
func TestHelpOverlay_FullSheetVerticallyCentered(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.width, m.height = 120, 44
	m.openModal(ModalHelp)
	m.focused = focusHelp
	out := m.renderHelpOverlay()

	// Sanity: this is the full sheet, not the mini.
	if strings.Contains(out, "↓ expand") {
		t.Fatalf("expected full sheet at 120x44, got mini")
	}
	// The box top border must NOT be on the first row -- there should be leading
	// blank rows above it (vertical centering). Top-pinned would put it at row 0.
	lines := strings.Split(out, "\n")
	firstBox := -1
	for i, l := range lines {
		if strings.Contains(l, "╭") {
			firstBox = i
			break
		}
	}
	if firstBox <= 0 {
		t.Fatalf("expected leading blank rows above a vertically-centered box, box top at line %d", firstBox)
	}
}

// TestHelpOverlay_FewerColumnsWhenTall verifies that with spare vertical room the
// full sheet uses fewer, wider columns so long descriptions are not truncated
// (bt-dx7k.1 dogfood). At 120x44 the longest desc renders in full.
func TestHelpOverlay_FewerColumnsWhenTall(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.width, m.height = 120, 44
	m.openModal(ModalHelp)
	m.focused = focusHelp
	out := m.renderHelpOverlay()

	// The full Back desc is a long one; with the old always-4-column layout at
	// 120 wide it was clipped to "back / quit (con...". Sourced from the Map so
	// the assertion tracks the binding, not a literal.
	want := m.keys.Global.Back.Help().Desc
	if !strings.Contains(out, want) {
		t.Errorf("full sheet at 120x44 should show the full desc %q (fewer/wider columns), but it was truncated", want)
	}
}

// TestHelpMini_ProjectsGate verifies ProjectsOrWisps appears in helpMiniRows
// only when m.workspaceMode is true (bt-dx7k.1 Task 3).
func TestHelpMini_ProjectsGate(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.width, m.height = 80, 30

	// Single-project: workspaceMode off — "projects / wisps" must be absent.
	m.workspaceMode = false
	rows := m.helpMiniRows()
	for _, r := range rows {
		if r.right == "projects / wisps" {
			t.Errorf("single-project: helpMiniRows() should NOT contain 'projects / wisps', but it does")
		}
	}

	// Multi-project: workspaceMode on — "projects / wisps" must be present.
	m.workspaceMode = true
	rows = m.helpMiniRows()
	found := false
	for _, r := range rows {
		if r.right == "projects / wisps" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("multi-project: helpMiniRows() should contain 'projects / wisps', but it does not")
	}
}

// TestHelpOverlay_SingleBoxAesthetic verifies the ? overlay renders in a single
// rounded modal box with inline-divider group headers and dim-mono theme tokens
// (bt-dx7k.1). The four global task group headers must appear, global binding
// descs must be present, and the box title "shortcuts" must appear exactly once
// (proving one box, not four).
func TestHelpOverlay_SingleBoxAesthetic(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.width, m.height = 120, 40
	m.openModal(ModalHelp)
	m.focused = focusHelp

	out := m.renderHelpOverlay()

	// (a) All four global task group headers present as inline dividers.
	for _, header := range []string{"SWITCH VIEWS", "DO THINGS", "WORKSPACE", "CHROME"} {
		if !strings.Contains(out, header) {
			t.Errorf("? overlay missing group header %q", header)
		}
	}

	// (b) Global binding descs present (sourced from the key.Map, not literals).
	for _, desc := range []string{"board", "refresh"} {
		if !strings.Contains(out, desc) {
			t.Errorf("? overlay missing global binding desc %q", desc)
		}
	}

	// (c) Exactly two occurrences of "shortcuts" proves a single box (one in the
	// title border, one from the Sidebar binding desc in the CHROME group body).
	// Old code had count==1 (no box title, only the Sidebar desc body row).
	// Four-box code would also have count==1 (Sidebar desc in one panel body, no "shortcuts" in panel titles).
	if n := strings.Count(out, "shortcuts"); n != 2 {
		t.Errorf("? overlay contains %d occurrences of %q, want exactly 2 (title border + Sidebar desc)", n, "shortcuts")
	}
}
