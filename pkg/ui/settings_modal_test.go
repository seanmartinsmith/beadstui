package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

func settingsTestModel(t *testing.T) Model {
	t.Helper()
	m := NewModel([]model.Issue{{ID: "1", Title: "One", Status: model.StatusOpen}}, nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

// TestSettingsCyclesThemeLive is the contract the whole screen exists for: a
// palette must apply as the user moves through the list, not on a commit step.
// Without it the picker is a menu of names, which is the state bt was already
// in via BT_THEME.
func TestSettingsCyclesThemeLive(t *testing.T) {
	restoreThemeGlobals(t)
	m := settingsTestModel(t)

	m.settingsModal.Reset()
	before := ColorPrimary

	it := m.settingsModal.Selected()
	if it == nil {
		t.Fatal("settings screen has no settings")
	}
	if it.Name() != "Color theme" {
		t.Fatalf("first setting = %q, want Color theme", it.Name())
	}

	// Step until the palette actually moves. Consecutive themes can resolve to
	// the same Primary, so a single step is not a reliable signal; the corpus
	// is 43 entries, so a bounded walk covers it without looping forever.
	moved := false
	for i := 0; i < len(ThemeNames()); i++ {
		it.Next(&m)
		if ColorPrimary != before {
			moved = true
			break
		}
	}
	if !moved {
		t.Error("cycling through the entire corpus never changed ColorPrimary; the theme is not being applied")
	}
}

// TestSettingsCancelReverts covers the difference between keep and cancel.
// Values apply as you scroll past them, so without a revert on esc, cancelling
// would silently keep whichever palette the user happened to stop on.
func TestSettingsCancelReverts(t *testing.T) {
	restoreThemeGlobals(t)
	m := settingsTestModel(t)

	m.settingsModal.Reset()
	original := ColorPrimary

	it := m.settingsModal.Selected()
	if it == nil {
		t.Fatal("settings screen has no settings")
	}
	for i := 0; i < len(ThemeNames()); i++ {
		it.Next(&m)
		if ColorPrimary != original {
			break
		}
	}
	if ColorPrimary == original {
		t.Skip("no palette in the corpus differs from the default Primary; nothing to revert")
	}

	m.openModal(ModalSettings)
	m = m.handleSettingsModalKeys(tea.KeyPressMsg{Code: tea.KeyEsc})

	if ColorPrimary != original {
		t.Error("esc did not restore the palette that was active when the screen opened")
	}
	if m.activeModal != ModalNone {
		t.Errorf("esc left modal %v open", m.activeModal)
	}
}

// TestSettingsKeepDoesNotRevert is the mirror: enter leaves the selection in
// place. Asserting both directions is what stops a future change from making
// keep and cancel behave identically, which would make the distinction
// invisible rather than broken.
func TestSettingsKeepDoesNotRevert(t *testing.T) {
	restoreThemeGlobals(t)
	m := settingsTestModel(t)

	m.settingsModal.Reset()
	original := ColorPrimary

	it := m.settingsModal.Selected()
	for i := 0; i < len(ThemeNames()); i++ {
		it.Next(&m)
		if ColorPrimary != original {
			break
		}
	}
	if ColorPrimary == original {
		t.Skip("no palette in the corpus differs from the default Primary")
	}
	picked := ColorPrimary

	m.openModal(ModalSettings)
	m = m.handleSettingsModalKeys(tea.KeyPressMsg{Code: tea.KeyEnter})

	if ColorPrimary != picked {
		t.Error("enter reverted the palette; keep and cancel are indistinguishable")
	}
}

// TestSettingsMenuRoutes covers the menu's whole job: every entry reaches the
// surface it names. A menu entry that silently goes nowhere is worse than no
// menu, because esc is the key a user presses when they are already lost.
func TestSettingsMenuRoutes(t *testing.T) {
	restoreThemeGlobals(t)

	for _, tc := range []struct {
		entry int
		want  ModalType
		name  string
	}{
		{menuOptions, ModalSettings, "OPTIONS"},
		{menuHelp, ModalHelp, "HELP"},
		{menuQuit, ModalQuitConfirm, "QUIT"},
	} {
		m := settingsTestModel(t)
		m.openModal(ModalSettingsMenu)
		m.settingsMenu.selected = tc.entry
		m = m.handleSettingsMenuKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.activeModal != tc.want {
			t.Errorf("%s routed to modal %v, want %v", tc.name, m.activeModal, tc.want)
		}
	}
}

// TestSettingsMenuWraps pins the wrap. Three entries is short enough that
// stopping at the ends costs more keystrokes than it saves mistakes.
func TestSettingsMenuWraps(t *testing.T) {
	s := NewSettingsMenuModel(DefaultTheme())
	s.MoveUp()
	if got := s.SelectedIndex(); got != menuQuit {
		t.Errorf("up from the first entry = %d, want %d (wrap to last)", got, menuQuit)
	}
	s.MoveDown()
	if got := s.SelectedIndex(); got != menuOptions {
		t.Errorf("down from the last entry = %d, want %d (wrap to first)", got, menuOptions)
	}
}

// TestSettingsViewRendersWithoutPanic guards the layout math at the sizes that
// break panels: a terminal narrow enough that the two columns collide.
func TestSettingsViewRendersWithoutPanic(t *testing.T) {
	restoreThemeGlobals(t)
	for _, size := range []struct{ w, h int }{
		{120, 40}, {80, 24}, {60, 20}, {40, 14}, {20, 8},
	} {
		s := NewSettingsModalModel(DefaultTheme())
		s.SetSize(size.w, size.h)
		if out := s.View(); out == "" {
			t.Errorf("options %dx%d rendered empty", size.w, size.h)
		}

		menu := NewSettingsMenuModel(DefaultTheme())
		menu.SetSize(size.w, size.h)
		if out := menu.View(); out == "" {
			t.Errorf("menu %dx%d rendered empty", size.w, size.h)
		}
	}
}
