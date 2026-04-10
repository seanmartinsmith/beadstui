package export

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func generateLayeredSnapshotIssues(levels, perLevel, fanIn int) []model.Issue {
	if levels < 1 {
		levels = 1
	}
	if perLevel < 1 {
		perLevel = 1
	}
	if fanIn < 0 {
		fanIn = 0
	}
	if fanIn > perLevel {
		fanIn = perLevel
	}

	total := levels * perLevel
	issues := make([]model.Issue, total)
	for level := 0; level < levels; level++ {
		for idxInLevel := 0; idxInLevel < perLevel; idxInLevel++ {
			idx := level*perLevel + idxInLevel
			id := fmt.Sprintf("L%02d-%04d", level, idxInLevel)
			issues[idx] = model.Issue{
				ID:        id,
				Title:     id,
				Status:    model.StatusOpen,
				IssueType: model.TypeTask,
				Priority:  2,
			}
		}
	}

	if fanIn == 0 || levels == 1 {
		return issues
	}

	step := perLevel / fanIn
	if step < 1 {
		step = 1
	}

	for level := 1; level < levels; level++ {
		prevStart := (level - 1) * perLevel
		curStart := level * perLevel

		for idxInLevel := 0; idxInLevel < perLevel; idxInLevel++ {
			issueIdx := curStart + idxInLevel
			for d := 0; d < fanIn; d++ {
				depPos := (idxInLevel + d*step) % perLevel
				depIdx := prevStart + depPos
				issues[issueIdx].Dependencies = append(issues[issueIdx].Dependencies, &model.Dependency{
					IssueID:     issues[issueIdx].ID,
					DependsOnID: issues[depIdx].ID,
					Type:        model.DepBlocks,
				})
			}
		}
	}

	return issues
}

func prepareSnapshotBench(levels, perLevel int) ([]model.Issue, *analysis.GraphStats) {
	issues := generateLayeredSnapshotIssues(levels, perLevel, 2)
	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.AnalyzeAsync(context.Background())
	stats.WaitForPhase2()
	return issues, stats
}

func BenchmarkGraphSnapshot_BuildLayout_Layered1000(b *testing.B) {
	issues, stats := prepareSnapshotBench(20, 50)
	opts := GraphSnapshotOptions{
		Issues:   issues,
		Stats:    stats,
		DataHash: "bench",
		Preset:   "compact",
	}

	b.ReportAllocs()
	b.ResetTimer()

	var layout layoutResult
	for i := 0; i < b.N; i++ {
		layout = buildLayout(opts)
	}
	runtime.KeepAlive(layout)
}

func BenchmarkGraphSnapshot_BuildLayout_Layered2000(b *testing.B) {
	issues, stats := prepareSnapshotBench(20, 100)
	opts := GraphSnapshotOptions{
		Issues:   issues,
		Stats:    stats,
		DataHash: "bench",
		Preset:   "compact",
	}

	b.ReportAllocs()
	b.ResetTimer()

	var layout layoutResult
	for i := 0; i < b.N; i++ {
		layout = buildLayout(opts)
	}
	runtime.KeepAlive(layout)
}

func BenchmarkGraphSnapshot_RenderSVG_Layered1000(b *testing.B) {
	issues, stats := prepareSnapshotBench(20, 50)
	opts := GraphSnapshotOptions{
		Issues:   issues,
		Stats:    stats,
		DataHash: "bench",
		Preset:   "compact",
	}
	layout := buildLayout(opts)

	var buf bytes.Buffer
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		if err := renderSVGToWriter(&buf, layout); err != nil {
			b.Fatalf("renderSVGToWriter: %v", err)
		}
	}
	runtime.KeepAlive(buf)
}

func BenchmarkGraphSnapshot_BuildLayoutAndRenderSVG_Layered1000(b *testing.B) {
	issues, stats := prepareSnapshotBench(20, 50)
	opts := GraphSnapshotOptions{
		Issues:   issues,
		Stats:    stats,
		DataHash: "bench",
		Preset:   "compact",
	}

	var buf bytes.Buffer
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		layout := buildLayout(opts)
		buf.Reset()
		if err := renderSVGToWriter(&buf, layout); err != nil {
			b.Fatalf("renderSVGToWriter: %v", err)
		}
	}
	runtime.KeepAlive(buf)
}
