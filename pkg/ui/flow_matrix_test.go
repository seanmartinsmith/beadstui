package ui_test

import (
	"strings"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/ui"
)

// =============================================================================
// FlowMatrixModel Tests (Interactive Dashboard) - bv-w4l0
// =============================================================================

func testFlowTheme() ui.Theme {
	return ui.DefaultTheme()
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
