package ui

import (
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func TestParseCapabilities(t *testing.T) {
	t.Run("export label", func(t *testing.T) {
		issue := model.Issue{
			ID:         "bt-001",
			Labels:     []string{"export:graph-analysis"},
			SourceRepo: "beadstui",
		}
		caps := parseCapabilities(issue)
		if len(caps) != 1 {
			t.Fatalf("expected 1 capability, got %d", len(caps))
		}
		if caps[0].Type != "export" {
			t.Errorf("expected type export, got %s", caps[0].Type)
		}
		if caps[0].Capability != "graph-analysis" {
			t.Errorf("expected capability graph-analysis, got %s", caps[0].Capability)
		}
		if caps[0].Project != "beadstui" {
			t.Errorf("expected project beadstui, got %s", caps[0].Project)
		}
	})

	t.Run("provides label", func(t *testing.T) {
		issue := model.Issue{
			ID:         "bd-002",
			Labels:     []string{"provides:issue-tracking"},
			SourceRepo: "beads",
		}
		caps := parseCapabilities(issue)
		if len(caps) != 1 {
			t.Fatalf("expected 1 capability, got %d", len(caps))
		}
		if caps[0].Type != "provides" {
			t.Errorf("expected type provides, got %s", caps[0].Type)
		}
		if caps[0].Capability != "issue-tracking" {
			t.Errorf("expected capability issue-tracking, got %s", caps[0].Capability)
		}
	})

	t.Run("external label", func(t *testing.T) {
		issue := model.Issue{
			ID:         "bt-003",
			Labels:     []string{"external:beads:issue-tracking"},
			SourceRepo: "beadstui",
		}
		caps := parseCapabilities(issue)
		if len(caps) != 1 {
			t.Fatalf("expected 1 capability, got %d", len(caps))
		}
		if caps[0].Type != "external" {
			t.Errorf("expected type external, got %s", caps[0].Type)
		}
		if caps[0].Capability != "issue-tracking" {
			t.Errorf("expected capability issue-tracking, got %s", caps[0].Capability)
		}
		if caps[0].TargetProject != "beads" {
			t.Errorf("expected target project beads, got %s", caps[0].TargetProject)
		}
	})

	t.Run("mixed labels", func(t *testing.T) {
		issue := model.Issue{
			ID:         "bt-004",
			Labels:     []string{"export:tui", "priority:high", "external:beads:cli", "status:open"},
			SourceRepo: "beadstui",
		}
		caps := parseCapabilities(issue)
		if len(caps) != 2 {
			t.Fatalf("expected 2 capabilities, got %d", len(caps))
		}
	})

	t.Run("no capability labels", func(t *testing.T) {
		issue := model.Issue{
			ID:     "bt-005",
			Labels: []string{"priority:high", "status:open"},
		}
		caps := parseCapabilities(issue)
		if len(caps) != 0 {
			t.Errorf("expected 0 capabilities, got %d", len(caps))
		}
	})

	t.Run("malformed external label", func(t *testing.T) {
		issue := model.Issue{
			ID:     "bt-006",
			Labels: []string{"external:nocolon"},
		}
		caps := parseCapabilities(issue)
		if len(caps) != 0 {
			t.Errorf("expected 0 capabilities for malformed external, got %d", len(caps))
		}
	})

	t.Run("empty labels", func(t *testing.T) {
		issue := model.Issue{
			ID:     "bt-007",
			Labels: nil,
		}
		caps := parseCapabilities(issue)
		if len(caps) != 0 {
			t.Errorf("expected 0 capabilities for nil labels, got %d", len(caps))
		}
	})

	t.Run("label with colon at end", func(t *testing.T) {
		issue := model.Issue{
			ID:     "bt-008",
			Labels: []string{"export:"},
		}
		caps := parseCapabilities(issue)
		if len(caps) != 0 {
			t.Errorf("expected 0 capabilities for trailing colon, got %d", len(caps))
		}
	})
}

func TestAggregateCapabilities(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-001", Labels: []string{"export:tui"}, SourceRepo: "beadstui"},
		{ID: "bd-001", Labels: []string{"export:cli", "export:issue-tracking"}, SourceRepo: "beads"},
		{ID: "bt-002", Labels: []string{"external:beads:cli"}, SourceRepo: "beadstui"},
		{ID: "mkt-001", Labels: []string{"external:beads:issue-tracking", "external:beadstui:tui"}, SourceRepo: "marketplace"},
	}

	exports, consumes, edges := aggregateCapabilities(issues)

	// Check exports
	if len(exports["beadstui"]) != 1 {
		t.Errorf("beadstui should have 1 export, got %d", len(exports["beadstui"]))
	}
	if len(exports["beads"]) != 2 {
		t.Errorf("beads should have 2 exports, got %d", len(exports["beads"]))
	}

	// Check consumes
	if len(consumes["beadstui"]) != 1 {
		t.Errorf("beadstui should have 1 consume, got %d", len(consumes["beadstui"]))
	}
	if len(consumes["marketplace"]) != 2 {
		t.Errorf("marketplace should have 2 consumes, got %d", len(consumes["marketplace"]))
	}

	// Check edges
	if len(edges) != 3 {
		t.Errorf("expected 3 edges, got %d", len(edges))
	}

	// All edges should be resolved (matching exports exist)
	for _, edge := range edges {
		if !edge.Resolved {
			t.Errorf("edge %s->%s (%s) should be resolved", edge.FromProject, edge.ToProject, edge.Capability)
		}
	}
}

func TestAggregateCapabilitiesUnresolved(t *testing.T) {
	issues := []model.Issue{
		{ID: "bt-001", Labels: []string{"external:beads:missing-feature"}, SourceRepo: "beadstui"},
	}

	_, _, edges := aggregateCapabilities(issues)

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Resolved {
		t.Error("edge should be unresolved (no matching export)")
	}
	if edges[0].FromProject != "beadstui" {
		t.Errorf("expected FromProject beadstui, got %s", edges[0].FromProject)
	}
	if edges[0].ToProject != "beads" {
		t.Errorf("expected ToProject beads, got %s", edges[0].ToProject)
	}
	if edges[0].Capability != "missing-feature" {
		t.Errorf("expected Capability missing-feature, got %s", edges[0].Capability)
	}
}

func TestAggregateCapabilitiesEmpty(t *testing.T) {
	exports, consumes, edges := aggregateCapabilities(nil)

	if len(exports) != 0 {
		t.Errorf("expected 0 exports, got %d", len(exports))
	}
	if len(consumes) != 0 {
		t.Errorf("expected 0 consumes, got %d", len(consumes))
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}
