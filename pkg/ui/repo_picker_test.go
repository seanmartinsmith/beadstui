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
