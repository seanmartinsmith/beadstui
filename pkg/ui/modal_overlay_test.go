package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// TestAlertsModalOccludesDetailPane is a regression guard for bt-l5xu: the
// shared alerts/notifications modal must fully cover the underlying detail
// pane so body text from the currently-selected bead doesn't bleed through
// along the modal's right border. Bug existed because panelWidth was capped
// at 80, leaving wide swaths of bg detail-pane content visible flanking the
// modal at typical split-view terminal widths (notably the right side of the
// modal, where the underlying detail viewport's content was visible between
// the modal's right border and the detail pane's right border).
//
// Verification: stuff the detail viewport with a sentinel string, render the
// composed View() with the modal open, and assert that no row in the modal's
// vertical band contains the sentinel anywhere.
func TestAlertsModalOccludesDetailPane(t *testing.T) {
	// Short sentinel so a thin leak slice still catches it. The bug bleeds
	// only the right-edge of the bg row past the modal's right border, which
	// at narrow split widths can be just a handful of cells wide.
	const sentinel = "BLD"

	cases := []struct {
		name string
		w, h int
	}{
		{"narrow split", 120, 30},
		{"wide split", 180, 50},
		{"very wide split", 220, 60},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := seedModel()
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tc.w, Height: tc.h})
			m = updated.(Model)
			// Populate the detail viewport with a recognizable sentinel so we
			// can detect any leak in the composite output.
			longLine := strings.Repeat(sentinel+" ", 60)
			m.viewport.SetContent(strings.Repeat(longLine+"\n", 40))

			// Open the notifications modal.
			m = pressRune(m, '1')
			if m.activeModal != ModalAlerts {
				t.Fatalf("expected ModalAlerts after pressing '1', got %v", m.activeModal)
			}

			view := m.View().Content
			rows := strings.Split(view, "\n")

			// Locate the modal band by scanning for the top (╭) and bottom (╰)
			// corners that appear at an interior column (not at column 0,
			// which is the outer split-view list border).
			topRow, bottomRow := -1, -1
			for i, r := range rows {
				stripped := ansi.Strip(r)
				if topRow == -1 {
					if idx := strings.Index(stripped, "╭"); idx > 0 {
						topRow = i
					}
				}
				if idx := strings.Index(stripped, "╰"); idx > 0 {
					bottomRow = i // last interior bottom corner wins
				}
			}
			if topRow == -1 || bottomRow == -1 || bottomRow <= topRow {
				t.Fatalf("could not locate modal band: top=%d bottom=%d", topRow, bottomRow)
			}

			for i := topRow; i <= bottomRow; i++ {
				stripped := ansi.Strip(rows[i])
				if strings.Contains(stripped, sentinel) {
					t.Errorf("modal band row %d leaks detail-pane content: %q", i, stripped)
				}
			}
		})
	}
}
