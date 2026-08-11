package ui

// The settings screen (bt-54c3, authorised by the 2026-08-11 amendment to
// bt-2aa49).
//
// Shape follows btop's options screen, which is the reference the maintainer
// asked for: a tab strip, a list of settings on the left showing each name
// above its current value, and a pane on the right describing whichever is
// selected. Left/right cycles the selected setting's value.
//
// Why a setting is an interface rather than a switch: this screen ships with
// one entry, and bt-gf3d.1's hotkey audit is expected to demote several
// low-frequency toggles into it. A switch would make each of those a new case
// in three places. A settingItem makes each one a value in a slice.

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// settingItem is one row of the settings screen.
//
// Value renders live rather than being cached at construction, because a
// setting that applies immediately -- the theme does -- would otherwise show
// the value it had when the screen opened.
type settingItem interface {
	Name() string
	Value() string
	Help() string
	// Prev and Next move the setting through its options. They apply the
	// change; there is no separate commit step, matching btop, where moving
	// through themes repaints the UI underneath the open menu.
	Prev(m *Model)
	Next(m *Model)
}

// themeSetting cycles the active palette across both corpora.
type themeSetting struct {
	names []string
	idx   int
}

func newThemeSetting() *themeSetting {
	names := ThemeNames()
	s := &themeSetting{names: names}
	// Start on whatever is actually rendering, so the screen opens showing the
	// user's current theme rather than the first alphabetically.
	current := SelectedThemeName()
	for i, n := range names {
		if n == current {
			s.idx = i
			break
		}
	}
	return s
}

func (s *themeSetting) Name() string { return "Color theme" }

// Value carries the position as well as the name. btop shows "19/43", which
// tells the user a corpus this size exists at all -- worth keeping, since
// bt ships 43 palettes and nothing else in the UI says so.
func (s *themeSetting) Value() string {
	if len(s.names) == 0 {
		return "(none)"
	}
	return s.names[s.idx]
}

func (s *themeSetting) Position() string {
	if len(s.names) == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", s.idx+1, len(s.names))
}

func (s *themeSetting) Help() string {
	return strings.Join([]string{
		"Set the color palette.",
		"",
		"bt ships two corpora. Names without a prefix are bt-authored,",
		"written directly against bt's own tokens. Names shown as",
		"\"btop:<name>\" come from the vendored upstream btop corpus and",
		"are adapted at load.",
		"",
		"A palette applies immediately so it can be judged against real",
		"data rather than a swatch. Any per-token tweaks in",
		"~/.config/bt/theme.yaml still layer on top and are never",
		"discarded by a selection here.",
		"",
		"BT_THEME=<name> selects one for a single run.",
	}, "\n")
}

func (s *themeSetting) Prev(m *Model) {
	if len(s.names) == 0 {
		return
	}
	s.idx--
	if s.idx < 0 {
		s.idx = len(s.names) - 1
	}
	m.applyThemeLive(s.names[s.idx])
}

func (s *themeSetting) Next(m *Model) {
	if len(s.names) == 0 {
		return
	}
	s.idx++
	if s.idx >= len(s.names) {
		s.idx = 0
	}
	m.applyThemeLive(s.names[s.idx])
}

// applyThemeLive swaps the active palette mid-session.
//
// Safe to call from Update without synchronisation: bt-1n0b1 established that
// the bubbletea event loop is the only reader of the Color* tokens and the
// styles built from them, and Update and View run consecutively on it.
//
// The real hazard is staleness, not races. Most sub-models are rebuilt on entry
// and pick up the new palette for free; the three below hold a Theme value copy
// or a style built from one and outlive the swap, so they need an explicit
// re-push or the list keeps rendering in the previous palette while everything
// around it changes.
func (m *Model) applyThemeLive(name string) {
	tf := LoadThemeNamed(name)
	ApplyThemeToGlobals(tf)
	ApplyThemeToThemeStruct(&m.theme, tf)

	// updateListDelegate rebuilds from current model state, so the delegate
	// picks up the new Theme along with the hint/claim state it already
	// carries. Reconstructing it here by hand would silently drop whichever
	// field gets added to IssueDelegate next.
	m.updateListDelegate()

	m.list.Styles.Filter.Focused.Prompt = lipgloss.NewStyle().Foreground(m.theme.Primary)
	m.list.Styles.Filter.Focused.Text = lipgloss.NewStyle().Foreground(m.theme.Primary)
	m.renderer = NewMarkdownRendererWithTheme(80, m.theme)

	// And the settings screen itself, so it repaints along with everything
	// underneath it rather than staying in the palette it opened in.
	m.settingsModal.SetTheme(m.theme)
}

// SettingsModalModel is the options screen.
type SettingsModalModel struct {
	tabs      []string
	activeTab int
	items     []settingItem
	selected  int
	width     int
	height    int
	theme     Theme

	// originalTheme is what was rendering when the screen opened, so esc can
	// put it back. btop commits on exit; reverting on cancel is the safer
	// default for a setting that repaints the whole UI as you scroll past it.
	originalTheme string
}

// NewSettingsModalModel builds the settings screen.
func NewSettingsModalModel(theme Theme) SettingsModalModel {
	return SettingsModalModel{
		// One tab today. The strip renders anyway: it is the affordance that
		// says more settings are coming, and bt-gf3d.1 will fill it.
		tabs:          []string{"general"},
		items:         []settingItem{newThemeSetting()},
		theme:         theme,
		originalTheme: SelectedThemeName(),
	}
}

// SetSize updates the screen dimensions.
func (s *SettingsModalModel) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// SetTheme re-points the screen at a new theme, so the settings screen itself
// repaints while the user cycles palettes underneath it.
func (s *SettingsModalModel) SetTheme(t Theme) { s.theme = t }

// Reset re-reads the current selection and re-arms the revert target. Called on
// open so a second visit does not offer to revert to a stale palette.
func (s *SettingsModalModel) Reset() {
	s.items = []settingItem{newThemeSetting()}
	s.selected = 0
	s.originalTheme = SelectedThemeName()
}

// OriginalTheme is the palette to restore if the user cancels.
func (s *SettingsModalModel) OriginalTheme() string { return s.originalTheme }

// SelectedThemeNameFromModal returns the palette currently selected on the
// screen, or "" if the theme setting is not present. Free function rather than
// a method because it reaches into the settingItem list, which is the screen's
// internal representation and should stay that way as more settings land.
func SelectedThemeNameFromModal(s *SettingsModalModel) string {
	for _, it := range s.items {
		if ts, ok := it.(*themeSetting); ok {
			return ts.Value()
		}
	}
	return ""
}

// MoveUp moves the selection up the settings list.
func (s *SettingsModalModel) MoveUp() {
	if s.selected > 0 {
		s.selected--
	}
}

// MoveDown moves the selection down the settings list.
func (s *SettingsModalModel) MoveDown() {
	if s.selected < len(s.items)-1 {
		s.selected++
	}
}

// Selected returns the focused setting, or nil when there are none.
func (s *SettingsModalModel) Selected() settingItem {
	if s.selected < 0 || s.selected >= len(s.items) {
		return nil
	}
	return s.items[s.selected]
}

// View renders the settings screen. Composited by OverlayCenterDimBackdrop in
// model_view.go, so it does not centre itself.
func (s *SettingsModalModel) View() string {
	if s.width == 0 {
		s.width = 80
	}
	if s.height == 0 {
		s.height = 24
	}
	t := s.theme

	// Two columns like btop: settings left, description right. The split is
	// proportional rather than fixed so the description still gets usable room
	// on a narrow terminal instead of wrapping every line.
	boxWidth := s.width * 3 / 4
	if boxWidth > 96 {
		boxWidth = 96
	}
	if boxWidth < 46 {
		boxWidth = 46
	}
	if boxWidth > s.width-4 {
		boxWidth = s.width - 4
	}

	boxHeight := s.height * 7 / 10
	if boxHeight < 12 {
		boxHeight = 12
	}
	if boxHeight > s.height-2 {
		boxHeight = s.height - 2
	}

	innerRows := boxHeight - 4
	if innerRows < 6 {
		innerRows = 6
	}

	leftWidth := boxWidth * 2 / 5
	if leftWidth < 22 {
		leftWidth = 22
	}
	rightWidth := boxWidth - leftWidth - 5
	if rightWidth < 16 {
		rightWidth = 16
	}

	var lines []string
	lines = append(lines, "  "+s.renderTabs())
	lines = append(lines, "")

	left := s.renderSettings(leftWidth)
	right := s.renderHelp(rightWidth)

	// Pad the shorter column so the two stay top-aligned and the panel does not
	// change height as the description length varies between settings.
	bodyRows := innerRows - 4
	if bodyRows < 4 {
		bodyRows = 4
	}
	for len(left) < bodyRows {
		left = append(left, "")
	}
	for len(right) < bodyRows {
		right = append(right, "")
	}
	if len(left) > bodyRows {
		left = left[:bodyRows]
	}
	if len(right) > bodyRows {
		right = right[:bodyRows]
	}

	divider := lipgloss.NewStyle().Foreground(t.Border).Render("│")
	for i := 0; i < bodyRows; i++ {
		l := left[i]
		pad := leftWidth - lipgloss.Width(l)
		if pad < 0 {
			pad = 0
		}
		lines = append(lines, "  "+l+strings.Repeat(" ", pad)+" "+divider+" "+right[i])
	}

	footer := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true).
		Render("j/k: select • ←/→: change • enter: keep • esc: cancel")
	lines = append(lines, "")
	lines = append(lines, "  "+footer)

	return RenderTitledPanel(strings.Join(lines, "\n"), PanelOpts{
		Title:   "Options",
		Width:   boxWidth,
		Height:  boxHeight,
		Focused: true,
	})
}

func (s *SettingsModalModel) renderTabs() string {
	t := s.theme
	active := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	idle := lipgloss.NewStyle().Foreground(t.Subtext)

	out := make([]string, 0, len(s.tabs))
	for i, name := range s.tabs {
		if i == s.activeTab {
			out = append(out, active.Render("["+name+"]"))
			continue
		}
		out = append(out, idle.Render(fmt.Sprintf("%d%s", i+1, name)))
	}
	return strings.Join(out, "   ")
}

// renderSettings renders the left column: each setting's name with its current
// value beneath, the selected one highlighted.
func (s *SettingsModalModel) renderSettings(width int) []string {
	t := s.theme
	// Filled bar for the selected setting, matching btop. Foreground comes from
	// the theme background so the label stays legible whatever Primary is --
	// several palettes in the corpus have a near-white Primary, where white-on-
	// Primary would vanish.
	nameSel := lipgloss.NewStyle().Foreground(ColorBg).Background(t.Primary).Bold(true)
	nameIdle := lipgloss.NewStyle().Foreground(t.Base.GetForeground()).Bold(true)
	valSel := lipgloss.NewStyle().Foreground(t.Primary)
	valIdle := lipgloss.NewStyle().Foreground(t.Subtext)

	var out []string
	for i, it := range s.items {
		selected := i == s.selected

		label := it.Name()
		if p, ok := it.(interface{ Position() string }); ok {
			if pos := p.Position(); pos != "" {
				label += " " + pos
			}
		}
		if selected {
			out = append(out, nameSel.Render(" "+truncateRunesHelper(label, width-2, "…")+" "))
		} else {
			out = append(out, nameIdle.Render(" "+truncateRunesHelper(label, width-2, "…")))
		}

		// The arrows are the affordance for left/right, and only the selected
		// row gets them -- on every row they would read as decoration.
		value := truncateRunesHelper(it.Value(), width-6, "…")
		if selected {
			out = append(out, valSel.Render(" ← "+value+" →"))
		} else {
			out = append(out, valIdle.Render("   "+value))
		}
		out = append(out, "")
	}
	return out
}

// renderHelp renders the right column: the selected setting's description.
func (s *SettingsModalModel) renderHelp(width int) []string {
	it := s.Selected()
	if it == nil {
		return nil
	}
	style := lipgloss.NewStyle().Foreground(s.theme.Subtext)

	var out []string
	for _, raw := range strings.Split(it.Help(), "\n") {
		if raw == "" {
			out = append(out, "")
			continue
		}
		for _, wrapped := range wrapPlain(raw, width) {
			out = append(out, style.Render(wrapped))
		}
	}
	return out
}

// wrapPlain wraps on word boundaries. The help text is authored with its own
// line breaks, so this only has to catch lines that overflow a narrow pane.
func wrapPlain(s string, width int) []string {
	if width <= 0 || len(s) <= width {
		return []string{s}
	}
	var out []string
	line := ""
	for _, word := range strings.Fields(s) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			out = append(out, line)
			line = word
		}
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}
