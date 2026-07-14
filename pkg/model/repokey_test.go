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
