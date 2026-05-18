package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// referenceIssue is a fixture used across search-score tests.
func referenceIssue() model.Issue {
	return model.Issue{
		ID:        "bt-test",
		Title:     "Test issue",
		Status:    model.StatusInProgress,
		Priority:  3,
		UpdatedAt: time.Now().Add(-40 * 24 * time.Hour), // ~1mo ago
	}
}

// TestSearchScoreBar_Filled checks that a value of 1.0 produces 10 full blocks.
func TestSearchScoreBar_Filled(t *testing.T) {
	bar := searchScoreBar(1.0)
	if bar != "██████████" {
		t.Fatalf("expected 10 full blocks for value 1.0, got %q", bar)
	}
}

// TestSearchScoreBar_Empty checks that a value of 0.0 produces 10 light shades.
func TestSearchScoreBar_Empty(t *testing.T) {
	bar := searchScoreBar(0.0)
	if bar != "░░░░░░░░░░" {
		t.Fatalf("expected 10 light shades for value 0.0, got %q", bar)
	}
}

// TestSearchScoreBar_Half checks that a value of 0.5 produces 5 filled + 5 empty.
func TestSearchScoreBar_Half(t *testing.T) {
	bar := searchScoreBar(0.5)
	if bar != "█████░░░░░" {
		t.Fatalf("expected 5+5 split for value 0.5, got %q", bar)
	}
}

// TestSearchScoreBar_Clamp checks that values > 1.0 are clamped to full bar.
func TestSearchScoreBar_Clamp(t *testing.T) {
	bar := searchScoreBar(2.0)
	if bar != "██████████" {
		t.Fatalf("expected clamped full bar for value 2.0, got %q", bar)
	}
}

// TestSearchScoreBar_Negative checks that negative values are treated as absolute.
func TestSearchScoreBar_Negative(t *testing.T) {
	bar := searchScoreBar(-0.5)
	if bar != "█████░░░░░" {
		t.Fatalf("expected abs(-0.5) = 0.5 bar, got %q", bar)
	}
}

// TestSearchScoreBar_Width checks that every bar is exactly 10 characters.
func TestSearchScoreBar_Width(t *testing.T) {
	vals := []float64{0, 0.1, 0.25, 0.5, 0.75, 0.9, 1.0}
	for _, v := range vals {
		bar := searchScoreBar(v)
		if len([]rune(bar)) != 10 {
			t.Errorf("bar for %.2f has %d runes, want 10: %q", v, len([]rune(bar)), bar)
		}
	}
}

// TestBuildSearchScoresANSI_SortOrder verifies that above-threshold components
// appear in descending contribution order. Status (1.0) > priority (0.4) >
// recency (0.285) so "status" bar must appear before "priority" before "recency".
func TestBuildSearchScoresANSI_SortOrder(t *testing.T) {
	components := map[string]float64{
		"pagerank": 0.000,
		"status":   1.000,
		"impact":   0.000,
		"priority": 0.400,
		"recency":  0.285,
	}
	item := referenceIssue()
	out := buildSearchScoresANSI(components, 0.484, 0.601, item)

	idxStatus := strings.Index(out, "status")
	idxPriority := strings.Index(out, "priority")
	idxRecency := strings.Index(out, "recency")

	if idxStatus < 0 {
		t.Fatal("output missing 'status'")
	}
	if idxPriority < 0 {
		t.Fatal("output missing 'priority'")
	}
	if idxRecency < 0 {
		t.Fatal("output missing 'recency'")
	}
	if idxStatus > idxPriority {
		t.Errorf("'status' (%.3f) should appear before 'priority' (%.3f)", 1.0, 0.4)
	}
	if idxPriority > idxRecency {
		t.Errorf("'priority' (%.3f) should appear before 'recency' (%.3f)", 0.4, 0.285)
	}
}

// TestBuildSearchScoresANSI_Suppression verifies that pagerank (0.000) and
// impact (0.000) are collapsed to the "not contributing" line rather than
// appearing as bar rows.
func TestBuildSearchScoresANSI_Suppression(t *testing.T) {
	components := map[string]float64{
		"pagerank": 0.000,
		"status":   1.000,
		"impact":   0.000,
		"priority": 0.400,
		"recency":  0.285,
	}
	item := referenceIssue()
	out := buildSearchScoresANSI(components, 0.484, 0.601, item)

	if !strings.Contains(out, "not contributing") {
		t.Errorf("output should contain 'not contributing' for suppressed components:\n%s", out)
	}
	// Suppressed names must appear on the collapsed line only, not as bar rows.
	// The "not contributing" line should contain pagerank and impact.
	ncLine := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "not contributing") {
			ncLine = line
			break
		}
	}
	if ncLine == "" {
		t.Fatal("could not find 'not contributing' line")
	}
	if !strings.Contains(ncLine, "pagerank") {
		t.Errorf("'not contributing' line should mention 'pagerank': %q", ncLine)
	}
	if !strings.Contains(ncLine, "impact") {
		t.Errorf("'not contributing' line should mention 'impact': %q", ncLine)
	}
}

// TestBuildSearchScoresANSI_SummaryLine verifies the final Hybrid/Text line
// is always present with formatted scores.
func TestBuildSearchScoresANSI_SummaryLine(t *testing.T) {
	components := map[string]float64{
		"status":   1.000,
		"priority": 0.400,
	}
	item := referenceIssue()
	out := buildSearchScoresANSI(components, 0.484, 0.601, item)

	if !strings.Contains(out, "Hybrid: 0.48") {
		t.Errorf("output should contain 'Hybrid: 0.48', got:\n%s", out)
	}
	if !strings.Contains(out, "Text: 0.60") {
		t.Errorf("output should contain 'Text: 0.60', got:\n%s", out)
	}
}

// TestBuildSearchScoresANSI_AllZero verifies that when all components are zero,
// the "not contributing" line appears and the Hybrid/Text line is still present.
func TestBuildSearchScoresANSI_AllZero(t *testing.T) {
	components := map[string]float64{
		"pagerank": 0.0,
		"status":   0.0,
		"priority": 0.0,
	}
	item := referenceIssue()
	out := buildSearchScoresANSI(components, 0.10, 0.10, item)

	if !strings.Contains(out, "not contributing") {
		t.Errorf("all-zero components should produce 'not contributing' line:\n%s", out)
	}
	if !strings.Contains(out, "Hybrid:") {
		t.Errorf("summary line should still appear:\n%s", out)
	}
}

// TestSearchScoreSummary_TopThree verifies that the summary picks the top 3
// above-threshold contributors and joins them with " + ".
func TestSearchScoreSummary_TopThree(t *testing.T) {
	components := map[string]float64{
		"pagerank": 0.000,
		"status":   1.000,
		"impact":   0.000,
		"priority": 0.400,
		"recency":  0.285,
	}
	summary := searchScoreSummary(components)
	// Top 3 by absolute value: status (1.0), priority (0.4), recency (0.285)
	// Expected tokens: "active", "priority", "recent"
	if !strings.Contains(summary, "active") {
		t.Errorf("summary should contain 'active' for status: %q", summary)
	}
	if !strings.Contains(summary, "priority") {
		t.Errorf("summary should contain 'priority': %q", summary)
	}
	if !strings.Contains(summary, "recent") {
		t.Errorf("summary should contain 'recent' for recency: %q", summary)
	}
	if !strings.Contains(summary, " + ") {
		t.Errorf("summary should join with ' + ': %q", summary)
	}
}

// TestSearchScoreSummary_BelowThreshold verifies that when all components are
// below the suppression threshold, the summary is empty.
func TestSearchScoreSummary_BelowThreshold(t *testing.T) {
	components := map[string]float64{
		"pagerank": 0.01,
		"status":   0.02,
	}
	summary := searchScoreSummary(components)
	if summary != "" {
		t.Errorf("expected empty summary when all components below threshold, got %q", summary)
	}
}

// TestSearchScoreSummary_OneContributor verifies that a single contributor
// produces a summary without " + ".
func TestSearchScoreSummary_OneContributor(t *testing.T) {
	components := map[string]float64{
		"status": 1.0,
	}
	summary := searchScoreSummary(components)
	if summary == "" {
		t.Fatal("expected non-empty summary for one above-threshold contributor")
	}
	if strings.Contains(summary, " + ") {
		t.Errorf("single-contributor summary should not contain ' + ': %q", summary)
	}
}

// TestSearchScoreAnchor_Status verifies that in_progress maps to "active".
func TestSearchScoreAnchor_Status(t *testing.T) {
	item := referenceIssue() // StatusInProgress
	anchor := searchScoreAnchor("status", 1.0, item)
	if !strings.HasPrefix(strings.TrimSpace(anchor), "active") {
		t.Errorf("expected 'active' for in_progress status, got %q", anchor)
	}
}

// TestSearchScoreAnchor_Priority verifies that priority maps to P-label.
func TestSearchScoreAnchor_Priority(t *testing.T) {
	item := referenceIssue() // Priority 3
	anchor := searchScoreAnchor("priority", 0.4, item)
	if !strings.HasPrefix(strings.TrimSpace(anchor), "P3") {
		t.Errorf("expected 'P3' for priority 3, got %q", anchor)
	}
}

// TestSearchScoreAnchor_Recency verifies that recency produces a relative time.
func TestSearchScoreAnchor_Recency(t *testing.T) {
	item := referenceIssue() // ~40d ago
	anchor := searchScoreAnchor("recency", 0.285, item)
	trimmed := strings.TrimSpace(anchor)
	if !strings.Contains(trimmed, "ago") && trimmed != "now" {
		t.Errorf("expected relative time string for recency, got %q", anchor)
	}
}

// TestSearchScoreAnchor_Pagerank verifies tier labels for various PR scores.
func TestSearchScoreAnchor_Pagerank(t *testing.T) {
	item := referenceIssue()
	cases := []struct {
		value    float64
		contains string
	}{
		{0.15, "top 5%"},
		{0.05, "top 20%"},
		{0.02, "mid"},
		{0.005, "low"},
		{0.0, "-"},
	}
	for _, tc := range cases {
		anchor := searchScoreAnchor("pagerank", tc.value, item)
		if !strings.Contains(anchor, tc.contains) {
			t.Errorf("pagerank %.3f: expected anchor to contain %q, got %q", tc.value, tc.contains, anchor)
		}
	}
}

// TestSearchScoreMinAbs_ThresholdBoundary verifies that a component at exactly
// the threshold is included, and one just below is suppressed.
func TestSearchScoreMinAbs_ThresholdBoundary(t *testing.T) {
	item := referenceIssue()

	// Exactly at threshold - should appear as a bar row.
	atThreshold := map[string]float64{
		"status": searchScoreMinAbs,
	}
	out := buildSearchScoresANSI(atThreshold, 0.1, 0.1, item)
	if strings.Contains(out, "not contributing") {
		t.Errorf("component at exactly threshold should NOT be suppressed:\n%s", out)
	}

	// Just below threshold - should be suppressed.
	belowThreshold := map[string]float64{
		"status": searchScoreMinAbs - 0.001,
	}
	out2 := buildSearchScoresANSI(belowThreshold, 0.1, 0.1, item)
	if !strings.Contains(out2, "not contributing") {
		t.Errorf("component below threshold should be suppressed:\n%s", out2)
	}
}
