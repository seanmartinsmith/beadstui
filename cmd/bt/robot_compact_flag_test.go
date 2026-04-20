package main

import (
	"testing"
)

func TestResolveRobotOutputShape(t *testing.T) {
	cases := []struct {
		name         string
		cliShape     string
		compactAlias bool
		fullAlias    bool
		env          string
		wantShape    string
		wantErr      bool
	}{
		{"default compact", "", false, false, "", robotShapeCompact, false},
		{"explicit compact", "compact", false, false, "", robotShapeCompact, false},
		{"explicit full", "full", false, false, "", robotShapeFull, false},
		{"compact alias", "", true, false, "", robotShapeCompact, false},
		{"full alias", "", false, true, "", robotShapeFull, false},
		{"env full", "", false, false, "full", robotShapeFull, false},
		{"env compact", "", false, false, "compact", robotShapeCompact, false},
		{"env uppercase full", "", false, false, "FULL", robotShapeFull, false},
		{"cli overrides env", "compact", false, false, "full", robotShapeCompact, false},
		{"alias overrides env", "", false, true, "compact", robotShapeFull, false},
		{"both aliases conflict", "", true, true, "", "", true},
		{"alias conflicts with shape", "full", true, false, "", "", true},
		{"unknown shape", "json", false, false, "", "", true},
		{"unknown env", "", false, false, "xml", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BT_OUTPUT_SHAPE", tc.env)
			got, err := resolveRobotOutputShape(tc.cliShape, tc.compactAlias, tc.fullAlias)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantShape {
				t.Errorf("got %q, want %q", got, tc.wantShape)
			}
		})
	}
}
