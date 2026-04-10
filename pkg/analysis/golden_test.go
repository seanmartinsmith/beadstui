package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// TestGraphFile represents a test graph file format.
type TestGraphFile struct {
	Description string   `json:"description"`
	Nodes       []string `json:"nodes"`
	Edges       [][]int  `json:"edges"`
}

// GoldenMetrics represents the expected metrics for a graph.
type GoldenMetrics struct {
	Description       string             `json:"description"`
	NodeCount         int                `json:"node_count"`
	EdgeCount         int                `json:"edge_count"`
	Density           float64            `json:"density"`
	PageRank          map[string]float64 `json:"pagerank"`
	Betweenness       map[string]float64 `json:"betweenness"`
	Eigenvector       map[string]float64 `json:"eigenvector"`
	Hubs              map[string]float64 `json:"hubs"`
	Authorities       map[string]float64 `json:"authorities"`
	CriticalPathScore map[string]float64 `json:"critical_path_score"`
	TopologicalOrder  []string           `json:"topological_order,omitempty"`
	CoreNumber        map[string]int     `json:"core_number"`
	Slack             map[string]float64 `json:"slack,omitempty"`
	HasCycles         bool               `json:"has_cycles"`
	Cycles            [][]string         `json:"cycles,omitempty"`
	OutDegree         map[string]int     `json:"out_degree"`
	InDegree          map[string]int     `json:"in_degree"`
}

// loadTestGraph loads a test graph JSON file and returns issues for the Analyzer.
func loadTestGraph(t *testing.T, path string) []model.Issue {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read test graph %s: %v", path, err)
	}

	var graph TestGraphFile
	if err := json.Unmarshal(data, &graph); err != nil {
		t.Fatalf("Failed to parse test graph %s: %v", path, err)
	}

	// Convert to issues with dependencies
	issues := make([]model.Issue, len(graph.Nodes))
	for i, nodeID := range graph.Nodes {
		issues[i] = model.Issue{
			ID:     nodeID,
			Title:  nodeID,
			Status: model.StatusOpen,
		}
	}

	// Build dependency map by target node
	depsBySource := make(map[int][]*model.Dependency)
	for _, edge := range graph.Edges {
		from, to := edge[0], edge[1]
		if from < len(graph.Nodes) && to < len(graph.Nodes) {
			// Edge from->to means "from depends on to" in bv convention
			dep := &model.Dependency{
				IssueID:     graph.Nodes[from],
				DependsOnID: graph.Nodes[to],
				Type:        model.DepBlocks,
			}
			depsBySource[from] = append(depsBySource[from], dep)
		}
	}

	// Attach dependencies to issues
	for i := range issues {
		if deps, ok := depsBySource[i]; ok {
			issues[i].Dependencies = deps
		}
	}

	return issues
}

// TestGenerateGoldenFiles generates expected metric values for all test graphs.
// Run with: go test -v -run TestGenerateGoldenFiles ./pkg/analysis/
// This updates the golden files in testdata/expected/
func TestGenerateGoldenFiles(t *testing.T) {
	if os.Getenv("GENERATE_GOLDEN") != "1" {
		t.Skip("Set GENERATE_GOLDEN=1 to regenerate golden files")
	}

	graphsDir := "../../testdata/graphs"
	expectedDir := "../../testdata/expected"

	// Ensure expected dir exists
	if err := os.MkdirAll(expectedDir, 0755); err != nil {
		t.Fatalf("Failed to create expected dir: %v", err)
	}

	// Find all test graphs
	entries, err := os.ReadDir(graphsDir)
	if err != nil {
		t.Fatalf("Failed to read graphs dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		graphPath := filepath.Join(graphsDir, entry.Name())
		baseName := entry.Name()[:len(entry.Name())-5] // Remove .json

		t.Run(baseName, func(t *testing.T) {
			// Load graph
			issues := loadTestGraph(t, graphPath)

			// Run analysis with full computation
			analyzer := NewAnalyzer(issues)
			config := FullAnalysisConfig()
			stats := analyzer.AnalyzeWithConfig(config)

			// Read original description
			data, _ := os.ReadFile(graphPath)
			var original TestGraphFile
			json.Unmarshal(data, &original)

			// Build golden metrics
			golden := GoldenMetrics{
				Description:       original.Description,
				NodeCount:         stats.NodeCount,
				EdgeCount:         stats.EdgeCount,
				Density:           stats.Density,
				PageRank:          stats.PageRank(),
				Betweenness:       stats.Betweenness(),
				Eigenvector:       stats.Eigenvector(),
				Hubs:              stats.Hubs(),
				Authorities:       stats.Authorities(),
				CriticalPathScore: stats.CriticalPathScore(),
				TopologicalOrder:  stats.TopologicalOrder,
				CoreNumber:        stats.CoreNumber(),
				Slack:             stats.Slack(),
				HasCycles:         len(stats.Cycles()) > 0,
				Cycles:            stats.Cycles(),
				OutDegree:         stats.OutDegree,
				InDegree:          stats.InDegree,
			}

			// Write golden file
			goldenPath := filepath.Join(expectedDir, baseName+"_metrics.json")
			goldenJSON, err := json.MarshalIndent(golden, "", "  ")
			if err != nil {
				t.Fatalf("Failed to marshal golden metrics: %v", err)
			}

			if err := os.WriteFile(goldenPath, goldenJSON, 0644); err != nil {
				t.Fatalf("Failed to write golden file %s: %v", goldenPath, err)
			}

			t.Logf("Generated %s", goldenPath)
		})
	}
}

// TestValidateGoldenFiles validates that Go analysis matches golden files.
func TestValidateGoldenFiles(t *testing.T) {
	graphsDir := "../../testdata/graphs"
	expectedDir := "../../testdata/expected"

	entries, err := os.ReadDir(expectedDir)
	if err != nil {
		t.Skipf("No expected dir found: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		baseName := entry.Name()[:len(entry.Name())-13] // Remove _metrics.json
		graphPath := filepath.Join(graphsDir, baseName+".json")
		goldenPath := filepath.Join(expectedDir, entry.Name())

		t.Run(baseName, func(t *testing.T) {
			// Skip if graph doesn't exist
			if _, err := os.Stat(graphPath); os.IsNotExist(err) {
				t.Skipf("Graph file not found: %s", graphPath)
			}

			// Load golden
			goldenData, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("Failed to read golden file: %v", err)
			}

			var expected GoldenMetrics
			if err := json.Unmarshal(goldenData, &expected); err != nil {
				t.Fatalf("Failed to parse golden file: %v", err)
			}

			// Run analysis
			issues := loadTestGraph(t, graphPath)
			analyzer := NewAnalyzer(issues)
			config := FullAnalysisConfig()
			stats := analyzer.AnalyzeWithConfig(config)

			// Validate basic counts
			if stats.NodeCount != expected.NodeCount {
				t.Errorf("NodeCount: got %d, want %d", stats.NodeCount, expected.NodeCount)
			}
			if stats.EdgeCount != expected.EdgeCount {
				t.Errorf("EdgeCount: got %d, want %d", stats.EdgeCount, expected.EdgeCount)
			}

			// Validate PageRank within tolerance (1e-5 for iterative convergence variance)
			validateMapFloat64(t, "PageRank", stats.PageRank(), expected.PageRank, 1e-5)

			// Validate Betweenness within tolerance
			validateMapFloat64(t, "Betweenness", stats.Betweenness(), expected.Betweenness, 1e-6)

			// Validate Eigenvector within tolerance
			validateMapFloat64(t, "Eigenvector", stats.Eigenvector(), expected.Eigenvector, 1e-6)

			// Validate HITS within tolerance
			validateMapFloat64(t, "Hubs", stats.Hubs(), expected.Hubs, 1e-6)
			validateMapFloat64(t, "Authorities", stats.Authorities(), expected.Authorities, 1e-6)

			// Validate Critical Path Score within tolerance
			validateMapFloat64(t, "CriticalPathScore", stats.CriticalPathScore(), expected.CriticalPathScore, 1e-6)

			// Validate cycle detection
			hasCycles := len(stats.Cycles()) > 0
			if hasCycles != expected.HasCycles {
				t.Errorf("HasCycles: got %v, want %v", hasCycles, expected.HasCycles)
			}
		})
	}
}

// validateMapFloat64 validates two maps are equal within tolerance.
func validateMapFloat64(t *testing.T, name string, actual, expected map[string]float64, tolerance float64) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Errorf("%s: got %d entries, want %d", name, len(actual), len(expected))
		return
	}

	// Sort keys for deterministic comparison
	keys := make([]string, 0, len(expected))
	for k := range expected {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		expVal := expected[k]
		actVal, ok := actual[k]
		if !ok {
			t.Errorf("%s[%s]: missing in actual", name, k)
			continue
		}
		diff := expVal - actVal
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			t.Errorf("%s[%s]: got %v, want %v (diff=%v)", name, k, actVal, expVal, diff)
		}
	}
}
