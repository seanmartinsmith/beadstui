package keys

import "charm.land/bubbles/v2/key"

// GlobalKeys are the bindings that fire identically (or with uniform
// context-aware semantics) regardless of which view or focus is active.
// See ADR-004 Decision 1.
//
// Two cascade exceptions are documented inline: q and Esc are
// 9-/10-branch state cascades in the dispatcher, modeled as a single
// binding here with a single Help.Desc ("back / quit (context-aware)"
// and "back / cancel (context-aware)"). The cascade lives in the
// dispatcher's case body, NOT in the Map.
//
// Column layout (FullHelp): Help & Chrome / Views / Workspace / Actions.
type GlobalKeys struct {
	// Help & Chrome
	Help              key.Binding // ? / f1
	Sidebar           key.Binding // ; / f2
	SidebarScrollDown key.Binding // ctrl+j (gated on showShortcutsSidebar)
	SidebarScrollUp   key.Binding // ctrl+k (gated on showShortcutsSidebar)
	Tutorial          key.Binding // `
	Quit              key.Binding // ctrl+c
	Back              key.Binding // q (cascade)
	Cancel            key.Binding // esc (cascade)

	// Views
	Board          key.Binding // b
	Graph          key.Binding // g
	Insights       key.Binding // i
	History        key.Binding // h
	Actionable     key.Binding // a
	FlowMatrix     key.Binding // f
	Tree           key.Binding // T
	LabelDashboard key.Binding // [ / f3
	Attention      key.Binding // ] / f4
	Epics          key.Binding // E

	// Workspace
	ProjectsOrWisps  key.Binding // w
	WorkspaceHomeAll key.Binding // W
	HybridPreset     key.Binding // H (gated on focusList + hybrid mode)

	// Actions
	Refresh       key.Binding // ctrl+r / f5
	SearchMode    key.Binding // ctrl+s
	BQL           key.Binding // :
	Recipes       key.Binding // '
	Alerts        key.Binding // !
	Notifications key.Binding // 1 (gated on not ViewAttention)
	SearchBounce  key.Binding // / (gated on split + non-list focus)
	PriorityHints key.Binding // p
	Export        key.Binding // x
	LabelPicker   key.Binding // l
}

// NewGlobalKeys returns the default global keymap.
func NewGlobalKeys() GlobalKeys {
	return GlobalKeys{
		// Help & Chrome
		Help: key.NewBinding(
			key.WithKeys("?", "f1"),
			key.WithHelp("?", "help"),
		),
		Sidebar: key.NewBinding(
			key.WithKeys(";", "f2"),
			key.WithHelp(";", "shortcuts"),
		),
		SidebarScrollDown: key.NewBinding(
			key.WithKeys("ctrl+j"),
			key.WithHelp("⌃j", "sidebar down"),
		),
		SidebarScrollUp: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("⌃k", "sidebar up"),
		),
		Tutorial: key.NewBinding(
			key.WithKeys("`"),
			key.WithHelp("`", "tutorial"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("⌃c", "quit"),
		),
		Back: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "back / quit (context-aware)"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back / cancel (context-aware)"),
		),

		// Views
		Board: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "board"),
		),
		Graph: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "graph"),
		),
		Insights: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "insights"),
		),
		History: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "history"),
		),
		Actionable: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "actionable"),
		),
		FlowMatrix: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "flow matrix"),
		),
		Tree: key.NewBinding(
			key.WithKeys("T"),
			key.WithHelp("T", "tree"),
		),
		LabelDashboard: key.NewBinding(
			key.WithKeys("[", "f3"),
			key.WithHelp("[", "labels"),
		),
		Attention: key.NewBinding(
			key.WithKeys("]", "f4"),
			key.WithHelp("]", "attention"),
		),
		Epics: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "epics"),
		),

		// Workspace
		ProjectsOrWisps: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "projects / wisps"),
		),
		WorkspaceHomeAll: key.NewBinding(
			key.WithKeys("W"),
			key.WithHelp("W", "home / all projects"),
		),
		HybridPreset: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "hybrid preset"),
		),

		// Actions
		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r", "f5"),
			key.WithHelp("⌃r", "refresh"),
		),
		SearchMode: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("⌃s", "cycle search ranker"),
		),
		BQL: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "BQL query"),
		),
		Recipes: key.NewBinding(
			key.WithKeys("'"),
			key.WithHelp("'", "recipes"),
		),
		Alerts: key.NewBinding(
			key.WithKeys("!"),
			key.WithHelp("!", "alerts"),
		),
		Notifications: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "notifications"),
		),
		SearchBounce: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		PriorityHints: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "priority hints"),
		),
		Export: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "export markdown"),
		),
		LabelPicker: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "label picker"),
		),
	}
}

// ShortHelp returns the bindings shown in the status-bar L1 hint slot
// when no view-specific Map is active. Most-used / most-orienting first.
//
// Per ADR-004 Decision 1, L1 is "active view's ShortHelp() only" — globals
// are background context. This method exists for the modal-or-empty-view
// fallback path; populated views provide their own ShortHelp.
func (k GlobalKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Sidebar, k.Refresh, k.Quit}
}

// FullHelp returns column-grouped bindings for the ; sidebar and ? overlay.
// Columns: Help & Chrome / Views / Workspace / Actions.
func (k GlobalKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Help & Chrome
		{k.Help, k.Sidebar, k.SidebarScrollDown, k.SidebarScrollUp, k.Tutorial, k.Quit, k.Back, k.Cancel},
		// Views
		{k.Board, k.Graph, k.Insights, k.History, k.Actionable, k.FlowMatrix, k.Tree, k.LabelDashboard, k.Attention, k.Epics},
		// Workspace
		{k.ProjectsOrWisps, k.WorkspaceHomeAll, k.HybridPreset},
		// Actions
		{k.Refresh, k.SearchMode, k.BQL, k.Recipes, k.Alerts, k.Notifications, k.SearchBounce, k.PriorityHints, k.Export, k.LabelPicker},
	}
}
