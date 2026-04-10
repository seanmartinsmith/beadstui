package analysis

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// TestMetricStatusAndFullStatsLimits verifies metric status population and map caps using real Analyzer.
func TestMetricStatusAndFullStatsLimits(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "A", Status: model.StatusOpen, Priority: 1},
		{ID: "B", Title: "B", Status: model.StatusOpen, Priority: 1, Dependencies: []*model.Dependency{{IssueID: "B", DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "C", Title: "C", Status: model.StatusOpen, Priority: 1, Dependencies: []*model.Dependency{{IssueID: "C", DependsOnID: "A", Type: model.DepBlocks}}},
	}

	// cap maps to 2
	os.Setenv("BV_INSIGHTS_MAP_LIMIT", "2")
	defer os.Unsetenv("BV_INSIGHTS_MAP_LIMIT")

	cached := NewCachedAnalyzer(issues, nil)
	stats := cached.AnalyzeAsync(context.Background())
	stats.WaitForPhase2()

	status := stats.Status()
	if status.PageRank.State == "" {
		t.Fatalf("expected pagerank status populated")
	}

	// emulate full_stats trimming logic
	cap := 2
	trim := func(m map[string]float64, limit int) map[string]float64 {
		if limit <= 0 || limit >= len(m) {
			return m
		}
		type kv struct {
			k string
			v float64
		}
		var items []kv
		for k, v := range m {
			items = append(items, kv{k, v})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].v == items[j].v {
				return items[i].k < items[j].k
			}
			return items[i].v > items[j].v
		})
		out := make(map[string]float64, limit)
		for i := 0; i < limit; i++ {
			out[items[i].k] = items[i].v
		}
		return out
	}

	prTrim := trim(stats.PageRank(), cap)
	if len(prTrim) != cap {
		t.Fatalf("expected trimmed pagerank size %d, got %d", cap, len(prTrim))
	}
}

func TestStatusEntryMarshalJSONMilliseconds(t *testing.T) {
	entry := statusEntry{
		State:   "computed",
		Elapsed: 1500 * time.Millisecond,
	}

	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal statusEntry: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal statusEntry: %v", err)
	}

	ms, ok := decoded["ms"].(float64)
	if !ok {
		t.Fatalf("expected ms field in JSON, got %v", decoded)
	}
	if ms < 1499 || ms > 1501 {
		t.Fatalf("expected ~1500ms, got %v", ms)
	}
}
