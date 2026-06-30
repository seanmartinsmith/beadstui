package ui

// Footer-flush measurement probe for bt-2f9ff. NOT a regression test — gated
// behind BT_FOOTER_PROBE, prints a table. For each (view, height) it renders
// the real View() and measures:
//   - lines:    total rendered rows (ANSI stripped)
//   - footerH:  lipgloss.Height(renderFooter()) — should be 1
//   - lastBlank: is the final visual row empty? (the "blank row beneath the
//                footer" signature the bead describes)
//   - tail:     last two rows, trimmed, so the footer is eyeballable
//
// Run: BT_FOOTER_PROBE=1 go test ./pkg/ui -run TestFooterFlushProbe -v

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// TestFooterPinnedToLastRow_OverflowView guards the footer's last-row
// guarantee (bt-2f9ff) for the insights/attention view, which over-renders its
// SetSize budget on BOTH axes — it emits ~32 rows for a height-1 budget and, at
// some widths (e.g. 120), a body line one cell wider than the terminal. Before
// the MaxWidth clamp in View(), that over-wide line wrapped under
// finalStyle.Width and pushed the JoinVertical past MaxHeight(m.height), which
// clipped the footer off the bottom entirely (last visual row became a panel
// border "│" instead of the status bar). The width sweep straddles the
// overshoot boundary so a regression in the clamp re-surfaces here.
func TestFooterPinnedToLastRow_OverflowView(t *testing.T) {
	const footerToken = "l:labels" // stable hint rendered in the status bar
	for _, w := range []int{100, 110, 120, 121, 140, 160} {
		for _, h := range []int{14, 20, 30} {
			m := NewModel(harnessIssues(), nil, "", nil)
			nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
			m = nm.(Model)
			m.openInsightsView()

			plain := ansi.Strip(m.View().Content)
			lines := strings.Split(plain, "\n")
			if len(lines) != h {
				t.Errorf("insights %dx%d: rendered %d rows, want exactly %d", w, h, len(lines), h)
			}
			last := lines[len(lines)-1]
			if !strings.Contains(last, footerToken) {
				t.Errorf("insights %dx%d: footer not pinned to last row; last row = %q", w, h, strings.TrimRight(last, " "))
			}
		}
	}
}

func TestFooterFlushProbe(t *testing.T) {
	if os.Getenv("BT_FOOTER_PROBE") == "" {
		t.Skip("set BT_FOOTER_PROBE=1 to run the footer-flush probe")
	}

	views := []struct {
		name  string
		setup func(*Model)
	}{
		{"list", nil},
		{"detail", func(m *Model) {
			harnessSelect(m, "bt-0qzp")
			m.showDetails = true
			m.focused = focusDetail
			m.updateViewportContent()
		}},
		{"actionable", func(m *Model) {
			m.mode = ViewActionable
			m.focused = focusActionable
		}},
		{"board", func(m *Model) {
			m.mode = ViewBoard
			m.focused = focusBoard
			m.refreshBoardAndGraphForCurrentFilter()
		}},
		{"insights", func(m *Model) { m.openInsightsView() }},
		{"alerts_modal", func(m *Model) { m.openModal(ModalAlerts) }},
	}

	// Sweep a contiguous run of integer heights so any height-parity bug shows
	// up as alternating rows.
	heights := []int{}
	for h := 14; h <= 40; h++ {
		heights = append(heights, h)
	}

	const width = 120

	for _, v := range views {
		t.Logf("=== view=%s width=%d ===", v.name, width)
		t.Logf("  %4s %5s %7s %9s  %s", "H", "lines", "footerH", "lastBlank", "tail(last 2 rows)")
		for _, h := range heights {
			issues := harnessIssues()
			m := NewModel(issues, nil, "", nil)
			nm, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: h})
			m = nm.(Model)
			if v.setup != nil {
				v.setup(&m)
			}

			content := m.View().Content
			plain := ansi.Strip(content)
			footerH := lipgloss.Height(m.renderFooter())

			lines := strings.Split(plain, "\n")
			n := len(lines)
			lastBlank := strings.TrimRight(lines[n-1], " ") == ""
			// Compose a short tail view of the last two non-truncated rows.
			row := func(i int) string {
				if i < 0 || i >= n {
					return ""
				}
				s := strings.TrimRight(lines[i], " ")
				if len(s) > 38 {
					s = s[:38]
				}
				return s
			}
			tail := fmt.Sprintf("[%q | %q]", row(n-2), row(n-1))

			flag := ""
			if n != h {
				flag += " <-LINES!=H"
			}
			if lastBlank {
				flag += " <-LASTBLANK"
			}
			if footerH != 1 {
				flag += " <-FOOTER>1"
			}
			t.Logf("  %4d %5d %7d %9t  %s%s", h, n, footerH, lastBlank, tail, flag)
		}
	}
}
