package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestView_MouseModeInvariant locks the bt-ll7 invariant: once the model is
// ready, every View() must request MouseModeCellMotion. The renderer relies
// on this to emit the matching disable sequence on shutdown — if a future
// branch returns a view without MouseMode set, mouse tracking can leak past
// teardown into the host terminal.
func TestView_MouseModeInvariant(t *testing.T) {
	cases := []struct {
		name  string
		setup func(Model) Model
	}{
		{"list view", func(m Model) Model { m.mode = ViewList; return m }},
		{"graph view", func(m Model) Model { m.mode = ViewGraph; return m }},
		{"board view", func(m Model) Model { m.mode = ViewBoard; return m }},
		{"insights view", func(m Model) Model { m.mode = ViewInsights; return m }},
		{"actionable view", func(m Model) Model { m.mode = ViewActionable; return m }},
		{"quit-confirm modal", func(m Model) Model { m.activeModal = ModalQuitConfirm; return m }},
		{"alerts modal", func(m Model) Model { m.activeModal = ModalAlerts; return m }},
		{"repo picker modal", func(m Model) Model { m.activeModal = ModalRepoPicker; return m }},
		{"label picker modal", func(m Model) Model { m.activeModal = ModalLabelPicker; return m }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setup(seedModel())
			v := m.View()
			if v.MouseMode != tea.MouseModeCellMotion {
				t.Fatalf("View().MouseMode = %v, want MouseModeCellMotion", v.MouseMode)
			}
			if !v.AltScreen {
				t.Fatalf("View().AltScreen = false, want true")
			}
		})
	}
}
