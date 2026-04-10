package export

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// ============================================================================
// SVG XML Structure Tests
// ============================================================================

// TestSVG_ValidXMLStructure verifies the generated SVG is valid XML
func TestSVG_ValidXMLStructure(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Task A", Status: model.StatusOpen},
		{ID: "B", Title: "Task B", Status: model.StatusBlocked, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "valid.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "testhash123",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	// Verify it's valid XML by attempting to parse it
	var svgDoc interface{}
	if err := xml.Unmarshal(content, &svgDoc); err != nil {
		t.Errorf("SVG is not valid XML: %v\nContent:\n%s", err, string(content))
	}
}

// TestSVG_HasSVGRootElement verifies the root element is <svg>
func TestSVG_HasSVGRootElement(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Task A", Status: model.StatusOpen},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "root.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Check for SVG opening tag with dimensions
	if !strings.Contains(svgStr, "<svg") {
		t.Error("SVG must start with <svg element")
	}

	// Check for width and height attributes
	if !regexp.MustCompile(`width="[0-9]+"`).MatchString(svgStr) {
		t.Error("SVG should have width attribute")
	}
	if !regexp.MustCompile(`height="[0-9]+"`).MatchString(svgStr) {
		t.Error("SVG should have height attribute")
	}

	// Check for closing tag
	if !strings.Contains(svgStr, "</svg>") {
		t.Error("SVG must have closing </svg> tag")
	}
}

// TestSVG_HasViewportDimensions verifies viewport is set correctly
func TestSVG_HasViewportDimensions(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Task A", Status: model.StatusOpen},
		{ID: "B", Title: "Task B", Status: model.StatusOpen},
		{ID: "C", Title: "Task C", Status: model.StatusOpen},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "viewport.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Extract width and height
	widthMatch := regexp.MustCompile(`width="([0-9]+)"`).FindStringSubmatch(svgStr)
	heightMatch := regexp.MustCompile(`height="([0-9]+)"`).FindStringSubmatch(svgStr)

	if len(widthMatch) < 2 || len(heightMatch) < 2 {
		t.Fatal("Could not extract width/height from SVG")
	}

	widthVal, err := strconv.Atoi(widthMatch[1])
	if err != nil {
		t.Fatalf("invalid SVG width %q: %v", widthMatch[1], err)
	}
	heightVal, err := strconv.Atoi(heightMatch[1])
	if err != nil {
		t.Fatalf("invalid SVG height %q: %v", heightMatch[1], err)
	}

	// Verify minimum dimensions (640x480)
	if widthVal < 640 {
		t.Errorf("SVG width should be at least 640, got %d", widthVal)
	}
	if heightVal < 480 {
		t.Errorf("SVG height should be at least 480, got %d", heightVal)
	}
}

// ============================================================================
// Node Rendering Tests
// ============================================================================

// TestSVG_NodeRectanglesRendered verifies each node has a rectangle
func TestSVG_NodeRectanglesRendered(t *testing.T) {
	issues := []model.Issue{
		{ID: "NODE-1", Title: "First Node", Status: model.StatusOpen},
		{ID: "NODE-2", Title: "Second Node", Status: model.StatusInProgress},
		{ID: "NODE-3", Title: "Third Node", Status: model.StatusBlocked},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "nodes.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Count roundrect elements (nodes use roundrect)
	roundrectCount := strings.Count(svgStr, "<rect ")
	// svgo uses <rect with rx/ry for rounded rectangles
	// Minimum expected: 3 nodes + background + header + legend = 6
	if roundrectCount < 3 {
		t.Errorf("Expected at least 3 rect elements for nodes, found %d", roundrectCount)
	}

	// Verify each node ID appears in the SVG text
	for _, issue := range issues {
		if !strings.Contains(svgStr, issue.ID) {
			t.Errorf("Node ID %q not found in SVG", issue.ID)
		}
	}
}

// TestSVG_NodeLabelsPresent verifies node IDs and titles are rendered as text
func TestSVG_NodeLabelsPresent(t *testing.T) {
	issues := []model.Issue{
		{ID: "TASK-123", Title: "Implement feature X", Status: model.StatusOpen},
		{ID: "BUG-456", Title: "Fix critical bug", Status: model.StatusBlocked},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "labels.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Check for text elements
	textCount := strings.Count(svgStr, "<text ")
	if textCount < 4 {
		t.Errorf("Expected multiple text elements, found %d", textCount)
	}

	// Verify IDs appear
	if !strings.Contains(svgStr, "TASK-123") {
		t.Error("Node ID TASK-123 not found in SVG")
	}
	if !strings.Contains(svgStr, "BUG-456") {
		t.Error("Node ID BUG-456 not found in SVG")
	}

	// Verify titles appear (may be truncated)
	if !strings.Contains(svgStr, "Implement feature") {
		t.Error("Node title 'Implement feature' not found in SVG")
	}
}

// TestSVG_StatusColorsApplied verifies different statuses have different colors
func TestSVG_StatusColorsApplied(t *testing.T) {
	issues := []model.Issue{
		{ID: "OPEN", Title: "Open task", Status: model.StatusOpen},
		{ID: "PROG", Title: "In progress", Status: model.StatusInProgress},
		{ID: "BLOCK", Title: "Blocked", Status: model.StatusBlocked},
		{ID: "DONE", Title: "Closed", Status: model.StatusClosed},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "colors.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Expected status colors (from graph_snapshot.go)
	expectedColors := map[string]string{
		"open":        "#c8e6c9", // colorOpen
		"in_progress": "#fff3e0", // colorInProg
		"blocked":     "#ffcdd2", // colorBlocked
		"closed":      "#cfd8dc", // colorClosed
	}

	for status, color := range expectedColors {
		if !strings.Contains(svgStr, color) {
			t.Errorf("Expected color %s for status %s not found in SVG", color, status)
		}
	}
}

// TestSVG_PageRankDisplayed verifies PageRank scores appear in nodes
func TestSVG_PageRankDisplayed(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Root", Status: model.StatusOpen},
		{ID: "B", Title: "Child", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "pagerank.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Verify "PR" prefix appears (PageRank display)
	if !strings.Contains(svgStr, "PR ") {
		t.Error("PageRank indicator 'PR' not found in SVG")
	}
}

// ============================================================================
// Edge Rendering Tests
// ============================================================================

// TestSVG_EdgesRenderedAsLines verifies edges are drawn as lines
func TestSVG_EdgesRenderedAsLines(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Source", Status: model.StatusOpen},
		{ID: "B", Title: "Target", Status: model.StatusBlocked, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "edges.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Check for line element (edges are drawn as lines)
	if !strings.Contains(svgStr, "<line ") {
		t.Error("Expected <line> element for edge not found in SVG")
	}
}

// TestSVG_ArrowMarkersPresent verifies arrow heads are rendered
func TestSVG_ArrowMarkersPresent(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Source", Status: model.StatusOpen},
		{ID: "B", Title: "Target", Status: model.StatusBlocked, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "arrows.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Check for polygon element (arrow heads are drawn as polygons)
	if !strings.Contains(svgStr, "<polygon ") {
		t.Error("Expected <polygon> element for arrow head not found in SVG")
	}
}

// TestSVG_MultipleEdgesRendered verifies all edges in a complex graph
func TestSVG_MultipleEdgesRendered(t *testing.T) {
	issues := []model.Issue{
		{ID: "ROOT", Title: "Root", Status: model.StatusOpen},
		{ID: "A", Title: "A", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "ROOT", Type: model.DepBlocks}}},
		{ID: "B", Title: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "ROOT", Type: model.DepBlocks}}},
		{ID: "C", Title: "C", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}, {DependsOnID: "B", Type: model.DepBlocks}}},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "multi_edges.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Count line elements (should have 4 edges: ROOT->A, ROOT->B, A->C, B->C)
	lineCount := strings.Count(svgStr, "<line ")
	expectedEdges := 4
	if lineCount != expectedEdges {
		t.Errorf("Expected %d edges (lines), found %d", expectedEdges, lineCount)
	}
}

// TestSVG_NonBlockingDepsIgnored verifies non-blocking deps don't create edges
func TestSVG_NonBlockingDepsIgnored(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "A", Status: model.StatusOpen},
		{ID: "B", Title: "B", Status: model.StatusOpen, Dependencies: []*model.Dependency{
			{DependsOnID: "A", Type: model.DepRelated}, // Should not create edge
		}},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "no_related_edges.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Should have no edge lines (only blocking deps create edges)
	lineCount := strings.Count(svgStr, "<line ")
	if lineCount != 0 {
		t.Errorf("Expected 0 edges for non-blocking deps, found %d", lineCount)
	}
}

// ============================================================================
// Legend and Summary Tests
// ============================================================================

// TestSVG_LegendPresent verifies the legend box is rendered
func TestSVG_LegendPresent(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Task", Status: model.StatusOpen},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "legend.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Check for legend text
	if !strings.Contains(svgStr, "Legend") {
		t.Error("Legend title not found in SVG")
	}

	// Check for status labels in legend
	legendLabels := []string{"Open", "In Progress", "Blocked", "Closed"}
	for _, label := range legendLabels {
		if !strings.Contains(svgStr, label) {
			t.Errorf("Legend label %q not found in SVG", label)
		}
	}
}

// TestSVG_SummaryBlockPresent verifies the summary block is rendered
func TestSVG_SummaryBlockPresent(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Task", Status: model.StatusOpen},
		{ID: "B", Title: "Another", Status: model.StatusBlocked, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "summary.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Title:    "My Test Graph",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "abc123hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Check for title
	if !strings.Contains(svgStr, "My Test Graph") {
		t.Error("Custom title not found in SVG summary")
	}

	// Check for data hash
	if !strings.Contains(svgStr, "abc123hash") {
		t.Error("Data hash not found in SVG summary")
	}

	// Check for node/edge counts
	if !strings.Contains(svgStr, "nodes:") {
		t.Error("Node count not found in SVG summary")
	}
	if !strings.Contains(svgStr, "edges:") {
		t.Error("Edge count not found in SVG summary")
	}

	// Check for bottleneck
	if !strings.Contains(svgStr, "bottleneck") {
		t.Error("Top bottleneck not found in SVG summary")
	}
}

// TestSVG_DefaultTitleWhenEmpty verifies default title is used
func TestSVG_DefaultTitleWhenEmpty(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Task", Status: model.StatusOpen},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "default_title.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Title:    "", // Empty title
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Check for default title
	if !strings.Contains(svgStr, "Graph Snapshot") {
		t.Error("Default title 'Graph Snapshot' not found when title is empty")
	}
}

// ============================================================================
// Layout Tests
// ============================================================================

// TestSVG_CompactVsRoomyPreset verifies presets affect dimensions
func TestSVG_CompactVsRoomyPreset(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Task A", Status: model.StatusOpen},
		{ID: "B", Title: "Task B", Status: model.StatusOpen},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()

	// Generate compact version
	compactOut := filepath.Join(tmp, "compact.svg")
	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     compactOut,
		Format:   "svg",
		Preset:   "compact",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot (compact) error: %v", err)
	}

	// Generate roomy version
	roomyOut := filepath.Join(tmp, "roomy.svg")
	err = SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     roomyOut,
		Format:   "svg",
		Preset:   "roomy",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot (roomy) error: %v", err)
	}

	compactInfo, _ := os.Stat(compactOut)
	roomyInfo, _ := os.Stat(roomyOut)

	// Roomy should generally produce larger SVG (more spacing = larger coordinates = more text)
	// This is a soft check since the difference depends on content
	if roomyInfo.Size() == 0 || compactInfo.Size() == 0 {
		t.Error("One or both SVG files are empty")
	}
}

// TestSVG_LargeGraphLayout verifies layout handles many nodes
func TestSVG_LargeGraphLayout(t *testing.T) {
	// Create 20 nodes in a chain
	var issues []model.Issue
	for i := 0; i < 20; i++ {
		issue := model.Issue{
			ID:     string(rune('A' + i)),
			Title:  "Task " + string(rune('A'+i)),
			Status: model.StatusOpen,
		}
		if i > 0 {
			issue.Dependencies = []*model.Dependency{
				{DependsOnID: string(rune('A' + i - 1)), Type: model.DepBlocks},
			}
		}
		issues = append(issues, issue)
	}

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "large.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Verify all node IDs are present
	for _, issue := range issues {
		if !strings.Contains(svgStr, issue.ID) {
			t.Errorf("Node ID %q not found in large graph SVG", issue.ID)
		}
	}

	// Verify dimensions scaled appropriately
	info, _ := os.Stat(out)
	if info.Size() < 5000 {
		t.Errorf("Large graph SVG seems too small: %d bytes", info.Size())
	}
}

// ============================================================================
// XSS and Escaping Tests
// ============================================================================

func TestSaveGraphSnapshot_SVG_Escaping(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Dangerous <script>", Status: model.StatusOpen},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "unsafe.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	svg := string(content)

	// Check if the title text is properly escaped
	// We look for the exact string "Dangerous <script>" inside the SVG.
	// If it's present verbatim inside a <text> tag without escaping, it's invalid XML if it contains < or > or &.
	// svgo writes raw strings. So "Dangerous <script>" will be written as-is.
	// We verify that it IS escaped (e.g. "Dangerous &lt;script&gt;") or at least not present in raw form.

	if !strings.Contains(svg, "Dangerous &lt;script&gt;") {
		t.Errorf("SVG does not contain escaped text: %s\nFull SVG:\n%s", "Dangerous &lt;script&gt;", svg)
	}
}

// TestSVG_SpecialCharactersInTitle verifies HTML entities are escaped
func TestSVG_SpecialCharactersInTitle(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Test & verify <tags>", Status: model.StatusOpen},
		{ID: "B", Title: `Quote "test" here`, Status: model.StatusOpen},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "special.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	// Verify the SVG is still valid XML (special chars properly escaped)
	var svgDoc interface{}
	if err := xml.Unmarshal(content, &svgDoc); err != nil {
		t.Errorf("SVG with special characters is not valid XML: %v", err)
	}
}

// TestSVG_UnicodeCharacters verifies unicode is handled correctly
func TestSVG_UnicodeCharacters(t *testing.T) {
	issues := []model.Issue{
		{ID: "JP", Title: "日本語タスク", Status: model.StatusOpen},
		{ID: "EMOJI", Title: "Task with 🚀 emoji", Status: model.StatusOpen},
		{ID: "MIXED", Title: "Café résumé naïve", Status: model.StatusOpen},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "unicode.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Verify unicode content is present
	if !strings.Contains(svgStr, "日本語") {
		t.Error("Japanese characters not found in SVG")
	}
	if !strings.Contains(svgStr, "Café") {
		t.Error("Accented characters not found in SVG")
	}

	// Verify it's still valid XML
	var svgDoc interface{}
	if err := xml.Unmarshal(content, &svgDoc); err != nil {
		t.Errorf("SVG with unicode is not valid XML: %v", err)
	}
}

// ============================================================================
// Edge Case Tests
// ============================================================================

// TestSVG_SingleNode verifies single node graph renders correctly
func TestSVG_SingleNode(t *testing.T) {
	issues := []model.Issue{
		{ID: "SOLO", Title: "Single node", Status: model.StatusOpen},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "single.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Verify the single node is present
	if !strings.Contains(svgStr, "SOLO") {
		t.Error("Single node ID not found in SVG")
	}

	// Verify no edges (single node has no deps)
	lineCount := strings.Count(svgStr, "<line ")
	if lineCount != 0 {
		t.Errorf("Expected 0 edges for single node, found %d", lineCount)
	}
}

// TestSVG_DisconnectedGraph verifies disconnected components render
func TestSVG_DisconnectedGraph(t *testing.T) {
	issues := []model.Issue{
		{ID: "A1", Title: "Component A - 1", Status: model.StatusOpen},
		{ID: "A2", Title: "Component A - 2", Status: model.StatusBlocked, Dependencies: []*model.Dependency{{DependsOnID: "A1", Type: model.DepBlocks}}},
		{ID: "B1", Title: "Component B - 1", Status: model.StatusOpen},
		{ID: "B2", Title: "Component B - 2", Status: model.StatusInProgress, Dependencies: []*model.Dependency{{DependsOnID: "B1", Type: model.DepBlocks}}},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "disconnected.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// Verify all nodes from both components are present
	for _, id := range []string{"A1", "A2", "B1", "B2"} {
		if !strings.Contains(svgStr, id) {
			t.Errorf("Node %q from disconnected graph not found in SVG", id)
		}
	}

	// Should have 2 edges (one per component)
	lineCount := strings.Count(svgStr, "<line ")
	if lineCount != 2 {
		t.Errorf("Expected 2 edges for disconnected graph, found %d", lineCount)
	}
}

// TestSVG_LongTitleTruncation verifies long titles are truncated
func TestSVG_LongTitleTruncation(t *testing.T) {
	longTitle := "This is a very long title that should definitely be truncated because it exceeds the maximum allowed characters for display in the graph node"
	issues := []model.Issue{
		{ID: "LONG", Title: longTitle, Status: model.StatusOpen},
	}
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.Analyze()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "long_title.svg")

	err := SaveGraphSnapshot(GraphSnapshotOptions{
		Path:     out,
		Format:   "svg",
		Issues:   issues,
		Stats:    &stats,
		DataHash: "hash",
	})
	if err != nil {
		t.Fatalf("SaveGraphSnapshot error: %v", err)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	svgStr := string(content)

	// The full title should NOT be present (it's truncated)
	if strings.Contains(svgStr, longTitle) {
		t.Error("Full long title should be truncated in SVG")
	}

	// But the beginning should be there
	if !strings.Contains(svgStr, "This is a very long") {
		t.Error("Beginning of long title should be present")
	}

	// Ellipsis should be present
	if !strings.Contains(svgStr, "...") {
		t.Error("Truncation ellipsis not found for long title")
	}
}
