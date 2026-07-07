package keys

import "charm.land/bubbles/v2/key"

// BoardNormalKeys are the bindings available when the board is focused and
// not in search mode. handleBoardKeys dispatches against these via
// key.Matches when m.board.IsSearchMode() == false.
//
// gg-combo (IsWaitingForG) is a conditional inside this Map (single
// keystroke, not a dwellable sub-state) per ADR-004 Decision 7.
//
// n/N (next/prev match) start Disabled; the dispatcher calls
// SetEnabled(true) on search-start and SetEnabled(false) on
// search-clear so help surfaces auto-hide them when no search is active,
// per ADR-004 Decision 3's case-by-case Disabled rule.
//
// Convention (per ADR-004 Decision 1, mirroring bt-ift6.2 ListKeys):
// arrows-primary, vim-keys-as-alternate. WithKeys lists arrow first, vim
// second; help text shows arrow first ("←/h", not "h/←").
type BoardNormalKeys struct {
	// Column nav
	Left  key.Binding
	Right key.Binding

	// Item nav
	Down     key.Binding
	Up       key.Binding
	PageDown key.Binding
	PageUp   key.Binding

	// Jump nav
	JumpTop    key.Binding
	JumpBottom key.Binding
	JumpFirst  key.Binding // H: jump to first column
	JumpLast   key.Binding // L: jump to last column

	// Digit column jumps (1-4) and dollar-to-last ($)
	// These are grouped together as positional jumps.
	JumpCol1   key.Binding
	JumpCol2   key.Binding
	JumpCol3   key.Binding
	JumpCol4   key.Binding
	JumpColEnd key.Binding

	// Vim gg combo (first g — handler sets IsWaitingForG; second g fires JumpTop)
	GotoTop key.Binding // g: start gg combo (IsWaitingForG conditional)

	// Search
	Search    key.Binding // /: enter search mode
	NextMatch key.Binding // n: next match (starts Disabled)
	PrevMatch key.Binding // N: prev match (starts Disabled)

	// Actions
	ToggleExpand key.Binding // d: inline card expand/collapse
	ToggleEmpty  key.Binding // z: cycle empty-column visibility (bt-oiaj.5 wave migration - was e; e is now the global field-edit binding, see docs/plans/2026-07-07-bt-edits-wave-oiaj13-5-6.md)
	CycleSwim    key.Binding // s: cycle swimlane mode
	CopyID       key.Binding // y: copy selected issue ID

	// Detail panel
	DetailToggle key.Binding // tab: show/hide detail panel
	DetailDown   key.Binding // ctrl+j: scroll detail panel down
	DetailUp     key.Binding // ctrl+k: scroll detail panel up

	// Open full detail view
	Enter key.Binding // enter: open in list detail view
}

// NewBoardNormalKeys returns the default board (non-search-mode) keymap.
// n/N start Disabled; caller enables them on search-start.
func NewBoardNormalKeys() BoardNormalKeys {
	nextMatch := key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "next match"),
	)
	nextMatch.SetEnabled(false)

	prevMatch := key.NewBinding(
		key.WithKeys("N"),
		key.WithHelp("N", "prev match"),
	)
	prevMatch.SetEnabled(false)

	return BoardNormalKeys{
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "prev column"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next column"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("⌃d", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("⌃u", "page up"),
		),
		JumpTop: key.NewBinding(
			key.WithKeys("home", "0"),
			key.WithHelp("home/0", "first card"),
		),
		JumpBottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G/end", "last card"),
		),
		JumpFirst: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "first column"),
		),
		JumpLast: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "last column"),
		),
		JumpCol1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "jump to col 1"),
		),
		JumpCol2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "jump to col 2"),
		),
		JumpCol3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "jump to col 3"),
		),
		JumpCol4: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "jump to col 4"),
		),
		JumpColEnd: key.NewBinding(
			key.WithKeys("$"),
			key.WithHelp("$", "last card in column"),
		),
		GotoTop: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("gg", "first card (gg)"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search cards"),
		),
		NextMatch: nextMatch,
		PrevMatch: prevMatch,
		ToggleExpand: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "expand card"),
		),
		ToggleEmpty: key.NewBinding(
			key.WithKeys("z"),
			key.WithHelp("z", "toggle empty columns"),
		),
		CycleSwim: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "cycle swimlane"),
		),
		CopyID: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy card ID"),
		),
		DetailToggle: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("⇥", "toggle detail panel"),
		),
		DetailDown: key.NewBinding(
			key.WithKeys("ctrl+j"),
			key.WithHelp("⌃j", "scroll detail down"),
		),
		DetailUp: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("⌃k", "scroll detail up"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "open full detail"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
// Most-used / most-orienting first.
func (k BoardNormalKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Search, k.CycleSwim, k.ToggleExpand, k.DetailToggle, k.CopyID}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Move / Jump / Search / Actions / Detail.
func (k BoardNormalKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Move
		{k.Left, k.Right, k.Down, k.Up, k.PageDown, k.PageUp},
		// Jump
		{k.JumpTop, k.JumpBottom, k.JumpFirst, k.JumpLast, k.GotoTop, k.JumpColEnd,
			k.JumpCol1, k.JumpCol2, k.JumpCol3, k.JumpCol4},
		// Search
		{k.Search, k.NextMatch, k.PrevMatch},
		// Actions
		{k.CycleSwim, k.ToggleExpand, k.ToggleEmpty, k.CopyID, k.Enter},
		// Detail panel
		{k.DetailToggle, k.DetailDown, k.DetailUp},
	}
}

// BoardSearchKeys is the keymap active when the board is in search mode
// (m.board.IsSearchMode() == true). The dispatcher routes all key input
// here when search mode is active, short-circuiting global view-switch
// keys so typed letters reach board.AppendSearchChar instead of firing
// view jumps (Decision 7 — the letter-leak bug documented in bt-ift6.3
// comments).
//
// Letter keys are NOT bindings here; they are the default catch-all
// (AppendSearchChar). Only the explicit control keys below are
// declared as bindings.
type BoardSearchKeys struct {
	Cancel    key.Binding // esc: cancel search, clear results
	Finish    key.Binding // enter: finish search, keep results for n/N nav
	Backspace key.Binding // backspace: delete last search char
	NextMatch key.Binding // n: next match
	PrevMatch key.Binding // N: prev match
}

// NewBoardSearchKeys returns the default board search-mode keymap.
func NewBoardSearchKeys() BoardSearchKeys {
	return BoardSearchKeys{
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel search"),
		),
		Finish: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "finish search"),
		),
		Backspace: key.NewBinding(
			key.WithKeys("backspace"),
			key.WithHelp("⌫", "delete char"),
		),
		NextMatch: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next match"),
		),
		PrevMatch: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev match"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot
// during search mode. Finish / Cancel first as resolution actions.
func (k BoardSearchKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Finish, k.Cancel, k.NextMatch, k.PrevMatch, k.Backspace}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay
// during search mode.
func (k BoardSearchKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Finish, k.Cancel, k.Backspace},
		{k.NextMatch, k.PrevMatch},
	}
}
