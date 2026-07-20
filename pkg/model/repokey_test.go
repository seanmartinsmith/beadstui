package model

import "testing"

// TestDisplayRepoName_AtlasAlias guards bt-z1pzj: the beads_global namespace
// (upstream `bd --global`) must render as "atlas" wherever bt surfaces a repo
// key, under both spellings RepoKey can produce - "beads_global" (SourceRepo,
// lowercased) and the bare ID-prefix fallback "global" - while any other key
// passes through unchanged.
func TestDisplayRepoName_AtlasAlias(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"beads_global lowercase", "beads_global", "atlas"},
		{"beads_global mixed case", "Beads_Global", "atlas"},
		{"bare global fallback", "global", "atlas"},
		{"bare global mixed case", "Global", "atlas"},
		{"ordinary repo key untouched", "bt", "bt"},
		{"ordinary repo key untouched (marketplace)", "marketplace", "marketplace"},
		{"empty key untouched", "", ""},
		// "globalize" contains "global" as a substring but must NOT alias -
		// IsAtlasNamespace must match the whole key, not a substring.
		{"substring collision not aliased", "globalize", "globalize"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DisplayRepoName(tt.key); got != tt.want {
				t.Errorf("DisplayRepoName(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// TestDeriveGlobalDisplayName guards bt-l76b8: the beads_global display label
// is the real ID prefix of the namespace's issues (identified by the
// authoritative SourceRepo db-name spelling), not a hardcoded "atlas". Returns
// "" when no global issue is present so callers keep the atlas default.
func TestDeriveGlobalDisplayName(t *testing.T) {
	tests := []struct {
		name   string
		issues []Issue
		want   string
	}{
		{
			name:   "atlas-prefixed global issue (this fleet, post rename-prefix)",
			issues: []Issue{{ID: "atlas-9k3", SourceRepo: "beads_global"}},
			want:   "atlas",
		},
		{
			name:   "foo-prefixed global issue (other fleet)",
			issues: []Issue{{ID: "foo-12", SourceRepo: "beads_global"}},
			want:   "foo",
		},
		{
			name:   "bare global fallback (SourceRepo empty, global- ID)",
			issues: []Issue{{ID: "global-7"}},
			want:   "global",
		},
		{
			name: "picks the global issue out of a mixed workspace",
			issues: []Issue{
				{ID: "bt-abc", SourceRepo: "beadstui"},
				{ID: "atlas-42", SourceRepo: "beads_global"},
			},
			want: "atlas",
		},
		{
			name:   "no global issue present",
			issues: []Issue{{ID: "bt-abc", SourceRepo: "beadstui"}},
			want:   "",
		},
		{
			name:   "empty slice",
			issues: nil,
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveGlobalDisplayName(tt.issues); got != tt.want {
				t.Errorf("DeriveGlobalDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDisplayRepoName_DerivedGlobalLabel guards bt-l76b8: once a real global
// prefix is set, DisplayRepoName returns it for the beads_global namespace under
// either raw spelling, while other keys pass through; an empty name resets to
// the atlas default so an empty/unavailable global DB never regresses.
func TestDisplayRepoName_DerivedGlobalLabel(t *testing.T) {
	// Restore the package default so test ordering can't leak the override into
	// the default-label tests.
	defer SetGlobalDisplayName("")

	// Default (no override) keeps the atlas fallback.
	if got := DisplayRepoName("beads_global"); got != "atlas" {
		t.Fatalf("default DisplayRepoName(beads_global) = %q, want atlas", got)
	}

	SetGlobalDisplayName("foo")
	if got := DisplayRepoName("beads_global"); got != "foo" {
		t.Errorf("DisplayRepoName(beads_global) = %q, want foo", got)
	}
	if got := DisplayRepoName("global"); got != "foo" {
		t.Errorf("DisplayRepoName(global) = %q, want foo", got)
	}
	if got := DisplayRepoName("bt"); got != "bt" {
		t.Errorf("DisplayRepoName(bt) = %q, want bt (unchanged)", got)
	}

	// Empty name resets to the atlas default (fallback semantics).
	SetGlobalDisplayName("")
	if got := DisplayRepoName("beads_global"); got != "atlas" {
		t.Errorf("after reset DisplayRepoName(beads_global) = %q, want atlas", got)
	}
}

func TestIsAtlasNamespace(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"beads_global", true},
		{"BEADS_GLOBAL", true},
		{"global", true},
		{"GLOBAL", true},
		{"bt", false},
		{"", false},
		{"globalize", false},
	}
	for _, tt := range tests {
		if got := IsAtlasNamespace(tt.key); got != tt.want {
			t.Errorf("IsAtlasNamespace(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}
