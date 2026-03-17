package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/loader"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// runProfileStartup runs profiled startup analysis and outputs results
func runProfileStartup(issues []model.Issue, loadDuration time.Duration, jsonOutput, forceFullAnalysis bool) {
	// Get actual beads path (respects BEADS_DIR)
	beadsDir, _ := loader.GetBeadsDir("")
	dataPath, _ := loader.FindJSONLPath(beadsDir)
	if dataPath == "" {
		dataPath = beadsDir // fallback
	}

	// Time analyzer construction
	buildStart := time.Now()
	analyzer := analysis.NewAnalyzer(issues)
	buildDuration := time.Since(buildStart)

	// Select config
	var config analysis.AnalysisConfig
	if forceFullAnalysis {
		config = analysis.FullAnalysisConfig()
	} else {
		nodeCount := len(issues)
		// Estimate edge count from issues
		edgeCount := 0
		for _, issue := range issues {
			edgeCount += len(issue.Dependencies)
		}
		config = analysis.ConfigForSize(nodeCount, edgeCount)
	}

	// Run profiled analysis
	_, profile := analyzer.AnalyzeWithProfile(config)

	// Add load and build durations to profile
	profile.BuildGraph = buildDuration

	// Calculate total including load
	totalWithLoad := loadDuration + profile.Total

	if jsonOutput {
		// JSON output
		output := struct {
			GeneratedAt     string                   `json:"generated_at"`
			DataPath        string                   `json:"data_path"`
			LoadJSONL       string                   `json:"load_jsonl"`
			Profile         *analysis.StartupProfile `json:"profile"`
			TotalWithLoad   string                   `json:"total_with_load"`
			Recommendations []string                 `json:"recommendations"`
		}{
			GeneratedAt:     timeNowUTCRFC3339(),
			DataPath:        dataPath,
			LoadJSONL:       loadDuration.String(),
			Profile:         profile,
			TotalWithLoad:   totalWithLoad.String(),
			Recommendations: generateProfileRecommendations(profile, loadDuration, totalWithLoad),
		}

		encoder := newRobotEncoder(os.Stdout)
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding profile: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Human-readable output
		printProfileReport(profile, loadDuration, totalWithLoad)
	}
}

// printProfileReport outputs a human-readable startup profile
func printProfileReport(profile *analysis.StartupProfile, loadDuration, totalWithLoad time.Duration) {
	fmt.Println("Startup Profile")
	fmt.Println("===============")
	fmt.Printf("Data: %d issues, %d dependencies, density=%.4f\n\n",
		profile.NodeCount, profile.EdgeCount, profile.Density)

	// Phase 1
	fmt.Println("Phase 1 (blocking):")
	fmt.Printf("  Load JSONL:      %v\n", formatDuration(loadDuration))
	fmt.Printf("  Build graph:     %v\n", formatDuration(profile.BuildGraph))
	fmt.Printf("  Degree:          %v\n", formatDuration(profile.Degree))
	fmt.Printf("  TopoSort:        %v\n", formatDuration(profile.TopoSort))
	fmt.Printf("  Total Phase 1:   %v\n\n", formatDuration(loadDuration+profile.BuildGraph+profile.Phase1))

	// Phase 2
	fmt.Println("Phase 2 (async in normal mode, sync for profiling):")
	printMetricLine("PageRank", profile.PageRank, profile.PageRankTO, profile.Config.ComputePageRank)
	printMetricLine("Betweenness", profile.Betweenness, profile.BetweennessTO, profile.Config.ComputeBetweenness)
	printMetricLine("Eigenvector", profile.Eigenvector, false, profile.Config.ComputeEigenvector)
	printMetricLine("HITS", profile.HITS, profile.HITSTO, profile.Config.ComputeHITS)
	printMetricLine("Critical Path", profile.CriticalPath, false, profile.Config.ComputeCriticalPath)
	printCyclesLine(profile)
	fmt.Printf("  Total Phase 2:   %v\n\n", formatDuration(profile.Phase2))

	// Total
	fmt.Printf("Total startup:     %v\n\n", formatDuration(totalWithLoad))

	// Configuration used
	fmt.Println("Configuration:")
	fmt.Printf("  Size tier: %s\n", getSizeTier(profile.NodeCount))
	skipped := profile.Config.SkippedMetrics()
	if len(skipped) > 0 {
		var names []string
		for _, s := range skipped {
			names = append(names, s.Name)
		}
		fmt.Printf("  Skipped metrics: %s\n", strings.Join(names, ", "))
	} else {
		fmt.Println("  All metrics computed")
	}
	fmt.Println()

	// Recommendations
	recommendations := generateProfileRecommendations(profile, loadDuration, totalWithLoad)
	if len(recommendations) > 0 {
		fmt.Println("Recommendations:")
		for _, rec := range recommendations {
			fmt.Printf("  %s\n", rec)
		}
	}
}

// printMetricLine prints a single metric timing line
func printMetricLine(name string, duration time.Duration, timedOut, computed bool) {
	if !computed {
		fmt.Printf("  %-14s [Skipped]\n", name+":")
		return
	}
	suffix := ""
	if timedOut {
		suffix = " (TIMEOUT)"
	}
	fmt.Printf("  %-14s %v%s\n", name+":", formatDuration(duration), suffix)
}

// printCyclesLine prints the cycles metric line with count
func printCyclesLine(profile *analysis.StartupProfile) {
	if !profile.Config.ComputeCycles {
		fmt.Printf("  %-14s [Skipped]\n", "Cycles:")
		return
	}
	suffix := ""
	if profile.CyclesTO {
		suffix = " (TIMEOUT)"
	} else if profile.CycleCount > 0 {
		suffix = fmt.Sprintf(" (found: %d)", profile.CycleCount)
	} else {
		suffix = " (none)"
	}
	fmt.Printf("  %-14s %v%s\n", "Cycles:", formatDuration(profile.Cycles), suffix)
}

// getSizeTier returns the size tier name based on node count
func getSizeTier(nodeCount int) string {
	switch {
	case nodeCount < 100:
		return "Small (<100 issues)"
	case nodeCount < 500:
		return "Medium (100-500 issues)"
	case nodeCount < 2000:
		return "Large (500-2000 issues)"
	default:
		return "XL (>2000 issues)"
	}
}

// generateProfileRecommendations generates actionable recommendations based on profile
func generateProfileRecommendations(profile *analysis.StartupProfile, loadDuration, totalWithLoad time.Duration) []string {
	var recs []string

	// Check overall startup time
	if totalWithLoad < 500*time.Millisecond {
		recs = append(recs, "✓ Startup within acceptable range (<500ms)")
	} else if totalWithLoad < 1*time.Second {
		recs = append(recs, "✓ Startup acceptable (<1s)")
	} else if totalWithLoad < 2*time.Second {
		// Check if full analysis is being used (no skipped metrics on a large graph)
		if len(profile.Config.SkippedMetrics()) == 0 && profile.NodeCount >= 500 {
			recs = append(recs, "⚠ Startup is slow (1-2s) - if using --force-full-analysis, consider removing it")
		} else {
			recs = append(recs, "⚠ Startup is slow (1-2s)")
		}
	} else {
		recs = append(recs, "⚠ Startup is very slow (>2s) - optimization recommended")
	}

	// Check for timeouts
	if profile.PageRankTO {
		recs = append(recs, "⚠ PageRank timed out - graph may be too large or dense")
	}
	if profile.BetweennessTO {
		recs = append(recs, "⚠ Betweenness timed out - this is expected for large graphs (>500 nodes)")
	}
	if profile.HITSTO {
		recs = append(recs, "⚠ HITS timed out - graph may have convergence issues")
	}
	if profile.CyclesTO {
		recs = append(recs, "⚠ Cycle detection timed out - graph may have many overlapping cycles")
	}

	// Check which metric is taking longest
	if profile.Config.ComputeBetweenness && profile.Betweenness > 0 {
		phase2NoZero := profile.Phase2
		if phase2NoZero > 0 {
			betweennessPercent := float64(profile.Betweenness) / float64(phase2NoZero) * 100
			if betweennessPercent > 50 {
				recs = append(recs, fmt.Sprintf("⚠ Betweenness taking %.0f%% of Phase 2 time - consider skipping for large graphs", betweennessPercent))
			}
		}
	}

	// Check for cycles
	if profile.CycleCount > 0 {
		recs = append(recs, fmt.Sprintf("⚠ Found %d circular dependencies - resolve to improve graph health", profile.CycleCount))
	}

	return recs
}
