package search

import (
	"path/filepath"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/loader"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func loadHybridFixtureIssues(t *testing.T) []model.Issue {
	t.Helper()
	path := filepath.Join("..", "..", "tests", "testdata", "search_hybrid.jsonl")
	issues, err := loader.LoadIssuesFromFile(path)
	if err != nil {
		t.Fatalf("load fixture issues: %v", err)
	}
	if len(issues) == 0 {
		t.Fatalf("fixture issues empty")
	}
	return issues
}

func TestHybridScorer_RealMetrics(t *testing.T) {
	issues := loadHybridFixtureIssues(t)
	cache := NewMetricsCache(NewAnalyzerMetricsLoader(issues))
	if err := cache.Refresh(); err != nil {
		t.Fatalf("refresh metrics cache: %v", err)
	}

	weights, err := GetPreset(PresetDefault)
	if err != nil {
		t.Fatalf("preset default: %v", err)
	}

	scorer := NewHybridScorer(weights, cache)
	metrics, ok := cache.Get("sh-1")
	if !ok {
		t.Fatalf("expected metrics for sh-1")
	}

	textScore := 0.75
	result, err := scorer.Score("sh-1", textScore)
	if err != nil {
		t.Fatalf("score sh-1: %v", err)
	}

	expected := weights.TextRelevance*textScore +
		weights.PageRank*metrics.PageRank +
		weights.Status*normalizeStatus(metrics.Status) +
		weights.Impact*normalizeImpact(metrics.BlockerCount, cache.MaxBlockerCount()) +
		weights.Priority*normalizePriority(metrics.Priority) +
		weights.Recency*normalizeRecency(metrics.UpdatedAt)

	if diff := result.FinalScore - expected; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("expected final score %f, got %f", expected, result.FinalScore)
	}

	if result.ComponentScores["pagerank"] != metrics.PageRank {
		t.Fatalf("pagerank component mismatch: %f vs %f", result.ComponentScores["pagerank"], metrics.PageRank)
	}
}

func TestHybridScorer_RealMetricsOrdering(t *testing.T) {
	issues := loadHybridFixtureIssues(t)
	cache := NewMetricsCache(NewAnalyzerMetricsLoader(issues))
	if err := cache.Refresh(); err != nil {
		t.Fatalf("refresh metrics cache: %v", err)
	}

	weights, err := GetPreset(PresetDefault)
	if err != nil {
		t.Fatalf("preset default: %v", err)
	}
	scorer := NewHybridScorer(weights, cache)

	textScore := 0.6
	left, err := scorer.Score("sh-1", textScore)
	if err != nil {
		t.Fatalf("score sh-1: %v", err)
	}
	right, err := scorer.Score("sh-4", textScore)
	if err != nil {
		t.Fatalf("score sh-4: %v", err)
	}

	if left.FinalScore <= right.FinalScore {
		t.Fatalf("expected sh-1 score %f to exceed sh-4 score %f", left.FinalScore, right.FinalScore)
	}
}
