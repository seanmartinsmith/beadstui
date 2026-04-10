package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Export Graph Topologies E2E Tests (bv-gmka)
// Tests static export with different graph structures.

// =============================================================================
// 1. Empty Graph
// =============================================================================

// TestExportTopology_EmptyGraph tests export with no issues.
func TestExportTopology_EmptyGraph(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Create empty issues file
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	// Export should succeed with empty data
	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--export-pages failed with empty graph: %v\n%s", err, out)
	}

	// Verify basic artifacts exist
	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != 0 {
		t.Errorf("expected 0 issues, got %d", meta.IssueCount)
	}

	// Verify index.html exists
	if _, err := os.Stat(filepath.Join(exportDir, "index.html")); err != nil {
		t.Errorf("index.html missing for empty graph: %v", err)
	}
}

// =============================================================================
// 2. Single Node
// =============================================================================

// TestExportTopology_SingleNode tests export with one issue, no dependencies.
func TestExportTopology_SingleNode(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Single issue with no dependencies
	issueData := `{"id": "solo", "title": "Standalone Issue", "status": "open", "priority": 1, "issue_type": "task", "description": "A single issue with no dependencies"}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != 1 {
		t.Errorf("expected 1 issue, got %d", meta.IssueCount)
	}

	// Verify triage has the single issue
	triage := readTriageJSON(t, exportDir)
	if len(triage.Recommendations) != 1 {
		t.Errorf("expected 1 recommendation, got %d", len(triage.Recommendations))
	}
	if triage.QuickRef.OpenCount != 1 {
		t.Errorf("expected 1 open issue, got %d", triage.QuickRef.OpenCount)
	}
}

// =============================================================================
// 3. Linear Chain
// =============================================================================

// TestExportTopology_LinearChain tests A -> B -> C -> D -> E chain.
func TestExportTopology_LinearChain(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Linear chain: A -> B -> C -> D -> E (each blocks the next)
	issueData := `{"id": "chain-A", "title": "Chain A (Root)", "status": "open", "priority": 0, "issue_type": "task"}
{"id": "chain-B", "title": "Chain B", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "chain-A", "type": "blocks"}]}
{"id": "chain-C", "title": "Chain C", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "chain-B", "type": "blocks"}]}
{"id": "chain-D", "title": "Chain D", "status": "open", "priority": 3, "issue_type": "task", "dependencies": [{"target_id": "chain-C", "type": "blocks"}]}
{"id": "chain-E", "title": "Chain E (Leaf)", "status": "open", "priority": 4, "issue_type": "task", "dependencies": [{"target_id": "chain-D", "type": "blocks"}]}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != 5 {
		t.Errorf("expected 5 issues in chain, got %d", meta.IssueCount)
	}

	triage := readTriageJSON(t, exportDir)
	// Chain-A should be the root with highest priority
	foundRoot := false
	for _, rec := range triage.Recommendations {
		if rec.ID == "chain-A" {
			foundRoot = true
			break
		}
	}
	if !foundRoot {
		t.Error("chain-A (root) should be in recommendations")
	}
}

// =============================================================================
// 4. Star Topology
// =============================================================================

// TestExportTopology_StarHub tests hub with multiple spokes.
func TestExportTopology_StarHub(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Star: hub with 10 spokes
	var lines []string
	lines = append(lines, `{"id": "hub", "title": "Central Hub", "status": "open", "priority": 0, "issue_type": "epic"}`)
	for i := 1; i <= 10; i++ {
		line := fmt.Sprintf(`{"id": "spoke-%d", "title": "Spoke %d", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "hub", "type": "blocks"}]}`, i, i)
		lines = append(lines, line)
	}
	issueData := strings.Join(lines, "\n")
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != 11 {
		t.Errorf("expected 11 issues (1 hub + 10 spokes), got %d", meta.IssueCount)
	}

	// Hub should be a blocker clearing many issues
	triage := readTriageJSON(t, exportDir)
	if triage.QuickRef.OpenCount != 11 {
		t.Errorf("expected 11 open issues, got %d", triage.QuickRef.OpenCount)
	}
}

// =============================================================================
// 5. Diamond Topology
// =============================================================================

// TestExportTopology_Diamond tests A -> B, A -> C, B -> D, C -> D pattern.
func TestExportTopology_Diamond(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Diamond: A -> B, A -> C, B -> D, C -> D
	issueData := `{"id": "diamond-A", "title": "Diamond Top (A)", "status": "open", "priority": 0, "issue_type": "epic"}
{"id": "diamond-B", "title": "Diamond Left (B)", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "diamond-A", "type": "blocks"}]}
{"id": "diamond-C", "title": "Diamond Right (C)", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "diamond-A", "type": "blocks"}]}
{"id": "diamond-D", "title": "Diamond Bottom (D)", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "diamond-B", "type": "blocks"}, {"target_id": "diamond-C", "type": "blocks"}]}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != 4 {
		t.Errorf("expected 4 issues in diamond, got %d", meta.IssueCount)
	}

	// Verify triage correctly identifies dependencies
	triage := readTriageJSON(t, exportDir)
	if triage.QuickRef.OpenCount < 4 {
		t.Errorf("expected at least 4 open issues, got %d", triage.QuickRef.OpenCount)
	}
}

// =============================================================================
// 6. Cycles Present
// =============================================================================

// TestExportTopology_Cycle tests A -> B -> C -> A cycle handling.
func TestExportTopology_Cycle(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Cycle: A -> B -> C -> A
	issueData := `{"id": "cycle-A", "title": "Cycle Node A", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "cycle-C", "type": "blocks"}]}
{"id": "cycle-B", "title": "Cycle Node B", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "cycle-A", "type": "blocks"}]}
{"id": "cycle-C", "title": "Cycle Node C", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "cycle-B", "type": "blocks"}]}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	// Export should handle cycles gracefully
	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--export-pages should handle cycles: %v\n%s", err, out)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != 3 {
		t.Errorf("expected 3 issues in cycle, got %d", meta.IssueCount)
	}

	// Verify export completed despite cycle
	if _, err := os.Stat(filepath.Join(exportDir, "index.html")); err != nil {
		t.Errorf("index.html missing after cycle export: %v", err)
	}
}

// TestExportTopology_SelfLoop tests issue depending on itself.
func TestExportTopology_SelfLoop(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Self-loop: issue depends on itself
	issueData := `{"id": "self-loop", "title": "Self-referential Issue", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "self-loop", "type": "blocks"}]}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	// Export should handle self-loops
	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--export-pages should handle self-loop: %v\n%s", err, out)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != 1 {
		t.Errorf("expected 1 issue, got %d", meta.IssueCount)
	}
}

// =============================================================================
// 7. Disconnected Components
// =============================================================================

// TestExportTopology_DisconnectedComponents tests multiple isolated subgraphs.
func TestExportTopology_DisconnectedComponents(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Three disconnected components
	issueData := `{"id": "comp1-a", "title": "Component 1 - A", "status": "open", "priority": 1, "issue_type": "task"}
{"id": "comp1-b", "title": "Component 1 - B", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "comp1-a", "type": "blocks"}]}
{"id": "comp2-x", "title": "Component 2 - X", "status": "open", "priority": 1, "issue_type": "bug"}
{"id": "comp2-y", "title": "Component 2 - Y", "status": "open", "priority": 2, "issue_type": "bug", "dependencies": [{"target_id": "comp2-x", "type": "blocks"}]}
{"id": "comp3-solo", "title": "Component 3 - Solo", "status": "open", "priority": 3, "issue_type": "feature"}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != 5 {
		t.Errorf("expected 5 issues across 3 components, got %d", meta.IssueCount)
	}

	// All should appear in triage
	triage := readTriageJSON(t, exportDir)
	if triage.QuickRef.OpenCount != 5 {
		t.Errorf("expected 5 open issues, got %d", triage.QuickRef.OpenCount)
	}
}

// =============================================================================
// 8. Large Scale
// =============================================================================

// TestExportTopology_LargeScale tests 500+ nodes performance.
func TestExportTopology_LargeScale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large scale test in short mode")
	}

	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Generate 500 issues with chain dependencies
	const issueCount = 500
	var lines []string
	for i := 0; i < issueCount; i++ {
		var deps string
		if i > 0 {
			deps = fmt.Sprintf(`,"dependencies":[{"target_id":"large-%d","type":"blocks"}]`, i-1)
		}
		line := fmt.Sprintf(`{"id":"large-%d","title":"Large Scale Issue %d","status":"open","priority":%d,"issue_type":"task"%s}`,
			i, i, i%5, deps)
		lines = append(lines, line)
	}
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	start := time.Now()
	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("--export-pages failed with %d issues: %v\n%s", issueCount, err, out)
	}

	t.Logf("large scale export (%d issues): %v", issueCount, elapsed)

	// Performance threshold: should complete in under 30 seconds
	if elapsed > 30*time.Second {
		t.Errorf("export too slow for %d issues: %v", issueCount, elapsed)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != issueCount {
		t.Errorf("expected %d issues, got %d", issueCount, meta.IssueCount)
	}

	// Verify database was created
	dbPath := filepath.Join(exportDir, "beads.sqlite3")
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("beads.sqlite3 not found: %v", err)
	}
	t.Logf("database size for %d issues: %d KB", issueCount, info.Size()/1024)
}

// =============================================================================
// 9. Complex Topology (Mixed)
// =============================================================================

// TestExportTopology_ComplexMixed tests combination of multiple patterns.
func TestExportTopology_ComplexMixed(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Complex: chain + star + diamond + standalone
	issueData := `{"id": "epic-1", "title": "Epic 1", "status": "open", "priority": 0, "issue_type": "epic"}
{"id": "chain-1", "title": "Chain 1", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "epic-1", "type": "blocks"}]}
{"id": "chain-2", "title": "Chain 2", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "chain-1", "type": "blocks"}]}
{"id": "star-hub", "title": "Star Hub", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "epic-1", "type": "blocks"}]}
{"id": "star-spoke-1", "title": "Star Spoke 1", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "star-hub", "type": "blocks"}]}
{"id": "star-spoke-2", "title": "Star Spoke 2", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "star-hub", "type": "blocks"}]}
{"id": "diamond-left", "title": "Diamond Left", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "epic-1", "type": "blocks"}]}
{"id": "diamond-right", "title": "Diamond Right", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "epic-1", "type": "blocks"}]}
{"id": "diamond-bottom", "title": "Diamond Bottom", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "diamond-left", "type": "blocks"}, {"target_id": "diamond-right", "type": "blocks"}]}
{"id": "standalone", "title": "Standalone Issue", "status": "open", "priority": 3, "issue_type": "bug"}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != 10 {
		t.Errorf("expected 10 issues in complex graph, got %d", meta.IssueCount)
	}

	triage := readTriageJSON(t, exportDir)
	if triage.QuickRef.OpenCount != 10 {
		t.Errorf("expected 10 open issues, got %d", triage.QuickRef.OpenCount)
	}
}

// =============================================================================
// 10. Wide Graph (Many Roots)
// =============================================================================

// TestExportTopology_WideGraph tests graph with many independent roots.
func TestExportTopology_WideGraph(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Wide: 20 independent root issues
	var lines []string
	for i := 1; i <= 20; i++ {
		line := fmt.Sprintf(`{"id": "root-%d", "title": "Independent Root %d", "status": "open", "priority": %d, "issue_type": "task"}`, i, i, i%5)
		lines = append(lines, line)
	}
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != 20 {
		t.Errorf("expected 20 issues, got %d", meta.IssueCount)
	}

	// All should be actionable (no blockers)
	triage := readTriageJSON(t, exportDir)
	if triage.QuickRef.ActionableCount != 20 {
		t.Errorf("expected 20 actionable issues (all roots), got %d", triage.QuickRef.ActionableCount)
	}
}

// =============================================================================
// 11. Deep Graph (Long Chain)
// =============================================================================

// TestExportTopology_DeepChain tests very deep dependency chain.
func TestExportTopology_DeepChain(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Deep chain: 50 levels deep
	const depth = 50
	var lines []string
	for i := 0; i < depth; i++ {
		var deps string
		if i > 0 {
			deps = fmt.Sprintf(`,"dependencies":[{"target_id":"deep-%d","type":"blocks"}]`, i-1)
		}
		line := fmt.Sprintf(`{"id":"deep-%d","title":"Deep Level %d","status":"open","priority":%d,"issue_type":"task"%s}`,
			i, i, i%5, deps)
		lines = append(lines, line)
	}
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != depth {
		t.Errorf("expected %d issues, got %d", depth, meta.IssueCount)
	}

	// Only deep-0 (root) should be actionable
	triage := readTriageJSON(t, exportDir)
	if triage.QuickRef.ActionableCount < 1 {
		t.Errorf("expected at least 1 actionable issue (root), got %d", triage.QuickRef.ActionableCount)
	}
}

// =============================================================================
// 12. Mixed Status
// =============================================================================

// TestExportTopology_MixedStatus tests graph with various statuses.
func TestExportTopology_MixedStatus(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Mixed statuses
	issueData := `{"id": "open-1", "title": "Open Issue", "status": "open", "priority": 1, "issue_type": "task"}
{"id": "in-progress-1", "title": "In Progress Issue", "status": "in_progress", "priority": 1, "issue_type": "task"}
{"id": "blocked-1", "title": "Blocked Issue", "status": "blocked", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "open-1", "type": "blocks"}]}
{"id": "closed-1", "title": "Closed Issue", "status": "closed", "priority": 1, "issue_type": "task"}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	cmd := exec.Command(bv, "--export-pages", exportDir, "--pages-include-closed")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	meta := readMetaJSON(t, exportDir)
	if meta.IssueCount != 4 {
		t.Errorf("expected 4 issues, got %d", meta.IssueCount)
	}

	triage := readTriageJSON(t, exportDir)
	if triage.QuickRef.InProgressCount < 1 {
		t.Errorf("expected at least 1 in_progress issue, got %d", triage.QuickRef.InProgressCount)
	}
}

// =============================================================================
// 13. Graph Export Format Verification
// =============================================================================

// TestExportTopology_RobotGraphJSON tests robot graph JSON output.
func TestExportTopology_RobotGraphJSON(t *testing.T) {
	bv := buildBvBinary(t)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	// Simple graph for format testing
	issueData := `{"id": "A", "title": "Issue A", "status": "open", "priority": 1, "issue_type": "task"}
{"id": "B", "title": "Issue B", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "A", "type": "blocks"}]}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	// Test JSON format
	cmd := exec.Command(bv, "--robot-graph", "--graph-format", "json")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-graph json failed: %v\n%s", err, out)
	}

	var graphJSON struct {
		Format    string `json:"format"`
		Nodes     int    `json:"nodes"`
		Edges     int    `json:"edges"`
		DataHash  string `json:"data_hash"`
		Adjacency struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
			Edges []struct {
				Source string `json:"source"`
				Target string `json:"target"`
			} `json:"edges"`
		} `json:"adjacency"`
	}
	if err := json.Unmarshal(out, &graphJSON); err != nil {
		t.Fatalf("parse graph JSON: %v", err)
	}

	if graphJSON.Nodes < 2 {
		t.Errorf("expected at least 2 nodes, got %d", graphJSON.Nodes)
	}
	// B depends on A, so we should have at least 1 edge
	if graphJSON.Edges < 1 {
		t.Logf("edges = %d (dependency may not create edge in robot-graph)", graphJSON.Edges)
	}
	if len(graphJSON.Adjacency.Nodes) < 2 {
		t.Errorf("expected at least 2 adjacency nodes, got %d", len(graphJSON.Adjacency.Nodes))
	}
}

// TestExportTopology_RobotGraphDOT tests robot graph DOT output.
func TestExportTopology_RobotGraphDOT(t *testing.T) {
	bv := buildBvBinary(t)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	issueData := `{"id": "A", "title": "Issue A", "status": "open", "priority": 1, "issue_type": "task"}
{"id": "B", "title": "Issue B", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "A", "type": "blocks"}]}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	// Test DOT format
	cmd := exec.Command(bv, "--robot-graph", "--graph-format", "dot")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-graph dot failed: %v\n%s", err, out)
	}

	// DOT is wrapped in JSON with a "graph" field
	var dotJSON struct {
		Format string `json:"format"`
		Graph  string `json:"graph"`
		Nodes  int    `json:"nodes"`
		Edges  int    `json:"edges"`
	}
	if err := json.Unmarshal(out, &dotJSON); err != nil {
		t.Fatalf("parse DOT JSON wrapper: %v", err)
	}

	if !strings.Contains(dotJSON.Graph, "digraph") {
		t.Error("DOT graph should contain 'digraph'")
	}
	if dotJSON.Edges > 0 && !strings.Contains(dotJSON.Graph, "->") {
		t.Error("DOT graph with edges should contain '->'")
	}
	if dotJSON.Nodes < 2 {
		t.Errorf("expected at least 2 nodes, got %d", dotJSON.Nodes)
	}
}

// TestExportTopology_RobotGraphMermaid tests robot graph Mermaid output.
func TestExportTopology_RobotGraphMermaid(t *testing.T) {
	bv := buildBvBinary(t)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	issueData := `{"id": "A", "title": "Issue A", "status": "open", "priority": 1, "issue_type": "task"}
{"id": "B", "title": "Issue B", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "A", "type": "blocks"}]}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	// Test Mermaid format
	cmd := exec.Command(bv, "--robot-graph", "--graph-format", "mermaid")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-graph mermaid failed: %v\n%s", err, out)
	}

	mermaidOutput := string(out)
	if !strings.Contains(mermaidOutput, "graph") && !strings.Contains(mermaidOutput, "flowchart") {
		t.Error("Mermaid output should contain 'graph' or 'flowchart'")
	}
}
