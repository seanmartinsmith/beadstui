package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRepoPickerSelectionAndToggle(t *testing.T) {
	repos := []string{"api", "web", "lib"}
	m := NewRepoPickerModel(repos, DefaultTheme(lipgloss.NewRenderer(nil)))
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

func TestRepoPickerViewContainsRepos(t *testing.T) {
	repos := []string{"api"}
	m := NewRepoPickerModel(repos, DefaultTheme(lipgloss.NewRenderer(nil)))
	m.SetSize(60, 20)

	out := m.View()
	if !strings.Contains(out, "Repo Filter") {
		t.Fatalf("expected title in view, got:\n%s", out)
	}
	if !strings.Contains(out, "api") {
		t.Fatalf("expected repo name in view, got:\n%s", out)
	}
}
