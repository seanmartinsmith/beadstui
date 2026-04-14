package ui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// LabelPickerModel provides a fuzzy search popup for quick label filtering
type LabelPickerModel struct {
	allLabels     []string
	labelCounts   map[string]int // count of issues per label
	filtered      []string
	input         textinput.Model
	selectedIndex int
	activeLabels  map[string]bool // currently applied label filters (shown with indicator)
	selected      map[string]bool // labels toggled in this session (space to toggle)
	width         int
	height        int
	theme         Theme
}

// NewLabelPickerModel creates a new label picker with fuzzy search
// labels should be pre-sorted by count descending (from LabelExtractionResult.TopLabels)
func NewLabelPickerModel(labels []string, counts map[string]int, theme Theme) LabelPickerModel {
	// Sort labels by count descending
	sorted := sortLabelsByCountDesc(labels, counts)

	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 50
	ti.SetWidth(30)
	ti.Focus()

	return LabelPickerModel{
		allLabels:     sorted,
		labelCounts:   counts,
		filtered:      sorted,
		input:         ti,
		selectedIndex: 0,
		theme:         theme,
	}
}

// sortLabelsByCountDesc sorts labels by count descending, then alphabetically for ties
func sortLabelsByCountDesc(labels []string, counts map[string]int) []string {
	sorted := make([]string, len(labels))
	copy(sorted, labels)
	sort.Slice(sorted, func(i, j int) bool {
		ci := counts[sorted[i]]
		cj := counts[sorted[j]]
		if ci != cj {
			return ci > cj // descending by count
		}
		return sorted[i] < sorted[j] // alphabetically for ties
	})
	return sorted
}

// SetSize updates the picker dimensions
func (m *LabelPickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetLabels updates the available labels with their counts
func (m *LabelPickerModel) SetLabels(labels []string, counts map[string]int) {
	m.labelCounts = counts
	m.allLabels = sortLabelsByCountDesc(labels, counts)
	m.filterLabels()
}

// SetActiveLabels sets the currently applied label filters so they can be indicated.
func (m *LabelPickerModel) SetActiveLabels(labels []string) {
	m.activeLabels = make(map[string]bool, len(labels))
	for _, l := range labels {
		m.activeLabels[l] = true
	}
	// Pre-select active labels so enter preserves the current filter
	m.selected = make(map[string]bool, len(labels))
	for _, l := range labels {
		m.selected[l] = true
	}
}

// ToggleSelected toggles the label under the cursor.
func (m *LabelPickerModel) ToggleSelected() {
	if len(m.filtered) == 0 || m.selectedIndex >= len(m.filtered) {
		return
	}
	label := m.filtered[m.selectedIndex]
	if m.selected == nil {
		m.selected = make(map[string]bool)
	}
	if m.selected[label] {
		delete(m.selected, label)
	} else {
		m.selected[label] = true
	}
}

// SelectedLabels returns the labels that have been toggled on.
func (m *LabelPickerModel) SelectedLabels() []string {
	var labels []string
	// Return in display order (allLabels order) for deterministic output
	for _, l := range m.allLabels {
		if m.selected[l] {
			labels = append(labels, l)
		}
	}
	return labels
}

// HasSelections returns true if any labels are toggled.
func (m *LabelPickerModel) HasSelections() bool {
	return len(m.selected) > 0
}

// MoveUp moves selection up, wrapping to the bottom.
func (m *LabelPickerModel) MoveUp() {
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
func (m *LabelPickerModel) MoveDown() {
	if len(m.filtered) == 0 {
		return
	}
	if m.selectedIndex < len(m.filtered)-1 {
		m.selectedIndex++
	} else {
		m.selectedIndex = 0
	}
}

// PageDown moves selection to the bottom of the next page.
func (m *LabelPickerModel) PageDown() {
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

// PageUp moves selection to the top of the previous page.
func (m *LabelPickerModel) PageUp() {
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

// visibleCount returns how many labels are visible in the picker.
func (m *LabelPickerModel) visibleCount() int {
	maxVisible := 10
	if m.height < 15 {
		maxVisible = m.height - 7
	}
	if maxVisible < 3 {
		maxVisible = 3
	}
	return maxVisible
}

// SelectedLabel returns the currently selected label
func (m *LabelPickerModel) SelectedLabel() string {
	if len(m.filtered) == 0 || m.selectedIndex >= len(m.filtered) {
		return ""
	}
	return m.filtered[m.selectedIndex]
}

// UpdateInput processes a key message for the text input
func (m *LabelPickerModel) UpdateInput(msg interface{}) {
	m.input, _ = m.input.Update(msg)
	m.filterLabels()
}

// Reset clears the input and resets selection.
// If active label filters are set, the cursor moves to the first one.
func (m *LabelPickerModel) Reset() {
	m.input.SetValue("")
	m.filterLabels()
	// Position cursor on the first active label if any are set
	if len(m.activeLabels) > 0 {
		for i, label := range m.filtered {
			if m.activeLabels[label] {
				m.selectedIndex = i
				return
			}
		}
	}
}

// filterLabels filters the labels based on current input using fuzzy matching.
// Selected (toggled) labels are always pinned at the top of the list so they
// remain visible and accessible even when they don't match the search query.
func (m *LabelPickerModel) filterLabels() {
	query := strings.ToLower(strings.TrimSpace(m.input.Value()))

	// Always allocate a fresh slice - never reuse m.allLabels' backing array
	var result []string

	if query == "" {
		// No search: pin selected at top, then the rest in count order
		if len(m.selected) > 0 {
			seen := make(map[string]bool, len(m.selected))
			for _, l := range m.allLabels {
				if m.selected[l] {
					result = append(result, l)
					seen[l] = true
				}
			}
			for _, l := range m.allLabels {
				if !seen[l] {
					result = append(result, l)
				}
			}
		} else {
			result = make([]string, len(m.allLabels))
			copy(result, m.allLabels)
		}
		m.filtered = result
		m.selectedIndex = 0
		return
	}

	type scored struct {
		label string
		score int
	}

	var matches []scored
	for _, label := range m.allLabels {
		if score := fuzzyScore(label, query); score > 0 {
			matches = append(matches, scored{label, score})
		}
	}

	// Sort by score (higher is better), then alphabetically
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].label < matches[j].label
	})

	// Pin selected labels at the top (in their original sort order), then matches
	seen := make(map[string]bool, len(matches)+len(m.selected))
	if len(m.selected) > 0 {
		for _, l := range m.allLabels {
			if m.selected[l] {
				result = append(result, l)
				seen[l] = true
			}
		}
	}
	for _, match := range matches {
		if !seen[match.label] {
			result = append(result, match.label)
		}
	}

	m.filtered = result

	// Keep selection in bounds
	if m.selectedIndex >= len(m.filtered) {
		m.selectedIndex = len(m.filtered) - 1
	}
	if m.selectedIndex < 0 {
		m.selectedIndex = 0
	}
}

// fuzzyScore returns a score for how well query matches label (0 = no match)
// Uses fzf-style scoring: consecutive matches, word boundary bonuses
func fuzzyScore(label, query string) int {
	label = strings.ToLower(label)
	query = strings.ToLower(query)

	// Exact match gets highest score
	if label == query {
		return 1000
	}

	// Prefix match gets high score
	if strings.HasPrefix(label, query) {
		return 500 + len(query)
	}

	// Contains match
	if strings.Contains(label, query) {
		return 200 + len(query)
	}

	// Fuzzy subsequence match
	li, qi := 0, 0
	score := 0
	consecutive := 0
	lastMatchIdx := -1

	for li < len(label) && qi < len(query) {
		if label[li] == query[qi] {
			qi++
			matchScore := 10

			// Bonus for consecutive matches
			if lastMatchIdx == li-1 {
				consecutive++
				matchScore += consecutive * 5
			} else {
				consecutive = 0
			}

			// Bonus for word boundary match
			if li == 0 || !unicode.IsLetter(rune(label[li-1])) {
				matchScore += 15
			}

			score += matchScore
			lastMatchIdx = li
		}
		li++
	}

	// Only count as match if all query chars were found
	if qi == len(query) {
		return score
	}
	return 0
}

const labelPickerHPad = 3 // horizontal padding inside box

// View renders the label picker overlay
func (m *LabelPickerModel) View() string {
	if m.width == 0 {
		m.width = 60
	}
	if m.height == 0 {
		m.height = 20
	}

	t := m.theme

	maxVisible := m.visibleCount()

	// Size box from ALL labels (not filtered) so it stays stable while typing
	maxLabelWidth := 0
	for _, label := range m.allLabels {
		count := m.labelCounts[label]
		w := len(label) + len(fmt.Sprintf(" (%d)", count))
		if w > maxLabelWidth {
			maxLabelWidth = w
		}
	}

	// Compute box width: hpad + cursor(2) + indicator(2) + space(1) + label+count + hpad
	lineWidth := labelPickerHPad + 2 + 2 + 1 + maxLabelWidth + labelPickerHPad

	// Footer - always use the longer version for stable width
	footerText := "space: toggle \u2022 \u2190/\u2192: page \u2022 enter: apply \u2022 esc: close"
	footerLineWidth := labelPickerHPad + len(footerText) + labelPickerHPad

	// Input line width
	inputLineWidth := labelPickerHPad + 4 + 30 + labelPickerHPad // "> " + input

	innerWidth := lineWidth
	if footerLineWidth > innerWidth {
		innerWidth = footerLineWidth
	}
	if inputLineWidth > innerWidth {
		innerWidth = inputLineWidth
	}

	boxWidth := innerWidth + 2 // add border chars
	if boxWidth > m.width-4 {
		boxWidth = m.width - 4
		innerWidth = boxWidth - 2
	}
	if boxWidth < 35 {
		boxWidth = 35
		innerWidth = boxWidth - 2
	}

	pad := strings.Repeat(" ", labelPickerHPad)

	var lines []string

	// Search input
	inputStyle := lipgloss.NewStyle().
		Foreground(t.Primary)
	inputLine := pad + inputStyle.Render("> ") + m.input.View()
	lines = append(lines, inputLine)
	lines = append(lines, "")

	// Label list with scroll - always render maxVisible lines for vertical stability
	activeStyle := lipgloss.NewStyle().Foreground(t.Primary)
	dimStyle := lipgloss.NewStyle().Foreground(t.Secondary)

	if len(m.filtered) == 0 {
		lines = append(lines, dimStyle.Render(pad+"No matching labels"))
		// Pad to fixed height
		for i := 1; i < maxVisible; i++ {
			lines = append(lines, "")
		}
	} else {
		// Page-aligned visible window so paging feels natural
		start := (m.selectedIndex / maxVisible) * maxVisible
		end := start + maxVisible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := start; i < end; i++ {
			label := m.filtered[i]
			isCursor := i == m.selectedIndex
			isSelected := m.selected[label]

			nameStyle := lipgloss.NewStyle().Foreground(t.Base.GetForeground())
			countStyle := lipgloss.NewStyle().Foreground(t.Secondary)
			if isCursor {
				nameStyle = nameStyle.Foreground(t.Primary).Bold(true)
				countStyle = countStyle.Foreground(t.Primary)
			}

			cursor := "  "
			if isCursor {
				cursor = nameStyle.Render("▸ ")
			}

			indicator := dimStyle.Render("• ")
			if isSelected {
				indicator = activeStyle.Render("✓ ")
			}

			count := m.labelCounts[label]
			countStr := countStyle.Render(fmt.Sprintf(" (%d)", count))
			line := pad + cursor + indicator + nameStyle.Render(label) + countStr
			lines = append(lines, line)
		}

		// Pad remaining lines to fixed height
		for i := end - start; i < maxVisible; i++ {
			lines = append(lines, "")
		}
	}

	// Page indicator + selection count (always present for vertical stability)
	pageStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Italic(true)
	selCountStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	selSuffix := ""
	if len(m.selected) > 0 {
		selSuffix = " • " + selCountStyle.Render(fmt.Sprintf("%d selected", len(m.selected)))
	}
	lines = append(lines, "")
	if len(m.filtered) > maxVisible {
		page := m.selectedIndex/maxVisible + 1
		totalPages := (len(m.filtered) + maxVisible - 1) / maxVisible
		lines = append(lines, pageStyle.Render(
			pad+fmt.Sprintf("%d/%d (%d labels)", page, totalPages, len(m.filtered)))+selSuffix)
	} else if len(m.filtered) > 0 {
		lines = append(lines, pageStyle.Render(
			pad+fmt.Sprintf("%d labels", len(m.filtered)))+selSuffix)
	} else {
		if selSuffix != "" {
			lines = append(lines, pad+selSuffix)
		} else {
			lines = append(lines, "")
		}
	}

	// Footer hints
	lines = append(lines, "")
	footerStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Italic(true)
	lines = append(lines, footerStyle.Render(pad+footerText))

	content := strings.Join(lines, "\n")

	return RenderTitledPanel(content, PanelOpts{
		Title:   "Filter by Label",
		Width:   boxWidth,
		Focused: true,
	})
}

// InputValue returns the current input value
func (m *LabelPickerModel) InputValue() string {
	return m.input.Value()
}

// FilteredCount returns the number of filtered labels
func (m *LabelPickerModel) FilteredCount() int {
	return len(m.filtered)
}

// itoa is a simple int to string helper
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
