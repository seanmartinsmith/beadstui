package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writeIssuesJSONL(t *testing.T, repoDir, content string) {
	t.Helper()
	beadsDir := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}
}

func TestRobotTriage_MalformedIssuesLine_NoStderr(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := t.TempDir()

	issues := `{"id":"A","title":"Alpha","status":"open","priority":1,"issue_type":"task"}` + "\n" +
		`{this is not json}` + "\n"
	writeIssuesJSONL(t, repoDir, issues)

	cmd := exec.Command(bv, "--robot-triage")
	cmd.Dir = repoDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("--robot-triage failed: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr; got:\n%s", got)
	}

	var payload struct {
		DataHash string `json:"data_hash"`
		Triage   any    `json:"triage"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json decode: %v\nout=%s", err, stdout.String())
	}
	if payload.DataHash == "" {
		t.Fatalf("expected data_hash to be set")
	}
	if payload.Triage == nil {
		t.Fatalf("expected triage payload to be present")
	}
}

func TestRobotSprintList_MalformedSprintLine_NoStderr(t *testing.T) {
	bv := buildBvBinary(t)
	sprints := `{"id":"sprint-1","name":"Sprint 1","bead_ids":["A"]}` + "\n" +
		`{this is not json}` + "\n"
	repoDir := createSprintRepo(t, sprints)

	cmd := exec.Command(bv, "--robot-sprint-list")
	cmd.Dir = repoDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("--robot-sprint-list failed: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr; got:\n%s", got)
	}

	var payload struct {
		SprintCount int `json:"sprint_count"`
		Sprints     []struct {
			ID string `json:"id"`
		} `json:"sprints"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json decode: %v\nout=%s", err, stdout.String())
	}
	if payload.SprintCount != 1 {
		t.Fatalf("expected sprint_count=1, got %d", payload.SprintCount)
	}
	if len(payload.Sprints) != 1 || payload.Sprints[0].ID != "sprint-1" {
		t.Fatalf("expected sprint-1 only, got %+v", payload.Sprints)
	}
}
