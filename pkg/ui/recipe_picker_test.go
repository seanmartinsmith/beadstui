package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/seanmartinsmith/beadstui/pkg/recipe"
)

func TestRecipePickerSelection(t *testing.T) {
	recipes := []recipe.Recipe{
		{Name: "Triage", Description: "Focus on blockers"},
		{Name: "Release", Description: "Prep for release"},
		{Name: "Cleanup", Description: "Debt sweep"},
	}

	m := NewRecipePickerModel(recipes, DefaultTheme())
	m.SetSize(80, 24)

	if sel := m.SelectedRecipe(); sel == nil || sel.Name != "Triage" {
		t.Fatalf("expected initial selection Triage, got %+v", sel)
	}

	m.MoveDown()
	if sel := m.SelectedRecipe(); sel == nil || sel.Name != "Release" {
		t.Fatalf("expected selection Release after MoveDown, got %+v", sel)
	}

	m.MoveUp()
	if sel := m.SelectedRecipe(); sel == nil || sel.Name != "Triage" {
		t.Fatalf("expected back to Triage after MoveUp, got %+v", sel)
	}
}

func TestRecipePickerViewContainsNames(t *testing.T) {
	recipes := []recipe.Recipe{
		{Name: "Alpha", Description: "First"},
	}
	m := NewRecipePickerModel(recipes, DefaultTheme())
	m.SetSize(60, 20)

	out := m.View()
	if !strings.Contains(out, "Alpha") {
		t.Fatalf("expected view to contain recipe name, got:\n%s", out)
	}
	if !strings.Contains(out, "Select Recipe") {
		t.Fatalf("expected view title, got:\n%s", out)
	}
}

func TestFormatRecipeInfo(t *testing.T) {
	if got := FormatRecipeInfo(nil); got != "" {
		t.Fatalf("expected empty string for nil recipe, got %q", got)
	}
	r := recipe.Recipe{Name: "Demo"}
	if got := FormatRecipeInfo(&r); got != "Recipe: Demo" {
		t.Fatalf("unexpected format: %s", got)
	}
}

// TestRecipePickerHeightBounded is a regression guard for bt-rhfo: the
// recipe-picker modal's rendered height must not exceed the available body
// height when there are more recipes than fit. Pre-fix the modal grew with
// content (11 recipes × 3 rows = 38 rows vs body height 39), which left no
// surrounding bg for OverlayCenterDimBackdrop to dim — the user perceived
// this as a broken dim-backdrop layer. Cap matches the alerts modal's
// content-comfortable pop-up sizing (~70% of body height).
func TestRecipePickerHeightBounded(t *testing.T) {
	cases := []struct{ w, h int }{
		{60, 20},
		{80, 24},
		{120, 30},
		{160, 40},
		{200, 50},
	}

	for _, sz := range cases {
		t.Run(fmt.Sprintf("%dx%d", sz.w, sz.h), func(t *testing.T) {
			m := seedModel()
			updated, _ := m.Update(tea.WindowSizeMsg{Width: sz.w, Height: sz.h})
			m = updated.(Model)
			m.recipePicker.SetSize(sz.w, sz.h-1)

			panel := m.recipePicker.View()
			panelRows := strings.Count(panel, "\n") + 1
			bodyRows := sz.h - 1

			// Cap is 70% rounded down (matches alertsPanelHeight); allow a
			// small slack for layout rounding plus the 12-row floor.
			maxAllowed := bodyRows*7/10 + 1
			if maxAllowed < 12 {
				maxAllowed = 12
			}
			if panelRows > maxAllowed {
				t.Errorf("recipe picker rendered %d rows, want <= %d (body=%d)",
					panelRows, maxAllowed, bodyRows)
			}
			// Must leave bg above and below for the dim-backdrop to read as
			// a pop-up. Skip the check at the floor (12) where small
			// terminals legitimately consume most of the body.
			if panelRows > 12 && bodyRows-panelRows < 4 {
				t.Errorf("recipe picker leaves only %d bg rows around it (body=%d, panel=%d) - dim-backdrop will look broken",
					bodyRows-panelRows, bodyRows, panelRows)
			}
		})
	}
}

// TestRecipePickerApostropheTogglesOff is the bt-4l28 regression guard: the
// open key (`'`) re-pressed while the modal is open must close it, matching
// the toggle convention used by Help (`?`), Sidebar (`;`), Tutorial, etc.
// Pre-fix the toggle-off branch lived in handleListKeys but the dispatcher's
// modal early-return routes the key to handleRecipePickerKeys first, leaving
// the branch unreachable.
func TestRecipePickerApostropheTogglesOff(t *testing.T) {
	m := seedModel()
	m.openModal(ModalRecipePicker)
	m.focused = focusRecipePicker

	if m.activeModal != ModalRecipePicker {
		t.Fatalf("setup: recipe picker should be open")
	}

	m = m.handleRecipePickerKeys(tea.KeyPressMsg{Code: '\'', Text: "'"})
	if m.activeModal == ModalRecipePicker {
		t.Fatalf("expected ' to close recipe picker, modal still open")
	}
	if m.focused != focusList {
		t.Fatalf("expected focus to return to focusList, got %v", m.focused)
	}
}

// TestRecipePickerUniformRowWidth is a regression guard for bt-rhfo dogfood:
// every rendered row of the recipe panel must end at the panel's right
// border. Pre-fix description rows ended at column ~28 while name/blank
// rows ended at column 50, leaving the right border ragged and bg content
// visible through the modal interior.
func TestRecipePickerUniformRowWidth(t *testing.T) {
	m := seedModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m = updated.(Model)
	m.recipePicker.SetSize(160, 39)

	view := m.recipePicker.View()
	rows := strings.Split(view, "\n")

	// The panel width is set inside RecipePickerModel.View; derive expected
	// width from the top border row (which is always the panel's full width).
	if len(rows) == 0 {
		t.Fatal("recipe picker rendered no rows")
	}
	want := lipgloss.Width(rows[0])
	if want < 30 {
		t.Fatalf("top border width=%d looks wrong, panel may be misconfigured", want)
	}
	for i, r := range rows {
		got := lipgloss.Width(r)
		if got != want {
			t.Errorf("row %d width = %d, want %d (top border); row content = %q",
				i, got, want, r)
		}
	}
}

// TestRecipePickerScrollIndicators verifies the scroll-indicator UX shows
// the truncated count in both directions when the recipe count exceeds the
// visible window (bt-rhfo).
func TestRecipePickerScrollIndicators(t *testing.T) {
	m := seedModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	m.recipePicker.SetSize(80, 23)

	total := m.recipePicker.RecipeCount()
	if total < 4 {
		t.Skip("need at least 4 recipes to exercise scroll indicators")
	}

	// Move past the first visible window so both indicators show.
	for i := 0; i < total/2; i++ {
		m.recipePicker.MoveDown()
	}

	view := m.recipePicker.View()
	if !strings.Contains(view, "↑") {
		t.Error("expected up indicator when scrolled past top")
	}
	if !strings.Contains(view, "↓") {
		t.Error("expected down indicator when more recipes follow")
	}

	// Reset to top: no up indicator.
	for i := 0; i < total; i++ {
		m.recipePicker.MoveUp()
	}
	if strings.Contains(m.recipePicker.View(), "↑") {
		t.Error("at top selection, did not expect up indicator")
	}
}
