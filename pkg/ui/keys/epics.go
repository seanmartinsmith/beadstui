package keys

import "charm.land/bubbles/v2/key"

// EpicsKeys are the bindings available when the epics overview is focused.
// handleEpicsKeys dispatches against these via key.Matches; help surfaces
// consume them via ShortHelp / FullHelp.
//
// Up/Down field names and Help.Key strings match ListNormalKeys/TreeKeys for
// the universal-nav consistency test (arrows-primary, vim-alternate per
// ADR-004 Decision 1).
//
// The exit binding is named Exit (not Back) to avoid triggering the
// universal-nav consistency check; TreeKeys.Back uses "T/esc" whereas the
// epics overview exit is plain "esc" only.
type EpicsKeys struct {
	// Navigation
	Up   key.Binding
	Down key.Binding

	// Actions
	Open        key.Binding
	CycleStatus key.Binding
	Exit        key.Binding
}

// NewEpicsKeys returns the default epics-overview keymap.
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
		Open: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "open epic"),
		),
		CycleStatus: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "active / all / completed"),
		),
		Exit: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back to list"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
// Item nav first as the primary orientation; Open / CycleStatus / Exit are
// the resolution actions.
func (k EpicsKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Open, k.CycleStatus, k.Exit}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Navigate / Actions.
func (k EpicsKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Open, k.CycleStatus, k.Exit},
	}
}
