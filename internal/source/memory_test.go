package source

import "testing"

// fixtureMemoriesJSON is a real `bd memories --json` capture shape: a flat
// key->body map plus the stray "schema_version" sibling (an integer; real
// memory bodies are strings - spec §4.2, §9). Verified live against the bt
// project's own embedded database.
const fixtureMemoriesJSON = `{
  "cross-platform-test-patterns-discovered-session-15-1": "Cross-platform test patterns discovered (session 15): use USERPROFILE not HOME on Windows.",
  "cross-prefix-deps": "Two cross-project dependency patterns in beads.",
  "schema_version": 1
}`

// TestParseMemoriesJSON_SkipsSchemaVersion is the acceptance-criterion test:
// the stray schema_version sibling (an int, not a memory body) must never
// surface as a Memory record.
func TestParseMemoriesJSON_SkipsSchemaVersion(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDEmbedded, Scope: "bt", Prefix: "bt", DisplayName: "bt"}

	got, err := ParseMemoriesJSON([]byte(fixtureMemoriesJSON), origin)
	if err != nil {
		t.Fatalf("ParseMemoriesJSON error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (schema_version must be skipped)", len(got))
	}
	for _, m := range got {
		if m.Key == "schema_version" {
			t.Errorf("schema_version leaked through as a Memory record: %+v", m)
		}
	}
}

// TestParseMemoriesJSON_OriginTagging checks every parsed Memory carries the
// origin passed in, and that key/body round-trip verbatim.
func TestParseMemoriesJSON_OriginTagging(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBeadsGlobal, Scope: "beads_global", Prefix: "beads_global", DisplayName: "atlas"}

	got, err := ParseMemoriesJSON([]byte(fixtureMemoriesJSON), origin)
	if err != nil {
		t.Fatalf("ParseMemoriesJSON error: %v", err)
	}
	var found bool
	for _, m := range got {
		if m.Origin != origin {
			t.Errorf("Memory %q Origin = %+v, want %+v", m.Key, m.Origin, origin)
		}
		if m.Key == "cross-prefix-deps" {
			found = true
			if m.Body != "Two cross-project dependency patterns in beads." {
				t.Errorf("Body = %q, want the fixture body verbatim", m.Body)
			}
		}
	}
	if !found {
		t.Fatal("expected key \"cross-prefix-deps\" not found in parsed memories")
	}
}

// TestParseMemoriesJSON_EmptyNamespace covers both zero-memory shapes: a
// bare empty object and an object containing only the schema_version
// sibling (the real shape `bd --global memories --json` emits for an
// empty beads_global namespace, verified live). Both must yield zero
// memories, not an error.
func TestParseMemoriesJSON_EmptyNamespace(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDServer, Scope: "proj"}

	cases := []string{
		`{}`,
		`{"schema_version": 1}`,
	}
	for _, raw := range cases {
		got, err := ParseMemoriesJSON([]byte(raw), origin)
		if err != nil {
			t.Fatalf("ParseMemoriesJSON(%q) error: %v", raw, err)
		}
		if len(got) != 0 {
			t.Errorf("ParseMemoriesJSON(%q) = %d memories, want 0", raw, len(got))
		}
	}
}

// TestParseMemoriesJSON_SkipsNonStringSiblings guards against a future bd
// adding another non-memory sibling key with a non-string value (schema_version
// is the one confirmed instance today, per spec §9, but the parser must
// degrade gracefully rather than hard-fail on an unexpected value shape,
// matching the codebase's general defensive-parsing convention).
func TestParseMemoriesJSON_SkipsNonStringSiblings(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDShared, Scope: "proj"}
	raw := `{"real-memory": "a body", "future_sibling": {"nested": true}, "schema_version": 1}`

	got, err := ParseMemoriesJSON([]byte(raw), origin)
	if err != nil {
		t.Fatalf("ParseMemoriesJSON error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (only \"real-memory\")", len(got))
	}
	if got[0].Key != "real-memory" {
		t.Errorf("Key = %q, want %q", got[0].Key, "real-memory")
	}
}

// TestParseMemoriesJSON_InvalidJSON surfaces a genuine parse failure (bd's
// output was not valid JSON at all) as an error rather than silently
// returning zero memories - that distinction matters to the adapter layer,
// which maps this to an "unavailable" source (spec §8).
func TestParseMemoriesJSON_InvalidJSON(t *testing.T) {
	origin := Origin{SourceKind: SourceKindBDEmbedded, Scope: "proj"}
	if _, err := ParseMemoriesJSON([]byte("not json"), origin); err == nil {
		t.Error("ParseMemoriesJSON(invalid JSON) error = nil, want non-nil")
	}
}
