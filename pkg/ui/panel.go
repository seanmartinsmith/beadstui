package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

// BorderVariant controls the weight of box-drawing characters.
type BorderVariant int

const (
	BorderNormal BorderVariant = iota // ╭─╮│╰╯
	BorderThick                       // ┏━┓┃┗┛
	BorderDouble                      // ╔═╗║╚╝
)

// PanelOpts configures a titled panel.
type PanelOpts struct {
	Title   string
	Width   int
	Height  int
	Focused bool
	Variant BorderVariant

	// CenterTitle places the title in the center of the top border
	// instead of left-aligned.
	CenterTitle bool

	// RightLabel, when non-empty, renders a right-aligned chunk in the
	// top border using TitleColor. The middle fill adjusts so the top-right
	// corner stays anchored regardless of label width (bt-46p6.10). Only
	// honored in the non-centered (left-aligned title) path.
	RightLabel string

	// Optional color overrides. When non-nil these take precedence
	// over the default focus-based colors, letting callers supply
	// custom border/title colors (e.g. per-column board colors,
	// dimmed "skipped" panels).
	BorderColor color.Color
	TitleColor  color.Color
}

// borderChars returns the box-drawing characters for a variant.
func borderChars(v BorderVariant) (tl, tr, bl, br, h, vert string) {
	switch v {
	case BorderThick:
		return "┏", "┓", "┗", "┛", "━", "┃"
	case BorderDouble:
		return "╔", "╗", "╚", "╝", "═", "║"
	default:
		return "╭", "╮", "╰", "╯", "─", "│"
	}
}

// RenderTitledPanel draws a box with the title inlined in the top border.
// The content is placed inside with no extra padding beyond the border itself.
//
//	╭─ Title ──────────────────╮
//	│ content                  │
//	╰──────────────────────────╯
func RenderTitledPanel(content string, opts PanelOpts) string {
	if opts.Width < 4 {
		opts.Width = 4
	}

	tl, tr, bl, br, h, vert := borderChars(opts.Variant)

	// Colors: use overrides when provided, otherwise derive from focus state.
	// Unfocused border uses ColorMuted (matches the unfocused title color)
	// rather than ColorBgHighlight so the frame stays readable on dark
	// terminals — bt-peo7 dogfooding showed that dim borders next to a
	// brighter title made multi-pane views (history) read as broken chrome
	// even though all four borders were rendering correctly.
	var borderColor, titleColor color.Color
	if opts.BorderColor != nil {
		borderColor = opts.BorderColor
	} else if opts.Focused {
		borderColor = ColorPrimary
	} else {
		borderColor = ColorMuted
	}

	if opts.TitleColor != nil {
		titleColor = opts.TitleColor
	} else if opts.Focused {
		titleColor = ColorPrimary
	} else {
		titleColor = ColorMuted
	}

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleStyle := lipgloss.NewStyle().Foreground(titleColor)
	if opts.Focused {
		titleStyle = titleStyle.Bold(true)
	}

	innerWidth := opts.Width - 2 // subtract left and right border chars

	// Build top line: ┌─ Title ─────┐
	var top strings.Builder
	top.WriteString(borderStyle.Render(tl))

	if opts.Title != "" {
		titleText := opts.Title
		// Truncate title if too wide (leave room for "─ " prefix, " ─" suffix, corners)
		maxTitle := innerWidth - 4 // "─ " + " ─" = 4 chars overhead
		if maxTitle < 1 {
			maxTitle = 1
		}
		titleDisplayWidth := runewidth.StringWidth(titleText)
		if titleDisplayWidth > maxTitle {
			titleText = runewidth.Truncate(titleText, maxTitle-1, "") + "…"
			titleDisplayWidth = runewidth.StringWidth(titleText)
		}

		if opts.CenterTitle {
			// Centered: ╭──── Title ─────╮
			// Between corners: leftFill + " " + title + " " + rightFill = innerWidth
			titleOverhead := titleDisplayWidth + 2 // " " + title + " "
			fillTotal := innerWidth - titleOverhead
			if fillTotal < 0 {
				fillTotal = 0
			}
			leftFill := fillTotal / 2
			rightFill := fillTotal - leftFill
			if leftFill > 0 {
				top.WriteString(borderStyle.Render(strings.Repeat(h, leftFill)))
			}
			top.WriteString(borderStyle.Render(" "))
			top.WriteString(titleStyle.Render(titleText))
			top.WriteString(borderStyle.Render(" "))
			if rightFill > 0 {
				top.WriteString(borderStyle.Render(strings.Repeat(h, rightFill)))
			}
		} else {
			// Left-aligned: ╭─ Title ──────╮  or  ╭─ Title ────── Label ─╮
			titleChunk := 2 + titleDisplayWidth + 1 // "─ " + title + " "
			rightChunk := 0
			var rightDisplay string
			if opts.RightLabel != "" {
				rightDisplay = opts.RightLabel
				rightDisplayWidth := runewidth.StringWidth(rightDisplay)
				// Match " label ─" (one space on each side of label, trailing dash).
				rightChunk = 1 + rightDisplayWidth + 2
			}
			fillTotal := innerWidth - titleChunk - rightChunk
			if fillTotal < 0 {
				fillTotal = 0
			}
			top.WriteString(borderStyle.Render(h + " "))
			top.WriteString(titleStyle.Render(titleText))
			top.WriteString(borderStyle.Render(" "))
			if rightChunk > 0 {
				if fillTotal > 0 {
					top.WriteString(borderStyle.Render(strings.Repeat(h, fillTotal)))
				}
				top.WriteString(borderStyle.Render(" "))
				top.WriteString(titleStyle.Render(rightDisplay))
				top.WriteString(borderStyle.Render(" " + h))
			} else if fillTotal > 0 {
				top.WriteString(borderStyle.Render(strings.Repeat(h, fillTotal)))
			}
		}
	} else if opts.RightLabel != "" {
		// No left title, only a right-aligned label (bt-fxbl). Renders as
		// ╭───────────── Label ─╮  — the panel-as-titled-strip variant.
		rightDisplay := opts.RightLabel
		rightDisplayWidth := runewidth.StringWidth(rightDisplay)
		// " label ─" overhead (space + label + space + trailing dash)
		rightChunk := 1 + rightDisplayWidth + 2
		fillTotal := innerWidth - rightChunk
		if fillTotal < 0 {
			fillTotal = 0
		}
		if fillTotal > 0 {
			top.WriteString(borderStyle.Render(strings.Repeat(h, fillTotal)))
		}
		top.WriteString(borderStyle.Render(" "))
		top.WriteString(titleStyle.Render(rightDisplay))
		top.WriteString(borderStyle.Render(" " + h))
	} else {
		top.WriteString(borderStyle.Render(strings.Repeat(h, innerWidth)))
	}
	top.WriteString(borderStyle.Render(tr))

	// Build bottom line: └──────────────┘
	bottom := borderStyle.Render(bl) +
		borderStyle.Render(strings.Repeat(h, innerWidth)) +
		borderStyle.Render(br)

	// Build content lines with side borders
	leftBorder := borderStyle.Render(vert)
	rightBorder := borderStyle.Render(vert)

	contentLines := strings.Split(content, "\n")

	// If height specified, pad or truncate content
	if opts.Height > 0 {
		visibleLines := opts.Height - 2 // subtract top and bottom border
		if visibleLines < 0 {
			visibleLines = 0
		}
		for len(contentLines) < visibleLines {
			contentLines = append(contentLines, "")
		}
		if len(contentLines) > visibleLines {
			contentLines = contentLines[:visibleLines]
		}
	}

	var body strings.Builder
	for _, line := range contentLines {
		// Pad or truncate each line to innerWidth. Truncate via ansi.Truncate
		// rather than runewidth.Truncate so SGR escape bytes in styled rows
		// are not counted as visible cells; runewidth.Truncate over-truncates
		// styled content, producing rows whose ansi.StringWidth is below
		// innerWidth. The compositor then sees fg rows of inconsistent widths
		// and drifts the modal's right border across rows (bt-l22b root cause,
		// surfaced after the defensive bg/fg padding fix).
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth {
			line = line + strings.Repeat(" ", innerWidth-lineWidth)
		} else if lineWidth > innerWidth {
			line = ansi.Truncate(line, innerWidth, "")
		}
		body.WriteString(leftBorder)
		body.WriteString(line)
		body.WriteString(rightBorder)
		body.WriteString("\n")
	}

	return top.String() + "\n" + body.String() + bottom
}

// OverlayCenter composites fg centered on top of bg, preserving ANSI styling.
// Uses charmbracelet/x/ansi for ANSI-aware string slicing so background colors
// are preserved in the left/right regions flanking the overlay.
// bgWidth/bgHeight are used only for centering math - the bg line count is
// preserved exactly so the view pipeline's height assumptions aren't broken.
//
// NOTE: For modal overlays, prefer OverlayCenterDimBackdrop. The dim variant
// is the canonical modal compositor (bt-o1hs) — it dims the underlying view
// so modals read as true pop-ups instead of content-shaped panels embedded in
// the surrounding view. OverlayCenter (this function) is reserved for
// non-modal overlays such as debug panels or transient hints where the user
// is meant to keep reading the underlying view.
func OverlayCenter(bg, fg string, bgWidth, bgHeight int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	fgWidth := 0
	for _, line := range fgLines {
		if w := ansi.StringWidth(line); w > fgWidth {
			fgWidth = w
		}
	}
	fgHeight := len(fgLines)

	// Center offsets (use bgHeight for vertical centering, not len(bgLines))
	startRow := (bgHeight - fgHeight) / 2
	startCol := (bgWidth - fgWidth) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	// Pad every bg row to bgWidth so the per-row slice positions are
	// consistent. Without this, rows whose underlying content is shorter
	// than bgWidth cause ansi.Truncate(line, startCol, "") to return the
	// whole short line and ansi.TruncateLeft(line, startCol+fgWidth, "")
	// to return empty - so the fg lands at column = visibleWidth(line)
	// instead of startCol on those rows, drifting the modal's vertical
	// borders across rows (bt-l22b). Appending plain spaces inherits
	// whatever SGR state the line ended in; lipgloss-rendered rows reset
	// at end-of-content so the padding renders as default-style blanks.
	for i, line := range bgLines {
		if w := ansi.StringWidth(line); w < bgWidth {
			bgLines[i] = line + strings.Repeat(" ", bgWidth-w)
		}
	}

	for i, fgLine := range fgLines {
		bgRow := startRow + i
		if bgRow < 0 || bgRow >= len(bgLines) {
			continue
		}

		// Pad fg row to fgWidth so the right bg slice resumes immediately
		// after the modal's actual right edge. Without this, fg rows whose
		// reported visible width is less than fgWidth (e.g., bt-rhfo class:
		// terminal renders a glyph wider than runewidth / ansi.StringWidth
		// reports) leave a gap between the modal's visible right edge and
		// where the right bg slice starts - drifting the modal's right
		// border across rows (bt-l22b hypothesis #3).
		if w := ansi.StringWidth(fgLine); w < fgWidth {
			fgLine = fgLine + strings.Repeat(" ", fgWidth-w)
		}

		bgLine := bgLines[bgRow]

		// ANSI-aware slicing with mid-glyph compensation (bt-3ykii). When the
		// cut columns fall inside a multi-cell glyph in the bg, ansi.Truncate
		// cuts BEFORE the glyph (left ends up short) and ansi.TruncateLeft
		// preserves the partial glyph (right ends up long). Normalize each
		// portion to its target width with plain-space padding / truncation
		// so the reassembled row equals bgWidth.
		left, right := sliceBgRow(bgLine, startCol, fgWidth, bgWidth, identityPad)

		bgLines[bgRow] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}

// identityPad returns its argument unchanged. Used as the padding wrapper for
// OverlayCenter where the bg has no compositor-applied SGR styling.
func identityPad(s string) string { return s }

// sliceBgRow extracts the left and right portions of bgLine for compositing
// around an fg modal at startCol with width fgWidth. The returned strings
// have visible widths of EXACTLY startCol and EXACTLY
// (bgWidth - startCol - fgWidth) cells, even when the cut columns fall
// mid-glyph. Without this, ansi.Truncate / ansi.TruncateLeft produce a left
// that is short (cuts before the partial glyph) or a right that is long
// (preserves the partial glyph), and the reassembled row width drifts row to
// row depending on bg content. bt-3ykii.
//
// padder wraps any synthesized space padding so the caller can match the
// surrounding SGR state (e.g., dim.Render for the dim-backdrop compositor).
// Pass identityPad for plain spaces.
func sliceBgRow(bgLine string, startCol, fgWidth, bgWidth int, padder func(string) string) (left, right string) {
	// Left side: ansi.Truncate cuts BEFORE a partial glyph, so the result
	// may be 1+ cells short. Pad up with synthesized spaces.
	left = ansi.Truncate(bgLine, startCol, "")
	if w := ansi.StringWidth(left); w < startCol {
		left = left + padder(strings.Repeat(" ", startCol-w))
	}

	// Right side: ansi.TruncateLeft refuses to split a partial glyph, so it
	// returns the partial glyph intact — making `right` too wide. Advance
	// the cut column past the partial glyph (one cell at a time, bounded by
	// bgWidth), then pad any resulting shortfall with leading spaces so the
	// total width matches the target.
	rightTarget := bgWidth - startCol - fgWidth
	if rightTarget < 0 {
		rightTarget = 0
	}
	cutAt := startCol + fgWidth
	right = ansi.TruncateLeft(bgLine, cutAt, "")
	for ansi.StringWidth(right) > rightTarget && cutAt < bgWidth {
		cutAt++
		right = ansi.TruncateLeft(bgLine, cutAt, "")
	}
	if w := ansi.StringWidth(right); w < rightTarget {
		right = padder(strings.Repeat(" ", rightTarget-w)) + right
	}
	return left, right
}

// OverlayBottomRight composites fg anchored to the bottom-right corner of bg,
// offset by marginRight/marginBottom cells, preserving ANSI styling. Reuses
// the same ANSI-aware row-slicing (sliceBgRow) as OverlayCenter — this is a
// positioning variant of the non-dim, non-modal overlay family documented in
// docs/design/tui-modal-compositing.md ("transient hints" is the doc's own
// example use case), not a parallel compositor.
//
// Used by the floating notification bubble (bt-kuvzj, pkg/ui/toast_bubble.go)
// to place a yazi-style toast independently of the footer, so it never
// competes with footer content for width — the root cause of bt-8scek's
// truncation, where the old embedded toast borrowed the footer's right zone
// and got ansi.Truncate'd.
func OverlayBottomRight(bg, fg string, bgWidth, bgHeight, marginRight, marginBottom int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	fgWidth := 0
	for _, line := range fgLines {
		if w := ansi.StringWidth(line); w > fgWidth {
			fgWidth = w
		}
	}
	fgHeight := len(fgLines)

	startRow := bgHeight - fgHeight - marginBottom
	startCol := bgWidth - fgWidth - marginRight
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	// Pad every bg row to bgWidth so per-row slice positions are stable, same
	// defense as OverlayCenter (bt-l22b class).
	for i, line := range bgLines {
		if w := ansi.StringWidth(line); w < bgWidth {
			bgLines[i] = line + strings.Repeat(" ", bgWidth-w)
		}
	}

	for i, fgLine := range fgLines {
		bgRow := startRow + i
		if bgRow < 0 || bgRow >= len(bgLines) {
			continue
		}

		if w := ansi.StringWidth(fgLine); w < fgWidth {
			fgLine = fgLine + strings.Repeat(" ", fgWidth-w)
		}

		bgLine := bgLines[bgRow]
		left, right := sliceBgRow(bgLine, startCol, fgWidth, bgWidth, identityPad)
		bgLines[bgRow] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}

// OverlayCenterDimBackdrop composites fg centered on top of bg like
// OverlayCenter, but additionally dims the entire visible bg so the modal
// reads as a true pop-up rather than a content-shaped panel embedded in the
// underlying view (bt-v8he).
//
// Visual goal: the modal renders at a content-comfortable width while the
// surrounding cells visually recede. This satisfies both the bleed-through
// guard from bt-l5xu (background can no longer compete for attention with the
// modal) and the pop-up aesthetic — the modal needn't span the terminal.
//
// Implementation: every bg line is stripped of its existing ANSI styling and
// re-rendered through a Faint+Muted style before the fg is composited on top.
// We strip rather than wrap because SGR is stateful — a naive Faint wrapper
// would be unset by any inline `\x1b[0m` reset within the bg line, leaving
// patches of un-dimmed text. Stripping yields a uniform receded backdrop;
// the modal's own styling remains intact since it is composited last.
func OverlayCenterDimBackdrop(bg, fg string, bgWidth, bgHeight int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	fgWidth := 0
	for _, line := range fgLines {
		if w := ansi.StringWidth(line); w > fgWidth {
			fgWidth = w
		}
	}
	fgHeight := len(fgLines)

	startRow := (bgHeight - fgHeight) / 2
	startCol := (bgWidth - fgWidth) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	// Faint+Muted style for the receded backdrop. Faint (SGR 2) lowers
	// intensity; the muted foreground gives the cells a recognizable but
	// unobtrusive tint. ColorMuted is reused from the panel's own muted-border
	// path to keep theme propagation consistent.
	dim := lipgloss.NewStyle().Faint(true).Foreground(ColorMuted)

	for i := range bgLines {
		// Strip any pre-existing ANSI styling so the dim wrap is uniform —
		// mid-line resets cannot punch holes in the receded look.
		plain := ansi.Strip(bgLines[i])
		// Pad to bgWidth so per-row slice positions are stable. Without
		// this, rows whose underlying content is shorter than bgWidth
		// cause ansi.Truncate / ansi.TruncateLeft to position the fg at
		// the wrong absolute column on that row, drifting the modal's
		// vertical borders across rows (bt-l22b). The dim wrap applies
		// uniformly to padding and content so the backdrop reads as one
		// receded surface.
		if w := ansi.StringWidth(plain); w < bgWidth {
			plain = plain + strings.Repeat(" ", bgWidth-w)
		}
		bgLines[i] = dim.Render(plain)
	}

	// Composite the modal onto the dimmed backdrop. The modal lines retain
	// their own styling because dim.Render only wraps the bg; we slice the
	// dimmed line, drop in the un-dimmed fg, and re-attach the dimmed right
	// region.
	for i, fgLine := range fgLines {
		bgRow := startRow + i
		if bgRow < 0 || bgRow >= len(bgLines) {
			continue
		}

		// Pad fg row to fgWidth so the right bg slice resumes immediately
		// after the modal's actual right edge (bt-l22b hypothesis #3).
		// Mirrors the same defense in OverlayCenter above.
		if w := ansi.StringWidth(fgLine); w < fgWidth {
			fgLine = fgLine + strings.Repeat(" ", fgWidth-w)
		}

		bgLine := bgLines[bgRow]
		// ANSI-aware slicing with mid-glyph compensation (bt-3ykii). Padding
		// is wrapped in dim.Render so the synthesized spaces blend into the
		// receded backdrop visually.
		left, right := sliceBgRow(bgLine, startCol, fgWidth, bgWidth, func(s string) string { return dim.Render(s) })

		bgLines[bgRow] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}
