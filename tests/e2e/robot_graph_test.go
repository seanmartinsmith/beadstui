package main_test

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestRobotGraph_JSONAndFilters(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	// Chain A -> B -> C (B depends on A, C depends on B).
	writeBeads(t, env, `{"id":"A","title":"Root","status":"open","priority":1,"issue_type":"task"}
{"id":"B","title":"Mid","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"B","depends_on_id":"A","type":"blocks"}]}
{"id":"C","title":"Leaf","status":"open","priority":3,"issue_type":"task","dependencies":[{"issue_id":"C","depends_on_id":"B","type":"blocks"}]}`)

	run := func(args ...string) map[string]any {
		t.Helper()
		cmd := exec.Command(bv, args...)
		cmd.Dir = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
		var payload map[string]any
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("json decode: %v\nout=%s", err, out)
		}
		return payload
	}

	// Default format is JSON adjacency list.
	payload := run("--robot-graph")
	if payload["data_hash"] == "" {
		t.Fatalf("missing data_hash")
	}
	if payload["format"] != "json" {
		t.Fatalf("format=%v; want json", payload["format"])
	}

	// Root+depth filter: C depth=1 should include C and B, but not A.
	filtered := run("--robot-graph", "--graph-root=C", "--graph-depth=1")
	if filtered["format"] != "json" {
		t.Fatalf("format=%v; want json", filtered["format"])
	}
	if nodes, ok := filtered["nodes"].(float64); !ok || int(nodes) != 2 {
		t.Fatalf("nodes=%v; want 2", filtered["nodes"])
	}
	adj, ok := filtered["adjacency"].(map[string]any)
	if !ok {
		t.Fatalf("missing adjacency")
	}
	nodeArr, ok := adj["nodes"].([]any)
	if !ok {
		t.Fatalf("adjacency.nodes not array: %T", adj["nodes"])
	}
	ids := make(map[string]bool)
	for _, n := range nodeArr {
		obj, _ := n.(map[string]any)
		if id, _ := obj["id"].(string); id != "" {
			ids[id] = true
		}
	}
	if !ids["C"] || !ids["B"] || ids["A"] {
		t.Fatalf("unexpected node set: %v", ids)
	}
}

func TestRobotGraph_DOTAndMermaid(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	writeBeads(t, env, `{"id":"A","title":"Root","status":"open","priority":1,"issue_type":"task"}
{"id":"B","title":"Mid","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"B","depends_on_id":"A","type":"blocks"}]}`)

	for _, tt := range []struct {
		name        string
		graphFormat string
		wantFormat  string
		wantSubstr  string
	}{
		{name: "dot", graphFormat: "dot", wantFormat: "dot", wantSubstr: "digraph"},
		{name: "mermaid", graphFormat: "mermaid", wantFormat: "mermaid", wantSubstr: "graph"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(bv, "--robot-graph", "--graph-format="+tt.graphFormat)
			cmd.Dir = env
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("run failed: %v\n%s", err, out)
			}
			var payload struct {
				Format string `json:"format"`
				Graph  string `json:"graph"`
			}
			if err := json.Unmarshal(out, &payload); err != nil {
				t.Fatalf("json decode: %v\nout=%s", err, out)
			}
			if payload.Format != tt.wantFormat {
				t.Fatalf("format=%q; want %q", payload.Format, tt.wantFormat)
			}
			if payload.Graph == "" {
				t.Fatalf("graph missing")
			}
			if !strings.Contains(payload.Graph, tt.wantSubstr) {
				t.Fatalf("graph missing %q:\n%s", tt.wantSubstr, payload.Graph)
			}
		})
	}
}
