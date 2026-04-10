package ui

import (
	"os"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// createTheme creates a theme for testing
func createTheme() Theme {
	return DefaultTheme(lipgloss.NewRenderer(os.Stdout))
}

func TestLabelDashboardModel_ScrollAndHomeEnd(t *testing.T) {
	m := NewLabelDashboardModel(Theme{})
	// height=3 -> visibleRows=2 (header + 2 rows)
	m.SetSize(80, 3)
	m.SetData([]analysis.LabelHealth{
		{Label: "a", HealthLevel: analysis.HealthLevelHealthy, Blocked: 0, Health: 90},
		{Label: "b", HealthLevel: analysis.HealthLevelHealthy, Blocked: 0, Health: 80},
		{Label: "c", HealthLevel: analysis.HealthLevelHealthy, Blocked: 0, Health: 70},
		{Label: "d", HealthLevel: analysis.HealthLevelHealthy, Blocked: 0, Health: 60},
		{Label: "e", HealthLevel: analysis.HealthLevelHealthy, Blocked: 0, Health: 50},
	})

	// Move cursor down within visible range; no scroll yet.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 1 || m.scrollOffset != 0 {
		t.Fatalf("after j: cursor=%d scroll=%d; want cursor=1 scroll=0", m.cursor, m.scrollOffset)
	}

	// Move down past bottom; should scroll.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 2 || m.scrollOffset != 1 {
		t.Fatalf("after j,j: cursor=%d scroll=%d; want cursor=2 scroll=1", m.cursor, m.scrollOffset)
	}

	// Move back up past top; should scroll up.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.cursor != 1 || m.scrollOffset != 1 {
		t.Fatalf("after k: cursor=%d scroll=%d; want cursor=1 scroll=1", m.cursor, m.scrollOffset)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.cursor != 0 || m.scrollOffset != 0 {
		t.Fatalf("after k,k: cursor=%d scroll=%d; want cursor=0 scroll=0", m.cursor, m.scrollOffset)
	}

	// End should jump to last item and scroll to bottom.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if m.cursor != 4 {
		t.Fatalf("after G: cursor=%d; want 4", m.cursor)
	}
	if m.scrollOffset != 3 {
		t.Fatalf("after G: scroll=%d; want 3", m.scrollOffset)
	}

	// Home should reset.
	m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.cursor != 0 || m.scrollOffset != 0 {
		t.Fatalf("after home: cursor=%d scroll=%d; want cursor=0 scroll=0", m.cursor, m.scrollOffset)
	}
}

func TestLabelDashboardModel_EnterReturnsSelectedLabel(t *testing.T) {
	m := NewLabelDashboardModel(Theme{})
	m.SetSize(80, 3)
	m.SetData([]analysis.LabelHealth{
		{Label: "backend", HealthLevel: analysis.HealthLevelWarning, Blocked: 1, Health: 60},
		{Label: "frontend", HealthLevel: analysis.HealthLevelHealthy, Blocked: 0, Health: 90},
	})

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label != "frontend" {
		t.Fatalf("enter label=%q; want %q", label, "frontend")
	}
}

// =============================================================================
// View Rendering Tests
// =============================================================================

func TestLabelDashboardModel_ViewEmptyLabels(t *testing.T) {
	m := NewLabelDashboardModel(Theme{})
	m.SetSize(80, 10)
	m.SetData([]analysis.LabelHealth{})

	view := m.View()
	expected := "No labels found"
	if view != expected {
		t.Errorf("View() with empty labels = %q, want %q", view, expected)
	}
}

func TestLabelDashboardModel_ViewNilLabels(t *testing.T) {
	m := NewLabelDashboardModel(Theme{})
	m.SetSize(80, 10)
	// Don't call SetData, labels is nil

	view := m.View()
	expected := "No labels found"
	if view != expected {
		t.Errorf("View() with nil labels = %q, want %q", view, expected)
	}
}

func TestLabelDashboardModel_ViewSingleLabel(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 10)
	m.SetData([]analysis.LabelHealth{
		{
			Label:       "bug",
			HealthLevel: analysis.HealthLevelHealthy,
			Blocked:     0,
			Health:      90,
			Velocity:    analysis.VelocityMetrics{ClosedLast7Days: 5, ClosedLast30Days: 20},
			Freshness:   analysis.FreshnessMetrics{StaleCount: 2},
		},
	})

	view := m.View()

	// Should contain header
	if !contains(view, "Label") {
		t.Error("View should contain 'Label' header")
	}
	if !contains(view, "Health") {
		t.Error("View should contain 'Health' header")
	}
	if !contains(view, "Blocked") {
		t.Error("View should contain 'Blocked' header")
	}

	// Should contain data
	if !contains(view, "bug") {
		t.Error("View should contain label 'bug'")
	}
	if !contains(view, "5/20") { // Velocity 7d/30d
		t.Error("View should contain velocity '5/20'")
	}
}

func TestLabelDashboardModel_ViewMultipleLabels(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 20)
	m.SetData([]analysis.LabelHealth{
		{Label: "feature", HealthLevel: analysis.HealthLevelHealthy, Health: 85},
		{Label: "bug", HealthLevel: analysis.HealthLevelWarning, Health: 55},
		{Label: "critical", HealthLevel: analysis.HealthLevelCritical, Health: 20, Blocked: 5},
	})

	view := m.View()

	// All labels should be visible
	if !contains(view, "feature") {
		t.Error("View should contain 'feature'")
	}
	if !contains(view, "bug") {
		t.Error("View should contain 'bug'")
	}
	if !contains(view, "critical") {
		t.Error("View should contain 'critical'")
	}
}

func TestLabelDashboardModel_ViewHealthBar(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(100, 10)
	m.SetData([]analysis.LabelHealth{
		{Label: "test", HealthLevel: analysis.HealthLevelHealthy, Health: 50},
	})

	view := m.View()

	// Health bar uses █ and ░ characters
	if !contains(view, "█") {
		t.Error("View should contain filled bar character '█'")
	}
	if !contains(view, "░") {
		t.Error("View should contain empty bar character '░'")
	}
}

func TestLabelDashboardModel_ViewBlockedIndicator(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(100, 10)
	m.SetData([]analysis.LabelHealth{
		{Label: "blocked-label", HealthLevel: analysis.HealthLevelWarning, Blocked: 3, Health: 50},
	})

	view := m.View()

	// Blocked labels show ⛔ indicator
	if !contains(view, "⛔") {
		t.Error("View should contain blocked indicator '⛔'")
	}
}

func TestLabelDashboardModel_ViewCriticalIndicator(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(100, 10)
	m.SetData([]analysis.LabelHealth{
		{Label: "urgent", HealthLevel: analysis.HealthLevelCritical, Blocked: 0, Health: 15},
	})

	view := m.View()

	// Critical labels show ! indicator
	if !contains(view, " !") {
		t.Error("View should contain critical indicator ' !'")
	}
}

func TestLabelDashboardModel_ViewScrolling(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	// height=4 means visibleRows=3 (header row takes 1)
	m.SetSize(80, 4)

	// All same health level and health score to maintain insertion order after sort
	labels := []analysis.LabelHealth{
		{Label: "aaa", HealthLevel: analysis.HealthLevelHealthy, Health: 90},
		{Label: "bbb", HealthLevel: analysis.HealthLevelHealthy, Health: 90},
		{Label: "ccc", HealthLevel: analysis.HealthLevelHealthy, Health: 90},
		{Label: "ddd", HealthLevel: analysis.HealthLevelHealthy, Health: 90},
		{Label: "eee", HealthLevel: analysis.HealthLevelHealthy, Health: 90},
	}
	m.SetData(labels)

	// Initially should show first labels (alphabetically sorted)
	view := m.View()
	if !contains(view, "aaa") {
		t.Error("Initial view should contain 'aaa'")
	}

	// Scroll down to bottom
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	view = m.View()

	// After scrolling to end, 'eee' (last alphabetically) should be visible
	if !contains(view, "eee") {
		t.Error("After G, view should contain 'eee'")
	}
}

// =============================================================================
// SetData Sorting Tests
// =============================================================================

func TestLabelDashboardModel_SetDataSortsCriticalFirst(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 20)
	m.SetData([]analysis.LabelHealth{
		{Label: "healthy", HealthLevel: analysis.HealthLevelHealthy, Health: 90},
		{Label: "critical", HealthLevel: analysis.HealthLevelCritical, Health: 20},
		{Label: "warning", HealthLevel: analysis.HealthLevelWarning, Health: 50},
	})

	// Move cursor to first position and select
	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label != "critical" {
		t.Errorf("First label should be 'critical' (sorted by health level), got %q", label)
	}
}

func TestLabelDashboardModel_SetDataSortsByBlockedCount(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 20)
	m.SetData([]analysis.LabelHealth{
		{Label: "less-blocked", HealthLevel: analysis.HealthLevelWarning, Blocked: 2, Health: 50},
		{Label: "more-blocked", HealthLevel: analysis.HealthLevelWarning, Blocked: 10, Health: 50},
	})

	// First should be more-blocked (higher blocked count)
	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label != "more-blocked" {
		t.Errorf("First label should be 'more-blocked', got %q", label)
	}
}

func TestLabelDashboardModel_SetDataSortsByHealth(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 20)
	m.SetData([]analysis.LabelHealth{
		{Label: "better", HealthLevel: analysis.HealthLevelWarning, Blocked: 0, Health: 60},
		{Label: "worse", HealthLevel: analysis.HealthLevelWarning, Blocked: 0, Health: 45},
	})

	// First should be worse (lower health, more urgent)
	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label != "worse" {
		t.Errorf("First label should be 'worse' (lower health), got %q", label)
	}
}

func TestLabelDashboardModel_SetDataSortsByNameTiebreaker(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 20)
	m.SetData([]analysis.LabelHealth{
		{Label: "zebra", HealthLevel: analysis.HealthLevelHealthy, Blocked: 0, Health: 90},
		{Label: "alpha", HealthLevel: analysis.HealthLevelHealthy, Blocked: 0, Health: 90},
	})

	// First should be alpha (alphabetical)
	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label != "alpha" {
		t.Errorf("First label should be 'alpha' (alphabetical), got %q", label)
	}
}

func TestLabelDashboardModel_SetDataCursorBoundsCheck(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 20)

	// Set initial data with 5 labels
	m.SetData([]analysis.LabelHealth{
		{Label: "a"}, {Label: "b"}, {Label: "c"}, {Label: "d"}, {Label: "e"},
	})

	// Move cursor to position 4
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")}) // Go to end

	// Now set data with only 2 labels - cursor should be clamped
	m.SetData([]analysis.LabelHealth{
		{Label: "x"}, {Label: "y"},
	})

	// Cursor should be at position 1 (last valid position)
	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Due to sorting, we just check it's one of the valid labels
	if label != "x" && label != "y" {
		t.Errorf("Label should be 'x' or 'y', got %q", label)
	}
}

func TestLabelDashboardModel_SetDataEmptyAfterPopulated(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 20)

	m.SetData([]analysis.LabelHealth{{Label: "test"}})
	m.SetData([]analysis.LabelHealth{}) // Set to empty

	view := m.View()
	if view != "No labels found" {
		t.Errorf("View after empty SetData = %q, want 'No labels found'", view)
	}
}

// =============================================================================
// Update Edge Cases
// =============================================================================

func TestLabelDashboardModel_UpdateDownKeyAtEnd(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 20)
	m.SetData([]analysis.LabelHealth{
		{Label: "only"},
	})

	// Try to move down when already at end
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})

	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label != "only" {
		t.Errorf("Cursor should stay at 'only', got %q", label)
	}
}

func TestLabelDashboardModel_UpdateUpKeyAtStart(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 20)
	m.SetData([]analysis.LabelHealth{
		{Label: "first"},
		{Label: "second"},
	})

	// Try to move up when at start
	m.Update(tea.KeyMsg{Type: tea.KeyUp})

	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label != "first" {
		t.Errorf("Cursor should stay at 'first', got %q", label)
	}
}

func TestLabelDashboardModel_UpdateEndKeySmallList(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 100) // Very tall, all items visible
	m.SetData([]analysis.LabelHealth{
		{Label: "a"}, {Label: "b"},
	})

	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label != "b" {
		t.Errorf("After End on small list, should be at 'b', got %q", label)
	}
}

func TestLabelDashboardModel_UpdateEnterEmptyList(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 20)
	m.SetData([]analysis.LabelHealth{})

	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label != "" {
		t.Errorf("Enter on empty list should return empty string, got %q", label)
	}
}

func TestLabelDashboardModel_UpdateVisibleRowsMinimum(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(80, 0) // height=0 -> visibleRows should be 1 minimum
	m.SetData([]analysis.LabelHealth{
		{Label: "a"}, {Label: "b"}, {Label: "c"},
	})

	// Navigation should still work
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if label != "b" {
		t.Errorf("Should navigate to 'b', got %q", label)
	}
}

// =============================================================================
// Column Width Tests
// =============================================================================

func TestLabelDashboardModel_ViewNarrowWidth(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(30, 10) // Very narrow
	m.SetData([]analysis.LabelHealth{
		{Label: "very-long-label-name", HealthLevel: analysis.HealthLevelHealthy, Health: 50},
	})

	// Should not panic and should truncate
	view := m.View()
	if view == "" {
		t.Error("View should not be empty even with narrow width")
	}
}

func TestLabelDashboardModel_ViewZeroWidth(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(0, 10)
	m.SetData([]analysis.LabelHealth{
		{Label: "test", HealthLevel: analysis.HealthLevelHealthy, Health: 50},
	})

	// Should not panic
	view := m.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

// =============================================================================
// Render Cell Tests
// =============================================================================

func TestLabelDashboardModel_RenderHealthBarBounds(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(100, 10)

	tests := []struct {
		name   string
		health int
	}{
		{"zero health", 0},
		{"max health", 100},
		{"negative health", -10},
		{"over max health", 150},
		{"mid health", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.SetData([]analysis.LabelHealth{
				{Label: "test", HealthLevel: analysis.HealthLevelHealthy, Health: tt.health},
			})
			// Should not panic
			view := m.View()
			if view == "" {
				t.Error("View should not be empty")
			}
		})
	}
}

func TestLabelDashboardModel_BlockedCellZero(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(100, 10)
	m.SetData([]analysis.LabelHealth{
		{Label: "unblocked", HealthLevel: analysis.HealthLevelHealthy, Blocked: 0, Health: 90},
	})

	view := m.View()
	// Zero blocked should just show "0", not styled
	if !contains(view, "0") {
		t.Error("View should contain '0' for unblocked count")
	}
}

func TestLabelDashboardModel_BlockedCellNonZero(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(100, 10)
	m.SetData([]analysis.LabelHealth{
		{Label: "blocked", HealthLevel: analysis.HealthLevelWarning, Blocked: 5, Health: 50},
	})

	view := m.View()
	// Should contain the blocked count
	if !contains(view, "5") {
		t.Error("View should contain blocked count '5'")
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestLabelDashboardModel_FullWorkflow(t *testing.T) {
	m := NewLabelDashboardModel(createTheme())
	m.SetSize(100, 10)

	// Set up realistic data
	labels := []analysis.LabelHealth{
		{
			Label:       "frontend",
			HealthLevel: analysis.HealthLevelHealthy,
			Health:      85,
			Blocked:     0,
			Velocity:    analysis.VelocityMetrics{ClosedLast7Days: 10, ClosedLast30Days: 35},
			Freshness:   analysis.FreshnessMetrics{StaleCount: 1},
		},
		{
			Label:       "backend",
			HealthLevel: analysis.HealthLevelWarning,
			Health:      55,
			Blocked:     3,
			Velocity:    analysis.VelocityMetrics{ClosedLast7Days: 2, ClosedLast30Days: 15},
			Freshness:   analysis.FreshnessMetrics{StaleCount: 5},
		},
		{
			Label:       "critical-bug",
			HealthLevel: analysis.HealthLevelCritical,
			Health:      20,
			Blocked:     8,
			Velocity:    analysis.VelocityMetrics{ClosedLast7Days: 0, ClosedLast30Days: 2},
			Freshness:   analysis.FreshnessMetrics{StaleCount: 10},
		},
	}
	m.SetData(labels)

	// View should render without panic
	view := m.View()
	if view == "" {
		t.Fatal("View should not be empty")
	}

	// Navigate down and select
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	// Home should return to start
	m.Update(tea.KeyMsg{Type: tea.KeyHome})
	label, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// First label should be critical (sorted first)
	if label != "critical-bug" {
		t.Errorf("First label should be 'critical-bug', got %q", label)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// contains checks if substr is in s (case-sensitive)
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
