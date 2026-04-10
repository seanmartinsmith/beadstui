package correlation

import (
	"testing"
	"time"
)

func createTestReport() *HistoryReport {
	now := time.Now()

	return &HistoryReport{
		GeneratedAt: now,
		CommitIndex: CommitIndex{
			"abc123def456": []string{"bv-1", "bv-2"},
			"def456ghi789": []string{"bv-1"},
			"ghi789abc123": []string{"bv-3"},
		},
		Histories: map[string]BeadHistory{
			"bv-1": {
				BeadID: "bv-1",
				Title:  "Fix authentication bug",
				Status: "closed",
				Commits: []CorrelatedCommit{
					{
						SHA:         "abc123def456",
						ShortSHA:    "abc123d",
						Message:     "fix: auth bug",
						Author:      "Dev One",
						AuthorEmail: "dev1@test.com",
						Timestamp:   now,
						Method:      MethodCoCommitted,
						Confidence:  0.95,
						Reason:      "Co-committed",
					},
					{
						SHA:         "def456ghi789",
						ShortSHA:    "def456g",
						Message:     "test: add auth tests",
						Author:      "Dev One",
						AuthorEmail: "dev1@test.com",
						Timestamp:   now.Add(-time.Hour),
						Method:      MethodExplicitID,
						Confidence:  0.90,
						Reason:      "Explicit ID",
					},
				},
			},
			"bv-2": {
				BeadID: "bv-2",
				Title:  "Add logging",
				Status: "open",
				Commits: []CorrelatedCommit{
					{
						SHA:         "abc123def456",
						ShortSHA:    "abc123d",
						Message:     "fix: auth bug",
						Author:      "Dev One",
						AuthorEmail: "dev1@test.com",
						Timestamp:   now,
						Method:      MethodTemporalAuthor,
						Confidence:  0.60,
						Reason:      "Temporal",
					},
				},
			},
			"bv-3": {
				BeadID: "bv-3",
				Title:  "Refactor database",
				Status: "in_progress",
				Commits: []CorrelatedCommit{
					{
						SHA:         "ghi789abc123",
						ShortSHA:    "ghi789a",
						Message:     "refactor: db layer",
						Author:      "Dev Two",
						AuthorEmail: "dev2@test.com",
						Timestamp:   now.Add(-2 * time.Hour),
						Method:      MethodCoCommitted,
						Confidence:  0.92,
						Reason:      "Co-committed",
					},
				},
			},
		},
	}
}

func TestNewReverseLookup(t *testing.T) {
	report := createTestReport()
	rl := NewReverseLookup(report)

	if rl == nil {
		t.Fatal("NewReverseLookup returned nil")
	}

	if len(rl.index) != 3 {
		t.Errorf("index has %d entries, want 3", len(rl.index))
	}

	if len(rl.beads) != 3 {
		t.Errorf("beads has %d entries, want 3", len(rl.beads))
	}
}

func TestLookupByCommit_Found(t *testing.T) {
	report := createTestReport()
	rl := NewReverseLookup(report)

	result, err := rl.LookupByCommit("abc123def456")
	if err != nil {
		t.Fatalf("LookupByCommit failed: %v", err)
	}

	if result.IsOrphan {
		t.Error("Should not be orphan")
	}

	if len(result.RelatedBeads) != 2 {
		t.Errorf("RelatedBeads has %d entries, want 2", len(result.RelatedBeads))
	}

	// Check that both beads are found
	beadIDs := make(map[string]bool)
	for _, rb := range result.RelatedBeads {
		beadIDs[rb.BeadID] = true
	}

	if !beadIDs["bv-1"] || !beadIDs["bv-2"] {
		t.Errorf("Expected bv-1 and bv-2, got %v", beadIDs)
	}
}

func TestLookupByCommit_ShortSHA(t *testing.T) {
	report := createTestReport()
	rl := NewReverseLookup(report)

	// Use short SHA prefix
	result, err := rl.LookupByCommit("abc123")
	if err != nil {
		t.Fatalf("LookupByCommit with short SHA failed: %v", err)
	}

	if result.IsOrphan {
		t.Error("Should not be orphan")
	}

	if len(result.RelatedBeads) != 2 {
		t.Errorf("RelatedBeads has %d entries, want 2", len(result.RelatedBeads))
	}
}

func TestLookupByCommit_NotFound(t *testing.T) {
	report := createTestReport()
	rl := NewReverseLookup(report)

	result, err := rl.LookupByCommit("notfound123")
	if err != nil {
		t.Fatalf("LookupByCommit failed: %v", err)
	}

	if !result.IsOrphan {
		t.Error("Should be orphan")
	}

	if len(result.RelatedBeads) != 0 {
		t.Errorf("RelatedBeads should be empty, got %d", len(result.RelatedBeads))
	}
}

func TestLookupByCommit_IncludesDetails(t *testing.T) {
	report := createTestReport()
	rl := NewReverseLookup(report)

	result, err := rl.LookupByCommit("abc123def456")
	if err != nil {
		t.Fatalf("LookupByCommit failed: %v", err)
	}

	// Should have commit message from details
	if result.Message != "fix: auth bug" {
		t.Errorf("Message = %q, want %q", result.Message, "fix: auth bug")
	}

	if result.Author != "Dev One" {
		t.Errorf("Author = %q, want %q", result.Author, "Dev One")
	}

	// Check related bead details
	for _, rb := range result.RelatedBeads {
		if rb.BeadID == "bv-1" {
			if rb.BeadTitle != "Fix authentication bug" {
				t.Errorf("BeadTitle = %q, want %q", rb.BeadTitle, "Fix authentication bug")
			}
			if rb.Method != MethodCoCommitted {
				t.Errorf("Method = %v, want %v", rb.Method, MethodCoCommitted)
			}
			if rb.Confidence != 0.95 {
				t.Errorf("Confidence = %v, want 0.95", rb.Confidence)
			}
		}
	}
}

func TestGetCorrelatedCommitCount(t *testing.T) {
	report := createTestReport()
	rl := NewReverseLookup(report)

	count := rl.GetCorrelatedCommitCount()
	if count != 3 {
		t.Errorf("GetCorrelatedCommitCount() = %d, want 3", count)
	}
}

func TestGetAllBeadIDs(t *testing.T) {
	report := createTestReport()
	rl := NewReverseLookup(report)

	ids := rl.GetAllBeadIDs()
	if len(ids) != 3 {
		t.Errorf("GetAllBeadIDs() returned %d IDs, want 3", len(ids))
	}

	// Check all expected IDs are present
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	for _, want := range []string{"bv-1", "bv-2", "bv-3"} {
		if !idSet[want] {
			t.Errorf("GetAllBeadIDs() missing %q", want)
		}
	}
}

func TestGetBeadCommitSummaries(t *testing.T) {
	report := createTestReport()
	rl := NewReverseLookup(report)

	summaries := rl.GetBeadCommitSummaries()
	if len(summaries) != 3 {
		t.Errorf("GetBeadCommitSummaries() returned %d summaries, want 3", len(summaries))
	}

	// Find bv-1 summary
	var bv1Summary *BeadCommitsSummary
	for i := range summaries {
		if summaries[i].BeadID == "bv-1" {
			bv1Summary = &summaries[i]
			break
		}
	}

	if bv1Summary == nil {
		t.Fatal("bv-1 summary not found")
	}

	if bv1Summary.CommitCount != 2 {
		t.Errorf("bv-1 CommitCount = %d, want 2", bv1Summary.CommitCount)
	}

	// Average confidence should be (0.95 + 0.90) / 2 = 0.925
	expectedAvg := 0.925
	if bv1Summary.AvgConfid != expectedAvg {
		t.Errorf("bv-1 AvgConfid = %v, want %v", bv1Summary.AvgConfid, expectedAvg)
	}
}

func TestNormalizeSHA(t *testing.T) {
	report := createTestReport()
	rl := NewReverseLookup(report)

	tests := []struct {
		input string
		want  string
	}{
		{"abc123def456", "abc123def456"}, // Full SHA in index
		{"abc123", "abc123def456"},       // Short prefix expands
		{"def456", "def456ghi789"},       // Another short prefix
		{"unknown", "unknown"},           // Unknown stays as-is
	}

	for _, tt := range tests {
		got := rl.normalizeSHA(tt.input)
		if got != tt.want {
			t.Errorf("normalizeSHA(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRelatedBead_AllFields(t *testing.T) {
	report := createTestReport()
	rl := NewReverseLookup(report)

	result, _ := rl.LookupByCommit("ghi789abc123")

	if len(result.RelatedBeads) != 1 {
		t.Fatalf("Expected 1 related bead, got %d", len(result.RelatedBeads))
	}

	rb := result.RelatedBeads[0]

	if rb.BeadID != "bv-3" {
		t.Errorf("BeadID = %q, want bv-3", rb.BeadID)
	}

	if rb.BeadTitle != "Refactor database" {
		t.Errorf("BeadTitle = %q, want 'Refactor database'", rb.BeadTitle)
	}

	if rb.BeadStatus != "in_progress" {
		t.Errorf("BeadStatus = %q, want 'in_progress'", rb.BeadStatus)
	}

	if rb.Method != MethodCoCommitted {
		t.Errorf("Method = %v, want MethodCoCommitted", rb.Method)
	}

	if rb.Confidence != 0.92 {
		t.Errorf("Confidence = %v, want 0.92", rb.Confidence)
	}

	if rb.Reason != "Co-committed" {
		t.Errorf("Reason = %q, want 'Co-committed'", rb.Reason)
	}
}

func TestOrphanStats_Empty(t *testing.T) {
	stats := &OrphanStats{
		TotalCommits:   0,
		OrphanCommits:  0,
		CorrelatedCmts: 0,
		OrphanRatio:    0,
	}

	if stats.OrphanRatio != 0 {
		t.Errorf("OrphanRatio should be 0 for empty stats")
	}
}

func TestCommitBeadResult_EmptyRelatedBeads(t *testing.T) {
	result := &CommitBeadResult{
		CommitSHA:    "test123",
		RelatedBeads: []RelatedBead{},
		IsOrphan:     true,
	}

	if len(result.RelatedBeads) != 0 {
		t.Error("RelatedBeads should be empty")
	}

	if !result.IsOrphan {
		t.Error("Should be marked as orphan")
	}
}
