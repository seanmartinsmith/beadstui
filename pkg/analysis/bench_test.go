package analysis_test

import (
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"

	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

// ============================================================================
// Full Analysis Benchmarks (the complete Analyze() pipeline)
// ============================================================================

func BenchmarkFullAnalysis_Sparse100(b *testing.B) {
	benchFullAnalysis(b, generateSparseGraph(100))
}

func BenchmarkFullAnalysis_Sparse500(b *testing.B) {
	benchFullAnalysis(b, generateSparseGraph(500))
}

func BenchmarkFullAnalysis_Sparse1000(b *testing.B) {
	benchFullAnalysis(b, generateSparseGraph(1000))
}

func BenchmarkFullAnalysis_Dense100(b *testing.B) {
	benchFullAnalysis(b, generateDenseGraph(100))
}

func BenchmarkFullAnalysis_Dense500(b *testing.B) {
	benchFullAnalysis(b, generateDenseGraph(500))
}

func BenchmarkFullAnalysis_Chain100(b *testing.B) {
	benchFullAnalysis(b, generateChainGraph(100))
}

func BenchmarkFullAnalysis_Chain500(b *testing.B) {
	benchFullAnalysis(b, generateChainGraph(500))
}

func BenchmarkFullAnalysis_Chain1000(b *testing.B) {
	benchFullAnalysis(b, generateChainGraph(1000))
}

func BenchmarkFullAnalysis_Wide500(b *testing.B) {
	benchFullAnalysis(b, generateWideGraph(500))
}

func BenchmarkFullAnalysis_Deep500(b *testing.B) {
	benchFullAnalysis(b, generateDeepGraph(500))
}

func BenchmarkFullAnalysis_Disconnected500(b *testing.B) {
	benchFullAnalysis(b, generateDisconnectedGraph(500))
}

// ============================================================================
// Robot Workload Benchmarks (end-to-end scoring + JSON-friendly structs)
// ============================================================================

func BenchmarkRobotTriage_Sparse500(b *testing.B) {
	issues := generateSparseGraph(500)
	opts := analysis.TriageOptions{WaitForPhase2: true}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = analysis.ComputeTriageWithOptions(issues, opts)
	}
}

func benchFullAnalysis(b *testing.B, issues []model.Issue) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		an := analysis.NewAnalyzer(issues)
		_ = an.Analyze()
	}
}

// ============================================================================
// Individual Algorithm Benchmarks (using raw gonum for isolation)
// ============================================================================

// PageRank benchmarks
func BenchmarkPageRank_Sparse100(b *testing.B) {
	benchPageRank(b, generateSparseGraph(100))
}

func BenchmarkPageRank_Sparse500(b *testing.B) {
	benchPageRank(b, generateSparseGraph(500))
}

func BenchmarkPageRank_Sparse1000(b *testing.B) {
	benchPageRank(b, generateSparseGraph(1000))
}

func BenchmarkPageRank_Dense500(b *testing.B) {
	benchPageRank(b, generateDenseGraph(500))
}

func BenchmarkPageRank_Chain1000(b *testing.B) {
	benchPageRank(b, generateChainGraph(1000))
}

func benchPageRank(b *testing.B, issues []model.Issue) {
	g := buildGraph(issues)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = network.PageRank(g, 0.85, 1e-6)
	}
}

// Betweenness benchmarks
func BenchmarkBetweenness_Sparse100(b *testing.B) {
	benchBetweenness(b, generateSparseGraph(100))
}

func BenchmarkBetweenness_Sparse500(b *testing.B) {
	benchBetweenness(b, generateSparseGraph(500))
}

func BenchmarkBetweenness_Chain500(b *testing.B) {
	benchBetweenness(b, generateChainGraph(500))
}

func BenchmarkBetweenness_Dense100(b *testing.B) {
	benchBetweenness(b, generateDenseGraph(100))
}

func benchBetweenness(b *testing.B, issues []model.Issue) {
	g := buildGraph(issues)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = network.Betweenness(g)
	}
}

// HITS benchmarks
func BenchmarkHITS_Sparse100(b *testing.B) {
	benchHITS(b, generateSparseGraph(100))
}

func BenchmarkHITS_Sparse500(b *testing.B) {
	benchHITS(b, generateSparseGraph(500))
}

func BenchmarkHITS_Dense100(b *testing.B) {
	benchHITS(b, generateDenseGraph(100))
}

func benchHITS(b *testing.B, issues []model.Issue) {
	g := buildGraph(issues)
	if g.Edges().Len() == 0 {
		b.Skip("Graph has no edges")
	}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = network.HITS(g, 1e-3)
	}
}

// Topological Sort benchmarks
func BenchmarkTopoSort_Sparse500(b *testing.B) {
	benchTopoSort(b, generateSparseGraph(500))
}

func BenchmarkTopoSort_Chain1000(b *testing.B) {
	benchTopoSort(b, generateChainGraph(1000))
}

func BenchmarkTopoSort_Deep1000(b *testing.B) {
	benchTopoSort(b, generateDeepGraph(1000))
}

func benchTopoSort(b *testing.B, issues []model.Issue) {
	g := buildGraph(issues)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = topo.Sort(g)
	}
}

// SCC (Tarjan) benchmarks - used for cycle pre-check
func BenchmarkTarjanSCC_Sparse500(b *testing.B) {
	benchTarjanSCC(b, generateSparseGraph(500))
}

func BenchmarkTarjanSCC_Chain1000(b *testing.B) {
	benchTarjanSCC(b, generateChainGraph(1000))
}

func BenchmarkTarjanSCC_Cyclic100(b *testing.B) {
	benchTarjanSCC(b, generateCyclicGraph(100))
}

func benchTarjanSCC(b *testing.B, issues []model.Issue) {
	g := buildGraph(issues)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = topo.TarjanSCC(g)
	}
}

// ============================================================================
// Analyzer Construction Benchmark
// ============================================================================

func BenchmarkNewAnalyzer_Sparse500(b *testing.B) {
	issues := generateSparseGraph(500)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = analysis.NewAnalyzer(issues)
	}
}

func BenchmarkNewAnalyzer_Sparse1000(b *testing.B) {
	issues := generateSparseGraph(1000)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = analysis.NewAnalyzer(issues)
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// buildGraph creates a gonum DirectedGraph from issues (same logic as Analyzer)
func buildGraph(issues []model.Issue) *simple.DirectedGraph {
	g := simple.NewDirectedGraph()
	idToNode := make(map[string]int64, len(issues))

	// Add nodes
	for _, issue := range issues {
		n := g.NewNode()
		g.AddNode(n)
		idToNode[issue.ID] = n.ID()
	}

	// Add edges (only blocking dependencies)
	for _, issue := range issues {
		u, ok := idToNode[issue.ID]
		if !ok {
			continue
		}

		for _, dep := range issue.Dependencies {
			if dep == nil || dep.Type != model.DepBlocks {
				continue
			}
			v, exists := idToNode[dep.DependsOnID]
			if exists {
				g.SetEdge(g.NewEdge(g.Node(u), g.Node(v)))
			}
		}
	}

	return g
}
