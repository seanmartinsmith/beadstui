package loader

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// parseJSONL is a small test helper to parse issues from JSONL data.
func parseJSONL(data []byte) ([]model.Issue, error) {
	return ParseIssues(bytes.NewReader(data))
}

// setupTestGitRepo creates a temporary git repo with beads files
func setupTestGitRepo(t *testing.T) (string, func()) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "git-loader-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	// Initialize git repo
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@test.com")
	runGit(t, tmpDir, "config", "user.name", "Test User")

	// Create .beads directory and initial file
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		cleanup()
		t.Fatalf("failed to create .beads dir: %v", err)
	}

	// Write initial beads file
	initialContent := `{"id":"ISSUE-1","title":"First issue","status":"open","priority":1,"issue_type":"task"}
{"id":"ISSUE-2","title":"Second issue","status":"open","priority":2,"issue_type":"task"}
`
	beadsFile := filepath.Join(beadsDir, "beads.base.jsonl")
	if err := os.WriteFile(beadsFile, []byte(initialContent), 0644); err != nil {
		cleanup()
		t.Fatalf("failed to write beads file: %v", err)
	}

	// Commit initial state
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "Initial commit")
	// Ensure subsequent commits have a distinct timestamp for deterministic date-based resolution
	time.Sleep(1500 * time.Millisecond)

	// Add a third issue in second commit
	updatedContent := `{"id":"ISSUE-1","title":"First issue","status":"open","priority":1,"issue_type":"task"}
{"id":"ISSUE-2","title":"Second issue","status":"open","priority":2,"issue_type":"task"}
{"id":"ISSUE-3","title":"Third issue","status":"open","priority":3,"issue_type":"task"}
`
	if err := os.WriteFile(beadsFile, []byte(updatedContent), 0644); err != nil {
		cleanup()
		t.Fatalf("failed to update beads file: %v", err)
	}

	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "Add third issue")

	return tmpDir, cleanup
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
	}
	return string(out)
}

func TestNewGitLoader(t *testing.T) {
	loader := NewGitLoader("/some/path")
	if loader.repoPath != "/some/path" {
		t.Errorf("expected repoPath /some/path, got %s", loader.repoPath)
	}
	if loader.cache == nil {
		t.Error("cache should not be nil")
	}
}

func TestNewGitLoaderWithCacheTTL(t *testing.T) {
	ttl := 10 * time.Minute
	loader := NewGitLoaderWithCacheTTL("/some/path", ttl)
	if loader.cache.maxAge != ttl {
		t.Errorf("expected cache TTL %v, got %v", ttl, loader.cache.maxAge)
	}
}

func TestGitLoader_LoadAt_HEAD(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	loader := NewGitLoader(repoDir)
	issues, err := loader.LoadAt("HEAD")
	if err != nil {
		t.Fatalf("LoadAt(HEAD) failed: %v", err)
	}

	if len(issues) != 3 {
		t.Errorf("expected 3 issues at HEAD, got %d", len(issues))
	}
}

func TestGitLoader_LoadAt_OlderCommit(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	loader := NewGitLoader(repoDir)
	issues, err := loader.LoadAt("HEAD~1")
	if err != nil {
		t.Fatalf("LoadAt(HEAD~1) failed: %v", err)
	}

	if len(issues) != 2 {
		t.Errorf("expected 2 issues at HEAD~1, got %d", len(issues))
	}
}

func TestGitLoader_ResolveRevision(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	loader := NewGitLoader(repoDir)

	// HEAD should resolve to a SHA
	sha, err := loader.ResolveRevision("HEAD")
	if err != nil {
		t.Fatalf("ResolveRevision(HEAD) failed: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("expected 40-char SHA, got %d chars: %s", len(sha), sha)
	}
}

func TestGitLoader_ResolveRevision_DateString(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	loader := NewGitLoader(repoDir)

	// Get author date of the first commit (HEAD~1) and ensure date resolution returns that SHA
	dateStr := runGitOutput(t, repoDir, "log", "--format=%aI", "-n1", "HEAD~1")
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		t.Fatalf("expected non-empty author date")
	}

	expectedSHA := strings.TrimSpace(runGitOutput(t, repoDir, "rev-parse", "HEAD~1"))

	sha, err := loader.ResolveRevision(dateStr)
	if err != nil {
		t.Fatalf("ResolveRevision(date) failed: %v", err)
	}

	if sha != expectedSHA {
		t.Fatalf("expected SHA %s for date %s, got %s", expectedSHA, dateStr, sha)
	}
}

func TestParseDateStringUsesLocalForDateOnly(t *testing.T) {
	dateStr := "2025-01-02"
	tm, ok := parseDateString(dateStr)
	if !ok {
		t.Fatalf("expected parseDateString to parse date-only string")
	}
	if tm.Location() != time.Local {
		t.Fatalf("expected location to be time.Local, got %v", tm.Location())
	}
}

func TestRevisionCacheExpires(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Use a generous TTL/sleep gap to avoid timing flake on slow clocks.
	loader := NewGitLoaderWithCacheTTL(repoDir, 50*time.Millisecond)

	// First load populates cache
	if _, err := loader.LoadAt("HEAD"); err != nil {
		t.Fatalf("LoadAt failed: %v", err)
	}
	if stats := loader.CacheStats(); stats.ValidEntries != 1 {
		t.Fatalf("expected 1 valid cache entry, got %d", stats.ValidEntries)
	}

	// Wait long enough for entry to expire
	time.Sleep(120 * time.Millisecond)

	// Cache should report zero valid entries, and LoadAt should still succeed (re-fetch)
	if stats := loader.CacheStats(); stats.ValidEntries != 0 {
		t.Fatalf("expected cache entry to expire, got %d valid", stats.ValidEntries)
	}
	if _, err := loader.LoadAt("HEAD"); err != nil {
		t.Fatalf("LoadAt after expiry failed: %v", err)
	}
}

func TestGetCommitsBetween(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Add a third commit touching beads file
	beadsFile := filepath.Join(repoDir, ".beads", "beads.base.jsonl")
	updated := `{"id":"ISSUE-1","title":"First issue","status":"open","priority":1,"issue_type":"task"}
{"id":"ISSUE-2","title":"Second issue","status":"open","priority":2,"issue_type":"task"}
{"id":"ISSUE-3","title":"Third issue","status":"open","priority":3,"issue_type":"task","assignee":"bob"}
`
	if err := os.WriteFile(beadsFile, []byte(updated), 0644); err != nil {
		t.Fatalf("update beads file: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "Assign third issue")

	loader := NewGitLoader(repoDir)

	// from first commit to HEAD should include the two newer commits
	fromSHA := strings.TrimSpace(runGitOutput(t, repoDir, "rev-parse", "HEAD~2"))
	revs, err := loader.GetCommitsBetween(fromSHA, "HEAD")
	if err != nil {
		t.Fatalf("GetCommitsBetween failed: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("expected 2 commits between first and HEAD, got %d", len(revs))
	}
	if revs[0].Message == "" || revs[1].Message == "" {
		t.Fatalf("expected commit messages to be populated: %+v", revs)
	}
}

func TestGitLoader_Cache(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	loader := NewGitLoader(repoDir)

	// First load - should populate cache
	issues1, err := loader.LoadAt("HEAD")
	if err != nil {
		t.Fatalf("first LoadAt failed: %v", err)
	}

	// Check cache stats
	stats := loader.CacheStats()
	if stats.ValidEntries != 1 {
		t.Errorf("expected 1 valid cache entry, got %d", stats.ValidEntries)
	}

	// Second load - should hit cache
	issues2, err := loader.LoadAt("HEAD")
	if err != nil {
		t.Fatalf("second LoadAt failed: %v", err)
	}

	if len(issues1) != len(issues2) {
		t.Error("cached and non-cached results differ")
	}
}

func TestGitLoader_ClearCache(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	loader := NewGitLoader(repoDir)

	// Load to populate cache
	_, err := loader.LoadAt("HEAD")
	if err != nil {
		t.Fatalf("LoadAt failed: %v", err)
	}

	// Clear cache
	loader.ClearCache()

	stats := loader.CacheStats()
	if stats.TotalEntries != 0 {
		t.Errorf("expected 0 entries after clear, got %d", stats.TotalEntries)
	}
}

func TestGitLoader_ListRevisions(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	loader := NewGitLoader(repoDir)

	revisions, err := loader.ListRevisions(10)
	if err != nil {
		t.Fatalf("ListRevisions failed: %v", err)
	}

	// We made 2 commits that touched beads files
	if len(revisions) != 2 {
		t.Errorf("expected 2 revisions, got %d", len(revisions))
	}

	// Revisions should be in reverse chronological order
	if len(revisions) >= 2 {
		if revisions[0].Message != "Add third issue" {
			t.Errorf("expected newest commit message 'Add third issue', got %q", revisions[0].Message)
		}
		if revisions[1].Message != "Initial commit" {
			t.Errorf("expected oldest commit message 'Initial commit', got %q", revisions[1].Message)
		}
	}
}

func TestGitLoader_HasBeadsAtRevision(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	loader := NewGitLoader(repoDir)

	// Should have beads at HEAD
	exists, err := loader.HasBeadsAtRevision("HEAD")
	if err != nil {
		t.Fatalf("HasBeadsAtRevision failed: %v", err)
	}
	if !exists {
		t.Error("expected beads to exist at HEAD")
	}
}

func TestGitLoader_InvalidRevision(t *testing.T) {
	repoDir, cleanup := setupTestGitRepo(t)
	defer cleanup()

	loader := NewGitLoader(repoDir)

	_, err := loader.LoadAt("nonexistent-branch")
	if err == nil {
		t.Error("expected error for invalid revision")
	}
}

func TestParseJSONL(t *testing.T) {
	data := []byte(`{"id":"TEST-1","title":"Test","status":"open","priority":1,"issue_type":"task"}
{"id":"TEST-2","title":"Test 2","status":"closed","priority":2,"issue_type":"task"}
`)
	issues, err := parseJSONL(data)
	if err != nil {
		t.Fatalf("parseJSONL failed: %v", err)
	}

	if len(issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(issues))
	}

	if issues[0].ID != "TEST-1" {
		t.Errorf("expected first issue ID TEST-1, got %s", issues[0].ID)
	}
}

func TestParseJSONL_SkipsMalformed(t *testing.T) {
	data := []byte(`{"id":"GOOD-1","title":"Good","status":"open","priority":1,"issue_type":"task"}
{this is not valid json}
{"id":"GOOD-2","title":"Good 2","status":"open","priority":2,"issue_type":"task"}
`)
	issues, err := parseJSONL(data)
	if err != nil {
		t.Fatalf("parseJSONL failed: %v", err)
	}

	// Should skip the malformed line
	if len(issues) != 2 {
		t.Errorf("expected 2 valid issues, got %d", len(issues))
	}
}

func TestParseJSONL_EmptyLines(t *testing.T) {
	data := []byte(`{"id":"TEST-1","title":"Test","status":"open","priority":1,"issue_type":"task"}

{"id":"TEST-2","title":"Test 2","status":"open","priority":2,"issue_type":"task"}

`)
	issues, err := parseJSONL(data)
	if err != nil {
		t.Fatalf("parseJSONL failed: %v", err)
	}

	if len(issues) != 2 {
		t.Errorf("expected 2 issues (empty lines skipped), got %d", len(issues))
	}
}

func TestCacheExpiry(t *testing.T) {
	// Use a very short TTL for testing
	loader := NewGitLoaderWithCacheTTL("/unused", 1*time.Millisecond)

	// Manually add to cache
	loader.cache.set("abc123", nil)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	// Should return no valid entries
	stats := loader.CacheStats()
	if stats.ValidEntries != 0 {
		t.Errorf("expected 0 valid entries after expiry, got %d", stats.ValidEntries)
	}
}
