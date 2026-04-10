package ui

import (
	"testing"
)

func TestHistoryModel_PreserveSelection(t *testing.T) {
	report := createTestHistoryReport() // Uses helper from history_test.go
	theme := testTheme()
	h := NewHistoryModel(report, theme)

	// We expect beads sorted by commit count descending:
	// bv-1 (2 commits)
	// bv-3 (2 commits)
	// bv-2 (1 commit)

	// Select "bv-3" (should be index 1)
	h.selectedBead = 1
	selectedID := h.SelectedBeadID()
	if selectedID != "bv-3" {
		t.Fatalf("setup: selectedBead=1 ID=%s, want bv-3. BeadIDs: %v", selectedID, h.beadIDs)
	}

	// Apply filter that keeps bv-3 but removes bv-1
	// bv-1 author: "Dev One"
	// bv-3 author: "Dev Two"
	// Filter by "Dev Two" should keep bv-3 and bv-2
	h.SetAuthorFilter("Dev Two")

	// Verify bv-3 is still selected
	// In the new list:
	// bv-3 (2 commits)
	// bv-2 (1 commit)
	// bv-3 should be index 0 now

	if h.SelectedBeadID() != "bv-3" {
		t.Errorf("selection lost after filter: ID=%s, want bv-3", h.SelectedBeadID())
	}
	if h.selectedBead != 0 {
		t.Errorf("selectedBead index = %d, want 0", h.selectedBead)
	}

	// Now apply filter that removes bv-3
	// Filter by "Dev One" -> only bv-1 remains
	h.SetAuthorFilter("Dev One")

	// Verify selection reset to 0 (bv-1), since bv-3 is gone
	if h.SelectedBeadID() != "bv-1" {
		t.Errorf("selection should reset to valid item: ID=%s, want bv-1", h.SelectedBeadID())
	}
	if h.selectedBead != 0 {
		t.Errorf("selectedBead index = %d, want 0", h.selectedBead)
	}
}
