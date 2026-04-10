package export

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestExportGraph_JSON(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "First Issue", Status: model.StatusOpen, Priority: 1},
		{ID: "bv-2", Title: "Second Issue", Status: model.StatusInProgress, Priority: 2,
			Dependencies: []*model.Dependency{
				{IssueID: "bv-2", DependsOnID: "bv-1", Type: model.DepBlocks},
			},
		},
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	config := GraphExportConfig{
		Format:   GraphFormatJSON,
		DataHash: "test-hash",
	}

	result, err := ExportGraph(issues, &stats, config)
	if err != nil {
		t.Fatalf("ExportGraph failed: %v", err)
	}

	if result.Format != "json" {
		t.Errorf("Expected format 'json', got %s", result.Format)
	}

	if result.Nodes != 2 {
		t.Errorf("Expected 2 nodes, got %d", result.Nodes)
	}

	if result.Edges != 1 {
		t.Errorf("Expected 1 edge, got %d", result.Edges)
	}

	if result.Adjacency == nil {
		t.Fatal("Expected adjacency to be non-nil for JSON format")
	}

	if len(result.Adjacency.Nodes) != 2 {
		t.Errorf("Expected 2 adjacency nodes, got %d", len(result.Adjacency.Nodes))
	}

	if len(result.Adjacency.Edges) != 1 {
		t.Errorf("Expected 1 adjacency edge, got %d", len(result.Adjacency.Edges))
	}
}

func TestExportGraph_DOT(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "First Issue", Status: model.StatusOpen, Priority: 1},
		{ID: "bv-2", Title: "Second Issue", Status: model.StatusClosed, Priority: 2,
			Dependencies: []*model.Dependency{
				{IssueID: "bv-2", DependsOnID: "bv-1", Type: model.DepBlocks},
			},
		},
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	config := GraphExportConfig{
		Format: GraphFormatDOT,
	}

	result, err := ExportGraph(issues, &stats, config)
	if err != nil {
		t.Fatalf("ExportGraph failed: %v", err)
	}

	if result.Format != "dot" {
		t.Errorf("Expected format 'dot', got %s", result.Format)
	}

	if result.Graph == "" {
		t.Error("Expected non-empty graph string for DOT format")
	}

	// Check DOT structure
	if !strings.Contains(result.Graph, "digraph G") {
		t.Error("DOT output should contain 'digraph G'")
	}

	if !strings.Contains(result.Graph, "bv-1") {
		t.Error("DOT output should contain node bv-1")
	}

	if !strings.Contains(result.Graph, "bv-2") {
		t.Error("DOT output should contain node bv-2")
	}

	if !strings.Contains(result.Graph, "->") {
		t.Error("DOT output should contain edge arrow")
	}

	// Check status colors
	if !strings.Contains(result.Graph, "#C8E6C9") { // open color
		t.Error("DOT output should contain open status color")
	}

	if !strings.Contains(result.Graph, "#CFD8DC") { // closed color
		t.Error("DOT output should contain closed status color")
	}
}

func TestExportGraph_DOT_EscapesBackslashesInIDs(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv\\1", Title: "First Issue", Status: model.StatusOpen, Priority: 1},
		{ID: "bv\\2", Title: "Second Issue", Status: model.StatusClosed, Priority: 2,
			Dependencies: []*model.Dependency{
				{IssueID: "bv\\2", DependsOnID: "bv\\1", Type: model.DepBlocks},
			},
		},
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	config := GraphExportConfig{
		Format: GraphFormatDOT,
	}

	result, err := ExportGraph(issues, &stats, config)
	if err != nil {
		t.Fatalf("ExportGraph failed: %v", err)
	}

	if !strings.Contains(result.Graph, "\"bv\\\\1\"") {
		t.Error("DOT output should escape backslashes in node ID bv\\1")
	}

	if !strings.Contains(result.Graph, "\"bv\\\\2\"") {
		t.Error("DOT output should escape backslashes in node ID bv\\2")
	}

	if !strings.Contains(result.Graph, "\"bv\\\\2\" -> \"bv\\\\1\"") {
		t.Error("DOT output should escape backslashes in edge IDs")
	}
}

func TestExportGraph_DOT_TruncationUTF8(t *testing.T) {
	title := strings.Repeat("å", 20) // 40 bytes, 20 runes
	issues := []model.Issue{
		{ID: "bv-utf8", Title: title, Status: model.StatusOpen, Priority: 1},
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	config := GraphExportConfig{Format: GraphFormatDOT}
	result, err := ExportGraph(issues, &stats, config)
	if err != nil {
		t.Fatalf("ExportGraph failed: %v", err)
	}

	if !utf8.ValidString(result.Graph) {
		t.Fatal("DOT output should be valid UTF-8")
	}
}

func TestExportGraph_DOT_EscapesNewlinesInLabels(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Hello\nWorld", Status: model.StatusOpen, Priority: 1},
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	config := GraphExportConfig{Format: GraphFormatDOT}
	result, err := ExportGraph(issues, &stats, config)
	if err != nil {
		t.Fatalf("ExportGraph failed: %v", err)
	}

	if strings.Contains(result.Graph, "Hello\nWorld") {
		t.Error("DOT output should not contain raw newlines inside labels")
	}
	if !strings.Contains(result.Graph, "Hello World") {
		t.Error("DOT output should replace newlines with spaces in labels")
	}
}

func TestExportGraph_Mermaid(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "First Issue", Status: model.StatusOpen, Priority: 1},
		{ID: "bv-2", Title: "Second Issue", Status: model.StatusBlocked, Priority: 2,
			Dependencies: []*model.Dependency{
				{IssueID: "bv-2", DependsOnID: "bv-1", Type: model.DepBlocks},
			},
		},
		{ID: "tombstone-1", Title: "Removed Issue", Status: model.StatusTombstone, Priority: 3},
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	config := GraphExportConfig{
		Format: GraphFormatMermaid,
	}

	result, err := ExportGraph(issues, &stats, config)
	if err != nil {
		t.Fatalf("ExportGraph failed: %v", err)
	}

	if result.Format != "mermaid" {
		t.Errorf("Expected format 'mermaid', got %s", result.Format)
	}

	if result.Graph == "" {
		t.Error("Expected non-empty graph string for Mermaid format")
	}

	// Check Mermaid structure
	if !strings.Contains(result.Graph, "graph TD") {
		t.Error("Mermaid output should contain 'graph TD'")
	}

	if !strings.Contains(result.Graph, "classDef open") {
		t.Error("Mermaid output should contain open class definition")
	}

	if !strings.Contains(result.Graph, "classDef blocked") {
		t.Error("Mermaid output should contain blocked class definition")
	}

	// Check for bold edge (blocks)
	if !strings.Contains(result.Graph, "==>") {
		t.Error("Mermaid output should contain bold edge for blocks dependency")
	}

	// Tombstone nodes should be styled as closed-like.
	if !strings.Contains(result.Graph, "class tombstone-1 closed") {
		t.Error("Mermaid output should style tombstone nodes as closed")
	}
}

func TestExportGraph_DOT_TombstoneUsesClosedColor(t *testing.T) {
	issues := []model.Issue{
		{ID: "tombstone-1", Title: "Removed Issue", Status: model.StatusTombstone, Priority: 3},
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	config := GraphExportConfig{Format: GraphFormatDOT}
	result, err := ExportGraph(issues, &stats, config)
	if err != nil {
		t.Fatalf("ExportGraph failed: %v", err)
	}

	if !strings.Contains(result.Graph, "fillcolor=\"#CFD8DC\"") {
		t.Error("DOT output should style tombstone nodes with the closed color")
	}
}

func TestExportGraph_LabelFilter(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "API Issue", Status: model.StatusOpen, Labels: []string{"api"}},
		{ID: "bv-2", Title: "UI Issue", Status: model.StatusOpen, Labels: []string{"ui"}},
		{ID: "bv-3", Title: "Another API Issue", Status: model.StatusOpen, Labels: []string{"api", "backend"}},
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	config := GraphExportConfig{
		Format: GraphFormatJSON,
		Label:  "api",
	}

	result, err := ExportGraph(issues, &stats, config)
	if err != nil {
		t.Fatalf("ExportGraph failed: %v", err)
	}

	if result.Nodes != 2 {
		t.Errorf("Expected 2 nodes (api labeled only), got %d", result.Nodes)
	}

	if result.FiltersApplied["label"] != "api" {
		t.Errorf("Expected label filter 'api', got %s", result.FiltersApplied["label"])
	}
}

func TestExportGraph_SubgraphRoot(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Root Issue", Status: model.StatusOpen},
		{ID: "bv-2", Title: "Child Issue", Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{IssueID: "bv-2", DependsOnID: "bv-1", Type: model.DepBlocks},
			},
		},
		{ID: "bv-3", Title: "Grandchild Issue", Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{IssueID: "bv-3", DependsOnID: "bv-2", Type: model.DepBlocks},
			},
		},
		{ID: "bv-4", Title: "Unrelated Issue", Status: model.StatusOpen},
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	config := GraphExportConfig{
		Format: GraphFormatJSON,
		Root:   "bv-2",
	}

	result, err := ExportGraph(issues, &stats, config)
	if err != nil {
		t.Fatalf("ExportGraph failed: %v", err)
	}

	// bv-2 depends on bv-1, so subgraph from bv-2 should include bv-1 and bv-2
	if result.Nodes != 2 {
		t.Errorf("Expected 2 nodes in subgraph from bv-2, got %d", result.Nodes)
	}

	if result.FiltersApplied["root"] != "bv-2" {
		t.Errorf("Expected root filter 'bv-2', got %s", result.FiltersApplied["root"])
	}
}

func TestExportGraph_EmptyResult(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Issue", Status: model.StatusOpen, Labels: []string{"api"}},
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	config := GraphExportConfig{
		Format: GraphFormatJSON,
		Label:  "nonexistent",
	}

	result, err := ExportGraph(issues, &stats, config)
	if err != nil {
		t.Fatalf("ExportGraph failed: %v", err)
	}

	if result.Nodes != 0 {
		t.Errorf("Expected 0 nodes for nonexistent label, got %d", result.Nodes)
	}
}

func TestGraphExportResult_JSON(t *testing.T) {
	result := &GraphExportResult{
		Format: "json",
		Nodes:  5,
		Edges:  3,
		Explanation: GraphExplanation{
			What:      "Test graph",
			WhenToUse: "Testing",
		},
	}

	data, err := result.JSON()
	if err != nil {
		t.Fatalf("JSON() failed: %v", err)
	}

	// Parse back
	var parsed GraphExportResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if parsed.Nodes != 5 {
		t.Errorf("Expected 5 nodes, got %d", parsed.Nodes)
	}
}

func TestExportGraph_DeterministicOutput(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "bv-3", Title: "Third", Status: model.StatusOpen, CreatedAt: now},
		{ID: "bv-1", Title: "First", Status: model.StatusOpen, CreatedAt: now},
		{ID: "bv-2", Title: "Second", Status: model.StatusOpen, CreatedAt: now,
			Dependencies: []*model.Dependency{
				{IssueID: "bv-2", DependsOnID: "bv-1", Type: model.DepBlocks},
				{IssueID: "bv-2", DependsOnID: "bv-3", Type: model.DepRelated},
			},
		},
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	config := GraphExportConfig{
		Format: GraphFormatDOT,
	}

	// Run twice and compare
	result1, _ := ExportGraph(issues, &stats, config)
	result2, _ := ExportGraph(issues, &stats, config)

	if result1.Graph != result2.Graph {
		t.Error("DOT output should be deterministic across calls")
	}
}
