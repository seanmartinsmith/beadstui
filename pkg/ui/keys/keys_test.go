package keys

import (
	"reflect"
	"testing"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

// allMaps lists every help.KeyMap exposed by this package.
//
// TestAllMapsRegistered_MatchesAppKeys below validates this stays in sync
// with the AppKeys struct via reflection — adding a Map field to AppKeys
// without adding it here is a test failure (covers the audit's class:
// silent test passes when registries hand-maintain).
func allMaps() map[string]help.KeyMap {
	return map[string]help.KeyMap{
		"Global":     NewGlobalKeys(),
		"Tree":       NewTreeKeys(),
		"ListNormal": NewListNormalKeys(),
		"ListSearch": NewListSearchKeys(),

		// Board (bt-ift6.3)
		"BoardNormal": NewBoardNormalKeys(),
		"BoardSearch": NewBoardSearchKeys(),

		// History (bt-ift6.6)
		"HistoryNormal":   NewHistoryNormalKeys(),
		"HistorySearch":   NewHistorySearchKeys(),
		"HistoryFileTree": NewHistoryFileTreeKeys(),

		"Insights":   NewInsightsKeys(),
		"Actionable": NewActionableKeys(),
		"FlowMatrix": NewFlowMatrixKeys(),
		"Graph":      NewGraphKeys(),
		"Epics":      NewEpicsKeys(),
		"EpicCard":   NewEpicCardKeys(),

		// Modal Maps (bt-ift6.9)
		"LabelPickerNav":    NewLabelPickerNavKeys(),
		"LabelPickerSearch": NewLabelPickerSearchKeys(),
		"RecipePicker":      NewRecipePickerKeys(),
		"BQLQuery":          NewBQLQueryKeys(),
		"TimeTravelInput":   NewTimeTravelInputKeys(),
		"RepoPickerNav":     NewRepoPickerNavKeys(),
		"RepoPickerSearch":  NewRepoPickerSearchKeys(),

		// Field-edit Maps (bt-oiaj.5)
		"FieldSelect": NewFieldSelectKeys(),
		"FieldPicker": NewFieldPickerKeys(),
		"FieldInput":  NewFieldInputKeys(),

		// Long-form field-edit Map (bt-oiaj.6, Slice C)
		"LongformEdit": NewLongformEditKeys(),
	}
}

// TestAllBindings_HaveHelpDesc enforces that every binding declared in any
// per-view Map carries non-empty Help.Desc. Help surfaces render Desc
// directly; an empty Desc means a binding listed in help with no
// description. See ADR-004 Decision 6.
func TestAllBindings_HaveHelpDesc(t *testing.T) {
	for name, m := range allMaps() {
		for col, group := range m.FullHelp() {
			for row, b := range group {
				if b.Help().Desc == "" {
					t.Errorf("%s.FullHelp()[%d][%d] (key=%q) has empty Help.Desc",
						name, col, row, b.Help().Key)
				}
			}
		}
		for i, b := range m.ShortHelp() {
			if b.Help().Desc == "" {
				t.Errorf("%s.ShortHelp()[%d] (key=%q) has empty Help.Desc",
					name, i, b.Help().Key)
			}
		}
	}
}

// TestAllMapsRegistered_MatchesAppKeys catches the "added a Map field to
// AppKeys but forgot allMaps()" drift class — silent test passes if not
// enforced. See ADR-004 Decision 6 (P1-S1 finding response).
func TestAllMapsRegistered_MatchesAppKeys(t *testing.T) {
	app := NewAppKeys()
	v := reflect.ValueOf(app)
	keyMapInterface := reflect.TypeOf((*help.KeyMap)(nil)).Elem()
	registered := allMaps()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		name := v.Type().Field(i).Name
		if !f.Type().Implements(keyMapInterface) {
			continue
		}
		if _, ok := registered[name]; !ok {
			t.Errorf("AppKeys.%s implements help.KeyMap but is not in allMaps()", name)
		}
	}
}

// TestUniversalNav_ConsistentAcrossViews asserts that for any binding name
// shared across per-view Map types (Up, Down, Left, Right, Enter, Esc,
// Back), the Help.Key strings match across all views that declare it.
// Help.Desc is allowed to differ — semantics legitimately vary (tree's `h`
// is "collapse / jump to parent"; list's `h` may differ).
//
// GlobalKeys is intentionally excluded: its bindings (e.g. Back = q cascade,
// Cancel = esc cascade) are global chrome with semantics distinct from
// per-view nav. The consistency contract is for views that share the
// universal-nav idiom.
//
// Defends the universal-nav-per-view declaration choice (ADR-004 Decision 1
// [v1 — revisit]) against two-contributor drift across PRs.
func TestUniversalNav_ConsistentAcrossViews(t *testing.T) {
	universal := []string{"Up", "Down", "Left", "Right", "Enter", "Esc", "Back"}

	app := NewAppKeys()
	appV := reflect.ValueOf(app)
	bindingType := reflect.TypeOf(key.Binding{})

	// helpKey[fieldName] maps to the first observed Help.Key string and the
	// view name where it was first seen, so a mismatch can be reported with
	// both sources.
	type seen struct {
		view    string
		helpKey string
	}
	helpKey := map[string]seen{}

	for i := 0; i < appV.NumField(); i++ {
		mapField := appV.Field(i)
		viewName := appV.Type().Field(i).Name
		if viewName == "Global" {
			// GlobalKeys is chrome, not a per-view nav consumer.
			continue
		}
		if mapField.Kind() != reflect.Struct {
			continue
		}

		mapT := mapField.Type()
		for j := 0; j < mapField.NumField(); j++ {
			fieldName := mapT.Field(j).Name
			if !contains(universal, fieldName) {
				continue
			}
			if mapField.Field(j).Type() != bindingType {
				continue
			}
			b := mapField.Field(j).Interface().(key.Binding)
			got := b.Help().Key

			if prior, ok := helpKey[fieldName]; ok {
				if prior.helpKey != got {
					t.Errorf("universal-nav %s drift: %s.Help.Key=%q, %s.Help.Key=%q",
						fieldName, prior.view, prior.helpKey, viewName, got)
				}
			} else {
				helpKey[fieldName] = seen{view: viewName, helpKey: got}
			}
		}
	}
}

// TestGlobalKeys_SearchModeDesc verifies the SearchMode binding carries the
// corrected help desc "cycle search ranker" (bt-dx7k.1 Task 4). The old desc
// "search mode" was misleading: ctrl+s cycles the ranker, it does not open search.
func TestGlobalKeys_SearchModeDesc(t *testing.T) {
	want := "cycle search ranker"
	got := NewGlobalKeys().SearchMode.Help().Desc
	if got != want {
		t.Errorf("SearchMode.Help().Desc = %q, want %q", got, want)
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
