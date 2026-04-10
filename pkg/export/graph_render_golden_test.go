package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/testutil"
)

type testGraphFile struct {
	Description string   `json:"description"`
	Nodes       []string `json:"nodes"`
	Edges       [][]int  `json:"edges"`
}

func loadGraphFixture(t *testing.T, name string) []model.Issue {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "graphs", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read graph fixture %s: %v", path, err)
	}

	var graph testGraphFile
	if err := json.Unmarshal(data, &graph); err != nil {
		t.Fatalf("parse graph fixture %s: %v", path, err)
	}

	issues := make([]model.Issue, len(graph.Nodes))
	for i, id := range graph.Nodes {
		issues[i] = model.Issue{
			ID:        id,
			Title:     id,
			Status:    model.StatusOpen,
			IssueType: model.TypeTask,
			Priority:  2,
		}
	}

	depsBySource := make(map[int][]*model.Dependency)
	for _, edge := range graph.Edges {
		if len(edge) != 2 {
			continue
		}
		from, to := edge[0], edge[1]
		if from < 0 || to < 0 || from >= len(graph.Nodes) || to >= len(graph.Nodes) {
			continue
		}
		depsBySource[from] = append(depsBySource[from], &model.Dependency{
			IssueID:     graph.Nodes[from],
			DependsOnID: graph.Nodes[to],
			Type:        model.DepBlocks,
		})
	}

	for i := range issues {
		if deps, ok := depsBySource[i]; ok {
			issues[i].Dependencies = deps
		}
	}

	return issues
}

func TestGraphRender_GoldenSVG(t *testing.T) {
	t.Parallel()

	fixtures := []string{"chain_10", "star_10", "diamond_5", "complex_20"}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			issues := loadGraphFixture(t, fixture)
			analyzer := analysis.NewAnalyzer(issues)
			stats := analyzer.AnalyzeWithConfig(analysis.FullAnalysisConfig())

			outPath := filepath.Join(t.TempDir(), fixture+".svg")
			err := SaveGraphSnapshot(GraphSnapshotOptions{
				Path:     outPath,
				Format:   "svg",
				Title:    "golden",
				Preset:   "compact",
				Issues:   issues,
				Stats:    &stats,
				DataHash: "golden",
			})
			if err != nil {
				t.Fatalf("SaveGraphSnapshot: %v", err)
			}

			svg, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("read snapshot %s: %v", outPath, err)
			}

			golden := testutil.NewGoldenFile(t, filepath.Join("..", "..", "testdata", "golden", "graph_render"), fixture+".svg.golden")
			golden.Assert(string(svg))
		})
	}
}

func TestGraphExport_GoldenMermaid(t *testing.T) {
	t.Parallel()

	fixtures := []string{"chain_10", "star_10", "diamond_5"}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			issues := loadGraphFixture(t, fixture)
			analyzer := analysis.NewAnalyzer(issues)
			stats := analyzer.AnalyzeWithConfig(analysis.FullAnalysisConfig())

			res, err := ExportGraph(issues, &stats, GraphExportConfig{
				Format:   GraphFormatMermaid,
				DataHash: "golden",
			})
			if err != nil {
				t.Fatalf("ExportGraph: %v", err)
			}

			golden := testutil.NewGoldenFile(t, filepath.Join("..", "..", "testdata", "golden", "graph_render"), fixture+".mermaid.golden")
			golden.Assert(res.Graph)
		})
	}
}
