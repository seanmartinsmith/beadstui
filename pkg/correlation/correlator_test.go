package correlation

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestBuildHistories_Empty(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	histories := c.buildHistories(nil, nil, nil)

	if len(histories) != 0 {
		t.Errorf("expected empty histories, got %d", len(histories))
	}
}

func TestBuildHistories_Basic(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	beads := []BeadInfo{
		{ID: "bv-1", Title: "Task 1", Status: "open"},
		{ID: "bv-2", Title: "Task 2", Status: "closed"},
	}

	now := time.Now()
	events := []BeadEvent{
		{BeadID: "bv-1", EventType: EventCreated, Timestamp: now.Add(-24 * time.Hour), Author: "Alice"},
		{BeadID: "bv-1", EventType: EventClaimed, Timestamp: now.Add(-12 * time.Hour), Author: "Alice"},
		{BeadID: "bv-2", EventType: EventCreated, Timestamp: now.Add(-48 * time.Hour), Author: "Bob"},
		{BeadID: "bv-2", EventType: EventClosed, Timestamp: now.Add(-1 * time.Hour), Author: "Bob"},
	}

	histories := c.buildHistories(beads, events, nil)

	if len(histories) != 2 {
		t.Errorf("expected 2 histories, got %d", len(histories))
	}

	h1 := histories["bv-1"]
	if len(h1.Events) != 2 {
		t.Errorf("expected 2 events for bv-1, got %d", len(h1.Events))
	}
	if h1.Milestones.Created == nil {
		t.Error("expected bv-1 to have created milestone")
	}
	if h1.Milestones.Claimed == nil {
		t.Error("expected bv-1 to have claimed milestone")
	}

	h2 := histories["bv-2"]
	if len(h2.Events) != 2 {
		t.Errorf("expected 2 events for bv-2, got %d", len(h2.Events))
	}
	if h2.CycleTime == nil {
		t.Error("expected bv-2 to have cycle time (closed bead)")
	}
}

func TestBuildCommitIndex(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	histories := map[string]BeadHistory{
		"bv-1": {
			BeadID: "bv-1",
			Commits: []CorrelatedCommit{
				{SHA: "abc123", Method: MethodCoCommitted},
				{SHA: "def456", Method: MethodCoCommitted},
			},
		},
		"bv-2": {
			BeadID: "bv-2",
			Commits: []CorrelatedCommit{
				{SHA: "abc123", Method: MethodCoCommitted}, // Same commit, different bead
				{SHA: "ghi789", Method: MethodCoCommitted},
			},
		},
	}

	index := c.buildCommitIndex(histories)

	if len(index) != 3 {
		t.Errorf("expected 3 unique commits in index, got %d", len(index))
	}

	// abc123 should reference both beads
	if len(index["abc123"]) != 2 {
		t.Errorf("expected abc123 to reference 2 beads, got %d", len(index["abc123"]))
	}
}

func TestCalculateStats_Empty(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	stats := c.calculateStats(make(map[string]BeadHistory), nil)

	if stats.TotalBeads != 0 {
		t.Errorf("expected 0 total beads, got %d", stats.TotalBeads)
	}
	if stats.BeadsWithCommits != 0 {
		t.Errorf("expected 0 beads with commits, got %d", stats.BeadsWithCommits)
	}
}

func TestCalculateStats_WithData(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	claimToClose := 24 * time.Hour
	histories := map[string]BeadHistory{
		"bv-1": {
			BeadID: "bv-1",
			Events: []BeadEvent{
				{Author: "Alice"},
			},
			Commits: []CorrelatedCommit{
				{SHA: "abc123", Author: "Alice", Method: MethodCoCommitted},
			},
			CycleTime: &CycleTime{ClaimToClose: &claimToClose},
		},
		"bv-2": {
			BeadID: "bv-2",
			Events: []BeadEvent{
				{Author: "Bob"},
			},
			Commits: []CorrelatedCommit{
				{SHA: "def456", Author: "Bob", Method: MethodExplicitID},
			},
		},
	}

	stats := c.calculateStats(histories, nil)

	if stats.TotalBeads != 2 {
		t.Errorf("expected 2 total beads, got %d", stats.TotalBeads)
	}
	if stats.BeadsWithCommits != 2 {
		t.Errorf("expected 2 beads with commits, got %d", stats.BeadsWithCommits)
	}
	if stats.TotalCommits != 2 {
		t.Errorf("expected 2 total commits, got %d", stats.TotalCommits)
	}
	if stats.UniqueAuthors != 2 {
		t.Errorf("expected 2 unique authors, got %d", stats.UniqueAuthors)
	}
	if stats.MethodDistribution["co_committed"] != 1 {
		t.Errorf("expected 1 co_committed, got %d", stats.MethodDistribution["co_committed"])
	}
	if stats.MethodDistribution["explicit_id"] != 1 {
		t.Errorf("expected 1 explicit_id, got %d", stats.MethodDistribution["explicit_id"])
	}
	if stats.AvgCycleTimeDays == nil {
		t.Error("expected avg cycle time to be set")
	}
}

func TestDescribeGitRange(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	tests := []struct {
		name     string
		opts     CorrelatorOptions
		expected string
	}{
		{
			name:     "no filters",
			opts:     CorrelatorOptions{},
			expected: "all history",
		},
		{
			name:     "with limit",
			opts:     CorrelatorOptions{Limit: 100},
			expected: "limit 100 commits",
		},
		{
			name: "with since",
			opts: func() CorrelatorOptions {
				since := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
				return CorrelatorOptions{Since: &since}
			}(),
			expected: "since 2024-01-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.describeGitRange(tt.opts)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCalculateDataHash(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	beads1 := []BeadInfo{
		{ID: "bv-1", Status: "open"},
		{ID: "bv-2", Status: "closed"},
	}

	beads2 := []BeadInfo{
		{ID: "bv-1", Status: "open"},
		{ID: "bv-2", Status: "open"}, // Different status
	}

	hash1 := c.calculateDataHash(beads1)
	hash2 := c.calculateDataHash(beads2)

	if hash1 == hash2 {
		t.Error("different bead data should produce different hashes")
	}

	// Same data should produce same hash
	hash1Again := c.calculateDataHash(beads1)
	if hash1 != hash1Again {
		t.Error("same bead data should produce same hash")
	}
}

func TestDedupCommits(t *testing.T) {
	commits := []CorrelatedCommit{
		{SHA: "abc123", Message: "First"},
		{SHA: "def456", Message: "Second"},
		{SHA: "abc123", Message: "First duplicate"}, // Duplicate SHA
		{SHA: "ghi789", Message: "Third"},
	}

	result := dedupCommits(commits)

	if len(result) != 3 {
		t.Errorf("expected 3 unique commits, got %d", len(result))
	}

	// First occurrence should be kept
	if result[0].Message != "First" {
		t.Errorf("expected first commit message to be 'First', got %s", result[0].Message)
	}
}

func TestNewCorrelator(t *testing.T) {
	c := NewCorrelator("/tmp/test")
	if c.repoPath != "/tmp/test" {
		t.Errorf("repoPath = %s, want /tmp/test", c.repoPath)
	}
	if c.extractor == nil {
		t.Error("extractor should not be nil")
	}
	if c.coCommitter == nil {
		t.Error("coCommitter should not be nil")
	}
}

func TestValidateRepository_NoGitDir(t *testing.T) {
	err := ValidateRepository("/nonexistent/path")
	if err == nil {
		t.Error("ValidateRepository should fail for nonexistent path")
	}
}

func TestValidateRepository_NoBeadsFile(t *testing.T) {
	// Use temp dir that exists but has no beads
	err := ValidateRepository("/tmp")
	if err == nil {
		t.Error("ValidateRepository should fail without beads file")
	}
}

func TestFindLatestCommitSHA_Empty(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	sha := c.findLatestCommitSHA(nil, nil)
	if sha != "" {
		t.Errorf("findLatestCommitSHA with empty inputs should return empty, got %s", sha)
	}
}

func TestFindLatestCommitSHA_FromEvents(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	now := time.Now()
	events := []BeadEvent{
		{CommitSHA: "older", Timestamp: now.Add(-1 * time.Hour)},
		{CommitSHA: "newest", Timestamp: now},
		{CommitSHA: "middle", Timestamp: now.Add(-30 * time.Minute)},
	}

	sha := c.findLatestCommitSHA(events, nil)
	if sha != "newest" {
		t.Errorf("findLatestCommitSHA = %s, want 'newest'", sha)
	}
}

func TestFindLatestCommitSHA_FromCommits(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	now := time.Now()
	commits := []CorrelatedCommit{
		{SHA: "commit_old", Timestamp: now.Add(-1 * time.Hour)},
		{SHA: "commit_newest", Timestamp: now},
	}

	sha := c.findLatestCommitSHA(nil, commits)
	if sha != "commit_newest" {
		t.Errorf("findLatestCommitSHA = %s, want 'commit_newest'", sha)
	}
}

func TestFindLatestCommitSHA_Mixed(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	now := time.Now()
	events := []BeadEvent{
		{CommitSHA: "event_sha", Timestamp: now.Add(-1 * time.Hour)},
	}
	commits := []CorrelatedCommit{
		{SHA: "commit_sha", Timestamp: now}, // This is newer
	}

	sha := c.findLatestCommitSHA(events, commits)
	if sha != "commit_sha" {
		t.Errorf("findLatestCommitSHA = %s, want 'commit_sha' (newer)", sha)
	}
}

func TestBuildHistories_WithCommits(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	beads := []BeadInfo{
		{ID: "bv-1", Title: "Task 1", Status: "in_progress"},
	}

	now := time.Now()
	events := []BeadEvent{
		{BeadID: "bv-1", EventType: EventClaimed, Timestamp: now, CommitSHA: "abc123"},
	}

	commits := []CorrelatedCommit{
		{SHA: "abc123", Author: "Test Author", BeadID: "bv-1"},
	}

	histories := c.buildHistories(beads, events, commits)

	h := histories["bv-1"]
	if len(h.Commits) != 1 {
		t.Errorf("expected 1 commit, got %d", len(h.Commits))
	}
	if h.LastAuthor != "Test Author" {
		t.Errorf("LastAuthor = %s, want 'Test Author'", h.LastAuthor)
	}
}

func TestCalculateStats_AvgCommitsPerBead(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	histories := map[string]BeadHistory{
		"bv-1": {
			Commits: []CorrelatedCommit{{SHA: "a1"}, {SHA: "a2"}},
		},
		"bv-2": {
			Commits: []CorrelatedCommit{{SHA: "b1"}},
		},
		"bv-3": {
			Commits: nil, // No commits
		},
	}

	stats := c.calculateStats(histories, nil)

	// 3 commits / 2 beads with commits = 1.5
	if stats.AvgCommitsPerBead != 1.5 {
		t.Errorf("AvgCommitsPerBead = %v, want 1.5", stats.AvgCommitsPerBead)
	}
}

func TestDescribeGitRange_Combined(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	opts := CorrelatorOptions{
		Since: &since,
		Until: &until,
		Limit: 100,
	}

	result := c.describeGitRange(opts)

	if result != "since 2024-01-01, until 2024-12-31, limit 100 commits" {
		t.Errorf("unexpected result: %s", result)
	}
}

// TestGenerateReport_RepoStatus_OutsideGit verifies that GenerateReport
// records RepoStatus.InsideWorkTree=false when the configured path is not
// a git work tree (bt-ezk8). Consumers (specifically the History view
// empty-state renderer) rely on this to distinguish "no commits" from
// "no git context."
func TestGenerateReport_RepoStatus_OutsideGit(t *testing.T) {
	tmp := t.TempDir()
	if hasGitParent(tmp) {
		t.Skipf("temp dir %s is inside a git repo; skipping", tmp)
	}
	c := NewCorrelator(tmp)
	beads := []BeadInfo{{ID: "bt-1", Title: "Test", Status: "open"}}

	report, err := c.GenerateReport(beads, CorrelatorOptions{})
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}
	if report.RepoStatus.InsideWorkTree {
		t.Errorf("expected InsideWorkTree=false outside a git repo, got true")
	}
	if report.RepoStatus.RepoPath != tmp {
		t.Errorf("expected RepoPath=%q, got %q", tmp, report.RepoStatus.RepoPath)
	}
}

// TestGenerateReport_RepoStatus_InsideGit verifies the in-repo branch
// records InsideWorkTree=true (bt-ezk8). The correlation package itself
// lives inside a git tree, so we use os.Getwd().
func TestGenerateReport_RepoStatus_InsideGit(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	c := NewCorrelator(wd)
	beads := []BeadInfo{{ID: "bt-1", Title: "Test", Status: "open"}}

	report, err := c.GenerateReport(beads, CorrelatorOptions{Limit: 1})
	if err != nil {
		t.Fatalf("GenerateReport returned error: %v", err)
	}
	if !report.RepoStatus.InsideWorkTree {
		t.Errorf("expected InsideWorkTree=true inside a git repo, got false")
	}
}

// initBareGitRepo bootstraps a minimal git repo in a fresh tempdir for
// dispatch tests: init, configure identity, create one empty commit so git
// log can run without erroring. Returns the absolute path (EvalSymlinks
// normalized so Windows short-name forms don't trip path equality checks).
// Skips the calling test if git is not on PATH.
func initBareGitRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git not available or %v failed: %v: %s", args, err, out)
		}
	}
	return dir
}

// seedJSONLOnDisk creates the .beads/ directory and a beads.jsonl file in
// repoPath so HasJSONLOnDisk(repoPath) returns true. Content is not parsed;
// the dispatcher only cares about the file's on-disk presence.
func seedJSONLOnDisk(t *testing.T, repoPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repoPath, ".beads"), 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".beads", "beads.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}
}

// newPoisonDoltDB returns a *sql.DB that has been Close()'d -- any query
// against it errors. Used as a sentinel in TestCorrelator_Dispatch_JSONLPath:
// if the JSONL branch in extractEvents is removed, control falls through to
// the Dolt branch which would query this DB and surface the error. A green
// test proves the JSONL branch was taken.
func newPoisonDoltDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite: %v", err)
	}
	return db
}

// TestCorrelator_Dispatch_JSONLPath covers branch 1 of extractEvents:
// jsonlTracked=true should route to the JSONL+git-diff extractor, never
// touching the Dolt DB even when one was wired in via NewCorrelatorWithDolt.
//
// Mutation protection: the doltDB is closed before GenerateReport runs. If
// dispatch removes the JSONLTracked check, control falls into the Dolt
// branch which queries the closed DB and produces a non-nil error. This
// test passes only when the JSONL branch fires first.
func TestCorrelator_Dispatch_JSONLPath(t *testing.T) {
	repo := initBareGitRepo(t)
	seedJSONLOnDisk(t, repo)

	c := NewCorrelatorWithDolt(repo, newPoisonDoltDB(t))
	report, err := c.GenerateReport(nil, CorrelatorOptions{})
	if err != nil {
		t.Fatalf("GenerateReport returned error (would happen if Dolt branch ran instead of JSONL): %v", err)
	}
	if !report.RepoStatus.InsideWorkTree {
		t.Errorf("InsideWorkTree = false, want true")
	}
	if !report.RepoStatus.JSONLTracked {
		t.Errorf("JSONLTracked = false, want true (JSONL is on disk)")
	}
}

// TestCorrelator_Dispatch_DoltOnlyPath covers branch 2 of extractEvents:
// jsonlTracked=false + non-nil doltDB should route to the Dolt-native
// extractor. The seeded events should appear in the report's histories with
// empty CommitSHA (the Dolt extractor's signature per 592c).
//
// Mutation protection: removing the Dolt branch would return nil events;
// the assertion on Histories["bt-1"].Events being non-empty fails. Replacing
// it with the JSONL path would call git log -- .beads/*.jsonl on a repo
// with no JSONL on disk -- the extractor returns empty, also failing the
// non-empty assertion.
func TestCorrelator_Dispatch_DoltOnlyPath(t *testing.T) {
	repo := initBareGitRepo(t)
	// Deliberately no seedJSONLOnDisk: this is the Dolt-only scenario.

	db := newDoltEventsFixture(t)
	t0 := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	insertEvent(t, db, "events", "bt-1", "created", "sms", "", "", "", t0)
	insertEvent(t, db, "events", "bt-1", "status_changed", "sms", "open", "in_progress", "", t0.Add(time.Hour))

	c := NewCorrelatorWithDolt(repo, db)
	beads := []BeadInfo{{ID: "bt-1", Title: "Test", Status: "in_progress"}}

	report, err := c.GenerateReport(beads, CorrelatorOptions{})
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}
	if !report.RepoStatus.InsideWorkTree {
		t.Errorf("InsideWorkTree = false, want true")
	}
	if report.RepoStatus.JSONLTracked {
		t.Errorf("JSONLTracked = true, want false (no JSONL on disk -> Dolt-only)")
	}

	h, ok := report.Histories["bt-1"]
	if !ok {
		t.Fatalf("Histories missing bt-1")
	}
	if len(h.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2 (Dolt extractor should populate)", len(h.Events))
	}
	for i, e := range h.Events {
		if e.CommitSHA != "" {
			t.Errorf("Events[%d].CommitSHA = %q, want empty (Dolt-extractor signature per 592c)", i, e.CommitSHA)
		}
	}
}

// TestCorrelator_Dispatch_DoltOnly_NoDB covers branch 3 of extractEvents:
// jsonlTracked=false + nil doltDB (caller used plain NewCorrelator on a
// Dolt-only repo) should return cleanly with an empty events list -- never
// error, never construct a nil-DB DoltExtractor.
//
// Mutation protection: removing the c.doltDB == nil guard would fall
// through to NewDoltExtractor(nil).Extract(...) which the bt-08sh.1
// nil-handle guard turns into a "dolt extractor: nil database handle"
// error. A green test confirms the guard is in place.
func TestCorrelator_Dispatch_DoltOnly_NoDB(t *testing.T) {
	repo := initBareGitRepo(t)
	// No JSONL on disk, no doltDB wired in.

	c := NewCorrelator(repo)
	beads := []BeadInfo{{ID: "bt-1", Title: "Test", Status: "open"}}

	report, err := c.GenerateReport(beads, CorrelatorOptions{})
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}
	if !report.RepoStatus.InsideWorkTree {
		t.Errorf("InsideWorkTree = false, want true")
	}
	if report.RepoStatus.JSONLTracked {
		t.Errorf("JSONLTracked = true, want false")
	}

	h, ok := report.Histories["bt-1"]
	if !ok {
		t.Fatalf("Histories missing bt-1")
	}
	if len(h.Events) != 0 {
		t.Errorf("len(Events) = %d, want 0 (no extractor wired)", len(h.Events))
	}
}

// TestHasJSONLOnDisk exercises the detection helper that drives dispatch.
// On-disk presence (not git-log history) is the canonical check -- bt-ydjw
// phase 1 verified that git-log on a repo where .beads/beads.jsonl was
// deleted in an earlier commit still returns a hit, producing the false
// positive this helper is designed to avoid.
func TestHasJSONLOnDisk(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		if HasJSONLOnDisk("") {
			t.Errorf("HasJSONLOnDisk(\"\") = true, want false")
		}
	})

	t.Run("no .beads directory", func(t *testing.T) {
		dir := t.TempDir()
		if HasJSONLOnDisk(dir) {
			t.Errorf("HasJSONLOnDisk(empty dir) = true, want false")
		}
	})

	for _, name := range []string{"beads.jsonl", "issues.jsonl", "beads.base.jsonl"} {
		t.Run("recognizes "+name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, ".beads", name), nil, 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
			if !HasJSONLOnDisk(dir) {
				t.Errorf("HasJSONLOnDisk with %s on disk = false, want true", name)
			}
		})
	}

	t.Run("empty .beads directory is not enough", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if HasJSONLOnDisk(dir) {
			t.Errorf("HasJSONLOnDisk with empty .beads/ = true, want false")
		}
	})
}
