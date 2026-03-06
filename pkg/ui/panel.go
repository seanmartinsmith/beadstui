package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// BorderVariant controls the weight of box-drawing characters.
type BorderVariant int

const (
	BorderNormal BorderVariant = iota // ┌─┐│└┘
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
}

// borderChars returns the box-drawing characters for a variant.
func borderChars(v BorderVariant) (tl, tr, bl, br, h, vert string) {
	switch v {
	case BorderThick:
		return "┏", "┓", "┗", "┛", "━", "┃"
	case BorderDouble:
		return "╔", "╗", "╚", "╝", "═", "║"
	default:
		return "┌", "┐", "└", "┘", "─", "│"
	}
}

// RenderTitledPanel draws a box with the title inlined in the top border.
// The content is placed inside with no extra padding beyond the border itself.
//
//	┌─ Title ──────────────────┐
//	│ content                  │
//	└──────────────────────────┘
func RenderTitledPanel(r *lipgloss.Renderer, content string, opts PanelOpts) string {
	if opts.Width < 4 {
		opts.Width = 4
	}

	tl, tr, bl, br, h, vert := borderChars(opts.Variant)

	// Colors
	var borderColor, titleColor lipgloss.AdaptiveColor
	if opts.Focused {
		borderColor = ColorPrimary
		titleColor = ColorPrimary
	} else {
		borderColor = ColorBgHighlight
		titleColor = ColorMuted
	}

	borderStyle := r.NewStyle().Foreground(borderColor)
	titleStyle := r.NewStyle().Foreground(titleColor)
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

		top.WriteString(borderStyle.Render(h + " "))
		top.WriteString(titleStyle.Render(titleText))
		top.WriteString(borderStyle.Render(" "))

		// Fill remaining with horizontal line
		used := 2 + titleDisplayWidth + 1 // "─ " + title + " "
		remaining := innerWidth - used
		if remaining > 0 {
			top.WriteString(borderStyle.Render(strings.Repeat(h, remaining)))
		}
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
