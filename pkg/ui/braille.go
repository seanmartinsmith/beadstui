package ui

import (
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

// Braille progress bars for the epics tree (bt-3ftfm.1).
//
// Each bar cell is one braille glyph: a 2-sub-column x 4-dot-row matrix. A
// filled run renders at FULL density (⣿) so it reads as a solid block; the
// unfilled track renders at LOW density (a dim mid/low line) AND in muted grey.
// The density difference - not color alone - is what keeps the filled/track
// boundary legible after ansi.Strip (the render-harness .txt dumps and any
// non-color terminal still convey progress); color then layers the per-status
// composition on top in the .ansi / freeze view.
//
// Dot-bit packing (base U+2800):
//
//	row0: col0=0x01 col1=0x08
//	row1: col0=0x02 col1=0x10
//	row2: col0=0x04 col1=0x20
//	row3: col0=0x40 col1=0x80
//
// Full cell = 0xFF -> U+28FF (⣿). Left column full = 0x47 -> U+2847 (⡇), the
// half-cell. Two horizontal sub-steps per cell give half-cell resolution.
const (
	brailleFull  = "⣿" // U+28FF: all 8 dots
	brailleHalf  = "⡇" // U+2847: left sub-column full (half cell)
	brailleTrack = "⠤" // U+2824: row2 both columns - a dim low line
)

// epicCounts is EpicRow's child-status tally lifted to a small struct so a
// project-header row can carry a lane rollup (sum of its epics) and an epic row
// can carry its own. Total counts every child; Done/InProgress/Blocked/Open are
// the status buckets (other statuses count toward Total but no bucket, so the
// composition track absorbs them).
type epicCounts struct {
	Done, Total               int
	InProgress, Blocked, Open int
	AtRisk                    int
}

// braillePlainBar renders a monochrome done/total fill bar: a full-density
// filled run (green) plus a half-cell at the boundary when the fraction lands
// mid-cell, then a dim low-density track for the remainder. Used for the nested
// child-epic mini bar where status composition is moot. Guards total<=0 and
// width<=0 (no divide-by-zero, no panic).
func braillePlainBar(done, total, width int) string {
	if width <= 0 {
		return ""
	}
	frac := 0.0
	if total > 0 {
		frac = float64(done) / float64(total)
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
	}

	subSteps := width * 2
	filled := int(math.Round(frac * float64(subSteps)))
	if filled > subSteps {
		filled = subSteps
	}
	fullCells := filled / 2
	half := filled%2 == 1

	var b strings.Builder
	filledStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
	trackStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	if fullCells > 0 {
		b.WriteString(filledStyle.Render(strings.Repeat(brailleFull, fullCells)))
	}
	used := fullCells
	if half && used < width {
		b.WriteString(filledStyle.Render(brailleHalf))
		used++
	}
	if used < width {
		b.WriteString(trackStyle.Render(strings.Repeat(brailleTrack, width-used)))
	}
	return b.String()
}

// brailleCompositionBar renders a full-width bar whose cells encode child-status
// composition: a green run for done, yellow for in-progress, red for blocked
// (all full-density ⣿), then a grey low-density track for the open remainder.
// width is the cell count. A cell straddling a category boundary takes its left
// sub-column's category (sub-cell precision loss of <=1 cell - documented, not
// over-engineered). Guards total<=0 (renders an all-grey track) and width<=0.
//
// In-progress maps to ColorPrioMedium (yellow) per the locked design decision;
// the theme has no semantic in-progress-yellow field (its InProgress token is
// blue), so the yellow priority token is the closest stable source.
func brailleCompositionBar(c epicCounts, width int, t Theme) string {
	if width <= 0 {
		return ""
	}

	subSteps := width * 2
	// Cumulative sub-step boundaries, rounded so the segments always sum to
	// exactly subSteps (no off-by-one from independent rounding).
	doneEnd, inprogEnd, blockedEnd := 0, 0, 0
	if c.Total > 0 {
		tot := float64(c.Total)
		doneEnd = int(math.Round(float64(c.Done) / tot * float64(subSteps)))
		inprogEnd = int(math.Round(float64(c.Done+c.InProgress) / tot * float64(subSteps)))
		blockedEnd = int(math.Round(float64(c.Done+c.InProgress+c.Blocked) / tot * float64(subSteps)))
	}

	doneStyle := lipgloss.NewStyle().Foreground(t.Open)            // green
	inprogStyle := lipgloss.NewStyle().Foreground(ColorPrioMedium) // yellow
	blockedStyle := lipgloss.NewStyle().Foreground(t.Blocked)      // red
	trackStyle := lipgloss.NewStyle().Foreground(t.Muted)          // grey

	// Classify each cell by its left sub-column, then render consecutive
	// same-category cells as one styled run (clean, compact ANSI).
	type cat int
	const (
		catDone cat = iota
		catInprog
		catBlocked
		catTrack
	)
	classify := func(cell int) cat {
		left := cell * 2
		switch {
		case left < doneEnd:
			return catDone
		case left < inprogEnd:
			return catInprog
		case left < blockedEnd:
			return catBlocked
		default:
			return catTrack
		}
	}
	glyphFor := func(k cat) (string, lipgloss.Style) {
		switch k {
		case catDone:
			return brailleFull, doneStyle
		case catInprog:
			return brailleFull, inprogStyle
		case catBlocked:
			return brailleFull, blockedStyle
		default:
			return brailleTrack, trackStyle
		}
	}

	var b strings.Builder
	i := 0
	for i < width {
		k := classify(i)
		j := i
		for j < width && classify(j) == k {
			j++
		}
		glyph, style := glyphFor(k)
		b.WriteString(style.Render(strings.Repeat(glyph, j-i)))
		i = j
	}
	return b.String()
}
