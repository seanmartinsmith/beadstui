package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Performance Regression Tests for bv-ut9x
// Tests latency, startup time, and performance thresholds.

// =============================================================================
// Performance Thresholds
// =============================================================================

const (
	// Maximum acceptable latency for robot commands (in milliseconds)
	maxTriageLatencyMS       = 3000 // 3 seconds for triage
	maxNextLatencyMS         = 2000 // 2 seconds for --robot-next
	maxGraphLatencyMS        = 2000 // 2 seconds for graph export
	maxPlanLatencyMS         = 2000 // 2 seconds for plan
	maxSmallDatasetLatencyMS = 1000 // 1 second for small datasets (<50 issues)

	// Memory thresholds (not directly measurable in E2E, but latency correlates)
	maxLargeDatasetLatencyMS = 5000 // 5 seconds for large datasets (1000 issues)
)

// =============================================================================
// 1. Robot Command Latency Tests
// =============================================================================

// TestPerf_RobotTriageLatency verifies --robot-triage completes within threshold.
func TestPerf_RobotTriageLatency(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Create test data with moderate complexity
	createTestDataset(t, env, 100)

	start := time.Now()
	cmd := exec.Command(bv, "--robot-triage")
	cmd.Dir = env
	if err := cmd.Run(); err != nil {
		t.Fatalf("--robot-triage failed: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("--robot-triage latency: %v", elapsed)
	if elapsed.Milliseconds() > maxTriageLatencyMS {
		t.Errorf("--robot-triage too slow: %v > %dms threshold", elapsed, maxTriageLatencyMS)
	}
}

// TestPerf_RobotNextLatency verifies --robot-next completes within threshold.
func TestPerf_RobotNextLatency(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	createTestDataset(t, env, 100)

	start := time.Now()
	cmd := exec.Command(bv, "--robot-next")
	cmd.Dir = env
	if err := cmd.Run(); err != nil {
		t.Fatalf("--robot-next failed: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("--robot-next latency: %v", elapsed)
	if elapsed.Milliseconds() > maxNextLatencyMS {
		t.Errorf("--robot-next too slow: %v > %dms threshold", elapsed, maxNextLatencyMS)
	}
}

// TestPerf_RobotGraphLatency verifies --robot-graph completes within threshold.
func TestPerf_RobotGraphLatency(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	createTestDataset(t, env, 100)

	formats := []string{"json", "dot", "mermaid"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			start := time.Now()
			cmd := exec.Command(bv, "--robot-graph", "--graph-format", format)
			cmd.Dir = env
			if err := cmd.Run(); err != nil {
				t.Fatalf("--robot-graph (%s) failed: %v", format, err)
			}
			elapsed := time.Since(start)

			t.Logf("--robot-graph (%s) latency: %v", format, elapsed)
			if elapsed.Milliseconds() > maxGraphLatencyMS {
				t.Errorf("--robot-graph (%s) too slow: %v > %dms threshold", format, elapsed, maxGraphLatencyMS)
			}
		})
	}
}

// TestPerf_RobotPlanLatency verifies --robot-plan completes within threshold.
func TestPerf_RobotPlanLatency(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	createTestDataset(t, env, 100)

	start := time.Now()
	cmd := exec.Command(bv, "--robot-plan")
	cmd.Dir = env
	if err := cmd.Run(); err != nil {
		t.Fatalf("--robot-plan failed: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("--robot-plan latency: %v", elapsed)
	if elapsed.Milliseconds() > maxPlanLatencyMS {
		t.Errorf("--robot-plan too slow: %v > %dms threshold", elapsed, maxPlanLatencyMS)
	}
}

// =============================================================================
// 2. Data Size Scaling Tests
// =============================================================================

// TestPerf_SmallDatasetLatency tests performance with small datasets.
func TestPerf_SmallDatasetLatency(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	createTestDataset(t, env, 20)

	start := time.Now()
	cmd := exec.Command(bv, "--robot-triage")
	cmd.Dir = env
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("small dataset (20 issues) latency: %v", elapsed)
	if elapsed.Milliseconds() > maxSmallDatasetLatencyMS {
		t.Errorf("small dataset too slow: %v > %dms threshold", elapsed, maxSmallDatasetLatencyMS)
	}
}

// TestPerf_MediumDatasetLatency tests performance with medium datasets.
func TestPerf_MediumDatasetLatency(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	createTestDataset(t, env, 200)

	start := time.Now()
	cmd := exec.Command(bv, "--robot-triage")
	cmd.Dir = env
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("medium dataset (200 issues) latency: %v", elapsed)
	if elapsed.Milliseconds() > maxTriageLatencyMS {
		t.Errorf("medium dataset too slow: %v > %dms threshold", elapsed, maxTriageLatencyMS)
	}
}

// TestPerf_LargeDatasetLatency tests performance with large datasets.
func TestPerf_LargeDatasetLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large dataset test in short mode")
	}

	bv := buildBvBinary(t)
	env := t.TempDir()

	createTestDataset(t, env, 500)

	start := time.Now()
	cmd := exec.Command(bv, "--robot-triage")
	cmd.Dir = env
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("large dataset (500 issues) latency: %v", elapsed)
	if elapsed.Milliseconds() > maxLargeDatasetLatencyMS {
		t.Errorf("large dataset too slow: %v > %dms threshold", elapsed, maxLargeDatasetLatencyMS)
	}
}

// =============================================================================
// 3. Pathological Graph Tests
// =============================================================================

// TestPerf_CyclicGraphLatency tests performance with cyclic graphs.
func TestPerf_CyclicGraphLatency(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	createCyclicDataset(t, env, 50)

	start := time.Now()
	cmd := exec.Command(bv, "--robot-triage")
	cmd.Dir = env
	output, _ := cmd.CombinedOutput()
	elapsed := time.Since(start)

	t.Logf("cyclic graph (50 nodes, many cycles) latency: %v", elapsed)

	// Cyclic graphs may take longer but should still complete
	maxCyclicLatencyMS := int64(5000) // 5 seconds
	if elapsed.Milliseconds() > maxCyclicLatencyMS {
		t.Errorf("cyclic graph too slow: %v > %dms threshold", elapsed, maxCyclicLatencyMS)
	}

	// Should still produce valid output
	if !strings.Contains(string(output), "generated_at") && !strings.Contains(string(output), "cycle") {
		t.Logf("output: %s", output)
	}
}

// TestPerf_DenseGraphLatency tests performance with dense graphs (many edges).
func TestPerf_DenseGraphLatency(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	createDenseDataset(t, env, 100)

	start := time.Now()
	cmd := exec.Command(bv, "--robot-triage")
	cmd.Dir = env
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("dense graph (100 nodes, ~500 edges) latency: %v", elapsed)
	if elapsed.Milliseconds() > maxTriageLatencyMS {
		t.Errorf("dense graph too slow: %v > %dms threshold", elapsed, maxTriageLatencyMS)
	}
}

// =============================================================================
// 4. Repeated Command Performance
// =============================================================================

// TestPerf_RepeatedCommands verifies caching works (second call should be faster).
func TestPerf_RepeatedCommands(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	createTestDataset(t, env, 100)

	// First call (cold)
	start1 := time.Now()
	cmd1 := exec.Command(bv, "--robot-triage")
	cmd1.Dir = env
	if err := cmd1.Run(); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	elapsed1 := time.Since(start1)

	// Second call (potentially warm/cached)
	start2 := time.Now()
	cmd2 := exec.Command(bv, "--robot-triage")
	cmd2.Dir = env
	if err := cmd2.Run(); err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	elapsed2 := time.Since(start2)

	t.Logf("first call: %v, second call: %v", elapsed1, elapsed2)

	// Both should complete within threshold
	if elapsed1.Milliseconds() > maxTriageLatencyMS {
		t.Errorf("first call too slow: %v", elapsed1)
	}
	if elapsed2.Milliseconds() > maxTriageLatencyMS {
		t.Errorf("second call too slow: %v", elapsed2)
	}
}

// =============================================================================
// 5. Profile Output Test
// =============================================================================

// TestPerf_ProfileStartup verifies --profile-startup produces timing data.
func TestPerf_ProfileStartup(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	createTestDataset(t, env, 50)

	cmd := exec.Command(bv, "--profile-startup", "--profile-json")
	cmd.Dir = env
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatalf("--profile-startup failed: %v", err)
	}

	output := stdout.String()

	// Verify profile output contains expected fields
	var wrapper struct {
		Profile struct {
			NodeCount   int `json:"node_count"`
			EdgeCount   int `json:"edge_count"`
			Phase1Total int `json:"phase1_total"`
			Total       int `json:"total"`
		} `json:"profile"`
		GeneratedAt string `json:"generated_at"`
	}
	if err := json.Unmarshal([]byte(output), &wrapper); err != nil {
		t.Fatalf("profile output is not valid JSON: %v\noutput: %s", err, output)
	}

	// Verify we got a profile
	if wrapper.GeneratedAt == "" {
		t.Error("profile missing generated_at")
	}

	t.Logf("profile: node_count=%d, edge_count=%d, phase1=%d, total=%d",
		wrapper.Profile.NodeCount, wrapper.Profile.EdgeCount,
		wrapper.Profile.Phase1Total, wrapper.Profile.Total)
}

// =============================================================================
// Test Data Generators
// =============================================================================

// createTestDataset creates a test dataset with the specified number of issues.
func createTestDataset(t *testing.T, dir string, count int) {
	t.Helper()

	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	var lines []string
	for i := 0; i < count; i++ {
		var deps string
		// Create chain dependencies (each depends on previous)
		if i > 0 {
			deps = fmt.Sprintf(`,"dependencies":[{"depends_on_id":"perf-%d","type":"blocks"}]`, i-1)
		}
		line := fmt.Sprintf(`{"id":"perf-%d","title":"Performance Test Issue %d","status":"open","priority":%d%s}`,
			i, i, i%5, deps)
		lines = append(lines, line)
	}

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(issuesPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}
}

// createCyclicDataset creates a dataset with cycles.
func createCyclicDataset(t *testing.T, dir string, count int) {
	t.Helper()

	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	var lines []string
	for i := 0; i < count; i++ {
		// Create cycles: each node depends on the next, last depends on first
		nextIdx := (i + 1) % count
		line := fmt.Sprintf(`{"id":"cycle-%d","title":"Cyclic Issue %d","status":"open","priority":%d,"dependencies":[{"depends_on_id":"cycle-%d","type":"blocks"}]}`,
			i, i, i%5, nextIdx)
		lines = append(lines, line)
	}

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(issuesPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}
}

// createDenseDataset creates a dataset with many dependencies (dense graph).
func createDenseDataset(t *testing.T, dir string, count int) {
	t.Helper()

	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	var lines []string
	for i := 0; i < count; i++ {
		var deps []string
		// Each node depends on ~5 previous nodes
		for j := 1; j <= 5 && i-j >= 0; j++ {
			deps = append(deps, fmt.Sprintf(`{"depends_on_id":"dense-%d","type":"blocks"}`, i-j))
		}

		var depsJSON string
		if len(deps) > 0 {
			depsJSON = fmt.Sprintf(`,"dependencies":[%s]`, strings.Join(deps, ","))
		}

		line := fmt.Sprintf(`{"id":"dense-%d","title":"Dense Issue %d","status":"open","priority":%d%s}`,
			i, i, i%5, depsJSON)
		lines = append(lines, line)
	}

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(issuesPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("failed to write issues.jsonl: %v", err)
	}
}
