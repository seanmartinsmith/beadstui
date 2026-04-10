package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPriorityBrief_AndAgentBriefBundle(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// A is actionable, B is blocked; ensures triage has content.
	writeBeads(t, env, `{"id":"A","title":"Unblocker","status":"open","priority":1,"issue_type":"task"}
{"id":"B","title":"Blocked","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"B","depends_on_id":"A","type":"blocks"}]}`)

	briefPath := filepath.Join(env, "brief.md")
	cmd := exec.Command(bv, "--priority-brief", briefPath)
	cmd.Dir = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--priority-brief failed: %v\n%s", err, out)
	}

	briefBytes, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("read brief.md: %v", err)
	}
	brief := string(briefBytes)
	if !strings.Contains(brief, "# 📊 Priority Brief") {
		t.Fatalf("brief missing header:\n%s", brief)
	}
	if !strings.Contains(brief, "**Hash:** `") {
		t.Fatalf("brief missing hash line:\n%s", brief)
	}
	if !strings.Contains(brief, "**A**") {
		t.Fatalf("brief missing issue A:\n%s", brief)
	}

	// Agent brief bundle
	bundleDir := filepath.Join(env, "agent-brief")
	cmd = exec.Command(bv, "--agent-brief", bundleDir)
	cmd.Dir = env
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--agent-brief failed: %v\n%s", err, out)
	}

	for _, name := range []string{"triage.json", "insights.json", "brief.md", "helpers.md", "meta.json"} {
		if _, err := os.Stat(filepath.Join(bundleDir, name)); err != nil {
			t.Fatalf("bundle missing %s: %v", name, err)
		}
	}

	metaBytes, err := os.ReadFile(filepath.Join(bundleDir, "meta.json"))
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}
	var meta struct {
		DataHash string   `json:"data_hash"`
		Files    []string `json:"files"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("meta.json decode: %v", err)
	}
	if meta.DataHash == "" {
		t.Fatalf("meta.json missing data_hash")
	}
	if len(meta.Files) == 0 {
		t.Fatalf("meta.json missing files list")
	}
}
