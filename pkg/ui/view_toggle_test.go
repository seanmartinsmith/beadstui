package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// TestViewSwitchKeysToggleBack (bt-yzfp2): the flow-matrix (f), attention (]),
// and label-dashboard ([) view-switch keys must toggle back to the list on a
// second press, matching Board/Graph/etc. Before the fix they stranded the
// user (only Esc exited).
func TestViewSwitchKeysToggleBack(t *testing.T) {
	cases := []struct {
		name string
		key  rune
		view ViewMode
	}{
		{"flow-matrix", 'f', ViewFlowMatrix},
		{"attention", ']', ViewAttention},
		{"label-dashboard", '[', ViewLabelDashboard},
	}

	issues := []model.Issue{
		{ID: "bv-1", Title: "One", Status: model.StatusOpen, Priority: 0},
		{ID: "bv-2", Title: "Two", Status: model.StatusInProgress, Priority: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewModel(issues, nil, "", nil)
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 50})
			m = updated.(Model)

			updated, _ = m.Update(tea.KeyPressMsg{Code: tc.key, Text: string(tc.key)})
			m = updated.(Model)
			if m.mode != tc.view {
				t.Fatalf("first press %q: expected %v, got %v", string(tc.key), tc.view, m.mode)
			}

			updated, _ = m.Update(tea.KeyPressMsg{Code: tc.key, Text: string(tc.key)})
			m = updated.(Model)
			if m.mode != ViewList {
				t.Errorf("second press %q: expected ViewList (toggle back), got %v", string(tc.key), m.mode)
			}
		})
	}
}
