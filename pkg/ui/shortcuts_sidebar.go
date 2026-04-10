package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ShortcutsSidebar provides a toggleable panel showing context-aware keyboard shortcuts
// Unlike the help overlay, this can remain visible while working (bv-3qi5)
type ShortcutsSidebar struct {
	width        int
	height       int
	scrollOffset int
	theme        Theme
	context      string // Current context for filtering shortcuts
}

// shortcutItem represents a single keyboard shortcut
type shortcutItem struct {
	key  string
	desc string
}

// shortcutSection groups shortcuts by category
type shortcutSection struct {
	title    string
	items    []shortcutItem
	contexts []string // Which contexts this section applies to (empty = all)
}

// NewShortcutsSidebar creates a new shortcuts sidebar
func NewShortcutsSidebar(theme Theme) ShortcutsSidebar {
	return ShortcutsSidebar{
		theme:   theme,
		width:   34, // Fixed width for sidebar (increased for readability)
		context: "list",
	}
}

// SetSize updates the sidebar dimensions
func (s *ShortcutsSidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// SetContext updates the current context for filtering shortcuts
func (s *ShortcutsSidebar) SetContext(ctx string) {
	s.context = ctx
}

// ScrollUp scrolls the sidebar content up
func (s *ShortcutsSidebar) ScrollUp() {
	if s.scrollOffset > 0 {
		s.scrollOffset--
	}
}

// ScrollDown scrolls the sidebar content down
func (s *ShortcutsSidebar) ScrollDown() {
	s.scrollOffset++
}

// ScrollPageUp scrolls up by a page
func (s *ShortcutsSidebar) ScrollPageUp() {
	s.scrollOffset -= 10
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
}

// ScrollPageDown scrolls down by a page
func (s *ShortcutsSidebar) ScrollPageDown() {
	s.scrollOffset += 10
}

// ResetScroll resets scroll position to top
func (s *ShortcutsSidebar) ResetScroll() {
	s.scrollOffset = 0
}

// Width returns the fixed width of the sidebar
func (s *ShortcutsSidebar) Width() int {
	return s.width
}

// allSections returns all shortcut sections with their contexts
func (s *ShortcutsSidebar) allSections() []shortcutSection {
	return []shortcutSection{
		{
			title:    "Navigation",
			contexts: []string{}, // All contexts
			items: []shortcutItem{
				{"j/k", "Move ↓/↑"},
				{"G/gg", "End/Start"},
				{"^d/^u", "Page ↓/↑"},
				{"Enter", "Details"},
				{"Esc", "Back"},
			},
		},
		{
			title:    "Views",
			contexts: []string{"list", "detail", "split"},
			items: []shortcutItem{
				{"a", "Actionable"},
				{"b", "Board"},
				{"g", "Graph"},
				{"h", "History"},
				{"i", "Insights"},
				{"?", "Help"},
				{";", "This sidebar"},
				{"p", "Priority hints"},
			},
		},
		{
			title:    "Graph",
			contexts: []string{"graph"},
			items: []shortcutItem{
				{"hjkl", "Navigate"},
				{"H/L", "Scroll ←/→"},
				{"PgUp/Dn", "Scroll ↑/↓"},
				{"Enter", "Jump to issue"},
			},
		},
		{
			title:    "Insights",
			contexts: []string{"insights"},
			items: []shortcutItem{
				{"h/l", "Switch panel"},
				{"j/k", "Select item"},
				{"^j/^k", "Scroll detail"},
				{"e", "Explanations"},
				{"x", "Calc proof"},
				{"m", "Heatmap"},
				{"Enter", "Jump to issue"},
			},
		},
		{
			title:    "History",
			contexts: []string{"history"},
			items: []shortcutItem{
				{"v", "Git/Bead mode"},
				{"/", "Search"},
				{"j/k", "Navigate ↓/↑"},
				{"J/K", "Detail ↓/↑"},
				{"Tab", "Focus toggle"},
				{"y", "Copy SHA"},
				{"o", "Open in browser"},
				{"g", "Graph view"},
				{"c", "Cycle filter"},
			},
		},
		{
			title:    "Board",
			contexts: []string{"board"},
			items: []shortcutItem{
				{"h/l", "Columns ←/→"},
				{"j/k", "Items ↓/↑"},
				{"Tab", "Toggle detail"},
				{"y", "Copy ID"},
				{"^j/^k", "Scroll detail"},
				{"Enter", "Full view"},
			},
		},
		{
			title:    "Filters",
			contexts: []string{"list", "split"},
			items: []shortcutItem{
				{"o", "Open only"},
				{"c", "Closed only"},
				{"r", "Ready (no blocks)"},
				{"l", "Label picker"},
				{"/", "Search"},
			},
		},
		{
			title:    "Actions",
			contexts: []string{"list", "detail", "split"},
			items: []shortcutItem{
				{"t/T", "Time-travel"},
				{"x", "Export .md"},
				{"y", "Copy ID"},
				{"C", "Copy"},
				{"O", "Open in $EDITOR"},
				{"'", "Recipe picker"},
				{"U", "Self-update"},
				{"V", "Cass sessions"},
			},
		},
	}
}

// View renders the sidebar
func (s *ShortcutsSidebar) View() string {
	t := s.theme

	// Styles
	titleStyle := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Align(lipgloss.Center).
		Width(s.width - 4)

	sectionStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Bold(true).
		MarginTop(1)

	keyStyle := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#BD93F9"}).
		Bold(true).
		Width(8)

	descStyle := t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground())

	dimStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Italic(true)

	// Build content
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Shortcuts"))
	sb.WriteString("\n")

	// Filter sections by context
	sections := s.allSections()
	for _, section := range sections {
		// Check if this section applies to current context
		if len(section.contexts) > 0 {
			found := false
			for _, ctx := range section.contexts {
				if ctx == s.context {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		sb.WriteString(sectionStyle.Render(section.title))
		sb.WriteString("\n")

		for _, item := range section.items {
			line := keyStyle.Render(item.key) + descStyle.Render(item.desc)
			sb.WriteString(line + "\n")
		}
	}

	// Build content lines for scrolling
	fullContent := sb.String()
	lines := strings.Split(fullContent, "\n")
	totalLines := len(lines)

	// Calculate visible area
	availableHeight := s.height - 4 // Reserve for border/padding and hint
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Clamp scroll
	maxScroll := totalLines - availableHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if s.scrollOffset > maxScroll {
		s.scrollOffset = maxScroll
	}

	// Extract visible lines
	startLine := s.scrollOffset
	endLine := startLine + availableHeight
	if endLine > totalLines {
		endLine = totalLines
	}
	visibleLines := lines[startLine:endLine]
	visibleContent := strings.Join(visibleLines, "\n")

	// Add scroll hint if needed
	var footer string
	if totalLines > availableHeight {
		scrollPercent := 0
		if maxScroll > 0 {
			scrollPercent = s.scrollOffset * 100 / maxScroll
		}
		footer = dimStyle.Render(fmt.Sprintf("j/k scroll %d%%", scrollPercent))
	} else {
		footer = dimStyle.Render("; hide")
	}

	// Combine content and footer
	content := visibleContent + "\n" + footer

	// Create the sidebar box
	boxStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Secondary).
		Padding(0, 1).
		Width(s.width).
		Height(s.height - 1).
		MaxHeight(s.height - 1)

	return boxStyle.Render(content)
}

// contextFromFocus returns the context string for the current focus
func ContextFromFocus(f focus) string {
	switch f {
	case focusList:
		return "list"
	case focusDetail:
		return "detail"
	case focusBoard:
		return "board"
	case focusGraph:
		return "graph"
	case focusInsights:
		return "insights"
	case focusHistory:
		return "history"
	case focusActionable:
		return "actionable"
	case focusLabelDashboard:
		return "label"
	default:
		return "list"
	}
}
