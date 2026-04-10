package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/lipgloss/tree"
)

// TutorialElement is the interface for renderable tutorial content
type TutorialElement interface {
	Render(theme Theme, width int) string
}

// Paragraph is a simple text paragraph
type Paragraph struct {
	Text string
}

func (p Paragraph) Render(theme Theme, width int) string {
	style := theme.Renderer.NewStyle().
		Width(width).
		Foreground(theme.Base.GetForeground())
	return style.Render(p.Text)
}

// Section is a styled section header with subtle underline decoration
type Section struct {
	Title string
}

func (s Section) Render(theme Theme, width int) string {
	r := theme.Renderer

	// Title with bold styling
	titleStyle := r.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	title := titleStyle.Render(s.Title)

	// Subtle underline using a thin line that spans the title width
	lineWidth := lipgloss.Width(s.Title)
	if lineWidth < 3 {
		lineWidth = 3
	}

	underlineStyle := r.NewStyle().
		Foreground(theme.Muted)

	underline := underlineStyle.Render(strings.Repeat("─", lineWidth))

	return title + "\n" + underline
}

// KeyBinding represents a single key binding
type KeyBinding struct {
	Key  string
	Desc string
}

// KeyTable renders a table of key bindings using lipgloss/table for proper alignment
type KeyTable struct {
	Bindings []KeyBinding
}

func (kt KeyTable) Render(theme Theme, width int) string {
	if len(kt.Bindings) == 0 {
		return ""
	}

	// Build rows from bindings
	rows := make([][]string, len(kt.Bindings))
	for i, b := range kt.Bindings {
		rows[i] = []string{b.Key, b.Desc}
	}

	// Create a styled table using lipgloss/table
	t := table.New().
		Rows(rows...).
		Border(lipgloss.HiddenBorder()).
		Width(width - 2).
		StyleFunc(func(row, col int) lipgloss.Style {
			// Key column styling
			if col == 0 {
				return theme.Renderer.NewStyle().
					Foreground(theme.Primary).
					Bold(true).
					Width(18).
					PaddingRight(1)
			}
			// Description column - alternating subtle background for readability
			baseStyle := theme.Renderer.NewStyle().
				Foreground(theme.Base.GetForeground())

			// Subtle alternating row colors for better visual scanning
			if row%2 == 0 {
				return baseStyle.Background(lipgloss.AdaptiveColor{Light: "#F8F8F8", Dark: "#2D2D2D"})
			}
			return baseStyle
		})

	return t.Render()
}

// Tip renders a highlighted tip/note box with lightbulb icon
type Tip struct {
	Text string
}

func (t Tip) Render(theme Theme, width int) string {
	r := theme.Renderer

	boxStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Feature).
		Padding(0, 1).
		Width(width - 2).
		Foreground(theme.Base.GetForeground())

	// Lightbulb icon with bold TIP label
	iconStyle := r.NewStyle().
		Foreground(theme.Feature).
		Bold(true)

	icon := iconStyle.Render("💡 TIP  ")

	return boxStyle.Render(icon + t.Text)
}

// StatusFlow renders a status flow diagram using lipgloss boxes with elegant arrows
type StatusFlow struct {
	Steps []FlowStep
}

type FlowStep struct {
	Label string
	Color lipgloss.AdaptiveColor
}

func (sf StatusFlow) Render(theme Theme, width int) string {
	r := theme.Renderer

	numSteps := len(sf.Steps)
	if numSteps == 0 {
		return ""
	}

	// Arrow style using proper arrow character
	arrowStyle := r.NewStyle().
		Foreground(theme.Muted).
		Bold(true)

	var boxes []string
	for i, step := range sf.Steps {
		boxStyle := r.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(step.Color).
			Foreground(step.Color).
			Padding(0, 1).
			Align(lipgloss.Center)

		boxes = append(boxes, boxStyle.Render(step.Label))

		// Add arrow between boxes (not after last) - use proper arrow character
		if i < numSteps-1 {
			boxes = append(boxes, arrowStyle.Render(" → "))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, boxes...)
}

// Code renders a code block with left accent border
type Code struct {
	Text string
}

func (c Code) Render(theme Theme, width int) string {
	r := theme.Renderer

	// Create a styled code block with left accent border
	accentBorder := lipgloss.Border{
		Left: "│",
	}

	style := r.NewStyle().
		Foreground(theme.Open).
		Background(lipgloss.AdaptiveColor{Light: "#F5F5F5", Dark: "#282A36"}).
		Border(accentBorder).
		BorderForeground(theme.Primary).
		PaddingLeft(1).
		PaddingRight(1).
		Width(width - 4)

	return style.Render(c.Text)
}

// Bullet renders a bulleted list with elegant bullet characters
type Bullet struct {
	Items []string
}

func (b Bullet) Render(theme Theme, width int) string {
	r := theme.Renderer

	// Bullet character in primary color
	bulletStyle := r.NewStyle().
		Foreground(theme.Primary)

	// Text with proper wrapping
	textStyle := r.NewStyle().
		Foreground(theme.Base.GetForeground()).
		Width(width - 4)

	var lines []string
	for _, item := range b.Items {
		bullet := bulletStyle.Render("  • ")
		text := textStyle.Render(item)
		lines = append(lines, bullet+text)
	}
	return strings.Join(lines, "\n")
}

// Spacer adds vertical space
type Spacer struct {
	Lines int
}

func (s Spacer) Render(theme Theme, width int) string {
	if s.Lines <= 0 {
		s.Lines = 1
	}
	return strings.Repeat("\n", s.Lines-1)
}

// Divider renders a horizontal divider line for visual separation
type Divider struct{}

func (d Divider) Render(theme Theme, width int) string {
	r := theme.Renderer

	lineStyle := r.NewStyle().
		Foreground(theme.Muted)

	lineWidth := width - 4
	if lineWidth < 10 {
		lineWidth = 10
	}

	return lineStyle.Render(strings.Repeat("─", lineWidth))
}

// Tree renders a hierarchical tree structure using lipgloss/tree
type Tree struct {
	Root     string
	Children []TutorialTreeNode
}

// TutorialTreeNode is a simple tree node for tutorial rendering.
// Named to avoid collision with TreeNode in tree.go (bv-gllx feature).
type TutorialTreeNode struct {
	Label    string
	Children []TutorialTreeNode
}

func (t Tree) Render(theme Theme, width int) string {
	r := theme.Renderer

	// Style for the tree items
	itemStyle := r.NewStyle().
		Foreground(theme.Base.GetForeground())

	// Style for the root
	rootStyle := r.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	// Style for the enumerators (├── └──)
	enumStyle := r.NewStyle().
		Foreground(theme.Muted)

	// Build the tree
	tr := tree.Root(rootStyle.Render(t.Root)).
		EnumeratorStyle(enumStyle).
		ItemStyle(itemStyle)

	// Add children recursively
	for _, child := range t.Children {
		tr = tr.Child(buildTreeNode(child, itemStyle, enumStyle))
	}

	return tr.String()
}

func buildTreeNode(node TutorialTreeNode, itemStyle, enumStyle lipgloss.Style) *tree.Tree {
	t := tree.Root(itemStyle.Render(node.Label)).
		EnumeratorStyle(enumStyle).
		ItemStyle(itemStyle)

	for _, child := range node.Children {
		t = t.Child(buildTreeNode(child, itemStyle, enumStyle))
	}

	return t
}

// InfoBox renders a highlighted info box with title and content
type InfoBox struct {
	Title   string
	Content string
	Color   lipgloss.AdaptiveColor
}

func (ib InfoBox) Render(theme Theme, width int) string {
	r := theme.Renderer

	titleStyle := r.NewStyle().
		Foreground(ib.Color).
		Bold(true)

	contentStyle := r.NewStyle().
		Foreground(theme.Base.GetForeground())

	boxStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ib.Color).
		Padding(0, 1).
		Width(width - 2)

	content := titleStyle.Render(ib.Title) + "\n" + contentStyle.Render(ib.Content)
	return boxStyle.Render(content)
}

// ValueProp renders a value proposition with styled number icon
type ValueProp struct {
	Icon string
	Text string
}

func (vp ValueProp) Render(theme Theme, width int) string {
	r := theme.Renderer

	// Icon/number in a small rounded box for visual pop
	iconStyle := r.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		Width(4)

	// Text with proper wrapping
	textStyle := r.NewStyle().
		Foreground(theme.Base.GetForeground()).
		Width(width - 6)

	return iconStyle.Render(vp.Icon) + textStyle.Render(vp.Text)
}

// Warning renders a warning box with alert styling
type Warning struct {
	Text string
}

func (w Warning) Render(theme Theme, width int) string {
	r := theme.Renderer

	boxStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Blocked).
		Padding(0, 1).
		Width(width - 2).
		Foreground(theme.Base.GetForeground())

	iconStyle := r.NewStyle().
		Foreground(theme.Blocked).
		Bold(true)

	icon := iconStyle.Render("⚠️  WARN ")

	return boxStyle.Render(icon + w.Text)
}

// Note renders an info-style note box
type Note struct {
	Text string
}

func (n Note) Render(theme Theme, width int) string {
	r := theme.Renderer

	boxStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.InProgress).
		Padding(0, 1).
		Width(width - 2).
		Foreground(theme.Base.GetForeground())

	iconStyle := r.NewStyle().
		Foreground(theme.InProgress).
		Bold(true)

	icon := iconStyle.Render("ℹ️  NOTE ")

	return boxStyle.Render(icon + n.Text)
}

// StyledTable renders a full table with headers using lipgloss/table
type StyledTable struct {
	Headers []string
	Rows    [][]string
}

func (st StyledTable) Render(theme Theme, width int) string {
	if len(st.Rows) == 0 {
		return ""
	}

	t := table.New().
		Headers(st.Headers...).
		Rows(st.Rows...).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(theme.Renderer.NewStyle().Foreground(theme.Border)).
		Width(width - 2).
		StyleFunc(func(row, col int) lipgloss.Style {
			// Header row styling
			if row == table.HeaderRow {
				return theme.Renderer.NewStyle().
					Foreground(theme.Primary).
					Bold(true).
					Align(lipgloss.Center).
					Padding(0, 1)
			}

			// Data rows with alternating colors
			baseStyle := theme.Renderer.NewStyle().
				Foreground(theme.Base.GetForeground()).
				Padding(0, 1)

			if row%2 == 0 {
				return baseStyle.Background(lipgloss.AdaptiveColor{Light: "#F8F8F8", Dark: "#2D2D2D"})
			}
			return baseStyle
		})

	return t.Render()
}

// ProgressIndicator shows a visual progress indicator (dots or bar style)
type ProgressIndicator struct {
	Current int
	Total   int
	Label   string
}

func (pi ProgressIndicator) Render(theme Theme, width int) string {
	r := theme.Renderer

	// Calculate progress
	if pi.Total <= 0 {
		pi.Total = 1
	}
	progress := float64(pi.Current) / float64(pi.Total)
	if progress > 1 {
		progress = 1
	}

	// Label
	labelStyle := r.NewStyle().
		Foreground(theme.Muted)

	// Progress bar - use lipgloss.Width for proper Unicode width calculation
	labelWidth := lipgloss.Width(pi.Label)
	barWidth := width - labelWidth - 10
	if barWidth < 10 {
		barWidth = 10
	}

	filledWidth := int(float64(barWidth) * progress)
	emptyWidth := barWidth - filledWidth

	filledStyle := r.NewStyle().
		Foreground(theme.Open).
		Background(theme.Open)

	emptyStyle := r.NewStyle().
		Foreground(theme.Muted).
		Background(lipgloss.AdaptiveColor{Light: "#E0E0E0", Dark: "#3D3D3D"})

	filled := filledStyle.Render(strings.Repeat("█", filledWidth))
	empty := emptyStyle.Render(strings.Repeat("░", emptyWidth))

	// Percentage - format as right-aligned 3-digit number with %
	pctStyle := r.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	pctValue := int(progress * 100)
	pct := pctStyle.Render(fmt.Sprintf("%3d%%", pctValue))

	return labelStyle.Render(pi.Label+" ") + filled + empty + " " + pct
}

// Highlight renders inline highlighted/emphasized text
type Highlight struct {
	Text  string
	Color lipgloss.AdaptiveColor
}

func (h Highlight) Render(theme Theme, width int) string {
	style := theme.Renderer.NewStyle().
		Foreground(h.Color).
		Bold(true)

	return style.Render(h.Text)
}

// renderElements renders a slice of elements with proper spacing
func renderElements(elements []TutorialElement, theme Theme, width int) string {
	var parts []string
	for _, elem := range elements {
		parts = append(parts, elem.Render(theme, width))
	}
	return strings.Join(parts, "\n")
}
