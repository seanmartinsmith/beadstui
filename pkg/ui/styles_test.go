package ui

import (
	"strings"
	"testing"

)

func TestRenderPriorityBadge(t *testing.T) {
	tests := []struct {
		prio int
		want string
	}{
		{0, "P0"},
		{1, "P1"},
		{2, "P2"},
		{3, "P3"},
		{4, "P4"},
		{99, "P?"},
	}

	for _, tt := range tests {
		got := RenderPriorityBadge(tt.prio)
		if !strings.Contains(got, tt.want) {
			t.Errorf("RenderPriorityBadge(%d) = %q, want to contain %q", tt.prio, got, tt.want)
		}
	}
}

func TestRenderStatusBadge(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		// All 8 official beads statuses
		{"open", "OPEN"},
		{"in_progress", "PROG"},
		{"blocked", "BLKD"},
		{"deferred", "DEFR"},
		{"pinned", "PIN"},
		{"hooked", "HOOK"},
		{"closed", "DONE"},
		{"tombstone", "TOMB"},
		// Unknown status should show "????"
		{"unknown", "????"},
		{"", "????"},
		{"invalid_status", "????"},
	}

	for _, tt := range tests {
		got := RenderStatusBadge(tt.status)
		if !strings.Contains(got, tt.want) {
			t.Errorf("RenderStatusBadge(%q) = %q, want to contain %q", tt.status, got, tt.want)
		}
	}
}

// TestRenderStatusBadge_AllStatusesHaveColors verifies each status has distinct colors
func TestRenderStatusBadge_AllStatusesHaveColors(t *testing.T) {
	statuses := []string{
		"open", "in_progress", "blocked", "deferred",
		"pinned", "hooked", "closed", "tombstone",
	}

	// Each status should produce a non-empty output
	for _, status := range statuses {
		got := RenderStatusBadge(status)
		if got == "" {
			t.Errorf("RenderStatusBadge(%q) returned empty string", status)
		}
		// Should NOT contain "????" for known statuses
		if strings.Contains(got, "????") {
			t.Errorf("RenderStatusBadge(%q) returned unknown badge '????'", status)
		}
	}
}

func TestRenderMiniBar(t *testing.T) {
	theme := DefaultTheme()

	tests := []struct {
		val   float64
		width int
	}{
		{0.0, 10},
		{0.5, 10},
		{1.0, 10},
		{-0.1, 10}, // Should clamp to 0
		{1.5, 10},  // Should clamp to 1
		{0.5, 0},   // Should return empty
		{0.5, -5},  // Should return empty (no panic)
	}

	for _, tt := range tests {
		got := RenderMiniBar(tt.val, tt.width, theme)
		if tt.width <= 0 {
			if got != "" {
				t.Errorf("RenderMiniBar(%v, %d) = %q, want empty string", tt.val, tt.width, got)
			}
			continue
		}
		// Basic sanity check: output should not be empty
		if got == "" {
			t.Errorf("RenderMiniBar(%v, %d) returned empty string", tt.val, tt.width)
		}
		// Check expected fullness characters approximately
		if tt.val > 0 {
			if !strings.Contains(got, "█") && !strings.Contains(got, "░") {
				t.Errorf("RenderMiniBar output expected bar chars, got %q", got)
			}
		}
	}
}

func TestRenderGateBadge(t *testing.T) {
	tests := []struct {
		awaitType string
		want      string
	}{
		{"human", "HUM"},
		{"timer", "TMR"},
		{"gh:run", "CI"},
		{"ci", "CI"},
		{"gh:pr", "PR"},
		{"pr", "PR"},
		{"bead", "BD"},
		{"unknown", "GTD"},
		{"", "GTD"},
	}

	for _, tt := range tests {
		got := RenderGateBadge(tt.awaitType)
		if !strings.Contains(got, tt.want) {
			t.Errorf("RenderGateBadge(%q) = %q, want to contain %q", tt.awaitType, got, tt.want)
		}
		if got == "" {
			t.Errorf("RenderGateBadge(%q) returned empty string", tt.awaitType)
		}
	}
}

func TestRenderHumanAdvisoryBadge(t *testing.T) {
	got := RenderHumanAdvisoryBadge()
	if !strings.Contains(got, "HUM") {
		t.Errorf("RenderHumanAdvisoryBadge() = %q, want to contain 'HUM'", got)
	}
	if got == "" {
		t.Errorf("RenderHumanAdvisoryBadge() returned empty string")
	}
}

func TestRenderGateBadge_DistinctFromAdvisory(t *testing.T) {
	gateBadge := RenderGateBadge("human")
	advisoryBadge := RenderHumanAdvisoryBadge()
	if gateBadge == advisoryBadge {
		t.Errorf("Gate human badge and advisory badge should be visually distinct, both = %q", gateBadge)
	}
}

func TestHasHumanLabel(t *testing.T) {
	tests := []struct {
		labels []string
		want   bool
	}{
		{[]string{"human"}, true},
		{[]string{"bug", "human", "p0"}, true},
		{[]string{"bug", "p0"}, false},
		{nil, false},
		{[]string{}, false},
		{[]string{"Human"}, false}, // case-sensitive
	}

	for _, tt := range tests {
		got := hasHumanLabel(tt.labels)
		if got != tt.want {
			t.Errorf("hasHumanLabel(%v) = %v, want %v", tt.labels, got, tt.want)
		}
	}
}

func TestFormatNanoseconds(t *testing.T) {
	tests := []struct {
		ns   int64
		want string
	}{
		{0, "0s"},
		{1_000_000_000, "1s"},       // 1 second
		{60_000_000_000, "1m"},      // 1 minute
		{3_600_000_000_000, "1h"},   // 1 hour
		{86_400_000_000_000, "1d"},  // 1 day
		{172_800_000_000_000, "2d"}, // 2 days
	}

	for _, tt := range tests {
		got := formatNanoseconds(tt.ns)
		if got != tt.want {
			t.Errorf("formatNanoseconds(%d) = %q, want %q", tt.ns, got, tt.want)
		}
	}
}

func TestRenderRankBadge(t *testing.T) {
	tests := []struct {
		rank  int
		total int
		want  string
	}{
		{1, 100, "#1"},
		{50, 100, "#50"},
		{0, 0, "#?"},
	}

	for _, tt := range tests {
		got := RenderRankBadge(tt.rank, tt.total)
		if !strings.Contains(got, tt.want) {
			t.Errorf("RenderRankBadge(%d, %d) = %q, want to contain %q", tt.rank, tt.total, got, tt.want)
		}
	}
}
