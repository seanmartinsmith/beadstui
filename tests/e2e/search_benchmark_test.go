package main_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/search"
)

type benchmarkMetricsLoader struct {
	metrics  map[string]search.IssueMetrics
	dataHash string
}

func (l *benchmarkMetricsLoader) LoadMetrics() (map[string]search.IssueMetrics, error) {
	return l.metrics, nil
}

func (l *benchmarkMetricsLoader) ComputeDataHash() (string, error) {
	return l.dataHash, nil
}

func buildBenchmarkData(b *testing.B, size int) (*search.VectorIndex, search.MetricsCache, []float32) {
	b.Helper()
	embedder := search.NewHashEmbedder(search.DefaultEmbeddingDim)
	idx := search.NewVectorIndex(embedder.Dim())

	docs := make(map[string]string, size)
	metrics := make(map[string]search.IssueMetrics, size)
	statuses := []string{"open", "in_progress", "blocked", "closed"}
	base := time.Now().Add(-90 * 24 * time.Hour)
	for i := 0; i < size; i++ {
		id := fmt.Sprintf("issue-%d", i)
		docs[id] = fmt.Sprintf("Issue %d about authentication and search ranking", i)
		metrics[id] = search.IssueMetrics{
			IssueID:      id,
			PageRank:     float64(i%100) / 100.0,
			Status:       statuses[i%len(statuses)],
			Priority:     i % 5,
			BlockerCount: i % 10,
			UpdatedAt:    base.Add(time.Duration(i%90) * 24 * time.Hour),
		}
	}

	if _, err := search.SyncVectorIndex(context.Background(), idx, embedder, docs, 128); err != nil {
		b.Fatalf("SyncVectorIndex: %v", err)
	}

	queryVecs, err := embedder.Embed(context.Background(), []string{"authentication"})
	if err != nil {
		b.Fatalf("Embed query: %v", err)
	}
	if len(queryVecs) != 1 {
		b.Fatalf("Embed query returned %d vectors", len(queryVecs))
	}

	loader := &benchmarkMetricsLoader{metrics: metrics, dataHash: fmt.Sprintf("bench-%d", size)}
	cache := search.NewMetricsCache(loader)
	if err := cache.Refresh(); err != nil {
		b.Fatalf("Refresh metrics cache: %v", err)
	}

	return idx, cache, queryVecs[0]
}

func BenchmarkSearchTextVsHybrid(b *testing.B) {
	idx, cache, query := buildBenchmarkData(b, 1000)
	weights, err := search.GetPreset(search.PresetDefault)
	if err != nil {
		b.Fatalf("preset default: %v", err)
	}

	b.Run("text-only", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := idx.SearchTopK(query, 10); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("hybrid-k10", func(b *testing.B) {
		scorer := search.NewHybridScorer(weights, cache)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			results, err := idx.SearchTopK(query, 10)
			if err != nil {
				b.Fatal(err)
			}
			for _, r := range results {
				if _, err := scorer.Score(r.IssueID, r.Score); err != nil {
					b.Fatal(err)
				}
			}
		}
	})

	b.Run("hybrid-k50", func(b *testing.B) {
		scorer := search.NewHybridScorer(weights, cache)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			results, err := idx.SearchTopK(query, 50)
			if err != nil {
				b.Fatal(err)
			}
			for _, r := range results {
				if _, err := scorer.Score(r.IssueID, r.Score); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}

func BenchmarkSearchAtScale(b *testing.B) {
	for _, size := range []int{100, 1000, 5000, 10000} {
		size := size
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			if testing.Short() && size > 1000 {
				b.Skip("skip large scale in short mode")
			}
			idx, cache, query := buildBenchmarkData(b, size)
			weights, err := search.GetPreset(search.PresetDefault)
			if err != nil {
				b.Fatalf("preset default: %v", err)
			}
			scorer := search.NewHybridScorer(weights, cache)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results, err := idx.SearchTopK(query, 10)
				if err != nil {
					b.Fatal(err)
				}
				for _, r := range results {
					if _, err := scorer.Score(r.IssueID, r.Score); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
