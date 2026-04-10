package ui_test

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/ui"
	"github.com/charmbracelet/lipgloss"
)

func generateLayeredIssues(levels, perLevel, fanIn int) []model.Issue {
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

func prepareGraphBench(levels, perLevel int) ([]model.Issue, *analysis.Insights, ui.Theme) {
	issues := generateLayeredIssues(levels, perLevel, 2)

	analyzer := analysis.NewAnalyzer(issues)
	stats := analyzer.AnalyzeAsync(context.Background())
	stats.WaitForPhase2()
	insights := stats.GenerateInsights(len(issues))

	theme := ui.DefaultTheme(lipgloss.NewRenderer(io.Discard))
	return issues, &insights, theme
}

func BenchmarkGraphModel_Rebuild_Layered1000(b *testing.B) {
	issues, insights, theme := prepareGraphBench(20, 50)

	b.ReportAllocs()
	b.ResetTimer()

	var g ui.GraphModel
	for i := 0; i < b.N; i++ {
		g = ui.NewGraphModel(issues, insights, theme)
	}
	runtime.KeepAlive(g)
}

func BenchmarkGraphModel_Rebuild_Layered2000(b *testing.B) {
	issues, insights, theme := prepareGraphBench(20, 100)

	b.ReportAllocs()
	b.ResetTimer()

	var g ui.GraphModel
	for i := 0; i < b.N; i++ {
		g = ui.NewGraphModel(issues, insights, theme)
	}
	runtime.KeepAlive(g)
}

func BenchmarkGraphModel_View_Narrow_Layered1000(b *testing.B) {
	issues, insights, theme := prepareGraphBench(20, 50)
	g := ui.NewGraphModel(issues, insights, theme)
	for i := 0; i < g.TotalCount()/2; i++ {
		g.MoveDown()
	}

	b.ReportAllocs()
	b.ResetTimer()

	var out string
	for i := 0; i < b.N; i++ {
		out = g.View(78, 40)
	}
	runtime.KeepAlive(out)
}

func BenchmarkGraphModel_Rebuild_Layered5000(b *testing.B) {
	issues, insights, theme := prepareGraphBench(50, 100)

	b.ReportAllocs()
	b.ResetTimer()

	var g ui.GraphModel
	for i := 0; i < b.N; i++ {
		g = ui.NewGraphModel(issues, insights, theme)
	}
	runtime.KeepAlive(g)
}

func BenchmarkGraphModel_Rebuild_Layered10000(b *testing.B) {
	issues, insights, theme := prepareGraphBench(100, 100)

	b.ReportAllocs()
	b.ResetTimer()

	var g ui.GraphModel
	for i := 0; i < b.N; i++ {
		g = ui.NewGraphModel(issues, insights, theme)
	}
	runtime.KeepAlive(g)
}

func BenchmarkGraphModel_View_Wide_Layered5000(b *testing.B) {
	issues, insights, theme := prepareGraphBench(50, 100)
	g := ui.NewGraphModel(issues, insights, theme)
	for i := 0; i < g.TotalCount()/2; i++ {
		g.MoveDown()
	}

	b.ReportAllocs()
	b.ResetTimer()

	var out string
	for i := 0; i < b.N; i++ {
		out = g.View(140, 40)
	}
	runtime.KeepAlive(out)
}

func BenchmarkGraphModel_View_Wide_Layered10000(b *testing.B) {
	issues, insights, theme := prepareGraphBench(100, 100)
	g := ui.NewGraphModel(issues, insights, theme)
	for i := 0; i < g.TotalCount()/2; i++ {
		g.MoveDown()
	}

	b.ReportAllocs()
	b.ResetTimer()

	var out string
	for i := 0; i < b.N; i++ {
		out = g.View(140, 40)
	}
	runtime.KeepAlive(out)
}
func BenchmarkGraphModel_View_Wide_Layered1000(b *testing.B) {
	issues, insights, theme := prepareGraphBench(20, 50)
	g := ui.NewGraphModel(issues, insights, theme)
	for i := 0; i < g.TotalCount()/2; i++ {
		g.MoveDown()
	}

	b.ReportAllocs()
	b.ResetTimer()

	var out string
	for i := 0; i < b.N; i++ {
		out = g.View(140, 40)
	}
	runtime.KeepAlive(out)
}
