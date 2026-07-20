package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/seanmartinsmith/beadstui/internal/source"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// memoriesFixtureAggregate builds a two-source, multi-memory aggregate for
// assertion tests: no live bd/Dolt needed, per the bead acceptance ("must
// not require a live server to test").
func memoriesFixtureAggregate() source.MemoriesAggregate {
	btOrigin := source.Origin{SourceKind: source.SourceKindBDEmbedded, Scope: "bt", Prefix: "bt", DisplayName: "bt"}
	atlasOrigin := source.Origin{SourceKind: source.SourceKindBeadsGlobal, Scope: "beads_global", Prefix: "beads_global", DisplayName: "atlas"}

	return source.MemoriesAggregate{
		Memories: []source.Memory{
			{Key: "cross-prefix-deps", Body: "Two cross-project dependency patterns in beads: bare cross-prefix and external: + ship.", Origin: btOrigin},
			{Key: "e2e-suite-duration", Body: "Full test suite takes about 8.5 minutes as of 2026-04-08.", Origin: btOrigin},
			{Key: "atlas-secrets-topology", Body: "1Password is the source of truth for runtime-injected secrets across the fleet.", Origin: atlasOrigin},
		},
	}
}

// TestMemoriesViewToggle covers bt-2ea7t.4: the Memories key toggles into
// ViewMemories/focusMemories and back to ViewList/focusList on a second
// press, matching the bt-yzfp2 toggle-back convention every other view
// switch follows (TestViewSwitchKeysToggleBack).
func TestMemoriesViewToggle(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "One", Status: model.StatusOpen, Priority: 0},
	}
	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
	m = updated.(Model)

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	m = updated.(Model)
	if m.mode != ViewMemories {
		t.Fatalf("after u: expected ViewMemories, got %v", m.mode)
	}
	if m.focused != focusMemories {
		t.Fatalf("after u: expected focusMemories, got %v", m.focused)
	}
	if !m.memoriesLoading {
		t.Errorf("after u: expected memoriesLoading=true (async dispatch), got false")
	}
	if cmd == nil {
		t.Errorf("after u: expected non-nil tea.Cmd from async dispatch, got nil")
	}
	if m.FocusState() != "memories" {
		t.Errorf("FocusState() = %q, want %q", m.FocusState(), "memories")
	}

	// Loading frame renders without panicking.
	frame := m.View().Content
	if frame == "" {
		t.Errorf("loading frame is empty")
	}

	// Second press exits back to the list.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	m = updated.(Model)
	if m.mode != ViewList {
		t.Errorf("second u: expected ViewList, got %v", m.mode)
	}
	if m.focused != focusList {
		t.Errorf("second u: expected focusList, got %v", m.focused)
	}
}

// TestMemoriesQuitKeyDoesNotQuitApp guards the Back(q)/Cancel(esc) cascade
// wiring: without an explicit `if m.mode == ViewMemories` branch in both
// cascades, "q" falls through to `return m, tea.Quit` (quits the whole app)
// and "esc" falls through to the quit-confirm modal. Both would be a bug a
// user hits immediately on their first "how do I leave this view" keypress.
func TestMemoriesQuitKeyDoesNotQuitApp(t *testing.T) {
	issues := []model.Issue{{ID: "bv-1", Title: "One", Status: model.StatusOpen}}

	for _, key := range []string{"q", "esc"} {
		t.Run(key, func(t *testing.T) {
			m := NewModel(issues, nil, "", nil, nil)
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
			m = updated.(Model)

			updated, _ = m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
			m = updated.(Model)
			if m.mode != ViewMemories {
				t.Fatalf("setup: expected ViewMemories, got %v", m.mode)
			}

			var kp tea.KeyPressMsg
			if key == "q" {
				kp = tea.KeyPressMsg{Code: 'q', Text: "q"}
			} else {
				kp = tea.KeyPressMsg{Code: tea.KeyEscape}
			}
			updated, cmd := m.Update(kp)
			m = updated.(Model)

			if cmd != nil {
				if _, isQuit := cmd().(tea.QuitMsg); isQuit {
					t.Fatalf("%q from Memories view triggered tea.Quit instead of exiting to ViewList", key)
				}
			}
			if m.mode != ViewList {
				t.Errorf("%q: expected ViewList, got %v", key, m.mode)
			}
			if m.focused != focusList {
				t.Errorf("%q: expected focusList, got %v", key, m.focused)
			}
			if !m.isSplitView {
				t.Errorf("%q: expected isSplitView restored to true, got false", key)
			}
			if m.activeModal == ModalQuitConfirm {
				t.Errorf("%q: expected no quit-confirm modal, got one", key)
			}
		})
	}
}

// seedMemories installs a fixture aggregate into the model as if
// MemoriesLoadedMsg had just arrived, and puts the model into the Memories
// view. Assertion tests build on this rather than driving the real
// LoadMemoriesCmd (which shells `bd` / opens Dolt connections) - the bead
// acceptance requires the view to be testable "even with seeded fixture
// data in tests -- it must not require a live server".
func seedMemories(t *testing.T, agg source.MemoriesAggregate, width, height int) Model {
	t.Helper()
	issues := []model.Issue{{ID: "bv-1", Title: "One", Status: model.StatusOpen}}
	m := NewModel(issues, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = updated.(Model)

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	m = updated.(Model)

	updated, _ = m.Update(MemoriesLoadedMsg{Aggregate: agg})
	m = updated.(Model)

	if m.memoriesLoading {
		t.Fatalf("setup: expected memoriesLoading=false after MemoriesLoadedMsg")
	}
	return m
}

// TestMemoriesGroupingAndDetail covers the core master/detail render: group
// headers by origin display name, memory keys listed under their group, and
// the selected memory's body in the detail pane.
func TestMemoriesGroupingAndDetail(t *testing.T) {
	m := seedMemories(t, memoriesFixtureAggregate(), 160, 40)

	content := ansi.Strip(m.View().Content)

	for _, want := range []string{"bt", "atlas", "cross-prefix-deps", "e2e-suite-duration", "atlas-secrets-topology"} {
		if !strings.Contains(content, want) {
			t.Errorf("split view missing %q; content:\n%s", want, content)
		}
	}
	// The first memory (sorted: "atlas-secrets-topology" < "cross-prefix-deps"
	// within its own group, but groups are sorted by DisplayName: "atlas" <
	// "bt") should be selected by default, so its body renders in the detail
	// pane.
	if !strings.Contains(content, "1Password is the source of truth") {
		t.Errorf("detail pane missing selected memory's body; content:\n%s", content)
	}
}

// TestMemoriesGroupLabelFallback covers bt-2ea7t.6: a source whose Origin
// has an empty DisplayName must still render a real, non-blank group
// header rather than the invisible " (N)" a bare DisplayName grouping key
// would produce. rebuildRows groups via Origin.Label() (DisplayName, else
// Scope, else "unknown source"), so this exercises both fallback tiers.
func TestMemoriesGroupLabelFallback(t *testing.T) {
	scopeOnlyOrigin := source.Origin{SourceKind: source.SourceKindBDShared, Scope: "portal", Prefix: "portal", DisplayName: ""}
	bareOrigin := source.Origin{SourceKind: source.SourceKindBDShared}

	agg := source.MemoriesAggregate{
		Memories: []source.Memory{
			{Key: "scope-fallback-memory", Body: "Falls back to Scope when DisplayName is empty.", Origin: scopeOnlyOrigin},
			{Key: "placeholder-fallback-memory", Body: "Falls back to the placeholder when both are empty.", Origin: bareOrigin},
		},
	}
	m := seedMemories(t, agg, 160, 40)

	content := ansi.Strip(m.View().Content)

	if !strings.Contains(content, "portal (1)") {
		t.Errorf("expected group header to fall back to Origin.Scope %q; content:\n%s", "portal (1)", content)
	}
	if !strings.Contains(content, "unknown source (1)") {
		t.Errorf("expected group header to fall back to the placeholder %q; content:\n%s", "unknown source (1)", content)
	}
	// Each fallback tier produces its own distinct, non-blank label (rather
	// than both collapsing into one shared "" group), so both memory keys
	// must be individually reachable under their own header.
	for _, want := range []string{"scope-fallback-memory", "placeholder-fallback-memory"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected memory key %q under its fallback-labeled group; content:\n%s", want, content)
		}
	}
}

// TestMemoriesSearch covers search filtering across keys and bodies: typing
// a query that matches only one memory's body text narrows the master pane
// to that memory (and its group), and the other memories/groups disappear.
func TestMemoriesSearch(t *testing.T) {
	m := seedMemories(t, memoriesFixtureAggregate(), 160, 40)

	updated, _ := m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m = updated.(Model)
	if !m.memories.IsSearchActive() {
		t.Fatalf("expected search active after /")
	}

	for _, r := range "8.5 minutes" {
		updated, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(Model)
	}

	content := ansi.Strip(m.View().Content)
	if !strings.Contains(content, "e2e-suite-duration") {
		t.Errorf("search for body text should surface e2e-suite-duration; content:\n%s", content)
	}
	if strings.Contains(content, "cross-prefix-deps") {
		t.Errorf("search should have filtered out cross-prefix-deps; content:\n%s", content)
	}
	if strings.Contains(content, "atlas-secrets-topology") {
		t.Errorf("search should have filtered out atlas-secrets-topology; content:\n%s", content)
	}

	// Cancel restores the full list.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.mode != ViewMemories {
		t.Fatalf("esc during search should cancel search, not exit the view; got mode=%v", m.mode)
	}
	content = ansi.Strip(m.View().Content)
	if !strings.Contains(content, "cross-prefix-deps") {
		t.Errorf("cancelling search should restore the full list; content:\n%s", content)
	}
}

// TestMemoriesEmptyState covers design spec S8: the empty state renders only
// when every source came back with zero memories.
func TestMemoriesEmptyState(t *testing.T) {
	m := seedMemories(t, source.MemoriesAggregate{}, 160, 40)

	content := ansi.Strip(m.View().Content)
	if !strings.Contains(content, "No memories found") {
		t.Errorf("expected empty state; content:\n%s", content)
	}
}

// TestMemoriesGCHiddenNote covers design spec S4.3/S8: excluded (Gas City)
// sources render a visible "N Gas City sources hidden" note rather than
// disappearing silently.
func TestMemoriesGCHiddenNote(t *testing.T) {
	agg := memoriesFixtureAggregate()
	agg.Excluded = []source.Origin{
		{SourceKind: source.SourceKindGasCity, Scope: "rig-a", DisplayName: "some-city"},
		{SourceKind: source.SourceKindGasCity, Scope: "rig-b", DisplayName: "some-city"},
	}
	m := seedMemories(t, agg, 160, 40)

	content := ansi.Strip(m.View().Content)
	if !strings.Contains(content, "2 Gas City sources hidden (own lens, coming later)") {
		t.Errorf("expected gc-hidden note; content:\n%s", content)
	}
}

// TestMemoriesGCHiddenNoteOnEmptyState covers the empty-state variant: when
// every bd source is ALSO empty, the gc-hidden note must still be visible on
// the empty-state screen (spec S8 "exclusion is visible, not silent" applies
// regardless of whether any memories were found).
func TestMemoriesGCHiddenNoteOnEmptyState(t *testing.T) {
	agg := source.MemoriesAggregate{
		Excluded: []source.Origin{
			{SourceKind: source.SourceKindGasCity, Scope: "rig-a", DisplayName: "some-city"},
		},
	}
	m := seedMemories(t, agg, 160, 40)

	content := ansi.Strip(m.View().Content)
	if !strings.Contains(content, "No memories found") {
		t.Errorf("expected empty state; content:\n%s", content)
	}
	if !strings.Contains(content, "1 Gas City source hidden (own lens, coming later)") {
		t.Errorf("expected gc-hidden note on empty state; content:\n%s", content)
	}
}

// TestMemoriesUnavailableNote covers design spec S8: an unreachable source
// (bd binary missing, server down) degrades to a visible note, not a
// failure screen.
func TestMemoriesUnavailableNote(t *testing.T) {
	agg := memoriesFixtureAggregate()
	agg.Unavailable = []source.UnavailableSource{
		{Origin: source.Origin{SourceKind: source.SourceKindBDServer, Scope: "dotfiles", DisplayName: "dotfiles"}, Err: errUnavailableFixture},
	}
	m := seedMemories(t, agg, 160, 40)

	content := ansi.Strip(m.View().Content)
	if !strings.Contains(content, "unavailable") {
		t.Errorf("expected unavailable-source note; content:\n%s", content)
	}
	if !strings.Contains(content, "dotfiles") {
		t.Errorf("expected unavailable note to name the source; content:\n%s", content)
	}
	// Not a failure screen: the loaded memories still render.
	if !strings.Contains(content, "cross-prefix-deps") {
		t.Errorf("unavailable source must not blank out the loaded memories; content:\n%s", content)
	}
}

var errUnavailableFixture = &fixtureErr{"shared Dolt server not running"}

type fixtureErr struct{ msg string }

func (e *fixtureErr) Error() string { return e.msg }

// TestMemoriesSmallTerminal covers the bead's small-terminal acceptance bar
// (14-30 row scrunched windows): the view must collapse to a single pane
// below SplitViewThreshold and render without panicking, still surfacing the
// master list content.
func TestMemoriesSmallTerminal(t *testing.T) {
	sizes := []struct{ w, h int }{
		{70, 20},
		{100, 16},
		{50, 14},
	}
	for _, sz := range sizes {
		t.Run("", func(t *testing.T) {
			m := seedMemories(t, memoriesFixtureAggregate(), sz.w, sz.h)

			view := m.View()
			content := ansi.Strip(view.Content)
			if content == "" {
				t.Fatalf("%dx%d: empty render", sz.w, sz.h)
			}
			rows := strings.Split(content, "\n")
			if len(rows) < sz.h-1 {
				t.Errorf("%dx%d: render produced %d rows, expected at least %d", sz.w, sz.h, len(rows), sz.h-1)
			}
			// Default cursor lands on the "atlas" group's memory first
			// (alphabetically first group); in single-pane mode the master
			// list is shown by default, so its key should be visible.
			if !strings.Contains(content, "atlas-secrets-topology") {
				t.Errorf("%dx%d: master pane missing memory key; content:\n%s", sz.w, sz.h, content)
			}
		})
	}
}

// TestMemoriesSingleWidthDetailToggle covers single-pane mode's tab/enter
// pane switch: Tab flips from master to detail and back.
func TestMemoriesSingleWidthDetailToggle(t *testing.T) {
	m := seedMemories(t, memoriesFixtureAggregate(), 70, 20)
	if m.memories.isSplitWidth() {
		t.Fatalf("setup: expected single-pane at width 70")
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)
	content := ansi.Strip(m.View().Content)
	if !strings.Contains(content, "1Password is the source of truth") {
		t.Errorf("tab should switch to detail pane; content:\n%s", content)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)
	content = ansi.Strip(m.View().Content)
	if !strings.Contains(content, "atlas-secrets-topology") {
		t.Errorf("second tab should switch back to master pane; content:\n%s", content)
	}
}

// TestMemoriesLoadCmd_DogfoodLiveProject is the dogfood check (bt-2ea7t.4):
// runs the real LoadMemoriesCmd discovery orchestrator (DetectPath ->
// SelectAdapter -> AggregateMemories, exactly as enterMemoriesView dispatches
// it) against THIS checkout's own .beads/ project and asserts bt's own real
// memories come back. Self-skips in any environment where that
// precondition doesn't hold (no `bd` binary, or not running inside a repo
// with .beads/ two directories up from the pkg/ui test binary's cwd) so it
// never flakes CI or a machine without bd installed - it only proves the
// wiring when it safely can.
func TestMemoriesLoadCmd_DogfoodLiveProject(t *testing.T) {
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd binary not found in PATH; skipping live dogfood check")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	// `go test ./pkg/ui` runs with cwd = pkg/ui; the project root (with
	// .beads/) is two levels up.
	repoRoot := filepath.Join(cwd, "..", "..")
	if _, err := os.Stat(filepath.Join(repoRoot, ".beads")); err != nil {
		t.Skip("no .beads/ found at the repo root; not running inside a live bt checkout")
	}

	msg := LoadMemoriesCmd(repoRoot)()
	loaded, ok := msg.(MemoriesLoadedMsg)
	if !ok {
		t.Fatalf("expected MemoriesLoadedMsg, got %T", msg)
	}

	if len(loaded.Aggregate.Memories) == 0 {
		t.Fatalf("expected bt's own memories to load from repoRoot=%s (dogfood requirement); got zero. unavailable=%v excluded=%v",
			repoRoot, loaded.Aggregate.Unavailable, loaded.Aggregate.Excluded)
	}

	foundOwnOrigin := false
	for _, mem := range loaded.Aggregate.Memories {
		if mem.Origin.SourceKind.IsBD() {
			foundOwnOrigin = true
			break
		}
	}
	if !foundOwnOrigin {
		t.Errorf("expected at least one memory tagged with a bd-managed Origin")
	}

	t.Logf("dogfood: loaded %d memories across sources (unavailable=%d, excluded=%d)",
		len(loaded.Aggregate.Memories), len(loaded.Aggregate.Unavailable), len(loaded.Aggregate.Excluded))
}
