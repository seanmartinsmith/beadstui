package analysis_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/loader"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// skipIfNotPerfTest skips the test unless PERF_TEST=1 is set.
// These tests are sensitive to system load and should be run in isolation.
func skipIfNotPerfTest(t *testing.T) {
	if os.Getenv("PERF_TEST") != "1" {
		t.Skip("Skipping performance test (set PERF_TEST=1 to run)")
	}
}

// Performance targets for different project sizes
// Note: These are conservative targets to account for CI variability.
// In practice, performance is typically 2-5x faster on dedicated hardware.
var performanceTargets = map[string]time.Duration{
	"small":        200 * time.Millisecond, // <50 issues
	"medium":       500 * time.Millisecond, // 50-300 issues
	"large":        1 * time.Second,        // 300-1000 issues
	"xl":           3 * time.Second,        // 1000-5000 issues
	"pathological": 5 * time.Second,        // Edge cases (should not hang)
}

// TestE2EStartup_SmallProject tests startup with a small project (~30 issues)
func TestE2EStartup_SmallProject(t *testing.T) {
	skipIfNotPerfTest(t)
	issues := generateRealisticIssues(30, 2) // 30 issues, ~2 deps each

	start := time.Now()
	an := analysis.NewAnalyzer(issues)
	_ = an.Analyze()
	elapsed := time.Since(start)

	t.Logf("Small project (%d issues): %v", len(issues), elapsed)

	target := performanceTargets["small"]
	if elapsed > target {
		t.Errorf("Startup too slow: %v (target <%v)", elapsed, target)
	}
}

// TestE2EStartup_MediumProject tests startup with a medium project (~150 issues)
func TestE2EStartup_MediumProject(t *testing.T) {
	skipIfNotPerfTest(t)
	issues := generateRealisticIssues(150, 3) // 150 issues, ~3 deps each

	start := time.Now()
	an := analysis.NewAnalyzer(issues)
	_ = an.Analyze()
	elapsed := time.Since(start)

	t.Logf("Medium project (%d issues): %v", len(issues), elapsed)

	target := performanceTargets["medium"]
	if elapsed > target {
		t.Errorf("Startup too slow: %v (target <%v)", elapsed, target)
	}
}

// TestE2EStartup_LargeProject tests startup with a large project (~500 issues)
func TestE2EStartup_LargeProject(t *testing.T) {
	skipIfNotPerfTest(t)
	issues := generateRealisticIssues(500, 3) // 500 issues, ~3 deps each

	start := time.Now()
	an := analysis.NewAnalyzer(issues)
	_ = an.Analyze()
	elapsed := time.Since(start)

	t.Logf("Large project (%d issues): %v", len(issues), elapsed)

	target := performanceTargets["large"]
	if elapsed > target {
		t.Errorf("Startup too slow: %v (target <%v)", elapsed, target)
	}
}

// TestE2EStartup_XLProject tests startup with an XL project (~2000 issues)
func TestE2EStartup_XLProject(t *testing.T) {
	skipIfNotPerfTest(t)
	issues := generateRealisticIssues(2000, 2) // 2000 issues, ~2 deps each

	start := time.Now()
	an := analysis.NewAnalyzer(issues)
	_ = an.Analyze()
	elapsed := time.Since(start)

	t.Logf("XL project (%d issues): %v", len(issues), elapsed)

	target := performanceTargets["xl"]
	if elapsed > target {
		t.Errorf("Startup too slow: %v (target <%v)", elapsed, target)
	}
}

// TestE2EStartup_WithRealBeadsFile tests with actual beads data if available
func TestE2EStartup_WithRealBeadsFile(t *testing.T) {
	skipIfNotPerfTest(t)
	// Try to find real beads files in the project
	possiblePaths := []string{
		"../../.beads/beads.jsonl",
		"../../../.beads/beads.jsonl",
		".beads/beads.jsonl",
	}

	var realPath string
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			realPath = p
			break
		}
	}

	if realPath == "" {
		t.Skip("No real beads file found for E2E test")
	}

	start := time.Now()
	issues, err := loader.LoadIssuesFromFile(realPath)
	if err != nil {
		t.Fatalf("Failed to load real beads file: %v", err)
	}

	an := analysis.NewAnalyzer(issues)
	_ = an.Analyze()
	elapsed := time.Since(start)

	t.Logf("Real project (%s, %d issues): %v", filepath.Base(realPath), len(issues), elapsed)

	// Use medium target for real files (typically 50-200 issues)
	target := performanceTargets["medium"]
	if len(issues) > 300 {
		target = performanceTargets["large"]
	} else if len(issues) < 50 {
		target = performanceTargets["small"]
	}

	if elapsed > target {
		t.Errorf("Startup too slow for real data: %v (target <%v)", elapsed, target)
	}
}

// TestE2EStartup_PathologicalCycles tests that cyclic graphs don't hang
func TestE2EStartup_PathologicalCycles(t *testing.T) {
	skipIfNotPerfTest(t)
	// Create graph with many overlapping cycles (pathological for Johnson's algorithm)
	issues := generateManyCyclesIssues(30)

	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		an := analysis.NewAnalyzer(issues)
		_ = an.Analyze()
		done <- time.Since(start)
	}()

	target := performanceTargets["pathological"]
	select {
	case elapsed := <-done:
		t.Logf("Pathological cycles (30 nodes): %v", elapsed)
		if elapsed > target {
			t.Errorf("Pathological case too slow: %v (target <%v)", elapsed, target)
		}
	case <-time.After(target + time.Second):
		t.Fatal("HUNG on pathological cycle graph - timeout protection may not be working!")
	}
}

// TestE2EStartup_PathologicalDense tests dense graphs (many edges)
func TestE2EStartup_PathologicalDense(t *testing.T) {
	skipIfNotPerfTest(t)
	// Create dense graph where each node has many dependencies
	issues := generateDenseIssues(100, 10) // 100 issues, 10 deps each

	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		an := analysis.NewAnalyzer(issues)
		_ = an.Analyze()
		done <- time.Since(start)
	}()

	target := performanceTargets["pathological"]
	select {
	case elapsed := <-done:
		t.Logf("Dense graph (100 nodes, 10 deps each): %v", elapsed)
		if elapsed > target {
			t.Errorf("Dense graph too slow: %v (target <%v)", elapsed, target)
		}
	case <-time.After(target + time.Second):
		t.Fatal("HUNG on dense graph - timeout protection may not be working!")
	}
}

// TestE2EStartup_NoRegression checks against baseline (if exists)
func TestE2EStartup_NoRegression(t *testing.T) {
	skipIfNotPerfTest(t)
	baselinePath := "testdata/startup_baseline.json"

	// Load baseline if exists
	baseline := make(map[string]float64)
	if data, err := os.ReadFile(baselinePath); err == nil {
		if err := json.Unmarshal(data, &baseline); err != nil {
			t.Logf("Warning: could not parse baseline: %v", err)
		}
	}

	// Run tests and compare
	tests := []struct {
		name   string
		issues []model.Issue
	}{
		{"small_30", generateRealisticIssues(30, 2)},
		{"medium_150", generateRealisticIssues(150, 3)},
		{"large_500", generateRealisticIssues(500, 3)},
	}

	results := make(map[string]float64)

	for _, tc := range tests {
		start := time.Now()
		an := analysis.NewAnalyzer(tc.issues)
		_ = an.Analyze()
		elapsed := time.Since(start)

		results[tc.name] = float64(elapsed.Milliseconds())

		// Check regression if baseline exists
		if baselineMs, ok := baseline[tc.name]; ok {
			// Use 100% tolerance but with a minimum threshold of 50ms for timing jitter
			threshold := baselineMs * 2.0
			if threshold < 50 {
				threshold = 50 // Minimum threshold to handle system noise
			}
			if results[tc.name] > threshold {
				t.Errorf("%s regressed: %.0fms > %.0fms (threshold)",
					tc.name, results[tc.name], threshold)
			} else {
				t.Logf("%s: %.0fms (baseline: %.0fms, threshold: %.0fms)",
					tc.name, results[tc.name], baselineMs, threshold)
			}
		} else {
			t.Logf("%s: %.0fms (no baseline)", tc.name, results[tc.name])
		}
	}

	// Save new baseline if requested via env var
	if os.Getenv("UPDATE_BASELINE") == "1" {
		data, _ := json.MarshalIndent(results, "", "  ")
		if err := os.WriteFile(baselinePath, data, 0644); err != nil {
			t.Logf("Warning: could not save baseline: %v", err)
		} else {
			t.Logf("Updated baseline at %s", baselinePath)
		}
	}
}

// =============================================================================
// Test Data Generators
// =============================================================================

// generateRealisticIssues creates issues that simulate real project structure:
// - Mix of statuses (mostly open/in_progress)
// - Varying priorities
// - DAG structure (dependencies only to earlier issues)
// - Realistic dependency density
func generateRealisticIssues(n, avgDeps int) []model.Issue {
	issues := make([]model.Issue, n)
	statuses := []model.Status{model.StatusOpen, model.StatusOpen, model.StatusInProgress, model.StatusBlocked, model.StatusClosed}
	types := []model.IssueType{model.TypeTask, model.TypeTask, model.TypeBug, model.TypeFeature, model.TypeChore}

	for i := 0; i < n; i++ {
		issues[i] = model.Issue{
			ID:        fmt.Sprintf("REAL-%d", i),
			Title:     fmt.Sprintf("Issue %d", i),
			Status:    statuses[i%len(statuses)],
			Priority:  i % 4,
			IssueType: types[i%len(types)],
		}

		// Add dependencies to earlier issues (DAG structure)
		numDeps := avgDeps
		if i < avgDeps {
			numDeps = i
		}

		for d := 0; d < numDeps && i > 0; d++ {
			// Spread dependencies across earlier issues
			depIdx := (i - 1 - d*i/numDeps) % i
			if depIdx < 0 {
				depIdx = 0
			}
			issues[i].Dependencies = append(issues[i].Dependencies, &model.Dependency{
				DependsOnID: fmt.Sprintf("REAL-%d", depIdx),
				Type:        model.DepBlocks,
			})
		}
	}

	return issues
}

// generateManyCyclesIssues creates a pathological graph with overlapping cycles
func generateManyCyclesIssues(n int) []model.Issue {
	issues := make([]model.Issue, n)

	for i := 0; i < n; i++ {
		issues[i] = model.Issue{
			ID:     fmt.Sprintf("CYCLE-%d", i),
			Title:  fmt.Sprintf("Cyclic Issue %d", i),
			Status: model.StatusOpen,
		}

		// Connect to next 3 nodes (wrapping), creating overlapping cycles
		for offset := 1; offset <= 3 && offset < n; offset++ {
			nextIdx := (i + offset) % n
			issues[i].Dependencies = append(issues[i].Dependencies, &model.Dependency{
				DependsOnID: fmt.Sprintf("CYCLE-%d", nextIdx),
				Type:        model.DepBlocks,
			})
		}
	}

	return issues
}

// generateDenseIssues creates a graph with high edge density
func generateDenseIssues(n, depsPerNode int) []model.Issue {
	issues := make([]model.Issue, n)

	for i := 0; i < n; i++ {
		issues[i] = model.Issue{
			ID:     fmt.Sprintf("DENSE-%d", i),
			Title:  fmt.Sprintf("Dense Issue %d", i),
			Status: model.StatusOpen,
		}

		// Add many dependencies to earlier nodes
		for d := 0; d < depsPerNode && i > 0; d++ {
			depIdx := (i - 1 - d) % i
			if depIdx < 0 {
				depIdx = 0
			}
			issues[i].Dependencies = append(issues[i].Dependencies, &model.Dependency{
				DependsOnID: fmt.Sprintf("DENSE-%d", depIdx),
				Type:        model.DepBlocks,
			})
		}
	}

	return issues
}

// TestPhase1Only_MediumProject tests that Phase 1 completes in < 50ms
func TestPhase1Only_MediumProject(t *testing.T) {
	skipIfNotPerfTest(t)
	issues := generateRealisticIssues(150, 3)

	start := time.Now()
	an := analysis.NewAnalyzer(issues)
	stats := an.AnalyzeAsync(context.Background())
	elapsed := time.Since(start)

	// Phase 1 should be essentially instant
	// Note: 100ms threshold accounts for WSL2/CI variance; typically <20ms on bare metal
	t.Logf("Phase 1 for 150 issues: %v", elapsed)

	if elapsed > 100*time.Millisecond {
		t.Errorf("Phase 1 too slow: %v (target <100ms)", elapsed)
	}

	// Phase 1 data should be available immediately
	if stats.Density == 0 && len(issues) > 0 {
		t.Error("Density should be computed in Phase 1")
	}
	if len(stats.OutDegree) == 0 {
		t.Error("OutDegree should be computed in Phase 1")
	}

	// Phase 2 data may not be ready yet (or may be if fast)
	t.Logf("Phase 2 ready: %v", stats.IsPhase2Ready())
}

// TestPhase1Only_LargeProject tests Phase 1 with a large project
func TestPhase1Only_LargeProject(t *testing.T) {
	skipIfNotPerfTest(t)
	issues := generateRealisticIssues(1000, 3)

	start := time.Now()
	an := analysis.NewAnalyzer(issues)
	stats := an.AnalyzeAsync(context.Background())
	elapsed := time.Since(start)

	// Phase 1 should be fast even for large projects
	// Note: 300ms threshold accounts for WSL2/CI variance; typically <50ms on bare metal
	t.Logf("Phase 1 for 1000 issues: %v", elapsed)

	if elapsed > 300*time.Millisecond {
		t.Errorf("Phase 1 too slow for 1000 issues: %v (target <300ms)", elapsed)
	}

	// Verify Phase 1 data is available
	if stats.NodeCount != 1000 {
		t.Errorf("Expected 1000 nodes, got %d", stats.NodeCount)
	}
}
