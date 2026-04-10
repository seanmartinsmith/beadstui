package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRobotBurndownIncludesScopeChanges(t *testing.T) {
	bv := buildBvBinary(t)

	repoDir := t.TempDir()
	beadsDir := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	issues := `{"id":"A","title":"Alpha","status":"open","priority":1,"issue_type":"task"}
{"id":"B","title":"Beta","status":"open","priority":2,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(issues), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour).Format(time.RFC3339)
	end := now.Add(24 * time.Hour).Format(time.RFC3339)

	sprintV1 := `{"id":"sprint-1","name":"Sprint 1","start_date":"` + start + `","end_date":"` + end + `","bead_ids":["A"]}`
	if err := os.WriteFile(filepath.Join(beadsDir, "sprints.jsonl"), []byte(sprintV1+"\n"), 0o644); err != nil {
		t.Fatalf("write sprints v1: %v", err)
	}

	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	git("init")
	git("add", ".beads/beads.jsonl", ".beads/sprints.jsonl")
	git("commit", "-m", "init sprint")

	// Commit 2: add B to sprint scope.
	sprintV2 := `{"id":"sprint-1","name":"Sprint 1","start_date":"` + start + `","end_date":"` + end + `","bead_ids":["A","B"]}`
	if err := os.WriteFile(filepath.Join(beadsDir, "sprints.jsonl"), []byte(sprintV2+"\n"), 0o644); err != nil {
		t.Fatalf("write sprints v2: %v", err)
	}
	git("add", ".beads/sprints.jsonl")
	git("commit", "-m", "add B to sprint")

	// Commit 3: remove A from sprint scope.
	sprintV3 := `{"id":"sprint-1","name":"Sprint 1","start_date":"` + start + `","end_date":"` + end + `","bead_ids":["B"]}`
	if err := os.WriteFile(filepath.Join(beadsDir, "sprints.jsonl"), []byte(sprintV3+"\n"), 0o644); err != nil {
		t.Fatalf("write sprints v3: %v", err)
	}
	git("add", ".beads/sprints.jsonl")
	git("commit", "-m", "remove A from sprint")

	cmd := exec.Command(bv, "--robot-burndown", "sprint-1")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-burndown failed: %v\n%s", err, out)
	}

	var payload struct {
		SprintID     string `json:"sprint_id"`
		ScopeChanges []struct {
			IssueID string `json:"issue_id"`
			Action  string `json:"action"`
		} `json:"scope_changes"`
		GeneratedAt string `json:"generated_at"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v\nout=%s", err, out)
	}
	if payload.SprintID != "sprint-1" {
		t.Fatalf("sprint_id=%q; want %q", payload.SprintID, "sprint-1")
	}
	if payload.GeneratedAt == "" {
		t.Fatalf("missing generated_at")
	}

	// Expect two scope changes: B added, then A removed (chronological).
	if len(payload.ScopeChanges) != 2 {
		t.Fatalf("scope_changes=%d; want 2; got=%+v", len(payload.ScopeChanges), payload.ScopeChanges)
	}
	if payload.ScopeChanges[0].IssueID != "B" || payload.ScopeChanges[0].Action != "added" {
		t.Fatalf("first scope change=%+v; want B added", payload.ScopeChanges[0])
	}
	if payload.ScopeChanges[1].IssueID != "A" || payload.ScopeChanges[1].Action != "removed" {
		t.Fatalf("second scope change=%+v; want A removed", payload.ScopeChanges[1])
	}
}
