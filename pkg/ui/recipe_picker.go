package ui

import (
	"fmt"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/recipe"

	"charm.land/lipgloss/v2"
)

// RecipePickerModel represents the recipe picker overlay
type RecipePickerModel struct {
	recipes       []recipe.Recipe
	selectedIndex int
	width         int
	height        int
	theme         Theme
}

// NewRecipePickerModel creates a new recipe picker
func NewRecipePickerModel(recipes []recipe.Recipe, theme Theme) RecipePickerModel {
	return RecipePickerModel{
		recipes:       recipes,
		selectedIndex: 0,
		theme:         theme,
	}
}

// SetSize updates the picker dimensions
func (m *RecipePickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// MoveUp moves selection up
func (m *RecipePickerModel) MoveUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

// MoveDown moves selection down
func (m *RecipePickerModel) MoveDown() {
	if m.selectedIndex < len(m.recipes)-1 {
		m.selectedIndex++
	}
}

// SelectedRecipe returns the currently selected recipe
func (m *RecipePickerModel) SelectedRecipe() *recipe.Recipe {
	if len(m.recipes) == 0 || m.selectedIndex >= len(m.recipes) {
		return nil
	}
	return &m.recipes[m.selectedIndex]
}

// SelectedIndex returns the current selection index
func (m *RecipePickerModel) SelectedIndex() int {
	return m.selectedIndex
}

// View renders the recipe picker panel. Composited into the view via
// OverlayCenterDimBackdrop in model_view.go (bt-vklk Phase 1 + bt-rhfo) so
// callers do not center it again. Returns the rendered titled panel; the
// dimmed-backdrop centering is the compositor's job.
//
// Panel height is capped at ~70% of the body height (matches the alerts modal
// pop-up sizing). When the recipe count exceeds the visible window, the
// viewport scrolls to keep the selected recipe in view and "↑ N more" /
// "↓ N more" indicators surface the truncated edges. Without the cap, an
// 11-recipe list grew to 38 rows and ate the entire body height, leaving no
// surrounding bg for OverlayCenterDimBackdrop to dim — the user perceived
// this as "the dim layer is invisible" (bt-rhfo dogfood, 2026-05-07).
func (m *RecipePickerModel) View() string {
	if m.width == 0 {
		m.width = 60
	}
	if m.height == 0 {
		m.height = 20
	}

	t := m.theme

	// Box width: content-comfortable, narrows on small terminals.
	boxWidth := 50
	if m.width < 60 {
		boxWidth = m.width - 10
	}
	if boxWidth < 30 {
		boxWidth = 30
	}

	// Box height: cap at ~70% of body height (matches alertsPanelHeight).
	boxHeight := m.height * 7 / 10
	if boxHeight < 12 {
		boxHeight = 12
	}
	if boxHeight > m.height-2 {
		boxHeight = m.height - 2
	}
	if boxHeight < 8 {
		boxHeight = 8
	}

	// Inner content rows (RenderTitledPanel reserves 2 for top/bottom border).
	// Subtract another 2 for the Padding(1, 2) wrap below, giving the budget
	// available for our own content lines.
	innerRows := boxHeight - 4
	if innerRows < 4 {
		innerRows = 4
	}

	// Layout budget: 2 rows for top/bottom scroll indicators (always rendered
	// — blank when not scrolled — so the layout stays stable across scroll
	// states) and 2 rows for the footer (blank separator + key hint).
	const indicatorRows = 2
	const footerRows = 2
	bodyRows := innerRows - indicatorRows - footerRows
	if bodyRows < 3 {
		bodyRows = 3
	}

	// Each recipe up to 3 rows (name + desc + separator). Last visible recipe
	// drops the trailing separator: 3N - 1 ≤ bodyRows → N = (bodyRows + 1) / 3.
	visibleCount := 0
	if len(m.recipes) > 0 {
		visibleCount = (bodyRows + 1) / 3
		if visibleCount < 1 {
			visibleCount = 1
		}
		if visibleCount > len(m.recipes) {
			visibleCount = len(m.recipes)
		}
	}

	// Viewport offset: keep selected in view, biased to ~1/3 down the window.
	offset := 0
	if visibleCount < len(m.recipes) {
		offset = m.selectedIndex - visibleCount/3
		if offset < 0 {
			offset = 0
		}
		if max := len(m.recipes) - visibleCount; offset > max {
			offset = max
		}
	}
	end := offset + visibleCount
	if end > len(m.recipes) {
		end = len(m.recipes)
	}

	scrollHintStyle := lipgloss.NewStyle().
		Foreground(t.Subtext).
		Italic(true)
	footerStyle := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Italic(true)

	// Match the alerts-modal padding pattern: prefix each line with a fixed
	// leading-space pad. RenderTitledPanel handles trailing-space padding to
	// innerWidth via lipgloss.Width on each line. Avoid wrapping the whole
	// block in lipgloss.NewStyle().Padding(...).Render(...): lipgloss does
	// not consistently pad multi-line styled content to a uniform width when
	// each line has its own SGR scope, producing description rows that end
	// early and leave the right border misaligned (bt-rhfo dogfood).
	const leadPad = "  "

	var lines []string
	lines = append(lines, "") // top breathing room

	// Top scroll indicator (or blank).
	if offset > 0 {
		lines = append(lines, leadPad+scrollHintStyle.Render(fmt.Sprintf("  ↑ %d more", offset)))
	} else {
		lines = append(lines, "")
	}

	for i := offset; i < end; i++ {
		r := m.recipes[i]
		isSelected := i == m.selectedIndex

		nameStyle := lipgloss.NewStyle()
		if isSelected {
			nameStyle = nameStyle.Foreground(t.Primary).Bold(true)
		} else {
			nameStyle = nameStyle.Foreground(t.Base.GetForeground())
		}

		prefix := "  "
		if isSelected {
			prefix = "▸ "
		}

		name := prefix + r.Name
		lines = append(lines, leadPad+nameStyle.Render(name))

		if r.Description != "" {
			descStyle := lipgloss.NewStyle().
				Foreground(t.Secondary).
				Italic(true)
			desc := "    " + truncateRunesHelper(r.Description, boxWidth-8, "…")
			lines = append(lines, leadPad+descStyle.Render(desc))
		}

		if i < end-1 {
			lines = append(lines, "")
		}
	}

	// Bottom scroll indicator (or blank).
	if remaining := len(m.recipes) - end; remaining > 0 {
		lines = append(lines, leadPad+scrollHintStyle.Render(fmt.Sprintf("  ↓ %d more", remaining)))
	} else {
		lines = append(lines, "")
	}

	lines = append(lines, "")
	lines = append(lines, leadPad+footerStyle.Render("j/k: navigate • enter: apply • esc: cancel"))
	lines = append(lines, "") // bottom breathing room

	content := strings.Join(lines, "\n")

	return RenderTitledPanel(content, PanelOpts{
		Title:   "Select Recipe",
		Width:   boxWidth,
		Height:  boxHeight,
		Focused: true,
	})
}

// RecipeCount returns the number of recipes
func (m *RecipePickerModel) RecipeCount() int {
	return len(m.recipes)
}

// FormatRecipeInfo returns a formatted string for the active recipe display
func FormatRecipeInfo(r *recipe.Recipe) string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("Recipe: %s", r.Name)
}
