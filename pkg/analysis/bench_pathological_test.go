package analysis_test

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"

	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/topo"
)

// ============================================================================
// Pathological Graph Benchmarks
// These test worst-case scenarios that could cause performance issues.
// ============================================================================

// Cycle detection with many overlapping cycles (exponential worst case)
// WARNING: These can be SLOW. The timeout protection should kick in.

func BenchmarkCycles_ManyCycles20(b *testing.B) {
	benchCycleDetection(b, generateManyCyclesGraph(20))
}

func BenchmarkCycles_ManyCycles30(b *testing.B) {
	// This should trigger timeout in production code
	benchCycleDetection(b, generateManyCyclesGraph(30))
}

func BenchmarkCycles_SingleCycle100(b *testing.B) {
	benchCycleDetection(b, generateCyclicGraph(100))
}

func benchCycleDetection(b *testing.B, issues []model.Issue) {
	g := buildGraph(issues)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Use timeout to prevent benchmark from hanging
		done := make(chan [][]interface{}, 1)
		go func() {
			cycles := topo.DirectedCyclesIn(g)
			done <- make([][]interface{}, len(cycles)) // Just count
		}()

		select {
		case <-done:
			// Completed
		case <-time.After(500 * time.Millisecond):
			// Timed out - this is expected for pathological cases
		}
	}
}

// Complete graph benchmarks (very dense - every node connects to every other)

func BenchmarkComplete_Betweenness15(b *testing.B) {
	// Complete graph with 15 nodes = 210 edges
	issues := generateCompleteGraph(15)
	g := buildGraph(issues)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = network.Betweenness(g)
	}
}

func BenchmarkComplete_PageRank20(b *testing.B) {
	issues := generateCompleteGraph(20)
	g := buildGraph(issues)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = network.PageRank(g, 0.85, 1e-6)
	}
}

func BenchmarkComplete_HITS15(b *testing.B) {
	issues := generateCompleteGraph(15)
	g := buildGraph(issues)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = network.HITS(g, 1e-3)
	}
}

// Long chain benchmarks (tests critical path / topological depth)

func BenchmarkLongChain_Betweenness500(b *testing.B) {
	issues := generateChainGraph(500)
	g := buildGraph(issues)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = network.Betweenness(g)
	}
}

func BenchmarkLongChain_TopoSort2000(b *testing.B) {
	issues := generateChainGraph(2000)
	g := buildGraph(issues)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = topo.Sort(g)
	}
}

// ============================================================================
// Full Analysis with Pathological Graphs
// Tests the complete pipeline with timeout protection
// ============================================================================

func BenchmarkFullAnalysis_ManyCycles20(b *testing.B) {
	issues := generateManyCyclesGraph(20)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		an := analysis.NewAnalyzer(issues)
		_ = an.Analyze()
	}
}

func BenchmarkFullAnalysis_ManyCycles30(b *testing.B) {
	// Should trigger cycle detection timeout
	issues := generateManyCyclesGraph(30)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		an := analysis.NewAnalyzer(issues)
		_ = an.Analyze()
	}
}

func BenchmarkFullAnalysis_Complete15(b *testing.B) {
	issues := generateCompleteGraph(15)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		an := analysis.NewAnalyzer(issues)
		_ = an.Analyze()
	}
}

// ============================================================================
// Timeout Verification Tests (not benchmarks, but important validation)
// ============================================================================

// TestTimeoutProtection_Betweenness verifies that large graphs don't hang
func TestTimeoutProtection_Betweenness(t *testing.T) {
	// Dense 1000-node graph - betweenness would take seconds without timeout
	issues := generateDenseGraph(1000)

	done := make(chan struct{})
	go func() {
		an := analysis.NewAnalyzer(issues)
		_ = an.Analyze()
		close(done)
	}()

	select {
	case <-done:
		// Good - completed (possibly with timeout)
	case <-time.After(3 * time.Second):
		t.Fatal("Analysis took too long - timeout protection may not be working")
	}
}

// TestTimeoutProtection_Cycles verifies that pathological cycle graphs don't hang
func TestTimeoutProtection_Cycles(t *testing.T) {
	// Many overlapping cycles - would cause exponential enumeration
	issues := generateManyCyclesGraph(30)

	done := make(chan struct{})
	go func() {
		an := analysis.NewAnalyzer(issues)
		_ = an.Analyze()
		close(done)
	}()

	select {
	case <-done:
		// Good - completed (likely with timeout marker)
	case <-time.After(3 * time.Second):
		t.Fatal("Analysis took too long - cycle timeout protection may not be working")
	}
}

// TestSCCPrecheck_AcyclicGraphFast verifies SCC pre-check skips enumeration
func TestSCCPrecheck_AcyclicGraphFast(t *testing.T) {
	// Large acyclic graph - SCC should detect no cycles quickly
	issues := generateChainGraph(500)

	start := time.Now()
	an := analysis.NewAnalyzer(issues)
	stats := an.Analyze()
	elapsed := time.Since(start)

	// Should complete quickly (SCC is O(V+E))
	if elapsed > 2*time.Second {
		t.Errorf("SCC pre-check should make acyclic analysis fast, took %v", elapsed)
	}

	// No cycles should be detected
	if len(stats.Cycles()) != 0 {
		t.Errorf("Expected no cycles in chain graph, got %d", len(stats.Cycles()))
	}
}
