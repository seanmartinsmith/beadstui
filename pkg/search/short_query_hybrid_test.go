package search

import (
	"context"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestHybridShortQueryPrefersLexicalMatch(t *testing.T) {
	now := time.Date(2025, 12, 1, 12, 0, 0, 0, time.UTC)
	issues := []model.Issue{
		{
			ID:          "bench-1",
			Title:       "Performance benchmarks for graph rendering",
			Description: "Add benchmarks for large graph rendering and profile hot paths.",
			Status:      model.StatusOpen,
			Priority:    2,
			IssueType:   model.TypeTask,
			CreatedAt:   now.Add(-72 * time.Hour),
			UpdatedAt:   now.Add(-24 * time.Hour),
		},
		{
			ID:          "core-1",
			Title:       "Core engine refactor",
			Description: "Refactor core execution engine for stability.",
			Status:      model.StatusOpen,
			Priority:    0,
			IssueType:   model.TypeTask,
			CreatedAt:   now.Add(-10 * 24 * time.Hour),
			UpdatedAt:   now.Add(-5 * 24 * time.Hour),
		},
		{
			ID:          "dep-1",
			Title:       "Feature A depends on core",
			Description: "Feature A unblock depends on core.",
			Status:      model.StatusOpen,
			Priority:    1,
			IssueType:   model.TypeTask,
			CreatedAt:   now.Add(-5 * 24 * time.Hour),
			UpdatedAt:   now.Add(-2 * 24 * time.Hour),
			Dependencies: []*model.Dependency{
				{
					IssueID:     "dep-1",
					DependsOnID: "core-1",
					Type:        model.DepBlocks,
					CreatedAt:   now.Add(-4 * 24 * time.Hour),
					CreatedBy:   "test",
				},
			},
		},
		{
			ID:          "dep-2",
			Title:       "Feature B depends on core",
			Description: "Feature B unblock depends on core.",
			Status:      model.StatusOpen,
			Priority:    1,
			IssueType:   model.TypeTask,
			CreatedAt:   now.Add(-4 * 24 * time.Hour),
			UpdatedAt:   now.Add(-2 * 24 * time.Hour),
			Dependencies: []*model.Dependency{
				{
					IssueID:     "dep-2",
					DependsOnID: "core-1",
					Type:        model.DepBlocks,
					CreatedAt:   now.Add(-3 * 24 * time.Hour),
					CreatedBy:   "test",
				},
			},
		},
		{
			ID:          "dep-3",
			Title:       "Feature C depends on core",
			Description: "Feature C unblock depends on core.",
			Status:      model.StatusOpen,
			Priority:    1,
			IssueType:   model.TypeTask,
			CreatedAt:   now.Add(-4 * 24 * time.Hour),
			UpdatedAt:   now.Add(-2 * 24 * time.Hour),
			Dependencies: []*model.Dependency{
				{
					IssueID:     "dep-3",
					DependsOnID: "core-1",
					Type:        model.DepBlocks,
					CreatedAt:   now.Add(-3 * 24 * time.Hour),
					CreatedBy:   "test",
				},
			},
		},
	}

	ctx := context.Background()
	embedder := NewHashEmbedder(DefaultEmbeddingDim)
	idx := NewVectorIndex(embedder.Dim())
	docs := DocumentsFromIssues(issues)
	if _, err := SyncVectorIndex(ctx, idx, embedder, docs, 64); err != nil {
		t.Fatalf("sync index: %v", err)
	}
	vecs, err := embedder.Embed(ctx, []string{"benchmarks"})
	if err != nil || len(vecs) != 1 {
		t.Fatalf("embed query: %v", err)
	}

	results, err := idx.SearchTopK(vecs[0], len(issues))
	if err != nil {
		t.Fatalf("SearchTopK: %v", err)
	}
	results = ApplyShortQueryLexicalBoost(results, "benchmarks", docs)

	cache := NewMetricsCache(NewAnalyzerMetricsLoader(issues))
	if err := cache.Refresh(); err != nil {
		t.Fatalf("refresh metrics: %v", err)
	}

	weights, err := GetPreset(PresetImpactFirst)
	if err != nil {
		t.Fatalf("preset: %v", err)
	}
	weights = AdjustWeightsForQuery(weights.Normalize(), "benchmarks")

	scorer := NewHybridScorer(weights, cache)
	hybridResults, err := scoreHybridResults(results, scorer)
	if err != nil {
		t.Fatalf("score hybrid: %v", err)
	}
	if len(hybridResults) == 0 {
		t.Fatalf("expected hybrid results")
	}
	if hybridResults[0].IssueID != "bench-1" {
		t.Fatalf("expected bench-1 to rank first for short query, got %s", hybridResults[0].IssueID)
	}
}
