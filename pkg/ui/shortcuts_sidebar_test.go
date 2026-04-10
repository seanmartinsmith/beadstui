package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewShortcutsSidebar(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	sidebar := NewShortcutsSidebar(theme)

	if sidebar.width != 34 {
		t.Errorf("Expected width 34, got %d", sidebar.width)
	}
	if sidebar.context != "list" {
		t.Errorf("Expected context 'list', got %q", sidebar.context)
	}
}

func TestShortcutsSidebarSetContext(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	sidebar := NewShortcutsSidebar(theme)

	sidebar.SetContext("graph")
	if sidebar.context != "graph" {
		t.Errorf("Expected context 'graph', got %q", sidebar.context)
	}

	sidebar.SetContext("insights")
	if sidebar.context != "insights" {
		t.Errorf("Expected context 'insights', got %q", sidebar.context)
	}
}

func TestShortcutsSidebarScrolling(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	sidebar := NewShortcutsSidebar(theme)

	// Initial scroll offset should be 0
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected initial scroll 0, got %d", sidebar.scrollOffset)
	}

	// Scroll down
	sidebar.ScrollDown()
	if sidebar.scrollOffset != 1 {
		t.Errorf("Expected scroll 1 after ScrollDown, got %d", sidebar.scrollOffset)
	}

	// Scroll up
	sidebar.ScrollUp()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 after ScrollUp, got %d", sidebar.scrollOffset)
	}

	// Scroll up at top should stay at 0
	sidebar.ScrollUp()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 at top, got %d", sidebar.scrollOffset)
	}

	// Page down
	sidebar.ScrollPageDown()
	if sidebar.scrollOffset != 10 {
		t.Errorf("Expected scroll 10 after PageDown, got %d", sidebar.scrollOffset)
	}

	// Page up
	sidebar.ScrollPageUp()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 after PageUp, got %d", sidebar.scrollOffset)
	}

	// Reset
	sidebar.scrollOffset = 5
	sidebar.ResetScroll()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 after Reset, got %d", sidebar.scrollOffset)
	}
}

func TestShortcutsSidebarView(t *testing.T) {
	theme := Theme{
		Renderer:  lipgloss.DefaultRenderer(),
		Primary:   lipgloss.AdaptiveColor{Light: "#00ff00", Dark: "#00ff00"},
		Secondary: lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Base:      lipgloss.NewStyle(),
	}
	sidebar := NewShortcutsSidebar(theme)
	sidebar.SetSize(28, 30)

	view := sidebar.View()
	if view == "" {
		t.Error("Expected non-empty view")
	}

	// Should contain title
	if !strings.Contains(view, "Shortcuts") {
		t.Error("Expected view to contain 'Shortcuts'")
	}

	// Should contain Navigation section
	if !strings.Contains(view, "Navigation") {
		t.Error("Expected view to contain 'Navigation'")
	}
}

func TestShortcutsSidebarContextFiltering(t *testing.T) {
	theme := Theme{
		Renderer:  lipgloss.DefaultRenderer(),
		Primary:   lipgloss.AdaptiveColor{Light: "#00ff00", Dark: "#00ff00"},
		Secondary: lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Base:      lipgloss.NewStyle(),
	}

	// Test graph context
	sidebar := NewShortcutsSidebar(theme)
	sidebar.SetSize(28, 50)
	sidebar.SetContext("graph")
	view := sidebar.View()

	if !strings.Contains(view, "Graph") {
		t.Error("Expected graph context to show Graph section")
	}

	// Test insights context
	sidebar.SetContext("insights")
	view = sidebar.View()

	if !strings.Contains(view, "Insights") {
		t.Error("Expected insights context to show Insights section")
	}
}

func TestContextFromFocus(t *testing.T) {
	tests := []struct {
		focus    focus
		expected string
	}{
		{focusList, "list"},
		{focusDetail, "detail"},
		{focusBoard, "board"},
		{focusGraph, "graph"},
		{focusInsights, "insights"},
		{focusHistory, "history"},
		{focusActionable, "actionable"},
		{focusLabelDashboard, "label"},
		{focusHelp, "list"}, // Default fallback
	}

	for _, tt := range tests {
		got := ContextFromFocus(tt.focus)
		if got != tt.expected {
			t.Errorf("ContextFromFocus(%d) = %q, want %q", tt.focus, got, tt.expected)
		}
	}
}

func TestShortcutsSidebarWidth(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	sidebar := NewShortcutsSidebar(theme)

	if sidebar.Width() != 34 {
		t.Errorf("Expected Width() = 34, got %d", sidebar.Width())
	}
}
