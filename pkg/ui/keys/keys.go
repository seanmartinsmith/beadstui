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
// upcoming HistoryNormal/HistorySearch/HistoryFileTree (.6),
// BoardNormal/BoardSearch (.3), and LabelPickerNav/LabelPickerSearch (.9).
//
// bt-ift6.1 wired Global + Tree at minimum. bt-ift6.2 adds ListNormal +
// ListSearch; bt-ift6.3-.9 add the rest.
type AppKeys struct {
	Global     GlobalKeys
	Tree       TreeKeys
	ListNormal ListNormalKeys
	ListSearch ListSearchKeys
}

// NewAppKeys returns the default keymap for every view. Wire into NewModel.
func NewAppKeys() AppKeys {
	return AppKeys{
		Global:     NewGlobalKeys(),
		Tree:       NewTreeKeys(),
		ListNormal: NewListNormalKeys(),
		ListSearch: NewListSearchKeys(),
	}
}
