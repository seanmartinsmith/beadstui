package ui

import (
	"fmt"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/testutil"
)

func copyIssues(in []model.Issue) []model.Issue {
	if in == nil {
		return nil
	}
	out := make([]model.Issue, len(in))
	copy(out, in)
	return out
}

func BenchmarkSnapshotSwap(b *testing.B) {
	for _, size := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("issues=%d", size), func(b *testing.B) {
			issues := testutil.QuickRandom(size, 0.01)

			m := NewModel(copyIssues(issues), nil, "")
			snapshot := NewSnapshotBuilder(copyIssues(issues)).Build()

			tm, _ := m.Update(SnapshotReadyMsg{Snapshot: snapshot})
			m = tm.(Model)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tm, _ := m.Update(SnapshotReadyMsg{Snapshot: snapshot})
				m = tm.(Model)
			}
		})
	}
}

func BenchmarkSnapshotBuilderBuild(b *testing.B) {
	cfg := analysis.AnalysisConfig{}

	for _, size := range []int{100, 500, 1000, 5000} {
		b.Run(fmt.Sprintf("issues=%d", size), func(b *testing.B) {
			base := testutil.QuickRandom(size, 0.01)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				issues := copyIssues(base)
				b.StartTimer()

				builder := NewSnapshotBuilder(issues)
				stats := builder.analyzer.AnalyzeWithConfig(cfg)
				builder.WithAnalysis(&stats)

				snap := builder.Build()
				if snap == nil {
					b.Fatalf("unexpected snapshot: nil")
				}
				if len(snap.Issues) != len(base) {
					b.Fatalf("unexpected snapshot issue count: got=%d want=%d", len(snap.Issues), len(base))
				}
			}
		})
	}
}
