// tree_bench_test.go - Performance benchmarks for tree view (bv-e0oi)
package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/charmbracelet/lipgloss"
)

// generateHierarchyIssues creates a set of issues with parent-child relationships.
// roots: number of root issues
// childrenPerNode: number of children each non-leaf node has
// depth: maximum depth of the tree
func generateHierarchyIssues(roots, childrenPerNode, depth int) []model.Issue {
	var issues []model.Issue
	counter := 0
	now := time.Now()

	var generate func(parentID string, currentDepth int)
	generate = func(parentID string, currentDepth int) {
		if currentDepth > depth {
			return
		}

		for i := 0; i < childrenPerNode; i++ {
			counter++
			id := fmt.Sprintf("issue-%d", counter)
			issue := model.Issue{
				ID:        id,
				Title:     fmt.Sprintf("Issue %d at depth %d", counter, currentDepth),
				Priority:  currentDepth % 5, // Vary priority
				IssueType: model.TypeTask,
				Status:    model.StatusOpen,
				CreatedAt: now.Add(time.Duration(counter) * time.Minute),
			}

			if parentID != "" {
				issue.Dependencies = []*model.Dependency{
					{
						IssueID:     id,
						DependsOnID: parentID,
						Type:        model.DepParentChild,
					},
				}
			}

			issues = append(issues, issue)
			generate(id, currentDepth+1)
		}
	}

	// Generate root nodes and their descendants
	for i := 0; i < roots; i++ {
		counter++
		id := fmt.Sprintf("root-%d", i)
		issues = append(issues, model.Issue{
			ID:        id,
			Title:     fmt.Sprintf("Root %d", i),
			Priority:  1,
			IssueType: model.TypeEpic,
			Status:    model.StatusOpen,
			CreatedAt: now.Add(time.Duration(i) * time.Hour),
		})
		generate(id, 1)
	}

	return issues
}

// generateFlatIssues creates issues with no parent-child relationships (all roots).
func generateFlatIssues(count int) []model.Issue {
	issues := make([]model.Issue, count)
	now := time.Now()

	for i := 0; i < count; i++ {
		issues[i] = model.Issue{
			ID:        fmt.Sprintf("flat-%d", i),
			Title:     fmt.Sprintf("Flat Issue %d", i),
			Priority:  i % 5,
			IssueType: model.TypeTask,
			Status:    model.StatusOpen,
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
		}
	}

	return issues
}

// BenchmarkTreeBuild measures tree building performance.
// Target: < 10ms for 1000 issues, Max acceptable: 50ms
func BenchmarkTreeBuild(b *testing.B) {
	benchmarks := []struct {
		name   string
		issues []model.Issue
	}{
		{"100_flat", generateFlatIssues(100)},
		{"500_flat", generateFlatIssues(500)},
		{"1000_flat", generateFlatIssues(1000)},
		{"100_hierarchy_depth3", generateHierarchyIssues(10, 3, 3)},
		{"500_hierarchy_depth4", generateHierarchyIssues(10, 4, 4)},
		{"1000_hierarchy_depth5", generateHierarchyIssues(10, 3, 5)},
	}

	theme := DefaultTheme(lipgloss.NewRenderer(nil))

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tree := NewTreeModel(theme)
				tree.Build(bm.issues)
			}
		})
	}
}

// BenchmarkTreeBuild1000 specifically tests the 1000 issue target.
func BenchmarkTreeBuild1000(b *testing.B) {
	issues := generateHierarchyIssues(20, 4, 4) // Creates ~1000 issues
	theme := DefaultTheme(lipgloss.NewRenderer(nil))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tree := NewTreeModel(theme)
		tree.Build(issues)
	}
}

// BenchmarkTreeRender measures rendering performance.
// Target: < 5ms for 100 visible nodes, Max acceptable: 20ms
func BenchmarkTreeRender(b *testing.B) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))

	benchmarks := []struct {
		name    string
		roots   int
		perNode int
		depth   int
		width   int
		height  int
	}{
		{"50_visible_80x24", 10, 2, 3, 80, 24},
		{"100_visible_120x40", 20, 3, 3, 120, 40},
		{"200_visible_160x50", 30, 3, 4, 160, 50},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			issues := generateHierarchyIssues(bm.roots, bm.perNode, bm.depth)
			tree := NewTreeModel(theme)
			tree.Build(issues)
			tree.SetSize(bm.width, bm.height)
			tree.ExpandAll()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = tree.View()
			}
		})
	}
}

// BenchmarkTreeRender100 specifically tests the 100 visible nodes target.
func BenchmarkTreeRender100(b *testing.B) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	issues := generateHierarchyIssues(20, 3, 3)

	tree := NewTreeModel(theme)
	tree.Build(issues)
	tree.SetSize(120, 40)
	tree.ExpandAll()

	// Verify we have ~100 visible nodes
	if tree.NodeCount() < 80 || tree.NodeCount() > 150 {
		b.Logf("Warning: NodeCount=%d (expected ~100)", tree.NodeCount())
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = tree.View()
	}
}

// BenchmarkFlatListRebuild measures flat list rebuild performance.
// Target: < 2ms, Max acceptable: 10ms
func BenchmarkFlatListRebuild(b *testing.B) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))

	benchmarks := []struct {
		name  string
		count int
	}{
		{"100_nodes", 100},
		{"500_nodes", 500},
		{"1000_nodes", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			issues := generateHierarchyIssues(bm.count/10, 3, 3)
			tree := NewTreeModel(theme)
			tree.Build(issues)
			tree.ExpandAll()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				tree.rebuildFlatList()
			}
		})
	}
}

// BenchmarkTreeNavigation measures cursor movement operations.
func BenchmarkTreeNavigation(b *testing.B) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	issues := generateHierarchyIssues(20, 3, 4)

	tree := NewTreeModel(theme)
	tree.Build(issues)
	tree.ExpandAll()

	b.Run("MoveDown", func(b *testing.B) {
		tree.cursor = 0
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tree.MoveDown()
			if tree.cursor >= tree.NodeCount()-1 {
				tree.cursor = 0
			}
		}
	})

	b.Run("MoveUp", func(b *testing.B) {
		tree.cursor = tree.NodeCount() - 1
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tree.MoveUp()
			if tree.cursor <= 0 {
				tree.cursor = tree.NodeCount() - 1
			}
		}
	})

	b.Run("ToggleExpand", func(b *testing.B) {
		nodeCount := tree.NodeCount()
		if nodeCount == 0 {
			b.Skip("No nodes to test")
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tree.cursor = i % nodeCount
			tree.ToggleExpand()
		}
	})

	b.Run("JumpToParent", func(b *testing.B) {
		nodeCount := tree.NodeCount()
		if nodeCount == 0 {
			b.Skip("No nodes to test")
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tree.cursor = (i * 7) % nodeCount // Jump around
			tree.JumpToParent()
		}
	})
}

// BenchmarkTreeExpandCollapse measures expand/collapse all operations.
func BenchmarkTreeExpandCollapse(b *testing.B) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	issues := generateHierarchyIssues(20, 4, 4)

	tree := NewTreeModel(theme)
	tree.Build(issues)

	b.Run("ExpandAll", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tree.CollapseAll()
			tree.ExpandAll()
		}
	})

	b.Run("CollapseAll", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tree.ExpandAll()
			tree.CollapseAll()
		}
	})
}

// BenchmarkSelectByID measures ID-based selection performance.
func BenchmarkSelectByID(b *testing.B) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	issues := generateHierarchyIssues(20, 4, 4)

	tree := NewTreeModel(theme)
	tree.Build(issues)
	tree.ExpandAll()

	// Collect some IDs to search for
	ids := make([]string, 0, 100)
	for i, node := range tree.flatList {
		if i%10 == 0 && node != nil && node.Issue != nil {
			ids = append(ids, node.Issue.ID)
		}
	}

	if len(ids) == 0 {
		b.Fatal("No IDs collected for benchmark")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ids[i%len(ids)]
		tree.SelectByID(id)
	}
}

// BenchmarkBuildTreePrefix measures prefix building performance.
func BenchmarkBuildTreePrefix(b *testing.B) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	issues := generateHierarchyIssues(10, 4, 5) // Deep tree

	tree := NewTreeModel(theme)
	tree.Build(issues)
	tree.ExpandAll()

	// Find a deep node
	var deepNode *IssueTreeNode
	for _, node := range tree.flatList {
		if node != nil && node.Depth >= 4 {
			deepNode = node
			break
		}
	}

	if deepNode == nil {
		b.Fatal("Could not find deep node for benchmark")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tree.buildTreePrefix(deepNode)
	}
}
