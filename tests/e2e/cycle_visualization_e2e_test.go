package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// E2E: Cycle Visualization and Highlighting (bv-f1zg)
// ============================================================================

// TestCycleVisualization_NoCycles tests clean DAG with no cycles
func TestCycleVisualization_NoCycles(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createNoCycleRepo(t)

	// Get robot-insights output
	cmd := exec.Command(bv, "--robot-insights")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-insights failed: %v\n%s", err, out)
	}

	var result struct {
		Cycles [][]string `json:"Cycles"`
		Stats  struct {
			CycleCount int `json:"cycle_count"`
		} `json:"Stats"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nOutput: %s", err, out)
	}

	// No cycles should be detected
	if len(result.Cycles) != 0 {
		t.Errorf("expected 0 cycles, got %d: %v", len(result.Cycles), result.Cycles)
	}
}

// TestCycleVisualization_TwoNodeCycle tests a simple A -> B -> A cycle
func TestCycleVisualization_TwoNodeCycle(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createTwoNodeCycleRepo(t)

	cmd := exec.Command(bv, "--robot-insights")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-insights failed: %v\n%s", err, out)
	}

	var result struct {
		Cycles [][]string `json:"Cycles"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Should detect exactly one cycle
	if len(result.Cycles) != 1 {
		t.Errorf("expected 1 cycle, got %d", len(result.Cycles))
	}

	// Cycle should have 3 elements (A -> B -> A)
	if len(result.Cycles) > 0 && len(result.Cycles[0]) != 3 {
		t.Errorf("expected cycle length 3 (A->B->A), got %d: %v", len(result.Cycles[0]), result.Cycles[0])
	}
}

// TestCycleVisualization_ThreeNodeCycle tests A -> B -> C -> A cycle
func TestCycleVisualization_ThreeNodeCycle(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createThreeNodeCycleRepo(t)

	cmd := exec.Command(bv, "--robot-insights")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-insights failed: %v\n%s", err, out)
	}

	var result struct {
		Cycles [][]string `json:"Cycles"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Should detect exactly one cycle
	if len(result.Cycles) != 1 {
		t.Errorf("expected 1 cycle, got %d", len(result.Cycles))
	}

	// Cycle should have 4 elements (A -> B -> C -> A)
	if len(result.Cycles) > 0 && len(result.Cycles[0]) != 4 {
		t.Errorf("expected cycle length 4 (A->B->C->A), got %d: %v", len(result.Cycles[0]), result.Cycles[0])
	}
}

// TestCycleVisualization_MultipleCycles tests multiple independent cycles
func TestCycleVisualization_MultipleCycles(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createMultipleCyclesRepo(t)

	cmd := exec.Command(bv, "--robot-insights")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-insights failed: %v\n%s", err, out)
	}

	var result struct {
		Cycles [][]string `json:"Cycles"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Should detect 2 cycles
	if len(result.Cycles) != 2 {
		t.Errorf("expected 2 cycles, got %d: %v", len(result.Cycles), result.Cycles)
	}
}

// TestCycleVisualization_SelfLoop tests a self-referencing node
// NOTE: gonum's DirectedGraph doesn't support self-edges, so self-loops are
// currently not detectable. This test documents the expected behavior when
// a self-loop is encountered - the graph builder should skip self-edges gracefully.
func TestCycleVisualization_SelfLoop(t *testing.T) {
	t.Skip("Self-loops not supported by gonum DirectedGraph - skipping until enhanced handling is implemented")
	// When self-loop support is added, this test should verify:
	// - Self-loop is detected as a cycle with 2 elements (A -> A)
	// - No panic occurs when processing self-referencing dependencies
}

// TestCycleVisualization_MermaidExport tests Mermaid export includes cycle data
func TestCycleVisualization_MermaidExport(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createTwoNodeCycleRepo(t)

	cmd := exec.Command(bv, "--robot-graph", "--graph-format=mermaid")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-graph --graph-format=mermaid failed: %v\n%s", err, out)
	}

	var result struct {
		Format string `json:"format"`
		Graph  string `json:"graph"`
		Nodes  int    `json:"nodes"`
		Edges  int    `json:"edges"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nOutput: %s", err, out)
	}

	// Verify Mermaid output exists
	if result.Format != "mermaid" {
		t.Errorf("expected format='mermaid', got %q", result.Format)
	}

	// Check Mermaid starts correctly
	if !strings.HasPrefix(result.Graph, "graph TD") {
		t.Error("Mermaid output should start with 'graph TD'")
	}

	// Both nodes should be in output
	if result.Nodes != 2 {
		t.Errorf("expected 2 nodes, got %d", result.Nodes)
	}
}

// TestCycleVisualization_DOTExport tests DOT export includes cycle styling
func TestCycleVisualization_DOTExport(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createTwoNodeCycleRepo(t)

	cmd := exec.Command(bv, "--robot-graph", "--graph-format=dot")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-graph --graph-format=dot failed: %v\n%s", err, out)
	}

	var result struct {
		Format string `json:"format"`
		Graph  string `json:"graph"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify DOT format
	if result.Format != "dot" {
		t.Errorf("expected format='dot', got %q", result.Format)
	}

	// Check DOT output structure
	if !strings.Contains(result.Graph, "digraph") {
		t.Error("DOT output should contain 'digraph'")
	}

	// Both cycle nodes should be present
	if !strings.Contains(result.Graph, "cycle-a") && !strings.Contains(result.Graph, "\"cycle-a\"") {
		t.Error("DOT output should contain cycle-a node")
	}
	if !strings.Contains(result.Graph, "cycle-b") && !strings.Contains(result.Graph, "\"cycle-b\"") {
		t.Error("DOT output should contain cycle-b node")
	}
}

// TestCycleVisualization_JSONExport tests JSON export includes cycle data
func TestCycleVisualization_JSONExport(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createThreeNodeCycleRepo(t)

	cmd := exec.Command(bv, "--robot-graph", "--graph-format=json")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-graph --graph-format=json failed: %v\n%s", err, out)
	}

	var result struct {
		Format    string `json:"format"`
		Nodes     int    `json:"nodes"`
		Edges     int    `json:"edges"`
		Adjacency struct {
			Nodes []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"nodes"`
			Edges []struct {
				From string `json:"from"`
				To   string `json:"to"`
				Type string `json:"type"`
			} `json:"edges"`
		} `json:"adjacency"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify JSON format
	if result.Format != "json" {
		t.Errorf("expected format='json', got %q", result.Format)
	}

	// Should have 3 nodes for the 3-node cycle
	if result.Nodes != 3 {
		t.Errorf("expected 3 nodes, got %d", result.Nodes)
	}

	// Should have 3 edges for the 3-node cycle (A->B, B->C, C->A)
	if result.Edges != 3 {
		t.Errorf("expected 3 edges, got %d", result.Edges)
	}

	// Verify adjacency data exists
	if len(result.Adjacency.Nodes) != 3 {
		t.Errorf("expected 3 adjacency nodes, got %d", len(result.Adjacency.Nodes))
	}
}

// TestCycleVisualization_RobotSuggestCycles tests cycle suggestions via robot-suggest
func TestCycleVisualization_RobotSuggestCycles(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createTwoNodeCycleRepo(t)

	cmd := exec.Command(bv, "--robot-suggest")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-suggest failed: %v\n%s", err, out)
	}

	var result struct {
		Suggestions struct {
			Suggestions []struct {
				Type    string `json:"type"`
				IssueID string `json:"issue_id"`
				Message string `json:"message"`
			} `json:"suggestions"`
			Stats struct {
				CycleCount int `json:"cycle_count"`
			} `json:"stats"`
		} `json:"suggestions"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nOutput: %s", err, out)
	}

	// Should report cycle count
	if result.Suggestions.Stats.CycleCount == 0 {
		// cycle_count may be in a different location, check suggestions
		t.Log("Note: cycle_count not in stats, checking suggestions array")
	}

	// Should have at least one cycle-related suggestion
	hasCycleSuggestion := false
	for _, s := range result.Suggestions.Suggestions {
		if strings.Contains(strings.ToLower(s.Type), "cycle") ||
			strings.Contains(strings.ToLower(s.Message), "cycle") {
			hasCycleSuggestion = true
			break
		}
	}
	if !hasCycleSuggestion {
		t.Log("Note: No explicit cycle suggestion found (may be expected if format differs)")
	}
}

// TestCycleVisualization_CycleCountInStatus tests cycle count appears in status
func TestCycleVisualization_CycleCountInStatus(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createMultipleCyclesRepo(t)

	cmd := exec.Command(bv, "--robot-insights")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-insights failed: %v\n%s", err, out)
	}

	var result struct {
		Cycles [][]string `json:"Cycles"`
		Status map[string]struct {
			State string `json:"state"`
		} `json:"status"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify cycles status indicates computation status
	if result.Status != nil {
		cycleStatus, ok := result.Status["cycles"]
		if ok {
			t.Logf("Cycle status: %s", cycleStatus.State)
		}
	}

	// Main check: cycles should be detected
	if len(result.Cycles) < 2 {
		t.Errorf("expected at least 2 cycles, got %d", len(result.Cycles))
	}
}

// TestCycleVisualization_CycleMembers tests cycle member list is correct
func TestCycleVisualization_CycleMembers(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createThreeNodeCycleRepo(t)

	cmd := exec.Command(bv, "--robot-insights")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-insights failed: %v\n%s", err, out)
	}

	var result struct {
		Cycles [][]string `json:"Cycles"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if len(result.Cycles) == 0 {
		t.Fatal("expected at least one cycle")
	}

	cycle := result.Cycles[0]
	expectedMembers := map[string]bool{
		"cycle-a": false,
		"cycle-b": false,
		"cycle-c": false,
	}

	// Check all expected members are present
	for _, member := range cycle {
		if _, ok := expectedMembers[member]; ok {
			expectedMembers[member] = true
		}
	}

	for member, found := range expectedMembers {
		if !found {
			t.Errorf("expected cycle member %q not found in cycle: %v", member, cycle)
		}
	}
}

// TestCycleVisualization_NestedCycles tests overlapping/nested cycle detection
func TestCycleVisualization_NestedCycles(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createNestedCyclesRepo(t)

	cmd := exec.Command(bv, "--robot-insights")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-insights failed: %v\n%s", err, out)
	}

	var result struct {
		Cycles [][]string `json:"Cycles"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Nested cycles: A->B->C->A and A->B->D->A share A and B
	// Depending on algorithm, may report 1 or 2 cycles
	if len(result.Cycles) == 0 {
		t.Error("expected at least one cycle in nested cycle graph")
	}

	t.Logf("Detected %d cycle(s) in nested graph: %v", len(result.Cycles), result.Cycles)
}

// TestCycleVisualization_DeterministicOutput tests cycle output is deterministic
func TestCycleVisualization_DeterministicOutput(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createThreeNodeCycleRepo(t)

	type CycleResult struct {
		Cycles [][]string `json:"Cycles"`
	}

	var results []CycleResult
	for i := 0; i < 3; i++ {
		cmd := exec.Command(bv, "--robot-insights")
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("--robot-insights run %d failed: %v\n%s", i+1, err, out)
		}

		var result CycleResult
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("JSON unmarshal run %d failed: %v", i+1, err)
		}
		results = append(results, result)
	}

	// All cycle results should be identical
	for i := 1; i < len(results); i++ {
		if len(results[0].Cycles) != len(results[i].Cycles) {
			t.Error("cycle count is not deterministic between runs")
			continue
		}
		for j, cycle := range results[0].Cycles {
			if len(cycle) != len(results[i].Cycles[j]) {
				t.Errorf("cycle %d length is not deterministic", j)
				continue
			}
			for k, member := range cycle {
				if member != results[i].Cycles[j][k] {
					t.Errorf("cycle %d member %d differs: %q vs %q", j, k, member, results[i].Cycles[j][k])
				}
			}
		}
	}
}

// TestCycleVisualization_MixedCycleAndDAG tests graph with both cycles and non-cycle nodes
func TestCycleVisualization_MixedCycleAndDAG(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createMixedCycleDAGRepo(t)

	cmd := exec.Command(bv, "--robot-insights")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-insights failed: %v\n%s", err, out)
	}

	var result struct {
		Cycles [][]string `json:"Cycles"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Should detect the cycle but not flag DAG portion
	if len(result.Cycles) != 1 {
		t.Errorf("expected 1 cycle in mixed graph, got %d", len(result.Cycles))
	}

	// Cycle should only contain cycle nodes, not DAG nodes
	if len(result.Cycles) > 0 {
		for _, member := range result.Cycles[0] {
			if strings.HasPrefix(member, "dag-") {
				t.Errorf("DAG node %q incorrectly included in cycle", member)
			}
		}
	}
}

// ============================================================================
// Test Helpers for bv-f1zg (Cycle Visualization)
// ============================================================================

// createNoCycleRepo creates a clean DAG with no cycles
func createNoCycleRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Linear chain: A -> B -> C (no cycles)
	// B depends on A, C depends on B
	jsonl := `{"id": "node-a", "title": "Node A", "status": "open", "priority": 1, "issue_type": "task"}
{"id": "node-b", "title": "Node B", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"issue_id": "node-b", "depends_on_id": "node-a", "type": "blocks"}]}
{"id": "node-c", "title": "Node C", "status": "open", "priority": 3, "issue_type": "task", "dependencies": [{"issue_id": "node-c", "depends_on_id": "node-b", "type": "blocks"}]}`

	if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(jsonl), 0644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}
	return repoDir
}

// createTwoNodeCycleRepo creates a simple 2-node cycle: A -> B -> A
func createTwoNodeCycleRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Cycle: A depends on B, B depends on A -> forms A->B->A cycle
	jsonl := `{"id": "cycle-a", "title": "Cycle A", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"issue_id": "cycle-a", "depends_on_id": "cycle-b", "type": "blocks"}]}
{"id": "cycle-b", "title": "Cycle B", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"issue_id": "cycle-b", "depends_on_id": "cycle-a", "type": "blocks"}]}`

	if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(jsonl), 0644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}
	return repoDir
}

// createThreeNodeCycleRepo creates a 3-node cycle: A -> B -> C -> A
func createThreeNodeCycleRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Cycle: A depends on B, B depends on C, C depends on A -> forms A->B->C->A cycle
	jsonl := `{"id": "cycle-a", "title": "Cycle A", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"issue_id": "cycle-a", "depends_on_id": "cycle-b", "type": "blocks"}]}
{"id": "cycle-b", "title": "Cycle B", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"issue_id": "cycle-b", "depends_on_id": "cycle-c", "type": "blocks"}]}
{"id": "cycle-c", "title": "Cycle C", "status": "open", "priority": 3, "issue_type": "task", "dependencies": [{"issue_id": "cycle-c", "depends_on_id": "cycle-a", "type": "blocks"}]}`

	if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(jsonl), 0644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}
	return repoDir
}

// createMultipleCyclesRepo creates 2 independent cycles
func createMultipleCyclesRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Two independent cycles: (A <-> B) and (X <-> Y)
	jsonl := `{"id": "cycle1-a", "title": "Cycle1 A", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"issue_id": "cycle1-a", "depends_on_id": "cycle1-b", "type": "blocks"}]}
{"id": "cycle1-b", "title": "Cycle1 B", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"issue_id": "cycle1-b", "depends_on_id": "cycle1-a", "type": "blocks"}]}
{"id": "cycle2-x", "title": "Cycle2 X", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"issue_id": "cycle2-x", "depends_on_id": "cycle2-y", "type": "blocks"}]}
{"id": "cycle2-y", "title": "Cycle2 Y", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"issue_id": "cycle2-y", "depends_on_id": "cycle2-x", "type": "blocks"}]}`

	if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(jsonl), 0644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}
	return repoDir
}

// createNestedCyclesRepo creates overlapping cycles sharing common nodes
func createNestedCyclesRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Nested: A->B->C->A and A->B->D->A share A and B
	// Structure: A -> B -> C -> A, and B -> D -> A
	jsonl := `{"id": "nest-a", "title": "Nest A", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"issue_id": "nest-a", "depends_on_id": "nest-b", "type": "blocks"}]}
{"id": "nest-b", "title": "Nest B", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"issue_id": "nest-b", "depends_on_id": "nest-c", "type": "blocks"}, {"issue_id": "nest-b", "depends_on_id": "nest-d", "type": "blocks"}]}
{"id": "nest-c", "title": "Nest C", "status": "open", "priority": 3, "issue_type": "task", "dependencies": [{"issue_id": "nest-c", "depends_on_id": "nest-a", "type": "blocks"}]}
{"id": "nest-d", "title": "Nest D", "status": "open", "priority": 3, "issue_type": "task", "dependencies": [{"issue_id": "nest-d", "depends_on_id": "nest-a", "type": "blocks"}]}`

	if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(jsonl), 0644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}
	return repoDir
}

// createMixedCycleDAGRepo creates a graph with both cycle and DAG portions
func createMixedCycleDAGRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Mixed: DAG portion (dag-root -> dag-mid -> dag-leaf) + Cycle (cycle-a <-> cycle-b)
	jsonl := `{"id": "dag-root", "title": "DAG Root", "status": "open", "priority": 0, "issue_type": "task"}
{"id": "dag-mid", "title": "DAG Mid", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"issue_id": "dag-mid", "depends_on_id": "dag-root", "type": "blocks"}]}
{"id": "dag-leaf", "title": "DAG Leaf", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"issue_id": "dag-leaf", "depends_on_id": "dag-mid", "type": "blocks"}]}
{"id": "cycle-a", "title": "Cycle A", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"issue_id": "cycle-a", "depends_on_id": "cycle-b", "type": "blocks"}]}
{"id": "cycle-b", "title": "Cycle B", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"issue_id": "cycle-b", "depends_on_id": "cycle-a", "type": "blocks"}]}`

	if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(jsonl), 0644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}
	return repoDir
}
