package correlation

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventType_String(t *testing.T) {
	tests := []struct {
		e    EventType
		want string
	}{
		{EventCreated, "created"},
		{EventClaimed, "claimed"},
		{EventClosed, "closed"},
		{EventReopened, "reopened"},
		{EventModified, "modified"},
	}
	for _, tt := range tests {
		if got := tt.e.String(); got != tt.want {
			t.Errorf("EventType.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestEventType_IsValid(t *testing.T) {
	tests := []struct {
		e    EventType
		want bool
	}{
		{EventCreated, true},
		{EventClaimed, true},
		{EventClosed, true},
		{EventReopened, true},
		{EventModified, true},
		{EventType("invalid"), false},
		{EventType(""), false},
	}
	for _, tt := range tests {
		if got := tt.e.IsValid(); got != tt.want {
			t.Errorf("EventType(%q).IsValid() = %v, want %v", tt.e, got, tt.want)
		}
	}
}

func TestCorrelationMethod_String(t *testing.T) {
	tests := []struct {
		c    CorrelationMethod
		want string
	}{
		{MethodCoCommitted, "co_committed"},
		{MethodExplicitID, "explicit_id"},
		{MethodTemporalAuthor, "temporal_author"},
	}
	for _, tt := range tests {
		if got := tt.c.String(); got != tt.want {
			t.Errorf("CorrelationMethod.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestCorrelationMethod_IsValid(t *testing.T) {
	tests := []struct {
		c    CorrelationMethod
		want bool
	}{
		{MethodCoCommitted, true},
		{MethodExplicitID, true},
		{MethodTemporalAuthor, true},
		{CorrelationMethod("invalid"), false},
		{CorrelationMethod(""), false},
	}
	for _, tt := range tests {
		if got := tt.c.IsValid(); got != tt.want {
			t.Errorf("CorrelationMethod(%q).IsValid() = %v, want %v", tt.c, got, tt.want)
		}
	}
}

func TestBeadEvent_JSONRoundtrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := BeadEvent{
		BeadID:      "bv-123",
		EventType:   EventClaimed,
		Timestamp:   now,
		CommitSHA:   "abc123def456",
		CommitMsg:   "feat: implement login",
		Author:      "Test User",
		AuthorEmail: "test@example.com",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded BeadEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.BeadID != original.BeadID {
		t.Errorf("BeadID mismatch: got %v, want %v", decoded.BeadID, original.BeadID)
	}
	if decoded.EventType != original.EventType {
		t.Errorf("EventType mismatch: got %v, want %v", decoded.EventType, original.EventType)
	}
	if !decoded.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", decoded.Timestamp, original.Timestamp)
	}
	if decoded.CommitSHA != original.CommitSHA {
		t.Errorf("CommitSHA mismatch: got %v, want %v", decoded.CommitSHA, original.CommitSHA)
	}
	if decoded.Author != original.Author {
		t.Errorf("Author mismatch: got %v, want %v", decoded.Author, original.Author)
	}
}

func TestCorrelatedCommit_JSONRoundtrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := CorrelatedCommit{
		SHA:         "abc123def456789",
		ShortSHA:    "abc123d",
		Message:     "fix(auth): resolve token expiry issue",
		Author:      "Developer",
		AuthorEmail: "dev@example.com",
		Timestamp:   now,
		Files: []FileChange{
			{Path: "pkg/auth/token.go", Action: "M", Insertions: 15, Deletions: 3},
			{Path: "pkg/auth/token_test.go", Action: "M", Insertions: 42, Deletions: 0},
		},
		Method:     MethodExplicitID,
		Confidence: 0.95,
		Reason:     "Commit message contains bead ID 'bv-123'",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded CorrelatedCommit
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.SHA != original.SHA {
		t.Errorf("SHA mismatch: got %v, want %v", decoded.SHA, original.SHA)
	}
	if decoded.Method != original.Method {
		t.Errorf("Method mismatch: got %v, want %v", decoded.Method, original.Method)
	}
	if decoded.Confidence != original.Confidence {
		t.Errorf("Confidence mismatch: got %v, want %v", decoded.Confidence, original.Confidence)
	}
	if len(decoded.Files) != len(original.Files) {
		t.Errorf("Files length mismatch: got %v, want %v", len(decoded.Files), len(original.Files))
	}
	if decoded.Files[0].Path != original.Files[0].Path {
		t.Errorf("Files[0].Path mismatch: got %v, want %v", decoded.Files[0].Path, original.Files[0].Path)
	}
}

func TestBeadHistory_JSONRoundtrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	claimToClose := 24 * time.Hour
	createToClose := 48 * time.Hour
	createToClaim := 24 * time.Hour

	createdEvent := BeadEvent{
		BeadID:    "bv-456",
		EventType: EventCreated,
		Timestamp: now.Add(-48 * time.Hour),
		CommitSHA: "create123",
	}
	claimedEvent := BeadEvent{
		BeadID:    "bv-456",
		EventType: EventClaimed,
		Timestamp: now.Add(-24 * time.Hour),
		CommitSHA: "claim456",
	}
	closedEvent := BeadEvent{
		BeadID:    "bv-456",
		EventType: EventClosed,
		Timestamp: now,
		CommitSHA: "close789",
	}

	original := BeadHistory{
		BeadID: "bv-456",
		Title:  "Implement feature X",
		Status: "closed",
		Events: []BeadEvent{createdEvent, claimedEvent, closedEvent},
		Milestones: BeadMilestones{
			Created: &createdEvent,
			Claimed: &claimedEvent,
			Closed:  &closedEvent,
		},
		Commits: []CorrelatedCommit{
			{
				SHA:        "fix123",
				ShortSHA:   "fix123",
				Message:    "fix: address edge case",
				Method:     MethodCoCommitted,
				Confidence: 1.0,
			},
		},
		CycleTime: &CycleTime{
			ClaimToClose:  &claimToClose,
			CreateToClose: &createToClose,
			CreateToClaim: &createToClaim,
		},
		LastAuthor: "Developer",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded BeadHistory
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.BeadID != original.BeadID {
		t.Errorf("BeadID mismatch: got %v, want %v", decoded.BeadID, original.BeadID)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title mismatch: got %v, want %v", decoded.Title, original.Title)
	}
	if len(decoded.Events) != len(original.Events) {
		t.Errorf("Events length mismatch: got %v, want %v", len(decoded.Events), len(original.Events))
	}
	if decoded.Milestones.Created == nil {
		t.Error("Milestones.Created is nil")
	}
	if decoded.CycleTime == nil {
		t.Error("CycleTime is nil")
	}
	if decoded.CycleTime.ClaimToClose == nil {
		t.Error("CycleTime.ClaimToClose is nil")
	}
}

func TestHistoryReport_JSONRoundtrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	avgCycleTime := 2.5

	original := HistoryReport{
		GeneratedAt: now,
		DataHash:    "abc123hash",
		GitRange:    "HEAD~100..HEAD",
		Stats: HistoryStats{
			TotalBeads:        50,
			BeadsWithCommits:  45,
			TotalCommits:      200,
			UniqueAuthors:     5,
			AvgCommitsPerBead: 4.0,
			AvgCycleTimeDays:  &avgCycleTime,
			MethodDistribution: map[string]int{
				"co_committed":    150,
				"explicit_id":     40,
				"temporal_author": 10,
			},
		},
		Histories: map[string]BeadHistory{
			"bv-001": {
				BeadID: "bv-001",
				Title:  "First bead",
				Status: "open",
			},
		},
		CommitIndex: CommitIndex{
			"sha123": {"bv-001", "bv-002"},
			"sha456": {"bv-003"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded HistoryReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.DataHash != original.DataHash {
		t.Errorf("DataHash mismatch: got %v, want %v", decoded.DataHash, original.DataHash)
	}
	if decoded.GitRange != original.GitRange {
		t.Errorf("GitRange mismatch: got %v, want %v", decoded.GitRange, original.GitRange)
	}
	if decoded.Stats.TotalBeads != original.Stats.TotalBeads {
		t.Errorf("Stats.TotalBeads mismatch: got %v, want %v", decoded.Stats.TotalBeads, original.Stats.TotalBeads)
	}
	if decoded.Stats.AvgCycleTimeDays == nil || *decoded.Stats.AvgCycleTimeDays != avgCycleTime {
		t.Errorf("Stats.AvgCycleTimeDays mismatch")
	}
	if len(decoded.Histories) != len(original.Histories) {
		t.Errorf("Histories length mismatch: got %v, want %v", len(decoded.Histories), len(original.Histories))
	}
	if len(decoded.CommitIndex) != len(original.CommitIndex) {
		t.Errorf("CommitIndex length mismatch: got %v, want %v", len(decoded.CommitIndex), len(original.CommitIndex))
	}
	if len(decoded.CommitIndex["sha123"]) != 2 {
		t.Errorf("CommitIndex['sha123'] length mismatch: got %v, want 2", len(decoded.CommitIndex["sha123"]))
	}
}

func TestFilterOptions_JSONRoundtrip(t *testing.T) {
	since := time.Now().UTC().Add(-7 * 24 * time.Hour).Truncate(time.Second)
	original := FilterOptions{
		BeadIDs:       []string{"bv-001", "bv-002"},
		Since:         &since,
		Authors:       []string{"alice@example.com", "bob@example.com"},
		MinConfidence: 0.8,
		IncludeClosed: true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded FilterOptions
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(decoded.BeadIDs) != len(original.BeadIDs) {
		t.Errorf("BeadIDs length mismatch: got %v, want %v", len(decoded.BeadIDs), len(original.BeadIDs))
	}
	if decoded.Since == nil || !decoded.Since.Equal(*original.Since) {
		t.Error("Since mismatch")
	}
	if decoded.MinConfidence != original.MinConfidence {
		t.Errorf("MinConfidence mismatch: got %v, want %v", decoded.MinConfidence, original.MinConfidence)
	}
	if decoded.IncludeClosed != original.IncludeClosed {
		t.Errorf("IncludeClosed mismatch: got %v, want %v", decoded.IncludeClosed, original.IncludeClosed)
	}
}

func TestFileChange_JSONRoundtrip(t *testing.T) {
	original := FileChange{
		Path:       "pkg/auth/handler.go",
		Action:     "M",
		Insertions: 25,
		Deletions:  10,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded FileChange
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("FileChange mismatch: got %+v, want %+v", decoded, original)
	}
}
