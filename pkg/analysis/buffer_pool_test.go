package analysis

import (
	"runtime"
	"sync"
	"testing"
)

// createTestBuffer creates a brandesBuffers with test data
func createTestBuffer() *brandesBuffers {
	return &brandesBuffers{
		sigma:     make([]float64, 0, 256),
		dist:      make([]int, 0, 256),
		delta:     make([]float64, 0, 256),
		pred:      make([][]int, 0, 256),
		queue:     make([]int, 0, 256),
		stack:     make([]int, 0, 256),
		neighbors: make([]int, 0, 32),
		bc:        make([]float64, 0, 256),
	}
}

// =============================================================================
// brandesBuffers Struct Tests
// =============================================================================

// TestBrandesBuffersInitialization verifies struct creation
func TestBrandesBuffersInitialization(t *testing.T) {
	t.Log("Testing brandesBuffers struct initialization...")

	buf := createTestBuffer()

	t.Logf("Created buffer with queue capacity: %d", cap(buf.queue))

	if buf.sigma == nil {
		t.Fatal("sigma slice should be initialized")
	}
	if buf.dist == nil {
		t.Fatal("dist slice should be initialized")
	}
	if buf.delta == nil {
		t.Fatal("delta slice should be initialized")
	}
	if buf.pred == nil {
		t.Fatal("pred slice should be initialized")
	}
	if buf.bc == nil {
		t.Fatal("bc slice should be initialized")
	}
	if cap(buf.queue) != 256 {
		t.Errorf("queue capacity: got %d, want 256", cap(buf.queue))
	}
	if cap(buf.stack) != 256 {
		t.Errorf("stack capacity: got %d, want 256", cap(buf.stack))
	}
	if cap(buf.neighbors) != 32 {
		t.Errorf("neighbors capacity: got %d, want 32", cap(buf.neighbors))
	}

	t.Log("PASS: All fields initialized correctly")
}

// =============================================================================
// reset() Method Tests
// =============================================================================

// TestResetClearsAllValues verifies reset produces clean state
func TestResetClearsAllValues(t *testing.T) {
	t.Log("Testing reset() clears all values...")

	// Create buffer with stale data
	buf := createTestBuffer()
	buf.reset(2)
	buf.sigma[1] = 999.0
	buf.dist[1] = 999
	buf.delta[1] = 999.0
	buf.bc[1] = 999.0
	buf.pred[1] = append(buf.pred[1], 1, 2, 3)
	buf.queue = append(buf.queue, 1, 2, 3)
	buf.stack = append(buf.stack, 1, 0)

	t.Logf("Before reset: sigma[1]=%v, dist[1]=%v, queue len=%d",
		buf.sigma[1], buf.dist[1], len(buf.queue))

	// Reset
	buf.reset(2)

	t.Logf("After reset: sigma[1]=%v, dist[1]=%v, queue len=%d",
		buf.sigma[1], buf.dist[1], len(buf.queue))

	// Verify reset state matches fresh allocation
	if buf.sigma[1] != 0.0 {
		t.Errorf("sigma[1] should be 0 after reset, got %v", buf.sigma[1])
	}
	if buf.dist[1] != -1 {
		t.Errorf("dist[1] should be -1 after reset, got %v", buf.dist[1])
	}
	if buf.delta[1] != 0.0 {
		t.Errorf("delta[1] should be 0 after reset, got %v", buf.delta[1])
	}
	if buf.bc[1] != 0.0 {
		t.Errorf("bc[1] should be 0 after reset, got %v", buf.bc[1])
	}
	if len(buf.pred[1]) != 0 {
		t.Errorf("pred[1] should be empty after reset, got %v", buf.pred[1])
	}
	if len(buf.queue) != 0 {
		t.Errorf("queue should be empty after reset, got len %d", len(buf.queue))
	}
	if len(buf.stack) != 0 {
		t.Errorf("stack should be empty after reset, got len %d", len(buf.stack))
	}

	t.Log("PASS: reset() produces correct initial state")
}

// TestResetRetainsPredCapacity verifies pred slices retain capacity
func TestResetRetainsPredCapacity(t *testing.T) {
	t.Log("Testing reset() retains predecessor slice capacity...")

	buf := createTestBuffer()

	// First reset - allocates small slice
	buf.reset(1)
	t.Logf("After first reset: pred[0] cap=%d", cap(buf.pred[0]))

	// Add predecessors to grow slice
	buf.pred[0] = append(buf.pred[0], 10, 20, 30, 40, 50)
	oldCap := cap(buf.pred[0])
	t.Logf("After appends: pred[0] cap=%d", oldCap)

	// Reset again - should retain capacity
	buf.reset(1)
	newCap := cap(buf.pred[0])
	t.Logf("After second reset: pred[0] cap=%d", newCap)

	if newCap < oldCap {
		t.Errorf("pred capacity should be retained: got %d, want >= %d", newCap, oldCap)
	}
	if len(buf.pred[0]) != 0 {
		t.Errorf("pred length should be 0 after reset, got %d", len(buf.pred[0]))
	}

	t.Log("PASS: reset() retains predecessor slice capacity")
}

// TestResetTriggersResizeOnOversizedBuffers verifies the 2x threshold reallocation.
func TestResetTriggersResizeOnOversizedBuffers(t *testing.T) {
	t.Log("Testing reset() resizes oversized buffers...")

	buf := createTestBuffer()

	// Grow buffers very large
	buf.reset(5000)
	buf.sigma[4999] = 1
	oldCap := cap(buf.sigma)
	t.Logf("Grew buffers to len=%d cap=%d", len(buf.sigma), oldCap)

	// Reset with tiny node set (should trigger resize due to 2x threshold)
	buf.reset(2)

	t.Logf("After reset with 2 nodes: sigma len=%d cap=%d", len(buf.sigma), cap(buf.sigma))

	// Should have been resized and only 2 entries remain
	if len(buf.sigma) != 2 {
		t.Errorf("oversized buffer should be resized: got %d entries, want 2", len(buf.sigma))
	}
	if len(buf.dist) != 2 {
		t.Errorf("dist buffer should be resized: got %d entries, want 2", len(buf.dist))
	}
	if cap(buf.sigma) >= oldCap {
		t.Errorf("expected sigma capacity to shrink: got %d, want < %d", cap(buf.sigma), oldCap)
	}
	if buf.dist[0] != -1 || buf.dist[1] != -1 {
		t.Errorf("dist entries should be -1 after reset, got %v", buf.dist)
	}

	t.Log("PASS: resize triggered for oversized buffers")
}

// TestResetHandlesEmptyNodes verifies reset with empty node slice
func TestResetHandlesEmptyNodes(t *testing.T) {
	t.Log("Testing reset() with empty node slice...")

	buf := createTestBuffer()
	buf.reset(2)
	buf.sigma[1] = 999.0

	buf.reset(0)

	if len(buf.queue) != 0 {
		t.Errorf("queue should be empty, got len %d", len(buf.queue))
	}
	if len(buf.stack) != 0 {
		t.Errorf("stack should be empty, got len %d", len(buf.stack))
	}

	t.Log("PASS: reset() handles empty node slice")
}

// =============================================================================
// Pool Behavior Tests
// =============================================================================

// TestPoolReturnsNonNilBuffer verifies pool.Get() works
func TestPoolReturnsNonNilBuffer(t *testing.T) {
	t.Log("Testing brandesPool.Get() returns valid buffer...")

	for i := 0; i < 10; i++ {
		buf := brandesPool.Get().(*brandesBuffers)
		if buf == nil {
			t.Fatal("pool should never return nil")
		}
		t.Logf("Got buffer %d: sigma=%p", i, buf.sigma)
		brandesPool.Put(buf)
	}

	t.Log("PASS: Pool consistently returns valid buffers")
}

// TestPoolPreallocation verifies pool's New() function allocates correctly
func TestPoolPreallocation(t *testing.T) {
	t.Log("Testing pool preallocation capacities...")

	// Note: We can't guarantee exact capacities because:
	// 1. Pool may return previously-used buffers with grown slices
	// 2. Pool may have been cleared by GC
	// What we CAN verify: buffers are always functional and non-nil

	buf := brandesPool.Get().(*brandesBuffers)
	if buf == nil {
		t.Fatal("Pool returned nil buffer")
	}

	// Verify all slices are initialized
	if buf.sigma == nil || buf.dist == nil || buf.delta == nil || buf.pred == nil || buf.bc == nil {
		t.Error("One or more slices are nil")
	}

	// Verify slices are at least usable (not nil)
	if buf.queue == nil {
		t.Error("queue slice is nil")
	}
	if buf.stack == nil {
		t.Error("stack slice is nil")
	}
	if buf.neighbors == nil {
		t.Error("neighbors slice is nil")
	}

	brandesPool.Put(buf)
	t.Log("PASS: Pool returns valid, usable buffers")
}

// TestPoolEvictionRecovery verifies behavior after GC
func TestPoolEvictionRecovery(t *testing.T) {
	t.Log("Testing pool recovery after GC eviction...")

	// Get and return a buffer
	buf1 := brandesPool.Get().(*brandesBuffers)
	buf1.reset(100)
	buf1.sigma[42] = 3.14
	brandesPool.Put(buf1)

	t.Log("Forcing GC to potentially evict pool entries...")
	runtime.GC()
	runtime.GC()

	// Get buffer again - might be new or recycled
	buf2 := brandesPool.Get().(*brandesBuffers)
	if buf2 == nil {
		t.Fatal("pool must return buffer even after GC")
	}

	// Key point: behavior is correct regardless of whether buf1 == buf2
	t.Logf("Got buffer after GC: sigma=%p (may or may not be same)", buf2.sigma)

	brandesPool.Put(buf2)
	t.Log("PASS: Pool handles GC eviction gracefully")
}

// =============================================================================
// Equivalence to Fresh Allocation Tests
// =============================================================================

// TestResetEquivalentToFreshAllocation is the KEY isomorphism test
func TestResetEquivalentToFreshAllocation(t *testing.T) {
	t.Log("Testing that reset() produces state equivalent to fresh allocation...")

	nodeCount := 3

	// Fresh allocation (baseline)
	fresh := &brandesBuffers{}
	fresh.reset(nodeCount)

	// Pooled + reset (optimized)
	pooled := brandesPool.Get().(*brandesBuffers)
	pooled.reset(10)
	pooled.sigma[9] = 999.0 // Add stale data
	pooled.dist[9] = 999
	pooled.delta[9] = 999.0
	pooled.bc[9] = 999.0
	pooled.reset(nodeCount)

	// Compare
	for i := 0; i < nodeCount; i++ {
		t.Logf("Index %d: fresh sigma=%v, pooled sigma=%v", i, fresh.sigma[i], pooled.sigma[i])

		if fresh.sigma[i] != pooled.sigma[i] {
			t.Errorf("sigma mismatch for index %d: fresh=%v, pooled=%v", i, fresh.sigma[i], pooled.sigma[i])
		}
		if fresh.dist[i] != pooled.dist[i] {
			t.Errorf("dist mismatch for index %d: fresh=%v, pooled=%v", i, fresh.dist[i], pooled.dist[i])
		}
		if fresh.delta[i] != pooled.delta[i] {
			t.Errorf("delta mismatch for index %d: fresh=%v, pooled=%v", i, fresh.delta[i], pooled.delta[i])
		}
		if fresh.bc[i] != pooled.bc[i] {
			t.Errorf("bc mismatch for index %d: fresh=%v, pooled=%v", i, fresh.bc[i], pooled.bc[i])
		}
		if len(fresh.pred[i]) != len(pooled.pred[i]) {
			t.Errorf("pred len mismatch for index %d: fresh=%d, pooled=%d", i, len(fresh.pred[i]), len(pooled.pred[i]))
		}
	}

	brandesPool.Put(pooled)
	t.Log("PASS: reset() produces state equivalent to fresh allocation")
}

// TestStaleEntriesNotAccessible verifies stale entries don't affect correctness
func TestStaleEntriesNotAccessible(t *testing.T) {
	t.Log("Testing that stale entries from previous usage don't affect results...")

	buf := createTestBuffer()

	// Simulate first usage with many nodes
	buf.reset(100)
	buf.sigma[50] = 500
	buf.dist[50] = 50
	t.Logf("Set values for 100 nodes")

	// Now reset with smaller set
	buf.reset(2)

	if len(buf.sigma) != 2 {
		t.Errorf("expected sigma len 2 after reset, got %d", len(buf.sigma))
	}
	if buf.sigma[1] != 0.0 {
		t.Errorf("sigma[1] should be 0, got %v", buf.sigma[1])
	}
	if buf.dist[1] != -1 {
		t.Errorf("dist[1] should be -1, got %v", buf.dist[1])
	}

	t.Log("PASS: Stale entries beyond current length are not accessible")
}

// =============================================================================
// Slice Behavior Tests
// =============================================================================

// TestSliceCapacityRetention verifies queue/stack retain capacity
func TestSliceCapacityRetention(t *testing.T) {
	t.Log("Testing slice capacity retention across resets...")

	buf := createTestBuffer()
	buf.reset(1)

	// Grow queue and stack
	for i := 0; i < 500; i++ {
		buf.queue = append(buf.queue, i)
		buf.stack = append(buf.stack, i)
	}
	queueCap := cap(buf.queue)
	stackCap := cap(buf.stack)
	t.Logf("Grew slices: queue cap=%d, stack cap=%d", queueCap, stackCap)

	// Reset
	buf.reset(1)

	// Capacity should be retained
	if cap(buf.queue) < queueCap {
		t.Errorf("queue capacity decreased: got %d, want >= %d", cap(buf.queue), queueCap)
	}
	if cap(buf.stack) < stackCap {
		t.Errorf("stack capacity decreased: got %d, want >= %d", cap(buf.stack), stackCap)
	}
	if len(buf.queue) != 0 {
		t.Errorf("queue length should be 0, got %d", len(buf.queue))
	}
	if len(buf.stack) != 0 {
		t.Errorf("stack length should be 0, got %d", len(buf.stack))
	}

	t.Log("PASS: Slice capacity retained, length reset")
}

// =============================================================================
// Concurrent Access / Race Condition Tests
// =============================================================================

// TestBufferPoolConcurrentAccess verifies no races under heavy concurrent load.
// Run with: go test -race -run TestBufferPoolConcurrentAccess -count=10
func TestBufferPoolConcurrentAccess(t *testing.T) {
	t.Log("Testing buffer pool concurrent access...")

	const numGoroutines = 50
	const iterationsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				// Get buffer, use it, return it
				buf := brandesPool.Get().(*brandesBuffers)
				if buf == nil {
					t.Error("Got nil buffer in concurrent access")
					return
				}

				// Simulate work
				buf.reset(1)
				buf.sigma[0] = float64(j)
				buf.queue = append(buf.queue, j)

				brandesPool.Put(buf)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("PASS: Completed %d concurrent operations without race",
		numGoroutines*iterationsPerGoroutine)
}

// TestBufferPoolLifecycle verifies correct Get/Put semantics
func TestBufferPoolLifecycle(t *testing.T) {
	t.Log("Testing buffer pool lifecycle...")

	// Get a buffer
	buf1 := brandesPool.Get().(*brandesBuffers)
	if buf1 == nil {
		t.Fatal("First Get returned nil")
	}

	// Modify it
	buf1.reset(100)
	buf1.sigma[42] = 1.5
	buf1.queue = append(buf1.queue, 100)
	t.Logf("Modified buffer: sigma[42]=%v, queue=%v", buf1.sigma[42], buf1.queue)

	// Return it
	brandesPool.Put(buf1)

	// Get again - might be same buffer or new one
	buf2 := brandesPool.Get().(*brandesBuffers)
	if buf2 == nil {
		t.Fatal("Second Get returned nil")
	}

	// Key invariant: no panic, no race
	t.Logf("Got second buffer: sigma=%p", buf2.sigma)
	brandesPool.Put(buf2)

	t.Log("PASS: Pool lifecycle works correctly")
}

// TestConcurrentPoolGetPut tests rapid Get/Put cycles
func TestConcurrentPoolGetPut(t *testing.T) {
	t.Log("Testing rapid concurrent Get/Put cycles...")

	const cycles = 1000
	const workers = 10

	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < cycles; i++ {
				buf := brandesPool.Get().(*brandesBuffers)
				if buf == nil {
					t.Error("Got nil buffer")
					return
				}
				// Immediately return
				brandesPool.Put(buf)
			}
		}()
	}

	wg.Wait()
	t.Logf("PASS: Completed %d rapid Get/Put cycles without race", cycles*workers)
}
