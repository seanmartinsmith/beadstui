// Package keys defines per-view key.Map types backing the unified dispatch
// and help-surface plumbing for bt's TUI. See ADR-004.
//
// Each per-view Map (TreeKeys, ListKeys, BoardKeys, ...) implements the
// help.KeyMap interface (ShortHelp, FullHelp) and is consumed by both the
// dispatcher (via key.Matches) and the help surfaces (status-bar L1, ;
// sidebar L1.5, ? overlay L2).
//
// AppKeys is the umbrella container the Model holds. Adding a new view's
// Map: add a field here and a constructor call in NewAppKeys, then add
// the same field name to allMaps() in keys_test.go so the registry
// completeness test stays in sync.
package keys

// AppKeys aggregates the GlobalKeys map plus every per-view key.Map.
// The Model embeds this so handlers can dispatch via m.keys.<View>.<Binding>
// and surfaces can look up the active map by name.
//
// Sub-state Maps are declared as flat siblings (e.g. ListNormal +
// ListSearch, not List.Normal + List.Search) per ADR-004 Decision 7. This
// keeps reflection-based tests one-level and matches Decision 7's "~15
// Maps total" cardinality: the .2 spine sets this convention for the
// HistoryNormal/HistorySearch/HistoryFileTree (.6) and
// BoardNormal/BoardSearch (.3) sub-state Maps.
//
// bt-ift6.1 wired Global + Tree at minimum. bt-ift6.2 adds ListNormal +
// ListSearch; bt-ift6.3 adds BoardNormal + BoardSearch; bt-ift6.4 adds
// Graph; bt-ift6.5 adds Insights; bt-ift6.6 adds HistoryNormal +
// HistorySearch + HistoryFileTree; bt-ift6.7 adds Actionable; bt-ift6.8
// adds FlowMatrix. bt-ift6.9 (modal Maps: LabelPicker*, RecipePicker,
// BQLQuery, TimeTravelInput, RepoPicker) is deferred pending a clean
// reimplementation - see the bead.
type AppKeys struct {
	Global     GlobalKeys
	Tree       TreeKeys
	ListNormal ListNormalKeys
	ListSearch ListSearchKeys

	// Board sub-state Maps (bt-ift6.3). Dispatcher selects via
	// m.board.IsSearchMode().
	BoardNormal BoardNormalKeys
	BoardSearch BoardSearchKeys

	// History sub-state Maps (bt-ift6.6). Dispatcher selects via
	// m.historyView.IsSearchActive() / m.historyView.FileTreeHasFocus().
	HistoryNormal   HistoryNormalKeys
	HistorySearch   HistorySearchKeys
	HistoryFileTree HistoryFileTreeKeys

	Insights   InsightsKeys
	Actionable ActionableKeys
	FlowMatrix FlowMatrixKeys
	Graph      GraphKeys
}

// NewAppKeys returns the default keymap for every view. Wire into NewModel.
func NewAppKeys() AppKeys {
	return AppKeys{
		Global:     NewGlobalKeys(),
		Tree:       NewTreeKeys(),
		ListNormal: NewListNormalKeys(),
		ListSearch: NewListSearchKeys(),

		// Board (bt-ift6.3)
		BoardNormal: NewBoardNormalKeys(),
		BoardSearch: NewBoardSearchKeys(),

		// History (bt-ift6.6)
		HistoryNormal:   NewHistoryNormalKeys(),
		HistorySearch:   NewHistorySearchKeys(),
		HistoryFileTree: NewHistoryFileTreeKeys(),

		Insights:   NewInsightsKeys(),
		Actionable: NewActionableKeys(),
		FlowMatrix: NewFlowMatrixKeys(),
		Graph:      NewGraphKeys(),
	}
}
