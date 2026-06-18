package keys

import "charm.land/bubbles/v2/key"

// GraphKeys are the bindings available when the graph view is focused.
// handleGraphKeys dispatches against these via key.Matches; help surfaces
// consume them via ShortHelp / FullHelp.
//
// Convention (per ADR-004 Decision 1, set by ListKeys in bt-ift6.2):
// arrows-primary, vim-keys-as-alternate. WithKeys lists arrow first, vim
// second; help text shows arrow first ("←/h", not "h/←").
//
// Up / Down field names match ListNormalKeys / TreeKeys for the
// universal-nav consistency test (same Help.Key strings).
type GraphKeys struct {
	// Navigate
	Up        key.Binding
	Down      key.Binding
	MoveLeft  key.Binding
	MoveRight key.Binding
	PageDown  key.Binding
	PageUp    key.Binding

	// Actions
	JumpToIssue key.Binding
	SwarmToggle key.Binding
}

// NewGraphKeys returns the default graph keymap.
func NewGraphKeys() GraphKeys {
	return GraphKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		MoveLeft: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "move left"),
		),
		MoveRight: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "move right"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("ctrl+d", "pgdown"),
			key.WithHelp("⌃d", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("ctrl+u", "pgup"),
			key.WithHelp("⌃u", "page up"),
		),
		JumpToIssue: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "jump to issue"),
		),
		SwarmToggle: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "toggle swarm view"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
// Nav first, then the two action bindings.
func (k GraphKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.MoveLeft, k.MoveRight, k.JumpToIssue, k.SwarmToggle}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Navigate / Actions.
func (k GraphKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigate
		{k.Up, k.Down, k.MoveLeft, k.MoveRight, k.PageDown, k.PageUp},
		// Actions
		{k.JumpToIssue, k.SwarmToggle},
	}
}
