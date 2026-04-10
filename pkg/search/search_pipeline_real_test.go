package search

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/loader"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func loadSearchHybridFixture(t *testing.T) []model.Issue {
	t.Helper()
	path := filepath.Join("..", "..", "tests", "testdata", "search_hybrid.jsonl")
	issues, err := loader.LoadIssuesFromFile(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	if len(issues) == 0 {
		t.Fatalf("fixture issues empty")
	}
	return issues
}

func buildIndexForIssues(t *testing.T, issues []model.Issue) (*VectorIndex, []float32) {
	t.Helper()
	ctx := context.Background()
	embedder := NewHashEmbedder(DefaultEmbeddingDim)
	docs := DocumentsFromIssues(issues)
	idx := NewVectorIndex(embedder.Dim())
	if _, err := SyncVectorIndex(ctx, idx, embedder, docs, 64); err != nil {
		t.Fatalf("sync index: %v", err)
	}
	vecs, err := embedder.Embed(ctx, []string{"auth"})
	if err != nil {
		t.Fatalf("embed query: %v", err)
	}
	if len(vecs) != 1 {
		t.Fatalf("expected 1 query vector, got %d", len(vecs))
	}
	return idx, vecs[0]
}

func scoreHybridResults(results []SearchResult, scorer HybridScorer) ([]HybridScore, error) {
	out := make([]HybridScore, 0, len(results))
	for _, result := range results {
		scored, err := scorer.Score(result.IssueID, result.Score)
		if err != nil {
			return nil, err
		}
		out = append(out, scored)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].FinalScore == out[j].FinalScore {
			return out[i].IssueID < out[j].IssueID
		}
		return out[i].FinalScore > out[j].FinalScore
	})

	return out, nil
}

func TestSearchPipelineHybridOverfetch(t *testing.T) {
	issues := loadSearchHybridFixture(t)
	idx, query := buildIndexForIssues(t, issues)

	cache := NewMetricsCache(NewAnalyzerMetricsLoader(issues))
	if err := cache.Refresh(); err != nil {
		t.Fatalf("refresh metrics: %v", err)
	}
	weights, err := GetPreset(PresetImpactFirst)
	if err != nil {
		t.Fatalf("preset default: %v", err)
	}
	scorer := NewHybridScorer(weights, cache)

	limit := 2
	textResults, err := idx.SearchTopK(query, limit)
	if err != nil {
		t.Fatalf("SearchTopK: %v", err)
	}
	limitedHybrid, err := scoreHybridResults(textResults, scorer)
	if err != nil {
		t.Fatalf("hybrid score limited: %v", err)
	}

	allResults, err := idx.SearchTopK(query, len(issues))
	if err != nil {
		t.Fatalf("SearchTopK all: %v", err)
	}
	fullHybrid, err := scoreHybridResults(allResults, scorer)
	if err != nil {
		t.Fatalf("hybrid score full: %v", err)
	}

	if len(fullHybrid) == 0 || len(limitedHybrid) == 0 {
		t.Fatalf("expected hybrid results")
	}

	if fullHybrid[0].IssueID == limitedHybrid[0].IssueID {
		t.Fatalf("expected over-fetch to change top result; full=%s limited=%s", fullHybrid[0].IssueID, limitedHybrid[0].IssueID)
	}
}

func TestSearchPipelineHybridOrderingStable(t *testing.T) {
	issues := loadSearchHybridFixture(t)
	idx, query := buildIndexForIssues(t, issues)

	cache := NewMetricsCache(NewAnalyzerMetricsLoader(issues))
	if err := cache.Refresh(); err != nil {
		t.Fatalf("refresh metrics: %v", err)
	}
	weights, err := GetPreset(PresetDefault)
	if err != nil {
		t.Fatalf("preset default: %v", err)
	}
	scorer := NewHybridScorer(weights, cache)

	results, err := idx.SearchTopK(query, len(issues))
	if err != nil {
		t.Fatalf("SearchTopK: %v", err)
	}
	hybridResults, err := scoreHybridResults(results, scorer)
	if err != nil {
		t.Fatalf("buildHybridScores: %v", err)
	}

	for i := 1; i < len(hybridResults); i++ {
		prev := hybridResults[i-1]
		cur := hybridResults[i]
		if prev.FinalScore < cur.FinalScore {
			t.Fatalf("hybrid ordering not descending at %d", i)
		}
		if prev.FinalScore == cur.FinalScore && prev.IssueID > cur.IssueID {
			t.Fatalf("hybrid tie-break not by ID at %d", i)
		}
	}
}
