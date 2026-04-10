package search

import (
	"context"
	"math"
	"testing"
)

func TestHashEmbedder_Deterministic(t *testing.T) {
	embedder := NewHashEmbedder(64)

	v1, err := embedder.Embed(context.Background(), []string{"memory leak bug"})
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}
	v2, err := embedder.Embed(context.Background(), []string{"memory leak bug"})
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(v1) != 1 || len(v2) != 1 {
		t.Fatalf("Unexpected output sizes: %d %d", len(v1), len(v2))
	}
	if len(v1[0]) != embedder.Dim() || len(v2[0]) != embedder.Dim() {
		t.Fatalf("Unexpected vector dims: %d %d", len(v1[0]), len(v2[0]))
	}

	for i := range v1[0] {
		if v1[0][i] != v2[0][i] {
			t.Fatalf("Vectors differ at %d: %v vs %v", i, v1[0][i], v2[0][i])
		}
	}
}

func TestHashEmbedder_CosineSimilarityTokenOverlap(t *testing.T) {
	embedder := NewHashEmbedder(DefaultEmbeddingDim)

	vecs, err := embedder.Embed(context.Background(), []string{
		"memory leak bug",
		"leak memory issue",
		"frontend styling polish",
	})
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	overlap := cosine(vecs[0], vecs[1])
	disjoint := cosine(vecs[0], vecs[2])

	if overlap <= disjoint {
		t.Fatalf("Expected overlap cosine > disjoint cosine; overlap=%f disjoint=%f", overlap, disjoint)
	}
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		na += ai * ai
		nb += bi * bi
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
