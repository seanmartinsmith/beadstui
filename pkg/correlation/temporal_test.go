package correlation

import (
	"strings"
	"testing"
	"time"
)

func TestExtractPathHints(t *testing.T) {
	tests := []struct {
		title string
		want  []string
	}{
		{
			title: "Fix authentication bug in pkg/auth",
			want:  []string{"pkg/auth"}, // "auth" in "authentication" is not a word boundary match
		},
		{
			title: "Update user login flow",
			want:  []string{"user", "login"},
		},
		{
			title: "Refactor database connection handler",
			want:  []string{"database", "handler"},
		},
		{
			title: "Add tests for api service",
			want:  []string{"tests", "api", "service"}, // "tests" is now a keyword
		},
		{
			title: "internal/config improvements",
			want:  []string{"internal/config"}, // Only the path, not "config" separately
		},
		{
			title: "Simple title with no hints",
			want:  nil,
		},
		{
			title: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := extractPathHints(tt.title)

			if tt.want == nil {
				if got != nil {
					t.Errorf("extractPathHints(%q) = %v, want nil", tt.title, got)
				}
				return
			}

			// Check that all expected hints are present
			for _, w := range tt.want {
				found := false
				for _, g := range got {
					if g == w {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("extractPathHints(%q) missing expected hint %q, got %v", tt.title, w, got)
				}
			}
		})
	}
}

func TestPathsMatchHints(t *testing.T) {
	tests := []struct {
		name  string
		files []FileChange
		hints []string
		want  bool
	}{
		{
			name: "match in path",
			files: []FileChange{
				{Path: "pkg/auth/login.go"},
			},
			hints: []string{"auth"},
			want:  true,
		},
		{
			name: "match in nested path",
			files: []FileChange{
				{Path: "internal/service/user/handler.go"},
			},
			hints: []string{"user"},
			want:  true,
		},
		{
			name: "no match",
			files: []FileChange{
				{Path: "pkg/billing/invoice.go"},
			},
			hints: []string{"auth", "login"},
			want:  false,
		},
		{
			name: "case insensitive",
			files: []FileChange{
				{Path: "pkg/AUTH/Login.go"},
			},
			hints: []string{"auth"},
			want:  true,
		},
		{
			name:  "empty files",
			files: []FileChange{},
			hints: []string{"auth"},
			want:  false,
		},
		{
			name: "empty hints",
			files: []FileChange{
				{Path: "pkg/auth/login.go"},
			},
			hints: []string{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathsMatchHints(tt.files, tt.hints)
			if got != tt.want {
				t.Errorf("pathsMatchHints() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		value, min, max, want float64
	}{
		{0.5, 0.0, 1.0, 0.5},   // Within range
		{-0.5, 0.0, 1.0, 0.0},  // Below min
		{1.5, 0.0, 1.0, 1.0},   // Above max
		{0.0, 0.0, 1.0, 0.0},   // At min
		{1.0, 0.0, 1.0, 1.0},   // At max
		{0.5, 0.2, 0.85, 0.5},  // Within temporal range
		{0.1, 0.2, 0.85, 0.2},  // Below temporal min
		{0.9, 0.2, 0.85, 0.85}, // Above temporal max
	}

	for _, tt := range tests {
		got := clamp(tt.value, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("clamp(%v, %v, %v) = %v, want %v", tt.value, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestCalculateTemporalConfidence(t *testing.T) {
	tc := NewTemporalCorrelator("/test/repo")
	now := time.Now()

	tests := []struct {
		name       string
		window     TemporalWindow
		files      []FileChange
		pathHints  []string
		authActive map[string]int
		wantRange  [2]float64
	}{
		{
			name: "base case - single bead, moderate window",
			window: TemporalWindow{
				AuthorEmail: "dev@test.com",
				Start:       now.Add(-12 * time.Hour),
				End:         now,
			},
			files:      []FileChange{{Path: "file.go"}},
			pathHints:  nil,
			authActive: map[string]int{"dev@test.com": 1},
			wantRange:  [2]float64{0.65, 0.85}, // base 0.50 + 0.20 (single bead) + 0.05 (moderate window)
		},
		{
			name: "short window boost",
			window: TemporalWindow{
				AuthorEmail: "dev@test.com",
				Start:       now.Add(-2 * time.Hour),
				End:         now,
			},
			files:      []FileChange{{Path: "file.go"}},
			pathHints:  nil,
			authActive: map[string]int{"dev@test.com": 1},
			wantRange:  [2]float64{0.75, 0.85}, // base 0.50 + 0.20 (single bead) + 0.10 (short window)
		},
		{
			name: "long window penalty",
			window: TemporalWindow{
				AuthorEmail: "dev@test.com",
				Start:       now.Add(-10 * 24 * time.Hour),
				End:         now,
			},
			files:      []FileChange{{Path: "file.go"}},
			pathHints:  nil,
			authActive: map[string]int{"dev@test.com": 1},
			wantRange:  [2]float64{0.50, 0.60}, // base 0.50 + 0.20 (single bead) - 0.15 (long window)
		},
		{
			name: "many beads penalty",
			window: TemporalWindow{
				AuthorEmail: "dev@test.com",
				Start:       now.Add(-12 * time.Hour),
				End:         now,
			},
			files:      []FileChange{{Path: "file.go"}},
			pathHints:  nil,
			authActive: map[string]int{"dev@test.com": 5},
			wantRange:  [2]float64{0.40, 0.50}, // base 0.50 - 0.10 (many beads) + 0.05 (moderate window)
		},
		{
			name: "path hint match boost",
			window: TemporalWindow{
				AuthorEmail: "dev@test.com",
				Start:       now.Add(-12 * time.Hour),
				End:         now,
			},
			files:      []FileChange{{Path: "pkg/auth/login.go"}},
			pathHints:  []string{"auth"},
			authActive: map[string]int{"dev@test.com": 2},
			wantRange:  [2]float64{0.70, 0.85}, // base 0.50 + 0.10 (2 beads) + 0.05 (moderate) + 0.15 (path match)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc.activeByAuth = tt.authActive
			got := tc.calculateTemporalConfidence(tt.window, tt.files, tt.pathHints)
			if got < tt.wantRange[0] || got > tt.wantRange[1] {
				t.Errorf("calculateTemporalConfidence() = %v, want in range [%v, %v]", got, tt.wantRange[0], tt.wantRange[1])
			}
		})
	}
}

func TestGenerateTemporalReason(t *testing.T) {
	tc := NewTemporalCorrelator("/test/repo")
	now := time.Now()

	tests := []struct {
		name           string
		window         TemporalWindow
		files          []FileChange
		pathHints      []string
		authActive     map[string]int
		expectContains []string
	}{
		{
			name: "basic reason",
			window: TemporalWindow{
				Author:      "Test Dev",
				AuthorEmail: "dev@test.com",
				Start:       now.Add(-12 * time.Hour),
				End:         now,
			},
			files:          []FileChange{{Path: "file.go"}},
			authActive:     map[string]int{"dev@test.com": 2},
			expectContains: []string{"Commit by Test Dev", "active window"},
		},
		{
			name: "short window",
			window: TemporalWindow{
				Author:      "Test Dev",
				AuthorEmail: "dev@test.com",
				Start:       now.Add(-2 * time.Hour),
				End:         now,
			},
			files:          []FileChange{{Path: "file.go"}},
			authActive:     map[string]int{"dev@test.com": 1},
			expectContains: []string{"short window", "only this bead active"},
		},
		{
			name: "long window with many beads",
			window: TemporalWindow{
				Author:      "Test Dev",
				AuthorEmail: "dev@test.com",
				Start:       now.Add(-10 * 24 * time.Hour),
				End:         now,
			},
			files:          []FileChange{{Path: "file.go"}},
			authActive:     map[string]int{"dev@test.com": 5},
			expectContains: []string{"long window", "5 beads active"},
		},
		{
			name: "path hint match",
			window: TemporalWindow{
				Author:      "Test Dev",
				AuthorEmail: "dev@test.com",
				Start:       now.Add(-12 * time.Hour),
				End:         now,
			},
			files:          []FileChange{{Path: "pkg/auth/login.go"}},
			pathHints:      []string{"auth"},
			authActive:     map[string]int{"dev@test.com": 1},
			expectContains: []string{"file paths match bead title keywords"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc.activeByAuth = tt.authActive
			_ = tc.calculateTemporalConfidence(tt.window, tt.files, tt.pathHints) // Ensure confidence calc works
			got := tc.generateTemporalReason(tt.window, tt.files, tt.pathHints)

			for _, exp := range tt.expectContains {
				if !strings.Contains(got, exp) {
					t.Errorf("generateTemporalReason() = %q, expected to contain %q", got, exp)
				}
			}
		})
	}
}

func TestExtractWindowFromMilestones(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-time.Hour)

	claimedEvent := &BeadEvent{
		BeadID:      "bv-123",
		EventType:   EventClaimed,
		Timestamp:   oneHourAgo,
		Author:      "Test Dev",
		AuthorEmail: "dev@test.com",
	}

	closedEvent := &BeadEvent{
		BeadID:      "bv-123",
		EventType:   EventClosed,
		Timestamp:   now,
		Author:      "Test Dev",
		AuthorEmail: "dev@test.com",
	}

	tests := []struct {
		name       string
		beadID     string
		title      string
		milestones BeadMilestones
		wantNil    bool
	}{
		{
			name:   "valid window",
			beadID: "bv-123",
			title:  "Fix auth bug",
			milestones: BeadMilestones{
				Claimed: claimedEvent,
				Closed:  closedEvent,
			},
			wantNil: false,
		},
		{
			name:   "missing claimed",
			beadID: "bv-123",
			title:  "Fix auth bug",
			milestones: BeadMilestones{
				Closed: closedEvent,
			},
			wantNil: true,
		},
		{
			name:   "missing closed",
			beadID: "bv-123",
			title:  "Fix auth bug",
			milestones: BeadMilestones{
				Claimed: claimedEvent,
			},
			wantNil: true,
		},
		{
			name:       "empty milestones",
			beadID:     "bv-123",
			title:      "Fix auth bug",
			milestones: BeadMilestones{},
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractWindowFromMilestones(tt.beadID, tt.title, tt.milestones)

			if tt.wantNil {
				if got != nil {
					t.Errorf("ExtractWindowFromMilestones() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("ExtractWindowFromMilestones() = nil, want non-nil")
			}

			if got.BeadID != tt.beadID {
				t.Errorf("BeadID = %q, want %q", got.BeadID, tt.beadID)
			}
			if got.Title != tt.title {
				t.Errorf("Title = %q, want %q", got.Title, tt.title)
			}
			if got.Author != claimedEvent.Author {
				t.Errorf("Author = %q, want %q", got.Author, claimedEvent.Author)
			}
			if !got.Start.Equal(claimedEvent.Timestamp) {
				t.Errorf("Start = %v, want %v", got.Start, claimedEvent.Timestamp)
			}
			if !got.End.Equal(closedEvent.Timestamp) {
				t.Errorf("End = %v, want %v", got.End, closedEvent.Timestamp)
			}
		})
	}
}

func TestSetSeenCommits(t *testing.T) {
	tc := NewTemporalCorrelator("/test/repo")

	commits := []CorrelatedCommit{
		{SHA: "abc123"},
		{SHA: "def456"},
		{SHA: "ghi789"},
	}

	tc.SetSeenCommits(commits)

	for _, c := range commits {
		if !tc.seenCommits[c.SHA] {
			t.Errorf("SetSeenCommits() did not mark %q as seen", c.SHA)
		}
	}

	if tc.seenCommits["unknown"] {
		t.Error("SetSeenCommits() incorrectly marked unknown SHA as seen")
	}
}

func TestSetActiveBeadsPerAuthor(t *testing.T) {
	tc := NewTemporalCorrelator("/test/repo")

	counts := map[string]int{
		"dev1@test.com": 3,
		"dev2@test.com": 1,
	}

	tc.SetActiveBeadsPerAuthor(counts)

	if tc.activeByAuth["dev1@test.com"] != 3 {
		t.Errorf("activeByAuth[dev1] = %d, want 3", tc.activeByAuth["dev1@test.com"])
	}
	if tc.activeByAuth["dev2@test.com"] != 1 {
		t.Errorf("activeByAuth[dev2] = %d, want 1", tc.activeByAuth["dev2@test.com"])
	}
}

func TestCalculateActiveBeadsPerAuthor(t *testing.T) {
	tc := NewTemporalCorrelator("/test/repo")

	now := time.Now()
	histories := map[string]BeadHistory{
		"bv-1": {
			Milestones: BeadMilestones{
				Claimed: &BeadEvent{AuthorEmail: "dev1@test.com", Timestamp: now},
			},
		},
		"bv-2": {
			Milestones: BeadMilestones{
				Claimed: &BeadEvent{AuthorEmail: "dev1@test.com", Timestamp: now},
			},
		},
		"bv-3": {
			Milestones: BeadMilestones{
				Claimed: &BeadEvent{AuthorEmail: "dev2@test.com", Timestamp: now},
			},
		},
		"bv-4": {
			Milestones: BeadMilestones{}, // No claimed event
		},
	}

	tc.calculateActiveBeadsPerAuthor(histories)

	if tc.activeByAuth["dev1@test.com"] != 2 {
		t.Errorf("activeByAuth[dev1] = %d, want 2", tc.activeByAuth["dev1@test.com"])
	}
	if tc.activeByAuth["dev2@test.com"] != 1 {
		t.Errorf("activeByAuth[dev2] = %d, want 1", tc.activeByAuth["dev2@test.com"])
	}
}
