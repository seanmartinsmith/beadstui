package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGraphNavigationStatePreservation tests that graph navigation maintains consistent state
// when refreshing or filtering the view.
func TestGraphNavigationStatePreservation(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Set up a graph with multiple nodes in layers
	beads := `{"id":"root","title":"Root Issue","status":"open","priority":1,"issue_type":"task"}
{"id":"mid-1","title":"Middle 1","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"mid-1","depends_on_id":"root","type":"blocks"}]}
{"id":"mid-2","title":"Middle 2","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"mid-2","depends_on_id":"root","type":"blocks"}]}
{"id":"leaf","title":"Leaf Issue","status":"open","priority":3,"issue_type":"task","dependencies":[{"issue_id":"leaf","depends_on_id":"mid-1","type":"blocks"}]}`
	writeBeads(t, env, beads)

	// Query the graph
	cmd := exec.Command(bv, "--robot-graph")
	cmd.Dir = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("robot-graph failed: %v\n%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode failed: %v\n%s", err, out)
	}

	// Verify all nodes are present
	adj, ok := payload["adjacency"].(map[string]any)
	if !ok {
		t.Fatalf("missing adjacency in output")
	}
	nodes, ok := adj["nodes"].([]any)
	if !ok {
		t.Fatalf("missing nodes array")
	}
	if len(nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(nodes))
	}

	// Verify edges are present
	edges, ok := adj["edges"].([]any)
	if !ok {
		t.Fatalf("missing edges array")
	}
	if len(edges) != 3 {
		t.Errorf("expected 3 edges, got %d", len(edges))
	}
}

// TestGraphNavigationRootFilter tests filtering graph to a root node
func TestGraphNavigationRootFilter(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Chain: A -> B -> C -> D
	beads := `{"id":"A","title":"Root A","status":"open","priority":1,"issue_type":"task"}
{"id":"B","title":"Node B","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"B","depends_on_id":"A","type":"blocks"}]}
{"id":"C","title":"Node C","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"C","depends_on_id":"B","type":"blocks"}]}
{"id":"D","title":"Leaf D","status":"open","priority":3,"issue_type":"task","dependencies":[{"issue_id":"D","depends_on_id":"C","type":"blocks"}]}`
	writeBeads(t, env, beads)

	// Full graph should have 4 nodes
	fullGraph := runRobotGraph(t, bv, env)
	fullAdj := fullGraph["adjacency"].(map[string]any)
	fullNodes := fullAdj["nodes"].([]any)
	if len(fullNodes) != 4 {
		t.Errorf("full graph: expected 4 nodes, got %d", len(fullNodes))
	}

	// Filtered to C with depth 1 should have C and B (or C and D depending on direction)
	cmd := exec.Command(bv, "--robot-graph", "--graph-root=C", "--graph-depth=1")
	cmd.Dir = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("filtered graph failed: %v\n%s", err, out)
	}

	var filtered map[string]any
	if err := json.Unmarshal(out, &filtered); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}

	filteredAdj := filtered["adjacency"].(map[string]any)
	filteredNodes := filteredAdj["nodes"].([]any)

	// Should have fewer nodes than full graph
	if len(filteredNodes) >= len(fullNodes) {
		t.Errorf("filtered graph should have fewer nodes than full graph")
	}

	// C should be in the result
	var hasC bool
	for _, n := range filteredNodes {
		node := n.(map[string]any)
		if node["id"] == "C" {
			hasC = true
			break
		}
	}
	if !hasC {
		t.Error("filtered graph missing root node C")
	}
}

// TestGraphNavigationDepthLevels tests different depth levels
func TestGraphNavigationDepthLevels(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Linear chain: A -> B -> C -> D -> E
	beads := `{"id":"A","title":"A","status":"open","priority":1,"issue_type":"task"}
{"id":"B","title":"B","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"B","depends_on_id":"A","type":"blocks"}]}
{"id":"C","title":"C","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"C","depends_on_id":"B","type":"blocks"}]}
{"id":"D","title":"D","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"D","depends_on_id":"C","type":"blocks"}]}
{"id":"E","title":"E","status":"open","priority":3,"issue_type":"task","dependencies":[{"issue_id":"E","depends_on_id":"D","type":"blocks"}]}`
	writeBeads(t, env, beads)

	depths := []struct {
		depth    string
		minNodes int
		maxNodes int
	}{
		{"1", 2, 3}, // Root + immediate neighbors
		{"2", 2, 4}, // Root + 2 levels
		{"3", 3, 5}, // Root + 3 levels
	}

	for _, tt := range depths {
		t.Run("depth_"+tt.depth, func(t *testing.T) {
			cmd := exec.Command(bv, "--robot-graph", "--graph-root=C", "--graph-depth="+tt.depth)
			cmd.Dir = env
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("graph depth %s failed: %v\n%s", tt.depth, err, out)
			}

			var payload map[string]any
			if err := json.Unmarshal(out, &payload); err != nil {
				t.Fatalf("json decode: %v", err)
			}

			adj := payload["adjacency"].(map[string]any)
			nodes := adj["nodes"].([]any)
			count := len(nodes)

			if count < tt.minNodes || count > tt.maxNodes {
				t.Errorf("depth %s: expected %d-%d nodes, got %d",
					tt.depth, tt.minNodes, tt.maxNodes, count)
			}
		})
	}
}

// TestGraphNavigationFormats tests different output formats work
func TestGraphNavigationFormats(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	beads := `{"id":"A","title":"Root","status":"open","priority":1,"issue_type":"task"}
{"id":"B","title":"Child","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"B","depends_on_id":"A","type":"blocks"}]}`
	writeBeads(t, env, beads)

	formats := []struct {
		format   string
		contains string
	}{
		{"json", "adjacency"},
		{"dot", "digraph"},
		{"mermaid", "graph"},
	}

	for _, tt := range formats {
		t.Run(tt.format, func(t *testing.T) {
			cmd := exec.Command(bv, "--robot-graph", "--graph-format="+tt.format)
			cmd.Dir = env
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("format %s failed: %v\n%s", tt.format, err, out)
			}

			var payload map[string]any
			if err := json.Unmarshal(out, &payload); err != nil {
				t.Fatalf("json decode: %v", err)
			}

			if payload["format"] != tt.format {
				t.Errorf("format=%v; want %s", payload["format"], tt.format)
			}

			// For non-JSON formats, check the graph field
			if tt.format != "json" {
				graph, ok := payload["graph"].(string)
				if !ok || graph == "" {
					t.Error("missing graph output")
				}
				if !strings.Contains(graph, tt.contains) {
					t.Errorf("graph missing expected content %q", tt.contains)
				}
			}
		})
	}
}

// TestGraphNavigationEmptyGraph tests behavior with no issues
func TestGraphNavigationEmptyGraph(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Create empty beads dir
	beadsDir := filepath.Join(env, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte{}, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cmd := exec.Command(bv, "--robot-graph")
	cmd.Dir = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("empty graph failed: %v\n%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	// Check if we have adjacency or nodes count
	nodesCount, _ := payload["nodes"].(float64)
	if int(nodesCount) != 0 {
		// Also check adjacency if present
		if adj, ok := payload["adjacency"].(map[string]any); ok {
			if nodes, ok := adj["nodes"].([]any); ok && len(nodes) != 0 {
				t.Errorf("expected 0 nodes, got %d", len(nodes))
			}
		}
	}
}

// TestGraphNavigationCycleHandling tests graphs with cycles render correctly
func TestGraphNavigationCycleHandling(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Create a cycle: A -> B -> C -> A
	beads := `{"id":"A","title":"A","status":"open","priority":1,"issue_type":"task","dependencies":[{"issue_id":"A","depends_on_id":"B","type":"blocks"}]}
{"id":"B","title":"B","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"B","depends_on_id":"C","type":"blocks"}]}
{"id":"C","title":"C","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"C","depends_on_id":"A","type":"blocks"}]}`
	writeBeads(t, env, beads)

	cmd := exec.Command(bv, "--robot-graph")
	cmd.Dir = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cycle graph failed: %v\n%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	// All nodes should be present despite cycle
	adj := payload["adjacency"].(map[string]any)
	nodes := adj["nodes"].([]any)
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}

	// All edges should be present
	edges := adj["edges"].([]any)
	if len(edges) != 3 {
		t.Errorf("expected 3 edges, got %d", len(edges))
	}
}

// TestGraphNavigationLargeGraph tests performance with many nodes
func TestGraphNavigationLargeGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large graph test in short mode")
	}

	bv := buildBvBinary(t)
	env := t.TempDir()

	// Generate 100 nodes
	var lines []string
	lines = append(lines, `{"id":"root","title":"Root","status":"open","priority":1,"issue_type":"task"}`)
	for i := 0; i < 99; i++ {
		lines = append(lines, fmt.Sprintf(`{"id":"node-%d","title":"Node %d","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"node-%d","depends_on_id":"root","type":"blocks"}]}`, i, i, i))
	}
	writeBeads(t, env, strings.Join(lines, "\n"))

	cmd := exec.Command(bv, "--robot-graph")
	cmd.Dir = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("large graph failed: %v\n%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	adj := payload["adjacency"].(map[string]any)
	nodes := adj["nodes"].([]any)
	if len(nodes) != 100 {
		t.Errorf("expected 100 nodes, got %d", len(nodes))
	}
}

// TestGraphNavigationStatusFiltering tests that status info is included
func TestGraphNavigationStatusFiltering(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	beads := `{"id":"open-1","title":"Open Issue","status":"open","priority":1,"issue_type":"task"}
{"id":"progress-1","title":"In Progress","status":"in_progress","priority":2,"issue_type":"task"}
{"id":"blocked-1","title":"Blocked","status":"blocked","priority":2,"issue_type":"task"}
{"id":"closed-1","title":"Closed","status":"closed","priority":3,"issue_type":"task"}`
	writeBeads(t, env, beads)

	cmd := exec.Command(bv, "--robot-graph")
	cmd.Dir = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("status graph failed: %v\n%s", err, out)
	}

	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	adj := payload["adjacency"].(map[string]any)
	nodes := adj["nodes"].([]any)

	// Check each node has status info
	statuses := make(map[string]bool)
	for _, n := range nodes {
		node := n.(map[string]any)
		if status, ok := node["status"].(string); ok {
			statuses[status] = true
		}
	}

	expected := []string{"open", "in_progress", "blocked", "closed"}
	for _, s := range expected {
		if !statuses[s] {
			t.Errorf("missing status %q in graph nodes", s)
		}
	}
}

// Helper to run robot-graph and return parsed payload
func runRobotGraph(t *testing.T, bv, env string, args ...string) map[string]any {
	t.Helper()
	allArgs := append([]string{"--robot-graph"}, args...)
	cmd := exec.Command(bv, allArgs...)
	cmd.Dir = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("robot-graph %v failed: %v\n%s", args, err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v\n%s", err, out)
	}
	return payload
}
