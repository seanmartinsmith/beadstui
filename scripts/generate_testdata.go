// +build ignore

// generate_testdata.go creates standard test datasets for benchmarking.
// Usage: go run scripts/generate_testdata.go
//
// Creates:
//   tests/testdata/benchmark/small.jsonl   (100 issues)
//   tests/testdata/benchmark/medium.jsonl  (1000 issues)
//   tests/testdata/benchmark/large.jsonl   (5000 issues)
//   tests/testdata/benchmark/huge.jsonl    (20000 issues)
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/testutil"
)

type datasetSpec struct {
	name  string
	size  int
	desc  string
}

var datasets = []datasetSpec{
	{"small", 100, "100 issues - sparse random DAG with ~10% edge density"},
	{"medium", 1000, "1000 issues - sparse random DAG with ~5% edge density"},
	{"large", 5000, "5000 issues - sparse random DAG with ~2% edge density"},
	{"huge", 20000, "20000 issues - sparse random DAG with ~1% edge density"},
}

func main() {
	outputDir := "tests/testdata/benchmark"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	for _, ds := range datasets {
		fmt.Printf("Generating %s dataset (%d issues)...\n", ds.name, ds.size)

		// Use different density for different sizes to keep it realistic
		density := calculateDensity(ds.size)

		cfg := testutil.GeneratorConfig{
			Seed:           int64(ds.size), // Reproducible per-size
			IDPrefix:       "BENCH",
			IncludeLabels:  true,
			IncludeMinutes: true,
			StatusMix:      []model.Status{model.StatusOpen, model.StatusInProgress, model.StatusClosed},
			TypeMix:        []model.IssueType{model.TypeTask, model.TypeBug, model.TypeFeature, model.TypeEpic},
		}

		gen := testutil.New(cfg)
		gf := gen.RandomDAG(ds.size, density)
		issues := gen.ToIssues(gf)

		// Add realistic content
		addRealisticContent(issues, ds.desc)

		jsonl := testutil.ToJSONL(issues)

		outputPath := filepath.Join(outputDir, ds.name+".jsonl")
		if err := os.WriteFile(outputPath, []byte(jsonl), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write %s: %v\n", outputPath, err)
			os.Exit(1)
		}

		fmt.Printf("  Written %s (%d bytes, %d edges)\n", outputPath, len(jsonl), len(gf.Edges))
	}

	fmt.Println("\nDone! Test datasets created in", outputDir)
}

func calculateDensity(size int) float64 {
	// Scale density inversely with size to keep edge count reasonable
	// Small: ~10% edges, Large: ~1% edges
	switch {
	case size <= 100:
		return 0.1
	case size <= 1000:
		return 0.05
	case size <= 5000:
		return 0.02
	default:
		return 0.01
	}
}

func addRealisticContent(issues []model.Issue, datasetDesc string) {
	titles := []string{
		"Implement authentication flow",
		"Fix memory leak in cache",
		"Add API rate limiting",
		"Refactor database queries",
		"Update documentation",
		"Add unit tests for parser",
		"Optimize graph traversal",
		"Fix race condition in worker",
		"Add metrics dashboard",
		"Implement retry logic",
	}

	descriptions := []string{
		"This task involves implementing the core functionality.\n\n## Details\n- Step 1: Research\n- Step 2: Implement\n- Step 3: Test",
		"Bug fix for critical performance issue.\n\n## Reproduction\n1. Run under load\n2. Observe memory growth\n3. Check for leaks",
		"New feature request from stakeholders.\n\n## Acceptance Criteria\n- [ ] Works correctly\n- [ ] Has tests\n- [ ] Documented",
	}

	for i := range issues {
		issues[i].Title = fmt.Sprintf("[%s] %s #%d", datasetDesc[:6], titles[i%len(titles)], i)
		issues[i].Description = descriptions[i%len(descriptions)]
	}
}
