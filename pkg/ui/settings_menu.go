package ui

// The esc menu (bt-54c3), the layer btop shows before its options screen:
// a short centred list over a dimmed backdrop.
//
// Every entry routes to a surface bt already has. That is deliberate -- the
// menu is a discovery affordance, not a second implementation of help or quit.
// It exists because esc is the key a user presses when they do not know what
// else to press, so it should answer "what can I do here" rather than only
// "what did I mean to close".
//
// It also restores the quit confirmation. Moving esc to this menu orphaned
// ModalQuitConfirm, since esc was its only route; QUIT below is that route now.

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// menuEntry is one row of the esc menu.
type menuEntry struct {
	label string
	desc  string
}

// SettingsMenuModel is the esc menu.
type SettingsMenuModel struct {
	entries  []menuEntry
	selected int
	width    int
	height   int
	theme    Theme
}

// Menu entry indices. Named rather than positional so the key handler does not
// depend on the order they render in.
const (
	menuOptions = iota
	menuHelp
	menuQuit
)

// NewSettingsMenuModel builds the esc menu.
func NewSettingsMenuModel(theme Theme) SettingsMenuModel {
	return SettingsMenuModel{
		entries: []menuEntry{
			{label: "OPTIONS", desc: "theme and display settings"},
			{label: "HELP", desc: "keybindings and views"},
			{label: "QUIT", desc: "leave bt"},
		},
		theme: theme,
	}
}

// SetSize updates the menu dimensions.
func (s *SettingsMenuModel) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// SetTheme re-points the menu at a new theme.
func (s *SettingsMenuModel) SetTheme(t Theme) { s.theme = t }

// Reset returns the selection to the top, so reopening does not resume on
// whatever was last chosen -- OPTIONS is the reason to open this.
func (s *SettingsMenuModel) Reset() { s.selected = 0 }

// MoveUp moves the selection up, wrapping. A three-entry list is short enough
// that wrapping is faster than stopping at the ends.
func (s *SettingsMenuModel) MoveUp() {
	s.selected--
	if s.selected < 0 {
		s.selected = len(s.entries) - 1
	}
}

// MoveDown moves the selection down, wrapping.
func (s *SettingsMenuModel) MoveDown() {
	s.selected++
	if s.selected >= len(s.entries) {
		s.selected = 0
	}
}

// SelectedIndex returns the focused entry index.
func (s *SettingsMenuModel) SelectedIndex() int { return s.selected }

// View renders the menu. Composited by OverlayCenterDimBackdrop.
func (s *SettingsMenuModel) View() string {
	if s.width == 0 {
		s.width = 80
	}
	if s.height == 0 {
		s.height = 24
	}
	t := s.theme

	boxWidth := 34
	if boxWidth > s.width-4 {
		boxWidth = s.width - 4
	}
	if boxWidth < 18 {
		boxWidth = 18
	}

	selStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	idleStyle := lipgloss.NewStyle().Foreground(t.Subtext)
	descStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)
	footerStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)

	var lines []string
	lines = append(lines, "")
	for i, e := range s.entries {
		prefix := "    "
		style := idleStyle
		if i == s.selected {
			prefix = "  ▸ "
			style = selStyle
		}
		lines = append(lines, prefix+style.Render(e.label))
		lines = append(lines, "      "+descStyle.Render(truncateRunesHelper(e.desc, boxWidth-8, "…")))
		if i < len(s.entries)-1 {
			lines = append(lines, "")
		}
	}
	lines = append(lines, "")
	lines = append(lines, "  "+footerStyle.Render("j/k • enter • esc"))
	lines = append(lines, "")

	return RenderTitledPanel(strings.Join(lines, "\n"), PanelOpts{
		Title:   "bt",
		Width:   boxWidth,
		Height:  len(lines) + 2,
		Focused: true,
	})
}
