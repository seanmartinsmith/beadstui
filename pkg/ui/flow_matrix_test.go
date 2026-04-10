package ui_test

import (
	"strings"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/ui"
)

// =============================================================================
// FlowMatrixView Tests (Legacy Summary Function)
// =============================================================================
// Note: The primary UI is now FlowMatrixModel (interactive). FlowMatrixView
// is kept for backward compatibility and returns a simple summary.

func TestFlowMatrixViewEmptyLabels(t *testing.T) {
	flow := analysis.CrossLabelFlow{
		Labels:     []string{},
		FlowMatrix: [][]int{},
	}

	result := ui.FlowMatrixView(flow, 80)
	expected := "No cross-label dependencies found"
	if result != expected {
		t.Errorf("FlowMatrixView() = %q, want %q", result, expected)
	}
}

func TestFlowMatrixViewNilLabels(t *testing.T) {
	flow := analysis.CrossLabelFlow{
		Labels:     nil,
		FlowMatrix: nil,
	}

	result := ui.FlowMatrixView(flow, 80)
	expected := "No cross-label dependencies found"
	if result != expected {
		t.Errorf("FlowMatrixView() with nil labels = %q, want %q", result, expected)
	}
}

func TestFlowMatrixViewSingleLabel(t *testing.T) {
	flow := analysis.CrossLabelFlow{
		Labels:     []string{"bug"},
		FlowMatrix: [][]int{{0}},
	}

	result := ui.FlowMatrixView(flow, 80)

	// Should contain header
	if !strings.Contains(result, "DEPENDENCY FLOW SUMMARY") {
		t.Errorf("FlowMatrixView() should contain header, got: %q", result)
	}

	// Should contain labels count
	if !strings.Contains(result, "Labels: 1") {
		t.Errorf("FlowMatrixView() should contain 'Labels: 1', got: %q", result)
	}
}

func TestFlowMatrixViewMultipleLabels(t *testing.T) {
	flow := analysis.CrossLabelFlow{
		Labels: []string{"bug", "feat", "docs"},
		FlowMatrix: [][]int{
			{0, 2, 1}, // bug blocks feat 2 times, docs 1 time
			{1, 0, 3}, // feat blocks bug 1 time, docs 3 times
			{0, 0, 0}, // docs blocks nothing
		},
		TotalCrossLabelDeps: 7,
	}

	result := ui.FlowMatrixView(flow, 80)

	// Should contain header
	if !strings.Contains(result, "DEPENDENCY FLOW SUMMARY") {
		t.Errorf("FlowMatrixView() should contain header, got: %q", result)
	}

	// Should contain labels count
	if !strings.Contains(result, "Labels: 3") {
		t.Errorf("FlowMatrixView() should contain 'Labels: 3', got: %q", result)
	}

	// Should contain cross-label deps count
	if !strings.Contains(result, "Cross-label dependencies: 7") {
		t.Errorf("FlowMatrixView() should contain 'Cross-label dependencies: 7', got: %q", result)
	}
}

func TestFlowMatrixViewWithBottlenecks(t *testing.T) {
	flow := analysis.CrossLabelFlow{
		Labels: []string{"api", "web"},
		FlowMatrix: [][]int{
			{0, 5},
			{2, 0},
		},
		BottleneckLabels:    []string{"api"},
		TotalCrossLabelDeps: 7,
	}

	result := ui.FlowMatrixView(flow, 80)

	// Should contain bottleneck info
	if !strings.Contains(result, "Bottleneck labels: [api]") {
		t.Errorf("FlowMatrixView() should contain bottleneck labels, got: %q", result)
	}
}

func TestFlowMatrixViewNotEmpty(t *testing.T) {
	flow := analysis.CrossLabelFlow{
		Labels: []string{"a", "b"},
		FlowMatrix: [][]int{
			{0, 1},
			{0, 0},
		},
	}

	result := ui.FlowMatrixView(flow, 80)

	if result == "" {
		t.Error("FlowMatrixView() should not return empty string for valid input")
	}

	if len(result) < 20 {
		t.Errorf("FlowMatrixView() output seems too short: %q", result)
	}
}

func TestFlowMatrixViewZeroWidth(t *testing.T) {
	flow := analysis.CrossLabelFlow{
		Labels:     []string{"test"},
		FlowMatrix: [][]int{{0}},
	}

	result := ui.FlowMatrixView(flow, 0)
	if result == "" {
		t.Error("FlowMatrixView(width=0) should not return empty string")
	}
}

func TestFlowMatrixViewNegativeWidth(t *testing.T) {
	flow := analysis.CrossLabelFlow{
		Labels:     []string{"test"},
		FlowMatrix: [][]int{{0}},
	}

	result := ui.FlowMatrixView(flow, -10)
	if result == "" {
		t.Error("FlowMatrixView(width=-10) should not return empty string")
	}
}

func TestFlowMatrixViewLargeMatrix(t *testing.T) {
	labels := []string{"api", "web", "db", "auth", "core"}
	n := len(labels)
	matrix := make([][]int, n)
	for i := range matrix {
		matrix[i] = make([]int, n)
		for j := range matrix[i] {
			if i != j {
				matrix[i][j] = i + j
			}
		}
	}

	flow := analysis.CrossLabelFlow{
		Labels:     labels,
		FlowMatrix: matrix,
	}

	result := ui.FlowMatrixView(flow, 120)

	// Should contain labels count
	if !strings.Contains(result, "Labels: 5") {
		t.Errorf("FlowMatrixView() should contain 'Labels: 5', got: %q", result)
	}
}

func TestFlowMatrixViewOutput(t *testing.T) {
	flow := analysis.CrossLabelFlow{
		Labels:              []string{"bug", "feat"},
		FlowMatrix:          [][]int{{0, 1}, {2, 0}},
		TotalCrossLabelDeps: 3,
		BottleneckLabels:    []string{},
	}

	result := ui.FlowMatrixView(flow, 80)

	// Verify structure
	lines := strings.Split(result, "\n")
	if len(lines) < 4 {
		t.Errorf("FlowMatrixView() should produce at least 4 lines, got %d", len(lines))
	}

	// First line should be header
	if !strings.Contains(lines[0], "DEPENDENCY FLOW SUMMARY") {
		t.Errorf("First line should be header, got: %q", lines[0])
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkFlowMatrixViewSmall(b *testing.B) {
	flow := analysis.CrossLabelFlow{
		Labels:     []string{"a", "b", "c"},
		FlowMatrix: [][]int{{0, 1, 2}, {0, 0, 1}, {1, 0, 0}},
	}
	for i := 0; i < b.N; i++ {
		_ = ui.FlowMatrixView(flow, 80)
	}
}

func BenchmarkFlowMatrixViewMedium(b *testing.B) {
	n := 10
	labels := make([]string, n)
	matrix := make([][]int, n)
	for i := 0; i < n; i++ {
		labels[i] = string(rune('a' + i))
		matrix[i] = make([]int, n)
		for j := 0; j < n; j++ {
			if i != j {
				matrix[i][j] = (i + j) % 5
			}
		}
	}
	flow := analysis.CrossLabelFlow{
		Labels:     labels,
		FlowMatrix: matrix,
	}
	for i := 0; i < b.N; i++ {
		_ = ui.FlowMatrixView(flow, 120)
	}
}

func BenchmarkFlowMatrixViewLarge(b *testing.B) {
	n := 20
	labels := make([]string, n)
	matrix := make([][]int, n)
	for i := 0; i < n; i++ {
		labels[i] = string(rune('a'+i%26)) + string(rune('0'+i/26))
		matrix[i] = make([]int, n)
		for j := 0; j < n; j++ {
			if i != j {
				matrix[i][j] = (i * j) % 10
			}
		}
	}
	flow := analysis.CrossLabelFlow{
		Labels:     labels,
		FlowMatrix: matrix,
	}
	for i := 0; i < b.N; i++ {
		_ = ui.FlowMatrixView(flow, 200)
	}
}

// =============================================================================
// FlowMatrixModel Tests (Interactive Dashboard) - bv-w4l0
// =============================================================================

func testFlowTheme() ui.Theme {
	return ui.DefaultTheme(nil)
}

func TestNewFlowMatrixModel(t *testing.T) {
	theme := testFlowTheme()
	m := ui.NewFlowMatrixModel(theme)

	// Should be able to call View() without panic
	view := m.View()
	if view == "" {
		t.Error("NewFlowMatrixModel().View() should not return empty string")
	}

	// SelectedLabel should return empty string for empty model
	if label := m.SelectedLabel(); label != "" {
		t.Errorf("SelectedLabel() = %q, want empty string for new model", label)
	}
}

func TestFlowMatrixModelSetData(t *testing.T) {
	theme := testFlowTheme()
	m := ui.NewFlowMatrixModel(theme)

	flow := &analysis.CrossLabelFlow{
		Labels: []string{"api", "web", "db"},
		FlowMatrix: [][]int{
			{0, 2, 1},
			{1, 0, 3},
			{0, 0, 0},
		},
		TotalCrossLabelDeps: 7,
		BottleneckLabels:    []string{"api"},
	}

	m.SetData(flow, nil)
	m.SetSize(80, 24)

	view := m.View()
	if strings.Contains(view, "No cross-label dependencies found") {
		t.Error("View() should not show 'no dependencies' after SetData with valid flow")
	}

	if label := m.SelectedLabel(); label == "" {
		t.Error("SelectedLabel() should return a label after SetData")
	}
}

func TestFlowMatrixModelSetDataEmpty(t *testing.T) {
	theme := testFlowTheme()
	m := ui.NewFlowMatrixModel(theme)

	m.SetData(nil, nil)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "No cross-label dependencies found") {
		t.Error("View() should show 'no dependencies' for nil flow data")
	}
}

func TestFlowMatrixModelNavigation(t *testing.T) {
	theme := testFlowTheme()
	m := ui.NewFlowMatrixModel(theme)

	flow := &analysis.CrossLabelFlow{
		Labels: []string{"api", "web", "db", "auth", "core"},
		FlowMatrix: [][]int{
			{0, 2, 1, 0, 1},
			{1, 0, 3, 1, 0},
			{0, 0, 0, 0, 0},
			{2, 1, 0, 0, 1},
			{0, 0, 1, 0, 0},
		},
		TotalCrossLabelDeps: 13,
	}

	m.SetData(flow, nil)
	m.SetSize(80, 24)

	initialLabel := m.SelectedLabel()
	if initialLabel == "" {
		t.Fatal("SelectedLabel() should return a label after SetData")
	}

	m.MoveDown()
	newLabel := m.SelectedLabel()
	if newLabel == initialLabel {
		t.Error("MoveDown() should change selected label")
	}

	m.MoveUp()
	backLabel := m.SelectedLabel()
	if backLabel != initialLabel {
		t.Errorf("MoveUp() should restore selection, got %q want %q", backLabel, initialLabel)
	}

	m.GoToEnd()
	if m.SelectedLabel() == "" {
		t.Error("GoToEnd() should select a label")
	}

	m.GoToStart()
	if m.SelectedLabel() != initialLabel {
		t.Errorf("GoToStart() should select first label")
	}
}

func TestFlowMatrixModelBoundary(t *testing.T) {
	theme := testFlowTheme()
	m := ui.NewFlowMatrixModel(theme)

	flow := &analysis.CrossLabelFlow{
		Labels:     []string{"only-one"},
		FlowMatrix: [][]int{{0}},
	}

	m.SetData(flow, nil)
	m.SetSize(80, 24)

	// Should not panic on boundary
	m.MoveDown()
	m.MoveDown()
	m.MoveUp()
	m.MoveUp()
	m.MoveUp()

	if m.SelectedLabel() != "only-one" {
		t.Errorf("SelectedLabel() = %q, want %q", m.SelectedLabel(), "only-one")
	}
}

func TestFlowMatrixModelTogglePanel(t *testing.T) {
	theme := testFlowTheme()
	m := ui.NewFlowMatrixModel(theme)

	flow := &analysis.CrossLabelFlow{
		Labels:     []string{"a", "b"},
		FlowMatrix: [][]int{{0, 1}, {1, 0}},
	}

	m.SetData(flow, nil)
	m.SetSize(80, 24)

	m.TogglePanel()
	view1 := m.View()
	m.TogglePanel()
	view2 := m.View()

	if view1 == "" || view2 == "" {
		t.Error("View() should not be empty after TogglePanel")
	}
}

func TestFlowMatrixModelDrilldown(t *testing.T) {
	theme := testFlowTheme()
	m := ui.NewFlowMatrixModel(theme)

	flow := &analysis.CrossLabelFlow{
		Labels:     []string{"api"},
		FlowMatrix: [][]int{{0}},
	}

	m.SetData(flow, nil)
	m.SetSize(80, 24)

	if m.SelectedDrilldownIssue() != nil {
		t.Error("SelectedDrilldownIssue() should return nil before OpenDrilldown")
	}

	m.OpenDrilldown()
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after OpenDrilldown")
	}
}

func TestFlowMatrixModelViewRendersContent(t *testing.T) {
	theme := testFlowTheme()
	m := ui.NewFlowMatrixModel(theme)

	flow := &analysis.CrossLabelFlow{
		Labels: []string{"backend", "frontend"},
		FlowMatrix: [][]int{
			{0, 5},
			{2, 0},
		},
		TotalCrossLabelDeps: 7,
		BottleneckLabels:    []string{"backend"},
	}

	m.SetData(flow, nil)
	m.SetSize(100, 30)

	view := m.View()
	if len(view) < 100 {
		t.Errorf("View() seems too short: %d chars", len(view))
	}
}

func TestFlowMatrixModelInvalidMatrix(t *testing.T) {
	theme := testFlowTheme()
	m := ui.NewFlowMatrixModel(theme)

	flow := &analysis.CrossLabelFlow{
		Labels:     []string{"a", "b", "c"},
		FlowMatrix: [][]int{{0}}, // Invalid: 1 row for 3 labels
	}

	m.SetData(flow, nil)
	m.SetSize(80, 24)

	// Should handle gracefully without panic
	_ = m.View()
}

func TestFlowMatrixModelEmptyOperations(t *testing.T) {
	theme := testFlowTheme()
	m := ui.NewFlowMatrixModel(theme)

	// Should not panic on empty model
	m.GoToEnd()
	m.GoToStart()
	m.MoveUp()
	m.MoveDown()
	m.TogglePanel()
	m.OpenDrilldown()

	if m.SelectedLabel() != "" {
		t.Errorf("SelectedLabel() = %q, want empty for empty model", m.SelectedLabel())
	}
}
