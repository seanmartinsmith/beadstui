# Single-Value Accessor Pattern for GraphStats

**Task Reference:** bv-4jfr
**Status:** Implemented

## Overview

This document describes the single-value accessor pattern for `GraphStats` metrics. The pattern eliminates O(n) map copies when only a single value is needed, providing O(1) lookups instead.

## Problem

The original `GraphStats` accessors returned full map copies for thread safety:

```go
// Old pattern - copies entire map O(n)
func (s *GraphStats) PageRank() map[string]float64 {
    s.mu.RLock()
    defer s.mu.RUnlock()
    cp := make(map[string]float64, len(s.pageRank))
    for k, v := range s.pageRank {
        cp[k] = v
    }
    return cp
}

// Usage (wasteful for single lookup)
score := stats.PageRank()["issue-123"]  // Copies 1000 entries to get 1 value
```

**Problems:**
1. O(n) allocation and copy for every call
2. No way to distinguish "not found" from "value is zero"
3. Hot paths calling multiple metrics multiply the overhead

## Solution

The new pattern provides three accessor styles:

### 1. Single-Value Accessor: `*Value(id) (T, bool)`

O(1) lookup with existence flag:

```go
// New pattern - O(1) lookup
func (s *GraphStats) PageRankValue(id string) (float64, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if s.pageRank == nil {
        return 0, false
    }
    v, ok := s.pageRank[id]
    return v, ok
}

// Usage
score, ok := stats.PageRankValue("issue-123")
if !ok {
    // Issue not found or Phase 2 not ready
}
```

### 2. Iterator Accessor: `*All(fn func(id, value) bool)`

Iterates without copying, caller controls termination:

```go
// Iterator - no copy, early termination possible
func (s *GraphStats) PageRankAll(fn func(id string, score float64) bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if s.pageRank == nil {
        return
    }
    for id, score := range s.pageRank {
        if !fn(id, score) {
            return
        }
    }
}

// Usage - find top 3
count := 0
stats.PageRankAll(func(id string, score float64) bool {
    if score > threshold {
        process(id, score)
        count++
    }
    return count < 3  // Stop after 3
})
```

### 3. Map Copy Accessor (Legacy): `*() map[string]T`

Retained for backward compatibility, marked deprecated:

```go
// Deprecated: Use PageRankValue() for single lookups
// or PageRankAll() for iteration. This method copies O(n) data.
func (s *GraphStats) PageRank() map[string]float64
```

## Available Accessors

### Score Metrics (float64)

| Metric | Value Accessor | Iterator |
|--------|---------------|----------|
| PageRank | `PageRankValue(id)` | `PageRankAll(fn)` |
| Betweenness | `BetweennessValue(id)` | `BetweennessAll(fn)` |
| Eigenvector | `EigenvectorValue(id)` | `EigenvectorAll(fn)` |
| Hub | `HubValue(id)` | `HubsAll(fn)` |
| Authority | `AuthorityValue(id)` | `AuthoritiesAll(fn)` |
| CriticalPath | `CriticalPathValue(id)` | `CriticalPathAll(fn)` |
| Slack | `SlackValue(id)` | `SlackAll(fn)` |

### Integer Metrics

| Metric | Value Accessor | Iterator |
|--------|---------------|----------|
| CoreNumber | `CoreNumberValue(id)` | `CoreNumberAll(fn)` |

### Boolean Metrics

| Metric | Value Accessor |
|--------|---------------|
| Articulation | `IsArticulationPoint(id)` |

### Rank Metrics (int, 1-based)

| Metric | Value Accessor |
|--------|---------------|
| PageRankRank | `PageRankRankValue(id)` |
| BetweennessRank | `BetweennessRankValue(id)` |
| EigenvectorRank | `EigenvectorRankValue(id)` |
| HubsRank | `HubsRankValue(id)` |
| AuthoritiesRank | `AuthoritiesRankValue(id)` |
| CriticalPathRank | `CriticalPathRankValue(id)` |
| InDegreeRank | `InDegreeRankValue(id)` |
| OutDegreeRank | `OutDegreeRankValue(id)` |

## Thread Safety

All accessors are thread-safe:
- `*Value()` methods use `RLock` for the single lookup
- `*All()` methods hold `RLock` during iteration (callback must not block)
- Legacy map copy methods use `RLock` during copy

**Important:** The `*All()` callback executes while holding the read lock. Keep callbacks fast and non-blocking.

## Performance

Benchmarks show significant improvement for single-value lookups:

```
BenchmarkPageRank_MapCopy/n=1000      50000 ns/op    40960 B/op    1 allocs/op
BenchmarkPageRank_SingleValue/n=1000    120 ns/op        0 B/op    0 allocs/op
```

For n=1000 issues:
- **Map copy:** ~50μs, 40KB allocation
- **Single value:** ~120ns, zero allocation
- **Speedup:** ~400x faster, zero allocations

Multiple metric access (5 metrics, 1000 issues):
- **Map copy:** ~250μs (5 × 50μs)
- **Single value:** ~600ns (5 × 120ns)

## Migration Guide

### Before (Map Copy)

```go
// Getting single value
pagerank := stats.PageRank()
if score, ok := pagerank[issueID]; ok {
    // use score
}

// Checking existence
if _, ok := stats.Betweenness()[issueID]; ok {
    // exists
}

// Iterating
for id, score := range stats.PageRank() {
    process(id, score)
}
```

### After (New Pattern)

```go
// Getting single value
if score, ok := stats.PageRankValue(issueID); ok {
    // use score
}

// Checking existence
if _, ok := stats.BetweennessValue(issueID); ok {
    // exists
}

// Iterating
stats.PageRankAll(func(id string, score float64) bool {
    process(id, score)
    return true  // continue iteration
})
```

## When to Use Each Pattern

| Scenario | Use |
|----------|-----|
| Need single value | `*Value()` |
| Need to check existence | `*Value()` (check bool) |
| Need all values, will process all | `*All()` |
| Need all values with early termination | `*All()` (return false to stop) |
| Need to pass map to external function | Legacy `*()` |
| Need map for modification/sorting | Legacy `*()` |

## Files

- `pkg/analysis/graph.go` - Implementation
- `pkg/analysis/graph_accessor_test.go` - Unit tests
- `pkg/analysis/graph_accessor_benchmark_test.go` - Benchmarks
- `docs/accessor_pattern.md` - This documentation
