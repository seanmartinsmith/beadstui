package datasource

import (
	"database/sql"
	"testing"
)

// parseIssueMetadata is the pure JSON-parsing surface added by bt-mhwy.0.
// DB-level integration is covered live against Dolt; these tests just pin the
// parser contract: nil on empty/invalid, populated map on valid JSON.

func TestParseIssueMetadata_Null(t *testing.T) {
	if got := parseIssueMetadata(sql.NullString{Valid: false}); got != nil {
		t.Errorf("NULL → want nil, got %v", got)
	}
}

func TestParseIssueMetadata_EmptyString(t *testing.T) {
	if got := parseIssueMetadata(sql.NullString{Valid: true, String: ""}); got != nil {
		t.Errorf("empty → want nil, got %v", got)
	}
}

func TestParseIssueMetadata_EmptyObject(t *testing.T) {
	if got := parseIssueMetadata(sql.NullString{Valid: true, String: "{}"}); got != nil {
		t.Errorf("`{}` → want nil, got %v", got)
	}
}

func TestParseIssueMetadata_SessionFields(t *testing.T) {
	raw := sql.NullString{
		Valid:  true,
		String: `{"created_by_session":"abc-123","claimed_by_session":"def-456","action":"claimed"}`,
	}
	m := parseIssueMetadata(raw)
	if m == nil {
		t.Fatal("expected populated map, got nil")
	}

	cases := map[string]string{
		"created_by_session": `"abc-123"`,
		"claimed_by_session": `"def-456"`,
		"action":             `"claimed"`,
	}
	for key, want := range cases {
		v, ok := m[key]
		if !ok {
			t.Errorf("missing key %q", key)
			continue
		}
		if string(v) != want {
			t.Errorf("%s = %s, want %s", key, string(v), want)
		}
	}
}

func TestParseIssueMetadata_Invalid(t *testing.T) {
	// Malformed JSON returns nil rather than bubbling an error; callers leave
	// the field zero. The upstream DB shouldn't emit bad JSON, but we don't
	// want to crash bt on surprise data.
	if got := parseIssueMetadata(sql.NullString{Valid: true, String: "{not valid"}); got != nil {
		t.Errorf("malformed → want nil, got %v", got)
	}
}

// TestLoadIssuesSimpleFallback pins that the minimal-column fallback
// (`loadIssuesSimple`) still compiles and exports the API that
// LoadIssuesFiltered relies on when the full SELECT fails against an older
// Dolt schema. Full integration is tested live.
func TestLoadIssuesSimpleFallback_Exists(t *testing.T) {
	var r *DoltReader
	_ = r // compile-time only
	// loadIssuesSimple is the escape hatch for older Dolt schemas that lack
	// newer columns (e.g., metadata, closed_by_session). Ensures the method
	// remains reachable after column catchup so bt degrades gracefully.
	_ = (*DoltReader).loadIssuesSimple
}
