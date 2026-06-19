package keys

import "charm.land/bubbles/v2/key"

// EpicCardKeys are the bindings available when the tier-2 epic focus card
// (ModalEpicCard) is open. handleEpicCardKeys dispatches against these via
// key.Matches; help surfaces consume them via ShortHelp / FullHelp.
//
// Up/Down field names and Help.Key strings match ListNormalKeys / EpicsKeys
// for the universal-nav consistency test (arrows-primary, vim-alternate per
// ADR-004 Decision 1).
//
// The exit binding is named Exit (not Back) to avoid the universal-nav
// consistency check, mirroring EpicsKeys: the card closes on plain "esc".
type EpicCardKeys struct {
	// Navigation (over the epic's children)
	Up   key.Binding
	Down key.Binding

	// Actions
	Open key.Binding // drill into the selected child
	Exit key.Binding // close the card
}

// NewEpicCardKeys returns the default focus-card keymap.
func NewEpicCardKeys() EpicCardKeys {
	return EpicCardKeys{
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
			key.WithHelp("⏎", "drill into child"),
		),
		Exit: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close card"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
func (k EpicCardKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Open, k.Exit}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Navigate / Actions.
func (k EpicCardKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Open, k.Exit},
	}
}
