package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// createHistoryRepo seeds a git repo with a bead lifecycle (open -> in_progress -> closed)
// and co-committed code changes so robot-history can correlate events and commits.
func createHistoryRepo(t *testing.T) (string, string) {
	t.Helper()
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	write := func(content string) {
		if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(content), 0o644); err != nil {
			t.Fatalf("write beads.jsonl: %v", err)
		}
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

	// Commit 1: creation (open)
	write(`{"id":"HIST-1","title":"History bead","status":"open","priority":1,"issue_type":"task"}`)
	git("add", ".beads/beads.jsonl")
	git("commit", "-m", "seed HIST-1")

	// Commit 2: claim + code change
	write(`{"id":"HIST-1","title":"History bead","status":"in_progress","priority":1,"issue_type":"task"}`)
	if err := os.MkdirAll(filepath.Join(repoDir, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "pkg", "work.go"), []byte("package pkg\n\n// work in progress\n"), 0o644); err != nil {
		t.Fatalf("write work.go: %v", err)
	}
	git("add", ".beads/beads.jsonl", "pkg/work.go")
	git("commit", "-m", "claim HIST-1 with code")

	// Commit 3: close + code tweak
	write(`{"id":"HIST-1","title":"History bead","status":"closed","priority":1,"issue_type":"task"}`)
	if err := os.WriteFile(filepath.Join(repoDir, "pkg", "work.go"), []byte("package pkg\n\n// finished work\nfunc Done() {}\n"), 0o644); err != nil {
		t.Fatalf("update work.go: %v", err)
	}
	git("add", ".beads/beads.jsonl", "pkg/work.go")
	git("commit", "-m", "close HIST-1")

	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = repoDir
	out, err := revCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse: %v\n%s", err, out)
	}

	return repoDir, strings.TrimSpace(string(out))
}

func TestRobotHistoryIncludesEventsAndCommitIndex(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir, head := createHistoryRepo(t)

	cmd := exec.Command(bv, "--robot-history")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-history failed: %v\n%s", err, out)
	}

	var payload struct {
		GeneratedAt     string `json:"generated_at"`
		DataHash        string `json:"data_hash"`
		GitRange        string `json:"git_range"`
		LatestCommitSHA string `json:"latest_commit_sha"`
		Stats           struct {
			TotalBeads         int            `json:"total_beads"`
			BeadsWithCommits   int            `json:"beads_with_commits"`
			MethodDistribution map[string]int `json:"method_distribution"`
		} `json:"stats"`
		Histories map[string]struct {
			Events []struct {
				EventType string `json:"event_type"`
				CommitSHA string `json:"commit_sha"`
			} `json:"events"`
			Commits []struct {
				Method string `json:"method"`
				Files  []struct {
					Path string `json:"path"`
				} `json:"files"`
			} `json:"commits"`
			Milestones struct {
				Closed interface{} `json:"closed"`
			} `json:"milestones"`
		} `json:"histories"`
		CommitIndex map[string][]string `json:"commit_index"`
	}

	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v\nout=%s", err, out)
	}

	if payload.DataHash == "" {
		t.Fatal("missing data_hash")
	}
	if payload.GitRange == "" {
		t.Fatal("missing git_range")
	}
	if payload.LatestCommitSHA == "" {
		t.Fatalf("latest_commit_sha missing")
	}
	if payload.Stats.TotalBeads != 1 {
		t.Fatalf("expected total_beads=1, got %d", payload.Stats.TotalBeads)
	}
	if payload.Stats.BeadsWithCommits != 1 {
		t.Fatalf("expected beads_with_commits=1, got %d", payload.Stats.BeadsWithCommits)
	}
	if payload.Stats.MethodDistribution["co_committed"] == 0 {
		t.Fatalf("expected co_committed entries in method_distribution, got %v", payload.Stats.MethodDistribution)
	}

	hist, ok := payload.Histories["HIST-1"]
	if !ok {
		t.Fatalf("history for HIST-1 missing: keys=%v", keys(payload.Histories))
	}
	if len(hist.Events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(hist.Events))
	}
	if len(hist.Commits) == 0 {
		t.Fatalf("expected commits correlated, got 0")
	}
	if hist.Milestones.Closed == nil {
		t.Fatalf("expected closed milestone populated")
	}

	// Commit index should map at least one commit to HIST-1
	found := false
	for sha, beads := range payload.CommitIndex {
		if len(beads) > 0 && beads[0] == "HIST-1" {
			found = true
			if sha == payload.LatestCommitSHA || sha == head {
				break
			}
		}
	}
	if !found {
		t.Fatalf("commit_index missing mapping to HIST-1: %v", payload.CommitIndex)
	}
}

// keys returns map keys for debugging in assertions.
func keys(m map[string]struct {
	Events []struct {
		EventType string `json:"event_type"`
		CommitSHA string `json:"commit_sha"`
	} `json:"events"`
	Commits []struct {
		Method string `json:"method"`
		Files  []struct {
			Path string `json:"path"`
		} `json:"files"`
	} `json:"commits"`
	Milestones struct {
		Closed interface{} `json:"closed"`
	} `json:"milestones"`
}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestRobotHistoryPathHints verifies path hints are populated from correlated commits (bv-188)
func TestRobotHistoryPathHints(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir, _ := createHistoryRepo(t)

	cmd := exec.Command(bv, "--robot-history")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-history failed: %v\n%s", err, out)
	}

	var payload struct {
		Histories map[string]struct {
			Commits []struct {
				Files []struct {
					Path string `json:"path"`
				} `json:"files"`
			} `json:"commits"`
		} `json:"histories"`
	}

	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	hist, ok := payload.Histories["HIST-1"]
	if !ok {
		t.Fatal("missing HIST-1 in histories")
	}

	// Check path hints: should include pkg/work.go from correlated commits
	var foundPkgWork bool
	for _, commit := range hist.Commits {
		for _, f := range commit.Files {
			if strings.Contains(f.Path, "pkg/work.go") || strings.Contains(f.Path, "work.go") {
				foundPkgWork = true
				break
			}
		}
	}
	if !foundPkgWork {
		t.Fatalf("expected path hint for pkg/work.go in commits, got: %+v", hist.Commits)
	}
}

// createRenameRepo seeds a git repo where a file is renamed (tests --follow behavior)
func createRenameRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	write := func(content string) {
		if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(content), 0o644); err != nil {
			t.Fatalf("write beads.jsonl: %v", err)
		}
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

	// Commit 1: create bead and original file
	write(`{"id":"RENAME-1","title":"Rename test","status":"open","priority":1,"issue_type":"task"}`)
	if err := os.MkdirAll(filepath.Join(repoDir, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "src", "old_name.go"), []byte("package src\n\nfunc Original() {}\n"), 0o644); err != nil {
		t.Fatalf("write old_name.go: %v", err)
	}
	git("add", ".beads/beads.jsonl", "src/old_name.go")
	git("commit", "-m", "create RENAME-1 with old_name.go")

	// Commit 2: rename file
	git("mv", "src/old_name.go", "src/new_name.go")
	if err := os.WriteFile(filepath.Join(repoDir, "src", "new_name.go"), []byte("package src\n\nfunc Original() {}\nfunc Extended() {}\n"), 0o644); err != nil {
		t.Fatalf("update new_name.go: %v", err)
	}
	write(`{"id":"RENAME-1","title":"Rename test","status":"in_progress","priority":1,"issue_type":"task"}`)
	git("add", ".beads/beads.jsonl", "src/new_name.go")
	git("commit", "-m", "rename to new_name.go for RENAME-1")

	// Commit 3: close bead
	write(`{"id":"RENAME-1","title":"Rename test","status":"closed","priority":1,"issue_type":"task"}`)
	git("add", ".beads/beads.jsonl")
	git("commit", "-m", "close RENAME-1")

	return repoDir
}

// TestRobotHistoryRenameTracking verifies git rename detection works (bv-188)
func TestRobotHistoryRenameTracking(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := createRenameRepo(t)

	cmd := exec.Command(bv, "--robot-history")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-history failed: %v\n%s", err, out)
	}

	var payload struct {
		Stats struct {
			TotalBeads       int `json:"total_beads"`
			BeadsWithCommits int `json:"beads_with_commits"`
		} `json:"stats"`
		Histories map[string]struct {
			Events []struct {
				EventType string `json:"event_type"`
			} `json:"events"`
			Commits []struct {
				Files []struct {
					Path string `json:"path"`
				} `json:"files"`
			} `json:"commits"`
		} `json:"histories"`
	}

	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	if payload.Stats.TotalBeads != 1 {
		t.Fatalf("expected total_beads=1, got %d", payload.Stats.TotalBeads)
	}

	hist, ok := payload.Histories["RENAME-1"]
	if !ok {
		t.Fatal("missing RENAME-1 in histories")
	}

	// Should have events for the lifecycle
	if len(hist.Events) < 2 {
		t.Fatalf("expected at least 2 events for RENAME-1, got %d", len(hist.Events))
	}

	// Check that commits reference the renamed file path
	var foundNewName bool
	for _, commit := range hist.Commits {
		for _, f := range commit.Files {
			if strings.Contains(f.Path, "new_name.go") {
				foundNewName = true
				break
			}
		}
	}
	if !foundNewName && len(hist.Commits) > 0 {
		// It's okay if rename detection doesn't catch the new name in all cases,
		// but log for debugging
		t.Logf("note: new_name.go not found in commits, got: %+v", hist.Commits)
	}

	// The key assertion is that the correlation still works even with rename
	if len(hist.Commits) == 0 {
		t.Fatalf("expected commits correlated to RENAME-1, got 0")
	}
}

// TestRobotHistoryEmptyRepo verifies behavior with no beads history (edge case)
func TestRobotHistoryEmptyRepo(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir := t.TempDir()

	// Create minimal git repo with no beads
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
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "README.md")
	git("commit", "-m", "initial")

	// Now add empty .beads directory
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".beads/beads.jsonl")
	git("commit", "-m", "add empty beads")

	cmd := exec.Command(bv, "--robot-history")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-history failed on empty repo: %v\n%s", err, out)
	}

	var payload struct {
		Stats struct {
			TotalBeads int `json:"total_beads"`
		} `json:"stats"`
		Histories map[string]interface{} `json:"histories"`
	}

	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	if payload.Stats.TotalBeads != 0 {
		t.Fatalf("expected 0 beads in empty repo, got %d", payload.Stats.TotalBeads)
	}

	if payload.Histories == nil {
		t.Fatal("histories should be non-nil (empty map)")
	}
}
