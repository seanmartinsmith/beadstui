package correlation

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestIsCodeFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// Code files
		{"pkg/auth/login.go", true},
		{"src/app.py", true},
		{"index.js", true},
		{"app.tsx", true},
		{"main.rs", true},
		{"App.java", true},
		{"config.yaml", true},
		{"data.json", true},
		{"README.md", true},
		{"schema.sql", true},
		{"script.sh", true},

		// Non-code files
		{"image.png", false},
		{"photo.jpg", false},
		{"document.pdf", false},
		{"archive.zip", false},
		{"binary.exe", false},
		{"data.csv", false},

		// Edge cases
		{"Makefile", false}, // No extension
		{".gitignore", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isCodeFile(tt.path)
			if got != tt.want {
				t.Errorf("isCodeFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsExcludedPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// Excluded paths
		{".beads/beads.jsonl", true},
		{".beads/issues.jsonl", true},
		{".bt/hooks.yaml", true},
		{".git/objects/abc", true},
		{"node_modules/lodash/index.js", true},
		{"vendor/github.com/pkg/errors/errors.go", true},
		{"__pycache__/module.pyc", true},
		{".venv/lib/python3.9/site.py", true},

		// Not excluded
		{"pkg/auth/login.go", false},
		{"src/components/Button.tsx", false},
		{"cmd/main.go", false},
		{"internal/service/user.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isExcludedPath(tt.path)
			if got != tt.want {
				t.Errorf("isExcludedPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestContainsBeadID(t *testing.T) {
	tests := []struct {
		text   string
		beadID string
		want   bool
	}{
		{"fix: resolve issue bv-123", "bv-123", true},
		{"feat(auth): implement login for BV-123", "bv-123", true}, // Case insensitive
		{"chore: update deps", "bv-123", false},
		{"", "bv-123", false},
		{"some text", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := containsBeadID(tt.text, tt.beadID)
			if got != tt.want {
				t.Errorf("containsBeadID(%q, %q) = %v, want %v", tt.text, tt.beadID, got, tt.want)
			}
		})
	}
}

func TestAllTestFiles(t *testing.T) {
	tests := []struct {
		name  string
		files []FileChange
		want  bool
	}{
		{
			name:  "empty list",
			files: []FileChange{},
			want:  false,
		},
		{
			name: "all go tests",
			files: []FileChange{
				{Path: "pkg/auth/login_test.go"},
				{Path: "pkg/auth/session_test.go"},
			},
			want: true,
		},
		{
			name: "all js tests",
			files: []FileChange{
				{Path: "src/app.test.js"},
				{Path: "src/utils.spec.ts"},
			},
			want: true,
		},
		{
			name: "mixed files",
			files: []FileChange{
				{Path: "pkg/auth/login.go"},
				{Path: "pkg/auth/login_test.go"},
			},
			want: false,
		},
		{
			name: "no test files",
			files: []FileChange{
				{Path: "pkg/auth/login.go"},
				{Path: "pkg/auth/session.go"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allTestFiles(tt.files)
			if got != tt.want {
				t.Errorf("allTestFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShortSHA(t *testing.T) {
	tests := []struct {
		sha  string
		want string
	}{
		{"abc123def456789012345678901234567890abcd", "abc123d"},
		{"abc123", "abc123"},
		{"abc", "abc"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.sha, func(t *testing.T) {
			got := shortSHA(tt.sha)
			if got != tt.want {
				t.Errorf("shortSHA(%q) = %q, want %q", tt.sha, got, tt.want)
			}
		})
	}
}

func TestExtractNewPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Simple rename
		{"old.go => new.go", "new.go"},
		// With braces
		{"pkg/{old => new}/file.go", "pkg/new/file.go"},
		// Complex braces
		{"{old => new}.go", "new.go"},
		// No rename
		{"regular/path.go", "regular/path.go"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractNewPath(tt.input)
			if got != tt.want {
				t.Errorf("extractNewPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCalculateConfidence(t *testing.T) {
	c := NewCoCommitExtractor("/test/repo")
	now := time.Now()

	tests := []struct {
		name      string
		event     BeadEvent
		files     []FileChange
		wantRange [2]float64 // [min, max] expected range
	}{
		{
			name: "base case",
			event: BeadEvent{
				BeadID:    "bv-123",
				CommitMsg: "fix: some bug",
			},
			files: []FileChange{
				{Path: "file.go"},
			},
			wantRange: [2]float64{0.94, 0.96}, // ~0.95
		},
		{
			name: "commit mentions bead ID",
			event: BeadEvent{
				BeadID:    "bv-123",
				CommitMsg: "fix: resolve bv-123",
			},
			files: []FileChange{
				{Path: "file.go"},
			},
			wantRange: [2]float64{0.98, 1.0}, // 0.95 + 0.04 = 0.99
		},
		{
			name: "shotgun commit",
			event: BeadEvent{
				BeadID:    "bv-123",
				CommitMsg: "refactor: big change",
			},
			files:     make([]FileChange, 25), // >20 files
			wantRange: [2]float64{0.84, 0.86}, // 0.95 - 0.10 = 0.85
		},
		{
			name: "only test files",
			event: BeadEvent{
				BeadID:    "bv-123",
				CommitMsg: "test: add tests",
			},
			files: []FileChange{
				{Path: "auth_test.go"},
				{Path: "user_test.go"},
			},
			wantRange: [2]float64{0.89, 0.91}, // 0.95 - 0.05 = 0.90
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.event.Timestamp = now
			got := c.calculateConfidence(tt.event, tt.files)
			if got < tt.wantRange[0] || got > tt.wantRange[1] {
				t.Errorf("calculateConfidence() = %v, want in range [%v, %v]", got, tt.wantRange[0], tt.wantRange[1])
			}
		})
	}
}

func TestGenerateReason(t *testing.T) {
	c := NewCoCommitExtractor("/test/repo")

	event := BeadEvent{
		BeadID:    "bv-123",
		EventType: EventClosed,
		CommitMsg: "fix: resolve bv-123",
	}

	files := []FileChange{{Path: "file.go"}}

	reason := c.generateReason(event, files, 0.99)

	if reason == "" {
		t.Error("reason should not be empty")
	}

	// Should mention the event type
	if !strings.Contains(reason, "closed") {
		t.Errorf("reason should mention event type, got: %s", reason)
	}

	// Should mention bead ID reference
	if !strings.Contains(reason, "bead ID") {
		t.Errorf("reason should mention bead ID reference, got: %s", reason)
	}
}

func TestCreateCorrelatedCommit(t *testing.T) {
	c := NewCoCommitExtractor("/test/repo")
	now := time.Now()

	event := BeadEvent{
		BeadID:      "bv-123",
		EventType:   EventClosed,
		Timestamp:   now,
		CommitSHA:   "abc123def456",
		CommitMsg:   "fix: close bv-123",
		Author:      "Test User",
		AuthorEmail: "test@example.com",
	}

	files := []FileChange{
		{Path: "pkg/auth/login.go", Action: "M", Insertions: 10, Deletions: 5},
	}

	commit := c.CreateCorrelatedCommit(event, files)

	if commit.SHA != event.CommitSHA {
		t.Errorf("SHA mismatch: got %s, want %s", commit.SHA, event.CommitSHA)
	}
	if commit.ShortSHA != "abc123d" {
		t.Errorf("ShortSHA mismatch: got %s", commit.ShortSHA)
	}
	if commit.Method != MethodCoCommitted {
		t.Errorf("Method should be MethodCoCommitted, got %s", commit.Method)
	}
	if commit.Confidence < 0.9 {
		t.Errorf("Confidence should be high for bead ID mention, got %v", commit.Confidence)
	}
	if len(commit.Files) != 1 {
		t.Errorf("Files count mismatch: got %d, want 1", len(commit.Files))
	}
	if commit.Author != event.Author {
		t.Errorf("Author mismatch: got %s, want %s", commit.Author, event.Author)
	}
}

func TestNewCoCommitExtractor(t *testing.T) {
	c := NewCoCommitExtractor("/tmp/test")
	if c.repoPath != "/tmp/test" {
		t.Errorf("repoPath = %s, want /tmp/test", c.repoPath)
	}
}

func TestExtractAllCoCommits_Empty(t *testing.T) {
	c := NewCoCommitExtractor("/tmp/test")

	// Empty events
	commits, err := c.ExtractAllCoCommits(nil)
	if err != nil {
		t.Fatalf("ExtractAllCoCommits(nil) failed: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("len(commits) = %d, want 0", len(commits))
	}
}

func TestExtractAllCoCommits_NonStatusEvents(t *testing.T) {
	c := NewCoCommitExtractor("/tmp/test")

	// Only non-status events (created, modified)
	events := []BeadEvent{
		{BeadID: "bv-1", EventType: EventCreated, CommitSHA: "abc"},
		{BeadID: "bv-2", EventType: EventModified, CommitSHA: "def"},
	}

	commits, err := c.ExtractAllCoCommits(events)
	if err != nil {
		t.Fatalf("ExtractAllCoCommits failed: %v", err)
	}
	// Should skip non-status events
	if len(commits) != 0 {
		t.Errorf("len(commits) = %d, want 0 (non-status events)", len(commits))
	}
}

// TestExtractAllCoCommits_EmptySHA_NoGitInvocation locks in the Dolt-only
// extraction path's contract (bt-08sh / bt-592c): when every event carries
// an empty CommitSHA, ExtractAllCoCommits must short-circuit without
// spawning a single git subprocess. Before the bt-08sh.2 fix, the per-event
// fallback inside the loop invoked `git show --name-status` with an empty
// SHA argument, which both wasted process spawns and produced parse noise.
//
// The assertion uses the gitCommand test seam to count invocations. We
// also verify the function returns cleanly with an empty result so the
// caller sees the expected events-only shape.
func TestExtractAllCoCommits_EmptySHA_NoGitInvocation(t *testing.T) {
	origGitCommand := gitCommand
	var gitCalls int
	gitCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" {
			gitCalls++
		}
		return origGitCommand(name, args...)
	}
	defer func() { gitCommand = origGitCommand }()

	c := NewCoCommitExtractor("/tmp/test")
	events := []BeadEvent{
		{BeadID: "bt-1", EventType: EventClaimed, CommitSHA: ""},
		{BeadID: "bt-2", EventType: EventClosed, CommitSHA: ""},
		{BeadID: "bt-3", EventType: EventClaimed, CommitSHA: ""},
	}

	commits, err := c.ExtractAllCoCommits(events)
	if err != nil {
		t.Fatalf("ExtractAllCoCommits returned error: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("len(commits) = %d, want 0 (all SHAs empty)", len(commits))
	}
	if gitCalls != 0 {
		t.Errorf("git subprocess invocations = %d, want 0 (no git work on empty-SHA input)", gitCalls)
	}
}

func TestGenerateReason_LargeCommit(t *testing.T) {
	c := NewCoCommitExtractor("/test/repo")

	event := BeadEvent{
		BeadID:    "bv-123",
		EventType: EventClaimed,
		CommitMsg: "big change",
	}

	// Create > 20 files to trigger large commit message
	files := make([]FileChange, 25)
	for i := range files {
		files[i] = FileChange{Path: "file" + string(rune('a'+i)) + ".go"}
	}

	reason := c.generateReason(event, files, 0.85)

	if !strings.Contains(reason, "large commit") {
		t.Errorf("reason should mention large commit, got: %s", reason)
	}
}

func TestGenerateReason_OnlyTestFiles(t *testing.T) {
	c := NewCoCommitExtractor("/test/repo")

	event := BeadEvent{
		BeadID:    "bv-123",
		EventType: EventClaimed,
		CommitMsg: "add tests",
	}

	files := []FileChange{
		{Path: "auth_test.go"},
		{Path: "login_test.go"},
	}

	reason := c.generateReason(event, files, 0.90)

	if !strings.Contains(reason, "test files") {
		t.Errorf("reason should mention test files, got: %s", reason)
	}
}

func TestCalculateConfidence_Combined(t *testing.T) {
	c := NewCoCommitExtractor("/test/repo")

	// Test combination: shotgun commit with bead ID mention
	event := BeadEvent{
		BeadID:    "bv-123",
		CommitMsg: "big refactor bv-123",
	}

	files := make([]FileChange, 30)
	for i := range files {
		files[i] = FileChange{Path: "file" + string(rune('a'+i)) + ".go"}
	}

	confidence := c.calculateConfidence(event, files)

	// Base 0.95 + 0.04 (bead ID) - 0.10 (shotgun) = 0.89
	if confidence < 0.88 || confidence > 0.90 {
		t.Errorf("Combined confidence = %v, expected ~0.89", confidence)
	}
}

func TestExtractNewPath_DoubleSlashBug(t *testing.T) {
	// Git output for renaming "pkg/old/file.go" to "pkg/file.go"
	// is "pkg/{old => }/file.go"
	input := "pkg/{old => }/file.go"

	// We expect "pkg/file.go"
	expected := "pkg/file.go"

	got := extractNewPath(input)

	if got != expected {
		t.Errorf("extractNewPath(%q) = %q; want %q", input, got, expected)
	}
}

// TestPrefetchCoCommittedFiles_ByteIdenticalToPerEvent verifies that the
// batched prefetch path (one git log per mode, regardless of SHA count)
// produces byte-identical FileChange output to the per-event path.
// This is the load-bearing acceptance criterion for bt-h01q -- without it,
// the perf win is meaningless because callers would observe behaviour drift.
//
// Strategy: build a hermetic fixture repo covering the diff shapes that
// matter (single-file commit, multi-file commit, rename, and a conflicted
// --no-ff merge whose combined diff is non-empty), run both paths on every
// SHA, assert equality. The fixture replaces the previous ambient-repo
// sampling, which was coupled to live git history and diverged across
// checkouts (bt-7qx80).
func TestPrefetchCoCommittedFiles_ByteIdenticalToPerEvent(t *testing.T) {
	dir, shas, mergeSHA := buildCoCommitFixtureRepo(t)

	c := NewCoCommitExtractor(dir)

	// Per-event reference path: call ExtractCoCommittedFiles on each SHA.
	want := make(map[string][]FileChange, len(shas))
	for _, sha := range shas {
		got, err := c.ExtractCoCommittedFiles(BeadEvent{CommitSHA: sha})
		if err != nil {
			t.Fatalf("ExtractCoCommittedFiles(%s): %v", sha, err)
		}
		want[sha] = got
	}

	// Batched path.
	got, err := c.prefetchCoCommittedFiles(shas)
	if err != nil {
		t.Fatalf("prefetchCoCommittedFiles: %v", err)
	}

	for _, sha := range shas {
		w := want[sha]
		g := got[sha]

		// Both paths legitimately produce no files for merge commits; treat
		// nil and empty as equivalent whenever both sides agree on emptiness.
		if len(w) == 0 && len(g) == 0 {
			continue
		}

		// Order within a single SHA should already match. But to be robust
		// against any output-ordering surprise on Windows, sort by Path
		// before comparing.
		sortByPath(w)
		sortByPath(g)

		if !reflect.DeepEqual(w, g) {
			t.Errorf("sha %s: mismatch\n  per-event: %#v\n  batched:   %#v", sha, w, g)
		}
	}

	// Pin the merge-commit semantic explicitly: merges contribute no
	// co-committed files on EITHER path (see getFilesChanged). If this ever
	// changes, it must change on both paths together.
	if n := len(want[mergeSHA]); n != 0 {
		t.Errorf("per-event path returned %d files for merge commit %s, want 0", n, shortSHA(mergeSHA))
	}
	if n := len(got[mergeSHA]); n != 0 {
		t.Errorf("batched path returned %d files for merge commit %s, want 0", n, shortSHA(mergeSHA))
	}
}

// buildCoCommitFixtureRepo builds a small hermetic git repo whose history
// covers the diff shapes the byte-identical contract must hold across:
// a seed commit, a multi-file commit, a rename, a side-branch commit, and a
// conflicted --no-ff merge (resolved by hand) whose combined diff is
// non-empty -- exactly the shape where `git show` (combined diff) and
// `git log --no-walk` (no diff) historically disagreed for merges (bt-7qx80).
// Files use code extensions so they survive the isCodeFile filter. Returns
// the repo dir, all commit SHAs oldest-first, and the merge commit's SHA.
func buildCoCommitFixtureRepo(t *testing.T) (dir string, shas []string, mergeSHA string) {
	t.Helper()

	dir = t.TempDir()

	git := func(args ...string) string {
		t.Helper()
		out, err := gitInFixture(dir, args...)
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return out
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	head := func() string {
		t.Helper()
		return git("rev-parse", "HEAD")
	}

	if out, err := gitInFixture(dir, "init", "-q", "-b", "main"); err != nil {
		t.Skipf("git not available or init failed: %v\n%s", err, out)
	}
	git("config", "user.name", "bt-test")
	git("config", "user.email", "bt-test@example.invalid")

	// A: seed commit.
	write("a.go", "package fixture\n")
	write("shared.go", "package fixture\n// base\n")
	git("add", ".")
	git("commit", "-q", "-m", "A: seed files")
	shas = append(shas, head())
	branchPoint := shas[0]

	// B: multi-file commit.
	write("a.go", "package fixture\n// edited\n")
	write("b.go", "package fixture\n// bee\n")
	git("add", ".")
	git("commit", "-q", "-m", "B: touch two files")
	shas = append(shas, head())

	// C (main): rename plus an edit to shared.go so main and side diverge.
	git("mv", "b.go", "renamed.go")
	write("shared.go", "package fixture\n// left\n")
	git("add", "shared.go")
	git("commit", "-q", "-m", "C: rename b.go, edit shared.go")
	shas = append(shas, head())

	// D (side): conflicting edit to shared.go.
	git("checkout", "-q", "-b", "side", branchPoint)
	write("shared.go", "package fixture\n// right\n")
	write("side.go", "package fixture\n// side\n")
	git("add", ".")
	git("commit", "-q", "-m", "D: side edit")
	shas = append(shas, head())

	// M: conflicted merge resolved by hand. Because the resolution differs
	// from both parents, `git show`'s combined diff for M is non-empty.
	git("checkout", "-q", "main")
	if out, err := gitInFixture(dir, "merge", "-q", "--no-ff", "--no-commit", "side"); err == nil {
		t.Fatalf("expected merge conflict on shared.go, got clean merge:\n%s", out)
	}
	write("shared.go", "package fixture\n// merged\n")
	git("add", "shared.go")
	git("commit", "-q", "--no-edit")
	mergeSHA = head()
	shas = append(shas, mergeSHA)

	return dir, shas, mergeSHA
}

// gitInFixture runs git in dir with a hermetic identity and config
// environment so fixture behavior does not depend on host git config.
func gitInFixture(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_AUTHOR_NAME=bt-test",
		"GIT_AUTHOR_EMAIL=bt-test@example.invalid",
		"GIT_COMMITTER_NAME=bt-test",
		"GIT_COMMITTER_EMAIL=bt-test@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// TestPrefetchCoCommittedFiles_Empty exercises the empty-input fast path.
func TestPrefetchCoCommittedFiles_Empty(t *testing.T) {
	c := NewCoCommitExtractor("/tmp/test")
	got, err := c.prefetchCoCommittedFiles(nil)
	if err != nil {
		t.Fatalf("prefetchCoCommittedFiles(nil): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

// TestIsCommitSHALine guards the parser's commit-boundary heuristic.
func TestIsCommitSHALine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"abc123def456789012345678901234567890abcd", true},
		{"ABC123DEF456789012345678901234567890ABCD", false}, // uppercase rejected (git emits lowercase)
		{"abc123def456789012345678901234567890abc", false},  // 39 chars
		{"abc123def456789012345678901234567890abcde", false}, // 41 chars
		{"M\tpath/to/file.go", false},
		{"10\t5\tpath/to/file.go", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := isCommitSHALine(tt.line); got != tt.want {
				t.Errorf("isCommitSHALine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// sortByPath sorts a FileChange slice by Path so reflect.DeepEqual is
// stable across paths whose ordering may vary between git invocations.
func sortByPath(files []FileChange) {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
}

func TestExtractNewPath_ComplexCases(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"{old => new}", "new"},
		{"src/{old => new}/main.go", "src/new/main.go"},
		{"src/{ => new}/main.go", "src/new/main.go"}, // Addition
		{"src/{old => }/main.go", "src/main.go"},     // Deletion - vulnerable case
		{"old => new", "new"},
	}

	for _, tc := range cases {
		got := extractNewPath(tc.input)
		if got != tc.expected {
			t.Errorf("extractNewPath(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}
