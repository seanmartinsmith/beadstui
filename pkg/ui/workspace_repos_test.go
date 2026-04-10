package ui

import (
	"reflect"
	"testing"
)

// =============================================================================
// normalizeRepoPrefixes Tests
// =============================================================================

func TestNormalizeRepoPrefixes(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "single prefix unchanged",
			input:    []string{"api"},
			expected: []string{"api"},
		},
		{
			name:     "strips trailing hyphen",
			input:    []string{"api-"},
			expected: []string{"api"},
		},
		{
			name:     "strips trailing colon",
			input:    []string{"api:"},
			expected: []string{"api"},
		},
		{
			name:     "strips trailing underscore",
			input:    []string{"api_"},
			expected: []string{"api"},
		},
		{
			name:     "strips multiple trailing separators",
			input:    []string{"api-_:"},
			expected: []string{"api"},
		},
		{
			name:     "lowercases input",
			input:    []string{"API", "Web", "LIB"},
			expected: []string{"api", "lib", "web"},
		},
		{
			name:     "trims whitespace",
			input:    []string{"  api  ", " web ", "lib"},
			expected: []string{"api", "lib", "web"},
		},
		{
			name:     "deduplicates entries",
			input:    []string{"api", "API", "api-", "Api"},
			expected: []string{"api"},
		},
		{
			name:     "sorts alphabetically",
			input:    []string{"web", "api", "lib"},
			expected: []string{"api", "lib", "web"},
		},
		{
			name:     "skips empty strings",
			input:    []string{"", "api", "", "web"},
			expected: []string{"api", "web"},
		},
		{
			name:     "skips whitespace-only strings",
			input:    []string{"   ", "api", "\t", "web"},
			expected: []string{"api", "web"},
		},
		{
			name:     "skips strings that become empty after trim",
			input:    []string{"---", "api", ":::", "web"},
			expected: []string{"api", "web"},
		},
		{
			name:     "complex normalization",
			input:    []string{"  API- ", "web:", "LIB_", "api", "WEB-"},
			expected: []string{"api", "lib", "web"},
		},
		{
			name:     "all entries become empty",
			input:    []string{"", "  ", "---"},
			expected: nil,
		},
		{
			name:     "preserves internal separators",
			input:    []string{"my-api", "web-app", "lib-core"},
			expected: []string{"lib-core", "my-api", "web-app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeRepoPrefixes(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("normalizeRepoPrefixes(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// sortedRepoKeys Tests
// =============================================================================

func TestSortedRepoKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]bool
		expected []string
	}{
		{
			name:     "nil map",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map",
			input:    map[string]bool{},
			expected: nil,
		},
		{
			name:     "single selected key",
			input:    map[string]bool{"api": true},
			expected: []string{"api"},
		},
		{
			name:     "single unselected key",
			input:    map[string]bool{"api": false},
			expected: []string{},
		},
		{
			name:     "multiple selected keys",
			input:    map[string]bool{"web": true, "api": true, "lib": true},
			expected: []string{"api", "lib", "web"},
		},
		{
			name:     "mixed selected and unselected",
			input:    map[string]bool{"api": true, "web": false, "lib": true},
			expected: []string{"api", "lib"},
		},
		{
			name:     "all unselected",
			input:    map[string]bool{"api": false, "web": false, "lib": false},
			expected: []string{},
		},
		{
			name:     "keys sorted alphabetically",
			input:    map[string]bool{"zebra": true, "alpha": true, "beta": true},
			expected: []string{"alpha", "beta", "zebra"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sortedRepoKeys(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("sortedRepoKeys(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// formatRepoList Tests
// =============================================================================

func TestFormatRepoList(t *testing.T) {
	tests := []struct {
		name     string
		repos    []string
		maxNames int
		expected string
	}{
		{
			name:     "nil repos",
			repos:    nil,
			maxNames: 3,
			expected: "",
		},
		{
			name:     "empty repos",
			repos:    []string{},
			maxNames: 3,
			expected: "",
		},
		{
			name:     "single repo",
			repos:    []string{"api"},
			maxNames: 3,
			expected: "api",
		},
		{
			name:     "repos within limit",
			repos:    []string{"api", "web"},
			maxNames: 3,
			expected: "api,web",
		},
		{
			name:     "repos at exact limit",
			repos:    []string{"api", "web", "lib"},
			maxNames: 3,
			expected: "api,web,lib",
		},
		{
			name:     "repos exceed limit by one",
			repos:    []string{"api", "web", "lib", "db"},
			maxNames: 3,
			expected: "api,web,lib+1",
		},
		{
			name:     "repos exceed limit by many",
			repos:    []string{"api", "web", "lib", "db", "cache", "auth"},
			maxNames: 2,
			expected: "api,web+4",
		},
		{
			name:     "maxNames is 1",
			repos:    []string{"api", "web", "lib"},
			maxNames: 1,
			expected: "api+2",
		},
		{
			name:     "maxNames is 0",
			repos:    []string{"api", "web", "lib"},
			maxNames: 0,
			expected: "3 repos",
		},
		{
			name:     "maxNames is negative",
			repos:    []string{"api", "web"},
			maxNames: -1,
			expected: "2 repos",
		},
		{
			name:     "single repo with maxNames 0",
			repos:    []string{"api"},
			maxNames: 0,
			expected: "1 repos",
		},
		{
			name:     "repos with special characters",
			repos:    []string{"my-api", "web-app"},
			maxNames: 3,
			expected: "my-api,web-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRepoList(tt.repos, tt.maxNames)
			if result != tt.expected {
				t.Errorf("formatRepoList(%v, %d) = %q, want %q", tt.repos, tt.maxNames, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Integration/Edge Case Tests
// =============================================================================

func TestWorkspaceRepos_Integration(t *testing.T) {
	// Test the typical workflow: normalize prefixes, select some, format for display
	t.Run("full workflow", func(t *testing.T) {
		// Raw prefixes from workspace
		raw := []string{"API-", "Web:", "LIB_", "api", "Database-"}

		// Normalize them
		normalized := normalizeRepoPrefixes(raw)
		expected := []string{"api", "database", "lib", "web"}
		if !reflect.DeepEqual(normalized, expected) {
			t.Errorf("normalized = %v, want %v", normalized, expected)
		}

		// User selects some repos
		selected := map[string]bool{
			"api":      true,
			"database": false,
			"lib":      true,
			"web":      true,
		}

		// Get sorted selected keys
		keys := sortedRepoKeys(selected)
		expectedKeys := []string{"api", "lib", "web"}
		if !reflect.DeepEqual(keys, expectedKeys) {
			t.Errorf("keys = %v, want %v", keys, expectedKeys)
		}

		// Format for display with truncation
		display := formatRepoList(keys, 2)
		if display != "api,lib+1" {
			t.Errorf("display = %q, want %q", display, "api,lib+1")
		}
	})
}

func TestNormalizeRepoPrefixes_OrderStability(t *testing.T) {
	// Run multiple times to ensure sort is deterministic
	input := []string{"web", "api", "lib", "db", "cache"}
	expected := []string{"api", "cache", "db", "lib", "web"}

	for i := 0; i < 10; i++ {
		result := normalizeRepoPrefixes(input)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("iteration %d: got %v, want %v", i, result, expected)
		}
	}
}
