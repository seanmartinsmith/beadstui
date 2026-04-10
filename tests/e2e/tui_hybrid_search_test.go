package main_test

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/loader"
	"github.com/Dicklesworthstone/beads_viewer/pkg/search"
	"github.com/Dicklesworthstone/beads_viewer/pkg/ui"
)

func TestTUIHybridSearchSmoke(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "search_hybrid.jsonl")
	issues, err := loader.LoadIssuesFromFile(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	if len(issues) == 0 {
		t.Fatalf("fixture issues empty")
	}

	docs := search.DocumentsFromIssues(issues)
	embedder := search.NewHashEmbedder(search.DefaultEmbeddingDim)
	idx := search.NewVectorIndex(embedder.Dim())
	if _, err := search.SyncVectorIndex(context.Background(), idx, embedder, docs, 64); err != nil {
		t.Fatalf("sync index: %v", err)
	}

	ids := make([]string, 0, len(issues))
	for _, issue := range issues {
		ids = append(ids, issue.ID)
	}
	sort.Strings(ids)

	cache := search.NewMetricsCache(search.NewAnalyzerMetricsLoader(issues))
	if err := cache.Refresh(); err != nil {
		t.Fatalf("refresh metrics: %v", err)
	}

	hybrid := ui.NewSemanticSearch()
	hybrid.SetIndex(idx, embedder)
	hybrid.SetIDs(ids)
	hybrid.SetMetricsCache(cache)
	hybrid.SetHybridConfig(true, search.PresetImpactFirst)

	ranks := hybrid.ComputeSemanticResults("auth")
	if len(ranks) == 0 {
		t.Fatalf("expected hybrid ranks")
	}

	scores, ok := hybrid.Scores("auth")
	if !ok {
		t.Fatalf("expected hybrid scores for term")
	}

	entries := make([]struct {
		id         string
		score      float64
		text       float64
		components map[string]float64
	}, 0, len(scores))
	for id, score := range scores {
		entries = append(entries, struct {
			id         string
			score      float64
			text       float64
			components map[string]float64
		}{
			id:         id,
			score:      score.Score,
			text:       score.TextScore,
			components: score.Components,
		})
	}
	if len(entries) == 0 {
		t.Fatalf("expected score entries")
	}

	foundComponents := false
	for _, entry := range entries {
		if entry.components != nil {
			foundComponents = true
			break
		}
	}
	if !foundComponents {
		t.Fatalf("expected hybrid component scores")
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].score == entries[j].score {
			return entries[i].id < entries[j].id
		}
		return entries[i].score > entries[j].score
	})

	max := 5
	if len(entries) < max {
		max = len(entries)
	}
	for i := 0; i < max; i++ {
		entry := entries[i]
		t.Logf("hybrid[%d] id=%s score=%.4f text=%.4f components=%v", i, entry.id, entry.score, entry.text, entry.components)
	}
}
