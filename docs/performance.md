# Performance Tuning Guide

This guide explains `bv`'s performance characteristics, how to diagnose slow startup, and available tuning options.

## Graph Analysis Performance

`bv` computes 9 graph-theoretic metrics on startup. Their computational complexity varies significantly:

| Metric | Complexity | 100 nodes | 500 nodes | 1000 nodes | 2000 nodes |
|--------|-----------|-----------|-----------|------------|------------|
| Degree | O(V) | <1ms | <1ms | <5ms | <10ms |
| TopologicalSort | O(V+E) | <1ms | <5ms | <5ms | <10ms |
| Critical Path | O(V+E) | <1ms | <5ms | <5ms | <10ms |
| PageRank | O(iter×E) | <5ms | ~20ms | ~40ms | ~100ms |
| Eigenvector | O(iter×E) | <5ms | ~15ms | ~30ms | ~70ms |
| HITS | O(iter×E) | <5ms | ~5ms | ~10ms | ~30ms |
| **Betweenness** | **O(V×E)** | ~10ms | ~300ms | **~1.3s** | **~4.6s** |
| **Cycles** | O((V+E)×C) | varies | varies | varies | varies |

**Key Insight:** Betweenness centrality and cycle detection are the primary performance bottlenecks for large graphs.

## Two-Phase Startup Architecture

`bv` uses a two-phase startup to ensure responsive UI:

### Phase 1: Instant (<50ms)
Computes metrics needed for initial render:
- Degree centrality (blocking indicators)
- Topological sort (execution order)
- Basic stats (counts, density)

**Result:** You see the issue list immediately.

### Phase 2: Background (async)
Computes expensive metrics in a background goroutine:
- PageRank
- Betweenness (with timeout)
- Eigenvector
- HITS
- Cycle detection
- Critical path scoring

**Result:** Insights dashboard shows "Computing..." until Phase 2 completes.

## Factors Affecting Performance

### 1. Graph Size (Node Count)
- Linear algorithms (degree, topo sort) scale with V+E
- Betweenness scales quadratically: O(V×E)
- For 2000+ nodes, betweenness can take 5+ seconds

### 2. Graph Density
```
density = edges / (nodes × (nodes - 1))
```

| Density | Classification | Impact |
|---------|---------------|--------|
| <0.01 | Sparse | Fast - most real projects |
| 0.01-0.05 | Normal | Standard performance |
| 0.05-0.15 | Dense | Betweenness may timeout |
| >0.15 | Very Dense | Consider simplifying deps |

### 3. Cycle Structure
Cycle detection uses Johnson's algorithm which enumerates ALL elementary cycles:
- **Acyclic graphs:** Fast (SCC pre-check returns immediately)
- **Few cycles:** Fast
- **Many overlapping cycles:** Can be exponential
- **Complete graph:** O((n-1)!) cycles - pathological case

A complete graph with 20 nodes has 19! ≈ 10^17 cycles. This is why cycle detection has strict timeouts.

## Size-Based Algorithm Selection

`bv` automatically adjusts algorithm selection based on graph size:

### Small Graphs (<100 nodes)
- All metrics computed with **exact algorithms**
- Generous timeouts (2 seconds)
- Full cycle enumeration (up to 1000)

### Medium Graphs (100-500 nodes)
- All metrics computed with **exact algorithms**
- Standard timeouts (500ms)
- Cycle limit: 100

### Large Graphs (500-2000 nodes)
- **Approximate betweenness** for sparse graphs (density < 0.01)
- Betweenness skipped for dense graphs
- Shorter timeouts (200-300ms)
- Cycle limit: 50

### XL Graphs (>2000 nodes)
- **Approximate betweenness** (sampling-based)
- Cycle detection skipped
- HITS skipped if density > 0.001
- Minimal timeouts

## Sampling-Based Betweenness Approximation

For large graphs (500+ nodes), `bv` uses a sampling-based approximation of betweenness centrality instead of the exact O(V×E) algorithm:

### How It Works
Instead of computing shortest paths from ALL nodes, we sample k pivot nodes and extrapolate:
1. Randomly select k pivot nodes
2. Compute betweenness contribution from each pivot
3. Scale up by (n/k) to estimate full betweenness

### Error Bounds
| Sample Size | Approximate Error |
|-------------|-------------------|
| k=50 | ~14% |
| k=100 | ~10% |
| k=200 | ~7% |

### Default Sample Sizes
| Graph Size | Sample Size |
|------------|-------------|
| <100 nodes | Exact (use full algorithm) |
| 100-500 nodes | min(50, 20% of nodes) |
| 500-2000 nodes | 100 |
| >2000 nodes | 200 |

### Performance Improvement
For a 1000-node graph:
- Exact: ~1.3 seconds
- Approximate (k=100): ~130ms (**10x faster**)

The approximation is sufficient for ranking purposes (identifying which nodes are most central) while dramatically improving startup time.

## CLI Flags for Performance

### Diagnostic Flags

```bash
# Show detailed startup timing breakdown
bv --profile-startup

# Machine-readable timing (JSON)
bv --profile-startup --profile-json
```

**Sample `--profile-startup` output:**
```
Startup Profile for /path/to/.beads/beads.jsonl
================================================
Data: 847 issues, 2341 dependencies, density=0.003

Phase 1 (blocking):
  Build graph:     12ms
  Degree:           3ms
  TopoSort:         5ms
  Total Phase 1:   20ms

Phase 2 (async):
  PageRank:        45ms
  Betweenness:    312ms (timeout: NO)
  Eigenvector:     28ms
  HITS:            19ms
  Cycles:          67ms (found: 3)
  Critical Path:   11ms
  Total Phase 2:  482ms

Total startup:    502ms

Recommendations:
  ✓ Startup within acceptable range (<1s)
  ⚠ Betweenness taking 60% of Phase 2 time
    Consider: --force-full-analysis only when needed
```

### Performance Control Flags

```bash
# Force compute ALL metrics regardless of graph size
# (May be slow for large graphs - use sparingly)
bv --force-full-analysis
```

## Troubleshooting Slow Startup

### Step 1: Profile Startup
```bash
bv --profile-startup
```

Identify which phase/metric is slow.

### Step 2: Check Graph Size
```bash
bv --robot-insights | jq '.stats | {nodeCount, edgeCount, density}'
```

For large graphs (>500 nodes), some metrics are automatically skipped.

### Step 3: Check for Cycles
```bash
bv --robot-insights | jq '.cycles'
```

Many cycles can cause slowdowns even with timeouts due to memory pressure.

### Step 4: Try Without Problem Metrics

If betweenness is the bottleneck:
- Check if your graph is >500 nodes - it should auto-skip
- If not, consider if you need betweenness metrics

If cycles are the bottleneck:
- Review your dependencies for circular patterns
- Use `bd` to break cycles: `bd unblock A --from B`

### Step 5: Report Issues

If startup is slow and profiling shows unexpected behavior:
```bash
bv --profile-startup --profile-json > profile.json
```

Include `profile.json` in your bug report.

## Performance Targets

| Graph Size | Target Startup |
|------------|----------------|
| <100 nodes | <100ms |
| 100-500 nodes | <300ms |
| 500-1000 nodes | <500ms |
| 1000-2000 nodes | <1s |
| >2000 nodes | <2s |

These targets assume Phase 1 (blocking) startup. Phase 2 completes asynchronously.

## Best Practices

### For Project Maintainers

1. **Keep dependency graphs sparse**
   - Only create blocking dependencies where truly needed
   - Use `related` type for informational links

2. **Avoid circular dependencies**
   - Cycles indicate design issues
   - Break cycles before they accumulate

3. **Monitor graph density**
   - Healthy: <0.05
   - Warning: >0.15

### For AI Agents

1. **Use robot flags for programmatic access**
   - `--robot-insights` for metrics
   - `--robot-plan` for actionable items

2. **Check for timeouts in robot output**
   - Timeout flags indicate metrics may be incomplete
   - Design agents to handle partial data gracefully

3. **For large repositories**
   - Use `--robot-plan` for immediate actionable items
   - Avoid forcing full analysis unless needed

## Timeout Configuration

All expensive algorithms have configurable timeouts:

| Algorithm | Default Timeout | Rationale |
|-----------|----------------|-----------|
| Betweenness | 500ms | O(V×E) can be seconds |
| PageRank | 500ms | Usually fast, defensive |
| HITS | 500ms | Usually fast, defensive |
| Cycle Detection | 500ms | Can be exponential |

When a timeout triggers:
- The metric is skipped or returns partial results
- A warning appears in the profile output
- The UI marks the metric as unavailable

## Advanced: Memory Considerations

For very large graphs:

1. **Cycle enumeration** is memory-hungry
   - Each cycle stores full path
   - Limited to `MaxCyclesToStore` (default: 100)

2. **Graph structure** uses gonum's sparse representation
   - Efficient for sparse graphs
   - ~100 bytes per node + ~50 bytes per edge

3. **Typical memory usage**
   - 1000 issues, 3000 deps: ~5MB
   - 5000 issues, 15000 deps: ~25MB
   - 10000 issues, 30000 deps: ~50MB

## Benchmarking

Run the benchmark suite to measure performance on your hardware:

```bash
# Run all benchmarks
./scripts/benchmark.sh

# Save baseline
./scripts/benchmark.sh baseline

# Compare after changes
./scripts/benchmark.sh compare

# Quick benchmarks (CI mode)
./scripts/benchmark.sh quick
```

See `benchmarks/` directory for detailed results.
