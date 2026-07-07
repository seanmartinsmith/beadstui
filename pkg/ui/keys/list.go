package keys

import "charm.land/bubbles/v2/key"

// ListNormalKeys are the bindings available when the main list is focused
// and not in filter mode. handleListKeys dispatches against these via
// key.Matches; help surfaces consume them via ShortHelp / FullHelp.
//
// Universal nav (Up / Down) is declared here per ADR-004 Decision 1
// [v1 — revisit]. Help-only — the bubbles list owns dispatch for arrow /
// vim nav — but declared so the help surface is truthful.
//
// Tab / < / > sit on this Map (not on GlobalKeys) per ADR-004 Decision 1's
// no-match-and-fall-through rule. Each view that wants split-view detail-
// toggle / pane-resize declares its own bindings.
//
// History (h) is intentionally absent — it lives on GlobalKeys.History
// because handleListKeys cannot return tea.Cmd and the history switch
// dispatches an async LoadHistoryCmd (bt-uizm).
//
// Convention (per ADR-004 Decision 1, set by ListKeys in bt-ift6.2):
// arrows-primary, vim-keys-as-alternate. WithKeys lists arrow first, vim
// second; help text shows arrow first ("↑/k", not "k/↑").
type ListNormalKeys struct {
	// Move
	Up         key.Binding
	Down       key.Binding
	JumpTop    key.Binding
	JumpBottom key.Binding
	PageDown   key.Binding
	PageUp     key.Binding

	// Filter (recipe-style filters that aren't the bubbles fuzzy filter — those
	// are entered via / and live in ListSearchKeys' state).
	//
	// No FilterAll binding: 'a' is GlobalKeys.Actionable and the global
	// view-switch shadows any list-scoped 'a' before this handler runs.
	// The pre-.2 handleListKeys had a dead `case "a"` for FilterAll that
	// was never reachable; .2 dropped it. Reset-to-all stays reachable by
	// pressing the active filter key again (o/c/r toggle to "all").
	FilterOpen   key.Binding
	FilterClosed key.Binding
	FilterReady  key.Binding

	// Detail & pane
	Enter            key.Binding
	EpicCard         key.Binding
	SplitFocusToggle key.Binding
	SplitShrinkLeft  key.Binding
	SplitShrinkRight key.Binding

	// Field edit (bt-oiaj.5). 'e' opens the field-select modal; it moved off
	// EpicCard (bt-oiaj.13 wave key migration - see
	// docs/plans/2026-07-07-bt-edits-wave-oiaj13-5-6.md) so 'e' is free for
	// the ratified edit binding (tkhq #1: "e reserved globally for edit").
	FieldEdit key.Binding

	// Sort / triage
	CycleSort        key.Binding
	CycleSortReverse key.Binding
	RecipeTriage     key.Binding

	// Time travel
	TimeTravelInput key.Binding

	// Actions
	CopyID       key.Binding
	CopyIssue    key.Binding
	OpenInEditor key.Binding
	CassSession  key.Binding
	SelfUpdate   key.Binding

	// Write verbs (bt-oiaj.10). Claim is the first bt write to ship: it shells
	// out `bd update <id> --claim` behind a confirm. 'm' = "claim it for me"
	// (the only free letter in "claim"; c/l/a/i are all taken).
	Claim key.Binding
}

// NewListNormalKeys returns the default list (non-filter-mode) keymap.
func NewListNormalKeys() ListNormalKeys {
	return ListNormalKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		JumpTop: key.NewBinding(
			key.WithKeys("home"),
			key.WithHelp("home", "jump to top"),
		),
		JumpBottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G/end", "jump to bottom"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("⌃d", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("⌃u", "page up"),
		),

		FilterOpen: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open issues"),
		),
		FilterClosed: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "closed issues"),
		),
		FilterReady: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "ready (no blockers)"),
		),

		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "open detail"),
		),
		EpicCard: key.NewBinding(
			key.WithKeys("F"),
			key.WithHelp("F", "epic card"),
		),
		SplitFocusToggle: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("⇥", "toggle split focus"),
		),
		FieldEdit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit field"),
		),
		SplitShrinkLeft: key.NewBinding(
			key.WithKeys("<"),
			key.WithHelp("<", "shrink list pane"),
		),
		SplitShrinkRight: key.NewBinding(
			key.WithKeys(">"),
			key.WithHelp(">", "expand list pane"),
		),

		CycleSort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "cycle sort"),
		),
		CycleSortReverse: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "cycle sort reverse"),
		),
		RecipeTriage: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "triage recipe"),
		),

		TimeTravelInput: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "time travel"),
		),

		CopyID: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy ID"),
		),
		CopyIssue: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "copy issue"),
		),
		OpenInEditor: key.NewBinding(
			key.WithKeys("O"),
			key.WithHelp("O", "open in editor"),
		),
		CassSession: key.NewBinding(
			key.WithKeys("V"),
			key.WithHelp("V", "cass session"),
		),
		SelfUpdate: key.NewBinding(
			key.WithKeys("U"),
			key.WithHelp("U", "self-update"),
		),

		Claim: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "claim (assign to me)"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
// Most-used / most-orienting first; arrow-nav is intrinsic so it's not
// repeated here in favor of action-driving bindings.
func (k ListNormalKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.FilterOpen, k.FilterClosed, k.FilterReady, k.CycleSort, k.CopyID}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Move / Filter / Detail & Pane / Sort / Time / Actions.
func (k ListNormalKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Move
		{k.Up, k.Down, k.JumpTop, k.JumpBottom, k.PageDown, k.PageUp},
		// Filter
		{k.FilterOpen, k.FilterClosed, k.FilterReady},
		// Detail & pane
		{k.Enter, k.EpicCard, k.SplitFocusToggle, k.SplitShrinkLeft, k.SplitShrinkRight},
		// Sort / triage
		{k.CycleSort, k.CycleSortReverse, k.RecipeTriage},
		// Time travel
		{k.TimeTravelInput},
		// Actions
		{k.CopyID, k.CopyIssue, k.OpenInEditor, k.CassSession, k.SelfUpdate, k.Claim, k.FieldEdit},
	}
}

// ListSearchKeys is the help-only Map for the list's filter-typing sub-state
// (m.list.FilterState() == list.Filtering). The bubbles list owns dispatch
// for filter typing, result navigation, apply, and cancel; this Map exists
// so the help surface is truthful in that state — Decision 7's principle
// applied at the within-handler level.
//
// Globals that pre-empt the filter-state guard (Ctrl+S search-mode cycle,
// H hybrid preset, Ctrl+C quit) are intentionally not redeclared here. They
// surface via GlobalKeys.FullHelp() concat in help surfaces; redeclaring
// them would create a drift surface.
//
// Up/Down field names match ListNormalKeys / TreeKeys for the universal-nav
// consistency test (same Help.Key strings). Apply / Cancel use field names
// outside the universal list so they don't trigger comparison.
type ListSearchKeys struct {
	Up     key.Binding
	Down   key.Binding
	Apply  key.Binding
	Cancel key.Binding
}

// NewListSearchKeys returns the help-only filter-mode keymap.
func NewListSearchKeys() ListSearchKeys {
	return ListSearchKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "prev result"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "next result"),
		),
		Apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "apply filter"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel filter"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot
// during filter mode. Apply / Cancel first because they're the resolution
// actions; result-nav is intrinsic.
func (k ListSearchKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Apply, k.Cancel, k.Up, k.Down}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay
// during filter mode. Columns: Move / Resolve.
func (k ListSearchKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Apply, k.Cancel},
	}
}
