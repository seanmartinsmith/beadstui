package search

import (
	"runtime"
	"testing"
)

func BenchmarkMetricsCacheGet(b *testing.B) {
	cache := buildBenchmarkMetricsCache(b, 1000)
	ids := buildBenchmarkIssueIDs(1000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ids[i%len(ids)])
	}
}

func BenchmarkMetricsCacheGetBatch(b *testing.B) {
	cache := buildBenchmarkMetricsCache(b, 1000)
	ids := buildBenchmarkIssueIDs(100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.GetBatch(ids)
	}
}

func BenchmarkMetricsCacheMemory(b *testing.B) {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	before := m.Alloc

	loader := &benchmarkMetricsLoader{
		metrics:  buildBenchmarkMetrics(10000),
		dataHash: "bench-10000",
	}
	cache := NewMetricsCache(loader)
	if err := cache.Refresh(); err != nil {
		b.Fatalf("Refresh metrics cache: %v", err)
	}

	runtime.GC()
	runtime.ReadMemStats(&m)
	after := m.Alloc

	_, _ = cache.Get("issue-0")
	b.ReportMetric(float64(after-before)/1024.0, "KB")
}
