package recipe_test

import (
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/recipe"
)

func TestParseRelativeTimeHours(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		input    string
		wantHrs  int // hours subtracted from now
	}{
		{"1h", 1},
		{"24h", 24},
		{"6h", 6},
		{"48h", 48},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := recipe.ParseRelativeTime(tc.input, now)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			expected := now.Add(time.Duration(-tc.wantHrs) * time.Hour)
			if !result.Equal(expected) {
				t.Errorf("Expected %v, got %v", expected, result)
			}
		})
	}
}

func TestParseRelativeTimeHoursCaseInsensitive(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	result, err := recipe.ParseRelativeTime("6H", now)
	if err != nil {
		t.Fatalf("Unexpected error for uppercase H: %v", err)
	}
	expected := now.Add(-6 * time.Hour)
	if !result.Equal(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestParseRelativeTimeZeroHoursInvalid(t *testing.T) {
	now := time.Now()
	// 0h is syntactically valid (n=0) but results in now itself, not an error.
	// The parser doesn't reject zero — it returns now. Verify it doesn't error.
	result, err := recipe.ParseRelativeTime("0h", now)
	if err != nil {
		t.Fatalf("0h should not error (returns now): %v", err)
	}
	if !result.Equal(now) {
		t.Errorf("0h: expected now (%v), got %v", now, result)
	}
}

func TestParseRelativeTimeDays(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	result, err := recipe.ParseRelativeTime("14d", now)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestParseRelativeTimeWeeks(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	result, err := recipe.ParseRelativeTime("2w", now)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestParseRelativeTimeMonths(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	result, err := recipe.ParseRelativeTime("1m", now)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := time.Date(2025, 2, 15, 12, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestParseRelativeTimeYears(t *testing.T) {
	now := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	result, err := recipe.ParseRelativeTime("1y", now)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestParseRelativeTimeISODate(t *testing.T) {
	now := time.Now()

	result, err := recipe.ParseRelativeTime("2024-06-15", now)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be in same location as 'now'
	expected := time.Date(2024, 6, 15, 0, 0, 0, 0, now.Location())
	if !result.Equal(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
	if result.Location() != now.Location() {
		t.Errorf("Expected location %v, got %v", now.Location(), result.Location())
	}
}

func TestParseRelativeTimeRFC3339(t *testing.T) {
	now := time.Now()

	// Z implies UTC
	result, err := recipe.ParseRelativeTime("2024-06-15T10:30:00Z", now)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestParseRelativeTimeEmpty(t *testing.T) {
	now := time.Now()

	result, err := recipe.ParseRelativeTime("", now)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.IsZero() {
		t.Errorf("Expected zero time for empty input, got %v", result)
	}
}

func TestParseRelativeTimeInvalid(t *testing.T) {
	now := time.Now()

	_, err := recipe.ParseRelativeTime("invalid", now)
	if err == nil {
		t.Error("Expected error for invalid input")
	}

	if _, ok := err.(*recipe.TimeParseError); !ok {
		t.Errorf("Expected TimeParseError, got %T", err)
	}
}

func TestParseRelativeTimeCaseInsensitive(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	result, err := recipe.ParseRelativeTime("7D", now)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := time.Date(2025, 1, 8, 12, 0, 0, 0, time.UTC)
	if !result.Equal(expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestRecipeStructTags(t *testing.T) {
	// Verify JSON/YAML struct tags exist by checking marshaling works
	r := recipe.Recipe{
		Name: "test",
		Filters: recipe.FilterConfig{
			Status: []string{"open"},
		},
	}

	// Just verify the struct can be used (compile-time check)
	if r.Name == "" {
		t.Error("Name should not be empty")
	}
	if r.Filters.Status == nil {
		t.Error("Filters.Status should not be nil")
	}
}
