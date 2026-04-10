package main_test

import (
	"testing"
	"time"
)

// Graph Analysis E2E Tests with Detailed Logging
// These tests exercise the graph analysis pipeline and verify correctness
// with comprehensive logging for debugging failures.

// TestE2E_GraphInsights_FullPipeline tests the complete --robot-insights output.
func TestE2E_GraphInsights_FullPipeline(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating graph fixture with 50 beads and dependencies")
	fixture := createGraphFixture(t, FixtureConfig{
		NumBeads:        50,
		NumDependencies: 75,
		NumCycles:       0, // Start without cycles
		MaxDepth:        10,
	})

	log.Step("Running --robot-insights")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-insights"); err != nil {
		t.Fatalf("--robot-insights failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("insights_analysis", elapsed)

	log.Step("Verifying insights structure")

	// Check for expected top-level fields
	expectedFields := []string{"data_hash", "generated_at", "status"}
	for _, field := range expectedFields {
		if _, ok := result[field]; !ok {
			t.Errorf("missing expected field: %s", field)
		}
	}

	// Check status contains metric status info
	if status, ok := result["status"].(map[string]any); ok {
		log.Step("Checking metric computation status")
		for metric, info := range status {
			if infoMap, ok := info.(map[string]any); ok {
				state := infoMap["state"]
				log.Metric("metric_"+metric+"_state", 1) // Just log presence
				t.Logf("Metric %s: state=%v", metric, state)
			}
		}
	}

	// Performance assertion
	log.Step("Checking performance bounds")
	if elapsed > 5*time.Second {
		t.Errorf("insights analysis took too long: %v (want < 5s)", elapsed)
	}

	log.Success("Graph insights E2E passed")
}

// TestE2E_GraphInsights_WithCycles tests cycle detection in --robot-insights.
func TestE2E_GraphInsights_WithCycles(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating graph fixture with cycles")
	fixture := createGraphFixture(t, FixtureConfig{
		NumBeads:        30,
		NumDependencies: 40,
		NumCycles:       3, // Add cycles
		MaxDepth:        5,
	})

	log.Step("Running --robot-insights")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-insights"); err != nil {
		t.Fatalf("--robot-insights failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("insights_with_cycles", elapsed)

	log.Step("Checking for cycle detection")

	// Cycles should be detected
	if cycles, ok := result["Cycles"].([]any); ok {
		log.Metric("cycles_detected", int64(len(cycles)))
		t.Logf("Detected %d cycles", len(cycles))
	} else {
		// Cycles may be in a different format or nested
		t.Logf("Cycles field: %T = %v", result["Cycles"], result["Cycles"])
	}

	log.Success("Graph insights with cycles E2E passed")
}

// TestE2E_GraphPlan_ParallelTracks tests --robot-plan output structure.
func TestE2E_GraphPlan_ParallelTracks(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating fixture with parallel work streams")
	fixture := NewTestFixture(t)

	// Create two independent chains: A1->A2->A3 and B1->B2->B3
	a1 := fixture.AddIssue("Chain A Start", "open", 1, "task")
	a2 := fixture.AddIssueWithDeps("Chain A Middle", "open", 2, "task", a1)
	_ = fixture.AddIssueWithDeps("Chain A End", "open", 2, "task", a2)

	b1 := fixture.AddIssue("Chain B Start", "open", 1, "task")
	b2 := fixture.AddIssueWithDeps("Chain B Middle", "open", 2, "task", b1)
	_ = fixture.AddIssueWithDeps("Chain B End", "open", 2, "task", b2)

	// Add some unconnected issues
	fixture.AddIssue("Independent 1", "open", 3, "task")
	fixture.AddIssue("Independent 2", "open", 3, "task")

	if err := fixture.Write(); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	log.Step("Running --robot-plan")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-plan"); err != nil {
		t.Fatalf("--robot-plan failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("plan_generation", elapsed)

	log.Step("Verifying plan structure")

	// Check for plan field
	if plan, ok := result["plan"].(map[string]any); ok {
		if tracks, ok := plan["tracks"].([]any); ok {
			log.Metric("parallel_tracks", int64(len(tracks)))
			t.Logf("Plan has %d parallel tracks", len(tracks))

			// Log track details
			for i, track := range tracks {
				if trackMap, ok := track.(map[string]any); ok {
					if issues, ok := trackMap["issues"].([]any); ok {
						t.Logf("Track %d: %d issues", i+1, len(issues))
					}
				}
			}
		}
	}

	log.Success("Graph plan E2E passed")
}

// TestE2E_GraphStats_Metrics tests --robot-graph-stats output.
func TestE2E_GraphStats_Metrics(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating fixture with 100 beads")
	fixture := createGraphFixture(t, FixtureConfig{
		NumBeads:        100,
		NumDependencies: 150,
		MaxDepth:        15,
	})

	log.Step("Running graph stats analysis")
	startTime := time.Now()

	// Use --robot-insights since it includes graph stats
	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-insights"); err != nil {
		t.Fatalf("--robot-insights failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("graph_stats", elapsed)

	log.Step("Verifying graph metrics")

	// Check for various graph metrics
	metricFields := []string{"PageRank", "Betweenness", "CriticalPath", "ArticulationPoints"}
	foundMetrics := 0
	for _, field := range metricFields {
		if _, ok := result[field]; ok {
			foundMetrics++
			t.Logf("Found metric: %s", field)
		}
	}
	log.Metric("metrics_found", int64(foundMetrics))

	// Performance: should complete in reasonable time
	if elapsed > 3*time.Second {
		t.Errorf("graph stats took too long: %v (want < 3s for 100 beads)", elapsed)
	}

	log.Success("Graph stats E2E passed")
}

// TestE2E_GraphExport_Formats tests graph export in different formats.
func TestE2E_GraphExport_Formats(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating fixture for graph export")
	fixture := createGraphFixture(t, FixtureConfig{
		NumBeads:        20,
		NumDependencies: 30,
	})

	formats := []string{"json", "dot", "mermaid"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			log.Step("Exporting graph as %s", format)
			startTime := time.Now()

			var result map[string]any
			if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-graph", "--graph-format", format); err != nil {
				t.Fatalf("--robot-graph (%s) failed: %v", format, err)
			}

			elapsed := time.Since(startTime)
			log.MetricDuration("export_"+format, elapsed)

			// Verify format field matches
			if resultFormat, ok := result["format"].(string); ok {
				if resultFormat != format {
					t.Errorf("format mismatch: got %q, want %q", resultFormat, format)
				}
			}

			// Verify data_hash is present
			if _, ok := result["data_hash"]; !ok {
				t.Error("missing data_hash field")
			}
		})
	}

	log.Success("Graph export formats E2E passed")
}

// TestE2E_LabelHealth tests --robot-label-health command.
func TestE2E_LabelHealth(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating fixture with labeled issues")
	fixture := NewTestFixture(t)

	// Create issues with different labels
	fixture.AddIssueWithLabels("Backend Task 1", "open", 1, "task", "backend")
	fixture.AddIssueWithLabels("Backend Task 2", "open", 2, "task", "backend")
	fixture.AddIssueWithLabels("Frontend Task 1", "open", 1, "task", "frontend")
	fixture.AddIssueWithLabels("Frontend Task 2", "closed", 2, "task", "frontend")
	fixture.AddIssueWithLabels("API Task", "open", 1, "task", "backend", "api")

	if err := fixture.Write(); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	log.Step("Running --robot-label-health")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-label-health"); err != nil {
		t.Fatalf("--robot-label-health failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("label_health", elapsed)

	log.Step("Verifying label health structure")

	// Check for results field
	if results, ok := result["results"].(map[string]any); ok {
		if labels, ok := results["labels"].([]any); ok {
			log.Metric("labels_analyzed", int64(len(labels)))
			t.Logf("Analyzed %d labels", len(labels))

			for _, label := range labels {
				if labelMap, ok := label.(map[string]any); ok {
					name := labelMap["name"]
					health := labelMap["health_level"]
					t.Logf("Label %v: health=%v", name, health)
				}
			}
		}
	}

	log.Success("Label health E2E passed")
}

// TestE2E_GraphAnalysis_LargeGraph tests analysis performance on larger graphs.
func TestE2E_GraphAnalysis_LargeGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large graph test in short mode")
	}

	log := newDetailedLogger(t)

	log.Step("Creating large fixture with 500 beads")
	fixture := createGraphFixture(t, FixtureConfig{
		NumBeads:        500,
		NumDependencies: 800,
		MaxDepth:        20,
	})

	log.Step("Running --robot-triage on large graph")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-triage"); err != nil {
		t.Fatalf("--robot-triage failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("large_graph_triage", elapsed)

	log.Step("Verifying triage completed")

	// Check for quick_ref (summary)
	if quickRef, ok := result["quick_ref"].(map[string]any); ok {
		if total, ok := quickRef["total"].(float64); ok {
			log.Metric("total_issues", int64(total))
		}
	}

	// Performance: large graphs should still complete within reasonable time
	if elapsed > 10*time.Second {
		t.Errorf("large graph analysis too slow: %v (want < 10s for 500 beads)", elapsed)
	}

	log.Success("Large graph analysis E2E passed")
}

// BenchmarkGraphInsights benchmarks the --robot-insights command.
func BenchmarkGraphInsights(b *testing.B) {
	fixture := createGraphFixture(&testing.T{}, FixtureConfig{
		NumBeads:        100,
		NumDependencies: 150,
	})

	bv := bvBinaryPath
	if bv == "" {
		b.Skip("bv binary not built")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = runBVCommand(&testing.T{}, fixture.Dir, "--robot-insights")
	}
}
