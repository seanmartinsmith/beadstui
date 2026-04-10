package search

import (
	"errors"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

type stubMetricsLoader struct {
	metrics   map[string]IssueMetrics
	hash      string
	loadErr   error
	hashErr   error
	loadCalls int
}

func (s *stubMetricsLoader) LoadMetrics() (map[string]IssueMetrics, error) {
	s.loadCalls++
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.metrics, nil
}

func (s *stubMetricsLoader) ComputeDataHash() (string, error) {
	if s.hashErr != nil {
		return "", s.hashErr
	}
	return s.hash, nil
}

func TestMetricsCache_Get_RefreshesOnHashChange(t *testing.T) {
	loader := &stubMetricsLoader{
		hash: "hash1",
		metrics: map[string]IssueMetrics{
			"A": {IssueID: "A", PageRank: 0.1, BlockerCount: 2},
		},
	}
	cache := NewMetricsCache(loader)

	metric, ok := cache.Get("A")
	if !ok {
		t.Fatal("expected metric to be found")
	}
	if metric.PageRank != 0.1 {
		t.Fatalf("expected PageRank 0.1, got %f", metric.PageRank)
	}
	if loader.loadCalls != 1 {
		t.Fatalf("expected 1 load call, got %d", loader.loadCalls)
	}

	loader.metrics["A"] = IssueMetrics{IssueID: "A", PageRank: 0.2}
	metric, ok = cache.Get("A")
	if !ok {
		t.Fatal("expected metric to be found on cache hit")
	}
	if metric.PageRank != 0.1 {
		t.Fatalf("expected cached PageRank 0.1, got %f", metric.PageRank)
	}
	if loader.loadCalls != 1 {
		t.Fatalf("expected no additional load calls, got %d", loader.loadCalls)
	}

	loader.hash = "hash2"
	loader.metrics = map[string]IssueMetrics{
		"A": {IssueID: "A", PageRank: 0.2, BlockerCount: 1},
	}
	metric, ok = cache.Get("A")
	if !ok {
		t.Fatal("expected metric to be found after refresh")
	}
	if metric.PageRank != 0.2 {
		t.Fatalf("expected refreshed PageRank 0.2, got %f", metric.PageRank)
	}
	if loader.loadCalls != 2 {
		t.Fatalf("expected 2 load calls after refresh, got %d", loader.loadCalls)
	}
}

func TestMetricsCache_GetBatch_DefaultsForMissing(t *testing.T) {
	loader := &stubMetricsLoader{
		hash: "hash1",
		metrics: map[string]IssueMetrics{
			"A": {IssueID: "A", PageRank: 0.3},
		},
	}
	cache := NewMetricsCache(loader)

	results := cache.GetBatch([]string{"A", "B"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results["A"].PageRank != 0.3 {
		t.Fatalf("expected A PageRank 0.3, got %f", results["A"].PageRank)
	}
	if results["B"].PageRank != defaultPageRank {
		t.Fatalf("expected B default PageRank %f, got %f", defaultPageRank, results["B"].PageRank)
	}
	if results["B"].Priority != 2 {
		t.Fatalf("expected B default priority 2, got %d", results["B"].Priority)
	}
}

func TestMetricsCache_Get_ReturnsDefaultOnError(t *testing.T) {
	loader := &stubMetricsLoader{hashErr: errors.New("boom")}
	cache := NewMetricsCache(loader)

	metric, ok := cache.Get("A")
	if ok {
		t.Fatal("expected ok=false on loader error")
	}
	if metric.PageRank != defaultPageRank {
		t.Fatalf("expected default PageRank %f, got %f", defaultPageRank, metric.PageRank)
	}
}

func TestAnalyzerMetricsLoader_LoadMetrics(t *testing.T) {
	now := time.Date(2025, 12, 18, 12, 0, 0, 0, time.UTC)
	dep := &model.Dependency{
		IssueID:     "A",
		DependsOnID: "B",
		Type:        model.DepBlocks,
	}
	issueA := model.Issue{
		ID:           "A",
		Title:        "Issue A",
		Status:       model.StatusOpen,
		IssueType:    model.TypeTask,
		Priority:     2,
		CreatedAt:    now,
		UpdatedAt:    now,
		Dependencies: []*model.Dependency{dep},
	}
	issueB := model.Issue{
		ID:        "B",
		Title:     "Issue B",
		Status:    model.StatusBlocked,
		IssueType: model.TypeTask,
		Priority:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	loader := NewAnalyzerMetricsLoader([]model.Issue{issueA, issueB})
	metrics, err := loader.LoadMetrics()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics entries, got %d", len(metrics))
	}

	metricA := metrics["A"]
	metricB := metrics["B"]

	if metricA.BlockerCount != 0 {
		t.Fatalf("expected A blocker count 0, got %d", metricA.BlockerCount)
	}
	if metricB.BlockerCount != 1 {
		t.Fatalf("expected B blocker count 1, got %d", metricB.BlockerCount)
	}
	if metricA.Status != string(model.StatusOpen) {
		t.Fatalf("expected A status open, got %q", metricA.Status)
	}
	if metricA.Priority != 2 {
		t.Fatalf("expected A priority 2, got %d", metricA.Priority)
	}
	if !metricA.UpdatedAt.Equal(now) {
		t.Fatalf("expected A UpdatedAt %v, got %v", now, metricA.UpdatedAt)
	}

	hash, err := loader.ComputeDataHash()
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}
	expectedHash := analysis.ComputeDataHash([]model.Issue{issueA, issueB})
	if hash != expectedHash {
		t.Fatalf("expected hash %q, got %q", expectedHash, hash)
	}
}
