package main

import (
	"strings"
	"testing"
)

// TestResolveSchemaVersion exercises the flag-vs-env-vs-default
// precedence ladder. The env var override is verified via t.Setenv so
// the test leaves the process environment clean afterwards.
func TestResolveSchemaVersion(t *testing.T) {
	cases := []struct {
		name    string
		cli     string
		env     string
		want    string
		wantErr string
	}{
		{"default empty falls to v1", "", "", robotSchemaV1, ""},
		{"env v1 is honored", "", "v1", robotSchemaV1, ""},
		{"env v2 is honored", "", "v2", robotSchemaV2, ""},
		{"env is case-insensitive", "", "V2", robotSchemaV2, ""},
		{"flag beats env", "v1", "v2", robotSchemaV1, ""},
		{"flag is case-insensitive", "V1", "", robotSchemaV1, ""},
		{"leading whitespace trimmed", "  v2  ", "", robotSchemaV2, ""},
		{"invalid flag errors", "bogus", "", "", `invalid --schema "bogus"`},
		{"invalid env errors", "", "bogus", "", `invalid --schema "bogus"`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BT_OUTPUT_SCHEMA", tc.env)
			got, err := resolveSchemaVersion(tc.cli)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("want error %q, got nil (got=%q)", tc.wantErr, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want contains %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("resolveSchemaVersion(%q, env=%q) = %q, want %q", tc.cli, tc.env, got, tc.want)
			}
		})
	}
}

// TestResolveSigilsMode mirrors TestResolveSchemaVersion for --sigils.
// Default is `verb` per the brainstorm decision.
func TestResolveSigilsMode(t *testing.T) {
	cases := []struct {
		name    string
		cli     string
		env     string
		want    string
		wantErr string
	}{
		{"default empty falls to verb", "", "", robotSigilVerb, ""},
		{"env strict is honored", "", "strict", robotSigilStrict, ""},
		{"env permissive is honored", "", "permissive", robotSigilPermissive, ""},
		{"flag beats env", "verb", "strict", robotSigilVerb, ""},
		{"env is case-insensitive", "", "STRICT", robotSigilStrict, ""},
		{"invalid flag errors", "laxmode", "", "", `invalid --sigils "laxmode"`},
		{"invalid env errors", "", "lax", "", `invalid --sigils "lax"`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BT_SIGIL_MODE", tc.env)
			got, err := resolveSigilsMode(tc.cli)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("want error %q, got nil (got=%q)", tc.wantErr, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want contains %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("resolveSigilsMode(%q, env=%q) = %q, want %q", tc.cli, tc.env, got, tc.want)
			}
		})
	}
}

// TestSigilsFlagExplicit distinguishes user-set from default-resolved.
// runRefs uses this to detect the --schema=v1 + --sigils=* conflict
// without false-positiving on the unset default.
func TestSigilsFlagExplicit(t *testing.T) {
	cases := []struct {
		name string
		cli  string
		env  string
		want bool
	}{
		{"nothing set", "", "", false},
		{"flag set", "strict", "", true},
		{"env set", "", "verb", true},
		{"both set", "strict", "verb", true},
		{"flag whitespace-only is not explicit", "   ", "", false},
		{"env whitespace-only is not explicit", "", "   ", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BT_SIGIL_MODE", tc.env)
			got := sigilsFlagExplicit(tc.cli)
			if got != tc.want {
				t.Errorf("sigilsFlagExplicit(%q, env=%q) = %v, want %v", tc.cli, tc.env, got, tc.want)
			}
		})
	}
}
