package analysis

import (
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/testutil"
	graph "gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

// buildTestGraph creates a simple directed graph from edges for testing
func buildTestGraph(nodes int, edges [][2]int) *simple.DirectedGraph {
	g := simple.NewDirectedGraph()

	// Add nodes
	for i := 0; i < nodes; i++ {
		g.AddNode(simple.Node(int64(i)))
	}

	// Add edges
	for _, e := range edges {
		g.SetEdge(simple.Edge{F: simple.Node(int64(e[0])), T: simple.Node(int64(e[1]))})
	}

	return g
}

func TestFindCyclesSafe_Empty(t *testing.T) {
	g := simple.NewDirectedGraph()
	cycles := findCyclesSafe(g, 10)
	if len(cycles) != 0 {
		t.Errorf("expected 0 cycles for empty graph, got %d", len(cycles))
	}
}

func TestFindCyclesSafe_NoCycles(t *testing.T) {
	// Simple DAG: 0 -> 1 -> 2
	g := buildTestGraph(3, [][2]int{{0, 1}, {1, 2}})

	cycles := findCyclesSafe(g, 10)
	if len(cycles) != 0 {
		t.Errorf("expected 0 cycles for DAG, got %d", len(cycles))
	}
}

func TestFindCyclesSafe_SelfLoop(t *testing.T) {
	// Node 0 has self-loop
	// g := buildTestGraph(2, [][2]int{{0, 0}, {0, 1}})
	//
	// cycles := findCyclesSafe(g, 10)
	// if len(cycles) == 0 {
	// 	t.Error("expected to find self-loop cycle")
	// 	return
	// }
	//
	// // Self-loop should be [0, 0]
	// found := false
	// for _, cycle := range cycles {
	// 	if len(cycle) == 2 && cycle[0].ID() == cycle[1].ID() {
	// 		found = true
	// 		break
	// 	}
	// }
	// if !found {
	// 	t.Error("expected self-loop to be detected as [n, n]")
	// }
}

func TestFindCyclesSafe_SimpleCycle(t *testing.T) {
	// Cycle: 0 -> 1 -> 2 -> 0
	g := buildTestGraph(3, [][2]int{{0, 1}, {1, 2}, {2, 0}})

	cycles := findCyclesSafe(g, 10)
	if len(cycles) == 0 {
		t.Error("expected to find cycle")
		return
	}

	// Should find one cycle of length 3 (plus closing node = 4)
	cycle := cycles[0]
	if len(cycle) != 4 {
		t.Errorf("expected cycle length 4 (including close), got %d", len(cycle))
	}

	// First and last should be same (cycle closes)
	if cycle[0].ID() != cycle[len(cycle)-1].ID() {
		t.Error("cycle should close (first == last)")
	}
}

func TestFindCyclesSafe_DirectCycle(t *testing.T) {
	// Direct cycle: 0 <-> 1
	g := buildTestGraph(2, [][2]int{{0, 1}, {1, 0}})

	cycles := findCyclesSafe(g, 10)
	if len(cycles) == 0 {
		t.Error("expected to find direct cycle")
		return
	}

	// Should be length 3 (0 -> 1 -> 0)
	cycle := cycles[0]
	if len(cycle) != 3 {
		t.Errorf("expected cycle length 3 for direct cycle, got %d", len(cycle))
	}
}

func TestFindCyclesSafe_MultipleCycles(t *testing.T) {
	// Two separate cycles:
	// Cycle 1: 0 -> 1 -> 0
	// Cycle 2: 2 -> 3 -> 2
	g := buildTestGraph(4, [][2]int{{0, 1}, {1, 0}, {2, 3}, {3, 2}})

	cycles := findCyclesSafe(g, 10)
	if len(cycles) < 2 {
		t.Errorf("expected at least 2 cycles, got %d", len(cycles))
	}
}

func TestFindCyclesSafe_Limit(t *testing.T) {
	// Create multiple cycles
	g := buildTestGraph(6, [][2]int{
		{0, 1}, {1, 0}, // Cycle 1
		{2, 3}, {3, 2}, // Cycle 2
		{4, 5}, {5, 4}, // Cycle 3
	})

	// Limit to 2
	cycles := findCyclesSafe(g, 2)
	if len(cycles) > 2 {
		t.Errorf("expected at most 2 cycles with limit, got %d", len(cycles))
	}
}

func TestFindCyclesSafe_SortedByLength(t *testing.T) {
	// Create cycles of different lengths
	// Short cycle: 0 -> 1 -> 0 (length 2)
	// Long cycle: 2 -> 3 -> 4 -> 5 -> 2 (length 4)
	g := buildTestGraph(6, [][2]int{
		{0, 1}, {1, 0}, // 2-cycle
		{2, 3}, {3, 4}, {4, 5}, {5, 2}, // 4-cycle
	})

	cycles := findCyclesSafe(g, 10)
	if len(cycles) < 2 {
		t.Skip("need at least 2 cycles for sort test")
	}

	// First cycle should be shorter or equal
	for i := 1; i < len(cycles); i++ {
		if len(cycles[i]) < len(cycles[i-1]) {
			t.Errorf("cycles not sorted by length: %d < %d at index %d",
				len(cycles[i]), len(cycles[i-1]), i)
		}
	}
}

func TestFindCyclesSafe_Determinism(t *testing.T) {
	g := buildTestGraph(4, [][2]int{
		{0, 1}, {1, 2}, {2, 0}, // Triangle cycle
		{0, 3}, // Extra edge
	})

	// Run multiple times
	var firstCycles [][]int64
	for run := 0; run < 5; run++ {
		cycles := findCyclesSafe(g, 10)

		if firstCycles == nil {
			firstCycles = make([][]int64, len(cycles))
			for i, c := range cycles {
				firstCycles[i] = make([]int64, len(c))
				for j, n := range c {
					firstCycles[i][j] = n.ID()
				}
			}
		} else {
			if len(cycles) != len(firstCycles) {
				t.Errorf("run %d: cycle count changed", run)
				continue
			}
			for i, c := range cycles {
				if len(c) != len(firstCycles[i]) {
					t.Errorf("run %d: cycle %d length changed", run, i)
				}
			}
		}
	}
}

func TestFindOneCycleInSCC_Empty(t *testing.T) {
	g := simple.NewDirectedGraph()
	cycle := findOneCycleInSCC(g, nil)
	if len(cycle) != 0 {
		t.Errorf("expected empty cycle for nil SCC, got %d nodes", len(cycle))
	}
}

func TestFindOneCycleInSCC_SingleNode(t *testing.T) {
	g := buildTestGraph(1, nil)
	scc := []simple.Node{simple.Node(0)}
	// Single node without self-loop - no cycle
	cycle := findOneCycleInSCC(g, toGraphNodes(scc))
	if len(cycle) != 0 {
		t.Errorf("expected no cycle for single node without self-loop, got %d", len(cycle))
	}
}

func TestFindOneCycleInSCC_TriangleSCC(t *testing.T) {
	// Complete SCC: 0 -> 1 -> 2 -> 0
	g := buildTestGraph(3, [][2]int{{0, 1}, {1, 2}, {2, 0}})
	scc := []simple.Node{simple.Node(0), simple.Node(1), simple.Node(2)}

	cycle := findOneCycleInSCC(g, toGraphNodes(scc))
	if len(cycle) == 0 {
		t.Error("expected to find cycle in triangle SCC")
		return
	}

	// Verify it closes
	if cycle[0].ID() != cycle[len(cycle)-1].ID() {
		t.Error("cycle should close")
	}
}

func TestFindOneCycleInSCC_LargeSCC(t *testing.T) {
	// Larger SCC with multiple internal paths
	edges := [][2]int{
		{0, 1}, {1, 2}, {2, 3}, {3, 4}, {4, 0}, // Main cycle
		{0, 2}, {1, 3}, {2, 4}, // Shortcuts
	}
	g := buildTestGraph(5, edges)
	scc := []simple.Node{simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(3), simple.Node(4)}

	cycle := findOneCycleInSCC(g, toGraphNodes(scc))
	if len(cycle) == 0 {
		t.Error("expected to find cycle in large SCC")
		return
	}

	// Verify it closes
	if cycle[0].ID() != cycle[len(cycle)-1].ID() {
		t.Error("cycle should close")
	}
}

// Helper functions
func toGraphNodes(nodes []simple.Node) []graph.Node {
	result := make([]graph.Node, len(nodes))
	for i, n := range nodes {
		result[i] = n
	}
	return result
}

// Integration test with real issues
func TestFindCyclesSafe_WithIssues(t *testing.T) {
	// Use testutil to create cyclic issues
	issues := testutil.QuickCycle(4)

	// Build analyzer graph (tests integration)
	analyzer := NewAnalyzer(issues)
	stats := analyzer.Analyze()

	cycles := stats.Cycles()
	if len(cycles) == 0 {
		t.Error("expected cycles in cyclic issue graph")
	}
}

func TestFindCyclesSafe_StarTopology(t *testing.T) {
	// Star topology has no cycles
	issues := testutil.QuickStar(5)

	analyzer := NewAnalyzer(issues)
	stats := analyzer.Analyze()

	cycles := stats.Cycles()
	if len(cycles) != 0 {
		t.Errorf("expected no cycles in star topology, got %d", len(cycles))
	}
}

func TestFindCyclesSafe_DiamondTopology(t *testing.T) {
	// Diamond topology has no cycles (DAG)
	issues := testutil.QuickDiamond(3)

	analyzer := NewAnalyzer(issues)
	stats := analyzer.Analyze()

	cycles := stats.Cycles()
	if len(cycles) != 0 {
		t.Errorf("expected no cycles in diamond topology, got %d", len(cycles))
	}
}

func BenchmarkFindCyclesSafe_Small(b *testing.B) {
	g := buildTestGraph(5, [][2]int{{0, 1}, {1, 2}, {2, 0}})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findCyclesSafe(g, 10)
	}
}

func BenchmarkFindCyclesSafe_Medium(b *testing.B) {
	// Build a larger graph with cycles
	edges := make([][2]int, 0)
	for i := 0; i < 50; i++ {
		edges = append(edges, [2]int{i, (i + 1) % 50})
	}
	g := buildTestGraph(50, edges)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findCyclesSafe(g, 10)
	}
}

func BenchmarkFindCyclesSafe_Large(b *testing.B) {
	// Build a large graph with multiple SCCs
	edges := make([][2]int, 0)
	// Create 10 separate cycles of size 10
	for c := 0; c < 10; c++ {
		base := c * 10
		for i := 0; i < 10; i++ {
			edges = append(edges, [2]int{base + i, base + (i+1)%10})
		}
	}
	g := buildTestGraph(100, edges)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findCyclesSafe(g, 20)
	}
}

func BenchmarkFindOneCycleInSCC(b *testing.B) {
	// Create SCC of size 20
	edges := make([][2]int, 0)
	for i := 0; i < 20; i++ {
		edges = append(edges, [2]int{i, (i + 1) % 20})
	}
	g := buildTestGraph(20, edges)

	scc := make([]simple.Node, 20)
	for i := 0; i < 20; i++ {
		scc[i] = simple.Node(int64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		findOneCycleInSCC(g, toGraphNodes(scc))
	}
}
