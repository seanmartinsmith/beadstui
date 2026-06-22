package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// ShortcutsSidebar provides a toggleable panel showing context-aware keyboard
// shortcuts. Unlike the help overlay, this can remain visible while working
// (bv-3qi5).
//
// Per ADR-004 (bt-ift6.10) it renders FullHelp() binding groups supplied by the
// active view's / modal's key.Map via SetBindings, not a hardcoded string
// table. The same key.Map source drives the L1 footer hint (ShortHelp) and the
// ? overlay (FullHelp), so the three help surfaces cannot drift.
type ShortcutsSidebar struct {
	width        int
	height       int
	scrollOffset int
	theme        Theme
	groups       [][]key.Binding // FullHelp() groups for the active view/modal
}

// NewShortcutsSidebar creates a new shortcuts sidebar
func NewShortcutsSidebar(theme Theme) ShortcutsSidebar {
	return ShortcutsSidebar{
		theme: theme,
		width: 34, // Fixed width for sidebar (increased for readability)
	}
}

// SetSize updates the sidebar dimensions
func (s *ShortcutsSidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// SetBindings sets the FullHelp() binding groups the sidebar renders. The model
// composes these from the active view's key.Map (GlobalKeys ++ the view Map, or
// the active modal's Map alone) via Model.sidebarHelpGroups, keeping the sidebar
// a pure consumer of the single key.Map source.
func (s *ShortcutsSidebar) SetBindings(groups [][]key.Binding) {
	s.groups = groups
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

// View renders the sidebar
func (s *ShortcutsSidebar) View() string {
	t := s.theme

	keyStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Width(8)

	descStyle := lipgloss.NewStyle().
		Foreground(t.Base.GetForeground())

	dimStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Italic(true)

	// Build content from the active key.Map's FullHelp() groups: each group is a
	// vertical section (one binding per row, key + desc columns) rendered as a
	// custom vertical layout (help.FullHelpView's horizontal columns are
	// illegible at this 34-col width, per ADR-004 Decision 1). Groups are
	// separated by a blank line. Disabled bindings and bindings without help
	// text are skipped so the surface stays truthful to the active state. The
	// "Shortcuts" title lives in the panel's top border (RenderTitledPanel).
	var sb strings.Builder
	firstGroup := true
	for _, group := range s.groups {
		rows := make([]string, 0, len(group))
		for _, b := range group {
			if !b.Enabled() {
				continue
			}
			h := b.Help()
			if h.Key == "" {
				continue
			}
			rows = append(rows, keyStyle.Render(h.Key)+descStyle.Render(h.Desc))
		}
		if len(rows) == 0 {
			continue
		}
		if !firstGroup {
			sb.WriteString("\n") // blank line between groups
		}
		firstGroup = false
		for _, r := range rows {
			sb.WriteString(r + "\n")
		}
	}

	fullContent := strings.TrimRight(sb.String(), "\n")
	lines := strings.Split(fullContent, "\n")
	totalLines := len(lines)

	// Reserve rows for the panel's top + bottom border (2) and the scroll/hide
	// footer hint (1).
	availableHeight := s.height - 3
	if availableHeight < 5 {
		availableHeight = 5
	}

	maxScroll := totalLines - availableHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if s.scrollOffset > maxScroll {
		s.scrollOffset = maxScroll
	}

	startLine := s.scrollOffset
	endLine := startLine + availableHeight
	if endLine > totalLines {
		endLine = totalLines
	}
	visibleContent := strings.Join(lines[startLine:endLine], "\n")

	var footer string
	if totalLines > availableHeight {
		scrollPercent := 0
		if maxScroll > 0 {
			scrollPercent = s.scrollOffset * 100 / maxScroll
		}
		footer = dimStyle.Render(fmt.Sprintf("ctrl+j/k scroll %d%%", scrollPercent))
	} else {
		footer = dimStyle.Render("; hide")
	}

	content := visibleContent + "\n" + footer

	// Match Issues/Details chrome — rounded borders + title-in-border (bt-lin9).
	// Title is centered to differentiate the auxiliary sidebar from the
	// content panels (Issues right-labeled, Details left-titled).
	return RenderTitledPanel(content, PanelOpts{
		Title:       "Shortcuts",
		CenterTitle: true,
		Width:       s.width,
		Height:      s.height,
	})
}
