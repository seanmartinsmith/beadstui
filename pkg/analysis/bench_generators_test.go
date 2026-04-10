package analysis_test

import (
	"fmt"
	"math/rand"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// Graph generators for benchmarks.
// Each generator creates a specific graph topology for performance testing.

// generateSparseGraph creates n nodes with ~2 random dependencies per node.
// Represents typical real-world issue trackers with moderate dependencies.
func generateSparseGraph(n int) []model.Issue {
	rng := rand.New(rand.NewSource(42)) // Deterministic for reproducibility
	issues := make([]model.Issue, n)

	for i := 0; i < n; i++ {
		issues[i] = model.Issue{
			ID:     fmt.Sprintf("SPARSE-%d", i),
			Status: model.StatusOpen,
		}

		// Add ~2 random dependencies to earlier nodes (avoiding cycles)
		numDeps := 1 + rng.Intn(3) // 1-3 deps
		for d := 0; d < numDeps && i > 0; d++ {
			depIdx := rng.Intn(i)
			issues[i].Dependencies = append(issues[i].Dependencies, &model.Dependency{
				DependsOnID: fmt.Sprintf("SPARSE-%d", depIdx),
				Type:        model.DepBlocks,
			})
		}
	}

	return issues
}

// generateDenseGraph creates n nodes with ~5 random dependencies per node.
// Tests performance under heavy interconnection.
func generateDenseGraph(n int) []model.Issue {
	rng := rand.New(rand.NewSource(42))
	issues := make([]model.Issue, n)

	for i := 0; i < n; i++ {
		issues[i] = model.Issue{
			ID:     fmt.Sprintf("DENSE-%d", i),
			Status: model.StatusOpen,
		}

		// Add ~5 random dependencies to earlier nodes
		numDeps := 3 + rng.Intn(5) // 3-7 deps
		for d := 0; d < numDeps && i > 0; d++ {
			depIdx := rng.Intn(i)
			issues[i].Dependencies = append(issues[i].Dependencies, &model.Dependency{
				DependsOnID: fmt.Sprintf("DENSE-%d", depIdx),
				Type:        model.DepBlocks,
			})
		}
	}

	return issues
}

// generateChainGraph creates a linear chain: 0 <- 1 <- 2 <- ... <- n-1.
// Tests topological sort and critical path performance.
func generateChainGraph(n int) []model.Issue {
	issues := make([]model.Issue, n)

	for i := 0; i < n; i++ {
		issues[i] = model.Issue{
			ID:     fmt.Sprintf("CHAIN-%d", i),
			Status: model.StatusOpen,
		}

		if i > 0 {
			issues[i].Dependencies = []*model.Dependency{
				{DependsOnID: fmt.Sprintf("CHAIN-%d", i-1), Type: model.DepBlocks},
			}
		}
	}

	return issues
}

// generateCyclicGraph creates a single cycle: 0 -> 1 -> 2 -> ... -> n-1 -> 0.
// Tests cycle detection performance.
func generateCyclicGraph(n int) []model.Issue {
	issues := make([]model.Issue, n)

	for i := 0; i < n; i++ {
		issues[i] = model.Issue{
			ID:     fmt.Sprintf("CYCLIC-%d", i),
			Status: model.StatusOpen,
		}

		// Each node depends on the next, wrapping around
		nextIdx := (i + 1) % n
		issues[i].Dependencies = []*model.Dependency{
			{DependsOnID: fmt.Sprintf("CYCLIC-%d", nextIdx), Type: model.DepBlocks},
		}
	}

	return issues
}

// generateManyCyclesGraph creates n nodes with many overlapping cycles.
// This is a pathological case that can cause exponential cycle enumeration.
func generateManyCyclesGraph(n int) []model.Issue {
	issues := make([]model.Issue, n)

	for i := 0; i < n; i++ {
		issues[i] = model.Issue{
			ID:     fmt.Sprintf("MANYCYC-%d", i),
			Status: model.StatusOpen,
		}

		// Create overlapping cycles by connecting to multiple neighbors
		// Each node connects to the next 2-3 nodes (wrapping)
		for offset := 1; offset <= 3 && offset < n; offset++ {
			nextIdx := (i + offset) % n
			issues[i].Dependencies = append(issues[i].Dependencies, &model.Dependency{
				DependsOnID: fmt.Sprintf("MANYCYC-%d", nextIdx),
				Type:        model.DepBlocks,
			})
		}
	}

	return issues
}

// generateCompleteGraph creates a complete graph where every node
// depends on every other node. This is highly pathological.
// WARNING: Only use small n (< 30) as complexity is O(n^2) edges.
func generateCompleteGraph(n int) []model.Issue {
	issues := make([]model.Issue, n)

	for i := 0; i < n; i++ {
		issues[i] = model.Issue{
			ID:     fmt.Sprintf("COMPLETE-%d", i),
			Status: model.StatusOpen,
		}

		// Connect to all other nodes
		for j := 0; j < n; j++ {
			if i != j {
				issues[i].Dependencies = append(issues[i].Dependencies, &model.Dependency{
					DependsOnID: fmt.Sprintf("COMPLETE-%d", j),
					Type:        model.DepBlocks,
				})
			}
		}
	}

	return issues
}

// generateDisconnectedGraph creates multiple disconnected components.
// Each component is a small chain of 5 nodes.
func generateDisconnectedGraph(n int) []model.Issue {
	componentSize := 5
	issues := make([]model.Issue, n)

	for i := 0; i < n; i++ {
		component := i / componentSize
		posInComponent := i % componentSize

		issues[i] = model.Issue{
			ID:     fmt.Sprintf("DISC-C%d-%d", component, posInComponent),
			Status: model.StatusOpen,
		}

		// Chain within component
		if posInComponent > 0 {
			issues[i].Dependencies = []*model.Dependency{
				{DependsOnID: fmt.Sprintf("DISC-C%d-%d", component, posInComponent-1), Type: model.DepBlocks},
			}
		}
	}

	return issues
}

// generateWideGraph creates a wide DAG where many nodes depend on a single root.
// Tests in-degree centrality and PageRank convergence.
func generateWideGraph(n int) []model.Issue {
	issues := make([]model.Issue, n)

	// Root node
	issues[0] = model.Issue{
		ID:     "WIDE-ROOT",
		Status: model.StatusOpen,
	}

	// All other nodes depend on root
	for i := 1; i < n; i++ {
		issues[i] = model.Issue{
			ID:     fmt.Sprintf("WIDE-%d", i),
			Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{DependsOnID: "WIDE-ROOT", Type: model.DepBlocks},
			},
		}
	}

	return issues
}

// generateDeepGraph creates a deep tree where root has children, each child
// has children, etc. Depth is log2(n), width is 2.
func generateDeepGraph(n int) []model.Issue {
	issues := make([]model.Issue, n)

	for i := 0; i < n; i++ {
		issues[i] = model.Issue{
			ID:     fmt.Sprintf("DEEP-%d", i),
			Status: model.StatusOpen,
		}

		// Each node depends on its parent (binary tree structure)
		if i > 0 {
			parentIdx := (i - 1) / 2
			issues[i].Dependencies = []*model.Dependency{
				{DependsOnID: fmt.Sprintf("DEEP-%d", parentIdx), Type: model.DepBlocks},
			}
		}
	}

	return issues
}
