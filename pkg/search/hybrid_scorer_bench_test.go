package search

import (
	"testing"
	"time"
)

func BenchmarkHybridScorerScore(b *testing.B) {
	cache := buildBenchmarkMetricsCache(b, 1000)
	weights, err := GetPreset(PresetDefault)
	if err != nil {
		b.Fatalf("preset default: %v", err)
	}
	scorer := NewHybridScorer(weights, cache)
	ids := buildBenchmarkIssueIDs(1000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ids[i%len(ids)]
		if _, err := scorer.Score(id, 0.75); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNormalizers(b *testing.B) {
	b.Run("status", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = normalizeStatus("open")
		}
	})
	b.Run("priority", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = normalizePriority(2)
		}
	})
	b.Run("recency", func(b *testing.B) {
		updated := time.Now().Add(-30 * 24 * time.Hour)
		for i := 0; i < b.N; i++ {
			_ = normalizeRecency(updated)
		}
	})
}
