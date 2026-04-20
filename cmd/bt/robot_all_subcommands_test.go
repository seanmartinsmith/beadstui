package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupDiffFixture builds a fixture project for contract tests that need
// to exercise `bt robot diff` against a historical commit.
func setupDiffFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Seed commit with the initial three issues.
	initial := `{"id":"epic-1","title":"Epic","status":"open","priority":0,"issue_type":"epic","description":"initial epic","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
{"id":"child-1","title":"Child","description":"initial body","status":"open","priority":1,"issue_type":"task","dependencies":[{"issue_id":"child-1","depends_on_id":"epic-1","type":"parent-child"}],"created_at":"2026-04-16T10:00:00Z","updated_at":"2026-04-16T10:00:00Z"}
`
	beadsFile := filepath.Join(beadsDir, "beads.jsonl")
	if err := os.WriteFile(beadsFile, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial beads: %v", err)
	}

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\nout=%s", args, err, out)
		}
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	runGit("init", "--quiet")
	runGit("add", "-A")
	runGit("commit", "-m", "initial", "--quiet")

	// Second commit: add, close, modify, reopen to populate all diff slots.
	mutated := `{"id":"epic-1","title":"Epic","status":"open","priority":0,"issue_type":"epic","description":"initial epic","created_at":"2026-04-15T10:00:00Z","updated_at":"2026-04-15T10:00:00Z"}
{"id":"child-1","title":"Child","description":"modified body","status":"closed","priority":1,"issue_type":"task","dependencies":[{"issue_id":"child-1","depends_on_id":"epic-1","type":"parent-child"}],"created_at":"2026-04-16T10:00:00Z","updated_at":"2026-04-20T10:00:00Z","closed_at":"2026-04-20T10:00:00Z"}
{"id":"child-2","title":"New child","description":"fresh body","status":"open","priority":1,"issue_type":"task","dependencies":[{"issue_id":"child-2","depends_on_id":"epic-1","type":"parent-child"}],"created_at":"2026-04-20T10:00:00Z","updated_at":"2026-04-20T10:00:00Z"}
`
	if err := os.WriteFile(beadsFile, []byte(mutated), 0o644); err != nil {
		t.Fatalf("write mutated beads: %v", err)
	}
	runGit("add", "-A")
	runGit("commit", "-m", "mutate", "--quiet")
	return dir
}

// TestRobotSubcommandsAcceptShapeFlag verifies that every touched robot
// subcommand accepts --shape/--compact/--full without erroring at flag
// parse time. For subcommands that require a bead id or path argument, a
// dummy value is supplied — we don't care about the data outcome, only
// that the shape flag integrates with the persistent flag set.
func TestRobotSubcommandsAcceptShapeFlag(t *testing.T) {
	dir := setupListFixture(t)
	exe := buildTestBinary(t)

	cases := []struct {
		name string
		args []string
	}{
		{"list", []string{"robot", "list", "--limit=1"}},
		{"triage", []string{"robot", "triage"}},
		{"next", []string{"robot", "next"}},
		{"insights", []string{"robot", "insights"}},
		{"plan", []string{"robot", "plan"}},
		{"priority", []string{"robot", "priority"}},
		{"alerts", []string{"robot", "alerts"}},
		{"search", []string{"robot", "search", "epic"}},
		{"suggest", []string{"robot", "suggest"}},
		{"drift", []string{"robot", "drift"}},
		{"blocker-chain", []string{"robot", "blocker-chain", "child-1"}},
		{"impact-network", []string{"robot", "impact-network", "child-1"}},
		{"causality", []string{"robot", "causality", "child-1"}},
		{"related", []string{"robot", "related", "child-1"}},
		{"impact", []string{"robot", "impact", "README.md"}},
		{"orphans", []string{"robot", "orphans"}},
		{"portfolio", []string{"robot", "portfolio"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name+"/compact", func(t *testing.T) {
			runAccepts(t, exe, dir, append(append([]string{}, tc.args...), "--compact"))
		})
		t.Run(tc.name+"/full", func(t *testing.T) {
			runAccepts(t, exe, dir, append(append([]string{}, tc.args...), "--full"))
		})
		t.Run(tc.name+"/shape-compact", func(t *testing.T) {
			runAccepts(t, exe, dir, append(append([]string{}, tc.args...), "--shape=compact"))
		})
		t.Run(tc.name+"/shape-full", func(t *testing.T) {
			runAccepts(t, exe, dir, append(append([]string{}, tc.args...), "--shape=full"))
		})
	}
}

// runAccepts runs the given subcommand and asserts it does NOT fail with
// a cobra flag-parse error. Non-zero exits from missing data (e.g., no
// search index built) are tolerated; flag errors are not.
func runAccepts(t *testing.T, exe, dir string, args []string) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1", "BT_EMBED_PROVIDER=stub")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Distinguish flag errors (unacceptable) from runtime errors
		// (acceptable for this test's scope).
		combined := strings.ToLower(string(out))
		if strings.Contains(combined, "unknown flag") ||
			strings.Contains(combined, "invalid argument") ||
			strings.Contains(combined, "flag needs an argument") ||
			strings.Contains(combined, "--compact and --full are mutually exclusive") {
			t.Fatalf("flag parse error for %v: %v\n%s", args, err, out)
		}
	}
	_ = errors.New // keep import footprint stable across refactors
}

// TestRobotDiffCompactDefault verifies that `bt robot diff` emits the
// compact shape by default, populates the schema envelope field, and
// excludes fat body fields from all four issue-slot slices.
func TestRobotDiffCompactDefault(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := setupDiffFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "diff", "--since=HEAD~1")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("diff failed: %v\nout=%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse diff: %v\nraw=%s", err, out)
	}
	if schema, _ := payload["schema"].(string); schema != "compact.v1" {
		t.Errorf("envelope.schema = %q, want compact.v1", schema)
	}
	diff, ok := payload["diff"].(map[string]any)
	if !ok {
		t.Fatalf("diff is not an object: %T", payload["diff"])
	}

	forbidden := []string{"description", "design", "acceptance_criteria", "notes", "comments", "close_reason"}
	slots := []string{"new_issues", "closed_issues", "removed_issues", "reopened_issues"}
	total := 0
	for _, slot := range slots {
		raw, ok := diff[slot].([]any)
		if !ok || len(raw) == 0 {
			continue
		}
		total += len(raw)
		for i, item := range raw {
			obj, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("diff.%s[%d] not an object", slot, i)
			}
			for _, k := range forbidden {
				if _, has := obj[k]; has {
					t.Errorf("diff.%s[%d].%s leaked into compact output", slot, i, k)
				}
			}
		}
	}
	if total == 0 {
		t.Errorf("diff fixture produced no issue-slot entries; check fixture setup")
	}
}

// TestRobotDiffFullRestoresBodies — regression: --full keeps the original
// SnapshotDiff wire shape and fat body fields come through.
func TestRobotDiffFullRestoresBodies(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	dir := setupDiffFixture(t)
	exe := buildTestBinary(t)

	cmd := exec.Command(exe, "robot", "diff", "--since=HEAD~1", "--full")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BT_TEST_MODE=1", "BT_NO_BROWSER=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("diff --full failed: %v\nout=%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, has := payload["schema"]; has {
		t.Errorf("envelope.schema must be omitted in full mode, got %v", payload["schema"])
	}

	diff := payload["diff"].(map[string]any)
	closed, _ := diff["closed_issues"].([]any)
	foundBody := false
	for _, item := range closed {
		obj, _ := item.(map[string]any)
		if d, ok := obj["description"].(string); ok && strings.Contains(d, "modified body") {
			foundBody = true
		}
	}
	if !foundBody {
		t.Errorf("expected description bodies in --full diff output")
	}
}
