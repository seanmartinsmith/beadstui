package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestTimingMetric_Record(t *testing.T) {
	m := newTimingMetric("test")

	// Record some measurements
	m.Record(100 * time.Millisecond)
	m.Record(200 * time.Millisecond)
	m.Record(50 * time.Millisecond)

	if m.Count() != 3 {
		t.Errorf("Count = %d; want 3", m.Count())
	}

	// Total should be 350ms = 350,000,000 ns
	expectedTotal := int64(350 * time.Millisecond)
	if m.TotalNs() != expectedTotal {
		t.Errorf("TotalNs = %d; want %d", m.TotalNs(), expectedTotal)
	}

	// Max should be 200ms
	expectedMax := int64(200 * time.Millisecond)
	if m.MaxNs() != expectedMax {
		t.Errorf("MaxNs = %d; want %d", m.MaxNs(), expectedMax)
	}

	// Min should be 50ms
	expectedMin := int64(50 * time.Millisecond)
	if m.MinNs() != expectedMin {
		t.Errorf("MinNs = %d; want %d", m.MinNs(), expectedMin)
	}

	// Avg should be ~116.67ms
	expectedAvg := int64(350 * time.Millisecond / 3)
	if m.AvgNs() != expectedAvg {
		t.Errorf("AvgNs = %d; want %d", m.AvgNs(), expectedAvg)
	}
}

func TestTimingMetric_Reset(t *testing.T) {
	m := newTimingMetric("test")
	m.Record(100 * time.Millisecond)

	if m.Count() != 1 {
		t.Errorf("Count = %d; want 1", m.Count())
	}

	m.Reset()

	if m.Count() != 0 {
		t.Errorf("Count after reset = %d; want 0", m.Count())
	}
	if m.TotalNs() != 0 {
		t.Errorf("TotalNs after reset = %d; want 0", m.TotalNs())
	}
	if m.MaxNs() != 0 {
		t.Errorf("MaxNs after reset = %d; want 0", m.MaxNs())
	}
}

func TestTimingMetric_Stats(t *testing.T) {
	m := newTimingMetric("test_metric")
	m.Record(100 * time.Millisecond)
	m.Record(200 * time.Millisecond)

	stats := m.Stats()

	if stats.Name != "test_metric" {
		t.Errorf("Name = %q; want %q", stats.Name, "test_metric")
	}
	if stats.Count != 2 {
		t.Errorf("Count = %d; want 2", stats.Count)
	}
	if stats.TotalMs != 300 {
		t.Errorf("TotalMs = %f; want 300", stats.TotalMs)
	}
	if stats.MaxMs != 200 {
		t.Errorf("MaxMs = %f; want 200", stats.MaxMs)
	}
}

func TestTimingMetric_ConcurrentAccess(t *testing.T) {
	m := newTimingMetric("concurrent")

	var wg sync.WaitGroup
	iterations := 1000
	goroutines := 10

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				m.Record(time.Duration(i) * time.Microsecond)
			}
		}()
	}

	wg.Wait()

	expectedCount := int64(goroutines * iterations)
	if m.Count() != expectedCount {
		t.Errorf("Count = %d; want %d", m.Count(), expectedCount)
	}
}

func TestTimer(t *testing.T) {
	m := newTimingMetric("timer_test")

	// Use Timer
	func() {
		defer Timer(m)()
		time.Sleep(10 * time.Millisecond)
	}()

	if m.Count() != 1 {
		t.Errorf("Count = %d; want 1", m.Count())
	}

	// Should have recorded at least 10ms
	if m.TotalNs() < int64(10*time.Millisecond) {
		t.Errorf("TotalNs = %d; want >= %d", m.TotalNs(), int64(10*time.Millisecond))
	}
}

func TestTimer_Nil(t *testing.T) {
	// Timer with nil metric should not panic
	done := Timer(nil)
	done() // Should not panic
}

func TestCacheMetric_HitMiss(t *testing.T) {
	m := newCacheMetric("test_cache")

	m.Hit()
	m.Hit()
	m.Miss()

	if m.Hits() != 2 {
		t.Errorf("Hits = %d; want 2", m.Hits())
	}
	if m.Misses() != 1 {
		t.Errorf("Misses = %d; want 1", m.Misses())
	}
	if m.Total() != 3 {
		t.Errorf("Total = %d; want 3", m.Total())
	}

	// Hit rate should be 2/3
	expectedRate := 2.0 / 3.0
	if m.HitRate() != expectedRate {
		t.Errorf("HitRate = %f; want %f", m.HitRate(), expectedRate)
	}
}

func TestCacheMetric_HitRateEmpty(t *testing.T) {
	m := newCacheMetric("empty")

	if m.HitRate() != 0 {
		t.Errorf("HitRate for empty cache = %f; want 0", m.HitRate())
	}
}

func TestCacheMetric_Reset(t *testing.T) {
	m := newCacheMetric("test")
	m.Hit()
	m.Miss()

	m.Reset()

	if m.Hits() != 0 {
		t.Errorf("Hits after reset = %d; want 0", m.Hits())
	}
	if m.Misses() != 0 {
		t.Errorf("Misses after reset = %d; want 0", m.Misses())
	}
}

func TestCacheMetric_Stats(t *testing.T) {
	m := newCacheMetric("test_cache")
	m.Hit()
	m.Hit()
	m.Hit()
	m.Miss()

	stats := m.Stats()

	if stats.Name != "test_cache" {
		t.Errorf("Name = %q; want %q", stats.Name, "test_cache")
	}
	if stats.Hits != 3 {
		t.Errorf("Hits = %d; want 3", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Misses = %d; want 1", stats.Misses)
	}
	if stats.Total != 4 {
		t.Errorf("Total = %d; want 4", stats.Total)
	}
	if stats.HitRate != 0.75 {
		t.Errorf("HitRate = %f; want 0.75", stats.HitRate)
	}
}

func TestCacheMetric_ConcurrentAccess(t *testing.T) {
	m := newCacheMetric("concurrent")

	var wg sync.WaitGroup
	iterations := 1000
	goroutines := 10

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if i%3 == 0 {
					m.Miss()
				} else {
					m.Hit()
				}
			}
		}()
	}

	wg.Wait()

	expectedTotal := int64(goroutines * iterations)
	if m.Total() != expectedTotal {
		t.Errorf("Total = %d; want %d", m.Total(), expectedTotal)
	}
}

func TestGetMemoryStats(t *testing.T) {
	stats := GetMemoryStats()

	// Should have some heap allocation
	if stats.HeapAllocMB <= 0 {
		t.Errorf("HeapAllocMB = %f; want > 0", stats.HeapAllocMB)
	}

	// Should have at least the test goroutine
	if stats.GoroutineCount < 1 {
		t.Errorf("GoroutineCount = %d; want >= 1", stats.GoroutineCount)
	}
}

func TestAllTimingMetrics(t *testing.T) {
	metrics := AllTimingMetrics()
	if len(metrics) == 0 {
		t.Error("AllTimingMetrics returned empty slice")
	}

	// Check that CycleDetection is in the list
	found := false
	for _, m := range metrics {
		if m == CycleDetection {
			found = true
			break
		}
	}
	if !found {
		t.Error("CycleDetection not in AllTimingMetrics")
	}
}

func TestAllCacheMetrics(t *testing.T) {
	metrics := AllCacheMetrics()
	if len(metrics) == 0 {
		t.Error("AllCacheMetrics returned empty slice")
	}

	// Check that GraphCache is in the list
	found := false
	for _, m := range metrics {
		if m == GraphCache {
			found = true
			break
		}
	}
	if !found {
		t.Error("GraphCache not in AllCacheMetrics")
	}
}

func TestResetAll(t *testing.T) {
	// Record some data
	CycleDetection.Record(100 * time.Millisecond)
	GraphCache.Hit()

	// Reset all
	ResetAll()

	// Verify reset
	if CycleDetection.Count() != 0 {
		t.Errorf("CycleDetection.Count after reset = %d; want 0", CycleDetection.Count())
	}
	if GraphCache.Total() != 0 {
		t.Errorf("GraphCache.Total after reset = %d; want 0", GraphCache.Total())
	}
}

func TestGetAllMetrics(t *testing.T) {
	ResetAll()

	// Record some data
	CycleDetection.Record(100 * time.Millisecond)
	GraphCache.Hit()
	GraphCache.Miss()

	output := GetAllMetrics()

	// Should have timing data
	if len(output.Timing) == 0 {
		t.Error("GetAllMetrics returned no timing data")
	}

	// Should have cache data
	if len(output.Cache) == 0 {
		t.Error("GetAllMetrics returned no cache data")
	}

	// Should have memory data
	if output.Memory.HeapAllocMB <= 0 {
		t.Error("GetAllMetrics returned invalid memory data")
	}
}

func TestEnabled(t *testing.T) {
	// Save original state
	originalEnabled := enabled

	// Test enabled
	SetEnabled(true)
	if !Enabled() {
		t.Error("Enabled() = false; want true")
	}

	// Test disabled
	SetEnabled(false)
	if Enabled() {
		t.Error("Enabled() = true; want false")
	}

	// When disabled, Record should be no-op
	m := newTimingMetric("disabled_test")
	m.Record(100 * time.Millisecond)
	if m.Count() != 0 {
		t.Errorf("Count when disabled = %d; want 0", m.Count())
	}

	// Restore original state
	SetEnabled(originalEnabled)
}

func BenchmarkTimingMetric_Record(b *testing.B) {
	m := newTimingMetric("bench")
	d := 100 * time.Microsecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Record(d)
	}
}

func BenchmarkCacheMetric_Hit(b *testing.B) {
	m := newCacheMetric("bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Hit()
	}
}

func BenchmarkTimer(b *testing.B) {
	m := newTimingMetric("bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		done := Timer(m)
		done()
	}
}
