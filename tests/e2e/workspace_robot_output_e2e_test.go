package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceRobotTriageCleanOutput(t *testing.T) {
	bv := buildBvBinary(t)

	workspaceRoot := t.TempDir()
	configPath := filepath.Join(workspaceRoot, ".bv", "workspace.yaml")

	// Create two repos with issues.
	apiBeadsDir := filepath.Join(workspaceRoot, "services", "api", ".beads")
	webBeadsDir := filepath.Join(workspaceRoot, "apps", "web", ".beads")
	if err := os.MkdirAll(apiBeadsDir, 0o755); err != nil {
		t.Fatalf("mkdir api beads: %v", err)
	}
	if err := os.MkdirAll(webBeadsDir, 0o755); err != nil {
		t.Fatalf("mkdir web beads: %v", err)
	}

	apiIssues := `{"id":"AUTH-1","title":"API auth","status":"open","priority":1,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(apiBeadsDir, "issues.jsonl"), []byte(apiIssues+"\n"), 0o644); err != nil {
		t.Fatalf("write api issues.jsonl: %v", err)
	}

	// Cross-repo dependency references must already be namespaced.
	webIssues := `{"id":"UI-1","title":"Web UI","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"UI-1","depends_on_id":"api-AUTH-1","type":"blocks"}]}`
	if err := os.WriteFile(filepath.Join(webBeadsDir, "issues.jsonl"), []byte(webIssues+"\n"), 0o644); err != nil {
		t.Fatalf("write web issues.jsonl: %v", err)
	}

	config := `
name: test-workspace
repos:
  - name: api
    path: services/api
    prefix: api-
  - name: web
    path: apps/web
    prefix: web-
discovery:
  enabled: false
`
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir .bv: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write workspace.yaml: %v", err)
	}

	cmd := exec.Command(bv, "--robot-triage", "--workspace", configPath)
	cmd.Dir = workspaceRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("--robot-triage --workspace failed: %v\nstderr=%s\nstdout=%s", err, stderr.String(), stdout.String())
	}
	if got := strings.TrimSpace(stderr.String()); got != "" {
		t.Fatalf("expected empty stderr for robot JSON, got: %s", got)
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON on stdout: %v\nstdout=%s", err, stdout.String())
	}
	if _, ok := payload["generated_at"]; !ok {
		t.Fatalf("missing generated_at")
	}
	if _, ok := payload["triage"]; !ok {
		t.Fatalf("missing triage")
	}
}
