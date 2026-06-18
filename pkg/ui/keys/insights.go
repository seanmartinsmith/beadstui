package keys

import "charm.land/bubbles/v2/key"

// InsightsKeys are the bindings available when the insights panel is focused.
// handleInsightsKeys dispatches against these via key.Matches; help surfaces
// consume them via ShortHelp / FullHelp.
//
// Panel nav (h/l/Tab) and item nav (j/k) follow the arrows-primary /
// vim-alternate convention set in .2 (ADR-004 Decision 1). Up/Down field
// names and Help.Key strings match ListNormalKeys for the universal-nav
// consistency test.
//
// The exit binding is named Exit (not Back) to avoid triggering the
// universal-nav consistency check; TreeKeys.Back uses "E/esc" whereas
// insights exit is plain "esc" only.
type InsightsKeys struct {
	// Panel navigation
	PrevPanel key.Binding
	NextPanel key.Binding

	// Item navigation
	Up   key.Binding
	Down key.Binding

	// Detail scroll
	ScrollDetailDown key.Binding
	ScrollDetailUp   key.Binding

	// Toggles
	Explanations key.Binding
	Calculation  key.Binding
	Heatmap      key.Binding

	// Actions
	JumpToIssue key.Binding
	Exit        key.Binding
}

// NewInsightsKeys returns the default insights panel keymap.
func NewInsightsKeys() InsightsKeys {
	return InsightsKeys{
		PrevPanel: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "prev panel"),
		),
		NextPanel: key.NewBinding(
			key.WithKeys("right", "l", "tab"),
			key.WithHelp("→/l/⇥", "next panel"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		ScrollDetailDown: key.NewBinding(
			key.WithKeys("ctrl+j"),
			key.WithHelp("⌃j", "scroll detail down"),
		),
		ScrollDetailUp: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("⌃k", "scroll detail up"),
		),
		Explanations: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "toggle explanations"),
		),
		Calculation: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "toggle calc proof"),
		),
		Heatmap: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "toggle heatmap"),
		),
		JumpToIssue: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "jump to issue"),
		),
		Exit: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back to list"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
// Panel nav and item nav first as the primary orientation actions;
// JumpToIssue and Exit are the resolution actions.
func (k InsightsKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.PrevPanel, k.NextPanel, k.Up, k.Down, k.JumpToIssue, k.Exit}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Panel & Item Nav / Detail Scroll / Toggles & Actions.
func (k InsightsKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Panel & item navigation
		{k.PrevPanel, k.NextPanel, k.Up, k.Down},
		// Detail scroll
		{k.ScrollDetailDown, k.ScrollDetailUp},
		// Toggles & actions
		{k.Explanations, k.Calculation, k.Heatmap, k.JumpToIssue, k.Exit},
	}
}
