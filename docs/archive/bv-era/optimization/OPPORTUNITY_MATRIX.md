# Performance Opportunity Matrix

**Task:** bd-29b3 - Profile + opportunity matrix (identify top hotspots)
**Date:** 2026-01-21
**Agent:** CopperHeron (Opus 4.5)

---

## Executive Summary

This matrix identifies **12 high-impact optimization opportunities** based on CPU/memory profiling of the analysis pipeline. The robot mode `--robot-triage` command is the primary target, as it's the critical path for AI agent integration.

**Current Performance (1000 issues):**
- Total time: ~295-910ms
- Target: <100ms for responsive AI agent integration
- Gap: 3-9x improvement needed

**Top 3 Quick Wins (Total: ~150ms savings):**
1. JSON pretty-printing opt-out: ~25ms
2. Selective Phase 2 waiting: ~100ms
3. Memoized blocker lookups: ~25ms

---

## Profiling Data Summary

### CPU Profile Hotspots (cpu_triage.prof)

| Rank | Function | Flat % | Cum % | Category |
|------|----------|--------|-------|----------|
| 1 | `runtime.memclrNoHeapPointers` | 27.92% | 27.92% | GC pressure |
| 2 | `runtime.scanobject` | 10.33% | 14.02% | GC scanning |
| 3 | `runtime.memmove` | 5.29% | 5.29% | Memory ops |
| 4 | `runtime.mapassign_fast64` | 0.62% | 5.90% | Map writes |
| 5 | `encoding/json.literalStore` | 0.49% | 5.29% | JSON parsing |
| 6 | `ComputeImpactScoresFromStats` | 0.49% | 3.81% | Scoring |
| 7 | `graphStructureHash` | 0.37% | 2.34% | Hashing |

**Key Insight:** 38% of CPU time is GC-related (`memclr` + `scanobject`), indicating excessive allocations.

### Memory Profile Hotspots (mem_triage.prof)

| Rank | Function | Flat MB | % Total | Category |
|------|----------|---------|---------|----------|
| 1 | `bufio.NewReaderSize` | 50.00 | 20.49% | I/O buffering |
| 2 | `NewAnalyzer` | 27.19 | 11.14% | Graph init |
| 3 | `TriageContext.UnblocksMap` | 13.98 | 5.73% | Dependency calc |
| 4 | `DirectedGraph.SetEdge` | 13.52 | 5.54% | gonum edges |
| 5 | `loadProjectBeads` | 11.98 | 4.91% | Test fixture |
| 6 | `os.readFileContents` | 11.97 | 4.91% | File I/O |
| 7 | `json.literalStore` | 9.04 | 3.71% | JSON parsing |
| 8 | `computeUnblocksMapInternal` | 7.27 | 2.98% | Dependency calc |
| 9 | `DirectedGraph.Edges` | 6.65 | 2.73% | gonum iterator |
| 10 | `brandesBuffers.reset` | 4.08 | 1.67% | Pooled buffers |

**Total profiled:** 244MB for analysis tests

---

## Opportunity Matrix

### Priority 1: Critical Path (Robot Mode)

| ID | Opportunity | Location | Est. Savings | Effort | Score |
|----|-------------|----------|--------------|--------|-------|
| **O1** | JSON pretty-print opt-out | `cmd/bv/main.go:2524` | 10-25ms | Low | **4.5** |
| **O2** | Selective Phase 2 waiting | `cmd/bv/main.go:2418` | 50-200ms | Medium | **4.0** |
| **O3** | Memoize GetOpenBlockers/GetBlockerDepth | `pkg/analysis/triage.go:608,1073,1352` | 15-30ms | Low | **4.0** |
| **O4** | High-perf JSON library (sonic/go-json) | `pkg/loader/loader.go:324-350` | 65-120ms | Medium | **3.5** |

### Priority 2: Allocation Reduction

| ID | Opportunity | Location | Est. Savings | Effort | Score |
|----|-------------|----------|--------------|--------|-------|
| **O5** | Git cache copy-on-write | `pkg/loader/git.go:240-277` | 5-15ms + 32KB | Medium | **3.0** |
| **O6** | Diff comparison pointer maps | `pkg/analysis/diff.go:149-157` | ~400KB | Low | **3.5** |
| **O7** | Cache adjacency lists | `pkg/analysis/*.go` | 15-25ms | High | **2.5** |

### Priority 3: TUI Performance

| ID | Opportunity | Location | Est. Savings | Effort | Score |
|----|-------------|----------|--------------|--------|-------|
| **O8** | Pre-compute lipgloss styles | `pkg/ui/delegate.go:74-296` | 48K allocs/s | Medium | **3.0** |
| **O9** | Recipe filtering with trie/bloom | `pkg/ui/snapshot.go:393-400` | O(n) -> O(1) | High | **2.0** |

### Priority 4: Future Optimizations

| ID | Opportunity | Location | Est. Savings | Effort | Score |
|----|-------------|----------|--------------|--------|-------|
| **O10** | Streaming JSONL parser | `pkg/loader/loader.go` | Memory peak | High | **2.0** |
| **O11** | Pool localBC maps (betweenness) | `pkg/analysis/betweenness_approx.go` | 20% allocs | Medium | **2.5** |
| **O12** | Pool nodes slice | `pkg/analysis/graph.go` | 5% allocs | Low | **3.0** |

**Score formula:** `Impact (1-5) * Confidence (0.6-1.0) / Effort (1-3)`

---

## Detailed Opportunity Analysis

### O1: JSON Pretty-Print Opt-Out (Score: 4.5)

**Current State:**
```go
encoder := json.NewEncoder(os.Stdout)
encoder.SetIndent("", "  ")  // Always pretty-prints
```

**Problem:** Pretty-printing adds 20-30% overhead via whitespace calculation and buffer management.

**Solution:**
```go
encoder := json.NewEncoder(os.Stdout)
if os.Getenv("BV_PRETTY_JSON") == "1" {
    encoder.SetIndent("", "  ")
}
// Default: compact JSON for agents
```

**Verification:** JSON content identical; only whitespace differs (RFC 7159 equivalent).

---

### O2: Selective Phase 2 Waiting (Score: 4.0)

**Current State:**
```go
opts := analysis.TriageOptions{
    WaitForPhase2: true,  // Blocks 100-500ms
}
```

**Problem:** Triage only uses PageRank and betweenness. Eigenvector, HITS, k-core, articulation, slack are computed but never read.

**Solution:**
```go
type TriageOptions struct {
    RequiredMetrics []MetricType  // Only wait for these
}

opts := TriageOptions{
    RequiredMetrics: []MetricType{MetricPageRank, MetricBetweenness},
}
```

**Impact:** Skip 4 unused metrics = 50-200ms savings.

---

### O3: Memoize Blocker Lookups (Score: 4.0)

**Current State:** `GetOpenBlockers()` called 3x per issue; `GetBlockerDepth()` called 2x with DFS each time.

**Problem:** For 10 recommendations with avg depth 3:
- 30 graph traversals (should be 10)
- 20 DFS traversals (should be 0 - memoized)

**Solution:** Extend `TriageContext` with `blockerDepthCache map[string]int`.

---

### O4: High-Performance JSON (Score: 3.5)

**Current:** `encoding/json.Unmarshal()` per line = 80-150ms for 1000 issues.

**Alternatives:**
| Library | Time | Speedup |
|---------|------|---------|
| stdlib | 80-150ms | 1x |
| goccy/go-json | 20-40ms | 4x |
| bytedance/sonic | 15-30ms | 5x |

**Note:** sonic requires CGO; go-json is pure Go.

---

## Implementation Roadmap

### Phase 1: Quick Wins (1-2 days)
- [ ] O1: JSON pretty-print opt-out
- [ ] O3: Memoize blocker lookups
- [ ] O6: Diff pointer maps

### Phase 2: Medium Effort (3-5 days)
- [ ] O2: Selective Phase 2 waiting
- [ ] O4: High-perf JSON library
- [ ] O5: Git cache COW

### Phase 3: High Effort (1-2 weeks)
- [ ] O7: Cached adjacency lists
- [ ] O8: Pre-computed styles
- [ ] O9: Recipe filtering optimization

---

## Benchmark Commands

```bash
# Current baseline
go test -bench=BenchmarkFullAnalysis ./pkg/analysis/... -benchmem

# Profile triage
go test -bench=BenchmarkTriage -cpuprofile=cpu.prof -memprofile=mem.prof ./pkg/analysis/...

# Robot mode timing
time ./bv --robot-triage > /dev/null

# Compare before/after
benchstat before.txt after.txt
```

---

## Success Criteria

| Metric | Current | Target | Method |
|--------|---------|--------|--------|
| `--robot-triage` latency | 295-910ms | <100ms | `time bv --robot-triage` |
| Peak memory (1000 issues) | ~244MB | <150MB | `go test -memprofile` |
| Allocations (triage) | ~660 allocs/op | <300 allocs/op | `go test -benchmem` |
| GC pause contribution | 38% CPU | <15% CPU | pprof |

---

## References

- [PERFORMANCE_ANALYSIS_ROUND_2.md](./PERFORMANCE_ANALYSIS_ROUND_2.md) - Detailed inefficiency audit
- [PERF_OPTIMIZATION_ROUND_1_RESULTS.md](./PERF_OPTIMIZATION_ROUND_1_RESULTS.md) - Brandes pooling results
- `benchmarks/current.txt` - Current benchmark results
- `cpu_triage.prof`, `mem_triage.prof` - Latest profile data
