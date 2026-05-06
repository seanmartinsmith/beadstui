package ui

import (
	"strings"
	"testing"
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

	// Layout: row 0 top border, row 1 top breathing, row 2+ repos.
	if idx, ok := m.ItemAtPanelY(2); !ok || idx != 0 {
		t.Errorf("row 2: got (%d, %v), want (0, true)", idx, ok)
	}
	if idx, ok := m.ItemAtPanelY(3); !ok || idx != 1 {
		t.Errorf("row 3: got (%d, %v), want (1, true)", idx, ok)
	}
	if idx, ok := m.ItemAtPanelY(4); !ok || idx != 2 {
		t.Errorf("row 4: got (%d, %v), want (2, true)", idx, ok)
	}

	// Chrome rows above and below the repo block.
	if _, ok := m.ItemAtPanelY(0); ok {
		t.Error("row 0 (top border) should not map to a repo")
	}
	if _, ok := m.ItemAtPanelY(1); ok {
		t.Error("row 1 (top breathing) should not map to a repo")
	}
	if _, ok := m.ItemAtPanelY(5); ok {
		t.Error("row 5 (blank after repos) should not map to a repo")
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
