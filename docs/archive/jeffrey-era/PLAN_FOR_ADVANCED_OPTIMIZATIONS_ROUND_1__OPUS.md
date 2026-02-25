# Performance Optimization Plan: Round 1

## Executive Summary

This document captures a rigorous performance analysis of beads_viewer following a data-driven methodology. The analysis identified a single dominant hotspot responsible for 71% of memory allocations and 49% of CPU time, with a clear optimization path that is provably isomorphic (outputs unchanged).

**Key Finding**: Buffer pooling for `singleSourceBetweenness` can eliminate **60-80%** of allocations from the dominant hotspot, with potential for 90%+ when combined with additional caching.

**Epsilon Relaxation**: Floating-point outputs are considered "equal" within epsilon tolerances (user-approved).

---

## 0) Hard Constraints

### Repo Invariants (Absolute)
- **Do not delete any file or directory** unless explicitly requested with exact command.
- Avoid broad refactors; prefer **one performance lever per change**.

### Methodology Invariants (A→G)
- **A) Baseline First**: Run benchmarks with `-benchmem -count=3`; record p50/p95/p99 latency, throughput, peak memory
- **B) Profile Before Proposing**: Capture CPU + allocation profiles; identify top 3-5 hotspots
- **C) Equivalence Oracle**: Define golden outputs + invariants; use property tests for large input spaces
- **D) Isomorphism Proof**: Every proposed change includes proof that outputs cannot change
- **E) Opportunity Matrix**: Rank by (Impact × Confidence) / Effort
- **F) Minimal Diffs**: One performance lever per change; include rollback guidance
- **G) Regression Guardrails**: Add benchmark thresholds and CI hooks

---

## 1) Architecture Snapshot

### 1A) Data Plane (robot + TUI share the same engine)

1. **Load issues** from `.beads/issues.jsonl` (preferred) or `.beads/beads.jsonl` fallback:
   - `pkg/loader/loader.go` (`GetBeadsDir`, `FindJSONLPath`, `LoadIssues`)

2. **Build graph** + compute metrics via Analyzer:
   - `pkg/analysis/graph.go` (`NewAnalyzer`, `AnalyzeAsync` / `AnalyzeWithProfile`)

3. **Derive higher-level outputs**:
   - Unified triage: `pkg/analysis/triage.go` (`ComputeTriageFromAnalyzer`)
   - Priority tuning: `pkg/analysis/priority.go`, `pkg/analysis/feedback.go`

4. **Presentation**:
   - Robot JSON: `cmd/bv/main.go` (`--robot-*`)
   - TUI snapshot: `pkg/ui/background_worker.go` → `pkg/ui/snapshot.go`

### 1B) Two-Phase Analysis Contract

- **Phase 1 (sync, "instant")**: Degree centrality, topological sort, density — enough for initial render
- **Phase 2 (async)**: PageRank, Betweenness, Eigenvector, HITS, Critical Path, Cycles, k-core, slack, articulation
- **Size-aware config**: `pkg/analysis/config.go` (`ConfigForSize`)
- **Status tracking**: Robot outputs include `status` so consumers know computed vs approx vs timeout vs skipped

---

## 2) Baseline Metrics

### 2A) Environment

```
go version go1.25.5 linux/amd64
cpu: AMD Ryzen Threadripper PRO 5975WX 32-Cores
nproc: 64
RAM: 499Gi
```

### 2B) Representative Workloads

| Dataset | Issues | Size | Description |
|---------|--------|------|-------------|
| `.beads/issues.jsonl` | 570 | 1.5MB | Real project data |
| `benchmark/medium.jsonl` | 1,000 | 3.4MB | Synthetic medium |
| `benchmark/large.jsonl` | 5,000 | 32MB | Synthetic large |

### 2C) Latency Distribution (p50/p95/p99)

Measurement harness:
```python
import math, os, subprocess, time

cmd = ['/tmp/bv_round1', '--robot-triage']
env = os.environ.copy()
env.update({'BV_ROBOT': '1', 'BV_NO_BROWSER': '1', 'BV_TEST_MODE': '1'})

warmup, runs = 5, 50
for _ in range(warmup):
    subprocess.run(cmd, cwd='/data/projects/beads_viewer', env=env,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)

samples = []
for _ in range(runs):
    t0 = time.perf_counter()
    subprocess.run(cmd, cwd='/data/projects/beads_viewer', env=env,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
    samples.append(time.perf_counter() - t0)

samples.sort()
def pct(p):
    k = (len(samples) - 1) * p / 100.0
    f, c = math.floor(k), math.ceil(k)
    return samples[int(k)] if f == c else samples[f] * (1 - (k - f)) + samples[c] * (k - f)

print(f'p50={pct(50)*1000:.1f}ms p95={pct(95)*1000:.1f}ms p99={pct(99)*1000:.1f}ms')
print(f'throughput={runs / sum(samples):.2f} runs/s')
```

| Dataset | p50 | p95 | p99 | Throughput |
|---------|----:|----:|----:|----------:|
| 570 issues | 52.6ms | 56.7ms | 59.2ms | 18.88 runs/s |
| 1,000 issues | 153.6ms | 158.7ms | 163.7ms | 6.53 runs/s |
| 5,000 issues | 1.457s | 1.518s | 1.540s | 0.68 runs/s |

### 2D) Peak Memory (RSS)

```bash
/usr/bin/time -v env BV_ROBOT=1 BV_NO_BROWSER=1 BV_TEST_MODE=1 /tmp/bv_round1 --robot-triage >/dev/null
```

| Dataset | Peak RSS |
|---------|----------|
| 570 issues | 38 MB |
| 1,000 issues | 42 MB |
| 5,000 issues | **429 MB** |

**Interpretation**: Typical repos are fast (p50 ~53ms). Scaling to 5k issues shows major memory spike and >1s latency — significant headroom for optimization.

### 2E) Startup Breakdown (`--profile-startup`)

| Phase | 570 issues | 5,000 issues |
|-------|------------|--------------|
| load_jsonl | 12.0ms | 347.5ms |
| betweenness | 9.1ms (63% of Phase 2) | 178.7ms |
| k-core | — | 262.7ms |
| slack | — | 96.4ms |
| eigenvector | — | 78.0ms |
| PageRank | — | 48.3ms |
| **total_with_load** | 27.1ms | 1.123s |

### 2F) Benchmark Results

| Benchmark | ns/op | allocs/op | B/op |
|-----------|------:|----------:|-----:|
| **ApproxBetweenness_500nodes_Exact** | 70,263,842 | **499,557** | 34,110,194 |
| ApproxBetweenness_500nodes_Sample100 | 13,586,671 | 199,548 | 29,574,627 |
| ApproxBetweenness_500nodes_Sample50 | 5,568,698 | 100,756 | 14,841,955 |
| RobotTriage_Sparse500 | 21,590,000 | 89,500 | 27,700,000 |
| FullAnalysis_Sparse500 | 14,232,742 | 82,316 | 26,428,791 |
| Cycles_ManyCycles30 | 500,355,711 | 3,335,574 | 1,824,125,368 |

---

## 3) Profiling Results

### 3A) CPU Profile (138.02s total sample time)

| Hotspot | Time (s) | % | Category |
|---------|----------|---|----------|
| runtime.gcDrain | 44.04 | 31.9% | GC overhead |
| runtime.mapassign_fast64 | 39.38 | 28.5% | Map operations |
| **singleSourceBetweenness** | 68.01 | **49.3%** | Core algorithm |
| runtime.scanobject | 18.24 | 13.2% | GC scanning |
| internal/runtime/maps.table.grow | 26.31 | 19.1% | Map growth |

**Root Cause**: ~45% GC (gcDrain + scanobject) + ~48% map operations. These overlap since map allocations trigger GC.

### 3B) Memory Profile (41.34GB total allocations)

| Allocator | Memory (MB) | % |
|-----------|-------------|---|
| **singleSourceBetweenness** (direct) | 29,463 | **71.3%** |
| gonum iterators (inside singleSourceBetweenness) | ~8,000 | ~19% |
| Other | ~4,000 | ~10% |

**Note**: The 71.3% represents DIRECT allocations (4 maps). The gonum iterator allocations (~19%) occur during `g.From(v)` calls INSIDE singleSourceBetweenness. Total during execution: **~90%**.

**Root Cause**: `singleSourceBetweenness` creates 4 fresh maps per call:

```go
sigma := make(map[int64]float64)  // N entries
dist := make(map[int64]int)       // N entries
delta := make(map[int64]float64)  // N entries
pred := make(map[int64][]int64)   // N entries + dynamic slices
```

For a 500-node graph with 100 samples: 400 maps created (4 per sample), totaling ~200K entries.

### 3C) I/O Profile Sanity Check

```bash
strace -c -f env BV_ROBOT=1 BV_NO_BROWSER=1 BV_TEST_MODE=1 /tmp/bv_round1 --robot-triage >/dev/null
```

**Observed**: Syscalls dominated by `futex`/`nanosleep`; only ~30 `read` calls. **Not I/O-bound**.

---

## 4) Equivalence Oracle

### 4A) Input Definition

For "same inputs", we require:
- Same parsed issue set (same JSONL contents after parsing/validation)
- Same analysis configuration (`ConfigForSize` given node/edge count)
- Same seed for approximate algorithms (approx betweenness uses a seed)
- Same "now" when time-based fields are included (triage staleness)

### 4B) Output Definition

**(1) Structural outputs must be exact:**
- IDs present and counts
- Membership of sets (actionable, blockers, quick wins)
- Cycle presence and members (when computed)
- JSON schema and field names
- Deterministic tie-breaking (by ID)

**(2) Float outputs are equal within epsilon:**
- Centralities and composite scores may differ slightly (parallel reduction, iteration order)
- Differences within epsilon are accepted

### 4C) Epsilon Policy

| Metric | Absolute Tolerance | Notes |
|--------|-------------------|-------|
| PageRank | 1e-5 | Iterative convergence variance |
| Betweenness (exact) | 1e-6 | Exact algorithm |
| Betweenness (approx) | 1e-6 abs + 1e-12 rel | Compare top-k rank stability |
| Eigenvector | 1e-6 | |
| HITS (Hubs/Authorities) | 1e-6 | |
| CriticalPathScore | 1e-6 | |

### 4D) Ordering Under Epsilon

If two scores differ by less than epsilon, ordering swaps are treated as equivalent **as long as**:
- The set of items is identical
- Swapped items were within epsilon of each other

This prevents flaky tests when floats drift in low bits.

### 4E) Existing Guardrails

| Test File | Coverage |
|-----------|----------|
| `pkg/analysis/golden_test.go` | Numeric tolerances for all metrics |
| `pkg/analysis/invariance_test.go` | Metamorphic/invariance properties |
| `pkg/analysis/betweenness_approx_test.go` | Epsilon stability for approx betweenness |
| `pkg/loader/fuzz_test.go` | Loader fuzzing |
| `tests/e2e/*` | Robot contract + perf checks |

### 4F) Invariants

1. **Betweenness bounds**: `0 ≤ BC(v) ≤ (n-1)(n-2)` for directed graphs
2. **PageRank sum**: `Σ PR(v) = 1.0`
3. **Deterministic ordering**: Sorted by value descending, then by ID ascending
4. **Map completeness**: All node IDs present in output maps

---

## 5) Opportunity Matrix

Scoring: **(Impact × Confidence) / Effort**

| # | Candidate | Impact | Confidence | Effort | Score | Notes |
|---|-----------|--------|------------|--------|-------|-------|
| **1** | **Buffer pooling for Brandes** | **0.70** | **0.95** | **0.40** | **1.66** | Dominant alloc_space (~71%) |
| 2 | Array-based indexing (no maps) | 0.50 | 0.90 | 0.50 | 0.90 | Cache-friendly |
| 3 | Remove undirected graph for k-core | 0.25 | 0.85 | 0.35 | 0.61 | On 5k nodes, k-core ~263ms |
| 4 | Cached adjacency lists | 0.30 | 0.70 | 0.60 | 0.35 | Avoid gonum iterator overhead |
| 5 | Slack computation reuse | 0.15 | 0.90 | 0.30 | 0.45 | Reuse topo order |
| 6 | Cross-process cache (robot mode) | 0.15 | 0.60 | 0.70 | 0.13 | Keyed by data_hash + config_hash |

**Round 1 Focus**: #1 (buffer pooling). Re-profile after, then evaluate #2-5.

---

## 6) Proposed Changes

### Change 1 (Primary): Buffer Pooling for Brandes' Algorithm

**Location**: `pkg/analysis/betweenness_approx.go:167-241`

**Current Implementation**:
```go
func singleSourceBetweenness(g *simple.DirectedGraph, source graph.Node, bc map[int64]float64) {
    sourceID := source.ID()
    nodes := graph.NodesOf(g.Nodes())  // Still allocates (not pooled)

    // ALLOCATES 4 FRESH MAPS PER CALL - THIS IS THE PROBLEM
    sigma := make(map[int64]float64)
    dist := make(map[int64]int)
    delta := make(map[int64]float64)
    pred := make(map[int64][]int64)

    // ... rest of algorithm
}
```

**Proposed Implementation**:
```go
// brandesBuffers holds reusable data structures for Brandes' algorithm.
type brandesBuffers struct {
    sigma     map[int64]float64
    dist      map[int64]int
    delta     map[int64]float64
    pred      map[int64][]int64
    queue     []int64
    stack     []int64
    neighbors []int64
}

var brandesPool = sync.Pool{
    New: func() interface{} {
        return &brandesBuffers{
            sigma:     make(map[int64]float64, 256),
            dist:      make(map[int64]int, 256),
            delta:     make(map[int64]float64, 256),
            pred:      make(map[int64][]int64, 256),
            queue:     make([]int64, 0, 256),
            stack:     make([]int64, 0, 256),
            neighbors: make([]int64, 0, 32),
        }
    },
}

// reset clears all values but retains map/slice capacity.
func (b *brandesBuffers) reset(nodes []graph.Node) {
    // Clear maps if they've grown too large (prevents unbounded memory growth)
    if len(b.sigma) > len(nodes)*2 {
        clear(b.sigma)
        clear(b.dist)
        clear(b.delta)
        clear(b.pred)
    }

    for _, n := range nodes {
        nid := n.ID()
        b.sigma[nid] = 0
        b.dist[nid] = -1
        b.delta[nid] = 0
        // Reuse slice backing array, clear length
        if existing, ok := b.pred[nid]; ok {
            b.pred[nid] = existing[:0]
        } else {
            b.pred[nid] = make([]int64, 0, 4)  // First-call allocation unavoidable
        }
    }
    b.queue = b.queue[:0]
    b.stack = b.stack[:0]
    b.neighbors = b.neighbors[:0]
}

func singleSourceBetweenness(g *simple.DirectedGraph, source graph.Node, bc map[int64]float64) {
    sourceID := source.ID()
    nodes := graph.NodesOf(g.Nodes())  // Still allocates

    // Get buffer from pool
    buf := brandesPool.Get().(*brandesBuffers)
    defer brandesPool.Put(buf)

    buf.reset(nodes)

    // Use buf.sigma, buf.dist, buf.delta, buf.pred instead of local maps

    // BFS phase - use pooled neighbors slice:
    // buf.neighbors = buf.neighbors[:0]
    // for to.Next() {
    //     buf.neighbors = append(buf.neighbors, to.Node().ID())
    // }

    // ... rest of algorithm unchanged
}
```

### What This Does NOT Address

```go
// Line 169 - STILL ALLOCATES (not pooled)
nodes := graph.NodesOf(g.Nodes())

// Line 116 in ApproxBetweenness - STILL ALLOCATES per goroutine
localBC := make(map[int64]float64)

// gonum iterator overhead from g.From(v) calls
```

To achieve 90%+ reduction, also pool `nodes` slice and `localBC` maps (requires Analyzer-level caching).

---

## 7) Isomorphism Proof

**Theorem**: Buffer reuse produces identical outputs to fresh allocation.

**Proof**:

1. **Initialization Equivalence**:
   - Current: `sigma[nid] = 0`, `dist[nid] = -1`, `delta[nid] = 0`
   - Proposed: Same assignments in `reset()`
   - Since we iterate over ALL nodes and explicitly set values, prior state is irrelevant.

2. **Graph Change Safety**:
   - If node IDs differ between calls, stale entries don't affect output
   - BFS only visits reachable nodes from source
   - All visited nodes are explicitly initialized before use

3. **Predecessor Slice Safety**:
   - Current: `pred[nid] = make([]int64, 0)`
   - Proposed: `pred[nid] = pred[nid][:0]` (reuses backing array)
   - Equivalence: Empty slice has `len=0`. Both produce identical behavior.

4. **Floating-Point Determinism**:
   - All arithmetic operations `(σ_v/σ_w) × (1+δ_w)` are identical
   - IEEE-754 floating-point is deterministic for same inputs
   - Ordering guaranteed by sorted node IDs in BFS/accumulation

5. **Concurrency Safety**:
   - Each goroutine in `ApproxBetweenness` gets its own buffer from pool
   - Buffers are returned AFTER results are written to `localBC` (before merge to `partialBC`)
   - Results live in `localBC`, not in the buffer, so merge is safe after buffer return
   - `sync.Pool` guarantees no concurrent access to same buffer

6. **Pool Eviction Safety**:
   - If buffer is evicted during GC and recreated, behavior is identical to fresh allocation

**QED** ∎

---

## 8) Follow-Up Changes (Post-#1 Profiling)

### Change 2: Remove Undirected Graph for k-core/Articulation

**Target**: `pkg/analysis/graph.go` (`computeCoreAndArticulation`, `computeKCore`, `findArticulationPoints`)

**Rationale**: On 5k nodes, k-core takes ~263ms and allocates significantly by constructing `simple.UndirectedGraph`.

**Minimal diff**:
- Build undirected adjacency view directly from directed edges
- Run k-core and articulation on that adjacency without allocating new graph

**Isomorphism**: Current implementation treats directed edges as undirected by inserting into undirected graph; direct adjacency construction is equivalent.

### Change 3: Linear-Time k-core Algorithm

**Target**: `computeKCore` loops `k=1..maxDeg` and rescans nodes; worst-case O(maxDeg·V).

**Proposed**: Batagelj–Zaveršnik linear-time k-core decomposition using bin-sort peeling.

**Isomorphism**: Computes same core number definition; only algorithmic route changes.

---

## 9) What We Considered

Explicit mapping of optimization techniques to this codebase:

### Clearly Applicable (Supported by Profiles)

| Technique | Application |
|-----------|-------------|
| Index-based lookup | Dense-index arrays for graph algorithms |
| Zero-copy / buffer reuse | Per-worker scratch buffers for Brandes |
| Bounded queues + backpressure | Worker pool rather than per-pivot goroutines |
| Topological sort reuse | Reuse Phase 1 topo order in Phase 2 helpers (slack) |
| Memory layout (SoA vs AoS) | Array-based sigma/dist/delta improves cache locality |
| Short-circuiting | Avoid work when metrics skipped by config/timeouts |

### Possibly Useful Later

| Technique | Notes |
|-----------|-------|
| Serialization format | JSON encoding may become significant at 20k+ issues |
| Cross-process caching | Helps repeated `bv --robot-*` on large graphs |

### Not Currently Justified

| Technique | Reason |
|-----------|--------|
| DP / convex optimization | No hotspots resemble scheduling/allocation problems |
| Tries / segment trees | Not the problem shape here |
| Lock-free data structures | Contention is not the limiter; allocations/GC are |

---

## 10) Expected Gains

### Before Optimization

| Metric | Value |
|--------|-------|
| Allocations/op (Sample100) | 199,548 |
| Bytes/op (Sample100) | 29.5 MB |
| GC CPU overhead | ~45% |

### After Optimization - Conservative (Buffer Pooling Only)

| Metric | Value | Reduction |
|--------|-------|-----------|
| Allocations/op (Sample100) | ~40,000 | **80%** |
| Bytes/op (Sample100) | ~8 MB | **73%** |
| GC CPU overhead | ~15% | **67%** |
| Throughput | 1.5-2x | — |

### After Optimization - With Additional Caching

If combined with node slice caching and localBC pooling:

| Metric | Value | Reduction |
|--------|-------|-----------|
| Allocations/op (Sample100) | ~5,000 | **97%** |
| Bytes/op (Sample100) | ~3 MB | **90%** |
| GC CPU overhead | ~5% | **89%** |
| Throughput | 3-5x | — |

---

## 11) Minimal Diff Summary

The change is isolated to a single file:

```
pkg/analysis/betweenness_approx.go
  - Add brandesBuffers struct (~25 lines)
  - Add brandesPool with sync.Pool (~10 lines)
  - Add reset() method (~20 lines)
  - Modify singleSourceBetweenness to use pool (~10 line changes)
```

**Total**: ~65 lines added/modified

**No API changes**: Function signature unchanged, output unchanged.

### Rollback Guidance

If issues arise:
1. Remove `brandesPool` and `brandesBuffers`
2. Restore original map allocations in `singleSourceBetweenness`
3. No external code changes required

---

## 12) Regression Guardrails

### 12A) Benchmarks to Watch

- `pkg/analysis/BenchmarkRobotTriage_Sparse500`
- `pkg/analysis/BenchmarkApproxBetweenness_500nodes_Sample100`
- `pkg/analysis/BenchmarkFullAnalysis_Sparse1000`

### 12B) Allocation Threshold Benchmark

```go
func BenchmarkBrandesAllocationThreshold(b *testing.B) {
    issues := generateSparseGraph(500)
    g := buildGraph(issues)
    nodes := graph.NodesOf(g.Nodes())

    b.ReportAllocs()
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        bc := make(map[int64]float64)
        singleSourceBetweenness(g, nodes[0], bc)
    }
    // After optimization: expect < 100 allocs/op (down from ~1000)
}
```

### 12C) Property Test

```go
func TestBetweennessOutputEquivalence(t *testing.T) {
    issues := generateSparseGraph(100)

    baseline := computeBetweennessBaseline(issues)
    optimized := computeBetweennessOptimized(issues)

    for id, baseVal := range baseline {
        optVal := optimized[id]
        if math.Abs(baseVal-optVal) > 1e-10 {
            t.Errorf("Mismatch for %s: baseline=%v, optimized=%v", id, baseVal, optVal)
        }
    }
}
```

### 12D) CI Integration

```yaml
- name: Check allocation regression
  run: |
    go test -bench=BenchmarkApproxBetweenness_500nodes_Sample100 \
      -benchmem ./pkg/analysis/... | tee bench.txt

    # Field positions: 1=name 2=iterations 3=ns/op 4="ns/op" 5=B/op 6="B/op" 7=allocs/op
    ALLOCS=$(grep 'Sample100' bench.txt | awk '{print $7}')
    if [ "$ALLOCS" -gt 50000 ]; then
      echo "Allocation regression: $ALLOCS > 50000"
      exit 1
    fi
```

---

## 13) Success Criteria

For Change #1 (buffer pooling), "success" is defined as:

| Criterion | Target |
|-----------|--------|
| alloc_space from betweenness | From ~71% to "not dominant" (<30%) |
| GC CPU time | runtime.gcDrain no longer dominates |
| Peak RSS (5k issues) | Drastic reduction from ~429MB |
| p95 latency (5k issues) | Measurable improvement |
| All tests | Green under `BV_NO_BROWSER=1 BV_TEST_MODE=1` |

---

## Appendix: Raw Profiling Commands

```bash
# Run benchmarks with memory stats
go test -bench=. -benchmem -count=3 ./pkg/analysis/... 2>&1 | tee bench_baseline.txt

# Generate CPU profile
go test -run=NONE -bench="BenchmarkApproxBetweenness" \
  -cpuprofile=cpu.prof -benchtime=3s ./pkg/analysis/...

# Generate memory profile
go test -run=NONE -bench="BenchmarkApproxBetweenness" \
  -memprofile=mem.prof -benchtime=3s ./pkg/analysis/...

# Analyze CPU profile
go tool pprof -top cpu.prof | head -40

# Analyze memory profile
go tool pprof -top mem.prof | head -40

# I/O profile
strace -c -f env BV_ROBOT=1 BV_NO_BROWSER=1 BV_TEST_MODE=1 /tmp/bv_round1 --robot-triage >/dev/null

# Interactive profile exploration
go tool pprof -http=:8080 cpu.prof

# Latency distribution (use Python harness from §2C)
```

---

*Generated: 2026-01-09*
*Author: Claude Code (performance analysis session)*
*Hybrid: Best elements from OPUS and GPT plans*
