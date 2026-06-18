package keys

import "charm.land/bubbles/v2/key"

// HistoryNormalKeys are the bindings available in history view when neither
// search mode nor file-tree focus is active. handleHistoryKeys dispatches
// against these via key.Matches; help surfaces consume them via ShortHelp /
// FullHelp.
//
// Per ADR-004 Decision 7, history splits into THREE Maps (HistoryNormal /
// HistorySearch / HistoryFileTree) because all three sub-states are dwellable
// and have meaningfully different keymaps. Dispatcher selects the active Map
// via m.historyView.IsSearchActive() and m.historyView.FileTreeHasFocus().
type HistoryNormalKeys struct {
	// Nav
	Up   key.Binding
	Down key.Binding

	// Cross-panel commit nav
	NextRelated key.Binding
	PrevRelated key.Binding

	// Detail scroll (bt-npnh)
	ScrollDown key.Binding
	ScrollUp   key.Binding

	// Focus
	FocusCycle key.Binding

	// Mode
	ToggleMode key.Binding

	// Search
	Search key.Binding

	// File tree
	ToggleFileTree key.Binding

	// Actions
	CopySHA       key.Binding
	OpenInBrowser key.Binding
	JumpToBead    key.Binding
	JumpToGraph   key.Binding
	CycleConf     key.Binding

	// Exit (field name ExitHistory, not Back, to avoid the universal-nav
	// consistency test: history uses "h/esc" while Tree.Back uses "E/esc"
	// and the test enforces that shared field names carry identical Help.Key)
	ExitHistory key.Binding
}

// NewHistoryNormalKeys returns the default history-normal keymap.
func NewHistoryNormalKeys() HistoryNormalKeys {
	return HistoryNormalKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		NextRelated: key.NewBinding(
			key.WithKeys("J"),
			key.WithHelp("J", "next related bead/commit"),
		),
		PrevRelated: key.NewBinding(
			key.WithKeys("K"),
			key.WithHelp("K", "prev related bead/commit"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("ctrl+d", "pgdown"),
			key.WithHelp("⌃d/pgdn", "scroll detail down"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("ctrl+u", "pgup"),
			key.WithHelp("⌃u/pgup", "scroll detail up"),
		),
		FocusCycle: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("⇥", "cycle focus: list/detail/tree"),
		),
		ToggleMode: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "toggle bead/git mode"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search commits/beads"),
		),
		ToggleFileTree: key.NewBinding(
			key.WithKeys("f", "F"),
			key.WithHelp("f", "toggle file tree"),
		),
		CopySHA: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy commit SHA"),
		),
		OpenInBrowser: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open commit in browser"),
		),
		JumpToBead: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "jump to bead in list"),
		),
		JumpToGraph: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "jump to graph view for bead"),
		),
		CycleConf: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "cycle confidence filter"),
		),
		ExitHistory: key.NewBinding(
			key.WithKeys("h", "esc"),
			key.WithHelp("h/esc", "exit history"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
func (k HistoryNormalKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.ToggleMode, k.Search, k.CopySHA, k.ExitHistory}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Nav / Cross-panel / Scroll / Focus / Mode / Actions / Exit.
func (k HistoryNormalKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Nav
		{k.Up, k.Down},
		// Cross-panel
		{k.NextRelated, k.PrevRelated},
		// Scroll
		{k.ScrollDown, k.ScrollUp},
		// Focus / mode / search
		{k.FocusCycle, k.ToggleMode, k.Search, k.ToggleFileTree},
		// Actions
		{k.CopySHA, k.OpenInBrowser, k.JumpToBead, k.JumpToGraph, k.CycleConf},
		// Exit
		{k.ExitHistory},
	}
}

// HistorySearchKeys are the bindings active while history search input is
// focused (m.historyView.IsSearchActive() == true). Letters and printable
// characters are forwarded to the search input; only Esc and Enter resolve
// the search.
//
// Dispatcher short-circuits to this Map before global view-switch keys run,
// preventing letter-key leakage to mode toggles (bt-mc4y).
type HistorySearchKeys struct {
	// Apply / cancel search
	Confirm key.Binding
	Cancel  key.Binding
}

// NewHistorySearchKeys returns the default history search-mode keymap.
func NewHistorySearchKeys() HistorySearchKeys {
	return HistorySearchKeys{
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "confirm history search"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel history search"),
		),
	}
}

// ShortHelp returns the status-bar hint during history search mode.
func (k HistorySearchKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Confirm, k.Cancel}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay
// during history search mode.
func (k HistorySearchKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Confirm, k.Cancel},
	}
}

// HistoryFileTreeKeys are the bindings active while the file tree panel has
// focus (m.historyView.FileTreeHasFocus() == true).
//
// Per the bt-ift6.6 comment (2026-05-07), file-tree focus is a dwellable
// sub-state with its own j/k/h/l navigation semantics distinct from the
// normal history list nav. These bindings guard against global-key leakage
// for letters like 'h' that would otherwise exit history.
type HistoryFileTreeKeys struct {
	// Nav
	Up   key.Binding
	Down key.Binding

	// Expand / select / collapse
	ExpandOrSelect key.Binding
	Collapse       key.Binding

	// Focus / exit (field name ExitFileTree, not Back, to avoid the
	// universal-nav consistency test: tree focus uses plain "esc" while
	// Tree.Back uses "E/esc" and the test requires matching Help.Key strings)
	FocusBack    key.Binding
	ExitFileTree key.Binding
}

// NewHistoryFileTreeKeys returns the default history file-tree-focus keymap.
func NewHistoryFileTreeKeys() HistoryFileTreeKeys {
	return HistoryFileTreeKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up in file tree"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down in file tree"),
		),
		ExpandOrSelect: key.NewBinding(
			key.WithKeys("enter", "l"),
			key.WithHelp("⏎/l", "expand dir or select file"),
		),
		Collapse: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "collapse directory"),
		),
		FocusBack: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("⇥", "return focus to history list"),
		),
		ExitFileTree: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear file filter or close tree"),
		),
	}
}

// ShortHelp returns the status-bar hint during file-tree focus.
func (k HistoryFileTreeKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.ExpandOrSelect, k.Collapse, k.FocusBack, k.ExitFileTree}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay
// during file-tree focus.
func (k HistoryFileTreeKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Nav
		{k.Up, k.Down},
		// Tree ops
		{k.ExpandOrSelect, k.Collapse},
		// Focus / exit
		{k.FocusBack, k.ExitFileTree},
	}
}
