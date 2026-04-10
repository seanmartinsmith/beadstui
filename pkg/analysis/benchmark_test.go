package analysis

import (
	"fmt"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/testutil"
)

func BenchmarkAnalyzePhase1Only(b *testing.B) {
	cfg := AnalysisConfig{}

	for _, size := range []int{100, 500, 1000, 5000} {
		b.Run(fmt.Sprintf("issues=%d", size), func(b *testing.B) {
			issues := testutil.QuickRandom(size, 0.01)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				analyzer := NewAnalyzer(issues)
				_ = analyzer.AnalyzeWithConfig(cfg)
			}
		})
	}
}
