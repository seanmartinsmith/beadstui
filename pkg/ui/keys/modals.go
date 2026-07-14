package keys

import "charm.land/bubbles/v2/key"

// LabelPickerNavKeys are the bindings for label picker nav sub-state --
// when the search input is NOT focused. handleLabelPickerNavKeys dispatches
// against these via key.Matches.
//
// Per ADR-004 Decision 7, LabelPicker splits into Nav + Search sub-states
// because the two modes have distinct per-key semantics (letters are nav
// no-ops in nav mode; they type into the search bar in search mode).
// Dispatcher selects the active Map via m.labelPicker.IsSearchFocused().
//
// Up/Down field names and Help.Key strings match ListNormalKeys / TreeKeys
// per the universal-nav consistency test (TestUniversalNav_ConsistentAcrossViews).
type LabelPickerNavKeys struct {
	// Nav
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding

	// Mode
	FocusSearch key.Binding

	// Selection
	Toggle key.Binding
	Apply  key.Binding

	// Exit (Close is the toggle key `l`; Cancel is esc)
	Close  key.Binding
	Cancel key.Binding
}

// NewLabelPickerNavKeys returns the default label picker nav-mode keymap.
func NewLabelPickerNavKeys() LabelPickerNavKeys {
	return LabelPickerNavKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("left", "pgup"),
			key.WithHelp("←/pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("right", "pgdown"),
			key.WithHelp("→/pgdn", "page down"),
		),
		FocusSearch: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "enter search"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("space"),
			key.WithHelp("spc", "toggle label"),
		),
		Apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "apply label filter"),
		),
		Close: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "close picker"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close picker"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot.
func (k LabelPickerNavKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Toggle, k.Apply, k.FocusSearch, k.Cancel}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
func (k LabelPickerNavKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Nav
		{k.Up, k.Down, k.PageUp, k.PageDown},
		// Selection / apply
		{k.Toggle, k.Apply},
		// Mode / exit
		{k.FocusSearch, k.Close, k.Cancel},
	}
}

// LabelPickerSearchKeys are the bindings for label picker search sub-state --
// when the search input IS focused. Letter keys and printable characters are
// forwarded to the text input; only control keys are matched here.
//
// Per ADR-004 Decision 7, this is the search sub-state sibling of
// LabelPickerNavKeys. Dispatcher selects it when m.labelPicker.IsSearchFocused().
//
// ResultUp/ResultDown use different field names from Up/Down to avoid
// the universal-nav consistency test checking them (their Help.Key strings
// include context text not found in the nav siblings).
type LabelPickerSearchKeys struct {
	// Result navigation (field names distinct from Up/Down to avoid
	// universal-nav consistency test)
	ResultUp   key.Binding
	ResultDown key.Binding

	// Resolve
	Apply      key.Binding
	BlurSearch key.Binding
}

// NewLabelPickerSearchKeys returns the default label picker search-mode keymap.
func NewLabelPickerSearchKeys() LabelPickerSearchKeys {
	return LabelPickerSearchKeys{
		ResultUp: key.NewBinding(
			key.WithKeys("up", "ctrl+p"),
			key.WithHelp("↑/⌃p", "prev result"),
		),
		ResultDown: key.NewBinding(
			key.WithKeys("down", "ctrl+n"),
			key.WithHelp("↓/⌃n", "next result"),
		),
		Apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "apply label filter"),
		),
		BlurSearch: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "exit search"),
		),
	}
}

// ShortHelp returns the status-bar hint during label picker search mode.
func (k LabelPickerSearchKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.ResultUp, k.ResultDown, k.Apply, k.BlurSearch}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay
// during label picker search mode.
func (k LabelPickerSearchKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.ResultUp, k.ResultDown},
		{k.Apply, k.BlurSearch},
	}
}

// RecipePickerKeys are the bindings for the recipe picker overlay.
// handleRecipePickerKeys dispatches against these via key.Matches.
//
// Up/Down field names match the universal-nav consistency test.
// Close is the `'` toggle key (same key opens and closes per bt-4l28).
// Cancel is esc.
type RecipePickerKeys struct {
	// Nav
	Up   key.Binding
	Down key.Binding

	// Apply
	Apply key.Binding

	// Exit (Close is the toggle key; Cancel is esc)
	Close  key.Binding
	Cancel key.Binding
}

// NewRecipePickerKeys returns the default recipe picker keymap.
func NewRecipePickerKeys() RecipePickerKeys {
	return RecipePickerKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "apply recipe"),
		),
		Close: key.NewBinding(
			key.WithKeys("'"),
			key.WithHelp("'", "close picker"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close picker"),
		),
	}
}

// ShortHelp returns the status-bar hint for the recipe picker.
func (k RecipePickerKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Apply, k.Cancel}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
func (k RecipePickerKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Nav
		{k.Up, k.Down},
		// Apply / exit
		{k.Apply, k.Close, k.Cancel},
	}
}

// BQLQueryKeys are the bindings for the BQL query modal. Letter keys are NOT
// matched here; the textinput component owns them via the default branch.
// handleBQLQueryKeys dispatches against these via key.Matches.
//
// HistoryPrev/HistoryNext use dedicated field names (not Up/Down) to avoid
// the universal-nav consistency test: their Help.Key strings include
// context text not shared with nav siblings.
type BQLQueryKeys struct {
	// History navigation (field names distinct from Up/Down to avoid
	// universal-nav consistency test)
	HistoryPrev key.Binding
	HistoryNext key.Binding

	// Apply / cancel
	Apply  key.Binding
	Cancel key.Binding
}

// NewBQLQueryKeys returns the default BQL query modal keymap.
func NewBQLQueryKeys() BQLQueryKeys {
	return BQLQueryKeys{
		HistoryPrev: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "prev query"),
		),
		HistoryNext: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "next query"),
		),
		Apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "run BQL query"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel query"),
		),
	}
}

// ShortHelp returns the status-bar hint for the BQL query modal.
func (k BQLQueryKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Apply, k.Cancel, k.HistoryPrev, k.HistoryNext}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
func (k BQLQueryKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.HistoryPrev, k.HistoryNext},
		{k.Apply, k.Cancel},
	}
}

// TimeTravelInputKeys are the bindings for the time-travel revision prompt.
// Letter keys are NOT matched here; the textinput component owns them via the
// default branch. handleTimeTravelInputKeys dispatches against these via
// key.Matches.
type TimeTravelInputKeys struct {
	Submit key.Binding
	Cancel key.Binding
}

// NewTimeTravelInputKeys returns the default time-travel input keymap.
func NewTimeTravelInputKeys() TimeTravelInputKeys {
	return TimeTravelInputKeys{
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "submit revision"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel time travel"),
		),
	}
}

// ShortHelp returns the status-bar hint for the time-travel input modal.
func (k TimeTravelInputKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Submit, k.Cancel}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
func (k TimeTravelInputKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Submit, k.Cancel},
	}
}

// RepoPickerNavKeys are the bindings for the repo picker nav sub-state --
// when the search input is NOT focused. handleRepoPickerNavKeys dispatches
// against these via key.Matches.
//
// Per the LabelPicker precedent (ADR-004 Decision 7), the repo picker splits
// into Nav + Search sub-states (Wave 2, bt-9lpib): letters are nav no-ops in
// nav mode; they type into the search bar in search mode. The dispatcher
// selects the active Map via m.repoPicker.IsSearchFocused().
//
// Up/Down field names match the universal-nav consistency test. PageUp/PageDown
// use dedicated field names (like LabelPickerNavKeys) so the ←/→ help strings
// do not trip the universal-nav Left/Right check.
// Close covers the additional exit keys q and w alongside Cancel (esc).
type RepoPickerNavKeys struct {
	// Nav
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding

	// Mode
	FocusSearch key.Binding

	// Selection
	Toggle    key.Binding
	ToggleAll key.Binding

	// Apply
	Apply key.Binding

	// Exit (Close is q/w; Cancel is esc)
	Close  key.Binding
	Cancel key.Binding
}

// NewRepoPickerNavKeys returns the default repo picker nav-mode keymap.
func NewRepoPickerNavKeys() RepoPickerNavKeys {
	return RepoPickerNavKeys{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("left", "pgup"),
			key.WithHelp("←/pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("right", "pgdown"),
			key.WithHelp("→/pgdn", "page down"),
		),
		FocusSearch: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "enter search"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("space"),
			key.WithHelp("spc", "toggle project"),
		),
		ToggleAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "toggle all projects"),
		),
		Apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "apply project filter"),
		),
		Close: key.NewBinding(
			key.WithKeys("q", "w"),
			key.WithHelp("q/w", "close picker"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "close picker"),
		),
	}
}

// ShortHelp returns the status-bar hint for the repo picker (nav mode).
func (k RepoPickerNavKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Toggle, k.Apply, k.FocusSearch, k.Cancel}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
func (k RepoPickerNavKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Nav
		{k.Up, k.Down, k.PageUp, k.PageDown},
		// Selection
		{k.Toggle, k.ToggleAll},
		// Mode / apply / exit
		{k.FocusSearch, k.Apply, k.Close, k.Cancel},
	}
}

// RepoPickerSearchKeys are the bindings for the repo picker search sub-state --
// when the search input IS focused. Letter keys and printable characters are
// forwarded to the text input; only control keys are matched here.
//
// Dispatcher selects it when m.repoPicker.IsSearchFocused(). ResultUp/ResultDown
// use different field names from Up/Down to avoid the universal-nav consistency
// test (their Help.Key strings include context text not found in the nav
// siblings).
type RepoPickerSearchKeys struct {
	// Result navigation (field names distinct from Up/Down to avoid
	// universal-nav consistency test)
	ResultUp   key.Binding
	ResultDown key.Binding

	// Resolve
	Apply      key.Binding
	BlurSearch key.Binding
}

// NewRepoPickerSearchKeys returns the default repo picker search-mode keymap.
func NewRepoPickerSearchKeys() RepoPickerSearchKeys {
	return RepoPickerSearchKeys{
		ResultUp: key.NewBinding(
			key.WithKeys("up", "ctrl+p"),
			key.WithHelp("↑/⌃p", "prev result"),
		),
		ResultDown: key.NewBinding(
			key.WithKeys("down", "ctrl+n"),
			key.WithHelp("↓/⌃n", "next result"),
		),
		Apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "apply project filter"),
		),
		BlurSearch: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "exit search"),
		),
	}
}

// ShortHelp returns the status-bar hint during repo picker search mode.
func (k RepoPickerSearchKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.ResultUp, k.ResultDown, k.Apply, k.BlurSearch}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay
// during repo picker search mode.
func (k RepoPickerSearchKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.ResultUp, k.ResultDown},
		{k.Apply, k.BlurSearch},
	}
}
