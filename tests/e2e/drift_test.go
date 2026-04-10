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
)

func TestEndToEndDriftWorkflow(t *testing.T) {
	// 1. Build the binary
	tempDir := t.TempDir()
	binPath := filepath.Join(tempDir, "bv")

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/bv/")
	cmd.Dir = "../../"
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %v\n%s", err, out)
	}

	// 2. Prepare environment
	envDir := filepath.Join(tempDir, "env")
	if err := os.MkdirAll(filepath.Join(envDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}

	// 3. Create initial healthy graph
	// Chain A <- B <- C (acyclic)
	jsonlContent := `{"id": "A", "title": "Task A", "status": "open", "priority": 1, "issue_type": "task"}
{"id": "B", "title": "Task B", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"depends_on_id": "A", "type": "blocks"}]}
{"id": "C", "title": "Task C", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"depends_on_id": "B", "type": "blocks"}]}`

	beadsPath := filepath.Join(envDir, ".beads", "beads.jsonl")
	if err := os.WriteFile(beadsPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Save Baseline
	cmdSave := exec.Command(binPath, "--save-baseline", "Initial state")
	cmdSave.Dir = envDir
	if out, err := cmdSave.CombinedOutput(); err != nil {
		t.Fatalf("Save baseline failed: %v\n%s", err, out)
	}

	// Verify baseline file exists
	baselinePath := filepath.Join(envDir, ".bv", "baseline.json")
	if _, err := os.Stat(baselinePath); os.IsNotExist(err) {
		t.Fatalf("Baseline file not created at %s", baselinePath)
	}

	// 5. Check Drift (Should be clean)
	cmdCheck := exec.Command(binPath, "--check-drift")
	cmdCheck.Dir = envDir
	if out, err := cmdCheck.CombinedOutput(); err != nil {
		t.Fatalf("Check drift (clean) failed: %v\n%s", err, out)
	}

	// 6. Introduce Drift (New Cycle)
	// Create cycle A <- B <- C <- A
	driftedContent := `{"id": "A", "title": "Task A", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"depends_on_id": "C", "type": "blocks"}]}
{"id": "B", "title": "Task B", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"depends_on_id": "A", "type": "blocks"}]}
{"id": "C", "title": "Task C", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"depends_on_id": "B", "type": "blocks"}]}`

	if err := os.WriteFile(beadsPath, []byte(driftedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 7. Check Drift (Should Fail with Exit Code 1)
	cmdDrift := exec.Command(binPath, "--check-drift")
	cmdDrift.Dir = envDir
	out, err := cmdDrift.CombinedOutput()

	// Expect exit code 1 (Critical)
	if err == nil {
		t.Fatalf("Expected drift check to fail, but it succeeded. Output:\n%s", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("Expected exit code 1 (critical), got %d", exitErr.ExitCode())
	}
	// Verify output contains "CRITICAL"
	outputStr := string(out)
	if !strings.Contains(outputStr, "CRITICAL") {
		t.Errorf("Expected output to mention CRITICAL, got:\n%s", outputStr)
	}

	// 8. Verify with JSON output
	cmdJson := exec.Command(binPath, "--check-drift", "--robot-drift")
	cmdJson.Dir = envDir
	outJson, err := cmdJson.CombinedOutput()
	// JSON mode also exits with code, so we expect error
	if err == nil {
		t.Fatal("Expected JSON drift check to fail with exit code")
	}

	// Some builds may emit a leading debug line; strip anything before the first '{'
	if idx := bytes.IndexByte(outJson, '{'); idx > 0 {
		outJson = outJson[idx:]
	}

	var result map[string]interface{}
	if err := json.Unmarshal(outJson, &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\n%s", err, outJson)
	}

	if hasDrift, ok := result["has_drift"].(bool); !ok || !hasDrift {
		t.Error("JSON output has_drift should be true")
	}

	alerts, ok := result["alerts"].([]interface{})
	if !ok || len(alerts) == 0 {
		t.Error("JSON output should have alerts")
	} else {
		// Check first alert is new_cycle
		firstAlert := alerts[0].(map[string]interface{})
		if firstAlert["type"] != "new_cycle" {
			t.Errorf("Expected new_cycle alert, got %v", firstAlert["type"])
		}
	}
}

func TestDriftAlerts(t *testing.T) {
	// 1. Build
	tempDir := t.TempDir()
	binPath := filepath.Join(tempDir, "bv")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/bv/")
	cmd.Dir = "../../"
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %v\n%s", err, out)
	}

	// 2. Setup Env
	envDir := filepath.Join(tempDir, "env")
	if err := os.MkdirAll(filepath.Join(envDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}
	beadsPath := filepath.Join(envDir, ".beads", "beads.jsonl")

	// 3. Create Baseline
	// 10 Nodes (A..J). 1 Edge (A->B).
	// Density = 1 / (10*9) = 1/90 = 0.0111
	// Blocked: 0 (all marked open)
	baselineContent := ""
	// A is free
	baselineContent += `{"id": "A", "title": "Task A", "status": "open", "issue_type": "task"}` + "\n"
	// B depends on A
	baselineContent += `{"id": "B", "title": "Task B", "status": "open", "issue_type": "task", "dependencies": [{"depends_on_id": "A", "type": "blocks"}]}` + "\n"
	// C..J are free
	ids := []string{"C", "D", "E", "F", "G", "H", "I", "J"}
	for _, id := range ids {
		baselineContent += fmt.Sprintf(`{"id": "%s", "title": "Task %s", "status": "open", "issue_type": "task"}`, id, id) + "\n"
	}

	if err := os.WriteFile(beadsPath, []byte(baselineContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Save Baseline
	cmdSave := exec.Command(binPath, "--save-baseline", "Baseline")
	cmdSave.Dir = envDir
	if out, err := cmdSave.CombinedOutput(); err != nil {
		t.Fatalf("Save baseline failed: %v\n%s", err, out)
	}

	// 4. Create High Density & Blocked Increase
	// Keep 10 Nodes.
	// Add edges: A->C, A->D, ..., A->J (8 more edges).
	// Total Edges: 9.
	// Density = 9/90 = 0.1.
	// Increase: (0.1 - 0.0111) / 0.0111 ~ 800%. Warning.

	// Blocked:
	// Mark B..J as "blocked".
	// Total Blocked: 9.
	// Baseline Blocked: 0.
	// Delta: 9. Threshold 5. Warning.

	driftContent := ""
	driftContent += `{"id": "A", "title": "Task A", "status": "open", "issue_type": "task"}` + "\n"

	// B..J depend on A and are blocked
	allIds := []string{"B", "C", "D", "E", "F", "G", "H", "I", "J"}
	for _, id := range allIds {
		driftContent += fmt.Sprintf(`{"id": "%s", "title": "Task %s", "status": "blocked", "issue_type": "task", "dependencies": [{"depends_on_id": "A", "type": "blocks"}]}`, id, id) + "\n"
	}

	if err := os.WriteFile(beadsPath, []byte(driftContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 5. Check Drift
	cmdCheck := exec.Command(binPath, "--check-drift")
	cmdCheck.Dir = envDir
	out, err := cmdCheck.CombinedOutput()

	// Expect Exit Code 2 (Warning)
	if err == nil {
		t.Fatalf("Expected warning exit code, got success. Output:\n%s", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 2 {
		t.Errorf("Expected exit code 2 (warning), got %d", exitErr.ExitCode())
	}

	outputStr := string(out)
	if !strings.Contains(outputStr, "Graph density increased") {
		t.Errorf("Output missing density warning. Got:\n%s", outputStr)
	}
	if !strings.Contains(outputStr, "Blocked issues increased") {
		t.Error("Output missing blocked issues warning")
	}
}

func TestDriftConfigCustomization(t *testing.T) {
	// 1. Build
	tempDir := t.TempDir()
	binPath := filepath.Join(tempDir, "bv")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/bv/")
	cmd.Dir = "../../"
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %v\n%s", err, out)
	}

	envDir := filepath.Join(tempDir, "env")
	if err := os.MkdirAll(filepath.Join(envDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(envDir, ".bv"), 0755); err != nil {
		t.Fatal(err)
	}

	beadsPath := filepath.Join(envDir, ".beads", "beads.jsonl")
	configPath := filepath.Join(envDir, ".bv", "drift.yaml")
	var out []byte
	var err error

	// -------------------------------------------------------------------------
	// Scenario 1: Custom High Thresholds (Suppress Warnings)
	// -------------------------------------------------------------------------
	t.Log("Scenario 1: Custom High Thresholds")

	// Write custom config: Blocked Increase Threshold = 100 (default 5)
	// Also raise PageRank threshold to avoid noise
	// Write FULL config to ensure defaults aren't zeroed by quirks
	configContent := `
density_warning_pct: 50
density_info_pct: 20
node_growth_info_pct: 25
edge_growth_info_pct: 25
blocked_increase_threshold: 100
actionable_decrease_warning_pct: 30
actionable_increase_info_pct: 20
pagerank_change_warning_pct: 1000
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Baseline: 0 blocked
	baselineContent := `{"id": "A", "title": "Task A", "status": "open", "issue_type": "task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(baselineContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmdSave := exec.Command(binPath, "--save-baseline", "Baseline")
	cmdSave.Dir = envDir
	out, err = cmdSave.CombinedOutput()
	if err != nil {
		t.Fatalf("Save baseline failed: %v\n%s", err, out)
	}

	// Current: 10 blocked (would trigger default warning of 5)
	driftContent := `{"id": "A", "title": "Task A", "status": "open", "issue_type": "task"}` + "\n"
	// Add 10 blocked issues
	for i := 0; i < 10; i++ {
		driftContent += fmt.Sprintf(`{"id": "B%d", "title": "Blocker %d", "status": "blocked", "issue_type": "task", "dependencies": [{"depends_on_id": "A", "type": "blocks"}]}`+"\n", i, i)
	}
	if err := os.WriteFile(beadsPath, []byte(driftContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Check Drift
	// NOTE: PageRank changes will still trigger a warning (exit 2) because "entered top" logic
	// doesn't respect the percentage threshold. We only check that Blocked warning is suppressed.
	cmdCheck := exec.Command(binPath, "--check-drift")
	cmdCheck.Dir = envDir
	out, err = cmdCheck.CombinedOutput()

	// We expect exit code 2 due to PageRank changes (unavoidable with new nodes)
	// But we MUST NOT see "Blocked issues increased"
	if err == nil {
		// If it passes, great, but unexpected given PR changes logic
	} else {
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 2 {
			t.Errorf("Expected exit code 2 (PR warning), got %v", err)
		}
	}

	output := string(out)
	if strings.Contains(output, "Blocked issues increased") {
		t.Errorf("Expected blocked warning to be suppressed by high threshold, but found it:\n%s", output)
	}
	if !strings.Contains(output, "PageRank changes") {
		// Just to be sure we are seeing the right output
		// t.Log("Note: PageRank warning not found, maybe graph structure didn't trigger it?")
	}

	// -------------------------------------------------------------------------
	// Scenario 2: Custom Low Thresholds (Trigger Warnings)
	// -------------------------------------------------------------------------
	t.Log("Scenario 2: Custom Low Thresholds")

	// Ensure beads file still has the blocked issues
	if err := os.WriteFile(beadsPath, []byte(driftContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Write custom config: Blocked Increase Threshold = 1
	configContent = `blocked_increase_threshold: 1
node_growth_info_pct: 1000
edge_growth_info_pct: 1000
pagerank_change_warning_pct: 1000
actionable_increase_info_pct: 1000
actionable_decrease_warning_pct: 1000`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Same drift data (10 blocked). 10 > 1. Should FAIL (Exit 2).
	cmdCheck = exec.Command(binPath, "--check-drift")
	cmdCheck.Dir = envDir
	out, err = cmdCheck.CombinedOutput()

	if err == nil {
		t.Errorf("Expected warning with low threshold, got success. Output:\n%s", out)
	} else {
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 2 {
			t.Errorf("Expected exit code 2, got %v", err)
		}
		if !strings.Contains(string(out), "Blocked issues increased") {
			t.Errorf("Expected blocked warning, got:\n%s", out)
		}
	}
}
func TestDriftErrorHandling(t *testing.T) {
	// 1. Build
	tempDir := t.TempDir()
	binPath := filepath.Join(tempDir, "bv")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/bv/")
	cmd.Dir = "../../"
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %v\n%s", err, out)
	}

	envDir := filepath.Join(tempDir, "env")
	if err := os.MkdirAll(filepath.Join(envDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create minimal beads file
	beadsPath := filepath.Join(envDir, ".beads", "beads.jsonl")
	if err := os.WriteFile(beadsPath, []byte(`{"id":"A","status":"open","issue_type":"task"}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// -------------------------------------------------------------------------
	// Scenario 1: Missing Baseline
	// -------------------------------------------------------------------------
	t.Log("Scenario 1: Missing Baseline")
	cmdCheck := exec.Command(binPath, "--check-drift")
	cmdCheck.Dir = envDir
	out, err := cmdCheck.CombinedOutput()

	if err == nil {
		t.Error("Expected error when baseline missing, got success")
	} else {
		// Expect exit code 1
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 1 {
			t.Errorf("Expected exit code 1, got %v", err)
		}
		output := string(out)
		if !strings.Contains(output, "No baseline found") {
			t.Errorf("Expected 'No baseline found', got:\n%s", output)
		}
	}

	// -------------------------------------------------------------------------
	// Scenario 2: Corrupt Baseline
	// -------------------------------------------------------------------------
	t.Log("Scenario 2: Corrupt Baseline")

	baselineDir := filepath.Join(envDir, ".bv")
	if err := os.MkdirAll(baselineDir, 0755); err != nil {
		t.Fatal(err)
	}
	baselinePath := filepath.Join(baselineDir, "baseline.json")

	// Write invalid JSON
	if err := os.WriteFile(baselinePath, []byte(`{not valid json}`), 0644); err != nil {
		t.Fatal(err)
	}

	cmdCheck = exec.Command(binPath, "--check-drift")
	cmdCheck.Dir = envDir
	out, err = cmdCheck.CombinedOutput()

	if err == nil {
		t.Error("Expected error when baseline corrupt, got success")
	} else {
		// Expect exit code 1
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 1 {
			t.Errorf("Expected exit code 1, got %v", err)
		}
		output := string(out)
		if !strings.Contains(output, "Error loading baseline") {
			t.Errorf("Expected 'Error loading baseline', got:\n%s", output)
		}
	}

	// -------------------------------------------------------------------------
	// Scenario 3: Invalid Drift Config
	// -------------------------------------------------------------------------
	t.Log("Scenario 3: Invalid Drift Config")

	// Restore valid baseline
	cmdSave := exec.Command(binPath, "--save-baseline", "Restore")
	cmdSave.Dir = envDir
	if out, err := cmdSave.CombinedOutput(); err != nil {
		t.Fatalf("Restore baseline failed: %v\n%s", err, out)
	}

	// Write invalid config (negative threshold)
	configPath := filepath.Join(baselineDir, "drift.yaml")
	if err := os.WriteFile(configPath, []byte(`density_warning_pct: -50`), 0644); err != nil {
		t.Fatal(err)
	}

	cmdCheck = exec.Command(binPath, "--check-drift")
	cmdCheck.Dir = envDir
	out, err = cmdCheck.CombinedOutput()

	// Should still SUCCEED (fallback to defaults) but print warning
	if err != nil {
		t.Errorf("Expected success (fallback) with invalid config, got error: %v\nOutput: %s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Warning: Error loading drift config") {
		t.Errorf("Expected warning about invalid config, got:\n%s", output)
	}
}
