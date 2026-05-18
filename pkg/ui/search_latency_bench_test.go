package ui

import (
	"context"
	"fmt"
	"testing"

	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/search"
	"github.com/seanmartinsmith/beadstui/pkg/testutil"
)

// keystrokeTyping models a user typing the word "probably" one char at a time --
// a realistic English-word fast-typing case that exercises the non-ID code path.
var keystrokeTyping = []string{"p", "pr", "pro", "prob", "proba", "probab", "probabl", "probably"}

// keystrokeIDTyping models a user typing a bead suffix -- the case where the
// idPriorityFilter is intentionally exercised.
var keystrokeIDTyping = []string{"q", "ql", "qla", "qlas", "qlasl"}

// buildBenchSemanticSearch constructs a SemanticSearch with a populated hash
// embedder + vector index over the given issues. Returns the search and the
// matching list of targets (FilterValue strings).
func buildBenchSemanticSearch(b *testing.B, issues []model.Issue) (*SemanticSearch, []string) {
	b.Helper()
	items := makeBenchItems(issues)
	targets := make([]string, len(items))
	ids := make([]string, len(items))
	docs := make(map[string]string, len(items))
	for i, it := range items {
		targets[i] = it.FilterValue()
		ids[i] = it.Issue.ID
		docs[it.Issue.ID] = search.IssueDocument(it.Issue)
	}

	const dim = 64
	embedder := search.NewHashEmbedder(dim)
	index := search.NewVectorIndex(dim)
	ctx := context.Background()
	for id, doc := range docs {
		vec, err := embedder.Embed(ctx, []string{doc})
		if err != nil || len(vec) != 1 {
			b.Fatalf("embed failed for %s", id)
		}
		hash := search.ComputeContentHash(doc)
		if err := index.Upsert(id, hash, vec[0]); err != nil {
			b.Fatalf("upsert failed for %s: %v", id, err)
		}
	}

	s := NewSemanticSearch()
	s.SetIndex(index, embedder)
	s.SetIDs(ids)
	s.SetDocs(docs)
	return s, targets
}

func makeBenchItems(issues []model.Issue) []IssueItem {
	out := make([]IssueItem, len(issues))
	for i := range issues {
		out[i] = IssueItem{Issue: issues[i]}
	}
	return out
}

func BenchmarkSearchFilter_HybridMode_TextTyping(b *testing.B) {
	for _, size := range []int{1000, 4000} {
		b.Run(fmt.Sprintf("issues=%d", size), func(b *testing.B) {
			issues := testutil.QuickRandom(size, 0.01)
			sem, targets := buildBenchSemanticSearch(b, issues)
			filterFn := semanticSearchFilter(sem)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, term := range keystrokeTyping {
					_ = filterFn(term, targets)
				}
			}
		})
	}
}

func BenchmarkSearchFilter_HybridMode_IDTyping(b *testing.B) {
	for _, size := range []int{1000, 4000} {
		b.Run(fmt.Sprintf("issues=%d", size), func(b *testing.B) {
			issues := testutil.QuickRandom(size, 0.01)
			sem, targets := buildBenchSemanticSearch(b, issues)
			filterFn := semanticSearchFilter(sem)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, term := range keystrokeIDTyping {
					_ = filterFn(term, targets)
				}
			}
		})
	}
}
