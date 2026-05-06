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
	if active == nil || len(active) <= 1 {
		// All-projects or single-project mode: open with nothing checked for quick-pick.
		// Multi-project groups (2+) preserve their checkmarks for add/remove.
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

// SetCursor moves the cursor to the given index. Out-of-bounds indices are
// clamped. Used by the mouse click handler (bt-hpsq).
func (m *RepoPickerModel) SetCursor(idx int) {
	if len(m.repos) == 0 {
		m.selectedIndex = 0
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.repos) {
		idx = len(m.repos) - 1
	}
	m.selectedIndex = idx
}

// repoPickerVerticalChrome is the row count outside the repo list itself:
// 1 (top border) + 1 (top breathing) + 1 (blank) + 1 (footer) + 1 (bottom
// border) = 5. Must stay aligned with View().
const repoPickerVerticalChrome = 5

// repoPickerMaxVisible caps the number of repo rows shown at once. With many
// projects in workspace mode the modal would otherwise grow without bound and
// overflow the terminal on smaller windows.
const repoPickerMaxVisible = 30

// repoRowOffsetInBox is the row offset (relative to the panel top border) at
// which the first repo row appears. Layout: row 0 top border, row 1 top
// breathing blank, row 2+ repos.
const repoRowOffsetInBox = 2

// visibleCount returns how many repo rows fit in the modal at the current
// terminal size. Mirrors the label picker pattern (bt-vr2h): aim for ~75%
// of bg, fall back to whatever fits, cap at repoPickerMaxVisible (30) on
// tall terminals. Without this cap the modal grew with len(m.repos) and
// overflowed scrunched terminals (no chrome around it).
func (m *RepoPickerModel) visibleCount() int {
	bg := m.height
	if bg < 1 {
		bg = 20 // fallback before SetSize is called
	}

	softTotal := int(float64(bg) * 0.75)
	if softTotal > bg {
		softTotal = bg
	}
	visible := softTotal - repoPickerVerticalChrome
	if visible < 1 {
		visible = bg - repoPickerVerticalChrome
	}

	if visible > repoPickerMaxVisible {
		visible = repoPickerMaxVisible
	}
	if len(m.repos) > 0 && visible > len(m.repos) {
		visible = len(m.repos)
	}
	if visible < 1 {
		visible = 1
	}
	return visible
}

// computeBoxWidth derives the modal's outer box width (including borders).
// Pure layout math so Dimensions() and View() share the same width budget.
func (m *RepoPickerModel) computeBoxWidth() int {
	maxNameLen := 0
	for _, repo := range m.repos {
		if len(repo) > maxNameLen {
			maxNameLen = len(repo)
		}
	}

	// Repo line: hpad + cursor(2) + indicator(2) + space(1) + name + hpad
	repoLineWidth := pickerHPad + 2 + 2 + 1 + maxNameLen + pickerHPad
	footerLineWidth := pickerHPad + len(pickerFooter) + pickerHPad

	innerWidth := repoLineWidth
	if footerLineWidth > innerWidth {
		innerWidth = footerLineWidth
	}

	boxWidth := innerWidth + 2 // border chars

	// Cap at 80% of terminal width so wide repo names don't stretch the
	// modal across the whole row on narrow terminals (bt-vr2h).
	if widthCap := int(float64(m.width) * 0.80); boxWidth > widthCap {
		boxWidth = widthCap
	}
	if boxWidth > m.width-4 {
		boxWidth = m.width - 4
	}
	if boxWidth < 30 {
		boxWidth = 30
	}
	return boxWidth
}

// Dimensions returns the modal's outer box (width, height) in cells, used by
// the mouse click handler to compute the centered panel start row/col.
func (m *RepoPickerModel) Dimensions() (int, int) {
	w := m.computeBoxWidth()
	h := m.visibleCount() + repoPickerVerticalChrome
	if len(m.repos) == 0 {
		// Empty-state shows a single "No projects available." line in place
		// of the repo block, so total content rows = 1 (still the same
		// chrome envelope).
		h = 1 + repoPickerVerticalChrome
	}
	return w, h
}

// ItemAtPanelY maps a Y coordinate relative to the picker's top border to
// a repo index. Returns (-1, false) for non-row regions (chrome, blanks,
// footer). Accounts for page-aligned scrolling when len(m.repos) exceeds
// visibleCount() (bt-vr2h).
func (m *RepoPickerModel) ItemAtPanelY(my int) (int, bool) {
	if len(m.repos) == 0 {
		return -1, false
	}
	maxVisible := m.visibleCount()
	relRow := my - repoRowOffsetInBox
	if relRow < 0 || relRow >= maxVisible {
		return -1, false
	}
	start := (m.selectedIndex / maxVisible) * maxVisible
	idx := start + relRow
	if idx >= len(m.repos) {
		return -1, false
	}
	return idx, true
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

	// Find the longest repo name (still needed locally for centering math).
	maxNameLen := 0
	for _, repo := range m.repos {
		if len(repo) > maxNameLen {
			maxNameLen = len(repo)
		}
	}

	boxWidth := m.computeBoxWidth()
	innerWidth := boxWidth - 2

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

		// Page-aligned visible window so paging feels natural and the modal
		// has a fixed total height regardless of len(m.repos) (bt-vr2h).
		maxVisible := m.visibleCount()
		start := (m.selectedIndex / maxVisible) * maxVisible
		end := start + maxVisible
		if end > len(m.repos) {
			end = len(m.repos)
		}

		for i := start; i < end; i++ {
			repo := m.repos[i]
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

		// Pad to fixed visibleCount so modal height stays constant across pages.
		for i := end - start; i < maxVisible; i++ {
			lines = append(lines, "")
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
