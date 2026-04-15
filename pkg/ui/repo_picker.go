package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// RepoPickerModel represents the repository filter picker overlay (workspace mode).
type RepoPickerModel struct {
	repos         []string
	selectedIndex int
	selected      map[string]bool // repo -> selected
	width         int
	height        int
	theme         Theme
}

// NewRepoPickerModel creates a new repo picker. By default, all repos are selected.
func NewRepoPickerModel(repos []string, theme Theme) RepoPickerModel {
	m := RepoPickerModel{
		repos:         append([]string(nil), repos...),
		selectedIndex: 0,
		selected:      make(map[string]bool, len(repos)),
		theme:         theme,
	}
	for _, r := range m.repos {
		m.selected[r] = true
	}
	return m
}

// SetSize updates the picker dimensions.
func (m *RepoPickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetActiveRepos initializes selection from the currently active repo filter (nil = all).
// Cursor moves to the first selected project (or stays at top if all/none).
func (m *RepoPickerModel) SetActiveRepos(active map[string]bool) {
	if len(m.repos) == 0 {
		m.selected = map[string]bool{}
		return
	}

	m.selected = make(map[string]bool, len(m.repos))
	if active == nil {
		for _, r := range m.repos {
			m.selected[r] = true
		}
		m.selectedIndex = 0
		return
	}

	firstSelected := -1
	for i, r := range m.repos {
		if active[r] {
			m.selected[r] = true
			if firstSelected == -1 {
				firstSelected = i
			}
		}
	}
	if firstSelected >= 0 {
		m.selectedIndex = firstSelected
	} else {
		m.selectedIndex = 0
	}
}

// MoveUp moves selection up, wrapping to the bottom.
func (m *RepoPickerModel) MoveUp() {
	if len(m.repos) == 0 {
		return
	}
	if m.selectedIndex > 0 {
		m.selectedIndex--
	} else {
		m.selectedIndex = len(m.repos) - 1
	}
}

// MoveDown moves selection down, wrapping to the top.
func (m *RepoPickerModel) MoveDown() {
	if len(m.repos) == 0 {
		return
	}
	if m.selectedIndex < len(m.repos)-1 {
		m.selectedIndex++
	} else {
		m.selectedIndex = 0
	}
}

// ToggleSelected toggles the selected state of the current repo.
func (m *RepoPickerModel) ToggleSelected() {
	if len(m.repos) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.repos) {
		return
	}
	r := m.repos[m.selectedIndex]
	m.selected[r] = !m.selected[r]
}

// AnySelected returns true if at least one repo is selected.
func (m *RepoPickerModel) AnySelected() bool {
	for _, r := range m.repos {
		if m.selected[r] {
			return true
		}
	}
	return false
}

// NoneSelected returns true if no repos are selected.
func (m *RepoPickerModel) NoneSelected() bool {
	return !m.AnySelected()
}

// AllSelected returns true if every repo is selected.
func (m *RepoPickerModel) AllSelected() bool {
	for _, r := range m.repos {
		if !m.selected[r] {
			return false
		}
	}
	return len(m.repos) > 0
}

// ToggleAll deselects all if any are selected, otherwise selects all.
func (m *RepoPickerModel) ToggleAll() {
	if m.AnySelected() {
		m.DeselectAll()
	} else {
		m.SelectAll()
	}
}

// SelectAll selects all repos.
func (m *RepoPickerModel) SelectAll() {
	for _, r := range m.repos {
		m.selected[r] = true
	}
}

// DeselectAll deselects all repos.
func (m *RepoPickerModel) DeselectAll() {
	for _, r := range m.repos {
		m.selected[r] = false
	}
}

// CursorRepo returns the repo name under the cursor.
func (m *RepoPickerModel) CursorRepo() string {
	if len(m.repos) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.repos) {
		return ""
	}
	return m.repos[m.selectedIndex]
}

// SelectedRepos returns the selected repos as a map (repo -> true).
func (m RepoPickerModel) SelectedRepos() map[string]bool {
	out := make(map[string]bool)
	for _, r := range m.repos {
		if m.selected[r] {
			out[r] = true
		}
	}
	return out
}

const pickerHPad = 3 // horizontal padding inside box

// footer hint text (no padding - added during render)
const pickerFooter = "toggle: space all: a \u2022 apply: enter"

// View renders the repo picker overlay.
func (m *RepoPickerModel) View() string {
	if m.width == 0 {
		m.width = 60
	}
	if m.height == 0 {
		m.height = 20
	}

	t := m.theme

	// Find the longest repo name to size the box
	maxNameLen := 0
	for _, repo := range m.repos {
		if len(repo) > maxNameLen {
			maxNameLen = len(repo)
		}
	}

	// Compute the widest content line to size the box.
	// Repo line: hpad + cursor(2) + indicator(2) + space(1) + name + hpad
	repoLineWidth := pickerHPad + 2 + 2 + 1 + maxNameLen + pickerHPad
	// Footer line: hpad + text + hpad
	footerLineWidth := pickerHPad + len(pickerFooter) + pickerHPad

	innerWidth := repoLineWidth
	if footerLineWidth > innerWidth {
		innerWidth = footerLineWidth
	}

	boxWidth := innerWidth + 2 // add border chars
	if boxWidth > m.width-4 {
		boxWidth = m.width - 4
		innerWidth = boxWidth - 2
	}
	if boxWidth < 30 {
		boxWidth = 30
		innerWidth = boxWidth - 2
	}

	pad := strings.Repeat(" ", pickerHPad)

	var lines []string
	lines = append(lines, "") // top breathing room

	if len(m.repos) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)
		lines = append(lines, emptyStyle.Render(pad+"No projects available."))
	} else {
		// Each line: cursor(2) + indicator(2) + space(1) + name
		lineContentWidth := 2 + 2 + 1 + maxNameLen
		// Center the block within the inner area (minus horizontal padding)
		availableWidth := innerWidth - pickerHPad*2
		leftExtra := (availableWidth - lineContentWidth) / 2
		if leftExtra < 0 {
			leftExtra = 0
		}
		centering := pad + strings.Repeat(" ", leftExtra)

		checkStyle := lipgloss.NewStyle().Foreground(t.Primary)
		uncheckStyle := lipgloss.NewStyle().Foreground(t.Secondary)

		for i, repo := range m.repos {
			isCursor := i == m.selectedIndex
			isSelected := m.selected[repo]

			nameStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
			if isCursor {
				nameStyle = nameStyle.Foreground(t.Primary).Bold(true)
			}

			cursor := "  "
			if isCursor {
				cursor = nameStyle.Render("▸ ")
			}

			indicator := uncheckStyle.Render("• ")
			if isSelected {
				indicator = checkStyle.Render("✓ ")
			}

			line := centering + cursor + indicator + nameStyle.Render(repo)
			lines = append(lines, line)
		}
	}

	lines = append(lines, "")
	footerStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Italic(true)
	lines = append(lines, footerStyle.Render(pad+pickerFooter))

	content := strings.Join(lines, "\n")

	return RenderTitledPanel(content, PanelOpts{
		Title:   "Project Filter",
		Width:   boxWidth,
		Focused: true,
	})
}
