package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestFooterData_StatusBarOverride(t *testing.T) {
	fd := FooterData{
		Width:       80,
		StatusMsg:   "Copied bt-abc1 to clipboard",
		StatusIsErr: false,
		FilterText:  "OPEN",
		FilterIcon:  "📂",
		TotalItems:  42,
	}
	out := fd.Render()
	if !strings.Contains(out, "Copied bt-abc1 to clipboard") {
		t.Errorf("status message should appear in output")
	}
	// When status is active, filter badge should NOT appear
	if strings.Contains(out, "OPEN") {
		t.Errorf("filter badge should not appear when status message is active")
	}
}

func TestFooterData_ErrorStatusBar(t *testing.T) {
	fd := FooterData{
		Width:       80,
		StatusMsg:   "No issue selected",
		StatusIsErr: true,
		TotalItems:  10,
	}
	out := fd.Render()
	if !strings.Contains(out, "No issue selected") {
		t.Errorf("error status message should appear in output")
	}
}

func TestFooterData_NormalFooter(t *testing.T) {
	fd := FooterData{
		Width:        120,
		FilterText:   "OPEN",
		FilterIcon:   "📂",
		HintText:     "L:labels • h:detail",
		CountOpen:    10,
		CountReady:   5,
		CountBlocked: 2,
		CountClosed:  3,
		TotalItems:   20,
		KeyHints:     []string{"⏎ details", "? help"},
	}
	out := fd.Render()
	if !strings.Contains(out, "OPEN") {
		t.Errorf("filter badge text should appear")
	}
	if !strings.Contains(out, "20 issues") {
		t.Errorf("issue count should appear")
	}
}

func TestFooterData_WorkspaceBadges(t *testing.T) {
	fd := FooterData{
		Width:            120,
		FilterText:       "ALL",
		FilterIcon:       "📋",
		HintText:         "L:labels • h:detail",
		WorkspaceMode:    true,
		WorkspaceSummary: "3 repos",
		RepoFilterLabel:  "bt, beads",
		TotalItems:       100,
		KeyHints:         []string{"? help"},
	}
	out := fd.Render()
	if !strings.Contains(out, "3 repos") {
		t.Errorf("workspace summary should appear")
	}
	if !strings.Contains(out, "bt, beads") {
		t.Errorf("repo filter label should appear")
	}
}

func TestFooterData_WorkerBadgeLevels(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		level WorkerLevel
		want  bool // should produce non-empty output
	}{
		{"none", "", WorkerLevelNone, false},
		{"info", "⠋ refreshing", WorkerLevelInfo, true},
		{"warning", "⚠ bg poll (5s)", WorkerLevelWarning, true},
		{"critical", "⚠ worker unresponsive", WorkerLevelCritical, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := FooterData{WorkerText: tt.text, WorkerLevel: tt.level}
			out := fd.renderWorkerBadge()
			if tt.want && out == "" {
				t.Errorf("expected non-empty worker badge for level %d", tt.level)
			}
			if !tt.want && out != "" {
				t.Errorf("expected empty worker badge for level %d", tt.level)
			}
		})
	}
}

func TestFooterData_AlertsBadge(t *testing.T) {
	fd := FooterData{AlertCount: 3, CriticalCount: 1, WarningCount: 2}
	out := fd.renderAlertsBadge()
	if !strings.Contains(out, "3 alerts") {
		t.Errorf("alert count should appear: %s", out)
	}
}

func TestFooterData_NoAlerts(t *testing.T) {
	fd := FooterData{AlertCount: 0}
	out := fd.renderAlertsBadge()
	if out != "" {
		t.Errorf("no alerts should produce empty badge")
	}
}

func TestFooterData_TimeTravelOverridesStats(t *testing.T) {
	fd := FooterData{
		Width:            120,
		FilterText:       "OPEN",
		FilterIcon:       "📂",
		HintText:         "L:labels • h:detail",
		TimeTravelActive: true,
		TimeTravelStats:  "⏱ 3d: +5 ✅2 ~3",
		TotalItems:       50,
		KeyHints:         []string{"? help"},
	}
	out := fd.Render()
	if !strings.Contains(out, "⏱ 3d") {
		t.Errorf("time travel stats should appear")
	}
}

func TestFooterData_SearchBadge(t *testing.T) {
	fd := FooterData{
		Width:      120,
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "L:labels • h:detail",
		SearchMode: "semantic",
		TotalItems: 30,
		KeyHints:   []string{"? help"},
	}
	out := fd.Render()
	if !strings.Contains(out, "semantic") {
		t.Errorf("search mode should appear in output")
	}
}

func TestFooterData_SortBadge(t *testing.T) {
	fd := FooterData{
		Width:      120,
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "L:labels • h:detail",
		SortLabel:  "priority",
		TotalItems: 30,
		KeyHints:   []string{"? help"},
	}
	out := fd.Render()
	if !strings.Contains(out, "priority") {
		t.Errorf("sort label should appear in output")
	}
}

func TestFooterData_ProgressiveHintTruncation(t *testing.T) {
	// Provide many hints in a narrow terminal — should truncate to fit
	fd := FooterData{
		Width:      40, // very narrow
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "L:labels",
		TotalItems: 10,
		KeyHints: []string{
			"⏎ details",
			"t diff",
			"S triage",
			"l labels",
			"? help",
		},
	}
	out := fd.Render()
	// Just verify it renders without panic and produces output
	if lipgloss.Width(out) == 0 {
		t.Errorf("footer should produce non-empty output even when narrow")
	}
}

func TestFooterData_UpdateBadge(t *testing.T) {
	fd := FooterData{
		Width:      120,
		FilterText: "ALL",
		FilterIcon: "📋",
		HintText:   "L:labels",
		UpdateTag:  "v0.2.0",
		TotalItems: 10,
		KeyHints:   []string{"? help"},
	}
	out := fd.Render()
	if !strings.Contains(out, "v0.2.0") {
		t.Errorf("update tag should appear in output")
	}
}

func TestFooterData_SecondaryInstance(t *testing.T) {
	fd := FooterData{
		Width:        120,
		FilterText:   "ALL",
		FilterIcon:   "📋",
		HintText:     "L:labels",
		SecondaryPID: 12345,
		TotalItems:   10,
		KeyHints:     []string{"? help"},
	}
	out := fd.Render()
	if !strings.Contains(out, "12345") {
		t.Errorf("secondary PID should appear in output")
	}
}
