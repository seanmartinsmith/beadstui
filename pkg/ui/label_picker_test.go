package ui

import "testing"

func TestFuzzyScoreExactMatch(t *testing.T) {
	score := fuzzyScore("api", "api")
	if score != 1000 {
		t.Errorf("Expected exact match score 1000, got %d", score)
	}
}

func TestFuzzyScorePrefixMatch(t *testing.T) {
	score := fuzzyScore("backend", "back")
	if score < 500 {
		t.Errorf("Expected prefix match score >= 500, got %d", score)
	}
}

func TestFuzzyScoreContainsMatch(t *testing.T) {
	score := fuzzyScore("my-backend-api", "backend")
	if score < 200 {
		t.Errorf("Expected contains match score >= 200, got %d", score)
	}
}

func TestFuzzyScoreSubsequenceMatch(t *testing.T) {
	score := fuzzyScore("backend", "bnd")
	if score <= 0 {
		t.Errorf("Expected subsequence match score > 0, got %d", score)
	}
}

func TestFuzzyScoreNoMatch(t *testing.T) {
	score := fuzzyScore("api", "xyz")
	if score != 0 {
		t.Errorf("Expected no match score 0, got %d", score)
	}
}

func TestFuzzyScoreCaseInsensitive(t *testing.T) {
	score1 := fuzzyScore("API", "api")
	score2 := fuzzyScore("api", "API")
	if score1 != 1000 || score2 != 1000 {
		t.Errorf("Expected case-insensitive exact match, got scores %d and %d", score1, score2)
	}
}

func TestFuzzyScoreWordBoundaryBonus(t *testing.T) {
	// Word boundary matches should score higher
	score1 := fuzzyScore("my-api-service", "as") // "a" at boundary, "s" in "service"
	score2 := fuzzyScore("myapiservice", "as")   // "a" and "s" not at boundaries
	if score1 <= score2 {
		t.Errorf("Expected word boundary match to score higher: boundary=%d, no-boundary=%d", score1, score2)
	}
}

func TestNewLabelPickerModel(t *testing.T) {
	labels := []string{"zebra", "api", "backend", "core"}
	counts := map[string]int{
		"zebra":   5,
		"api":     10,
		"backend": 3,
		"core":    7,
	}
	picker := NewLabelPickerModel(labels, counts, Theme{})

	// Should be sorted by count descending: api(10), core(7), zebra(5), backend(3)
	if picker.allLabels[0] != "api" {
		t.Errorf("Expected first label to be 'api' (highest count), got %s", picker.allLabels[0])
	}
	if picker.allLabels[1] != "core" {
		t.Errorf("Expected second label to be 'core' (second highest), got %s", picker.allLabels[1])
	}
	if picker.allLabels[3] != "backend" {
		t.Errorf("Expected last label to be 'backend' (lowest count), got %s", picker.allLabels[3])
	}
}

func TestLabelPickerSetLabels(t *testing.T) {
	picker := NewLabelPickerModel([]string{"a"}, map[string]int{"a": 1}, Theme{})
	picker.SetLabels([]string{"z", "m", "a"}, map[string]int{"z": 10, "m": 5, "a": 1})

	if len(picker.allLabels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(picker.allLabels))
	}
	// Should be sorted by count descending: z(10), m(5), a(1)
	if picker.allLabels[0] != "z" {
		t.Errorf("Expected first label 'z' (highest count), got %s", picker.allLabels[0])
	}
}

func TestLabelPickerNavigation(t *testing.T) {
	labels := []string{"api", "backend", "core"}
	// All same count so sorted alphabetically for ties
	counts := map[string]int{"api": 5, "backend": 5, "core": 5}
	picker := NewLabelPickerModel(labels, counts, Theme{})

	if picker.SelectedLabel() != "api" {
		t.Errorf("Expected initial selection 'api', got %s", picker.SelectedLabel())
	}

	picker.MoveDown()
	if picker.SelectedLabel() != "backend" {
		t.Errorf("Expected 'backend' after MoveDown, got %s", picker.SelectedLabel())
	}

	picker.MoveDown()
	if picker.SelectedLabel() != "core" {
		t.Errorf("Expected 'core' after second MoveDown, got %s", picker.SelectedLabel())
	}

	// At end, MoveDown wraps to top
	picker.MoveDown()
	if picker.SelectedLabel() != "api" {
		t.Errorf("Expected 'api' after wrap down, got %s", picker.SelectedLabel())
	}

	// At top, MoveUp wraps to bottom
	picker.MoveUp()
	if picker.SelectedLabel() != "core" {
		t.Errorf("Expected 'core' after wrap up, got %s", picker.SelectedLabel())
	}
}

func TestLabelPickerEmptySelection(t *testing.T) {
	picker := NewLabelPickerModel([]string{}, map[string]int{}, Theme{})
	if picker.SelectedLabel() != "" {
		t.Errorf("Expected empty selection from empty labels, got %s", picker.SelectedLabel())
	}
}

func TestLabelPickerFilteredCount(t *testing.T) {
	labels := []string{"api", "api-v2", "backend", "core"}
	counts := map[string]int{"api": 5, "api-v2": 3, "backend": 2, "core": 1}
	picker := NewLabelPickerModel(labels, counts, Theme{})

	if picker.FilteredCount() != 4 {
		t.Errorf("Expected 4 filtered labels initially, got %d", picker.FilteredCount())
	}
}

func TestLabelPickerReset(t *testing.T) {
	labels := []string{"api", "backend"}
	counts := map[string]int{"api": 5, "backend": 5}
	picker := NewLabelPickerModel(labels, counts, Theme{})
	picker.MoveDown()
	picker.Reset()

	if picker.InputValue() != "" {
		t.Errorf("Expected empty input after Reset, got %s", picker.InputValue())
	}
	if picker.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex 0 after Reset, got %d", picker.selectedIndex)
	}
}

// TestLabelPickerOpensWithSearchUnfocused asserts the bt-wnda contract: the
// modal lands focus on the labels list, not the search input. Pressing "/"
// transitions to search-focused mode; Esc inside search blurs but keeps the
// picker open (the close-modal Esc is owned by handleLabelPickerKeys).
func TestLabelPickerOpensWithSearchUnfocused(t *testing.T) {
	picker := NewLabelPickerModel([]string{"api", "backend"}, map[string]int{"api": 5, "backend": 3}, Theme{})

	if picker.IsSearchFocused() {
		t.Fatal("expected search to start unfocused (bt-wnda)")
	}
	if picker.input.Focused() {
		t.Fatal("expected text input Focused() to be false on open")
	}

	picker.FocusSearch()
	if !picker.IsSearchFocused() {
		t.Fatal("FocusSearch did not flip searchFocused")
	}
	if !picker.input.Focused() {
		t.Fatal("FocusSearch did not focus underlying text input")
	}

	picker.BlurSearch()
	if picker.IsSearchFocused() {
		t.Fatal("BlurSearch did not unfocus search")
	}
	if picker.input.Focused() {
		t.Fatal("BlurSearch did not blur underlying text input")
	}
}

// TestLabelPickerResetReturnsToNavigationMode asserts the picker reopens in
// nav mode every time. Without this guard, a user who searched in a prior
// session would return to a search-focused picker on the next open.
func TestLabelPickerResetReturnsToNavigationMode(t *testing.T) {
	picker := NewLabelPickerModel([]string{"api"}, map[string]int{"api": 1}, Theme{})
	picker.FocusSearch()

	picker.Reset()

	if picker.IsSearchFocused() {
		t.Fatal("Reset did not return to navigation mode (bt-wnda)")
	}
	if picker.input.Focused() {
		t.Fatal("Reset did not blur underlying text input")
	}
}

// TestLabelPickerVisibleCountScalesWithHeight asserts the bt-wnda size fix:
// the modal shows substantially more labels at typical terminal heights than
// the prior 10-row cap, while staying clamped on small/large terminals.
func TestLabelPickerVisibleCountScalesWithHeight(t *testing.T) {
	tests := []struct {
		height   int
		expected int
	}{
		{12, 4},  // tiny terminal: visibleCount = 12 - 8
		{20, 12}, // medium: 20 - 8 = 12 visible (was 10 before bt-wnda)
		{30, 22}, // tall: 30 - 8 = 22
		{40, 30}, // very tall: clamped to labelPickerMaxVisible
		{60, 30}, // huge: still clamped to 30
		{8, 3},   // extremely tiny: clamped to floor of 3
	}
	for _, tc := range tests {
		p := NewLabelPickerModel([]string{"api"}, map[string]int{"api": 1}, Theme{})
		p.SetSize(60, tc.height)
		got := p.visibleCount()
		if got != tc.expected {
			t.Errorf("height=%d: visibleCount=%d, want %d", tc.height, got, tc.expected)
		}
	}
}

// TestLabelPickerItemAtPanelY maps row clicks back to filtered indices and
// returns ok=false for chrome rows (input, blanks, page indicator, footer).
// This guards bt-wnda mouse routing.
func TestLabelPickerItemAtPanelY(t *testing.T) {
	labels := []string{"api", "backend", "core", "data", "edge"}
	counts := map[string]int{"api": 5, "backend": 4, "core": 3, "data": 2, "edge": 1}
	p := NewLabelPickerModel(labels, counts, Theme{})
	p.SetSize(60, 30) // visibleCount = 22, plenty of room

	// First label appears at row 3 (top border, input, blank, then labels).
	idx, ok := p.ItemAtPanelY(3)
	if !ok || idx != 0 {
		t.Errorf("row 3: got (%d, %v), want (0, true)", idx, ok)
	}
	idx, ok = p.ItemAtPanelY(4)
	if !ok || idx != 1 {
		t.Errorf("row 4: got (%d, %v), want (1, true)", idx, ok)
	}

	// Top border is chrome.
	if _, ok := p.ItemAtPanelY(0); ok {
		t.Error("row 0 (top border) should not map to a label")
	}
	// Input row is chrome (handled separately via IsSearchRow).
	if _, ok := p.ItemAtPanelY(1); ok {
		t.Error("row 1 (input) should not map to a label")
	}
	// Blank between input and labels.
	if _, ok := p.ItemAtPanelY(2); ok {
		t.Error("row 2 (blank) should not map to a label")
	}
	// Beyond the last label: chrome (page indicator / footer / bottom border).
	if _, ok := p.ItemAtPanelY(28); ok {
		t.Error("row 28 (well past last label, in chrome) should not map")
	}

	// Past the end of the filtered list: not ok.
	p2 := NewLabelPickerModel([]string{"only"}, map[string]int{"only": 1}, Theme{})
	p2.SetSize(60, 30)
	if _, ok := p2.ItemAtPanelY(4); ok {
		t.Error("row past last filtered item should not map")
	}
}

// TestLabelPickerIsSearchRow guards the click-to-focus-search routing.
func TestLabelPickerIsSearchRow(t *testing.T) {
	p := NewLabelPickerModel([]string{"api"}, map[string]int{"api": 1}, Theme{})
	if !p.IsSearchRow(1) {
		t.Error("row 1 should be the search input row")
	}
	if p.IsSearchRow(0) {
		t.Error("row 0 (top border) should not be the search row")
	}
	if p.IsSearchRow(3) {
		t.Error("row 3 (first label) should not be the search row")
	}
}

// TestLabelPickerSetCursor clamps out-of-bounds indices.
func TestLabelPickerSetCursor(t *testing.T) {
	p := NewLabelPickerModel([]string{"a", "b", "c"}, map[string]int{"a": 3, "b": 2, "c": 1}, Theme{})

	p.SetCursor(1)
	if p.selectedIndex != 1 {
		t.Errorf("SetCursor(1): got %d, want 1", p.selectedIndex)
	}
	p.SetCursor(99)
	if p.selectedIndex != 2 {
		t.Errorf("SetCursor(99): got %d, want 2 (clamped to len-1)", p.selectedIndex)
	}
	p.SetCursor(-5)
	if p.selectedIndex != 0 {
		t.Errorf("SetCursor(-5): got %d, want 0 (clamped to 0)", p.selectedIndex)
	}
}

func TestItoaHelper(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
		{-5, "-5"},
	}

	for _, tc := range tests {
		result := itoa(tc.input)
		if result != tc.expected {
			t.Errorf("itoa(%d) = %s, want %s", tc.input, result, tc.expected)
		}
	}
}
