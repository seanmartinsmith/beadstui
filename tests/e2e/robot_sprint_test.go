package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func createSprintRepo(t *testing.T, sprintsJSONL string) string {
	t.Helper()
	repoDir := t.TempDir()
	beadsDir := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	// Minimal beads file so bv can start.
	const beads = `{"id":"A","title":"Alpha","status":"open","priority":1,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}

	if sprintsJSONL != "" {
		if err := os.WriteFile(filepath.Join(beadsDir, "sprints.jsonl"), []byte(sprintsJSONL), 0o644); err != nil {
			t.Fatalf("write sprints.jsonl: %v", err)
		}
	}

	return repoDir
}

func TestRobotSprintList_Empty(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createSprintRepo(t, "")

	cmd := exec.Command(bv, "--robot-sprint-list")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-sprint-list failed: %v\n%s", err, out)
	}
	t.Logf("--robot-sprint-list output:\n%s", out)

	var payload struct {
		GeneratedAt string `json:"generated_at"`
		SprintCount int    `json:"sprint_count"`
		Sprints     []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"sprints"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v\nout=%s", err, out)
	}
	if payload.GeneratedAt == "" {
		t.Fatalf("expected generated_at to be set")
	}
	if payload.SprintCount != 0 {
		t.Fatalf("expected sprint_count=0, got %d", payload.SprintCount)
	}
	if len(payload.Sprints) != 0 {
		t.Fatalf("expected 0 sprints, got %d", len(payload.Sprints))
	}
}

func TestRobotSprintList_Multiple(t *testing.T) {
	bv := buildBvBinary(t)
	sprints := `{"id":"sprint-1","name":"Sprint 1","bead_ids":["A"]}` + "\n" +
		`{"id":"sprint-2","name":"Sprint 2","bead_ids":["A"]}` + "\n"
	repoDir := createSprintRepo(t, sprints)
	t.Logf("sprints.jsonl:\n%s", sprints)

	cmd := exec.Command(bv, "--robot-sprint-list")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-sprint-list failed: %v\n%s", err, out)
	}
	t.Logf("--robot-sprint-list output:\n%s", out)

	var payload struct {
		SprintCount int `json:"sprint_count"`
		Sprints     []struct {
			ID string `json:"id"`
		} `json:"sprints"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v\nout=%s", err, out)
	}
	if payload.SprintCount != 2 {
		t.Fatalf("expected sprint_count=2, got %d", payload.SprintCount)
	}
	if len(payload.Sprints) != 2 {
		t.Fatalf("expected 2 sprints, got %d", len(payload.Sprints))
	}
}

func TestRobotSprintShow_Found(t *testing.T) {
	bv := buildBvBinary(t)
	sprints := `{"id":"sprint-1","name":"Sprint 1","bead_ids":["A"]}` + "\n"
	repoDir := createSprintRepo(t, sprints)

	cmd := exec.Command(bv, "--robot-sprint-show", "sprint-1")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-sprint-show failed: %v\n%s", err, out)
	}
	t.Logf("--robot-sprint-show output:\n%s", out)

	var payload struct {
		GeneratedAt string `json:"generated_at"`
		DataHash    string `json:"data_hash"`
		Sprint      struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"sprint"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v\nout=%s", err, out)
	}
	if payload.GeneratedAt == "" {
		t.Fatalf("expected generated_at to be set")
	}
	if payload.DataHash == "" {
		t.Fatalf("expected data_hash to be set")
	}
	if payload.Sprint.ID != "sprint-1" {
		t.Fatalf("expected sprint.id sprint-1, got %q", payload.Sprint.ID)
	}
	if payload.Sprint.Name != "Sprint 1" {
		t.Fatalf("expected sprint.name Sprint 1, got %q", payload.Sprint.Name)
	}
}

func TestRobotSprintShow_NotFound(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createSprintRepo(t, "")

	cmd := exec.Command(bv, "--robot-sprint-show", "missing")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected --robot-sprint-show to fail for missing sprint")
	}
	t.Logf("--robot-sprint-show missing output:\n%s", out)
	if string(out) == "" {
		t.Fatalf("expected error output for missing sprint")
	}
}
