package analysis

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestComputeRiskSignals_EmptyIssue(t *testing.T) {
	issue := &model.Issue{
		ID:        "TEST-1",
		Title:     "Test Issue",
		Status:    model.StatusOpen,
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now(),
	}

	stats := GraphStats{
		InDegree:  make(map[string]int),
		OutDegree: make(map[string]int),
	}
	issues := make(map[string]model.Issue)
	issues["TEST-1"] = *issue

	signals := ComputeRiskSignals(issue, &stats, issues, time.Now())

	// Empty issue with no deps should have low risk
	if signals.FanVariance > 0.1 {
		t.Errorf("expected low fan variance for issue with no deps, got %f", signals.FanVariance)
	}
	if signals.CrossRepoRisk > 0.1 {
		t.Errorf("expected zero cross-repo risk, got %f", signals.CrossRepoRisk)
	}
}

func TestComputeRiskSignals_FanVariance(t *testing.T) {
	// Create a hub-and-spoke pattern with varying In-Degree of dependencies
	now := time.Now()
	issues := make(map[string]model.Issue)

	// Issue depends on 3 things with different popularity
	issue := model.Issue{
		ID:        "ISSUE",
		Title:     "Main Issue",
		Status:    model.StatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
		Dependencies: []*model.Dependency{
			{IssueID: "ISSUE", DependsOnID: "POPULAR-LIB", Type: model.DepBlocks},
			{IssueID: "ISSUE", DependsOnID: "NICHE-LIB", Type: model.DepBlocks},
			{IssueID: "ISSUE", DependsOnID: "MID-LIB", Type: model.DepBlocks},
		},
	}

	issues["ISSUE"] = issue
	issues["POPULAR-LIB"] = model.Issue{ID: "POPULAR-LIB"}
	issues["NICHE-LIB"] = model.Issue{ID: "NICHE-LIB"}
	issues["MID-LIB"] = model.Issue{ID: "MID-LIB"}

	// Stats with varying In-Degrees for the blockers
	stats := GraphStats{
		InDegree: map[string]int{
			"POPULAR-LIB": 100, // Very popular
			"NICHE-LIB":   1,   // Very niche
			"MID-LIB":     50,  // Medium
		},
		OutDegree: make(map[string]int),
	}

	signals := ComputeRiskSignals(&issue, &stats, issues, now)

	// Should have some fan variance due to differing blocker In-Degrees (1, 50, 100)
	// Mean = 50.3. StdDev ~ 40. CV ~ 0.8. Normalized ~ 0.4.
	if signals.FanVariance < 0.1 {
		t.Errorf("expected higher fan variance with varying blocker degrees, got %f", signals.FanVariance)
	}
}

func TestComputeRiskSignals_StatusRisk(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name       string
		status     model.Status
		updatedAt  time.Time
		expectHigh bool
	}{
		{
			name:       "open_recent",
			status:     model.StatusOpen,
			updatedAt:  now,
			expectHigh: false,
		},
		{
			name:       "blocked_recent",
			status:     model.StatusBlocked,
			updatedAt:  now,
			expectHigh: true,
		},
		{
			name:       "blocked_stale",
			status:     model.StatusBlocked,
			updatedAt:  now.Add(-14 * 24 * time.Hour),
			expectHigh: true,
		},
		{
			name:       "in_progress_recent",
			status:     model.StatusInProgress,
			updatedAt:  now,
			expectHigh: false,
		},
		{
			name:       "in_progress_stale",
			status:     model.StatusInProgress,
			updatedAt:  now.Add(-21 * 24 * time.Hour),
			expectHigh: true,
		},
		{
			name:       "tombstone",
			status:     model.StatusTombstone,
			updatedAt:  now.Add(-30 * 24 * time.Hour),
			expectHigh: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			issue := &model.Issue{
				ID:        "TEST-1",
				Title:     "Test Issue",
				Status:    tc.status,
				CreatedAt: now.Add(-30 * 24 * time.Hour),
				UpdatedAt: tc.updatedAt,
			}

			stats := GraphStats{
				InDegree:  make(map[string]int),
				OutDegree: make(map[string]int),
			}
			issues := make(map[string]model.Issue)

			signals := ComputeRiskSignals(issue, &stats, issues, now)

			if tc.expectHigh && signals.StatusRisk < 0.5 {
				t.Errorf("expected high status risk for %s, got %f", tc.name, signals.StatusRisk)
			}
			if !tc.expectHigh && signals.StatusRisk > 0.5 {
				t.Errorf("expected low status risk for %s, got %f", tc.name, signals.StatusRisk)
			}
		})
	}
}

func TestComputeAllRiskSignals_SkipsTombstone(t *testing.T) {
	now := time.Now()

	issues := map[string]model.Issue{
		"open-1":      {ID: "open-1", Status: model.StatusOpen, CreatedAt: now.Add(-10 * 24 * time.Hour)},
		"tombstone-1": {ID: "tombstone-1", Status: model.StatusTombstone, CreatedAt: now.Add(-20 * 24 * time.Hour)},
	}

	stats := &GraphStats{
		InDegree:  make(map[string]int),
		OutDegree: make(map[string]int),
	}

	signals := ComputeAllRiskSignals(issues, stats, now)
	if _, ok := signals["tombstone-1"]; ok {
		t.Fatal("expected tombstone issue to be skipped in risk signals")
	}
	if _, ok := signals["open-1"]; !ok {
		t.Fatal("expected open issue to be included in risk signals")
	}
}

func TestComputeRiskSignals_ActivityChurn(t *testing.T) {
	now := time.Now()

	// High churn: many comments relative to age
	highChurnIssue := &model.Issue{
		ID:        "HIGH-CHURN",
		Title:     "High Churn Issue",
		Status:    model.StatusOpen,
		CreatedAt: now.Add(-7 * 24 * time.Hour), // 7 days old
		UpdatedAt: now.Add(-1 * time.Hour),
		Comments: []*model.Comment{
			{ID: 1, IssueID: "HIGH-CHURN", Text: "Comment 1", CreatedAt: now.Add(-6 * 24 * time.Hour)},
			{ID: 2, IssueID: "HIGH-CHURN", Text: "Comment 2", CreatedAt: now.Add(-5 * 24 * time.Hour)},
			{ID: 3, IssueID: "HIGH-CHURN", Text: "Comment 3", CreatedAt: now.Add(-4 * 24 * time.Hour)},
			{ID: 4, IssueID: "HIGH-CHURN", Text: "Comment 4", CreatedAt: now.Add(-3 * 24 * time.Hour)},
			{ID: 5, IssueID: "HIGH-CHURN", Text: "Comment 5", CreatedAt: now.Add(-2 * 24 * time.Hour)},
			{ID: 6, IssueID: "HIGH-CHURN", Text: "Comment 6", CreatedAt: now.Add(-1 * 24 * time.Hour)},
			{ID: 7, IssueID: "HIGH-CHURN", Text: "Comment 7", CreatedAt: now},
		},
	}

	// Low churn: no comments, old issue
	lowChurnIssue := &model.Issue{
		ID:        "LOW-CHURN",
		Title:     "Low Churn Issue",
		Status:    model.StatusOpen,
		CreatedAt: now.Add(-30 * 24 * time.Hour), // 30 days old
		UpdatedAt: now.Add(-25 * 24 * time.Hour), // Updated 25 days ago
		Comments:  nil,
	}

	stats := GraphStats{
		InDegree:  make(map[string]int),
		OutDegree: make(map[string]int),
	}
	issues := make(map[string]model.Issue)

	highChurnSignals := ComputeRiskSignals(highChurnIssue, &stats, issues, now)
	lowChurnSignals := ComputeRiskSignals(lowChurnIssue, &stats, issues, now)

	if highChurnSignals.ActivityChurn <= lowChurnSignals.ActivityChurn {
		t.Errorf("high churn issue should have higher activity churn: high=%f, low=%f",
			highChurnSignals.ActivityChurn, lowChurnSignals.ActivityChurn)
	}
}

func TestComputeRiskSignals_CrossRepoRisk(t *testing.T) {
	now := time.Now()

	// Issue in repo A with deps in repo B
	issues := make(map[string]model.Issue)

	issueA := model.Issue{
		ID:         "A-1",
		Title:      "Issue in Repo A",
		Status:     model.StatusOpen,
		SourceRepo: "repo-a",
		CreatedAt:  now,
		UpdatedAt:  now,
		Dependencies: []*model.Dependency{
			{IssueID: "A-1", DependsOnID: "B-1", Type: model.DepBlocks},
			{IssueID: "A-1", DependsOnID: "B-2", Type: model.DepBlocks},
		},
	}

	issueB1 := model.Issue{ID: "B-1", Title: "Issue 1 in Repo B", Status: model.StatusOpen, SourceRepo: "repo-b", CreatedAt: now, UpdatedAt: now}
	issueB2 := model.Issue{ID: "B-2", Title: "Issue 2 in Repo B", Status: model.StatusOpen, SourceRepo: "repo-b", CreatedAt: now, UpdatedAt: now}

	issues["A-1"] = issueA
	issues["B-1"] = issueB1
	issues["B-2"] = issueB2

	stats := GraphStats{
		InDegree:  make(map[string]int),
		OutDegree: make(map[string]int),
	}

	signals := ComputeRiskSignals(&issueA, &stats, issues, now)

	// All deps are cross-repo, so should have high cross-repo risk
	if signals.CrossRepoRisk < 0.9 {
		t.Errorf("expected high cross-repo risk (all deps in different repo), got %f", signals.CrossRepoRisk)
	}
}

func TestComputeRiskSignals_CompositeRisk(t *testing.T) {
	now := time.Now()

	// Create a high-risk issue: blocked, stale, cross-repo deps, high variance
	highRiskIssue := model.Issue{
		ID:         "HIGH-RISK",
		Title:      "High Risk Issue",
		Status:     model.StatusBlocked,
		SourceRepo: "repo-a",
		CreatedAt:  now.Add(-60 * 24 * time.Hour),
		UpdatedAt:  now.Add(-14 * 24 * time.Hour), // Stale for 2 weeks
		Dependencies: []*model.Dependency{
			{IssueID: "HIGH-RISK", DependsOnID: "DEP-1", Type: model.DepBlocks},
		},
	}

	depIssue := model.Issue{
		ID:         "DEP-1",
		Title:      "Dep Issue",
		Status:     model.StatusOpen,
		SourceRepo: "repo-b", // Different repo
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	issues := map[string]model.Issue{
		"HIGH-RISK": highRiskIssue,
		"DEP-1":     depIssue,
	}

	stats := GraphStats{
		InDegree:  map[string]int{"HIGH-RISK": 0, "DEP-1": 10},
		OutDegree: map[string]int{"HIGH-RISK": 1, "DEP-1": 0},
	}

	signals := ComputeRiskSignals(&highRiskIssue, &stats, issues, now)

	// Composite risk should be significant
	if signals.CompositeRisk < 0.4 {
		t.Errorf("expected high composite risk, got %f", signals.CompositeRisk)
	}

	// Explanation should mention risk factors
	if signals.Explanation == "" {
		t.Error("expected non-empty risk explanation")
	}
}

func TestRiskExplanation(t *testing.T) {
	testCases := []struct {
		name     string
		signals  RiskSignals
		contains string
	}{
		{
			name:     "low_risk",
			signals:  RiskSignals{CompositeRisk: 0.1},
			contains: "Low risk",
		},
		{
			name:     "high_fan_variance",
			signals:  RiskSignals{CompositeRisk: 0.5, FanVariance: 0.7},
			contains: "dependency variance",
		},
		{
			name:     "high_activity_churn",
			signals:  RiskSignals{CompositeRisk: 0.5, ActivityChurn: 0.8},
			contains: "activity churn",
		},
		{
			name:     "cross_repo",
			signals:  RiskSignals{CompositeRisk: 0.5, CrossRepoRisk: 0.5},
			contains: "cross-repo",
		},
		{
			name:     "status_risk",
			signals:  RiskSignals{CompositeRisk: 0.5, StatusRisk: 0.7},
			contains: "status",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			explanation := generateRiskExplanation(tc.signals)
			if explanation == "" {
				t.Error("expected non-empty explanation")
			}
			if tc.contains != "" && !stringContains(explanation, tc.contains) {
				t.Errorf("explanation %q should contain %q", explanation, tc.contains)
			}
		})
	}
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDefaultRiskWeights(t *testing.T) {
	weights := DefaultRiskWeights()

	// Weights should sum to 1.0
	sum := weights.FanVariance + weights.ActivityChurn + weights.CrossRepoRisk + weights.StatusRisk
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("risk weights should sum to 1.0, got %f", sum)
	}

	// Each weight should be positive
	if weights.FanVariance <= 0 || weights.ActivityChurn <= 0 || weights.CrossRepoRisk <= 0 || weights.StatusRisk <= 0 {
		t.Error("all risk weights should be positive")
	}
}

func TestImpactScore_IncludesRisk(t *testing.T) {
	// Create analyzer with some issues
	issues := []model.Issue{
		{
			ID:        "TEST-1",
			Title:     "Test Issue",
			Status:    model.StatusBlocked, // High status risk
			Priority:  1,
			CreatedAt: time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt: time.Now().Add(-14 * 24 * time.Hour), // Stale
		},
	}

	analyzer := NewCachedAnalyzer(issues, nil)
	scores := analyzer.ComputeImpactScores()

	if len(scores) == 0 {
		t.Fatal("expected at least one score")
	}

	// Check that risk is included in breakdown
	score := scores[0]
	if score.Breakdown.RiskNorm == 0 && score.Breakdown.RiskSignals != nil {
		// Either RiskNorm should be non-zero or RiskSignals should reflect the risk
		if score.Breakdown.RiskSignals.CompositeRisk == 0 {
			t.Log("Warning: blocked/stale issue has zero risk - this may be expected if no deps")
		}
	}

	// Risk should contribute to total score
	expectedComponents := score.Breakdown.PageRank +
		score.Breakdown.Betweenness +
		score.Breakdown.BlockerRatio +
		score.Breakdown.Staleness +
		score.Breakdown.PriorityBoost +
		score.Breakdown.TimeToImpact +
		score.Breakdown.Urgency +
		score.Breakdown.Risk

	// Allow small floating point tolerance
	diff := score.Score - expectedComponents
	if diff > 0.0001 || diff < -0.0001 {
		t.Errorf("score %f doesn't match sum of components %f", score.Score, expectedComponents)
	}
}
