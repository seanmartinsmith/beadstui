package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// sourceFixtureBeads is a multi-prefix issue set for --source contract tests.
// Two projects' worth of issues under one beads.jsonl; filtering by ID prefix
// should isolate each.
const sourceFixtureBeads = `{"id":"bt-a","title":"bt one","status":"open","priority":1,"issue_type":"task","source_repo":"bt","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
{"id":"bt-b","title":"bt two","status":"open","priority":2,"issue_type":"task","source_repo":"bt","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
{"id":"cass-a","title":"cass one","status":"open","priority":1,"issue_type":"task","source_repo":"cass","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
`

func setupSourceFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(sourceFixtureBeads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}
	return dir
}

// TestRobotList_SourceFilter_Matches — --source=cass filters the list down to
// cass-* issues only.
func TestRobotList_SourceFilter_Matches(t *testing.T) {
	dir := setupSourceFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "list", "--source", "cass")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list --source=cass failed: %v\nout=%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	count, _ := payload["count"].(float64)
	if int(count) != 1 {
		t.Errorf("count = %v, want 1", count)
	}
	issues, _ := payload["issues"].([]any)
	if len(issues) != 1 {
		t.Fatalf("issues len = %d, want 1", len(issues))
	}
	first, _ := issues[0].(map[string]any)
	if id, _ := first["id"].(string); id != "cass-a" {
		t.Errorf("filtered id = %q, want cass-a", id)
	}
	// Query echo carries the source filter.
	q, _ := payload["query"].(map[string]any)
	if src, _ := q["source"].(string); src != "cass" {
		t.Errorf("query.source = %q, want cass", src)
	}
}

// TestRobotList_SourceFilter_NonexistentEmpty — --source=nonexistent yields
// an empty list, exit 0, not an error.
func TestRobotList_SourceFilter_NonexistentEmpty(t *testing.T) {
	dir := setupSourceFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "list", "--source", "nonexistent-project")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list --source=nonexistent should exit 0; err=%v\nout=%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	count, _ := payload["count"].(float64)
	if int(count) != 0 {
		t.Errorf("count = %v, want 0 for unknown source", count)
	}
}

// TestRobotList_SourceFilter_CommaSeparated — multiple projects via
// comma-separated list.
func TestRobotList_SourceFilter_CommaSeparated(t *testing.T) {
	dir := setupSourceFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "list", "--source", "bt,cass")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list --source=bt,cass failed: %v\nout=%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	count, _ := payload["count"].(float64)
	if int(count) != 3 {
		t.Errorf("count = %v, want 3 (bt-a, bt-b, cass-a)", count)
	}
}
