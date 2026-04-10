package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEndToEndBuildAndRun(t *testing.T) {
	binPath := buildBvBinary(t)
	tempDir := t.TempDir()

	// 2. Prepare a fake environment with .beads/beads.jsonl (canonical filename)
	envDir := filepath.Join(tempDir, "env")
	if err := os.MkdirAll(filepath.Join(envDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}

	jsonlContent := `{"id": "bd-1", "title": "E2E Test Issue", "status": "open", "priority": 0, "issue_type": "bug"}`
	if err := os.WriteFile(filepath.Join(envDir, ".beads", "beads.jsonl"), []byte(jsonlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Run bv --version to verify it runs
	runCmd := exec.Command(binPath, "--version")
	runCmd.Dir = envDir
	if out, err := runCmd.CombinedOutput(); err != nil {
		t.Fatalf("Execution failed: %v\n%s", err, out)
	}
}

func TestEndToEndRobotPlan(t *testing.T) {
	binPath := buildBvBinary(t)
	tempDir := t.TempDir()

	// 2. Create environment with dependency chain
	envDir := filepath.Join(tempDir, "env")
	if err := os.MkdirAll(filepath.Join(envDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create issues: epic -> task -> subtask (dependency chain)
	jsonlContent := `{"id": "epic-1", "title": "Epic", "status": "open", "priority": 0, "issue_type": "epic"}
{"id": "task-1", "title": "Task", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "epic-1", "type": "child_of"}]}
{"id": "subtask-1", "title": "Subtask", "status": "open", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "task-1", "type": "blocks"}]}`

	if err := os.WriteFile(filepath.Join(envDir, ".beads", "beads.jsonl"), []byte(jsonlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Run bv --robot-plan
	runCmd := exec.Command(binPath, "--robot-plan")
	runCmd.Dir = envDir
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-plan failed: %v\n%s", err, out)
	}

	// 4. Verify output is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("--robot-plan output is not valid JSON: %v\nOutput: %s", err, out)
	}

	// 5. Verify expected top-level structure
	if _, ok := result["generated_at"]; !ok {
		t.Error("--robot-plan output missing 'generated_at' field")
	}
	plan, ok := result["plan"].(map[string]interface{})
	if !ok {
		t.Fatalf("'plan' is not an object: %T", result["plan"])
	}

	// 6. Verify plan structure
	if _, ok := plan["tracks"]; !ok {
		t.Error("--robot-plan output missing 'plan.tracks' field")
	}
	if _, ok := plan["summary"]; !ok {
		t.Error("--robot-plan output missing 'plan.summary' field")
	}

	// 7. Verify tracks is an array
	tracks, ok := plan["tracks"].([]interface{})
	if !ok {
		t.Fatalf("'plan.tracks' is not an array: %T", plan["tracks"])
	}

	// Should have at least one track with actionable items
	if len(tracks) == 0 {
		t.Error("Expected at least one track in execution plan")
	}
}

func TestEndToEndRobotInsights(t *testing.T) {
	binPath := buildBvBinary(t)
	tempDir := t.TempDir()

	// 2. Create environment
	envDir := filepath.Join(tempDir, "env")
	if err := os.MkdirAll(filepath.Join(envDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}

	jsonlContent := `{"id": "A", "title": "Root", "status": "open", "priority": 1, "issue_type": "task"}
{"id": "B", "title": "Child", "status": "open", "priority": 1, "issue_type": "task", "dependencies": [{"issue_id": "B", "depends_on_id": "A", "type": "blocks"}]}`
	if err := os.WriteFile(filepath.Join(envDir, ".beads", "beads.jsonl"), []byte(jsonlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Run bv --robot-insights
	runCmd := exec.Command(binPath, "--robot-insights")
	runCmd.Dir = envDir
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-insights failed: %v\n%s", err, out)
	}

	// 4. Verify output
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("--robot-insights output is not valid JSON: %v\nOutput: %s", err, out)
	}

	if _, ok := result["Bottlenecks"]; !ok {
		t.Error("missing 'Bottlenecks'")
	}
	if _, ok := result["Stats"]; !ok {
		t.Error("missing 'Stats'")
	}
}

func TestEndToEndRobotPriority(t *testing.T) {
	binPath := buildBvBinary(t)
	tempDir := t.TempDir()

	// 2. Create environment
	envDir := filepath.Join(tempDir, "env")
	if err := os.MkdirAll(filepath.Join(envDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a mis-prioritized item (High impact but Low priority) to trigger recommendation
	jsonlContent := `{"id": "IMP-1", "title": "Important", "status": "open", "priority": 5, "issue_type": "task"}
{"id": "DEP-1", "title": "Dependent 1", "status": "open", "issue_type": "task", "dependencies": [{"issue_id": "DEP-1", "depends_on_id": "IMP-1", "type": "blocks"}]}
{"id": "DEP-2", "title": "Dependent 2", "status": "open", "issue_type": "task", "dependencies": [{"issue_id": "DEP-2", "depends_on_id": "IMP-1", "type": "blocks"}]}`
	if err := os.WriteFile(filepath.Join(envDir, ".beads", "beads.jsonl"), []byte(jsonlContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Run bv --robot-priority
	runCmd := exec.Command(binPath, "--robot-priority")
	runCmd.Dir = envDir
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-priority failed: %v\n%s", err, out)
	}

	// 4. Verify output
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("--robot-priority output is not valid JSON: %v\nOutput: %s", err, out)
	}

	if _, ok := result["recommendations"]; !ok {
		t.Error("missing 'recommendations'")
	}
}

func TestEndToEndRobotRecipes(t *testing.T) {
	binPath := buildBvBinary(t)
	tempDir := t.TempDir()

	// 2. Create environment (doesn't need beads file strictly, but loader checks)
	envDir := filepath.Join(tempDir, "env")
	if err := os.MkdirAll(filepath.Join(envDir, ".beads"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(envDir, ".beads", "beads.jsonl"), []byte("{}"), 0644)

	// 3. Run bv --robot-recipes
	runCmd := exec.Command(binPath, "--robot-recipes")
	runCmd.Dir = envDir
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-recipes failed: %v\n%s", err, out)
	}

	// 4. Verify output
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("--robot-recipes output is not valid JSON: %v\nOutput: %s", err, out)
	}

	if _, ok := result["recipes"]; !ok {
		t.Error("missing 'recipes'")
	}
}
