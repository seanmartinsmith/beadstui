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
// bt-ift6.1 wires Global + Tree at minimum (the AppKeys minimum-fields
// gate per ADR-004 Decision 4 / scope-guardian P2-S9). bt-ift6.2 adds
// List; bt-ift6.3-.9 add the rest.
type AppKeys struct {
	Global GlobalKeys
	Tree   TreeKeys
}

// NewAppKeys returns the default keymap for every view. Wire into NewModel.
func NewAppKeys() AppKeys {
	return AppKeys{
		Global: NewGlobalKeys(),
		Tree:   NewTreeKeys(),
	}
}
