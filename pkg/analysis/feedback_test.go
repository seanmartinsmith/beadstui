package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultFeedbackData(t *testing.T) {
	f := DefaultFeedbackData()

	if f.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", f.Version)
	}

	if len(f.Adjustments) != 8 {
		t.Errorf("Expected 8 weight adjustments, got %d", len(f.Adjustments))
	}

	for _, adj := range f.Adjustments {
		if adj.Adjustment != 1.0 {
			t.Errorf("Expected default adjustment 1.0 for %s, got %f", adj.Name, adj.Adjustment)
		}
	}
}

func TestRecordFeedback(t *testing.T) {
	f := DefaultFeedbackData()

	breakdown := ScoreBreakdown{
		PageRankNorm:      0.8,
		BetweennessNorm:   0.5,
		BlockerRatioNorm:  0.3,
		StalenessNorm:     0.1,
		PriorityBoostNorm: 0.6,
		TimeToImpactNorm:  0.4,
		UrgencyNorm:       0.2,
		RiskNorm:          0.1,
	}

	// Record accept feedback
	err := f.RecordFeedback("test-1", "accept", 0.85, breakdown)
	if err != nil {
		t.Fatalf("Failed to record feedback: %v", err)
	}

	if f.Stats.TotalAccepted != 1 {
		t.Errorf("Expected 1 accepted, got %d", f.Stats.TotalAccepted)
	}

	if len(f.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(f.Events))
	}

	// Check that weights were adjusted
	weights := f.GetAdjustedWeights()
	if weights["PageRank"] <= 1.0 {
		t.Errorf("Expected PageRank weight to increase after accept, got %f", weights["PageRank"])
	}
}

func TestRecordIgnoreFeedback(t *testing.T) {
	f := DefaultFeedbackData()

	breakdown := ScoreBreakdown{
		PageRankNorm:      0.9, // High contribution should decrease
		BetweennessNorm:   0.1,
		BlockerRatioNorm:  0.1,
		StalenessNorm:     0.1,
		PriorityBoostNorm: 0.1,
		TimeToImpactNorm:  0.1,
		UrgencyNorm:       0.1,
		RiskNorm:          0.1,
	}

	err := f.RecordFeedback("test-2", "ignore", 0.75, breakdown)
	if err != nil {
		t.Fatalf("Failed to record feedback: %v", err)
	}

	if f.Stats.TotalIgnored != 1 {
		t.Errorf("Expected 1 ignored, got %d", f.Stats.TotalIgnored)
	}

	// Check that PageRank weight decreased (since it was high contributor to ignored)
	weights := f.GetAdjustedWeights()
	if weights["PageRank"] >= 1.0 {
		t.Errorf("Expected PageRank weight to decrease after ignore, got %f", weights["PageRank"])
	}
}

func TestInvalidAction(t *testing.T) {
	f := DefaultFeedbackData()

	err := f.RecordFeedback("test-3", "invalid", 0.5, ScoreBreakdown{})
	if err == nil {
		t.Error("Expected error for invalid action")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	// Create feedback and record some data
	f := DefaultFeedbackData()
	breakdown := ScoreBreakdown{PageRankNorm: 0.7}
	f.RecordFeedback("test-1", "accept", 0.8, breakdown)
	f.RecordFeedback("test-2", "ignore", 0.6, breakdown)

	// Save
	err := f.Save(dir)
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, FeedbackFile)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Feedback file was not created")
	}

	// Load
	loaded, err := LoadFeedback(dir)
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	if loaded.Stats.TotalAccepted != 1 {
		t.Errorf("Expected 1 accepted, got %d", loaded.Stats.TotalAccepted)
	}

	if loaded.Stats.TotalIgnored != 1 {
		t.Errorf("Expected 1 ignored, got %d", loaded.Stats.TotalIgnored)
	}

	if len(loaded.Events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(loaded.Events))
	}
}

func TestLoadNonexistent(t *testing.T) {
	dir := t.TempDir()

	// Loading from nonexistent file should return defaults
	f, err := LoadFeedback(dir)
	if err != nil {
		t.Fatalf("Failed to load from nonexistent: %v", err)
	}

	if len(f.Events) != 0 {
		t.Errorf("Expected empty events, got %d", len(f.Events))
	}
}

func TestGetEffectiveWeights(t *testing.T) {
	f := DefaultFeedbackData()

	// Initially, effective weights should match base weights (normalized)
	effective := f.GetEffectiveWeights()

	// Check that weights sum to ~1.0
	var total float64
	for _, w := range effective {
		total += w
	}

	if total < 0.99 || total > 1.01 {
		t.Errorf("Expected weights to sum to ~1.0, got %f", total)
	}
}

func TestReset(t *testing.T) {
	f := DefaultFeedbackData()

	// Add some feedback
	f.RecordFeedback("test-1", "accept", 0.8, ScoreBreakdown{PageRankNorm: 0.5})
	f.RecordFeedback("test-2", "ignore", 0.6, ScoreBreakdown{PageRankNorm: 0.5})

	// Reset
	f.Reset()

	if len(f.Events) != 0 {
		t.Errorf("Expected no events after reset, got %d", len(f.Events))
	}

	if f.Stats.TotalAccepted != 0 {
		t.Errorf("Expected 0 accepted after reset, got %d", f.Stats.TotalAccepted)
	}

	// Adjustments should be back to 1.0
	for _, adj := range f.Adjustments {
		if adj.Adjustment != 1.0 {
			t.Errorf("Expected adjustment 1.0 after reset, got %f", adj.Adjustment)
		}
	}
}

func TestSummary(t *testing.T) {
	f := DefaultFeedbackData()

	// Empty summary
	summary := f.Summary()
	if summary == "" {
		t.Error("Expected non-empty summary")
	}

	// Summary with data
	f.RecordFeedback("test-1", "accept", 0.8, ScoreBreakdown{})
	summary = f.Summary()
	if summary == "" {
		t.Error("Expected non-empty summary after feedback")
	}
}

func TestToJSON(t *testing.T) {
	f := DefaultFeedbackData()
	f.RecordFeedback("test-1", "accept", 0.9, ScoreBreakdown{PageRankNorm: 0.8})

	jsonData := f.ToJSON()

	if !jsonData.Enabled {
		t.Error("Expected feedback to be enabled after recording")
	}

	if jsonData.TotalEvents != 1 {
		t.Errorf("Expected 1 event, got %d", jsonData.TotalEvents)
	}

	if len(jsonData.WeightAdjustments) != 8 {
		t.Errorf("Expected 8 weight adjustments, got %d", len(jsonData.WeightAdjustments))
	}

	if len(jsonData.EffectiveWeights) != 8 {
		t.Errorf("Expected 8 effective weights, got %d", len(jsonData.EffectiveWeights))
	}
}

func TestWeightAdjustmentBounds(t *testing.T) {
	f := DefaultFeedbackData()

	// Record many accept feedbacks with high PageRank to test upper bound
	breakdown := ScoreBreakdown{PageRankNorm: 1.0}
	for i := 0; i < 100; i++ {
		f.RecordFeedback("test", "accept", 1.0, breakdown)
	}

	weights := f.GetAdjustedWeights()
	if weights["PageRank"] > 2.0 {
		t.Errorf("PageRank weight exceeded upper bound: %f", weights["PageRank"])
	}

	// Record many ignore feedbacks to test lower bound
	f.Reset()
	for i := 0; i < 100; i++ {
		f.RecordFeedback("test", "ignore", 1.0, breakdown)
	}

	weights = f.GetAdjustedWeights()
	if weights["PageRank"] < 0.5 {
		t.Errorf("PageRank weight went below lower bound: %f", weights["PageRank"])
	}
}
