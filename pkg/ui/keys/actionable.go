package keys

import "charm.land/bubbles/v2/key"

// ActionableKeys are the bindings available when the actionable view is focused.
// handleActionableKeys dispatches against these via key.Matches; help surfaces
// consume them via ShortHelp / FullHelp.
//
// Convention (per ADR-004 Decision 1, established by ListKeys in bt-ift6.2):
// arrows-primary, vim-keys-as-alternate. WithKeys lists arrow first, vim
// second; help text shows arrow first ("↑/k", not "k/↑").
type ActionableKeys struct {
	// Navigate
	Up   key.Binding
	Down key.Binding

	// Select
	Enter key.Binding
}

// NewActionableKeys returns the default actionable-view keymap.
func NewActionableKeys() ActionableKeys {
	return ActionableKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "jump to issue in list"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
// Enter first (the primary action); nav follows.
func (k ActionableKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Up, k.Down}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Navigate / Select.
func (k ActionableKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigate
		{k.Up, k.Down},
		// Select
		{k.Enter},
	}
}
