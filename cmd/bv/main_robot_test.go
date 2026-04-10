package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRobotPlanAndPriorityIncludeMetadata runs the built binary against a tiny fixture project
// to assert that robot-plan and robot-priority include data_hash, analysis_config, and status.
func TestRobotPlanAndPriorityIncludeMetadata(t *testing.T) {
	dir := t.TempDir()
	// create minimal .beads directory with beads.jsonl
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	beads := `{"id":"TEST-1","title":"A","status":"open","priority":1,"issue_type":"task"}
{"id":"TEST-2","title":"B","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"TEST-2","depends_on_id":"TEST-1","type":"blocks"}]}
`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	exe := buildTestBinary(t)

	runAndCheck := func(flag string) {
		cmd := exec.Command(exe, flag)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s failed: %v, out=%s", flag, err, string(out))
		}
		var payload map[string]any
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("%s json: %v", flag, err)
		}
		if _, ok := payload["data_hash"]; !ok {
			t.Fatalf("%s missing data_hash", flag)
		}
		if _, ok := payload["analysis_config"]; !ok {
			t.Fatalf("%s missing analysis_config", flag)
		}
		statusAny, ok := payload["status"]
		if !ok {
			t.Fatalf("%s missing status", flag)
		}

		status, ok := statusAny.(map[string]any)
		if !ok {
			t.Fatalf("%s status not an object", flag)
		}

		// Ensure the status contract is usable at process exit (no pending/empty states).
		expected := []string{"PageRank", "Betweenness", "Eigenvector", "HITS", "Critical", "Cycles", "KCore", "Articulation", "Slack"}
		for _, metric := range expected {
			entryAny, ok := status[metric]
			if !ok {
				t.Fatalf("%s status missing %s", flag, metric)
			}
			entry, ok := entryAny.(map[string]any)
			if !ok {
				t.Fatalf("%s status.%s not an object", flag, metric)
			}
			stateAny, ok := entry["state"]
			if !ok {
				t.Fatalf("%s status.%s missing state", flag, metric)
			}
			state, _ := stateAny.(string)
			if state == "" {
				t.Fatalf("%s status.%s state empty", flag, metric)
			}
			if state == "pending" {
				t.Fatalf("%s status.%s still pending at exit", flag, metric)
			}
		}
	}

	runAndCheck("--robot-plan")
	runAndCheck("--robot-priority")
}

// buildTestBinary builds the current module's bv binary for testing.
func buildTestBinary(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "bv-testbin")
	cmd := exec.Command("go", "build", "-o", exe, ".")
	cmd.Dir = "." // build current package
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build bv: %v, out=%s", err, string(out))
	}
	return exe
}

// TestTOONOutputFormat verifies that --format=toon produces valid TOON output (bd-2lmf)
func TestTOONOutputFormat(t *testing.T) {
	// Check if tru binary is available
	if _, err := exec.LookPath("tru"); err != nil {
		t.Skip("tru binary not available, skipping TOON tests")
	}

	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	beads := `{"id":"TEST-1","title":"Test Issue","status":"open","priority":1,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	exe := buildTestBinary(t)

	// Test TOON output for robot-next
	cmd := exec.Command(exe, "--robot-next", "--format=toon")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("robot-next with toon failed: %v", err)
	}

	// TOON output should not start with { (that's JSON)
	toonOut := string(out)
	if len(toonOut) > 0 && toonOut[0] == '{' {
		t.Fatalf("TOON output looks like JSON, expected TOON format: %s", toonOut[:min(100, len(toonOut))])
	}

	// Should contain key: value pattern typical of TOON
	if !containsKeyValuePattern(toonOut) {
		t.Fatalf("TOON output doesn't look like TOON: %s", toonOut[:min(200, len(toonOut))])
	}
}

// TestTOONRoundTrip verifies that TOON output can be decoded back to JSON (bd-2lmf)
func TestTOONRoundTrip(t *testing.T) {
	// Check if tru binary is available
	truPath, err := exec.LookPath("tru")
	if err != nil {
		t.Skip("tru binary not available, skipping TOON round-trip test")
	}

	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	beads := `{"id":"TEST-1","title":"Round Trip Test","status":"open","priority":2,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	exe := buildTestBinary(t)

	// Get TOON output
	cmd := exec.Command(exe, "--robot-next", "--format=toon")
	cmd.Dir = dir
	toonOut, err := cmd.Output()
	if err != nil {
		t.Fatalf("robot-next with toon failed: %v", err)
	}

	// Decode TOON back to JSON using tru --decode
	decodeCmd := exec.Command(truPath, "--decode")
	decodeCmd.Stdin = strings.NewReader(string(toonOut))
	jsonOut, err := decodeCmd.Output()
	if err != nil {
		t.Fatalf("tru --decode failed: %v", err)
	}

	// Verify the decoded JSON is valid and contains expected fields
	var payload map[string]interface{}
	if err := json.Unmarshal(jsonOut, &payload); err != nil {
		t.Fatalf("decoded JSON is invalid: %v, content: %s", err, string(jsonOut))
	}

	// Check required fields are present
	if _, ok := payload["id"]; !ok {
		t.Error("decoded payload missing 'id' field")
	}
	if _, ok := payload["title"]; !ok {
		t.Error("decoded payload missing 'title' field")
	}
	if _, ok := payload["generated_at"]; !ok {
		t.Error("decoded payload missing 'generated_at' field")
	}
}

// TestTOONTokenStats verifies that --stats produces token statistics on stderr (bd-2lmf)
func TestTOONTokenStats(t *testing.T) {
	// Check if tru binary is available
	if _, err := exec.LookPath("tru"); err != nil {
		t.Skip("tru binary not available, skipping TOON stats test")
	}

	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	beads := `{"id":"TEST-1","title":"Stats Test Issue","status":"open","priority":1,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	exe := buildTestBinary(t)

	// Test --stats flag with TOON output
	cmd := exec.Command(exe, "--robot-next", "--format=toon", "--stats")
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	_, err := cmd.Output()
	if err != nil {
		t.Fatalf("robot-next with stats failed: %v", err)
	}

	stderrStr := stderr.String()
	// Should contain token statistics
	if !strings.Contains(stderrStr, "tok") || !strings.Contains(stderrStr, "savings") {
		t.Errorf("--stats should produce token statistics on stderr, got: %s", stderrStr)
	}
}

// TestTOONSchemaOutput verifies that --robot-schema works with TOON format (bd-2lmf)
func TestTOONSchemaOutput(t *testing.T) {
	// Check if tru binary is available
	if _, err := exec.LookPath("tru"); err != nil {
		t.Skip("tru binary not available, skipping TOON schema test")
	}

	exe := buildTestBinary(t)

	// Test --robot-schema with TOON format
	cmd := exec.Command(exe, "--robot-schema", "--format=toon")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("robot-schema with toon failed: %v", err)
	}

	toonOut := string(out)
	// Should produce valid TOON output
	if len(toonOut) > 0 && toonOut[0] == '{' {
		t.Fatalf("TOON output looks like JSON, expected TOON format")
	}

	// Should contain schema_version key
	if !strings.Contains(toonOut, "schema_version") {
		t.Error("TOON schema output missing schema_version")
	}
}

// containsKeyValuePattern checks if the string looks like TOON format
func containsKeyValuePattern(s string) bool {
	// TOON format typically has lines like "key: value" without the JSON braces/quotes
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Look for key: value pattern (not JSON's "key": value)
		if strings.Contains(trimmed, ": ") && !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "\"") {
			return true
		}
	}
	return false
}
