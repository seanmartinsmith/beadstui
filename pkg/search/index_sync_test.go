package search

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSyncVectorIndex_IncrementalUpdates(t *testing.T) {
	embedder, err := NewEmbedderFromConfig(EmbeddingConfig{Provider: ProviderHash, Dim: 16})
	if err != nil {
		t.Fatalf("NewEmbedderFromConfig: %v", err)
	}

	idx := NewVectorIndex(embedder.Dim())
	docs1 := map[string]string{
		"A": "Fix login flow\nAdd OAuth redirect handling",
		"B": "Update docs\nReadme improvements",
	}

	stats, err := SyncVectorIndex(context.Background(), idx, embedder, docs1, 0)
	if err != nil {
		t.Fatalf("SyncVectorIndex: %v", err)
	}
	if stats.Added != 2 || stats.Embedded != 2 || stats.Updated != 0 || stats.Removed != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if idx.Size() != 2 {
		t.Fatalf("expected 2 entries, got %d", idx.Size())
	}

	// Second sync with identical docs should not re-embed.
	stats2, err := SyncVectorIndex(context.Background(), idx, embedder, docs1, 0)
	if err != nil {
		t.Fatalf("SyncVectorIndex: %v", err)
	}
	if stats2.Skipped != 2 || stats2.Embedded != 0 || stats2.Added != 0 || stats2.Updated != 0 || stats2.Removed != 0 {
		t.Fatalf("unexpected stats: %+v", stats2)
	}

	// Change A, remove B, add C.
	docs2 := map[string]string{
		"A": "Fix login flow\nHandle PKCE code verifier",
		"C": "Add tests\nCover edge cases",
	}
	stats3, err := SyncVectorIndex(context.Background(), idx, embedder, docs2, 0)
	if err != nil {
		t.Fatalf("SyncVectorIndex: %v", err)
	}
	if stats3.Updated != 1 || stats3.Added != 1 || stats3.Removed != 1 || stats3.Embedded != 2 {
		t.Fatalf("unexpected stats: %+v", stats3)
	}
	if idx.Size() != 2 {
		t.Fatalf("expected 2 entries after update, got %d", idx.Size())
	}
	if _, ok := idx.Get("B"); ok {
		t.Fatalf("expected B to be removed")
	}
	if _, ok := idx.Get("C"); !ok {
		t.Fatalf("expected C to be present")
	}
}

func TestLoadOrNewVectorIndex(t *testing.T) {
	embedder := NewHashEmbedder(8)
	path := filepath.Join(t.TempDir(), "semantic", "index.bvvi")

	idx, loaded, err := LoadOrNewVectorIndex(path, embedder.Dim())
	if err != nil {
		t.Fatalf("LoadOrNewVectorIndex: %v", err)
	}
	if loaded {
		t.Fatalf("expected loaded=false for missing file")
	}
	if idx.Dim != embedder.Dim() {
		t.Fatalf("dim mismatch: got %d want %d", idx.Dim, embedder.Dim())
	}

	if err := idx.Upsert("A", ComputeContentHash("a"), []float32{1, 0, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := idx.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loadedIdx, loaded2, err := LoadOrNewVectorIndex(path, embedder.Dim())
	if err != nil {
		t.Fatalf("LoadOrNewVectorIndex: %v", err)
	}
	if !loaded2 {
		t.Fatalf("expected loaded=true after save")
	}
	if loadedIdx.Size() != 1 {
		t.Fatalf("expected 1 entry, got %d", loadedIdx.Size())
	}
}
