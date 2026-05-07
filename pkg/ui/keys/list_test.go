package keys

import (
	"reflect"
	"testing"

	"charm.land/bubbles/v2/key"
)

// TestListNormalKeys_AllFieldsRendered: every key.Binding declared on
// ListNormalKeys must appear at least once in ShortHelp() ∪ FullHelp().
// Mirrors TestTreeKeys_AllFieldsRendered (ADR-004 Decision 6).
func TestListNormalKeys_AllFieldsRendered(t *testing.T) {
	k := NewListNormalKeys()
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

// TestListSearchKeys_AllFieldsRendered: same invariant for ListSearchKeys
// (the help-only filter-mode Map). Even though dispatch is bubbles-owned,
// every declared field has to surface in at least one help slice.
func TestListSearchKeys_AllFieldsRendered(t *testing.T) {
	k := NewListSearchKeys()
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
