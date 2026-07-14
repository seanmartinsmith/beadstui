package ui

import (
	"strings"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/ui/keys"

	tea "charm.land/bubbletea/v2"
)

func TestRepoPickerSelectionAndToggle(t *testing.T) {
	repos := []string{"api", "web", "lib"}
	m := NewRepoPickerModel(repos, DefaultTheme())
	m.SetSize(80, 24)

	// Default is all selected
	if got := len(m.SelectedRepos()); got != 3 {
		t.Fatalf("expected 3 selected repos by default, got %d", got)
	}

	// Toggle first repo off
	m.ToggleSelected()
	if got := len(m.SelectedRepos()); got != 2 {
		t.Fatalf("expected 2 selected after toggle, got %d", got)
	}

	// Select all
	m.SelectAll()
	if got := len(m.SelectedRepos()); got != 3 {
		t.Fatalf("expected 3 selected after SelectAll, got %d", got)
	}
}

func TestRepoPickerToggleAll(t *testing.T) {
	repos := []string{"api", "web", "lib"}
	m := NewRepoPickerModel(repos, DefaultTheme())
	m.SetSize(80, 24)

	// All selected -> ToggleAll deselects all
	m.ToggleAll()
	if got := len(m.SelectedRepos()); got != 0 {
		t.Fatalf("expected 0 selected after ToggleAll (was all), got %d", got)
	}

	// None selected -> ToggleAll selects all
	m.ToggleAll()
	if got := len(m.SelectedRepos()); got != 3 {
		t.Fatalf("expected 3 selected after ToggleAll (was none), got %d", got)
	}

	// Some selected -> ToggleAll deselects all
	m.ToggleSelected() // deselect first
	if !m.AnySelected() {
		t.Fatal("expected some selected after toggling one off")
	}
	m.ToggleAll()
	if got := len(m.SelectedRepos()); got != 0 {
		t.Fatalf("expected 0 selected after ToggleAll (was some), got %d", got)
	}
}

func TestRepoPickerViewContainsRepos(t *testing.T) {
	repos := []string{"api"}
	m := NewRepoPickerModel(repos, DefaultTheme())
	m.SetSize(60, 20)

	out := m.View()
	if !strings.Contains(out, "Project Filter") {
		t.Fatalf("expected title in view, got:\n%s", out)
	}
	if !strings.Contains(out, "api") {
		t.Fatalf("expected repo name in view, got:\n%s", out)
	}
}

// TestRepoPickerItemAtPanelY guards bt-hpsq mouse routing: panel-relative Y
// coordinates map to repo indices, with chrome rows returning ok=false.
func TestRepoPickerItemAtPanelY(t *testing.T) {
	repos := []string{"api", "web", "lib"}
	m := NewRepoPickerModel(repos, DefaultTheme())
	m.SetSize(60, 20)

	// Layout: row 0 top border, row 1 search input, row 2 blank, row 3+ repos.
	if idx, ok := m.ItemAtPanelY(3); !ok || idx != 0 {
		t.Errorf("row 3: got (%d, %v), want (0, true)", idx, ok)
	}
	if idx, ok := m.ItemAtPanelY(4); !ok || idx != 1 {
		t.Errorf("row 4: got (%d, %v), want (1, true)", idx, ok)
	}
	if idx, ok := m.ItemAtPanelY(5); !ok || idx != 2 {
		t.Errorf("row 5: got (%d, %v), want (2, true)", idx, ok)
	}

	// Chrome rows above and below the repo block.
	if _, ok := m.ItemAtPanelY(0); ok {
		t.Error("row 0 (top border) should not map to a repo")
	}
	if _, ok := m.ItemAtPanelY(1); ok {
		t.Error("row 1 (search input) should not map to a repo")
	}
	if _, ok := m.ItemAtPanelY(2); ok {
		t.Error("row 2 (blank) should not map to a repo")
	}
	if _, ok := m.ItemAtPanelY(6); ok {
		t.Error("row 6 (blank after repos) should not map to a repo")
	}

	// Empty repo list is always a no-op.
	empty := NewRepoPickerModel([]string{}, DefaultTheme())
	empty.SetSize(60, 20)
	if _, ok := empty.ItemAtPanelY(2); ok {
		t.Error("empty picker should never report ok=true")
	}
}

// TestRepoPickerSetCursorClamps mirrors the labels picker SetCursor guard.
func TestRepoPickerSetCursorClamps(t *testing.T) {
	m := NewRepoPickerModel([]string{"a", "b", "c"}, DefaultTheme())

	m.SetCursor(1)
	if m.selectedIndex != 1 {
		t.Errorf("SetCursor(1): got %d, want 1", m.selectedIndex)
	}
	m.SetCursor(99)
	if m.selectedIndex != 2 {
		t.Errorf("SetCursor(99): got %d, want 2 (clamped)", m.selectedIndex)
	}
	m.SetCursor(-3)
	if m.selectedIndex != 0 {
		t.Errorf("SetCursor(-3): got %d, want 0 (clamped)", m.selectedIndex)
	}
}

// TestRepoPickerDimensions sanity-checks Dimensions() against the layout
// constants used by the click handler.
func TestRepoPickerDimensions(t *testing.T) {
	repos := []string{"api", "web", "lib"}
	m := NewRepoPickerModel(repos, DefaultTheme())
	m.SetSize(80, 20)

	w, h := m.Dimensions()
	if w < 30 {
		t.Errorf("Dimensions width: got %d, want >= 30 (floor)", w)
	}
	expectedH := len(repos) + repoPickerVerticalChrome
	if h != expectedH {
		t.Errorf("Dimensions height: got %d, want %d", h, expectedH)
	}

	// Empty picker still yields a valid box height.
	empty := NewRepoPickerModel([]string{}, DefaultTheme())
	empty.SetSize(80, 20)
	_, eh := empty.Dimensions()
	if eh != 1+repoPickerVerticalChrome {
		t.Errorf("empty picker height: got %d, want %d", eh, 1+repoPickerVerticalChrome)
	}
}

// TestRepoPickerModalAlwaysFitsInBg covers bt-vr2h: with a long repo list,
// the modal must not grow past the bg passed to SetSize. Before the
// visibleCount cap, Dimensions() returned len(repos)+chrome unconditionally
// and overflowed scrunched terminals.
func TestRepoPickerModalAlwaysFitsInBg(t *testing.T) {
	// Eighteen repos -- mirrors the dogfood-2026-05-06 image showing the
	// project filter overflowing on a small window.
	repos := []string{
		"beads", "bt", "cctui", "cnvs", "dev_browser", "dotfiles",
		"lil_sto", "marketplace", "portal", "portfolio", "remotion",
		"sms", "sym", "tpane", "updoots", "alpha", "beta", "gamma",
	}
	for bg := 9; bg <= 60; bg++ {
		m := NewRepoPickerModel(repos, DefaultTheme())
		m.SetSize(120, bg)
		_, h := m.Dimensions()
		if h > bg {
			t.Errorf("bg=%d (%d repos): modal h=%d exceeds bg; will clip on overlay center", bg, len(repos), h)
		}
	}
}

// TestRepoPickerVisibleCountScalesWithHeight asserts the bt-vr2h percentage
// cap on the project filter modal -- mirrors TestLabelPickerVisibleCountScalesWithHeight.
func TestRepoPickerVisibleCountScalesWithHeight(t *testing.T) {
	repos := make([]string, 50) // far more than any cap so the visibleCount math drives
	for i := range repos {
		repos[i] = "repo"
	}

	tests := []struct {
		height   int
		expected int
		note     string
	}{
		{8, 1, "extremely tiny: fallback to bg-chrome, floored to 1"},
		{12, 1, "tiny: 75%*12=9, minus 8 chrome = 1"},
		{20, 7, "small: 75%*20=15, minus 8 chrome = 7"},
		{30, 14, "medium: 75%*30=22, minus 8 chrome = 14"},
		{50, 29, "tall: 75%*50=37, minus 8 chrome = 29"},
		{80, 30, "huge: 75%*80=60, minus 8 chrome = 52, clamp at repoPickerMaxVisible (30)"},
	}
	for _, tc := range tests {
		m := NewRepoPickerModel(repos, DefaultTheme())
		m.SetSize(120, tc.height)
		got := m.visibleCount()
		if got != tc.expected {
			t.Errorf("height=%d (%s): visibleCount=%d, want %d", tc.height, tc.note, got, tc.expected)
		}
	}
}

// TestRepoPickerAtlasPinnedFirst covers bt-z1pzj: the beads_global namespace
// (displayed "atlas") is pinned to the top of the picker regardless of its
// position in the enumerated list, under either raw spelling.
func TestRepoPickerAtlasPinnedFirst(t *testing.T) {
	cases := []struct {
		name    string
		repos   []string
		wantTop string // raw key expected first
	}{
		{"beads_global mid-list", []string{"bt", "sym", "beads_global", "world"}, "beads_global"},
		{"global mid-list", []string{"bt", "sym", "global", "world"}, "global"},
		{"already first", []string{"beads_global", "bt", "sym"}, "beads_global"},
		{"no atlas keeps order", []string{"bt", "sym", "world"}, "bt"},
	}
	for _, tc := range cases {
		m := NewRepoPickerModel(tc.repos, DefaultTheme())
		if got := m.filtered[0]; got != tc.wantTop {
			t.Errorf("%s: filtered[0]=%q, want %q", tc.name, got, tc.wantTop)
		}
		// The pinned row still displays as "atlas" via DisplayRepoName.
		if tc.wantTop == "beads_global" || tc.wantTop == "global" {
			m.SetSize(80, 24)
			if !strings.Contains(m.View(), "atlas") {
				t.Errorf("%s: view should display the atlas alias", tc.name)
			}
		}
	}

	// Relative order of the non-atlas repos is preserved.
	m := NewRepoPickerModel([]string{"bt", "sym", "beads_global", "world"}, DefaultTheme())
	want := []string{"beads_global", "bt", "sym", "world"}
	for i, w := range want {
		if m.filtered[i] != w {
			t.Fatalf("filtered=%v, want %v", m.filtered, want)
		}
	}
}

// TestRepoPickerSearchFilter covers the bt-9lpib core: typing narrows the
// visible list by case-insensitive substring, atlas stays pinned among
// matches, and a query for the display alias finds beads_global.
func TestRepoPickerSearchFilter(t *testing.T) {
	repos := []string{"bt", "beads", "sym", "beads_global", "world"}
	m := NewRepoPickerModel(repos, DefaultTheme())

	// "be" matches beads (and beads_global via raw name is not matched -- it
	// matches on display "atlas"; but "beads" substring still hits it? no:
	// display of beads_global is "atlas"). So "be" -> beads only among the b* .
	m.input.SetValue("be")
	m.filterRepos()
	if got := m.filtered; !equalStringSlices(got, []string{"beads"}) {
		t.Errorf(`query "be": filtered=%v, want [beads]`, got)
	}

	// "b" matches bt and beads (beads_global displays as atlas, so no b-match).
	m.input.SetValue("b")
	m.filterRepos()
	if got := m.filtered; !equalStringSlices(got, []string{"bt", "beads"}) {
		t.Errorf(`query "b": filtered=%v, want [bt beads]`, got)
	}

	// The display alias is searchable: "atl" finds beads_global, pinned first.
	m.input.SetValue("atl")
	m.filterRepos()
	if got := m.filtered; !equalStringSlices(got, []string{"beads_global"}) {
		t.Errorf(`query "atl": filtered=%v, want [beads_global]`, got)
	}

	// Case-insensitive.
	m.input.SetValue("WORLD")
	m.filterRepos()
	if got := m.filtered; !equalStringSlices(got, []string{"world"}) {
		t.Errorf(`query "WORLD": filtered=%v, want [world]`, got)
	}

	// No match -> empty filtered, compact modal.
	m.input.SetValue("zzz")
	m.filterRepos()
	if len(m.filtered) != 0 {
		t.Errorf(`query "zzz": filtered=%v, want empty`, m.filtered)
	}

	// Clearing restores the full list, atlas pinned.
	m.input.SetValue("")
	m.filterRepos()
	if len(m.filtered) != len(repos) || m.filtered[0] != "beads_global" {
		t.Errorf("cleared query: filtered=%v, want all %d with beads_global first", m.filtered, len(repos))
	}
}

// TestRepoPickerSearchFocusRouting covers the two-mode key wiring (bt-9lpib):
// "/" focuses search, typed letters route to the input and narrow the list,
// and Esc blurs without closing the modal.
func TestRepoPickerSearchFocusRouting(t *testing.T) {
	m := Model{theme: DefaultTheme(), keys: keys.NewAppKeys(), focused: focusRepoPicker}
	m.openModal(ModalRepoPicker)
	m.availableRepos = []string{"beads", "bt", "sym", "world"}
	m.repoPicker = NewRepoPickerModel(m.availableRepos, m.theme)
	m.repoPicker.SetSize(80, 24)

	if m.repoPicker.IsSearchFocused() {
		t.Fatal("picker should open in nav mode (search blurred)")
	}

	// "/" enters search mode.
	m = m.handleRepoPickerKeys(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.repoPicker.IsSearchFocused() {
		t.Fatal(`"/" should focus the search input`)
	}

	// Typed letters route to the input and narrow the list.
	m = m.handleRepoPickerKeys(tea.KeyPressMsg{Code: 'b', Text: "b"})
	if v := m.repoPicker.InputValue(); v != "b" {
		t.Fatalf("input value=%q, want b", v)
	}
	if got := m.repoPicker.filtered; !equalStringSlices(got, []string{"beads", "bt"}) {
		t.Errorf("after typing b: filtered=%v, want [beads bt]", got)
	}

	// Esc blurs search but keeps the modal open (query preserved).
	m = m.handleRepoPickerKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.repoPicker.IsSearchFocused() {
		t.Error("Esc in search mode should blur, not stay focused")
	}
	if m.activeModal != ModalRepoPicker {
		t.Error("Esc in search mode should not close the modal")
	}
	if m.repoPicker.InputValue() != "b" {
		t.Error("query buffer should be preserved after blur")
	}

	// Esc again (nav mode) closes the modal.
	m = m.handleRepoPickerKeys(tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.activeModal != ModalNone {
		t.Error("Esc in nav mode should close the modal")
	}
}

// TestRepoPickerPaging covers bt-6ltx9: with more projects than fit, PageDown
// and PageUp jump a full page and the View shows a page indicator.
func TestRepoPickerPaging(t *testing.T) {
	repos := make([]string, 40)
	for i := range repos {
		repos[i] = "repo" + string(rune('a'+i%26)) + string(rune('0'+i/26))
	}
	m := NewRepoPickerModel(repos, DefaultTheme())
	m.SetSize(120, 20) // visibleCount = 75%*20-8 = 7

	pageSize := m.visibleCount()
	if pageSize >= len(repos) {
		t.Fatalf("test setup: pageSize %d should be < %d for paging", pageSize, len(repos))
	}

	if m.selectedIndex != 0 {
		t.Fatalf("initial selectedIndex=%d, want 0", m.selectedIndex)
	}
	// PageDown jumps to the bottom of the next page; PageUp to the top of the
	// previous page (matches the label picker paging semantics).
	m.PageDown() // 0 -> bottom of page 1
	if m.selectedIndex != 2*pageSize-1 {
		t.Errorf("PageDown from page 0: selectedIndex=%d, want %d", m.selectedIndex, 2*pageSize-1)
	}
	m.PageDown() // -> bottom of page 2
	if m.selectedIndex != 3*pageSize-1 {
		t.Errorf("PageDown from page 1: selectedIndex=%d, want %d", m.selectedIndex, 3*pageSize-1)
	}
	m.PageUp() // -> top of page 1
	if m.selectedIndex != pageSize {
		t.Errorf("PageUp: selectedIndex=%d, want %d (top of page 1)", m.selectedIndex, pageSize)
	}
	m.PageUp() // -> top of page 0
	if m.selectedIndex != 0 {
		t.Errorf("PageUp to top: selectedIndex=%d, want 0", m.selectedIndex)
	}

	// The View shows a "page/total" indicator when the list overflows.
	totalPages := (len(repos) + pageSize - 1) / pageSize
	want := "1/" + itoa(totalPages)
	if !strings.Contains(m.View(), want) {
		t.Errorf("view should contain page indicator %q", want)
	}
}

// TestRepoPickerSelectionSurvivesFilter guards that a checked project stays in
// the applied filter even when a search query hides it (bt-9lpib).
func TestRepoPickerSelectionSurvivesFilter(t *testing.T) {
	repos := []string{"beads", "bt", "sym", "world"}
	m := NewRepoPickerModel(repos, DefaultTheme())
	m.SetActiveRepos(nil) // start with nothing checked

	// Check "world" via the cursor.
	m.input.SetValue("world")
	m.filterRepos()
	m.SetCursor(0)
	m.ToggleSelected()
	if !m.SelectedRepos()["world"] {
		t.Fatal("world should be selected")
	}

	// Now filter to a different project; the world selection must persist.
	m.input.SetValue("bt")
	m.filterRepos()
	if !m.SelectedRepos()["world"] {
		t.Error("world selection should survive a filter that hides it")
	}
}

// TestRepoPickerViewShowsChrome verifies the search prompt and project count
// appear in the rendered modal.
func TestRepoPickerViewShowsChrome(t *testing.T) {
	m := NewRepoPickerModel([]string{"bt", "sym"}, DefaultTheme())
	m.SetSize(80, 24)
	out := m.View()
	if !strings.Contains(out, ">") {
		t.Error("view should render the search prompt")
	}
	if !strings.Contains(out, "2 projects") {
		t.Errorf("view should render project count, got:\n%s", out)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
