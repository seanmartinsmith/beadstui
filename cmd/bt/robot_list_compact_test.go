package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// listFixtureBeads exercises every slot CompactIssue cares about: core fields,
// a populated metadata.created_by_session, a parent-child edge, a blocking
// edge whose target is closed, and a relates-to edge.
const listFixtureBeads = `{"id":"epic-1","title":"Epic parent","status":"open","priority":0,"issue_type":"epic","labels":["area:cli"],"description":"epic description that must never appear in compact output","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-20T10:00:00Z"}
{"id":"blk-1","title":"Closed blocker","status":"closed","priority":1,"issue_type":"task","description":"resolved","created_at":"2026-04-10T10:00:00Z","updated_at":"2026-04-12T10:00:00Z","closed_at":"2026-04-12T10:00:00Z"}
{"id":"child-1","title":"Child with metadata","description":"child description body","design":"child design body","acceptance_criteria":"child criteria","notes":"child notes","status":"open","priority":1,"issue_type":"task","labels":["area:cli"],"metadata":{"created_by_session":"cc-sess-a","claimed_by_session":"cc-sess-a"},"dependencies":[{"issue_id":"child-1","depends_on_id":"epic-1","type":"parent-child"},{"issue_id":"child-1","depends_on_id":"blk-1","type":"blocks"},{"issue_id":"child-1","depends_on_id":"epic-1","type":"related"}],"created_at":"2026-04-16T10:00:00Z","updated_at":"2026-04-18T10:00:00Z"}
`

// setupListFixture writes a tiny project under t.TempDir() for the robot
// list bellwether contract tests and returns the project directory.
func setupListFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(listFixtureBeads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}
	return dir
}

// runListFixture invokes the built bt binary in the fixture project.
func runListFixture(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	exe := buildTestBinary(t)
	cmd := exec.Command(exe, append([]string{"robot", "list"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bt robot list %v failed: %v\nout=%s", args, err, string(out))
	}
	return out
}

// TestRobotListCompactDefault verifies the compact shape is the default and
// that its wire payload contains none of the fat body fields.
func TestRobotListCompactDefault(t *testing.T) {
	dir := setupListFixture(t)
	out := runListFixture(t, dir)

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v\nraw=%s", err, string(out))
	}

	if schema, _ := payload["schema"].(string); schema != "compact.v1" {
		t.Errorf("envelope.schema = %q, want compact.v1", schema)
	}

	issues, ok := payload["issues"].([]any)
	if !ok {
		t.Fatalf("issues is not an array: %T", payload["issues"])
	}
	if len(issues) == 0 {
		t.Fatalf("expected at least one issue in compact output")
	}

	forbidden := []string{"description", "design", "acceptance_criteria", "notes", "comments", "close_reason"}
	required := []string{"id", "title", "status", "priority", "issue_type",
		"blockers_count", "unblocks_count", "children_count", "relates_count", "is_blocked",
		"created_at", "updated_at"}
	for i, raw := range issues {
		obj, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("issues[%d] is not an object: %T", i, raw)
		}
		for _, k := range forbidden {
			if _, has := obj[k]; has {
				t.Errorf("issues[%d].%s leaked into compact output", i, k)
			}
		}
		for _, k := range required {
			if _, has := obj[k]; !has {
				t.Errorf("issues[%d] missing required compact field %s", i, k)
			}
		}
	}
}

// TestRobotListFullPreservesBodies verifies --full restores the pre-compact
// shape: body fields present, envelope.schema omitted (omitempty).
func TestRobotListFullPreservesBodies(t *testing.T) {
	dir := setupListFixture(t)
	out := runListFixture(t, dir, "--full")

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v\nraw=%s", err, string(out))
	}
	if _, has := payload["schema"]; has {
		t.Errorf("envelope.schema must be omitted in full mode, got %v", payload["schema"])
	}
	issues, _ := payload["issues"].([]any)

	sawBody := false
	for _, raw := range issues {
		obj, _ := raw.(map[string]any)
		if id, _ := obj["id"].(string); id == "child-1" {
			if d, ok := obj["description"].(string); ok && strings.Contains(d, "child description body") {
				sawBody = true
			}
		}
	}
	if !sawBody {
		t.Errorf("--full output missing expected description body; full payload = %s", string(out))
	}
}

// TestRobotListShapeEnvAndAliases — env var and --compact/--full aliases
// pick the same shape as explicit --shape.
func TestRobotListShapeEnvAndAliases(t *testing.T) {
	dir := setupListFixture(t)

	cases := []struct {
		name       string
		env        map[string]string
		args       []string
		wantSchema string
	}{
		{"env compact", map[string]string{"BT_OUTPUT_SHAPE": "compact"}, nil, "compact.v1"},
		{"env full", map[string]string{"BT_OUTPUT_SHAPE": "full"}, nil, ""},
		{"--compact alias", nil, []string{"--compact"}, "compact.v1"},
		{"--full alias", nil, []string{"--full"}, ""},
		{"--shape compact", nil, []string{"--shape=compact"}, "compact.v1"},
		{"--shape full", nil, []string{"--shape=full"}, ""},
		{"cli overrides env", map[string]string{"BT_OUTPUT_SHAPE": "full"}, []string{"--compact"}, "compact.v1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exe := buildTestBinary(t)
			cmd := exec.Command(exe, append([]string{"robot", "list"}, tc.args...)...)
			cmd.Dir = dir
			env := append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
			for k, v := range tc.env {
				env = append(env, k+"="+v)
			}
			cmd.Env = env
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run failed: %v out=%s", err, out)
			}
			var payload map[string]any
			if err := json.Unmarshal(out, &payload); err != nil {
				t.Fatalf("parse: %v", err)
			}
			got, _ := payload["schema"].(string)
			if got != tc.wantSchema {
				t.Errorf("schema = %q, want %q", got, tc.wantSchema)
			}
		})
	}
}

// TestRobotListCompactFields validates the semantic counts (is_blocked,
// parent_id, children_count, blockers_count, relates_count, session bridge)
// on the fixture beads.
func TestRobotListCompactFields(t *testing.T) {
	dir := setupListFixture(t)
	out := runListFixture(t, dir)

	var payload struct {
		Issues []map[string]any `json:"issues"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	byID := map[string]map[string]any{}
	for _, iss := range payload.Issues {
		id, _ := iss["id"].(string)
		byID[id] = iss
	}

	epic := byID["epic-1"]
	if epic == nil {
		t.Fatalf("epic-1 missing from compact output")
	}
	if c, _ := epic["children_count"].(float64); c != 1 {
		t.Errorf("epic-1.children_count = %v, want 1", c)
	}

	child := byID["child-1"]
	if child == nil {
		t.Fatalf("child-1 missing from compact output")
	}
	if pid, _ := child["parent_id"].(string); pid != "epic-1" {
		t.Errorf("child-1.parent_id = %q, want epic-1", pid)
	}
	if bc, _ := child["blockers_count"].(float64); bc != 1 {
		t.Errorf("child-1.blockers_count = %v, want 1", bc)
	}
	if rc, _ := child["relates_count"].(float64); rc != 1 {
		t.Errorf("child-1.relates_count = %v, want 1", rc)
	}
	// blk-1 is closed, so child-1.is_blocked must be false.
	if blocked, _ := child["is_blocked"].(bool); blocked {
		t.Errorf("child-1.is_blocked = true, want false (blocker is closed)")
	}
	if cs, _ := child["created_by_session"].(string); cs != "cc-sess-a" {
		t.Errorf("child-1.created_by_session = %q, want cc-sess-a", cs)
	}
}

// TestRobotListFullIssuesKeys captures the keys present in a --full compact
// regression: when this test starts asserting different keys, it tells us
// the full-mode wire shape has drifted.
func TestRobotListFullIssuesKeys(t *testing.T) {
	dir := setupListFixture(t)
	out := runListFixture(t, dir, "--full")

	var payload struct {
		Issues []map[string]any `json:"issues"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(payload.Issues) == 0 {
		t.Fatalf("expected issues in --full output")
	}

	// Collect union of keys across all issues.
	keys := map[string]struct{}{}
	for _, iss := range payload.Issues {
		for k := range iss {
			keys[k] = struct{}{}
		}
	}

	required := []string{"id", "title", "status", "priority", "issue_type", "description", "created_at", "updated_at"}
	for _, k := range required {
		if _, has := keys[k]; !has {
			var sorted []string
			for k := range keys {
				sorted = append(sorted, k)
			}
			sort.Strings(sorted)
			t.Errorf("--full issue missing key %q; saw keys=%v", k, sorted)
		}
	}
}
