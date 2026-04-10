package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/testutil"
)

func BenchmarkLoadIssuesFromFile(b *testing.B) {
	for _, size := range []int{100, 500, 1000, 5000} {
		b.Run(fmt.Sprintf("issues=%d", size), func(b *testing.B) {
			dir := b.TempDir()
			path := filepath.Join(dir, "beads.jsonl")

			issues := testutil.QuickRandom(size, 0.01)
			content := testutil.ToJSONL(issues)
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				b.Fatalf("write issues file: %v", err)
			}

			opts := ParseOptions{
				WarningHandler: func(string) {},
			}

			b.SetBytes(int64(len(content)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				loaded, err := LoadIssuesFromFileWithOptions(path, opts)
				if err != nil {
					b.Fatalf("load issues: %v", err)
				}
				if len(loaded) != len(issues) {
					b.Fatalf("unexpected issue count: got=%d want=%d", len(loaded), len(issues))
				}
			}
		})
	}
}
