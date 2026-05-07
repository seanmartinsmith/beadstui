package keys

import (
	"reflect"
	"testing"

	"charm.land/bubbles/v2/key"
)

// TestTreeKeys_AllFieldsRendered asserts every key.Binding declared as a
// field on TreeKeys appears at least once in ShortHelp() ∪ FullHelp().
// This catches "added a field but forgot to put it in FullHelp()" silently —
// the audit's failure mode at a smaller scope. See ADR-004 Decision 6.
//
// Reflection-based intentionally; replaces hardcoded `total == 13`-style
// cardinality tests that trip on every legitimate addition or removal of
// a binding.
func TestTreeKeys_AllFieldsRendered(t *testing.T) {
	k := NewTreeKeys()
	rendered := map[string]bool{}
	for _, b := range k.ShortHelp() {
		rendered[b.Help().Key] = true
	}
	for _, group := range k.FullHelp() {
		for _, b := range group {
			rendered[b.Help().Key] = true
		}
	}
	v := reflect.ValueOf(k)
	bindingType := reflect.TypeOf(key.Binding{})
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Type() != bindingType {
			continue
		}
		b := v.Field(i).Interface().(key.Binding)
		if !rendered[b.Help().Key] {
			t.Errorf("field %s (key=%q) not present in ShortHelp+FullHelp",
				v.Type().Field(i).Name, b.Help().Key)
		}
	}
}

// TestGlobalKeys_AllFieldsRendered: same invariant for GlobalKeys (the
// other fully-wired Map in bt-ift6.1). bt-ift6.2-.9 will add similar
// per-view tests; this one establishes the pattern.
func TestGlobalKeys_AllFieldsRendered(t *testing.T) {
	k := NewGlobalKeys()
	rendered := map[string]bool{}
	for _, b := range k.ShortHelp() {
		rendered[b.Help().Key] = true
	}
	for _, group := range k.FullHelp() {
		for _, b := range group {
			rendered[b.Help().Key] = true
		}
	}
	v := reflect.ValueOf(k)
	bindingType := reflect.TypeOf(key.Binding{})
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Type() != bindingType {
			continue
		}
		b := v.Field(i).Interface().(key.Binding)
		if !rendered[b.Help().Key] {
			t.Errorf("field %s (key=%q) not present in ShortHelp+FullHelp",
				v.Type().Field(i).Name, b.Help().Key)
		}
	}
}
