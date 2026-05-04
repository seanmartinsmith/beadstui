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

	// Colors: use overrides when provided, otherwise derive from focus state
	var borderColor, titleColor color.Color
	if opts.BorderColor != nil {
		borderColor = opts.BorderColor
	} else if opts.Focused {
		borderColor = ColorPrimary
	} else {
		borderColor = ColorBgHighlight
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
		// Pad or truncate each line to innerWidth
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth {
			line = line + strings.Repeat(" ", innerWidth-lineWidth)
		} else if lineWidth > innerWidth {
			line = runewidth.Truncate(line, innerWidth, "")
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

	for i, fgLine := range fgLines {
		bgRow := startRow + i
		if bgRow < 0 || bgRow >= len(bgLines) {
			continue
		}

		bgLine := bgLines[bgRow]

		// ANSI-aware slicing: keep styling intact in left/right bg regions
		left := ansi.Truncate(bgLine, startCol, "")
		right := ansi.TruncateLeft(bgLine, startCol+fgWidth, "")

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

		bgLine := bgLines[bgRow]
		left := ansi.Truncate(bgLine, startCol, "")
		right := ansi.TruncateLeft(bgLine, startCol+fgWidth, "")

		bgLines[bgRow] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}
