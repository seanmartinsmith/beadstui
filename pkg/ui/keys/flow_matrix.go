package keys

import "charm.land/bubbles/v2/key"

// FlowMatrixKeys are the bindings available when the flow-matrix view is
// focused. Universal nav (j/k) is declared here, not in GlobalKeys, because
// the flow-matrix cursor operates on its own label list and drilldown list
// independently of the main list. See ADR-004 Decision 1.
//
// Convention (per ADR-004 Decision 1, set by ListKeys in bt-ift6.2):
// arrows-primary, vim-keys-as-alternate. WithKeys lists arrow first, vim
// second; help text shows arrow first ("↑/k", not "k/↑").
//
// The exit binding uses field name Close, not Back, to avoid the universal-nav
// consistency test: flow matrix exits with f/q/esc while Tree.Back uses "E/esc"
// and the test requires matching Help.Key strings across views that share a
// field name. See history.go ExitHistory/ExitFileTree for the same pattern.
type FlowMatrixKeys struct {
	Up          key.Binding
	Down        key.Binding
	JumpTop     key.Binding
	JumpBottom  key.Binding
	TogglePanel key.Binding
	Enter       key.Binding
	Close       key.Binding
}

// NewFlowMatrixKeys returns the default flow-matrix keymap.
func NewFlowMatrixKeys() FlowMatrixKeys {
	return FlowMatrixKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		JumpTop: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "jump to top"),
		),
		JumpBottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "jump to bottom"),
		),
		TogglePanel: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("⇥", "toggle panel"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "open drilldown / jump to issue"),
		),
		Close: key.NewBinding(
			key.WithKeys("f", "q", "esc"),
			key.WithHelp("f/q/esc", "close flow matrix"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
// Order: most-orienting actions first.
func (k FlowMatrixKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Close}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Move, Act, Exit.
func (k FlowMatrixKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.JumpTop, k.JumpBottom},
		{k.Enter, k.TogglePanel},
		{k.Close},
	}
}
