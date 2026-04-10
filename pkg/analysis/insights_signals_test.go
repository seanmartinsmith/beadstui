package analysis

import (
	"sort"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// Graph: square A-B-C-D (cycle) + leaf E attached to C.
// Undirected view: A-B, B-C, C-D, D-A, C-E.
// Expectations:
// - Core numbers: A,B,C,D >=2; E lower.
// - Articulation: C (removing disconnects E).
func TestInsightsIncludesCoreAndArticulation(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{IssueID: "B", DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{IssueID: "C", DependsOnID: "B", Type: model.DepBlocks}}},
		{ID: "D", Status: model.StatusOpen, Dependencies: []*model.Dependency{{IssueID: "D", DependsOnID: "C", Type: model.DepBlocks}}},
		{ID: "A2", Status: model.StatusOpen, Dependencies: []*model.Dependency{{IssueID: "A2", DependsOnID: "D", Type: model.DepBlocks}, {IssueID: "A2", DependsOnID: "A", Type: model.DepBlocks}}}, // closes cycle
		{ID: "E", Status: model.StatusOpen, Dependencies: []*model.Dependency{{IssueID: "E", DependsOnID: "C", Type: model.DepBlocks}}},
	}

	an := NewAnalyzer(issues)
	stats := an.Analyze()
	ins := stats.GenerateInsights(10)

	if len(ins.Cores) == 0 {
		t.Fatalf("expected cores list populated; core map=%v", stats.CoreNumber())
	}
	if ins.Cores[0].Value < ins.Cores[len(ins.Cores)-1].Value {
		t.Fatalf("cores not sorted desc: %#v", ins.Cores)
	}
	foundC := false
	for _, id := range ins.Articulation {
		if id == "C" {
			foundC = true
		}
	}
	if !foundC {
		t.Fatalf("expected articulation points to include C, got %v", ins.Articulation)
	}
}

// TestKCoreLinearChain verifies k-core on a simple linear chain A->B->C->D.
// Linear chains have all nodes with k-core = 1 (each node has at most 1 neighbor in undirected view).
func TestKCoreLinearChain(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}}},
		{ID: "D", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "C", Type: model.DepBlocks}}},
	}

	an := NewAnalyzer(issues)
	stats := an.Analyze()
	coreNum := stats.CoreNumber()

	// All nodes in a linear chain should have k-core = 1
	for id, core := range coreNum {
		if core != 1 {
			t.Errorf("node %s: expected k-core=1, got %d", id, core)
		}
	}
}

// TestKCoreTriangle verifies k-core on a triangle (3-clique).
// All nodes in a triangle have k-core = 2.
func TestKCoreTriangle(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}, {DependsOnID: "C", Type: model.DepBlocks}}},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "C", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen},
	}

	an := NewAnalyzer(issues)
	stats := an.Analyze()
	coreNum := stats.CoreNumber()

	// All nodes in a triangle should have k-core = 2
	for id, core := range coreNum {
		if core != 2 {
			t.Errorf("node %s: expected k-core=2 in triangle, got %d", id, core)
		}
	}
}

// TestKCoreDisconnectedNodes verifies k-core on isolated nodes.
func TestKCoreDisconnectedNodes(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen},
		{ID: "B", Status: model.StatusOpen},
		{ID: "C", Status: model.StatusOpen},
	}

	an := NewAnalyzer(issues)
	stats := an.Analyze()
	coreNum := stats.CoreNumber()

	// Isolated nodes have k-core = 0
	for id, core := range coreNum {
		if core != 0 {
			t.Errorf("isolated node %s: expected k-core=0, got %d", id, core)
		}
	}
}

// TestArticulationLinearChain verifies articulation points in a linear chain.
// In A-B-C-D, nodes B and C are articulation points.
func TestArticulationLinearChain(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}}},
		{ID: "D", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "C", Type: model.DepBlocks}}},
	}

	an := NewAnalyzer(issues)
	stats := an.Analyze()
	artPts := stats.ArticulationPoints()
	sort.Strings(artPts)

	// B and C are articulation points (removing either disconnects the chain)
	expected := []string{"B", "C"}
	if len(artPts) != 2 {
		t.Fatalf("expected 2 articulation points, got %d: %v", len(artPts), artPts)
	}
	for i, exp := range expected {
		if artPts[i] != exp {
			t.Errorf("articulation point %d: expected %s, got %s", i, exp, artPts[i])
		}
	}
}

// TestArticulationTriangle verifies no articulation points in a triangle.
// A triangle (3-clique) has no cut vertices.
func TestArticulationTriangle(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}, {DependsOnID: "C", Type: model.DepBlocks}}},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "C", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen},
	}

	an := NewAnalyzer(issues)
	stats := an.Analyze()
	artPts := stats.ArticulationPoints()

	if len(artPts) != 0 {
		t.Errorf("expected no articulation points in triangle, got %v", artPts)
	}
}

// TestSlackLinearChain verifies slack computation on a linear chain.
// In A->B->C->D, all nodes are on the critical path, so slack = 0.
func TestSlackLinearChain(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}}},
		{ID: "D", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "C", Type: model.DepBlocks}}},
	}

	an := NewAnalyzer(issues)
	stats := an.Analyze()
	slack := stats.Slack()

	// All nodes on critical path should have slack = 0
	for id, s := range slack {
		if s != 0 {
			t.Errorf("node %s: expected slack=0 on critical path, got %f", id, s)
		}
	}
}

// TestSlackParallelPaths verifies slack computation with parallel paths.
// Graph: A -> B -> D and A -> C -> D
// The shorter parallel path should have slack.
func TestSlackParallelPaths(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "D", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}, {DependsOnID: "C", Type: model.DepBlocks}}},
	}

	an := NewAnalyzer(issues)
	stats := an.Analyze()
	slack := stats.Slack()

	// A and D are on critical path (slack = 0)
	// B and C are parallel, both have same path length
	if slack["A"] != 0 {
		t.Errorf("node A: expected slack=0 (start), got %f", slack["A"])
	}
	if slack["D"] != 0 {
		t.Errorf("node D: expected slack=0 (end), got %f", slack["D"])
	}
	// B and C should have equal slack (symmetric parallel paths)
	if slack["B"] != slack["C"] {
		t.Errorf("nodes B and C should have equal slack: B=%f, C=%f", slack["B"], slack["C"])
	}
}

// TestSlackDiamondWithLongPath verifies slack with asymmetric diamond.
// Graph: A -> B -> C -> E and A -> D -> E
// Short path A->D->E has slack, long path A->B->C->E is critical.
func TestSlackDiamondWithLongPath(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Status: model.StatusOpen},
		{ID: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}}},
		{ID: "D", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "E", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "C", Type: model.DepBlocks}, {DependsOnID: "D", Type: model.DepBlocks}}},
	}

	an := NewAnalyzer(issues)
	stats := an.Analyze()
	slack := stats.Slack()

	// Critical path: A->B->C->E (length 3)
	// Short path: A->D->E (length 2)
	// D should have slack = 1 (can be delayed by 1 step)
	if slack["A"] != 0 {
		t.Errorf("node A: expected slack=0, got %f", slack["A"])
	}
	if slack["B"] != 0 {
		t.Errorf("node B: expected slack=0 (critical path), got %f", slack["B"])
	}
	if slack["C"] != 0 {
		t.Errorf("node C: expected slack=0 (critical path), got %f", slack["C"])
	}
	if slack["D"] != 1 {
		t.Errorf("node D: expected slack=1 (short path), got %f", slack["D"])
	}
	if slack["E"] != 0 {
		t.Errorf("node E: expected slack=0, got %f", slack["E"])
	}
}

// TestGraphSignalsEmptyGraph verifies signals on empty graph.
func TestGraphSignalsEmptyGraph(t *testing.T) {
	an := NewAnalyzer([]model.Issue{})
	stats := an.Analyze()

	if len(stats.CoreNumber()) != 0 {
		t.Error("expected empty core number map")
	}
	if len(stats.ArticulationPoints()) != 0 {
		t.Error("expected no articulation points")
	}
	if stats.Slack() != nil && len(stats.Slack()) != 0 {
		t.Error("expected nil or empty slack map")
	}
}

// TestGraphSignalsSingleNode verifies signals on single node.
func TestGraphSignalsSingleNode(t *testing.T) {
	issues := []model.Issue{{ID: "A", Status: model.StatusOpen}}

	an := NewAnalyzer(issues)
	stats := an.Analyze()

	coreNum := stats.CoreNumber()
	if coreNum["A"] != 0 {
		t.Errorf("single node should have k-core=0, got %d", coreNum["A"])
	}
	if len(stats.ArticulationPoints()) != 0 {
		t.Error("single node should have no articulation points")
	}
}
