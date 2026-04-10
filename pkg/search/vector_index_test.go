package search

import (
	"path/filepath"
	"testing"
)

func TestVectorIndex_SaveLoad_RoundTrip(t *testing.T) {
	idx := NewVectorIndex(4)

	if err := idx.Upsert("A", ComputeContentHash("a"), []float32{1, 0, 0, 0}); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}
	if err := idx.Upsert("B", ComputeContentHash("b"), []float32{0, 1, 0, 0}); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	path := filepath.Join(t.TempDir(), "semantic", "index.bvvi")
	if err := idx.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadVectorIndex(path)
	if err != nil {
		t.Fatalf("LoadVectorIndex failed: %v", err)
	}
	if loaded.Dim != 4 {
		t.Fatalf("Dim mismatch: got %d want %d", loaded.Dim, 4)
	}
	if loaded.Size() != 2 {
		t.Fatalf("Size mismatch: got %d want %d", loaded.Size(), 2)
	}

	a, ok := loaded.Get("A")
	if !ok {
		t.Fatalf("Expected entry A")
	}
	if a.ContentHash != ComputeContentHash("a") {
		t.Fatalf("Content hash mismatch for A")
	}
	if len(a.Vector) != 4 || a.Vector[0] != 1 {
		t.Fatalf("Vector mismatch for A: %#v", a.Vector)
	}
}

func TestVectorIndex_SearchTopK_OrderAndTieBreak(t *testing.T) {
	idx := NewVectorIndex(2)

	// Both entries have equal similarity to query; tie-break should be IssueID ascending.
	if err := idx.Upsert("B", ComputeContentHash("b"), []float32{1, 0}); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}
	if err := idx.Upsert("A", ComputeContentHash("a"), []float32{1, 0}); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	results, err := idx.SearchTopK([]float32{1, 0}, 2)
	if err != nil {
		t.Fatalf("SearchTopK failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}
	if results[0].IssueID != "A" || results[1].IssueID != "B" {
		t.Fatalf("Unexpected order: %#v", results)
	}
}

func TestVectorIndex_Errors(t *testing.T) {
	idx := NewVectorIndex(3)

	if err := idx.Upsert("A", ComputeContentHash("a"), []float32{1, 2}); err == nil {
		t.Fatalf("Expected dim mismatch error on Upsert")
	}
	if _, err := idx.SearchTopK([]float32{1, 2}, 1); err == nil {
		t.Fatalf("Expected dim mismatch error on SearchTopK")
	}
}

func TestContentHash_HexRoundTrip(t *testing.T) {
	h := ComputeContentHash("hello world")
	hexStr := h.Hex()
	if len(hexStr) != 64 {
		t.Fatalf("Expected 64-char hex, got %d", len(hexStr))
	}
	parsed, err := ParseContentHashHex(hexStr)
	if err != nil {
		t.Fatalf("ParseContentHashHex failed: %v", err)
	}
	if parsed != h {
		t.Fatalf("Hash round-trip mismatch")
	}
}
