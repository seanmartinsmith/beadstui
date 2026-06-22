package ui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

func sidebarTestTheme() Theme {
	return Theme{
		Primary:   lipgloss.Color("#00ff00"),
		Secondary: lipgloss.Color("#888888"),
		Base:      lipgloss.NewStyle(),
	}
}

// TestShortcutsSidebar_RendersBindingGroups verifies the sidebar renders the
// key + desc of every enabled binding fed to it via SetBindings, i.e. it is a
// FullHelp() consumer rather than a hardcoded string table (bt-ift6.10).
func TestShortcutsSidebar_RendersBindingGroups(t *testing.T) {
	sidebar := NewShortcutsSidebar(sidebarTestTheme())
	sidebar.SetSize(34, 40)
	sidebar.SetBindings([][]key.Binding{
		{
			key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
			key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
		},
		{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "open detail")),
		},
	})

	view := sidebar.View()
	for _, want := range []string{"↑/k", "move up", "↓/j", "move down", "⏎", "open detail"} {
		if !strings.Contains(view, want) {
			t.Errorf("sidebar view missing %q\n%s", want, view)
		}
	}
}

// TestShortcutsSidebar_SkipsDisabledAndEmptyBindings verifies disabled bindings
// and bindings with no help key are omitted from the rendered sidebar.
func TestShortcutsSidebar_SkipsDisabledAndEmptyBindings(t *testing.T) {
	sidebar := NewShortcutsSidebar(sidebarTestTheme())
	sidebar.SetSize(34, 40)

	disabled := key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "disabled action"))
	disabled.SetEnabled(false)
	noHelp := key.NewBinding(key.WithKeys("x")) // no WithHelp -> empty Help.Key
	enabled := key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "enabled action"))

	sidebar.SetBindings([][]key.Binding{{disabled, noHelp, enabled}})

	view := sidebar.View()
	if strings.Contains(view, "disabled action") {
		t.Errorf("sidebar should skip disabled bindings\n%s", view)
	}
	if !strings.Contains(view, "enabled action") {
		t.Errorf("sidebar should render enabled bindings\n%s", view)
	}
}

// TestSidebarHelpGroups_NonModalComposesGlobalAndView verifies the ; sidebar
// composes the global bindings plus the active view's bindings when no modal is
// open, so the single key.Map source feeds the surface (bt-ift6.10).
func TestSidebarHelpGroups_NonModalComposesGlobalAndView(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.mode = ViewList
	m.activeModal = ModalNone

	descs := map[string]bool{}
	for _, g := range m.sidebarHelpGroups() {
		for _, b := range g {
			descs[b.Help().Desc] = true
		}
	}
	if !descs["board"] {
		t.Errorf("expected a global binding (board) in non-modal sidebar groups")
	}
	if !descs["cycle sort"] {
		t.Errorf("expected a list binding (cycle sort) in non-modal sidebar groups")
	}
}

// TestSidebarHelpGroups_ModalShowsModalOnly verifies the ; sidebar shows the
// active modal's own bindings and not the global view-switch keys while a modal
// is open (modals own the sidebar per ADR-004 Decision 4).
func TestSidebarHelpGroups_ModalShowsModalOnly(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.mode = ViewList
	m.activeModal = ModalRecipePicker

	descs := map[string]bool{}
	for _, g := range m.sidebarHelpGroups() {
		for _, b := range g {
			descs[b.Help().Desc] = true
		}
	}
	if descs["board"] {
		t.Errorf("global view-switch (board) must not appear while a modal owns the sidebar")
	}
	if !descs["apply recipe"] {
		t.Errorf("expected a recipe-picker binding (apply recipe) in modal sidebar groups")
	}
}

func TestNewShortcutsSidebar(t *testing.T) {
	theme := DefaultTheme()
	sidebar := NewShortcutsSidebar(theme)

	if sidebar.width != 34 {
		t.Errorf("Expected width 34, got %d", sidebar.width)
	}
}

func TestShortcutsSidebarScrolling(t *testing.T) {
	theme := DefaultTheme()
	sidebar := NewShortcutsSidebar(theme)

	// Initial scroll offset should be 0
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected initial scroll 0, got %d", sidebar.scrollOffset)
	}

	// Scroll down
	sidebar.ScrollDown()
	if sidebar.scrollOffset != 1 {
		t.Errorf("Expected scroll 1 after ScrollDown, got %d", sidebar.scrollOffset)
	}

	// Scroll up
	sidebar.ScrollUp()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 after ScrollUp, got %d", sidebar.scrollOffset)
	}

	// Scroll up at top should stay at 0
	sidebar.ScrollUp()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 at top, got %d", sidebar.scrollOffset)
	}

	// Page down
	sidebar.ScrollPageDown()
	if sidebar.scrollOffset != 10 {
		t.Errorf("Expected scroll 10 after PageDown, got %d", sidebar.scrollOffset)
	}

	// Page up
	sidebar.ScrollPageUp()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 after PageUp, got %d", sidebar.scrollOffset)
	}

	// Reset
	sidebar.scrollOffset = 5
	sidebar.ResetScroll()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 after Reset, got %d", sidebar.scrollOffset)
	}
}

func TestShortcutsSidebarView(t *testing.T) {
	sidebar := NewShortcutsSidebar(sidebarTestTheme())
	sidebar.SetSize(28, 30)
	sidebar.SetBindings([][]key.Binding{
		{key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "open detail"))},
	})

	view := sidebar.View()
	if view == "" {
		t.Error("Expected non-empty view")
	}

	// Should contain the panel title (rendered in the top border).
	if !strings.Contains(view, "Shortcuts") {
		t.Error("Expected view to contain 'Shortcuts'")
	}

	// Should render the fed binding's desc.
	if !strings.Contains(view, "open detail") {
		t.Error("Expected view to render the supplied binding desc")
	}
}

func TestShortcutsSidebarWidth(t *testing.T) {
	theme := DefaultTheme()
	sidebar := NewShortcutsSidebar(theme)

	if sidebar.Width() != 34 {
		t.Errorf("Expected Width() = 34, got %d", sidebar.Width())
	}
}
