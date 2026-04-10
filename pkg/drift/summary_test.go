package drift

import (
	"strings"
	"testing"
)

func TestSummary_NoDrift(t *testing.T) {
	r := &Result{HasDrift: false}
	got := r.Summary()
	want := "No drift detected"
	if !strings.Contains(got, want) {
		t.Errorf("Summary() = %q, want substring %q", got, want)
	}
}

func TestSummary_MixedSeverities(t *testing.T) {
	r := &Result{
		HasDrift:      true,
		CriticalCount: 1,
		WarningCount:  1,
		InfoCount:     1,
		Alerts: []Alert{
			{
				Type:     AlertNewCycle,
				Severity: SeverityCritical,
				Message:  "Critical Alert",
				Details:  []string{"Detail A", "Detail B"},
			},
			{
				Type:     AlertBlockedIncrease,
				Severity: SeverityWarning,
				Message:  "Warning Alert",
			},
			{
				Type:     AlertNodeCountChange,
				Severity: SeverityInfo,
				Message:  "Info Alert",
			},
		},
	}

	got := r.Summary()

	// Check headers
	if !strings.Contains(got, "🔴 CRITICAL: 1") {
		t.Error("Summary missing critical count")
	}
	if !strings.Contains(got, "🟡 WARNING: 1") {
		t.Error("Summary missing warning count")
	}
	if !strings.Contains(got, "🔵 INFO: 1") {
		t.Error("Summary missing info count")
	}

	// Check icons and messages
	if !strings.Contains(got, "🔴 [new_cycle] Critical Alert") {
		t.Error("Summary missing critical alert line")
	}
	if !strings.Contains(got, "🟡 [blocked_increase] Warning Alert") {
		t.Error("Summary missing warning alert line")
	}
	if !strings.Contains(got, "ℹ️ [node_count_change] Info Alert") {
		t.Error("Summary missing info alert line")
	}

	// Check details
	if !strings.Contains(got, "- Detail A") {
		t.Error("Summary missing detail A")
	}
	if !strings.Contains(got, "- Detail B") {
		t.Error("Summary missing detail B")
	}
}

func TestSummary_OnlyInfo(t *testing.T) {
	r := &Result{
		HasDrift:  true,
		InfoCount: 2,
		Alerts: []Alert{
			{Type: AlertNodeCountChange, Severity: SeverityInfo, Message: "Info 1"},
			{Type: AlertEdgeCountChange, Severity: SeverityInfo, Message: "Info 2"},
		},
	}

	got := r.Summary()

	if strings.Contains(got, "CRITICAL") {
		t.Error("Summary should not contain CRITICAL")
	}
	if strings.Contains(got, "WARNING") {
		t.Error("Summary should not contain WARNING")
	}
	if !strings.Contains(got, "🔵 INFO: 2") {
		t.Error("Summary missing info count")
	}
}
