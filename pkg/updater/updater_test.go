package updater

import (
	"testing"
)

// ============================================================================
// compareVersions tests
// ============================================================================

func TestCompareVersions_Basic(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int // 1 if v1>v2, -1 if v1<v2, 0 if equal
	}{
		// Basic comparisons
		{"equal versions", "v1.0.0", "v1.0.0", 0},
		{"v1 greater major", "v2.0.0", "v1.0.0", 1},
		{"v1 less major", "v1.0.0", "v2.0.0", -1},
		{"v1 greater minor", "v1.1.0", "v1.0.0", 1},
		{"v1 less minor", "v1.0.0", "v1.1.0", -1},
		{"v1 greater patch", "v1.0.1", "v1.0.0", 1},
		{"v1 less patch", "v1.0.0", "v1.0.1", -1},

		// Without 'v' prefix
		{"no v prefix equal", "1.0.0", "1.0.0", 0},
		{"no v prefix greater", "2.0.0", "1.0.0", 1},
		{"no v prefix less", "1.0.0", "2.0.0", -1},

		// Mixed v prefix
		{"mixed prefix v1 has v", "v1.0.0", "1.0.0", 0},
		{"mixed prefix v2 has v", "1.0.0", "v1.0.0", 0},
		{"mixed prefix different versions", "v2.0.0", "1.0.0", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.v1, tt.v2)
			if got != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestCompareVersions_RealVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		// Real version scenarios
		{"current vs older", "v0.9.0", "v0.8.0", 1},
		{"current vs newer", "v0.8.0", "v0.9.0", -1},
		{"major upgrade", "v1.0.0", "v0.9.0", 1},
		{"patch update", "v0.9.1", "v0.9.0", 1},
		{"double digit minor", "v0.10.0", "v0.9.0", 1},
		{"double digit patch", "v0.9.10", "v0.9.9", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.v1, tt.v2)
			if got != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestCompareVersions_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		// Partial versions (less than 3 parts)
		{"two part equal", "v1.0", "v1.0", 0},
		{"two part vs three part equal", "v1.0", "v1.0.0", 0},
		{"one part", "v1", "v1.0.0", 0},

		// Large version numbers
		{"large major", "v100.0.0", "v99.0.0", 1},
		{"large minor", "v1.100.0", "v1.99.0", 1},
		{"large patch", "v1.0.100", "v1.0.99", 1},

		// Zero versions
		{"all zeros", "v0.0.0", "v0.0.0", 0},
		{"zero vs one", "v0.0.0", "v0.0.1", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.v1, tt.v2)
			if got != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestCompareVersions_InvalidVersions(t *testing.T) {
	// When parsing fails, it falls back to lexicographic comparison
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		// Non-numeric versions (fallback to lexicographic)
		{"alpha vs beta", "alpha", "beta", -1},
		{"beta vs alpha", "beta", "alpha", 1},
		{"same string", "alpha", "alpha", 0},

		// Version with extra parts (semver with pre-release)
		{"prerelease lower", "1.0.0-alpha", "1.0.0", -1}, // prerelease should be lower

		// Empty strings
		{"empty vs empty", "", "", 0},
		{"empty vs version", "", "v1.0.0", -1},
		{"version vs empty", "v1.0.0", "", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.v1, tt.v2)
			if got != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

func TestCompareVersions_PrereleaseOrdering(t *testing.T) {
	t.Run("prerelease vs prerelease same base", func(t *testing.T) {
		if got := compareVersions("v1.0.0-alpha", "v1.0.0-beta"); got >= 0 {
			t.Errorf("expected alpha < beta when base equal; got %d", got)
		}
	})

	t.Run("prerelease vs release", func(t *testing.T) {
		if got := compareVersions("v1.2.3-rc1", "v1.2.3"); got >= 0 {
			t.Errorf("expected prerelease < release; got %d", got)
		}
	})
}

func TestCompareVersions_Symmetry(t *testing.T) {
	// Test that comparison is antisymmetric: if v1 > v2 then v2 < v1
	versions := []string{
		"v0.7.0",
		"v0.8.0",
		"v0.9.0",
		"v1.0.0",
		"v1.0.1",
		"v1.1.0",
		"v2.0.0",
	}

	for i, v1 := range versions {
		for j, v2 := range versions {
			result1 := compareVersions(v1, v2)
			result2 := compareVersions(v2, v1)

			// result1 and result2 should be negatives of each other
			if result1 != -result2 {
				t.Errorf("Symmetry violation: compareVersions(%q, %q) = %d but compareVersions(%q, %q) = %d",
					v1, v2, result1, v2, v1, result2)
			}

			// Check ordering consistency
			if i < j && result1 >= 0 {
				t.Errorf("Ordering violation: %q should be less than %q but got %d", v1, v2, result1)
			}
			if i > j && result1 <= 0 {
				t.Errorf("Ordering violation: %q should be greater than %q but got %d", v1, v2, result1)
			}
			if i == j && result1 != 0 {
				t.Errorf("Equality violation: %q should equal %q but got %d", v1, v2, result1)
			}
		}
	}
}

func TestCompareVersions_Transitivity(t *testing.T) {
	// Test transitivity: if v1 < v2 and v2 < v3 then v1 < v3
	testCases := []struct {
		v1, v2, v3 string
	}{
		{"v0.7.0", "v0.8.0", "v0.9.0"},
		{"v0.9.0", "v0.9.1", "v1.0.0"},
		{"v1.0.0", "v1.1.0", "v2.0.0"},
	}

	for _, tc := range testCases {
		r12 := compareVersions(tc.v1, tc.v2)
		r23 := compareVersions(tc.v2, tc.v3)
		r13 := compareVersions(tc.v1, tc.v3)

		if r12 != -1 || r23 != -1 || r13 != -1 {
			t.Errorf("Transitivity violation: %q < %q < %q should all hold, got %d, %d, %d",
				tc.v1, tc.v2, tc.v3, r12, r23, r13)
		}
	}
}

func TestCompareVersions_DevBuilds(t *testing.T) {
	tests := []struct {
		name     string
		v1       string // remote
		v2       string // local
		expected int    // 1 if remote>local (update needed), -1 if local>remote (no update)
	}{
		// Unparseable dev versions
		{"dev is newer than release", "v1.0.0", "dev", -1},
		{"nightly is newer than release", "v1.0.0", "nightly", -1},
		{"dirty string is newer than release", "v1.0.0", "dirty", -1},
		{"local is newer than release", "v1.0.0", "local", -1},
		{"snapshot is newer than release", "v1.0.0", "snapshot", -1},
		{"git-abc123 is newer than release", "v1.0.0", "git-abc123", -1},

		// Parseable dev builds with suffixes - THE BUG FIX CASES
		// These should NOT trigger update prompts when local is a dev build of the same version
		{"v0.11.2-dirty should not update to v0.11.2", "v0.11.2", "v0.11.2-dirty", -1},
		{"v0.11.2-dev should not update to v0.11.2", "v0.11.2", "v0.11.2-dev", -1},
		{"v0.11.2-local should not update to v0.11.2", "v0.11.2", "v0.11.2-local", -1},
		{"v0.11.2-nightly should not update to v0.11.2", "v0.11.2", "v0.11.2-nightly", -1},
		{"v0.11.2-snapshot should not update to v0.11.2", "v0.11.2", "v0.11.2-snapshot", -1},
		{"v0.11.2-git.abc123 should not update to v0.11.2", "v0.11.2", "v0.11.2-git.abc123", -1},

		// Dev builds with lower base version SHOULD prompt for update
		{"v0.11.3 should update from v0.11.2-dirty", "v0.11.3", "v0.11.2-dirty", 1},
		{"v1.0.0 should update from v0.11.2-dev", "v1.0.0", "v0.11.2-dev", 1},

		// Dev build with higher base version should NOT prompt
		{"v0.11.2 should not update from v0.11.3-dirty", "v0.11.2", "v0.11.3-dirty", -1},

		// Normal pre-releases (NOT dev builds) should still follow semver
		{"v0.11.2-alpha is lower than v0.11.2", "v0.11.2", "v0.11.2-alpha", 1},
		{"v0.11.2-beta is lower than v0.11.2", "v0.11.2", "v0.11.2-beta", 1},
		{"v0.11.2-rc1 is lower than v0.11.2", "v0.11.2", "v0.11.2-rc1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.v1, tt.v2)
			if got != tt.expected {
				t.Errorf("compareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, got, tt.expected)
			}
		})
	}
}

// ============================================================================
// Release struct tests
// ============================================================================

func TestRelease_Fields(t *testing.T) {
	// Test that Release struct can be properly instantiated
	rel := Release{
		TagName: "v0.9.0",
		HTMLURL: "https://github.com/Dicklesworthstone/beads_viewer/releases/tag/v0.9.0",
	}

	if rel.TagName != "v0.9.0" {
		t.Errorf("Expected TagName v0.9.0, got %s", rel.TagName)
	}
	if rel.HTMLURL == "" {
		t.Error("Expected HTMLURL to be set")
	}
}

// ============================================================================
// Version comparison with current version (integration-like tests)
// ============================================================================

func TestCompareVersions_AgainstCurrentVersion(t *testing.T) {
	// These tests ensure version comparison works correctly with a sample version
	// Note: uses a fixed test version, not the actual app version from version.go
	currentVersion := "v0.10.2"

	tests := []struct {
		name       string
		newVersion string
		shouldBe   string // "newer", "older", "same"
	}{
		{"patch update", "v0.10.3", "newer"},
		{"minor update", "v0.11.0", "newer"},
		{"major update", "v1.0.0", "newer"},
		{"same version", "v0.10.2", "same"},
		{"older patch", "v0.10.1", "older"},
		{"older minor", "v0.9.0", "older"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.newVersion, currentVersion)

			switch tt.shouldBe {
			case "newer":
				if result != 1 {
					t.Errorf("Expected %q to be newer than %q, got %d", tt.newVersion, currentVersion, result)
				}
			case "older":
				if result != -1 {
					t.Errorf("Expected %q to be older than %q, got %d", tt.newVersion, currentVersion, result)
				}
			case "same":
				if result != 0 {
					t.Errorf("Expected %q to be same as %q, got %d", tt.newVersion, currentVersion, result)
				}
			}
		})
	}
}

// ============================================================================
// Benchmark tests
// ============================================================================

func BenchmarkCompareVersions_SemVer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compareVersions("v1.2.3", "v1.2.4")
	}
}

func BenchmarkCompareVersions_Fallback(b *testing.B) {
	for i := 0; i < b.N; i++ {
		compareVersions("alpha-1.2.3", "alpha-1.2.4")
	}
}

func TestCompareVersions_SemVerNumericIdentifiers(t *testing.T) {
	// Standard SemVer: identifiers consisting of only digits are compared numerically.
	// rc.2 < rc.10
	// Currently, lexicographic comparison makes "rc.10" < "rc.2" (FALSE)

	v1 := "v1.0.0-rc.2"
	v2 := "v1.0.0-rc.10"

	// Expect v2 > v1 -> returns -1
	expected := -1
	got := compareVersions(v1, v2)

	if got != expected {
		t.Errorf("compareVersions(%q, %q) = %d; want %d (numeric prerelease comparison failure)", v1, v2, got, expected)
	}
}
