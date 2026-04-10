// Package cass provides safety tests for the cass integration.
// These tests verify the critical safety guarantee: cass integration is completely
// invisible when cass is not installed or not working.
//
// This is the MOST IMPORTANT TEST FILE for the cass feature.
// From the epic: "Users without cass must NEVER see error messages, 'no sessions found'
// states, broken UI, or loading indicators for cass features."
package cass

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// SCENARIO 1: No cass Binary
// Environment: cass not in PATH
// Expected: All bv functionality works exactly as before
// =============================================================================

func TestSafety_NoCassBinary_DetectorReturnsNotInstalled(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "", errors.New("executable file not found in $PATH")
	}

	status := d.Check()
	if status != StatusNotInstalled {
		t.Errorf("Check() = %v, want StatusNotInstalled when cass not in PATH", status)
	}
}

func TestSafety_NoCassBinary_SearcherReturnsEmptyNotError(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "", errors.New("executable file not found in $PATH")
	}

	s := NewSearcher(d)
	resp := s.Search(context.Background(), SearchOptions{Query: "test"})

	// CRITICAL: Results must never be nil - always return empty slice
	if resp.Results == nil {
		t.Fatal("Results must NEVER be nil, even when cass is not installed")
	}

	// Empty results, NOT an error dialog
	if len(resp.Results) != 0 {
		t.Errorf("len(Results) = %d, want 0 when cass not installed", len(resp.Results))
	}

	// Meta.Error is for internal logging only, never shown to users
	// It should be set so we can debug, but the UI layer ignores it
	if resp.Meta.Error == "" {
		t.Error("Meta.Error should be set for logging (but never shown to user)")
	}
}

func TestSafety_NoCassBinary_NoBlockingOrDelay(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "", errors.New("not found")
	}

	start := time.Now()
	_ = d.Check()
	elapsed := time.Since(start)

	// Detection without cass should be nearly instant (< 10ms)
	if elapsed > 10*time.Millisecond {
		t.Errorf("Detection took %v when cass not installed, want < 10ms", elapsed)
	}
}

func TestSafety_NoCassBinary_IsHealthyReturnsFalse(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "", errors.New("not found")
	}

	// Must return false BEFORE Check() called
	if d.IsHealthy() {
		t.Error("IsHealthy() should return false before Check() is called")
	}

	d.Check()

	// Must return false AFTER Check() when cass not installed
	if d.IsHealthy() {
		t.Error("IsHealthy() should return false when cass not installed")
	}
}

// =============================================================================
// SCENARIO 2: cass Health Check Fails
// Environment: cass binary exists but returns exit 1 or 3
// Expected: Same as no cass - silently disabled
// =============================================================================

func TestSafety_HealthCheckFails_ExitCodeOne(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 1, nil // Exit code 1 = needs indexing
	}

	status := d.Check()
	if status != StatusNeedsIndex {
		t.Errorf("Check() = %v, want StatusNeedsIndex for exit code 1", status)
	}

	// Search should return empty results, not error
	s := NewSearcher(d)
	resp := s.Search(context.Background(), SearchOptions{Query: "test"})

	if resp.Results == nil || len(resp.Results) != 0 {
		t.Error("Search should return empty (not nil) results when cass needs indexing")
	}
}

func TestSafety_HealthCheckFails_ExitCodeThree(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 3, nil // Exit code 3 = index corrupt
	}

	status := d.Check()
	if status != StatusNeedsIndex {
		t.Errorf("Check() = %v, want StatusNeedsIndex for exit code 3", status)
	}
}

func TestSafety_HealthCheckFails_CommandError(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return -1, errors.New("permission denied")
	}

	status := d.Check()
	if status != StatusNotInstalled {
		t.Errorf("Check() = %v, want StatusNotInstalled on command error", status)
	}
}

func TestSafety_HealthCheckFails_RecheckAfterTTL(t *testing.T) {
	checkCount := 0
	d := NewDetectorWithOptions(WithCacheTTL(50 * time.Millisecond))
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		checkCount++
		if checkCount == 1 {
			return 1, nil // First check: needs indexing
		}
		return 0, nil // Subsequent checks: healthy
	}

	// First check
	status := d.Check()
	if status != StatusNeedsIndex {
		t.Errorf("First Check() = %v, want StatusNeedsIndex", status)
	}

	// Second check - should use cache
	status = d.Check()
	if status != StatusNeedsIndex {
		t.Errorf("Second Check() = %v, want StatusNeedsIndex (cached)", status)
	}
	if checkCount != 1 {
		t.Errorf("checkCount = %d after second Check(), want 1 (cached)", checkCount)
	}

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	// Third check - should re-check and find healthy
	status = d.Check()
	if status != StatusHealthy {
		t.Errorf("Check() after TTL = %v, want StatusHealthy", status)
	}
	if checkCount != 2 {
		t.Errorf("checkCount = %d, want 2", checkCount)
	}
}

// =============================================================================
// SCENARIO 3: cass Search Times Out
// Environment: cass healthy but search hangs
// Expected: Graceful degradation, empty results, no UI blocking
// =============================================================================

func TestSafety_SearchTimeout_ReturnsEmptyResults(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcherWithOptions(d, WithSearchTimeout(50*time.Millisecond))
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// Simulate hanging search
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
			return []byte(`{"results":[{"title":"Too late"}]}`), nil
		}
	}

	start := time.Now()
	resp := s.Search(context.Background(), SearchOptions{Query: "test"})
	elapsed := time.Since(start)

	// Must return empty results, not error dialog
	if resp.Results == nil {
		t.Fatal("Results must never be nil on timeout")
	}
	if len(resp.Results) != 0 {
		t.Errorf("len(Results) = %d, want 0 on timeout", len(resp.Results))
	}

	// Must respect timeout - not hang
	if elapsed > 150*time.Millisecond {
		t.Errorf("Search took %v, should timeout around 50ms", elapsed)
	}
}

func TestSafety_SearchTimeout_UINotBlocked(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcherWithOptions(d, WithSearchTimeout(100*time.Millisecond))
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	// Simulate UI making search request with its own timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	resp := s.Search(ctx, SearchOptions{Query: "test"})
	elapsed := time.Since(start)

	// Context cancellation should be respected
	if elapsed > 80*time.Millisecond {
		t.Errorf("Search took %v, should respect context timeout of 30ms", elapsed)
	}

	if resp.Results == nil {
		t.Fatal("Results must never be nil")
	}
}

func TestSafety_SearchTimeout_CanNavigateWhilePending(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcherWithOptions(d, WithSearchTimeout(500*time.Millisecond))
	searchStarted := make(chan struct{})
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		close(searchStarted)
		<-ctx.Done()
		return nil, ctx.Err()
	}

	// Start search in background
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Search(ctx, SearchOptions{Query: "test"})
	}()

	// Wait for search to start
	<-searchStarted

	// Simulate "UI navigation" - cancel the search
	cancel()

	// Should complete quickly
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - search cancelled cleanly
	case <-time.After(100 * time.Millisecond):
		t.Error("Search cancellation took too long, would block UI navigation")
	}
}

// =============================================================================
// SCENARIO 4: cass Returns Malformed JSON
// Environment: cass outputs garbage/invalid JSON
// Expected: Treated as no results, no crash
// =============================================================================

func TestSafety_MalformedJSON_ReturnsEmptyResults(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	testCases := []struct {
		name   string
		output string
	}{
		{"completely invalid", "{not valid json at all"},
		{"truncated", `{"results":[{"title":"test`},
		{"binary garbage", "\x00\x01\x02\x03\x04"},
		{"empty object", "{}"},
		{"wrong type", `{"results": "not an array"}`},
		{"null results", `{"results": null}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSearcher(d)
			s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte(tc.output), nil
			}

			// Must not panic
			resp := s.Search(context.Background(), SearchOptions{Query: "test"})

			// Must return empty results, never nil
			if resp.Results == nil {
				t.Fatalf("Results must NEVER be nil, even with malformed JSON: %s", tc.name)
			}

			// Must be empty (not error dialog)
			if len(resp.Results) != 0 {
				t.Errorf("len(Results) = %d, want 0 for malformed JSON: %s", len(resp.Results), tc.name)
			}
		})
	}
}

func TestSafety_MalformedJSON_SubsequentSearchesWork(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	callCount := 0
	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return []byte(`{malformed}`), nil
		}
		return []byte(`{"results":[{"title":"good result"}]}`), nil
	}

	// First search with bad JSON
	resp1 := s.Search(context.Background(), SearchOptions{Query: "test"})
	if len(resp1.Results) != 0 {
		t.Error("First search should return empty results")
	}

	// Second search should still work
	resp2 := s.Search(context.Background(), SearchOptions{Query: "test"})
	if len(resp2.Results) != 1 {
		t.Errorf("Second search should return 1 result, got %d", len(resp2.Results))
	}
}

// =============================================================================
// SCENARIO 5: cass Returns Empty Results
// Environment: cass healthy, no matching sessions
// Expected: No indicator shown - same as cass-not-installed state
// =============================================================================

func TestSafety_EmptyResults_NoVisualDifference(t *testing.T) {
	// Create two detectors - one with cass installed (empty results), one without
	dNotInstalled := NewDetector()
	dNotInstalled.lookPath = func(name string) (string, error) {
		return "", errors.New("not found")
	}

	dInstalled := NewDetector()
	dInstalled.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	dInstalled.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	dInstalled.Check()

	sNotInstalled := NewSearcher(dNotInstalled)
	sInstalled := NewSearcher(dInstalled)
	sInstalled.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`{"results":[],"meta":{"total":0}}`), nil
	}

	respNotInstalled := sNotInstalled.Search(context.Background(), SearchOptions{Query: "test"})
	respInstalled := sInstalled.Search(context.Background(), SearchOptions{Query: "test"})

	// Both should have empty results (not nil)
	if respNotInstalled.Results == nil || respInstalled.Results == nil {
		t.Fatal("Results must never be nil")
	}

	// Both should have zero results
	if len(respNotInstalled.Results) != len(respInstalled.Results) {
		t.Errorf("Empty results should look identical: not-installed=%d, installed=%d",
			len(respNotInstalled.Results), len(respInstalled.Results))
	}

	// Neither should show as "error" to user
	// Meta.Error is internal only, but UI behavior must be identical
}

func TestSafety_EmptyResults_ZeroTotal(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`{"results":[],"meta":{"total":0}}`), nil
	}

	resp := s.Search(context.Background(), SearchOptions{Query: "nonexistent"})

	if resp.Results == nil {
		t.Fatal("Results must never be nil")
	}
	if len(resp.Results) != 0 {
		t.Errorf("len(Results) = %d, want 0", len(resp.Results))
	}
	if resp.Meta.Total != 0 {
		t.Errorf("Meta.Total = %d, want 0", resp.Meta.Total)
	}
}

// =============================================================================
// SCENARIO 6: cass Becomes Unhealthy Mid-Operation (Race Condition)
// Environment: cass healthy at detection, fails during search
// Expected: Graceful degradation, cache NOT invalidated
// =============================================================================

func TestSafety_RaceCondition_SearchFailsAfterHealthyDetection(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil // Healthy
	}
	d.Check()

	// Verify detector shows healthy
	if !d.IsHealthy() {
		t.Fatal("Detector should show healthy initially")
	}

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// Simulate cass crashing during search
		return nil, errors.New("cass: segmentation fault")
	}

	resp := s.Search(context.Background(), SearchOptions{Query: "test"})

	// Must return empty results, not propagate error
	if resp.Results == nil {
		t.Fatal("Results must never be nil")
	}
	if len(resp.Results) != 0 {
		t.Errorf("len(Results) = %d, want 0 after mid-operation failure", len(resp.Results))
	}

	// CRITICAL: Detection cache should NOT be invalidated by search failure
	// (to avoid cache thrashing)
	if !d.CacheValid() {
		t.Error("Detection cache should remain valid after search failure")
	}
}

func TestSafety_RaceCondition_SubsequentOperationsContinue(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	failCount := 0
	successCount := 0
	mu := sync.Mutex{}

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		mu.Lock()
		fc := failCount
		failCount++
		mu.Unlock()

		if fc%2 == 0 {
			// Every other call fails
			return nil, errors.New("random failure")
		}
		return []byte(`{"results":[{"title":"ok"}]}`), nil
	}

	// Run multiple searches
	for i := 0; i < 10; i++ {
		resp := s.Search(context.Background(), SearchOptions{Query: "test"})
		if resp.Results == nil {
			t.Fatal("Results must never be nil")
		}
		if len(resp.Results) > 0 {
			successCount++
		}
	}

	// Some should succeed, some should fail gracefully
	if successCount == 0 {
		t.Error("Expected some searches to succeed")
	}
	if successCount == 10 {
		t.Error("Expected some searches to fail (for test validity)")
	}
}

func TestSafety_RaceCondition_ConcurrentSearchesDuringInstability(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	var failureCount int32
	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// Randomly fail
		if time.Now().UnixNano()%2 == 0 {
			return nil, errors.New("random failure")
		}
		return []byte(`{"results":[]}`), nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := s.Search(context.Background(), SearchOptions{Query: "test"})
			if resp.Results == nil {
				atomic.AddInt32(&failureCount, 1)
			}
		}()
	}
	wg.Wait()

	if failureCount > 0 {
		t.Errorf("%d goroutines got nil Results, Results must NEVER be nil", failureCount)
	}
}

// =============================================================================
// SCENARIO 7: Startup Time Impact
// Environment: cass not installed
// Expected: Zero startup time impact (< 50ms difference)
// =============================================================================

func TestSafety_StartupTime_DetectionIsAsync(t *testing.T) {
	// Simulate slow detection
	d := NewDetectorWithOptions(WithHealthTimeout(500 * time.Millisecond))
	d.lookPath = func(name string) (string, error) {
		time.Sleep(100 * time.Millisecond) // Simulate slow PATH lookup
		return "", errors.New("not found")
	}

	// Status() should return immediately (StatusUnknown) without blocking
	start := time.Now()
	status := d.Status()
	elapsed := time.Since(start)

	if status != StatusUnknown {
		t.Errorf("Status() before Check() = %v, want StatusUnknown", status)
	}
	if elapsed > 5*time.Millisecond {
		t.Errorf("Status() took %v, should be instant (< 5ms)", elapsed)
	}
}

func TestSafety_StartupTime_IsHealthyFastPath(t *testing.T) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		time.Sleep(100 * time.Millisecond)
		return 0, nil
	}

	// IsHealthy() should return false immediately without blocking
	start := time.Now()
	healthy := d.IsHealthy()
	elapsed := time.Since(start)

	if healthy {
		t.Error("IsHealthy() before Check() should return false")
	}
	if elapsed > 5*time.Millisecond {
		t.Errorf("IsHealthy() took %v, should be instant (< 5ms)", elapsed)
	}
}

func TestSafety_StartupTime_NoCassVsWithCass(t *testing.T) {
	// Measure time for "no cass" scenario
	dNone := NewDetector()
	dNone.lookPath = func(name string) (string, error) {
		return "", errors.New("not found")
	}

	start := time.Now()
	dNone.Check()
	elapsedNone := time.Since(start)

	// Measure time for "cass healthy" scenario
	dHealthy := NewDetector()
	dHealthy.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	dHealthy.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}

	start = time.Now()
	dHealthy.Check()
	elapsedHealthy := time.Since(start)

	// Difference should be minimal (< 50ms as per spec)
	var diff time.Duration
	if elapsedHealthy > elapsedNone {
		diff = elapsedHealthy - elapsedNone
	} else {
		diff = elapsedNone - elapsedHealthy
	}

	if diff > 50*time.Millisecond {
		t.Errorf("Difference between no-cass and healthy-cass = %v, want < 50ms", diff)
	}

	t.Logf("Detection times: no-cass=%v, healthy-cass=%v, diff=%v", elapsedNone, elapsedHealthy, diff)
}

func TestSafety_StartupTime_SearcherCreationFast(t *testing.T) {
	d := NewDetector()

	start := time.Now()
	_ = NewSearcher(d)
	elapsed := time.Since(start)

	// Searcher creation should be instant
	if elapsed > 1*time.Millisecond {
		t.Errorf("NewSearcher() took %v, should be < 1ms", elapsed)
	}
}

// =============================================================================
// INTEGRATION: End-to-End Safety Guarantee
// =============================================================================

func TestSafety_EndToEnd_InvisibilityGuarantee(t *testing.T) {
	// This test verifies the core promise: bv behaves identically whether
	// cass is installed or not, UNLESS cass actually returns useful results.

	scenarios := []struct {
		name          string
		setupLookPath func() func(string) (string, error)
		setupHealth   func() func(context.Context, string, ...string) (int, error)
		setupSearch   func() func(context.Context, string, ...string) ([]byte, error)
	}{
		{
			name: "cass not in PATH",
			setupLookPath: func() func(string) (string, error) {
				return func(name string) (string, error) {
					return "", errors.New("not found")
				}
			},
		},
		{
			name: "cass exists but health check fails",
			setupLookPath: func() func(string) (string, error) {
				return func(name string) (string, error) {
					return "/usr/bin/cass", nil
				}
			},
			setupHealth: func() func(context.Context, string, ...string) (int, error) {
				return func(ctx context.Context, name string, args ...string) (int, error) {
					return 3, nil // Index corrupt
				}
			},
		},
		{
			name: "cass healthy but search times out",
			setupLookPath: func() func(string) (string, error) {
				return func(name string) (string, error) {
					return "/usr/bin/cass", nil
				}
			},
			setupHealth: func() func(context.Context, string, ...string) (int, error) {
				return func(ctx context.Context, name string, args ...string) (int, error) {
					return 0, nil
				}
			},
			setupSearch: func() func(context.Context, string, ...string) ([]byte, error) {
				return func(ctx context.Context, name string, args ...string) ([]byte, error) {
					<-ctx.Done()
					return nil, ctx.Err()
				}
			},
		},
		{
			name: "cass healthy but returns garbage",
			setupLookPath: func() func(string) (string, error) {
				return func(name string) (string, error) {
					return "/usr/bin/cass", nil
				}
			},
			setupHealth: func() func(context.Context, string, ...string) (int, error) {
				return func(ctx context.Context, name string, args ...string) (int, error) {
					return 0, nil
				}
			},
			setupSearch: func() func(context.Context, string, ...string) ([]byte, error) {
				return func(ctx context.Context, name string, args ...string) ([]byte, error) {
					return []byte("not json"), nil
				}
			},
		},
		{
			name: "cass healthy but returns empty results",
			setupLookPath: func() func(string) (string, error) {
				return func(name string) (string, error) {
					return "/usr/bin/cass", nil
				}
			},
			setupHealth: func() func(context.Context, string, ...string) (int, error) {
				return func(ctx context.Context, name string, args ...string) (int, error) {
					return 0, nil
				}
			},
			setupSearch: func() func(context.Context, string, ...string) ([]byte, error) {
				return func(ctx context.Context, name string, args ...string) ([]byte, error) {
					return []byte(`{"results":[]}`), nil
				}
			},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			d := NewDetectorWithOptions(WithHealthTimeout(50 * time.Millisecond))

			if sc.setupLookPath != nil {
				d.lookPath = sc.setupLookPath()
			}
			if sc.setupHealth != nil {
				d.runCommand = sc.setupHealth()
			}

			s := NewSearcherWithOptions(d, WithSearchTimeout(50*time.Millisecond))
			if sc.setupSearch != nil {
				s.runCommand = sc.setupSearch()
			}

			// THE INVISIBILITY GUARANTEE:
			// Search must ALWAYS return:
			// 1. Non-nil Results slice
			// 2. Complete within reasonable time (no hang)
			// 3. No panic or crash

			start := time.Now()
			resp := s.Search(context.Background(), SearchOptions{Query: "test"})
			elapsed := time.Since(start)

			// Must complete within timeout + buffer
			if elapsed > 200*time.Millisecond {
				t.Errorf("Search took %v, expected < 200ms", elapsed)
			}

			// Results must NEVER be nil
			if resp.Results == nil {
				t.Fatal("INVISIBILITY VIOLATED: Results is nil")
			}

			// Must be safe to iterate
			for range resp.Results {
				// Just verify we can iterate without panic
			}
		})
	}
}

// =============================================================================
// BENCHMARK: Verify performance meets requirements
// =============================================================================

func BenchmarkSafety_NoCassDetection(b *testing.B) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "", errors.New("not found")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Invalidate() // Force fresh check each time
		d.Check()
	}
}

func BenchmarkSafety_HealthyDetection(b *testing.B) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Invalidate()
		d.Check()
	}
}

func BenchmarkSafety_SearchWithEmptyResults(b *testing.B) {
	d := NewDetector()
	d.lookPath = func(name string) (string, error) {
		return "/usr/local/bin/cass", nil
	}
	d.runCommand = func(ctx context.Context, name string, args ...string) (int, error) {
		return 0, nil
	}
	d.Check()

	s := NewSearcher(d)
	s.runCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`{"results":[]}`), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Search(context.Background(), SearchOptions{Query: "test"})
	}
}
