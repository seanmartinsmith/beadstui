package ui

import (
	"fmt"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/model"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// RepoPickerModel represents the repository filter picker overlay (workspace mode).
type RepoPickerModel struct {
	repos    []string        // canonical full list (original enumeration order)
	filtered []string        // display/nav list: atlas-pinned, search-narrowed
	input    textinput.Model // search box (Wave 2, bt-9lpib core)
	// searchFocused gates whether typed characters route to the text input or
	// are interpreted as navigation. The picker opens with searchFocused false
	// (mirrors the label picker, bt-wnda): the user lands on the project list;
	// "/" focuses the search bar, Esc inside search blurs it without closing.
	searchFocused bool
	selectedIndex int
	selected      map[string]bool // repo -> selected
	width         int
	height        int
	theme         Theme
}

// NewRepoPickerModel creates a new repo picker. By default, all repos are selected.
func NewRepoPickerModel(repos []string, theme Theme) RepoPickerModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 50
	ti.SetWidth(30)
	// The View renders its own styled "> " prompt, so clear the textinput's
	// built-in one to avoid a doubled ">".
	ti.Prompt = ""
	// Search starts blurred (bt-wnda parity): the user lands on the list.
	ti.Blur()

	m := RepoPickerModel{
		repos:         append([]string(nil), repos...),
		input:         ti,
		selectedIndex: 0,
		selected:      make(map[string]bool, len(repos)),
		theme:         theme,
	}
	for _, r := range m.repos {
		m.selected[r] = true
	}
	m.filterRepos()
	return m
}

// SetSize updates the picker dimensions.
func (m *RepoPickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// FocusSearch routes typed characters to the search input.
func (m *RepoPickerModel) FocusSearch() {
	m.searchFocused = true
	m.input.Focus()
}

// BlurSearch returns focus to the project list. The search query buffer is
// preserved so the user can resume editing without retyping.
func (m *RepoPickerModel) BlurSearch() {
	m.searchFocused = false
	m.input.Blur()
}

// IsSearchFocused reports whether the text input owns keyboard focus.
func (m *RepoPickerModel) IsSearchFocused() bool {
	return m.searchFocused
}

// UpdateInput processes a key message for the text input, then re-filters.
func (m *RepoPickerModel) UpdateInput(msg interface{}) {
	m.input, _ = m.input.Update(msg)
	m.filterRepos()
}

// InputValue returns the current search query.
func (m *RepoPickerModel) InputValue() string {
	return m.input.Value()
}

// atlasFirst returns repos reordered so any beads_global/global namespace key
// sorts to the front (bt-z1pzj: first-class pinned row), preserving the
// relative order of every other repo. Display-only reordering; m.selected keys
// and the active-repo filter stay on the raw spelling, so selection/filtering
// logic is untouched.
func atlasFirst(repos []string) []string {
	pinned := make([]string, 0, 1)
	rest := make([]string, 0, len(repos))
	for _, r := range repos {
		if model.IsAtlasNamespace(r) {
			pinned = append(pinned, r)
		} else {
			rest = append(rest, r)
		}
	}
	return append(pinned, rest...)
}

// repoMatches reports whether repo's display name contains query (already
// lower-cased and trimmed). Matches on DisplayRepoName so a user searching for
// "atlas" finds the beads_global namespace by the label they actually see.
func repoMatches(repo, query string) bool {
	return strings.Contains(strings.ToLower(model.DisplayRepoName(repo)), query)
}

// filterRepos rebuilds m.filtered from the current search query. Empty query =
// the full list; otherwise a case-insensitive substring narrow. The atlas
// namespace is always pinned to the top of whatever survives the filter.
func (m *RepoPickerModel) filterRepos() {
	query := strings.ToLower(strings.TrimSpace(m.input.Value()))

	var result []string
	if query == "" {
		result = append(result, m.repos...)
	} else {
		for _, r := range m.repos {
			if repoMatches(r, query) {
				result = append(result, r)
			}
		}
	}

	m.filtered = atlasFirst(result)

	// Keep the cursor in bounds after the list changes size.
	if m.selectedIndex >= len(m.filtered) {
		m.selectedIndex = len(m.filtered) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
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
	for i, r := range m.filtered {
		if active[r] {
			m.selected[r] = true
			if firstSelected == -1 {
				firstSelected = i
			}
		}
	}
	// Selection is keyed on the raw repo name regardless of display position,
	// so also mark any active repo not currently in the filtered view.
	for r := range active {
		if _, ok := m.selected[r]; !ok {
			for _, rr := range m.repos {
				if rr == r {
					m.selected[r] = true
				}
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
	if len(m.filtered) == 0 {
		return
	}
	if m.selectedIndex > 0 {
		m.selectedIndex--
	} else {
		m.selectedIndex = len(m.filtered) - 1
	}
}

// MoveDown moves selection down, wrapping to the top.
func (m *RepoPickerModel) MoveDown() {
	if len(m.filtered) == 0 {
		return
	}
	if m.selectedIndex < len(m.filtered)-1 {
		m.selectedIndex++
	} else {
		m.selectedIndex = 0
	}
}

// PageDown moves selection to the bottom of the next page (bt-6ltx9).
func (m *RepoPickerModel) PageDown() {
	if len(m.filtered) == 0 {
		return
	}
	pageSize := m.visibleCount()
	currentPageStart := (m.selectedIndex / pageSize) * pageSize
	target := currentPageStart + pageSize + pageSize - 1 // bottom of next page
	if target >= len(m.filtered) {
		target = len(m.filtered) - 1
	}
	m.selectedIndex = target
}

// PageUp moves selection to the top of the previous page (bt-6ltx9).
func (m *RepoPickerModel) PageUp() {
	if len(m.filtered) == 0 {
		return
	}
	pageSize := m.visibleCount()
	currentPageStart := (m.selectedIndex / pageSize) * pageSize
	target := currentPageStart - pageSize // top of previous page
	if target < 0 {
		target = 0
	}
	m.selectedIndex = target
}

// ToggleSelected toggles the selected state of the current repo.
func (m *RepoPickerModel) ToggleSelected() {
	if len(m.filtered) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.filtered) {
		return
	}
	r := m.filtered[m.selectedIndex]
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
	if len(m.filtered) == 0 || m.selectedIndex < 0 || m.selectedIndex >= len(m.filtered) {
		return ""
	}
	return m.filtered[m.selectedIndex]
}

// SelectedRepos returns the selected repos as a map (repo -> true). Selection
// spans the full repo set, not just the currently filtered view, so a search
// that hides a checked project does not drop it from the applied filter.
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
	if len(m.filtered) == 0 {
		m.selectedIndex = 0
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.filtered) {
		idx = len(m.filtered) - 1
	}
	m.selectedIndex = idx
}

// repoPickerVerticalChrome is the row count outside the repo list itself:
// 1 (top border) + 1 (search input) + 1 (blank) + 1 (blank) + 1 (page
// indicator) + 1 (blank) + 1 (footer) + 1 (bottom border) = 8. Must stay
// aligned with View(). Mirrors labelPickerVerticalChrome.
const repoPickerVerticalChrome = 8

// repoPickerMaxVisible caps the number of repo rows shown at once. With many
// projects in workspace mode the modal would otherwise grow without bound and
// overflow the terminal on smaller windows.
const repoPickerMaxVisible = 30

// repoRowOffsetInBox is the row offset (relative to the panel top border) at
// which the first repo row appears. Layout: row 0 top border, row 1 search
// input, row 2 blank, row 3+ repos.
const repoRowOffsetInBox = 3

// repoSearchRowOffsetInBox is the row offset (relative to panel top border) of the
// search input row. A click here focuses the search input.
const repoSearchRowOffsetInBox = 1

// visibleCount returns how many repo rows fit in the modal at the current
// terminal size. Mirrors the label picker pattern (bt-vr2h): aim for ~75%
// of bg, fall back to whatever fits, cap at repoPickerMaxVisible (30) on
// tall terminals, and never exceed the filtered count so the modal stays
// compact for a short project list (paging engages only on genuine overflow,
// which is precisely the bt-6ltx9 scenario).
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
	// Never taller than the list itself (min 1) so the modal stays compact for
	// a short project list; paging engages only on genuine overflow.
	n := len(m.filtered)
	if n < 1 {
		n = 1
	}
	if visible > n {
		visible = n
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
		if l := len(model.DisplayRepoName(repo)); l > maxNameLen {
			maxNameLen = l
		}
	}

	// Repo line: hpad + cursor(2) + indicator(2) + space(1) + name + hpad
	repoLineWidth := pickerHPad + 2 + 2 + 1 + maxNameLen + pickerHPad
	footerLineWidth := pickerHPad + len(pickerFooter) + pickerHPad
	// Input line: hpad + "> "(2) + input width(30) + hpad
	inputLineWidth := pickerHPad + 2 + 30 + pickerHPad

	innerWidth := repoLineWidth
	if footerLineWidth > innerWidth {
		innerWidth = footerLineWidth
	}
	if inputLineWidth > innerWidth {
		innerWidth = inputLineWidth
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
	return w, h
}

// ItemAtPanelY maps a Y coordinate relative to the picker's top border to
// a filtered-list index. Returns (-1, false) for non-row regions (chrome,
// input, blanks, page indicator, footer). Accounts for page-aligned
// scrolling when len(m.filtered) exceeds visibleCount() (bt-vr2h).
func (m *RepoPickerModel) ItemAtPanelY(my int) (int, bool) {
	if len(m.filtered) == 0 {
		return -1, false
	}
	maxVisible := m.visibleCount()
	relRow := my - repoRowOffsetInBox
	if relRow < 0 || relRow >= maxVisible {
		return -1, false
	}
	start := (m.selectedIndex / maxVisible) * maxVisible
	idx := start + relRow
	if idx >= len(m.filtered) {
		return -1, false
	}
	return idx, true
}

// IsSearchRow reports whether the given panel-relative Y is the search input
// row. Used by mouse routing to focus the search input on click.
func (m *RepoPickerModel) IsSearchRow(my int) bool {
	return my == repoSearchRowOffsetInBox
}

const pickerHPad = 3 // horizontal padding inside box

// footer hint text (no padding - added during render). Mirrors the label
// picker footer convention: select-all ("a") lives in the ; sidebar / ?
// overlay rather than the footer, keeping the line short enough to render
// without truncation on typical terminals.
const pickerFooter = "toggle: space search: / page: ←/→ • apply: enter"

// View renders the repo picker overlay.
func (m *RepoPickerModel) View() string {
	if m.width == 0 {
		m.width = 60
	}
	if m.height == 0 {
		m.height = 20
	}

	t := m.theme

	// Find the longest displayed repo name (still needed locally for centering).
	maxNameLen := 0
	for _, repo := range m.repos {
		if l := len(model.DisplayRepoName(repo)); l > maxNameLen {
			maxNameLen = l
		}
	}

	boxWidth := m.computeBoxWidth()
	innerWidth := boxWidth - 2

	pad := strings.Repeat(" ", pickerHPad)
	maxVisible := m.visibleCount()

	var lines []string

	// Search input row.
	inputStyle := lipgloss.NewStyle().Foreground(t.Primary)
	lines = append(lines, pad+inputStyle.Render("> ")+m.input.View())
	lines = append(lines, "")

	if len(m.filtered) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)
		msg := "No projects available."
		if strings.TrimSpace(m.input.Value()) != "" {
			msg = "No matching projects"
		}
		lines = append(lines, emptyStyle.Render(pad+msg))
		for i := 1; i < maxVisible; i++ {
			lines = append(lines, "")
		}
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
		// has a fixed total height regardless of len(m.filtered) (bt-vr2h).
		start := (m.selectedIndex / maxVisible) * maxVisible
		end := start + maxVisible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := start; i < end; i++ {
			repo := m.filtered[i]
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
				indicator = checkStyle.Render(activeGlyphs.Success + " ")
			}

			// DisplayRepoName aliases beads_global to "atlas" for display
			// (bt-z1pzj); m.repos/m.selected keep the raw key so selection
			// and filtering are untouched. atlasFirst() pins it to the top.
			line := centering + cursor + indicator + nameStyle.Render(model.DisplayRepoName(repo))
			lines = append(lines, line)
		}

		// Pad to fixed visibleCount so modal height stays constant across pages.
		for i := end - start; i < maxVisible; i++ {
			lines = append(lines, "")
		}
	}

	// Page indicator / count row (always present for vertical stability).
	pageStyle := lipgloss.NewStyle().Foreground(t.Secondary).Italic(true)
	lines = append(lines, "")
	if len(m.filtered) > maxVisible {
		page := m.selectedIndex/maxVisible + 1
		totalPages := (len(m.filtered) + maxVisible - 1) / maxVisible
		lines = append(lines, pageStyle.Render(
			pad+fmt.Sprintf("%d/%d (%d projects)", page, totalPages, len(m.filtered))))
	} else if len(m.filtered) > 0 {
		lines = append(lines, pageStyle.Render(
			pad+fmt.Sprintf("%d projects", len(m.filtered))))
	} else {
		lines = append(lines, "")
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
