# Performance Optimization Round 1: Results

## Summary

Implemented buffer pooling for Brandes' algorithm in `pkg/analysis/betweenness_approx.go`.
This optimization targets the dominant allocation hotspot identified through profiling.

## Key Metrics

### BenchmarkApproxBetweenness_500nodes_Sample100

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| allocs/op | 199,548 | 149,447 | **25%** |
| B/op | 29,574,627 | 11,895,089 | **60%** |
| ns/op | 13,420,876 | 5,929,813 | **56%** |

### BenchmarkApproxBetweenness_500nodes_Sample50

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| allocs/op | 100,757 | 74,896 | **26%** |
| B/op | 14,842,128 | 5,677,960 | **62%** |
| ns/op | 5,512,771 | 3,176,322 | **42%** |

### BenchmarkApproxBetweenness_500nodes_Exact

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| allocs/op | 499,557 | 499,557 | 0% (expected) |
| B/op | 34,110,015 | 34,110,051 | 0% (expected) |
| ns/op | 68,866,948 | 69,911,721 | 0% (expected) |

Note: The "Exact" benchmark falls through to `network.Betweenness(g)` (gonum's implementation)
when `sampleSize >= nodeCount`, so buffer pooling has no effect on it.

## Profile Comparison

### CPU Profile

| Function | Before | After | Change |
|----------|--------|-------|--------|
| runtime.gcDrain | 36.40% | 21.02% | -15.4pp |
| singleSourceBetweenness (flat) | 1.47% | 2.93% | +1.5pp |
| singleSourceBetweenness (cum) | 50.05% | 64.67% | +14.6pp |
| runtime.mapassign_fast64 | 26.45% | 19.69% | -6.8pp |
| (*brandesBuffers).reset | N/A | 14.10% | new |

**Analysis**: GC overhead dropped significantly (36% → 21%), confirming reduced allocation
pressure. The reset method adds ~14% overhead but this is amortized across all BFS traversals.

### Memory Profile

| Allocator | Before (flat) | After (flat) | Change |
|-----------|---------------|--------------|--------|
| singleSourceBetweenness | 74.80% | 18.93% | **-55.9pp** |
| gonum iterator.newMapIterEdges | 10.32% | 25.61% | +15.3pp |
| gonum iterator.(*mapIter).next | 7.67% | 19.76% | +12.1pp |
| (*brandesBuffers).reset | N/A | 12.17% | new |

**Analysis**: The optimization successfully eliminated singleSourceBetweenness as the dominant
allocator. The remaining allocations are now dominated by gonum iterator overhead (45% combined),
which is addressed in Round 2 via cached adjacency lists.

## Implementation Details

### Changes Made

1. **`brandesBuffers` struct** (lines 26-34): Holds reusable maps and slices for BFS
2. **`brandesPool` sync.Pool** (lines 46-58): Thread-safe buffer reuse with 256-entry pre-allocation
3. **`reset()` method** (lines 74-105): Clears buffers while retaining capacity
4. **`singleSourceBetweenness`** (lines 264-333): Modified to use pooled buffers

### Memory Strategy

- Maps cleared via `clear()` builtin (Go 1.21+) when grown >2x expected size
- Slices reset via `[:0]` to retain backing array
- Predecessor slices retain capacity for reuse

### Concurrency Model

Each goroutine in `ApproxBetweenness` gets its own buffer from the pool. Buffers are returned
after results are written to `localBC`, ensuring thread safety.

## Verification

### Tests Pass

- All unit tests: `go test ./pkg/analysis/...`
- Race detector: `go test -race ./pkg/analysis/...`
- E2E integration: `./scripts/test_buffer_pooling_e2e.sh`

### Determinism Verified

Results are identical with same seed across multiple runs. The `TestApproxBetweenness_Determinism`
test confirms this with 1e-10 epsilon tolerance.

### Equivalence Proven

`TestResetEquivalentToFreshAllocation` verifies that pooled buffer output matches fresh allocation
output for various graph topologies (sparse, dense, chain).

## Deviation from Predictions

The original plan predicted 80% allocation reduction. We achieved 25% reduction. Key differences:

1. **localBC map not pooled**: Each goroutine still creates a fresh `localBC` map
2. **nodes slice**: `graph.NodesOf()` allocates on each call
3. **gonum iterator overhead**: ~50% of allocations come from gonum iterators

These are candidates for Round 2 optimization.

## Next Steps (Round 2)

Based on post-pooling profiles, priority targets:

1. **Cache adjacency lists** - Eliminate gonum iterator allocations (45% of memory)
2. **Pool localBC maps** - Per-goroutine result maps
3. **Pool nodes slice** - Reuse node list across samples

Expected Round 2 improvement: Additional 50-60% allocation reduction.

## Commit Reference

- Baseline capture: `chore: capture pre-optimization baseline benchmarks`
- Implementation: `perf: implement buffer pooling for Brandes' algorithm`
- Unit tests: `test: add comprehensive buffer pool unit and race tests`
- E2E tests: `test: add E2E integration test script for buffer pooling`

## Files Modified

- `pkg/analysis/betweenness_approx.go` - Buffer pooling implementation
- `pkg/analysis/buffer_pool_test.go` - Unit and race condition tests (new)
- `scripts/test_buffer_pooling_e2e.sh` - E2E integration tests (new)
- `benchmarks/baseline_round1_*.txt` - Baseline metrics
- `benchmarks/profile_comparison.txt` - Post-optimization profiles
