package analysis

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// AnalysisConfig controls which metrics to compute and their timeouts.
// This enables size-based algorithm selection for optimal performance.
type AnalysisConfig struct {
	// Betweenness centrality (expensive: O(V*E))
	ComputeBetweenness       bool
	BetweennessTimeout       time.Duration
	BetweennessSkipReason    string          // Set when skipped, explains why
	BetweennessMode          BetweennessMode // "exact", "approximate", or "skip"
	BetweennessSampleSize    int             // Sample size for approximate mode
	BetweennessIsApproximate bool            // True if approximation was used (set after computation)

	// PageRank
	ComputePageRank    bool
	PageRankTimeout    time.Duration
	PageRankSkipReason string

	// HITS (Hubs and Authorities)
	ComputeHITS    bool
	HITSTimeout    time.Duration
	HITSSkipReason string

	// Cycle detection (potentially exponential)
	ComputeCycles    bool
	CyclesTimeout    time.Duration
	MaxCyclesToStore int
	CyclesSkipReason string

	// Eigenvector centrality (usually fast)
	ComputeEigenvector bool

	// Critical path scoring (fast, O(V+E))
	ComputeCriticalPath bool

	// Advanced graph signals (bv-t1js optimization)
	// These can be skipped for triage-only mode to reduce latency
	ComputeKCore        bool // k-core decomposition
	ComputeArticulation bool // Articulation points
	ComputeSlack        bool // Scheduling slack
}

// DefaultConfig returns the default analysis configuration.
// All metrics enabled with standard timeouts. Uses exact betweenness.
func DefaultConfig() AnalysisConfig {
	cfg := AnalysisConfig{
		ComputeBetweenness: true,
		BetweennessMode:    BetweennessExact,
		BetweennessTimeout: 500 * time.Millisecond,

		ComputePageRank: true,
		PageRankTimeout: 500 * time.Millisecond,

		ComputeHITS: true,
		HITSTimeout: 500 * time.Millisecond,

		ComputeCycles:    true,
		CyclesTimeout:    500 * time.Millisecond,
		MaxCyclesToStore: 100,

		ComputeEigenvector:  true,
		ComputeCriticalPath: true,

		ComputeKCore:        true,
		ComputeArticulation: true,
		ComputeSlack:        true,
	}
	return ApplyEnvOverrides(cfg)
}

// ConfigForSize returns an appropriate configuration based on graph size.
// Larger graphs get more aggressive timeouts and may use approximate algorithms.
//
// Size tiers:
//   - Small (<100 nodes): Full analysis with exact algorithms, generous timeouts
//   - Medium (100-500 nodes): Exact algorithms with standard timeouts
//   - Large (500-2000 nodes): Approximate betweenness for sparse graphs, skip for dense
//   - XL (>2000 nodes): Approximate betweenness, skip cycles and HITS for dense graphs
func ConfigForSize(nodeCount, edgeCount int) AnalysisConfig {
	density := 0.0
	if nodeCount > 1 {
		density = float64(edgeCount) / float64(nodeCount*(nodeCount-1))
	}

	var cfg AnalysisConfig
	switch {
	case nodeCount < 100:
		// Small graph: run everything with generous timeouts, exact betweenness
		cfg = AnalysisConfig{
			ComputeBetweenness: true,
			BetweennessMode:    BetweennessExact,
			BetweennessTimeout: 2 * time.Second,

			ComputePageRank: true,
			PageRankTimeout: 2 * time.Second,

			ComputeHITS: true,
			HITSTimeout: 2 * time.Second,

			ComputeCycles:    true,
			CyclesTimeout:    2 * time.Second,
			MaxCyclesToStore: 1000,

			ComputeEigenvector:  true,
			ComputeCriticalPath: true,

			ComputeKCore:        true,
			ComputeArticulation: true,
			ComputeSlack:        true,
		}

	case nodeCount < 500:
		// Medium graph: standard timeouts, exact betweenness
		cfg = AnalysisConfig{
			ComputeBetweenness: true,
			BetweennessMode:    BetweennessExact,
			BetweennessTimeout: 500 * time.Millisecond,

			ComputePageRank: true,
			PageRankTimeout: 500 * time.Millisecond,

			ComputeHITS: true,
			HITSTimeout: 500 * time.Millisecond,

			ComputeCycles:    true,
			CyclesTimeout:    500 * time.Millisecond,
			MaxCyclesToStore: 100,

			ComputeEigenvector:  true,
			ComputeCriticalPath: true,

			ComputeKCore:        true,
			ComputeArticulation: true,
			ComputeSlack:        true,
		}

	case nodeCount < 2000:
		// Large graph: use approximate betweenness, shorter timeouts
		cfg = AnalysisConfig{
			ComputePageRank: true,
			PageRankTimeout: 300 * time.Millisecond,

			ComputeHITS: true,
			HITSTimeout: 300 * time.Millisecond,

			ComputeCycles:    true,
			CyclesTimeout:    300 * time.Millisecond,
			MaxCyclesToStore: 50,

			ComputeEigenvector:  true,
			ComputeCriticalPath: true,

			ComputeKCore:        true,
			ComputeArticulation: true,
			ComputeSlack:        true,
		}

		// Use approximate betweenness for large sparse graphs, skip for dense
		if density < 0.01 {
			cfg.ComputeBetweenness = true
			cfg.BetweennessMode = BetweennessApproximate
			cfg.BetweennessSampleSize = RecommendSampleSize(nodeCount, edgeCount)
			cfg.BetweennessTimeout = 500 * time.Millisecond // More time for sampling
		} else {
			cfg.ComputeBetweenness = false
			cfg.BetweennessMode = BetweennessSkip
			cfg.BetweennessSkipReason = "graph too dense (density > 0.01)"
		}

	default:
		// XL graph (>2000 nodes): use approximate betweenness with larger sample
		cfg = AnalysisConfig{
			// Use approximate betweenness for XL graphs
			ComputeBetweenness:    true,
			BetweennessMode:       BetweennessApproximate,
			BetweennessSampleSize: RecommendSampleSize(nodeCount, edgeCount),
			BetweennessTimeout:    500 * time.Millisecond,

			ComputePageRank: true,
			PageRankTimeout: 200 * time.Millisecond,

			ComputeCycles:    false,
			CyclesSkipReason: "graph too large (>2000 nodes)",
			MaxCyclesToStore: 10,

			ComputeEigenvector:  true,
			ComputeCriticalPath: true,

			ComputeKCore:        true,
			ComputeArticulation: true,
			ComputeSlack:        true,
		}

		// Only compute HITS for very sparse XL graphs
		if density < 0.001 {
			cfg.ComputeHITS = true
			cfg.HITSTimeout = 200 * time.Millisecond
		} else {
			cfg.ComputeHITS = false
			cfg.HITSSkipReason = "graph too large and dense"
		}
	}
	return ApplyEnvOverrides(cfg)
}

// FullAnalysisConfig returns a config that computes all metrics regardless of size.
// Useful when --force-full-analysis is specified. Uses exact betweenness.
func FullAnalysisConfig() AnalysisConfig {
	cfg := AnalysisConfig{
		ComputeBetweenness: true,
		BetweennessMode:    BetweennessExact, // Force exact for full analysis
		BetweennessTimeout: 30 * time.Second, // Very generous for forced full analysis

		ComputePageRank: true,
		PageRankTimeout: 30 * time.Second,

		ComputeHITS: true,
		HITSTimeout: 30 * time.Second,

		ComputeCycles:    true,
		CyclesTimeout:    30 * time.Second,
		MaxCyclesToStore: 10000,

		ComputeEigenvector:  true,
		ComputeCriticalPath: true,

		ComputeKCore:        true,
		ComputeArticulation: true,
		ComputeSlack:        true,
	}
	return ApplyEnvOverrides(cfg)
}

// TriageConfig returns a minimal config optimized for triage operations.
// Only computes PageRank and Betweenness which are needed for triage scoring.
// Skips Eigenvector, HITS, Cycles, k-core, articulation, and slack for 50-200ms savings.
// (bv-t1js optimization)
func TriageConfig() AnalysisConfig {
	cfg := AnalysisConfig{
		ComputeBetweenness:    true,
		BetweennessMode:       BetweennessApproximate,
		BetweennessSampleSize: 50, // Fast approximation
		BetweennessTimeout:    200 * time.Millisecond,

		ComputePageRank: true,
		PageRankTimeout: 200 * time.Millisecond,

		// Disable metrics not needed for triage
		ComputeHITS:         false,
		ComputeCycles:       false,
		ComputeEigenvector:  false,
		ComputeCriticalPath: false,
		ComputeKCore:        false,
		ComputeArticulation: false,
		ComputeSlack:        false,
	}
	return ApplyEnvOverrides(cfg)
}

// AllPhase2Disabled returns true if all Phase 2 metrics are disabled.
// When this returns true, the Phase 2 goroutine can be skipped entirely.
func (c AnalysisConfig) AllPhase2Disabled() bool {
	return !c.ComputeBetweenness &&
		!c.ComputePageRank &&
		!c.ComputeHITS &&
		!c.ComputeCycles &&
		!c.ComputeEigenvector &&
		!c.ComputeCriticalPath &&
		!c.ComputeKCore &&
		!c.ComputeArticulation &&
		!c.ComputeSlack
}

// NoPhase2Config returns a config with all Phase 2 metrics disabled.
// Use this when Phase 2 metrics are not needed (e.g., all issues closed).
func NoPhase2Config() AnalysisConfig {
	return AnalysisConfig{
		// All Phase 2 metrics disabled
		ComputeBetweenness:  false,
		ComputePageRank:     false,
		ComputeHITS:         false,
		ComputeCycles:       false,
		ComputeEigenvector:  false,
		ComputeCriticalPath: false,
		ComputeKCore:        false,
		ComputeArticulation: false,
		ComputeSlack:        false,
	}
}

// SkippedMetrics returns a list of metrics that are configured to be skipped.
func (c AnalysisConfig) SkippedMetrics() []SkippedMetric {
	var skipped []SkippedMetric

	if !c.ComputeBetweenness {
		skipped = append(skipped, SkippedMetric{
			Name:   "Betweenness",
			Reason: c.BetweennessSkipReason,
		})
	}
	if !c.ComputePageRank {
		skipped = append(skipped, SkippedMetric{
			Name:   "PageRank",
			Reason: c.PageRankSkipReason,
		})
	}
	if !c.ComputeHITS {
		skipped = append(skipped, SkippedMetric{
			Name:   "HITS",
			Reason: c.HITSSkipReason,
		})
	}
	if !c.ComputeCycles {
		skipped = append(skipped, SkippedMetric{
			Name:   "Cycles",
			Reason: c.CyclesSkipReason,
		})
	}

	return skipped
}

// SkippedMetric describes a metric that was skipped and why.
type SkippedMetric struct {
	Name   string
	Reason string
}

const (
	// EnvSkipPhase2 disables most Phase 2 metrics (centrality, cycles, critical path).
	EnvSkipPhase2 = "BV_SKIP_PHASE2"
	// EnvPhase2TimeoutSeconds overrides per-metric Phase 2 timeouts when set (>0).
	EnvPhase2TimeoutSeconds = "BV_PHASE2_TIMEOUT_S"
)

// ApplyEnvOverrides applies environment-variable tunables to the analysis config.
//
// Supported:
//   - BV_SKIP_PHASE2=1: skip expensive Phase 2 metrics (PageRank, Betweenness, HITS, Cycles,
//     Eigenvector, Critical Path). (k-core/articulation/slack remain enabled.)
//   - BV_PHASE2_TIMEOUT_S=N: override per-metric timeouts to N seconds (must be >0).
func ApplyEnvOverrides(cfg AnalysisConfig) AnalysisConfig {
	if envBool(EnvSkipPhase2) {
		cfg.ComputeBetweenness = false
		cfg.BetweennessMode = BetweennessSkip
		cfg.BetweennessSkipReason = "BV_SKIP_PHASE2 set"

		cfg.ComputePageRank = false
		cfg.PageRankSkipReason = "BV_SKIP_PHASE2 set"

		cfg.ComputeHITS = false
		cfg.HITSSkipReason = "BV_SKIP_PHASE2 set"

		cfg.ComputeCycles = false
		cfg.CyclesSkipReason = "BV_SKIP_PHASE2 set"

		cfg.ComputeEigenvector = false
		cfg.ComputeCriticalPath = false
	}

	if seconds, ok := envPositiveInt(EnvPhase2TimeoutSeconds); ok {
		timeout := time.Duration(seconds) * time.Second
		if cfg.ComputeBetweenness {
			cfg.BetweennessTimeout = timeout
		}
		if cfg.ComputePageRank {
			cfg.PageRankTimeout = timeout
		}
		if cfg.ComputeHITS {
			cfg.HITSTimeout = timeout
		}
		if cfg.ComputeCycles {
			cfg.CyclesTimeout = timeout
		}
	}

	return cfg
}

func envPositiveInt(name string) (int, bool) {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func envBool(name string) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
