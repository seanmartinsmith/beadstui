package keys

import "charm.land/bubbles/v2/key"

// EpicsKeys are the bindings available when the epics tree is focused.
// handleEpicsKeys dispatches against these via key.Matches; help surfaces
// consume them via ShortHelp / FullHelp.
//
// Up/Down field names and Help.Key strings match ListNormalKeys/TreeKeys for
// the universal-nav consistency test (arrows-primary, vim-alternate per
// ADR-004 Decision 1). Expand/Collapse are deliberately NOT named
// Right/Left/Enter so they sit outside the universal-nav consistency contract
// (their key sets are tree-specific: enter is the context-sensitive Open).
//
// The exit binding is named Exit (not Back) to avoid triggering the
// universal-nav consistency check; TreeKeys.Back uses "T/esc" whereas the
// epics overview exit is plain "esc" only.
type EpicsKeys struct {
	// Navigation
	Up   key.Binding
	Down key.Binding

	// Tree
	Expand      key.Binding // expand epic/lane (and focus subtree)
	Collapse    key.Binding // collapse epic/lane (or jump to parent)
	CollapseAll key.Binding // collapse every epic back to the lane overview

	// Actions
	Open        key.Binding // enter: expand epic/header, or drill into a child
	CycleStatus key.Binding
	Card        key.Binding // open the single-epic focus card (zoom)
	Exit        key.Binding
}

// NewEpicsKeys returns the default epics-tree keymap.
func NewEpicsKeys() EpicsKeys {
	return EpicsKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Expand: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "expand"),
		),
		Collapse: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "collapse"),
		),
		CollapseAll: key.NewBinding(
			key.WithKeys("z"),
			key.WithHelp("z", "collapse all"),
		),
		Open: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "expand / drill"),
		),
		CycleStatus: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "active / all / completed"),
		),
		Card: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "zoom card"),
		),
		Exit: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back to list"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
// Item nav first as the primary orientation; expand/drill/zoom/exit are the
// resolution actions.
func (k EpicsKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Expand, k.Collapse, k.Open, k.CycleStatus, k.Card, k.Exit}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Navigate / Tree / Actions.
func (k EpicsKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Expand, k.Collapse, k.CollapseAll},
		{k.Open, k.CycleStatus, k.Card, k.Exit},
	}
}
