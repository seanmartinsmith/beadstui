package ui

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// =============================================================================
// TreeModel.ClickAt geometry tests (bt-w8j8.2)
// =============================================================================

// flatTreeIssues returns n root-level issues with no parent-child hierarchy,
// so each renders as exactly one leaf row with an empty prefix -- useful for
// row-math tests that don't care about tree structure.
func flatTreeIssues(n int) []model.Issue {
	issues := make([]model.Issue, 0, n)
	for i := 0; i < n; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("bd-%04d", i),
			Title:     "row",
			Priority:  2,
			IssueType: model.TypeTask,
		})
	}
	return issues
}

// TestTreeClickAt_OutOfBounds verifies every out-of-bounds coordinate misses
// (Index=-1), mirroring HistoryModel.ClickAt's noPane guard.
func TestTreeClickAt_OutOfBounds(t *testing.T) {
	tree := NewTreeModel(testTheme())
	tree.Build(flatTreeIssues(5))
	tree.SetSize(40, 10)

	cases := []struct {
		name string
		x, y int
	}{
		{"x<0", -1, 0},
		{"y<0", 0, -1},
		{"x>=width", 40, 0},
		{"y>=height", 0, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hit := tree.ClickAt(tc.x, tc.y)
			if hit.Index != -1 {
				t.Errorf("expected Index=-1 for %s, got %d", tc.name, hit.Index)
			}
		})
	}
}

// TestTreeClickAt_BasicRowMapping verifies y maps directly to a flatList
// index when the viewport is unscrolled (offset 0), since the tree body has
// no header row unlike HistoryModel.
func TestTreeClickAt_BasicRowMapping(t *testing.T) {
	tree := NewTreeModel(testTheme())
	tree.Build(flatTreeIssues(5))
	tree.SetSize(60, 10)

	hit := tree.ClickAt(5, 0)
	if hit.Index != 0 {
		t.Errorf("y=0: expected Index=0, got %d", hit.Index)
	}
	hit = tree.ClickAt(5, 2)
	if hit.Index != 2 {
		t.Errorf("y=2: expected Index=2, got %d", hit.Index)
	}
}

// TestTreeClickAt_BelowLastNodeMisses verifies clicks in the gap below the
// last rendered node (when the tree has fewer nodes than the viewport
// height) miss rather than resolving to a stale/out-of-range index.
func TestTreeClickAt_BelowLastNodeMisses(t *testing.T) {
	tree := NewTreeModel(testTheme())
	tree.Build(flatTreeIssues(3))
	tree.SetSize(60, 10) // height 10 > 3 nodes

	hit := tree.ClickAt(5, 3)
	if hit.Index != -1 {
		t.Errorf("click just past the last node: expected miss, got Index=%d", hit.Index)
	}
	hit = tree.ClickAt(5, 9)
	if hit.Index != -1 {
		t.Errorf("click in the gap near the bottom: expected miss, got Index=%d", hit.Index)
	}
}

// TestTreeClickAt_ViewportScrolled verifies the click-to-index math accounts
// for viewportOffset, not just the raw y coordinate.
func TestTreeClickAt_ViewportScrolled(t *testing.T) {
	tree := NewTreeModel(testTheme())
	tree.Build(flatTreeIssues(50))
	tree.SetSize(60, 10)
	tree.viewportOffset = 20

	hit := tree.ClickAt(5, 0)
	if hit.Index != 20 {
		t.Errorf("scrolled offset 20, y=0: expected Index=20, got %d", hit.Index)
	}
	hit = tree.ClickAt(5, 9)
	if hit.Index != 29 {
		t.Errorf("scrolled offset 20, y=9: expected Index=29, got %d", hit.Index)
	}
}

// treeHierarchyIssues returns a root issue with a single child, giving a
// 2-node tree where the root has children (expand indicator) and the child
// is a leaf (bullet indicator, no expand affordance).
func treeHierarchyIssues() []model.Issue {
	now := time.Now()
	return []model.Issue{
		{ID: "root", Title: "Root", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "child", Title: "Child", Priority: 1, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "child", DependsOnID: "root", Type: model.DepParentChild}},
		},
	}
}

// TestTreeClickAt_ExpandIndicatorColumn verifies OnExpandIndicator is only
// set when the click lands on the glyph column of a node that has children;
// clicking the same column on a leaf (which renders a bullet, not a
// disclosure triangle) never sets it.
func TestTreeClickAt_ExpandIndicatorColumn(t *testing.T) {
	tree := NewTreeModel(testTheme())
	tree.Build(treeHierarchyIssues())
	tree.SetSize(60, 10)

	// Root (index 0, depth 0) has an empty prefix, so its indicator glyph is
	// at column 0.
	hit := tree.ClickAt(0, 0)
	if hit.Index != 0 {
		t.Fatalf("expected Index=0 for root row, got %d", hit.Index)
	}
	if !hit.OnExpandIndicator {
		t.Error("expected OnExpandIndicator=true clicking column 0 of root (has children)")
	}

	// Clicking further right on the root row (into the title area) should
	// still select the row but not register as an indicator hit.
	hit = tree.ClickAt(10, 0)
	if hit.Index != 0 || hit.OnExpandIndicator {
		t.Errorf("expected Index=0, OnExpandIndicator=false clicking title area, got Index=%d OnExpandIndicator=%v",
			hit.Index, hit.OnExpandIndicator)
	}

	// Child (index 1) is a leaf -- it has no expand affordance regardless of
	// which column is clicked.
	hit = tree.ClickAt(0, 1)
	if hit.Index != 1 {
		t.Fatalf("expected Index=1 for child row, got %d", hit.Index)
	}
	if hit.OnExpandIndicator {
		t.Error("leaf node should never report OnExpandIndicator=true")
	}
}

// =============================================================================
// TreeModel.SelectIndex / MoveCursorBy tests (bt-w8j8.2)
// =============================================================================

// TestTreeSelectIndex verifies SelectIndex clamps out-of-range targets and
// keeps the cursor visible after an absolute jump (mirroring mouse-click
// selection, as opposed to the relative MoveUp/MoveDown steps).
func TestTreeSelectIndex(t *testing.T) {
	tree := NewTreeModel(testTheme())
	tree.Build(flatTreeIssues(50))
	tree.SetSize(60, 10)

	tree.SelectIndex(30)
	if tree.cursor != 30 {
		t.Fatalf("expected cursor=30, got %d", tree.cursor)
	}
	if tree.cursor < tree.viewportOffset || tree.cursor >= tree.viewportOffset+10 {
		t.Errorf("cursor %d not visible with offset %d", tree.cursor, tree.viewportOffset)
	}

	tree.SelectIndex(-5)
	if tree.cursor != 0 {
		t.Errorf("expected SelectIndex(-5) to clamp to 0, got %d", tree.cursor)
	}

	tree.SelectIndex(1000)
	if tree.cursor != 49 {
		t.Errorf("expected SelectIndex(1000) to clamp to last index 49, got %d", tree.cursor)
	}
}

// TestTreeSelectIndex_EmptyTree verifies SelectIndex is a no-op (no panic,
// cursor stays 0) when the tree has no nodes.
func TestTreeSelectIndex_EmptyTree(t *testing.T) {
	tree := NewTreeModel(testTheme())
	tree.Build(nil)
	tree.SetSize(60, 10)

	tree.SelectIndex(5)
	if tree.cursor != 0 {
		t.Errorf("expected cursor to stay 0 on empty tree, got %d", tree.cursor)
	}
}

// TestTreeMoveCursorBy verifies delta-based cursor movement clamps at both
// bounds and keeps the cursor visible, matching the mouse-wheel ramp's needs.
func TestTreeMoveCursorBy(t *testing.T) {
	tree := NewTreeModel(testTheme())
	tree.Build(flatTreeIssues(50))
	tree.SetSize(60, 10)
	tree.cursor = 10

	tree.MoveCursorBy(5)
	if tree.cursor != 15 {
		t.Fatalf("expected cursor=15 after MoveCursorBy(5), got %d", tree.cursor)
	}

	tree.MoveCursorBy(-100)
	if tree.cursor != 0 {
		t.Errorf("expected MoveCursorBy(-100) to clamp to 0, got %d", tree.cursor)
	}

	tree.MoveCursorBy(1000)
	if tree.cursor != 49 {
		t.Errorf("expected MoveCursorBy(1000) to clamp to last index 49, got %d", tree.cursor)
	}
}

// =============================================================================
// Model.handleMouseClick ViewTree dispatch tests (bt-w8j8.2)
// =============================================================================

// treeMouseTestModel builds a model with n flat issues, wired up exactly like
// the E-key tree-toggle handler wires m.tree (model_update_input.go), for
// dispatch-level mouse tests against handleMouseClick/handleMouseWheel.
func treeMouseTestModel(issues []model.Issue) Model {
	m := NewModel(issues, nil, "", nil, nil)
	m.width = 80
	m.height = 30
	m.mode = ViewTree
	m.tree.Build(issues)
	// Mirrors model_view.go's render-time SetSize (bodyW, height-1) -- the
	// authoritative size, since the E-key handler's own SetSize call is
	// overwritten on the next render.
	m.tree.SetSize(m.bodyWidth(), m.height-1)
	m.focused = focusTree
	return m
}

// TestHandleMouseClick_TreeSelectsRow verifies a click on a tree row moves
// the tree cursor to that row and focuses the tree.
func TestHandleMouseClick_TreeSelectsRow(t *testing.T) {
	m := treeMouseTestModel(flatTreeIssues(20))
	m.focused = focusList // start elsewhere to prove the click switches focus

	got, _ := m.handleMouseClick(tea.MouseClickMsg{X: 5, Y: 3, Button: tea.MouseLeft})
	if got.focused != focusTree {
		t.Fatalf("expected focusTree after click, got %v", got.focused)
	}
	if got.tree.cursor != 3 {
		t.Fatalf("expected tree cursor=3 after click on row 3, got %d", got.tree.cursor)
	}
}

// TestHandleMouseClick_TreeExpandToggle verifies a click on the expand
// indicator column of a node with children toggles it, changing NodeCount.
func TestHandleMouseClick_TreeExpandToggle(t *testing.T) {
	m := treeMouseTestModel(treeHierarchyIssues())

	if got := m.tree.NodeCount(); got != 2 {
		t.Fatalf("expected 2 visible nodes before toggle (auto-expanded), got %d", got)
	}

	// Root is at (index 0, depth 0): indicator glyph sits at column 0.
	got, _ := m.handleMouseClick(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if n := got.tree.NodeCount(); n != 1 {
		t.Fatalf("expected 1 visible node after collapsing root via click, got %d", n)
	}

	// Click again on the now-collapsed root to re-expand it.
	got, _ = got.handleMouseClick(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if n := got.tree.NodeCount(); n != 2 {
		t.Fatalf("expected 2 visible nodes after re-expanding root via click, got %d", n)
	}
}

// TestHandleMouseClick_TreeDoubleClickSyncsDetail verifies a second click on
// the same row within listDoubleClickWindow syncs the main list selection to
// the tree's selection and focuses the detail pane -- the tree view's "open
// the bead" gesture (there is no full-screen detail viewport of its own).
func TestHandleMouseClick_TreeDoubleClickSyncsDetail(t *testing.T) {
	issues := flatTreeIssues(20)
	m := treeMouseTestModel(issues)
	m.isSplitView = true

	first, _ := m.handleMouseClick(tea.MouseClickMsg{X: 5, Y: 4, Button: tea.MouseLeft})
	if first.focused != focusTree {
		t.Fatalf("first click: expected focusTree, got %v", first.focused)
	}
	if first.tree.cursor != 4 {
		t.Fatalf("first click: expected tree cursor=4, got %d", first.tree.cursor)
	}

	second, _ := first.handleMouseClick(tea.MouseClickMsg{X: 5, Y: 4, Button: tea.MouseLeft})
	if second.focused != focusDetail {
		t.Fatalf("second click (double-click): expected focusDetail, got %v", second.focused)
	}
	wantID := issues[4].ID
	if sel := second.list.SelectedItem(); sel == nil {
		t.Fatal("expected a selected list item after double-click sync")
	} else if it, ok := sel.(IssueItem); !ok || it.Issue.ID != wantID {
		t.Errorf("expected list selection synced to %s, got %+v", wantID, sel)
	}
}

// TestHandleMouseClick_TreeDoubleClickWindowExpired verifies a second click
// past listDoubleClickWindow is treated as a fresh single click (selects
// only), not a double-click.
func TestHandleMouseClick_TreeDoubleClickWindowExpired(t *testing.T) {
	m := treeMouseTestModel(flatTreeIssues(20))
	m.isSplitView = true
	m.lastTreeClickAt = time.Now().Add(-2 * listDoubleClickWindow)
	m.lastTreeClickRow = 4

	got, _ := m.handleMouseClick(tea.MouseClickMsg{X: 5, Y: 4, Button: tea.MouseLeft})
	if got.focused != focusTree {
		t.Fatalf("expired double-click window: expected focusTree (single click), got %v", got.focused)
	}
}

// TestHandleMouseClick_TreeSidebarColumnIgnored verifies clicks landing at or
// past bodyWidth (the shortcuts sidebar's reserved columns) do not select a
// tree row, mirroring clickListPane's single-pane sidebar guard (bt-bxu6u).
func TestHandleMouseClick_TreeSidebarColumnIgnored(t *testing.T) {
	m := treeMouseTestModel(flatTreeIssues(20))
	m.width = 120
	m.showShortcutsSidebar = true
	m.shortcutsSidebar.SetSize(24, 28) // bodyWidth shrinks to 120-24 = 96
	m.tree.SetSize(m.bodyWidth(), m.height-1)
	m.focused = focusList

	got, _ := m.handleMouseClick(tea.MouseClickMsg{X: m.bodyWidth(), Y: 3, Button: tea.MouseLeft})
	if got.focused != focusList {
		t.Fatalf("click in sidebar columns should not focus the tree, got %v", got.focused)
	}
	if got.tree.cursor != 0 {
		t.Fatalf("click in sidebar columns should not move the tree cursor, got %d", got.tree.cursor)
	}
}

// TestHandleMouseClick_TreeOutOfBoundsYIgnored verifies a click below the
// last rendered node (tree shorter than the viewport) is a no-op.
func TestHandleMouseClick_TreeOutOfBoundsYIgnored(t *testing.T) {
	m := treeMouseTestModel(flatTreeIssues(3))
	m.focused = focusList

	got, _ := m.handleMouseClick(tea.MouseClickMsg{X: 5, Y: 20, Button: tea.MouseLeft})
	if got.focused != focusList {
		t.Fatalf("out-of-bounds click should not focus the tree, got %v", got.focused)
	}
}

// =============================================================================
// Model.handleMouseWheel focusTree ramp tests (bt-w8j8.2)
// =============================================================================

// TestHandleMouseWheel_TreeSingleTickMovesOneNode verifies an isolated wheel
// tick advances the tree cursor by exactly one node.
func TestHandleMouseWheel_TreeSingleTickMovesOneNode(t *testing.T) {
	m := treeMouseTestModel(flatTreeIssues(50))
	m.tree.cursor = 10

	m, _ = m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if got := m.tree.cursor; got != 11 {
		t.Fatalf("isolated down-tick should advance 1 node from cursor 10, got %d", got)
	}
}

// TestHandleMouseWheel_TreeRampAdvancesMultipleNodes verifies a fast burst of
// wheel ticks (seeded mid-ramp) advances the tree cursor by the ramped step,
// the same speed-ramp treatment focusList gets (bt-citoc).
func TestHandleMouseWheel_TreeRampAdvancesMultipleNodes(t *testing.T) {
	m := treeMouseTestModel(flatTreeIssues(100))
	m.tree.cursor = 0
	// Seed mid-ramp so the next down-tick -> streak 7 -> step 1 + 7/2 = 4.
	m.lastWheelDir = +1
	m.lastWheelAt = time.Now()
	m.wheelStreak = 6

	m, _ = m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if got := m.tree.cursor; got != 4 {
		t.Fatalf("ramped down-tick should advance 4 nodes from cursor 0, got %d", got)
	}
}

// TestHandleMouseWheel_TreeRampClampsAtBottom verifies the ramped step never
// moves the cursor past the last node.
func TestHandleMouseWheel_TreeRampClampsAtBottom(t *testing.T) {
	m := treeMouseTestModel(flatTreeIssues(20))
	m.tree.cursor = 18
	m.lastWheelDir = +1
	m.lastWheelAt = time.Now()
	m.wheelStreak = 100 // step capped at wheelStepMax, well past the last index

	m, _ = m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if got := m.tree.cursor; got != 19 {
		t.Fatalf("ramped tick at the bottom should clamp to last index 19, got %d", got)
	}
}

// TestHandleMouseWheel_TreeRampClampsAtTop verifies the ramped step (moving
// up) never moves the cursor before the first node.
func TestHandleMouseWheel_TreeRampClampsAtTop(t *testing.T) {
	m := treeMouseTestModel(flatTreeIssues(20))
	m.tree.cursor = 1
	m.lastWheelDir = -1
	m.lastWheelAt = time.Now()
	m.wheelStreak = 100 // step capped at wheelStepMax, well past the first index

	m, _ = m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if got := m.tree.cursor; got != 0 {
		t.Fatalf("ramped tick at the top should clamp to first index 0, got %d", got)
	}
}
