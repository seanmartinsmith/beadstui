package correlation

import "testing"

func TestExtractPathHintsKeywords(t *testing.T) {
	hints := extractPathHints("Add tests for API service")
	expect := []string{"api", "service", "tests"}
	for _, e := range expect {
		found := false
		for _, h := range hints {
			if h == e {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing expected hint %q in %v", e, hints)
		}
	}
}

func TestExtractIDsOrdering(t *testing.T) {
	m := NewExplicitMatcher("/tmp/test")
	msg := "fix: [AUTH-1] closes BV-2 and refs PROJ-3"
	matches := m.ExtractIDsFromMessage(msg)
	want := []string{"auth-1", "bv-2", "proj-3"}
	if len(matches) != len(want) {
		t.Fatalf("got %d matches, want %d", len(matches), len(want))
	}
	for i, w := range want {
		if matches[i].ID != w {
			t.Fatalf("idx %d: got %s want %s", i, matches[i].ID, w)
		}
	}
}
