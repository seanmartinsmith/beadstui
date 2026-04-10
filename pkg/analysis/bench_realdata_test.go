package analysis_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// ============================================================================
// Real-Data Benchmarks: Use the project's actual .beads directory
// ============================================================================
//
// These benchmarks use real issue data from this project to test realistic
// workloads. This is critical for accurate profiling and optimization work.
//
// Run with profiling:
//   go test -bench=BenchmarkRealData -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./pkg/analysis/...
//
// View profiles:
//   go tool pprof -http=:8080 cpu.prof
//   go tool pprof -http=:8080 mem.prof

// findProjectBeadsDir locates the .beads directory relative to the test file
func findProjectBeadsDir() (string, error) {
	// Try common relative paths from test execution directory
	candidates := []string{
		".beads",       // Current directory
		"../../.beads", // pkg/analysis -> root
	}

	// Also check BEADS_DIR environment variable
	if envDir := os.Getenv("BEADS_DIR"); envDir != "" {
		candidates = append([]string{envDir}, candidates...)
	}

	for _, candidate := range candidates {
		issuesPath := filepath.Join(candidate, "issues.jsonl")
		if _, err := os.Stat(issuesPath); err == nil {
			return candidate, nil
		}
	}

	return "", os.ErrNotExist
}

// loadProjectBeads loads issues from the project's .beads directory
func loadProjectBeads(tb testing.TB) []model.Issue {
	tb.Helper()

	beadsDir, err := findProjectBeadsDir()
	if err != nil {
		tb.Skipf("No .beads directory found: %v", err)
	}

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")
	content, err := os.ReadFile(issuesPath)
	if err != nil {
		tb.Fatalf("Failed to read issues from %s: %v", issuesPath, err)
	}

	var issues []model.Issue
	scanner := bufio.NewScanner(bytes.NewReader(content))
	// Increase buffer for large lines
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var issue model.Issue
		if err := json.Unmarshal(line, &issue); err != nil {
			continue
		}
		issues = append(issues, issue)
	}

	if err := scanner.Err(); err != nil {
		tb.Fatalf("Scanner error: %v", err)
	}

	if len(issues) == 0 {
		tb.Skip("No issues found in .beads/issues.jsonl")
	}

	return issues
}

// BenchmarkRealData_FullTriage benchmarks the complete triage workflow
// on the project's real issue data.
func BenchmarkRealData_FullTriage(b *testing.B) {
	issues := loadProjectBeads(b)
	b.Logf("Loaded %d real issues for benchmark", len(issues))

	opts := analysis.TriageOptions{WaitForPhase2: true}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = analysis.ComputeTriageWithOptions(issues, opts)
	}
}

// BenchmarkRealData_TriagePhase1Only benchmarks triage without waiting for
// Phase 2 async metrics (PageRank, betweenness, etc.)
func BenchmarkRealData_TriagePhase1Only(b *testing.B) {
	issues := loadProjectBeads(b)
	b.Logf("Loaded %d real issues for benchmark", len(issues))

	opts := analysis.TriageOptions{WaitForPhase2: false}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = analysis.ComputeTriageWithOptions(issues, opts)
	}
}

// BenchmarkRealData_FullAnalysis benchmarks the complete graph analysis
// (not triage) on real data.
func BenchmarkRealData_FullAnalysis(b *testing.B) {
	issues := loadProjectBeads(b)
	b.Logf("Loaded %d real issues for benchmark", len(issues))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		an := analysis.NewAnalyzer(issues)
		_ = an.AnalyzeWithConfig(analysis.FullAnalysisConfig())
	}
}

// BenchmarkRealData_FastAnalysis benchmarks Phase 1 only analysis.
func BenchmarkRealData_FastAnalysis(b *testing.B) {
	issues := loadProjectBeads(b)
	b.Logf("Loaded %d real issues for benchmark", len(issues))

	cfg := analysis.AnalysisConfig{} // Default: Phase 1 only

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		an := analysis.NewAnalyzer(issues)
		_ = an.AnalyzeWithConfig(cfg)
	}
}

// BenchmarkRealData_GraphBuild benchmarks only the graph construction
// (NewAnalyzer) without running any analysis.
func BenchmarkRealData_GraphBuild(b *testing.B) {
	issues := loadProjectBeads(b)
	b.Logf("Loaded %d real issues for benchmark", len(issues))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = analysis.NewAnalyzer(issues)
	}
}

// BenchmarkRealData_IssueLoading benchmarks loading and parsing issues from JSONL.
func BenchmarkRealData_IssueLoading(b *testing.B) {
	beadsDir, err := findProjectBeadsDir()
	if err != nil {
		b.Skipf("No .beads directory found: %v", err)
	}

	issuesPath := filepath.Join(beadsDir, "issues.jsonl")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		content, _ := os.ReadFile(issuesPath)
		var issues []model.Issue
		scanner := bufio.NewScanner(bytes.NewReader(content))
		buf := make([]byte, 1024*1024)
		scanner.Buffer(buf, len(buf))
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var issue model.Issue
			if err := json.Unmarshal(line, &issue); err != nil {
				continue
			}
			issues = append(issues, issue)
		}
		_ = issues
	}
}
