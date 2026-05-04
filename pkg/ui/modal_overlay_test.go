package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/seanmartinsmith/beadstui/pkg/cass"
)

// faintSGR is the ANSI Select-Graphic-Rendition byte sequence for the Faint
// (dim) attribute. The dimmed-backdrop compositor (bt-v8he) wraps every bg
// line in this attribute so the modal reads as a true pop-up. Tests assert
// presence of this sequence in modal-adjacent rows.
const faintSGR = "\x1b[2"

// TestAlertsModalOccludesDetailPane is a regression guard for bt-l5xu and
// bt-v8he: the shared alerts/notifications modal must occlude the underlying
// detail pane (bt-l5xu, no bleed-through of body text) AND render at a
// content-comfortable width rather than spanning the terminal (bt-v8he,
// pop-up aesthetic). The two are reconciled by dimming the backdrop instead
// of widening the modal.
//
// Pre-bt-l5xu: panel capped at 80, bg leaked along the modal's flanks.
// bt-l5xu fix: panel sized to m.width-4 — solved leak, lost pop-up shape.
// bt-v8he fix: panel re-capped at 100, OverlayCenterDimBackdrop applies
//	Faint to all bg cells so they recede visually instead of being absent.
//
// Verifications:
//  1. No row in the modal's vertical band contains the bg-viewport sentinel
//     in its un-dimmed form. The sentinel may still appear inside a Faint
//     wrap on rows flanking the modal — that's the new contract; we check
//     the rows BETWEEN the borders specifically (the modal's interior),
//     where the modal's own content overwrites the bg entirely.
//  2. Modal content width is bounded — at every tested terminal width the
//     modal stays at the 100-cell cap (or m.width-4 if narrower). This
//     guards against a regression that quietly widens the modal back to
//     full width.
//  3. Rows immediately around the modal contain the Faint SGR code,
//     proving the backdrop is dimmed rather than blanked or untouched.
func TestAlertsModalOccludesDetailPane(t *testing.T) {
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
			longLine := strings.Repeat(sentinel+" ", 60)
			m.viewport.SetContent(strings.Repeat(longLine+"\n", 40))

			m = pressRune(m, '1')
			if m.activeModal != ModalAlerts {
				t.Fatalf("expected ModalAlerts after pressing '1', got %v", m.activeModal)
			}

			view := m.View().Content
			rows := strings.Split(view, "\n")

			// Locate the modal band. The dimmed-backdrop compositor (bt-v8he)
			// leaves multiple `╭` corners on the screen — the outer list panel
			// at column 0, the details panel mid-row, and the modal itself
			// inside the dimmed backdrop. The modal is the one whose width
			// matches alertsPanelWidth(), so detect by computing its expected
			// left column (the centering math from OverlayCenterDimBackdrop)
			// and confirming a corner sits there.
			panelW := m.alertsPanelWidth()
			expectedLeft := (tc.w - panelW) / 2
			expectedRight := expectedLeft + panelW - 1

			topRow, bottomRow := -1, -1
			for i, r := range rows {
				stripped := ansi.Strip(r)
				if idx := strings.Index(stripped, "╭─ Notifications"); idx >= 0 && topRow == -1 {
					topRow = i
					if idx != expectedLeft {
						t.Logf("modal top corner at byte-col %d, expected ~%d (string indexing reflects byte not cell offset)",
							idx, expectedLeft)
					}
				}
				// Bottom border of the modal is a row of `─` flanked by
				// `╰` and `╯` with no other box-drawing on the row's
				// modal-column span. Match by anchoring on the modal's
				// column being a `╰`.
				if topRow != -1 && bottomRow == -1 && i > topRow {
					if strings.Contains(stripped, "╰─") &&
						strings.Contains(stripped, "─╯") &&
						!strings.Contains(stripped, " Notifications") {
						bottomRow = i
					}
				}
			}
			if topRow == -1 || bottomRow == -1 {
				t.Fatalf("could not locate modal band: top=%d bottom=%d", topRow, bottomRow)
			}

			// (1) Modal interior must not leak the sentinel. Slice the
			// modal's column range out of each interior row and assert no
			// sentinel is present. The dimmed backdrop flanks may legitimately
			// carry stripped-and-dimmed bg content — that's the new contract.
			for i := topRow + 1; i < bottomRow; i++ {
				stripped := ansi.Strip(rows[i])
				if expectedLeft+1 >= len(stripped) {
					continue
				}
				rightCut := expectedRight
				if rightCut > len(stripped) {
					rightCut = len(stripped)
				}
				interior := stripped[expectedLeft+1 : rightCut]
				if strings.Contains(interior, sentinel) {
					t.Errorf("modal interior row %d leaks detail-pane content: %q", i, interior)
				}
			}

			// (2) Modal content width is bounded. The terminal-width-minus-4
			// fallback only kicks in below the cap; at every tested width we
			// expect exactly the cap.
			expected := tc.w - 4
			if expected > 100 {
				expected = 100
			}
			if panelW != expected {
				t.Errorf("alertsPanelWidth() = %d, want %d (terminal width %d)",
					panelW, expected, tc.w)
			}

			// (3) Backdrop is dimmed. Some row at or near the modal band
			// must carry the Faint SGR code, proving the compositor wrapped
			// bg cells rather than leaving them untouched.
			scanStart := topRow - 1
			if scanStart < 0 {
				scanStart = 0
			}
			scanEnd := bottomRow + 1
			if scanEnd >= len(rows) {
				scanEnd = len(rows) - 1
			}
			foundDim := false
			for i := scanStart; i <= scanEnd; i++ {
				if strings.Contains(rows[i], faintSGR) {
					foundDim = true
					break
				}
			}
			if !foundDim {
				t.Errorf("expected Faint SGR (\\x1b[2m) in rows %d-%d, found none - backdrop not dimmed",
					scanStart, scanEnd)
			}
		})
	}
}

// TestAllModalsUseDimBackdrop is the bt-o1hs regression guard: every modal
// overlay must dim the backdrop via OverlayCenterDimBackdrop, not the non-dim
// OverlayCenter. Per the unified pop-up aesthetic introduced by bt-v8he and
// extended to all modals by bt-o1hs, opening any modal must produce Faint SGR
// codes in the rendered output around the modal — proving the compositor
// wrapped the bg cells rather than leaving them untouched.
//
// The five modal types covered: Alerts (already correct, regression guard),
// RepoPicker (Project Filter), LabelPicker (Label Filter), AgentPrompt,
// CassSession, Update.
func TestAllModalsUseDimBackdrop(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Model)
	}{
		{
			name:  "alerts",
			setup: func(m *Model) { m.activeModal = ModalAlerts },
		},
		{
			name: "repo picker",
			setup: func(m *Model) {
				m.repoPicker = NewRepoPickerModel([]string{"alpha", "beta"}, m.theme)
				m.activeModal = ModalRepoPicker
			},
		},
		{
			name: "label picker",
			setup: func(m *Model) {
				m.labelPicker = NewLabelPickerModel([]string{"area:tui"}, map[string]int{"area:tui": 1}, m.theme)
				m.activeModal = ModalLabelPicker
			},
		},
		{
			name: "agent prompt",
			setup: func(m *Model) {
				m.agentPromptModal = NewAgentPromptModal("/test/AGENTS.md", "AGENTS.md", m.theme)
				m.activeModal = ModalAgentPrompt
			},
		},
		{
			name: "cass session",
			setup: func(m *Model) {
				m.cassModal = NewCassSessionModal("bt-fix", cass.CorrelationResult{BeadID: "bt-fix"}, m.theme)
				m.activeModal = ModalCassSession
			},
		},
		{
			name: "update",
			setup: func(m *Model) {
				m.updateModal = NewUpdateModal("v1.0.0", "", m.theme)
				m.activeModal = ModalUpdate
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := seedModel()
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
			m = updated.(Model)
			tc.setup(&m)

			view := m.View().Content
			if view == "" {
				t.Fatal("View() returned empty content")
			}

			if !strings.Contains(view, faintSGR) {
				t.Errorf("expected Faint SGR (\\x1b[2m) in rendered output for %s modal, found none — backdrop not dimmed (modal must use OverlayCenterDimBackdrop, not OverlayCenter)",
					tc.name)
			}
		})
	}
}

// TestModalContentWidth_ConstantAcrossTerminalSizes guards the chrome
// stability promise of bt-v8he: as the terminal grows wider the modal does
// not — it stays capped at the content-comfortable width so the pop-up
// aesthetic holds at any reasonable terminal size.
func TestModalContentWidth_ConstantAcrossTerminalSizes(t *testing.T) {
	widths := []int{120, 160, 200, 240, 320}
	const expected = 100 // cap from alertsPanelWidth

	var prev int
	for i, w := range widths {
		m := seedModel()
		updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 40})
		m = updated.(Model)
		got := m.alertsPanelWidth()
		if got != expected {
			t.Errorf("width=%d: alertsPanelWidth()=%d, want %d", w, got, expected)
		}
		if i > 0 && got != prev {
			t.Errorf("width=%d: alertsPanelWidth() changed from %d to %d as terminal grew",
				w, prev, got)
		}
		prev = got
	}
}
