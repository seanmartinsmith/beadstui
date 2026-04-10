package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// captureStdout runs f while capturing stdout to a string.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	_ = w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	return buf.String()
}

func TestFormatDurationAndSizeTier(t *testing.T) {
	if got := formatDuration(500 * time.Microsecond); got != "  0.50ms" {
		t.Fatalf("formatDuration micro expected 0.50ms got %s", got)
	}
	if got := formatDuration(42 * time.Millisecond); got != "    42ms" {
		t.Fatalf("formatDuration ms expected 42ms got %s", got)
	}
	if tier := getSizeTier(50); tier != "Small (<100 issues)" {
		t.Fatalf("size tier small wrong: %s", tier)
	}
	if tier := getSizeTier(150); tier != "Medium (100-500 issues)" {
		t.Fatalf("size tier medium wrong: %s", tier)
	}
	if tier := getSizeTier(800); tier != "Large (500-2000 issues)" {
		t.Fatalf("size tier large wrong: %s", tier)
	}
	if tier := getSizeTier(5000); tier != "XL (>2000 issues)" {
		t.Fatalf("size tier XL wrong: %s", tier)
	}
}

func TestGenerateProfileRecommendations(t *testing.T) {
	cfg := analysis.FullAnalysisConfig()
	profile := &analysis.StartupProfile{
		NodeCount:  600,
		Config:     cfg,
		PageRankTO: true,
	}
	recs := generateProfileRecommendations(profile, 200*time.Millisecond, 600*time.Millisecond)
	if len(recs) == 0 {
		t.Fatalf("expected recommendations")
	}
	foundStartup := false
	foundPR := false
	for _, r := range recs {
		if strings.Contains(r, "Startup") {
			foundStartup = true
		}
		if strings.Contains(r, "PageRank timed out") {
			foundPR = true
		}
	}
	if !foundStartup || !foundPR {
		t.Fatalf("missing expected recommendations: %+v", recs)
	}
}

func TestPrintMetricAndCyclesLines(t *testing.T) {
	out := captureStdout(t, func() {
		printMetricLine("PR", 10*time.Millisecond, true, true)
		printMetricLine("Skip", 0, false, false)
		printCyclesLine(&analysis.StartupProfile{
			Config:     analysis.FullAnalysisConfig(),
			Cycles:     5 * time.Millisecond,
			CycleCount: 2,
		})
	})
	if !strings.Contains(out, "PR") || !strings.Contains(out, "Skip") || !strings.Contains(out, "Cycles:") {
		t.Fatalf("printMetric/Cycles output missing content: %s", out)
	}
}

func TestPrintCyclesLineSkipped(t *testing.T) {
	profile := &analysis.StartupProfile{Config: analysis.AnalysisConfig{ComputeCycles: false}}
	out := captureStdout(t, func() {
		printCyclesLine(profile)
	})
	if !strings.Contains(out, "[Skipped]") {
		t.Fatalf("expected skipped cycles line, got %q", out)
	}
}

func TestPrintDiffSummaryAndRepeatChar(t *testing.T) {
	diff := &analysis.SnapshotDiff{
		Summary: analysis.DiffSummary{
			HealthTrend:      "improving",
			IssuesAdded:      1,
			IssuesClosed:     1,
			IssuesRemoved:    1,
			IssuesReopened:   1,
			IssuesModified:   1,
			CyclesIntroduced: 1,
			CyclesResolved:   1,
		},
		NewIssues:      []model.Issue{{ID: "A", Title: "New", Priority: 1}},
		ClosedIssues:   []model.Issue{{ID: "B", Title: "Closed"}},
		ReopenedIssues: []model.Issue{{ID: "C", Title: "Reopened"}},
		ModifiedIssues: []analysis.ModifiedIssue{{IssueID: "D", Title: "Mod", Changes: []analysis.FieldChange{{Field: "status", OldValue: "open", NewValue: "closed"}}}},
		NewCycles:      [][]string{{"X", "Y"}},
		MetricDeltas:   analysis.MetricDeltas{TotalIssues: 1, OpenIssues: -1, BlockedIssues: 2, CycleCount: 1},
	}
	out := captureStdout(t, func() {
		printDiffSummary(diff, "HEAD~1")
	})
	for _, snippet := range []string{"Changes since HEAD~1", "Health Trend", "New Issues", "Cycles"} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("printDiffSummary missing %s in output:\n%s", snippet, out)
		}
	}
	if rep := repeatChar('-', 5); rep != "-----" {
		t.Fatalf("repeatChar expected 5 dashes, got %q", rep)
	}
}

func TestPrintProfileReport(t *testing.T) {
	cfg := analysis.FullAnalysisConfig()
	profile := &analysis.StartupProfile{
		NodeCount:    2,
		EdgeCount:    1,
		Density:      0.5,
		BuildGraph:   1 * time.Millisecond,
		Degree:       1 * time.Millisecond,
		TopoSort:     1 * time.Millisecond,
		Phase1:       1 * time.Millisecond,
		PageRank:     1 * time.Millisecond,
		Betweenness:  1 * time.Millisecond,
		Eigenvector:  1 * time.Millisecond,
		HITS:         1 * time.Millisecond,
		CriticalPath: 1 * time.Millisecond,
		Cycles:       1 * time.Millisecond,
		Phase2:       3 * time.Millisecond,
		Total:        5 * time.Millisecond,
		Config:       cfg,
	}
	out := captureStdout(t, func() {
		printProfileReport(profile, 2*time.Millisecond, 7*time.Millisecond)
	})
	if !strings.Contains(out, "Startup Profile") || !strings.Contains(out, "PageRank") {
		t.Fatalf("printProfileReport missing expected text")
	}
}

func TestBuildMetricItems(t *testing.T) {
	items := buildMetricItems(map[string]float64{"A": 3, "B": 5, "C": 1}, 2)
	if len(items) != 2 {
		t.Fatalf("expected top 2 items, got %d", len(items))
	}
	if items[0].ID != "B" || items[1].ID != "A" {
		t.Fatalf("items not sorted desc: %+v", items)
	}
	if buildMetricItems(nil, 3) != nil {
		t.Fatalf("nil metrics should return nil")
	}
}

func TestRepeatChar(t *testing.T) {
	if got := repeatChar('x', 4); got != "xxxx" {
		t.Fatalf("repeatChar mismatch: %q", got)
	}
}

func TestPrintDiffSummary(t *testing.T) {
	diff := &analysis.SnapshotDiff{
		Summary: analysis.DiffSummary{
			IssuesAdded:      1,
			IssuesClosed:     2,
			IssuesReopened:   1,
			IssuesModified:   3,
			CyclesIntroduced: 1,
			CyclesResolved:   0,
			HealthTrend:      "improving",
		},
		NewIssues:      []model.Issue{{ID: "N1", Title: "New"}},
		ClosedIssues:   []model.Issue{{ID: "C1", Title: "Closed"}},
		ModifiedIssues: []analysis.ModifiedIssue{{IssueID: "M1", Title: "Mod"}},
	}
	out := captureStdout(t, func() {
		printDiffSummary(diff, "HEAD~2")
	})
	for _, s := range []string{"Changes since HEAD~2", "+ 1 new issues", "Closed Issues", "~ 3 issues modified"} {
		if !strings.Contains(out, s) {
			t.Fatalf("printDiffSummary missing %q: %s", s, out)
		}
	}
}

func TestRunProfileStartupJSON(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
	}
	out := captureStdout(t, func() {
		runProfileStartup(issues, 5*time.Millisecond, true, false)
	})
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("runProfileStartup JSON unmarshal failed: %v\n%s", err, out)
	}
	if payload["profile"] == nil {
		t.Fatalf("expected profile field in output")
	}
}
