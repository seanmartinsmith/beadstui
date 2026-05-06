package correlation

import (
	"os"
	"os/exec"
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
// produces byte-identical FileChange output to the per-event path
// (two git show per SHA). This is the load-bearing acceptance criterion
// for bt-h01q -- without it, the perf win is meaningless because callers
// would observe behaviour drift.
//
// Strategy: pull a handful of real SHAs from the ambient repo, run both
// paths on each, assert reflect.DeepEqual on the FileChange slices.
func TestPrefetchCoCommittedFiles_ByteIdenticalToPerEvent(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	// Walk up to repo root so git log sees the full history regardless of
	// where the test binary is invoked from.
	root := wd
	for {
		if _, err := os.Stat(root + "/.git"); err == nil {
			break
		}
		parent := pathDir(root)
		if parent == root {
			t.Skip("not inside a git repo")
		}
		root = parent
	}

	// Pull 8 recent SHAs. Enough to cover varied diffs (renames, big commits,
	// small commits) without making the test slow.
	out, err := exec.Command("git", "-C", root, "log", "-n", "8", "--format=%H").Output()
	if err != nil {
		t.Skipf("git log failed: %v", err)
	}
	shas := strings.Fields(strings.TrimSpace(string(out)))
	if len(shas) == 0 {
		t.Skip("no commits in repo")
	}

	c := NewCoCommitExtractor(root)

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

		// Order within a single SHA should already match (both pull from
		// `git show` / `git log` for that commit, which produces files in
		// stable order). But to be robust against any output-ordering
		// surprise on Windows, sort by Path before comparing.
		sortByPath(w)
		sortByPath(g)

		if !reflect.DeepEqual(w, g) {
			t.Errorf("sha %s: mismatch\n  per-event: %#v\n  batched:   %#v", sha, w, g)
		}
	}
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

// pathDir is a tiny helper to avoid importing path/filepath just for one call.
func pathDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return p
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
