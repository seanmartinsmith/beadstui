package keys

import "charm.land/bubbles/v2/key"

// TreeKeys are the bindings available when the hierarchical tree view is
// focused. Universal nav (j/k/h/l) is declared here, not in GlobalKeys,
// because the tree's h/l semantics (collapse-to-parent / expand-to-child)
// are tree-specific. See ADR-004 Decision 1.
//
// Convention (per ADR-004 Decision 1, set by ListKeys in bt-ift6.2):
// arrows-primary, vim-keys-as-alternate. WithKeys lists arrow first, vim
// second; help text shows arrow first ("↑/k", not "k/↑").
type TreeKeys struct {
	Up          key.Binding
	Down        key.Binding
	Collapse    key.Binding
	Expand      key.Binding
	Toggle      key.Binding
	JumpTop     key.Binding
	JumpBottom  key.Binding
	ExpandAll   key.Binding
	CollapseAll key.Binding
	PageDown    key.Binding
	PageUp      key.Binding
	SyncDetail  key.Binding
	Back        key.Binding
}

// NewTreeKeys returns the default tree keymap.
func NewTreeKeys() TreeKeys {
	return TreeKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Collapse: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "collapse / jump to parent"),
		),
		Expand: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "expand / move to child"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("enter", "space"),
			key.WithHelp("⏎/␣", "toggle expand"),
		),
		JumpTop: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "jump to top"),
		),
		JumpBottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "jump to bottom"),
		),
		ExpandAll: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "expand all"),
		),
		CollapseAll: key.NewBinding(
			key.WithKeys("O"),
			key.WithHelp("O", "collapse all"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("ctrl+d", "pgdown"),
			key.WithHelp("⌃d", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("ctrl+u", "pgup"),
			key.WithHelp("⌃u", "page up"),
		),
		SyncDetail: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("⇥", "sync to detail pane"),
		),
		Back: key.NewBinding(
			key.WithKeys("T", "esc"),
			key.WithHelp("T/esc", "back to list"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
// Order matters: most ergonomic / most-used first.
func (k TreeKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Toggle, k.Back}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Move, Operate, Page, Exit.
func (k TreeKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Collapse, k.Expand, k.JumpTop, k.JumpBottom},
		{k.Toggle, k.ExpandAll, k.CollapseAll, k.SyncDetail},
		{k.PageDown, k.PageUp},
		{k.Back},
	}
}
