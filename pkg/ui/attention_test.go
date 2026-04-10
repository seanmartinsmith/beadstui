package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/mattn/go-runewidth"
)

func TestComputeAttentionView_Empty(t *testing.T) {
	out, err := ComputeAttentionView(nil, 80)
	if err != nil {
		t.Fatalf("ComputeAttentionView error: %v", err)
	}
	if !strings.Contains(out, "Rank") || !strings.Contains(out, "Label") || !strings.Contains(out, "Attention") || !strings.Contains(out, "Reason") {
		t.Fatalf("expected header columns, got:\n%s", out)
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line (header only), got %d:\n%s", len(lines), out)
	}
}

func TestComputeAttentionView_RespectsWidthWhenWideEnough(t *testing.T) {
	const width = 80
	out, err := ComputeAttentionView(nil, width)
	if err != nil {
		t.Fatalf("ComputeAttentionView error: %v", err)
	}

	line := strings.TrimSuffix(out, "\n")
	if got := runewidth.StringWidth(line); got != width {
		t.Fatalf("expected header width %d, got %d:\n%q", width, got, line)
	}
}

func TestComputeAttentionView_SingleLabelFormatting(t *testing.T) {
	now := time.Now().UTC()
	issues := []model.Issue{{
		ID:        "A",
		Title:     "A",
		Status:    model.StatusOpen,
		IssueType: model.TypeTask,
		Priority:  2,
		Labels:    []string{"backend"},
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-1 * time.Hour),
	}}

	out, err := ComputeAttentionView(issues, 80)
	if err != nil {
		t.Fatalf("ComputeAttentionView error: %v", err)
	}
	if !strings.Contains(out, "backend") {
		t.Fatalf("expected label in output, got:\n%s", out)
	}
	if !strings.Contains(out, "1.00") {
		t.Fatalf("expected attention score with 2 decimals, got:\n%s", out)
	}
	if !strings.Contains(out, "blocked=0 stale=0 vel=1.0") {
		t.Fatalf("expected reason fields, got:\n%s", out)
	}
}

func TestComputeAttentionView_LimitsToTop10AndIsDeterministic(t *testing.T) {
	now := time.Now().UTC()

	// Create 11 distinct labels; with identical issue shape they tie on score and
	// should sort by label name (then truncated to top 10).
	var issues []model.Issue
	for i := 1; i <= 11; i++ {
		label := "l" + pad2(i) // l01..l11 for lexicographic stability
		issues = append(issues, model.Issue{
			ID:        "ISSUE-" + label,
			Title:     "Issue " + label,
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Priority:  2,
			Labels:    []string{label},
			CreatedAt: now.Add(-24 * time.Hour),
			UpdatedAt: now.Add(-1 * time.Hour),
		})
	}

	out, err := ComputeAttentionView(issues, 120)
	if err != nil {
		t.Fatalf("ComputeAttentionView error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 1+10 {
		t.Fatalf("expected %d lines (header + 10 rows), got %d:\n%s", 11, len(lines), out)
	}

	if !strings.Contains(out, "l01") || !strings.Contains(out, "l10") {
		t.Fatalf("expected output to include l01..l10 labels, got:\n%s", out)
	}
	if strings.Contains(out, "l11") {
		t.Fatalf("expected l11 to be excluded by top-10 limit, got:\n%s", out)
	}
}

func TestComputeAttentionView_TruncatesCells(t *testing.T) {
	now := time.Now().UTC()
	longLabel := "this-is-a-very-very-long-label-name"
	issues := []model.Issue{{
		ID:        "A",
		Title:     "A",
		Status:    model.StatusOpen,
		IssueType: model.TypeTask,
		Priority:  2,
		Labels:    []string{longLabel},
		CreatedAt: now.Add(-24 * time.Hour),
		UpdatedAt: now.Add(-24 * time.Hour),
	}}

	out, err := ComputeAttentionView(issues, 40) // forces reason col min width (20) + truncation
	if err != nil {
		t.Fatalf("ComputeAttentionView error: %v", err)
	}

	wantLabel := truncate(longLabel, 18)
	if !strings.Contains(out, wantLabel) {
		t.Fatalf("expected truncated label %q in output, got:\n%s", wantLabel, out)
	}

	wantReason := truncate("blocked=0 stale=0 vel=1.0", 20)
	if !strings.Contains(out, wantReason) {
		t.Fatalf("expected truncated reason %q in output, got:\n%s", wantReason, out)
	}
}

func pad2(i int) string {
	return fmt.Sprintf("%02d", i)
}
