package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestEstimateETAForIssue_Basic(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	issues := []model.Issue{
		{
			ID:        "test-1",
			Title:     "Test issue",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Labels:    []string{"backend"},
		},
	}

	eta, err := EstimateETAForIssue(issues, nil, "test-1", 1, now)
	if err != nil {
		t.Fatalf("EstimateETAForIssue failed: %v", err)
	}

	if eta.IssueID != "test-1" {
		t.Errorf("Expected issue ID 'test-1', got %q", eta.IssueID)
	}

	if eta.EstimatedMinutes <= 0 {
		t.Errorf("Expected positive estimated minutes, got %d", eta.EstimatedMinutes)
	}

	if eta.Confidence <= 0 || eta.Confidence > 1 {
		t.Errorf("Expected confidence between 0 and 1, got %f", eta.Confidence)
	}

	if eta.ETADate.Before(now) {
		t.Errorf("ETA date should be in the future, got %v", eta.ETADate)
	}

	if eta.ETADateHigh.Before(eta.ETADate) {
		t.Errorf("High estimate should be >= ETA date")
	}

	if len(eta.Factors) == 0 {
		t.Error("Expected at least one factor")
	}
}

func TestEstimateETAForIssue_NotFound(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{}

	_, err := EstimateETAForIssue(issues, nil, "nonexistent", 1, now)
	if err == nil {
		t.Error("Expected error for nonexistent issue")
	}
}

func TestEstimateETAForIssue_WithExplicitEstimate(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	explicitMinutes := 120

	issues := []model.Issue{
		{
			ID:               "test-1",
			Title:            "Test issue with estimate",
			Status:           model.StatusOpen,
			IssueType:        model.TypeTask,
			EstimatedMinutes: &explicitMinutes,
		},
	}

	eta, err := EstimateETAForIssue(issues, nil, "test-1", 1, now)
	if err != nil {
		t.Fatalf("EstimateETAForIssue failed: %v", err)
	}

	// Explicit estimate should be higher confidence
	if eta.Confidence < 0.3 {
		t.Errorf("Expected higher confidence with explicit estimate, got %f", eta.Confidence)
	}

	// Should mention explicit estimate in factors
	hasExplicitFactor := false
	for _, f := range eta.Factors {
		if strings.HasPrefix(f, "estimate:") {
			hasExplicitFactor = true
			break
		}
	}
	if !hasExplicitFactor {
		t.Error("Expected factor mentioning estimate")
	}
}

func TestEstimateETAForIssue_TypeWeights(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	taskIssue := model.Issue{
		ID:        "task-1",
		Title:     "Task",
		Status:    model.StatusOpen,
		IssueType: model.TypeTask,
	}

	epicIssue := model.Issue{
		ID:        "epic-1",
		Title:     "Epic",
		Status:    model.StatusOpen,
		IssueType: model.TypeEpic,
	}

	taskETA, _ := EstimateETAForIssue([]model.Issue{taskIssue}, nil, "task-1", 1, now)
	epicETA, _ := EstimateETAForIssue([]model.Issue{epicIssue}, nil, "epic-1", 1, now)

	// Epic should take longer than task (type weight is higher)
	if epicETA.EstimatedDays <= taskETA.EstimatedDays {
		t.Errorf("Epic should have longer ETA than task: epic=%f, task=%f",
			epicETA.EstimatedDays, taskETA.EstimatedDays)
	}
}

func TestEstimateETAForIssue_MultipleAgents(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	issues := []model.Issue{
		{
			ID:        "test-1",
			Title:     "Test issue",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
		},
	}

	eta1, _ := EstimateETAForIssue(issues, nil, "test-1", 1, now)
	eta2, _ := EstimateETAForIssue(issues, nil, "test-1", 2, now)

	// 2 agents should complete faster (roughly half the time)
	if eta2.EstimatedDays >= eta1.EstimatedDays {
		t.Errorf("2 agents should be faster than 1: 1 agent=%f days, 2 agents=%f days",
			eta1.EstimatedDays, eta2.EstimatedDays)
	}
}

func TestEstimateETAForIssue_VelocityFromClosures(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	closedAt := now.Add(-7 * 24 * time.Hour) // 7 days ago

	issues := []model.Issue{
		{
			ID:        "open-1",
			Title:     "Open issue",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Labels:    []string{"backend"},
		},
		{
			ID:        "closed-1",
			Title:     "Closed issue",
			Status:    model.StatusClosed,
			IssueType: model.TypeTask,
			Labels:    []string{"backend"},
			ClosedAt:  &closedAt,
		},
	}

	eta, err := EstimateETAForIssue(issues, nil, "open-1", 1, now)
	if err != nil {
		t.Fatalf("EstimateETAForIssue failed: %v", err)
	}

	// Should have velocity factor from closure history
	hasVelocityFactor := false
	for _, f := range eta.Factors {
		if strings.HasPrefix(f, "velocity:") {
			hasVelocityFactor = true
			break
		}
	}
	if !hasVelocityFactor {
		t.Error("Expected velocity factor from closure history")
	}
}

func TestEstimateETAForIssue_DepthAffectsComplexity(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	issues := []model.Issue{
		{
			ID:          "depth-1",
			Title:       "Depth issue",
			Status:      model.StatusOpen,
			IssueType:   model.TypeTask,
			Description: "",
		},
	}

	baseETA, err := EstimateETAForIssue(issues, nil, "depth-1", 1, now)
	if err != nil {
		t.Fatalf("EstimateETAForIssue failed: %v", err)
	}

	stats := &GraphStats{
		criticalPathScore: map[string]float64{"depth-1": 10},
	}
	depthETA, err := EstimateETAForIssue(issues, stats, "depth-1", 1, now)
	if err != nil {
		t.Fatalf("EstimateETAForIssue failed: %v", err)
	}

	if depthETA.EstimatedMinutes <= baseETA.EstimatedMinutes {
		t.Errorf("Expected depth to increase estimated minutes: base=%d, depth=%d", baseETA.EstimatedMinutes, depthETA.EstimatedMinutes)
	}
	if depthETA.EstimatedDays <= baseETA.EstimatedDays {
		t.Errorf("Expected depth to increase estimated days: base=%f, depth=%f", baseETA.EstimatedDays, depthETA.EstimatedDays)
	}
}

func TestComputeMedianEstimatedMinutes(t *testing.T) {
	// No estimates - should return default
	emptyIssues := []model.Issue{{ID: "1"}}
	median := computeMedianEstimatedMinutes(emptyIssues)
	if median != DefaultEstimatedMinutes {
		t.Errorf("Expected default %d for empty estimates, got %d", DefaultEstimatedMinutes, median)
	}

	// Odd number of estimates
	est30, est60, est90 := 30, 60, 90
	oddIssues := []model.Issue{
		{ID: "1", EstimatedMinutes: &est30},
		{ID: "2", EstimatedMinutes: &est60},
		{ID: "3", EstimatedMinutes: &est90},
	}
	median = computeMedianEstimatedMinutes(oddIssues)
	if median != 60 {
		t.Errorf("Expected median 60 for odd count, got %d", median)
	}

	// Even number of estimates
	est120 := 120
	evenIssues := []model.Issue{
		{ID: "1", EstimatedMinutes: &est30},
		{ID: "2", EstimatedMinutes: &est60},
		{ID: "3", EstimatedMinutes: &est90},
		{ID: "4", EstimatedMinutes: &est120},
	}
	median = computeMedianEstimatedMinutes(evenIssues)
	// Median of [30, 60, 90, 120] = (60 + 90) / 2 = 75
	if median != 75 {
		t.Errorf("Expected median 75 for even count, got %d", median)
	}
}

func TestClampFloat(t *testing.T) {
	if clampFloat(0.5, 0.0, 1.0) != 0.5 {
		t.Error("Value in range should not change")
	}
	if clampFloat(-0.5, 0.0, 1.0) != 0.0 {
		t.Error("Value below range should be clamped to lo")
	}
	if clampFloat(1.5, 0.0, 1.0) != 1.0 {
		t.Error("Value above range should be clamped to hi")
	}
}

func TestDurationDays(t *testing.T) {
	if durationDays(0) != 0 {
		t.Error("0 days should return 0 duration")
	}
	if durationDays(-1) != 0 {
		t.Error("negative days should return 0 duration")
	}

	oneDay := durationDays(1)
	expected := 24 * time.Hour
	if oneDay != expected {
		t.Errorf("Expected %v for 1 day, got %v", expected, oneDay)
	}
}

// TestHasLabel tests the hasLabel helper function
func TestHasLabelETA(t *testing.T) {
	if !hasLabel([]string{"a", "b", "c"}, "b") {
		t.Error("hasLabel should find 'b' in slice")
	}
	if !hasLabel([]string{"API"}, "api") {
		t.Error("hasLabel should be case-insensitive")
	}
	if hasLabel([]string{"a", "b", "c"}, "d") {
		t.Error("hasLabel should not find 'd' in slice")
	}
	if hasLabel([]string{}, "a") {
		t.Error("hasLabel should return false for empty slice")
	}
	if hasLabel(nil, "a") {
		t.Error("hasLabel should return false for nil slice")
	}
}

func TestEstimateETAForIssue_AllIssueTypes(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	types := []struct {
		issueType model.IssueType
		name      string
	}{
		{model.TypeBug, "bug"},
		{model.TypeTask, "task"},
		{model.TypeChore, "chore"},
		{model.TypeFeature, "feature"},
		{model.TypeEpic, "epic"},
	}

	for _, tc := range types {
		t.Run(tc.name, func(t *testing.T) {
			issues := []model.Issue{
				{
					ID:        "test-1",
					Title:     "Test " + tc.name,
					Status:    model.StatusOpen,
					IssueType: tc.issueType,
				},
			}
			eta, err := EstimateETAForIssue(issues, nil, "test-1", 1, now)
			if err != nil {
				t.Fatalf("EstimateETAForIssue failed for %s: %v", tc.name, err)
			}
			if eta.EstimatedMinutes <= 0 {
				t.Errorf("Expected positive minutes for %s, got %d", tc.name, eta.EstimatedMinutes)
			}
		})
	}
}

func TestEstimateETAForIssue_ZeroAgents(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	issues := []model.Issue{
		{
			ID:        "test-1",
			Title:     "Test issue",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
		},
	}

	// Zero agents should normalize to 1
	eta, err := EstimateETAForIssue(issues, nil, "test-1", 0, now)
	if err != nil {
		t.Fatalf("EstimateETAForIssue failed: %v", err)
	}
	if eta.Agents != 1 {
		t.Errorf("Expected agents to be normalized to 1, got %d", eta.Agents)
	}
}

func TestEstimateETAForIssue_DescriptionLengthImpact(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	shortDesc := "Short description"
	longDesc := strings.Repeat("This is a very long description. ", 100) // ~3000 chars

	shortIssue := model.Issue{
		ID:          "short-1",
		Title:       "Short desc issue",
		Status:      model.StatusOpen,
		IssueType:   model.TypeTask,
		Description: shortDesc,
	}

	longIssue := model.Issue{
		ID:          "long-1",
		Title:       "Long desc issue",
		Status:      model.StatusOpen,
		IssueType:   model.TypeTask,
		Description: longDesc,
	}

	shortETA, _ := EstimateETAForIssue([]model.Issue{shortIssue}, nil, "short-1", 1, now)
	longETA, _ := EstimateETAForIssue([]model.Issue{longIssue}, nil, "long-1", 1, now)

	// Longer description should increase complexity
	if longETA.EstimatedMinutes <= shortETA.EstimatedMinutes {
		t.Errorf("Long description should increase estimate: short=%d, long=%d",
			shortETA.EstimatedMinutes, longETA.EstimatedMinutes)
	}
}

func TestEstimateETAForIssue_NoLabelsConfidencePenalty(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	withLabels := model.Issue{
		ID:        "with-labels",
		Title:     "Issue with labels",
		Status:    model.StatusOpen,
		IssueType: model.TypeTask,
		Labels:    []string{"backend", "api"},
	}

	noLabels := model.Issue{
		ID:        "no-labels",
		Title:     "Issue without labels",
		Status:    model.StatusOpen,
		IssueType: model.TypeTask,
		Labels:    []string{},
	}

	withLabelsETA, _ := EstimateETAForIssue([]model.Issue{withLabels}, nil, "with-labels", 1, now)
	noLabelsETA, _ := EstimateETAForIssue([]model.Issue{noLabels}, nil, "no-labels", 1, now)

	// No labels should have lower confidence (penalty applied)
	if noLabelsETA.Confidence >= withLabelsETA.Confidence {
		t.Errorf("No labels should have lower confidence: with=%f, without=%f",
			withLabelsETA.Confidence, noLabelsETA.Confidence)
	}
}

func TestVelocityMinutesPerDayForLabel_EdgeCases(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	since := now.Add(-30 * 24 * time.Hour)

	// No closed issues
	openOnly := []model.Issue{
		{ID: "1", Status: model.StatusOpen},
	}
	v, n := velocityMinutesPerDayForLabel(openOnly, "", since, 60)
	if v != 0 || n != 0 {
		t.Errorf("No closed issues should return 0 velocity: v=%f, n=%d", v, n)
	}

	// Closed issues before window
	oldClosed := now.Add(-60 * 24 * time.Hour) // 60 days ago
	oldClosures := []model.Issue{
		{ID: "1", Status: model.StatusClosed, ClosedAt: &oldClosed},
	}
	v, n = velocityMinutesPerDayForLabel(oldClosures, "", since, 60)
	if v != 0 || n != 0 {
		t.Errorf("Old closures should return 0 velocity: v=%f, n=%d", v, n)
	}

	// Closed issues within window
	recentClosed := now.Add(-7 * 24 * time.Hour) // 7 days ago
	est120 := 120
	recentClosures := []model.Issue{
		{ID: "1", Status: model.StatusClosed, ClosedAt: &recentClosed, EstimatedMinutes: &est120, Labels: []string{"api"}},
	}
	v, n = velocityMinutesPerDayForLabel(recentClosures, "api", since, 60)
	if n != 1 {
		t.Errorf("Expected 1 sample, got %d", n)
	}
	if v <= 0 {
		t.Errorf("Expected positive velocity, got %f", v)
	}
}

func TestEstimateETAConfidence_AllCases(t *testing.T) {
	est120 := 120

	tests := []struct {
		name            string
		issue           model.Issue
		velocitySamples int
		minExpected     float64
		maxExpected     float64
	}{
		{
			name:            "no estimate, no samples, no labels",
			issue:           model.Issue{Labels: nil},
			velocitySamples: 0,
			minExpected:     0.10, // 0.25 - 0.05 - 0.05 = 0.15, clamped to 0.10
			maxExpected:     0.20,
		},
		{
			name:            "with estimate, no samples, with labels",
			issue:           model.Issue{EstimatedMinutes: &est120, Labels: []string{"api"}},
			velocitySamples: 0,
			minExpected:     0.40, // 0.25 + 0.25 - 0.05 = 0.45
			maxExpected:     0.50,
		},
		{
			name:            "with estimate, few samples (1-4), with labels",
			issue:           model.Issue{EstimatedMinutes: &est120, Labels: []string{"api"}},
			velocitySamples: 3,
			minExpected:     0.55, // 0.25 + 0.25 + 0.10 = 0.60
			maxExpected:     0.65,
		},
		{
			name:            "with estimate, medium samples (5-14), with labels",
			issue:           model.Issue{EstimatedMinutes: &est120, Labels: []string{"api"}},
			velocitySamples: 10,
			minExpected:     0.65, // 0.25 + 0.25 + 0.20 = 0.70
			maxExpected:     0.75,
		},
		{
			name:            "with estimate, many samples (15+), with labels",
			issue:           model.Issue{EstimatedMinutes: &est120, Labels: []string{"api"}},
			velocitySamples: 20,
			minExpected:     0.75, // 0.25 + 0.25 + 0.30 = 0.80
			maxExpected:     0.85,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conf := estimateETAConfidence(tc.issue, tc.velocitySamples)
			if conf < tc.minExpected || conf > tc.maxExpected {
				t.Errorf("Expected confidence in [%f, %f], got %f", tc.minExpected, tc.maxExpected, conf)
			}
		})
	}
}

func TestEstimateETAForIssue_GlobalVelocityFallback(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	closedAt := now.Add(-7 * 24 * time.Hour)

	// Issue with labels that have no velocity data, but global velocity exists
	issues := []model.Issue{
		{
			ID:        "target",
			Title:     "Target issue",
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Labels:    []string{"rare-label"}, // No closures for this label
		},
		{
			ID:       "closed-global",
			Title:    "Closed issue",
			Status:   model.StatusClosed,
			ClosedAt: &closedAt,
			Labels:   []string{"other-label"},
		},
	}

	eta, err := EstimateETAForIssue(issues, nil, "target", 1, now)
	if err != nil {
		t.Fatalf("EstimateETAForIssue failed: %v", err)
	}

	// Should fallback to global velocity
	hasGlobalVelocity := false
	for _, f := range eta.Factors {
		if strings.Contains(f, "global") {
			hasGlobalVelocity = true
			break
		}
	}
	if !hasGlobalVelocity {
		t.Error("Expected global velocity fallback in factors")
	}
}
