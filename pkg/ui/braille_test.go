package ui

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Glyph literals the bars are expected to emit. Kept as literals (not impl
// constants) so the test asserts the actual on-screen characters independently
// of how braille.go names them.
const (
	glyphFull  = "⣿" // U+28FF, all 8 dots — a fully filled cell
	glyphHalf  = "⡇" // U+2847, left sub-column full — a half-filled cell
	glyphTrack = "⠤" // U+2824, low-density mid/low line — the unfilled track
)

func TestBraillePlainBar(t *testing.T) {
	tests := []struct {
		name               string
		done, total, width int
		want               string // expected ANSI-stripped glyphs
	}{
		{"empty 0%", 0, 10, 10, strings.Repeat(glyphTrack, 10)},
		{"full 100%", 10, 10, 10, strings.Repeat(glyphFull, 10)},
		{"half 50%", 5, 10, 10, strings.Repeat(glyphFull, 5) + strings.Repeat(glyphTrack, 5)},
		{"quarter 25% exercises half-cell", 1, 4, 10, strings.Repeat(glyphFull, 2) + glyphHalf + strings.Repeat(glyphTrack, 7)},
		{"three-quarter 75% half-cell", 3, 4, 10, strings.Repeat(glyphFull, 7) + glyphHalf + strings.Repeat(glyphTrack, 2)},
		{"odd width 3 at 50%", 1, 2, 3, glyphFull + glyphHalf + glyphTrack},
		{"zero total no divide-by-zero", 0, 0, 6, strings.Repeat(glyphTrack, 6)},
		{"non-positive width is empty", 5, 10, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ansi.Strip(braillePlainBar(tt.done, tt.total, tt.width))
			if got != tt.want {
				t.Errorf("braillePlainBar(%d, %d, %d) = %q, want %q",
					tt.done, tt.total, tt.width, got, tt.want)
			}
		})
	}
}

// TestBrailleCompositionBar_Structure checks the segment run lengths via the
// ANSI-stripped glyph counts: every filled child status (done/in-progress/
// blocked) renders as a full-density cell, the open remainder as a low-density
// track. This is the signal that survives ansi.Strip (and non-color terminals).
func TestBrailleCompositionBar_Structure(t *testing.T) {
	th := DefaultTheme()
	tests := []struct {
		name      string
		c         epicCounts
		width     int
		wantFull  int // count of full-density cells (done+inprog+blocked)
		wantTrack int // count of low-density track cells (open remainder)
	}{
		{"all open is all track", epicCounts{Total: 5, Open: 5}, 8, 0, 8},
		{"all done is all full", epicCounts{Total: 5, Done: 5}, 8, 8, 0},
		{"mixed composition", epicCounts{Done: 2, InProgress: 1, Blocked: 1, Open: 4, Total: 8}, 8, 4, 4},
		{"zero total renders all track", epicCounts{}, 6, 0, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stripped := ansi.Strip(brailleCompositionBar(tt.c, tt.width, th))
			if got := strings.Count(stripped, glyphFull); got != tt.wantFull {
				t.Errorf("full cells = %d, want %d (stripped=%q)", got, tt.wantFull, stripped)
			}
			if got := strings.Count(stripped, glyphTrack); got != tt.wantTrack {
				t.Errorf("track cells = %d, want %d (stripped=%q)", got, tt.wantTrack, stripped)
			}
		})
	}
}

// TestBrailleCompositionBar_Colors checks the composition is colored per the
// locked decision: done=green, in-progress=yellow, blocked=red, open=grey.
// Gated on the color profile being active so it is meaningful where color
// renders (the case the visual sign-off cares about) and skipped otherwise.
func TestBrailleCompositionBar_Colors(t *testing.T) {
	th := DefaultTheme()
	probe := lipgloss.NewStyle().Foreground(th.Open).Render("x")
	if probe == "x" {
		t.Skip("color profile inactive in this environment; structure covered by TestBrailleCompositionBar_Structure")
	}
	fgSeq := func(c color.Color) string {
		r := lipgloss.NewStyle().Foreground(c).Render("x")
		return r[:strings.Index(r, "x")]
	}
	out := brailleCompositionBar(epicCounts{Done: 2, InProgress: 2, Blocked: 2, Open: 2, Total: 8}, 8, th)
	for _, tc := range []struct {
		name string
		seq  string
	}{
		{"done=green", fgSeq(th.Open)},
		{"in-progress=yellow", fgSeq(ColorPrioMedium)},
		{"blocked=red", fgSeq(th.Blocked)},
		{"open=grey track", fgSeq(th.Muted)},
	} {
		if tc.seq == "" {
			continue
		}
		if !strings.Contains(out, tc.seq) {
			t.Errorf("composition bar missing %s color sequence %q in %q", tc.name, tc.seq, out)
		}
	}
}
