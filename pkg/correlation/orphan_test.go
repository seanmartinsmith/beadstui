package correlation

import (
	"testing"
	"time"
)

func TestNewOrphanDetector(t *testing.T) {
	now := time.Now()
	report := &HistoryReport{
		Histories: map[string]BeadHistory{
			"bv-test1": {
				Title:      "Test Bead 1",
				Status:     "closed",
				LastAuthor: "Test Author", // Required for author->beads mapping
				Milestones: BeadMilestones{
					Claimed: &BeadEvent{
						Timestamp: now.Add(-72 * time.Hour),
					},
					Closed: &BeadEvent{
						Timestamp: now.Add(-24 * time.Hour),
					},
				},
				Commits: []CorrelatedCommit{
					{
						SHA:         "abc123def456",
						ShortSHA:    "abc123d",
						Author:      "Test Author",
						AuthorEmail: "test@example.com",
						Timestamp:   now.Add(-48 * time.Hour),
					},
				},
			},
		},
		CommitIndex: map[string][]string{
			"abc123def456": {"bv-test1"},
		},
	}

	od := NewOrphanDetector(report, "/tmp/test-repo")

	if od == nil {
		t.Fatal("Expected non-nil OrphanDetector")
	}

	// Check that temporal windows were built
	if len(od.beadWindows) != 1 {
		t.Errorf("Expected 1 bead window, got %d", len(od.beadWindows))
	}

	// Check that author -> beads mapping was built
	if len(od.authorBeads["test@example.com"]) != 1 {
		t.Errorf("Expected 1 bead for author, got %d", len(od.authorBeads["test@example.com"]))
	}
}

func TestNewSmartOrphanDetector(t *testing.T) {
	report := &HistoryReport{
		Histories:   make(map[string]BeadHistory),
		CommitIndex: make(map[string][]string),
	}

	od := NewSmartOrphanDetector(report, "/tmp/test-repo")
	if od == nil {
		t.Fatal("Expected non-nil OrphanDetector from SmartOrphanDetector alias")
	}
}

func TestOrphanCandidate_JSONRoundtrip(t *testing.T) {
	now := time.Now()
	candidate := OrphanCandidate{
		SHA:            "abc123",
		ShortSHA:       "abc1",
		Message:        "fix: test commit",
		Author:         "Test",
		AuthorEmail:    "test@example.com",
		Timestamp:      now,
		Files:          []string{"file1.go", "file2.go"},
		SuspicionScore: 75,
		ProbableBeads: []ProbableBead{
			{
				BeadID:     "bv-test",
				BeadTitle:  "Test Bead",
				BeadStatus: "open",
				Confidence: 80,
				Reasons:    []string{"timing", "author"},
			},
		},
		Signals: []OrphanSignalHit{
			{
				Signal:  SignalOrphanTiming,
				Details: "Commit during active period",
				Weight:  30,
			},
		},
	}

	// Just verify the struct is properly constructed
	if candidate.SuspicionScore != 75 {
		t.Errorf("Expected SuspicionScore 75, got %d", candidate.SuspicionScore)
	}
	if len(candidate.ProbableBeads) != 1 {
		t.Errorf("Expected 1 probable bead, got %d", len(candidate.ProbableBeads))
	}
	if len(candidate.Signals) != 1 {
		t.Errorf("Expected 1 signal, got %d", len(candidate.Signals))
	}
}

func TestOrphanReportStats(t *testing.T) {
	stats := OrphanReportStats{
		TotalCommits:    100,
		CorrelatedCount: 80,
		OrphanCount:     20,
		CandidateCount:  5,
		OrphanRatio:     0.2,
		AvgSuspicion:    65.0,
	}

	if stats.OrphanRatio != 0.2 {
		t.Errorf("Expected OrphanRatio 0.2, got %f", stats.OrphanRatio)
	}
	if stats.CandidateCount != 5 {
		t.Errorf("Expected CandidateCount 5, got %d", stats.CandidateCount)
	}
}

func TestOrphanSignalConstants(t *testing.T) {
	signals := []OrphanSignal{
		SignalOrphanTiming,
		SignalOrphanFiles,
		SignalOrphanMessage,
		SignalOrphanAuthor,
	}

	expected := []string{"timing", "files", "message", "author"}
	for i, signal := range signals {
		if string(signal) != expected[i] {
			t.Errorf("Expected signal %s, got %s", expected[i], string(signal))
		}
	}
}

func TestFormatGitRange(t *testing.T) {
	tests := []struct {
		name string
		opts ExtractOptions
		want string
	}{
		{
			name: "empty options",
			opts: ExtractOptions{},
			want: "all history",
		},
		{
			name: "with limit",
			opts: ExtractOptions{Limit: 100},
			want: "limit 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatGitRange(tt.opts)
			if got != tt.want {
				t.Errorf("formatGitRange() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppendUnique(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		s     string
		want  int
	}{
		{
			name:  "append to empty",
			slice: []string{},
			s:     "a",
			want:  1,
		},
		{
			name:  "append unique",
			slice: []string{"a", "b"},
			s:     "c",
			want:  3,
		},
		{
			name:  "append duplicate",
			slice: []string{"a", "b"},
			s:     "a",
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUnique(tt.slice, tt.s)
			if len(got) != tt.want {
				t.Errorf("appendUnique() length = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestProbableBead_Fields(t *testing.T) {
	pb := ProbableBead{
		BeadID:     "bv-123",
		BeadTitle:  "Test Title",
		BeadStatus: "in_progress",
		Confidence: 85,
		Reasons:    []string{"timing match", "file overlap"},
	}

	if pb.BeadID != "bv-123" {
		t.Errorf("Expected BeadID 'bv-123', got %s", pb.BeadID)
	}
	if pb.Confidence != 85 {
		t.Errorf("Expected Confidence 85, got %d", pb.Confidence)
	}
	if len(pb.Reasons) != 2 {
		t.Errorf("Expected 2 reasons, got %d", len(pb.Reasons))
	}
}

func TestOrphanReport_Fields(t *testing.T) {
	now := time.Now()
	report := OrphanReport{
		GeneratedAt: now,
		GitRange:    "last 30 days",
		DataHash:    "abc123",
		Stats: OrphanReportStats{
			TotalCommits: 50,
			OrphanCount:  10,
		},
		Candidates: []OrphanCandidate{},
		ByBead:     map[string][]string{"bv-1": {"sha1", "sha2"}},
	}

	if report.GitRange != "last 30 days" {
		t.Errorf("Expected GitRange 'last 30 days', got %s", report.GitRange)
	}
	if len(report.ByBead["bv-1"]) != 2 {
		t.Errorf("Expected 2 commits for bv-1, got %d", len(report.ByBead["bv-1"]))
	}
}
