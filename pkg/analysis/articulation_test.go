package analysis

import (
	"testing"
)

// Ensures articulation detection works with node ID 0 (no-parent sentinel safety).
func TestFindArticulationPointsHandlesZeroID(t *testing.T) {
	// Explicit IDs: 0-1-2 chain; 1 should be articulation.
	adj := undirectedAdjacency{
		nodes: []int64{0, 1, 2},
		neighbors: [][]int64{
			{1},
			{0, 2},
			{1},
		},
	}

	ap := findArticulationPoints(adj)
	if !ap[1] {
		t.Fatalf("expected node 1 to be articulation, got %v", ap)
	}
	if ap[0] || ap[2] {
		t.Fatalf("endpoints should not be articulation: %v", ap)
	}
}
