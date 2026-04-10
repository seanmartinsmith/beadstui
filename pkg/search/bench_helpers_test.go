package search

import (
	"fmt"
	"testing"
	"time"
)

type benchmarkMetricsLoader struct {
	metrics  map[string]IssueMetrics
	dataHash string
}

func (l *benchmarkMetricsLoader) LoadMetrics() (map[string]IssueMetrics, error) {
	return l.metrics, nil
}

func (l *benchmarkMetricsLoader) ComputeDataHash() (string, error) {
	return l.dataHash, nil
}

func buildBenchmarkMetricsCache(tb testing.TB, size int) MetricsCache {
	tb.Helper()
	loader := &benchmarkMetricsLoader{
		metrics:  buildBenchmarkMetrics(size),
		dataHash: fmt.Sprintf("bench-%d", size),
	}
	cache := NewMetricsCache(loader)
	if err := cache.Refresh(); err != nil {
		tb.Fatalf("Refresh metrics cache: %v", err)
	}
	return cache
}

func buildBenchmarkMetrics(size int) map[string]IssueMetrics {
	metrics := make(map[string]IssueMetrics, size)
	statuses := []string{"open", "in_progress", "blocked", "closed"}
	base := time.Now().Add(-90 * 24 * time.Hour)
	for i := 0; i < size; i++ {
		id := fmt.Sprintf("issue-%d", i)
		metrics[id] = IssueMetrics{
			IssueID:      id,
			PageRank:     float64(i%100) / 100.0,
			Status:       statuses[i%len(statuses)],
			Priority:     i % 5,
			BlockerCount: i % 10,
			UpdatedAt:    base.Add(time.Duration(i%90) * 24 * time.Hour),
		}
	}
	return metrics
}

func buildBenchmarkIssueIDs(size int) []string {
	ids := make([]string, size)
	for i := 0; i < size; i++ {
		ids[i] = fmt.Sprintf("issue-%d", i)
	}
	return ids
}
