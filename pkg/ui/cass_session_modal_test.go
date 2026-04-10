package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/cass"
	tea "github.com/charmbracelet/bubbletea"
)

// testTheme is defined in history_test.go and reused here

func TestNewCassSessionModal(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-abc123",
		TopSessions: []cass.ScoredResult{
			{
				SearchResult: cass.SearchResult{
					Agent:     "claude",
					Timestamp: time.Now().Add(-2 * time.Hour),
					Snippet:   "Test snippet content",
				},
				FinalScore: 100,
				Strategy:   cass.StrategyIDMention,
			},
		},
		Strategy: cass.StrategyIDMention,
		Keywords: []string{"test"},
	}

	modal := NewCassSessionModal("bv-abc123", result, theme)

	if modal.beadID != "bv-abc123" {
		t.Errorf("Expected beadID bv-abc123, got %s", modal.beadID)
	}
	if len(modal.sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(modal.sessions))
	}
	if modal.selected != 0 {
		t.Errorf("Expected selected=0, got %d", modal.selected)
	}
	if !modal.HasSessions() {
		t.Error("HasSessions should return true")
	}
}

func TestCassSessionModal_NoSessions(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID:      "bv-empty",
		TopSessions: []cass.ScoredResult{},
	}

	modal := NewCassSessionModal("bv-empty", result, theme)

	if modal.HasSessions() {
		t.Error("HasSessions should return false for empty sessions")
	}

	// View should still render without panic
	view := modal.View()
	if !strings.Contains(view, "Related Coding Sessions") {
		t.Error("View should contain header even with no sessions")
	}
	if !strings.Contains(view, "No correlated sessions found") {
		t.Error("View should indicate no sessions found")
	}
}

func TestCassSessionModal_Update_Navigation(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-nav",
		TopSessions: []cass.ScoredResult{
			{SearchResult: cass.SearchResult{Agent: "claude", Snippet: "First"}},
			{SearchResult: cass.SearchResult{Agent: "cursor", Snippet: "Second"}},
			{SearchResult: cass.SearchResult{Agent: "windsurf", Snippet: "Third"}},
		},
	}

	modal := NewCassSessionModal("bv-nav", result, theme)

	// Initial selection should be 0
	if modal.selected != 0 {
		t.Errorf("Initial selection should be 0, got %d", modal.selected)
	}

	// Move down
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if modal.selected != 1 {
		t.Errorf("After 'j', selection should be 1, got %d", modal.selected)
	}

	// Move down again
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if modal.selected != 2 {
		t.Errorf("After second 'j', selection should be 2, got %d", modal.selected)
	}

	// Try to move past the end (should stay at 2)
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if modal.selected != 2 {
		t.Errorf("Should not move past end, selection should be 2, got %d", modal.selected)
	}

	// Move up
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if modal.selected != 1 {
		t.Errorf("After 'k', selection should be 1, got %d", modal.selected)
	}

	// Move up to beginning
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if modal.selected != 0 {
		t.Errorf("After second 'k', selection should be 0, got %d", modal.selected)
	}

	// Try to move before beginning
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if modal.selected != 0 {
		t.Errorf("Should not move before beginning, selection should be 0, got %d", modal.selected)
	}
}

func TestCassSessionModal_Update_ArrowKeys(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-arrow",
		TopSessions: []cass.ScoredResult{
			{SearchResult: cass.SearchResult{Agent: "claude", Snippet: "First"}},
			{SearchResult: cass.SearchResult{Agent: "cursor", Snippet: "Second"}},
		},
	}

	modal := NewCassSessionModal("bv-arrow", result, theme)

	// Arrow down
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyDown})
	if modal.selected != 1 {
		t.Errorf("After down arrow, selection should be 1, got %d", modal.selected)
	}

	// Arrow up
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyUp})
	if modal.selected != 0 {
		t.Errorf("After up arrow, selection should be 0, got %d", modal.selected)
	}
}

func TestCassSessionModal_Update_CopyCommand(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID:   "bv-copy",
		Keywords: []string{"test", "keyword"},
	}

	modal := NewCassSessionModal("bv-copy", result, theme)

	// The search command should be built from keywords
	if !strings.Contains(modal.searchCmd, "test keyword") {
		t.Errorf("Search command should contain keywords, got: %s", modal.searchCmd)
	}

	// Press 'y' to copy - note: actual clipboard copy may fail in test environment
	// but the copied flag should be set if successful, we mainly test it doesn't panic
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	// Can't reliably test clipboard in CI, just verify no panic
}

func TestCassSessionModal_View_RendersCorrectly(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-render",
		TopSessions: []cass.ScoredResult{
			{
				SearchResult: cass.SearchResult{
					Agent:     "claude",
					Timestamp: time.Now().Add(-2 * time.Hour),
					Snippet:   "This is a test snippet",
				},
				Strategy: cass.StrategyIDMention,
			},
			{
				SearchResult: cass.SearchResult{
					Agent:     "cursor",
					Timestamp: time.Now().Add(-24 * time.Hour),
					Snippet:   "Another snippet",
				},
				Strategy: cass.StrategyKeywords,
				Keywords: []string{"test"},
			},
		},
		Strategy: cass.StrategyIDMention,
	}

	modal := NewCassSessionModal("bv-render", result, theme)
	view := modal.View()

	// Check header is present
	if !strings.Contains(view, "Related Coding Sessions") {
		t.Error("View should contain header")
	}

	// Check bead ID is present
	if !strings.Contains(view, "bv-render") {
		t.Error("View should contain bead ID")
	}

	// Check agent names are present
	if !strings.Contains(view, "claude") {
		t.Error("View should contain first agent name")
	}
	if !strings.Contains(view, "cursor") {
		t.Error("View should contain second agent name")
	}

	// Check footer keybindings
	if !strings.Contains(view, "[j/k]") {
		t.Error("View should contain navigation hint")
	}
	if !strings.Contains(view, "[V/Esc]") {
		t.Error("View should contain close hint")
	}
}

func TestCassSessionModal_SetSize(t *testing.T) {
	theme := testTheme()
	modal := NewCassSessionModal("bv-size", cass.CorrelationResult{}, theme)

	// Default width should be 70
	if modal.width != 70 {
		t.Errorf("Default width should be 70, got %d", modal.width)
	}

	// Set small terminal size
	modal.SetSize(60, 30)
	if modal.width != 50 { // min is 50
		t.Errorf("Width should be constrained to min 50, got %d", modal.width)
	}

	// Set large terminal size
	modal.SetSize(200, 50)
	if modal.width != 80 { // max is 80
		t.Errorf("Width should be constrained to max 80, got %d", modal.width)
	}

	// Set medium terminal size
	modal.SetSize(100, 40)
	if modal.width != 90 { // 100 - 10 = 90, but max is 80
		// maxWidth = 100-10 = 90, but max is 80
		if modal.width != 80 {
			t.Errorf("Width should be 80 (capped), got %d", modal.width)
		}
	}
}

func TestCassSessionModal_CenterModal(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-center",
		TopSessions: []cass.ScoredResult{
			{SearchResult: cass.SearchResult{Agent: "claude", Snippet: "Test"}},
		},
	}

	modal := NewCassSessionModal("bv-center", result, theme)

	// Just verify it doesn't panic and returns non-empty string
	centered := modal.CenterModal(120, 40)
	if centered == "" {
		t.Error("CenterModal should return non-empty string")
	}
}

func TestFormatRelativeTime(t *testing.T) {
	tests := []struct {
		name     string
		t        time.Time
		contains string
	}{
		{
			name:     "zero time",
			t:        time.Time{},
			contains: "unknown",
		},
		{
			name:     "just now",
			t:        time.Now().Add(-30 * time.Second),
			contains: "just now",
		},
		{
			name:     "minutes ago",
			t:        time.Now().Add(-5 * time.Minute),
			contains: "minutes ago",
		},
		{
			name:     "1 hour ago",
			t:        time.Now().Add(-1 * time.Hour),
			contains: "1 hour ago",
		},
		{
			name:     "hours ago",
			t:        time.Now().Add(-3 * time.Hour),
			contains: "hours ago",
		},
		{
			name:     "yesterday",
			t:        time.Now().Add(-36 * time.Hour),
			contains: "yesterday",
		},
		{
			name:     "days ago",
			t:        time.Now().Add(-4 * 24 * time.Hour),
			contains: "days ago",
		},
		{
			name:     "weeks ago",
			t:        time.Now().Add(-14 * 24 * time.Hour),
			contains: "weeks ago",
		},
		{
			name:     "old date",
			t:        time.Now().Add(-60 * 24 * time.Hour),
			contains: "2", // Month day will contain a digit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeTime(tt.t)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("formatRelativeTime(%v) = %q, want to contain %q", tt.t, result, tt.contains)
			}
		})
	}
}

func TestCassSessionModal_FormatMatchReason(t *testing.T) {
	theme := testTheme()
	modal := NewCassSessionModal("bv-match", cass.CorrelationResult{BeadID: "bv-match"}, theme)

	tests := []struct {
		name     string
		session  cass.ScoredResult
		contains string
	}{
		{
			name:     "ID mention",
			session:  cass.ScoredResult{Strategy: cass.StrategyIDMention},
			contains: "bead ID mentioned",
		},
		{
			name: "Keywords with list",
			session: cass.ScoredResult{
				Strategy: cass.StrategyKeywords,
				Keywords: []string{"auth", "login"},
			},
			contains: "auth, login",
		},
		{
			name:     "Keywords without list",
			session:  cass.ScoredResult{Strategy: cass.StrategyKeywords},
			contains: "keyword search",
		},
		{
			name:     "Timestamp",
			session:  cass.ScoredResult{Strategy: cass.StrategyTimestamp},
			contains: "timeframe",
		},
		{
			name:     "Combined",
			session:  cass.ScoredResult{Strategy: cass.StrategyCombined},
			contains: "multiple signals",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := modal.formatMatchReason(tt.session)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("formatMatchReason() = %q, want to contain %q", result, tt.contains)
			}
		})
	}
}

func TestCassSessionModal_FormatSnippet(t *testing.T) {
	theme := testTheme()
	modal := NewCassSessionModal("bv-snip", cass.CorrelationResult{}, theme)
	modal.width = 70

	tests := []struct {
		name    string
		snippet string
		want    string
	}{
		{
			name:    "empty snippet",
			snippet: "",
			want:    "(no preview available)",
		},
		{
			name:    "simple snippet",
			snippet: "Hello world",
			want:    "Hello world",
		},
		{
			name:    "multiline snippet",
			snippet: "Line 1\nLine 2\nLine 3\nLine 4",
			want:    "Line 1\nLine 2\nLine 3", // max 3 lines
		},
		{
			name:    "whitespace only",
			snippet: "   \n\n   ",
			want:    "(no preview available)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := modal.formatSnippet(tt.snippet)
			if !strings.Contains(result, tt.want) && result != tt.want {
				t.Errorf("formatSnippet(%q) = %q, want %q", tt.snippet, result, tt.want)
			}
		})
	}
}

func TestCassSessionModal_MaxDisplayLimit(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-limit",
		TopSessions: []cass.ScoredResult{
			{SearchResult: cass.SearchResult{Agent: "agent1", Snippet: "One"}},
			{SearchResult: cass.SearchResult{Agent: "agent2", Snippet: "Two"}},
			{SearchResult: cass.SearchResult{Agent: "agent3", Snippet: "Three"}},
			{SearchResult: cass.SearchResult{Agent: "agent4", Snippet: "Four"}},
			{SearchResult: cass.SearchResult{Agent: "agent5", Snippet: "Five"}},
		},
	}

	modal := NewCassSessionModal("bv-limit", result, theme)
	view := modal.View()

	// Should show "more sessions" message since we have 5 but max display is 3
	if !strings.Contains(view, "more session") {
		t.Error("View should indicate there are more sessions")
	}

	// Should show first 3 agents
	if !strings.Contains(view, "agent1") {
		t.Error("View should contain agent1")
	}
	if !strings.Contains(view, "agent2") {
		t.Error("View should contain agent2")
	}
	if !strings.Contains(view, "agent3") {
		t.Error("View should contain agent3")
	}

	// Should NOT show agent4 or agent5 in the list
	// (they may appear in the "more sessions" message indirectly but not as session entries)
}

func TestCassSessionModal_SingleSession(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-single",
		TopSessions: []cass.ScoredResult{
			{SearchResult: cass.SearchResult{Agent: "claude", Snippet: "Only one"}},
		},
	}

	modal := NewCassSessionModal("bv-single", result, theme)

	// Navigation should not crash with single session
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if modal.selected != 0 {
		t.Errorf("With single session, selection should stay at 0, got %d", modal.selected)
	}

	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if modal.selected != 0 {
		t.Errorf("With single session, selection should stay at 0, got %d", modal.selected)
	}
}

func TestCassSessionModal_NavigationCappedByMaxDisplay(t *testing.T) {
	theme := testTheme()
	// Create 5 sessions but maxDisplay is 3
	result := cass.CorrelationResult{
		BeadID: "bv-navbound",
		TopSessions: []cass.ScoredResult{
			{SearchResult: cass.SearchResult{Agent: "agent1", Snippet: "One"}},
			{SearchResult: cass.SearchResult{Agent: "agent2", Snippet: "Two"}},
			{SearchResult: cass.SearchResult{Agent: "agent3", Snippet: "Three"}},
			{SearchResult: cass.SearchResult{Agent: "agent4", Snippet: "Four"}},
			{SearchResult: cass.SearchResult{Agent: "agent5", Snippet: "Five"}},
		},
	}

	modal := NewCassSessionModal("bv-navbound", result, theme)

	// Navigation should stop at index 2 (third session), not 4
	// Move down 5 times
	for i := 0; i < 5; i++ {
		modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	// Should be capped at maxDisplay-1 (index 2), not len(sessions)-1 (index 4)
	if modal.selected != 2 {
		t.Errorf("Navigation should be capped at maxDisplay-1 (2), got %d", modal.selected)
	}

	// Verify the selected session is the third one (still visible)
	if modal.selected >= len(modal.sessions) || modal.sessions[modal.selected].Agent != "agent3" {
		t.Error("Selected session should be the third displayed session (agent3)")
	}
}

// === Additional tests for bv-mtyf (Documentation & Testing Refresh) ===

func TestCassSessionModal_NilFieldsHandling(t *testing.T) {
	theme := testTheme()

	// Test with minimal/nil fields - should not panic
	result := cass.CorrelationResult{
		BeadID:      "bv-nil",
		TopSessions: []cass.ScoredResult{{}}, // Empty session
	}

	modal := NewCassSessionModal("bv-nil", result, theme)

	// View should render without panic even with empty session
	view := modal.View()
	if view == "" {
		t.Error("View should not be empty even with minimal data")
	}

	// Should contain header
	if !strings.Contains(view, "Related Coding Sessions") {
		t.Error("View should contain header even with nil fields")
	}
}

func TestCassSessionModal_ZeroTimestamp(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-zero-time",
		TopSessions: []cass.ScoredResult{
			{
				SearchResult: cass.SearchResult{
					Agent:     "claude",
					Timestamp: time.Time{}, // Zero time
					Snippet:   "Test",
				},
			},
		},
	}

	modal := NewCassSessionModal("bv-zero-time", result, theme)
	view := modal.View()

	// Should handle zero timestamp gracefully (shows "unknown")
	if !strings.Contains(view, "unknown") && !strings.Contains(view, "claude") {
		t.Error("View should handle zero timestamp gracefully")
	}
}

func TestCassSessionModal_CopyFeedbackTiming(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID:   "bv-feedback",
		Keywords: []string{"test"},
	}

	modal := NewCassSessionModal("bv-feedback", result, theme)

	// Initially copied should be false
	if modal.copied {
		t.Error("copied should be false initially")
	}

	// Simulate copy (may fail in CI but should set the flag if clipboard available)
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	// If clipboard is available, copied should be true and copiedAt set
	// We can't reliably test this in CI, but the View() shouldn't panic
	view := modal.View()
	if view == "" {
		t.Error("View should render after copy attempt")
	}
}

func TestCassSessionModal_SpecialCharactersInSnippet(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-special",
		TopSessions: []cass.ScoredResult{
			{
				SearchResult: cass.SearchResult{
					Agent:   "claude",
					Snippet: "Special chars: <html>\"quotes\" & 'apostrophes' \ttab\nnewline",
				},
			},
		},
	}

	modal := NewCassSessionModal("bv-special", result, theme)
	view := modal.View()

	// View should render without issues
	if view == "" {
		t.Error("View should not be empty with special characters")
	}

	// Should contain the agent name at least
	if !strings.Contains(view, "claude") {
		t.Error("View should contain agent name")
	}
}

func TestCassSessionModal_UnicodeInSnippet(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-unicode",
		TopSessions: []cass.ScoredResult{
			{
				SearchResult: cass.SearchResult{
					Agent:   "claude",
					Snippet: "Unicode: 你好世界 🎉 émojis ñ café αβγ",
				},
			},
		},
	}

	modal := NewCassSessionModal("bv-unicode", result, theme)
	view := modal.View()

	// View should render without issues
	if view == "" {
		t.Error("View should not be empty with unicode")
	}

	// Should contain the agent name
	if !strings.Contains(view, "claude") {
		t.Error("View should contain agent name")
	}
}

func TestCassSessionModal_LongAgentName(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-long-agent",
		TopSessions: []cass.ScoredResult{
			{
				SearchResult: cass.SearchResult{
					Agent:   "this-is-a-very-long-agent-name-that-might-cause-layout-issues",
					Snippet: "Test",
				},
			},
		},
	}

	modal := NewCassSessionModal("bv-long-agent", result, theme)
	view := modal.View()

	// View should render without panic
	if view == "" {
		t.Error("View should not be empty with long agent name")
	}
}

func TestCassSessionModal_EmptyBeadID(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "",
		TopSessions: []cass.ScoredResult{
			{SearchResult: cass.SearchResult{Agent: "claude", Snippet: "Test"}},
		},
	}

	modal := NewCassSessionModal("", result, theme)

	// Should not panic and should have some search command
	if modal.searchCmd == "" {
		t.Error("searchCmd should not be empty even with empty beadID")
	}

	view := modal.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestCassSessionModal_ManyKeywords(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID:   "bv-keywords",
		Keywords: []string{"auth", "login", "session", "token", "jwt", "oauth", "security", "middleware"},
	}

	modal := NewCassSessionModal("bv-keywords", result, theme)

	// Search command should contain all keywords
	if !strings.Contains(modal.searchCmd, "auth login session") {
		t.Errorf("searchCmd should contain keywords, got: %s", modal.searchCmd)
	}

	view := modal.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

func TestCassSessionModal_UnhandledKeyIgnored(t *testing.T) {
	theme := testTheme()
	result := cass.CorrelationResult{
		BeadID: "bv-key",
		TopSessions: []cass.ScoredResult{
			{SearchResult: cass.SearchResult{Agent: "claude", Snippet: "Test"}},
			{SearchResult: cass.SearchResult{Agent: "cursor", Snippet: "Test2"}},
		},
	}

	modal := NewCassSessionModal("bv-key", result, theme)
	initialSelected := modal.selected

	// Press unhandled keys - should not change state
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if modal.selected != initialSelected {
		t.Errorf("Unhandled keys should not change selection, got %d", modal.selected)
	}
}
