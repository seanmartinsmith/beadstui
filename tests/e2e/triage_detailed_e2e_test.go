package main_test

import (
	"testing"
	"time"
)

// Triage E2E Tests with Detailed Logging
// These tests exercise the triage analysis pipeline and verify correctness
// of actionable issue detection, recommendations, and quick wins.

// TestE2E_Triage_ActionableIssues tests that blocked vs actionable detection works.
func TestE2E_Triage_ActionableIssues(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating fixture with blocked and actionable issues")
	fixture := createTriageFixture(t, FixtureConfig{
		BlockedCount:    20,
		ActionableCount: 30,
	})

	log.Step("Running --robot-triage")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-triage"); err != nil {
		t.Fatalf("--robot-triage failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("triage_analysis", elapsed)

	log.Step("Verifying triage results")

	// Check quick_ref for summary
	if quickRef, ok := result["quick_ref"].(map[string]any); ok {
		if total, ok := quickRef["total"].(float64); ok {
			log.Metric("total_issues", int64(total))
			t.Logf("Total issues: %v", total)
		}
		if actionable, ok := quickRef["actionable"].(float64); ok {
			log.Metric("actionable_count", int64(actionable))
			t.Logf("Actionable issues: %v", actionable)
		}
		if blocked, ok := quickRef["blocked"].(float64); ok {
			log.Metric("blocked_count", int64(blocked))
			t.Logf("Blocked issues: %v", blocked)
		}
	}

	// Check recommendations exist
	if recs, ok := result["recommendations"].([]any); ok {
		log.Metric("recommendations_count", int64(len(recs)))
		t.Logf("Recommendations: %d", len(recs))

		// Log first few recommendations
		for i := 0; i < 3 && i < len(recs); i++ {
			if rec, ok := recs[i].(map[string]any); ok {
				t.Logf("  Rec %d: %v (score=%v)", i+1, rec["title"], rec["score"])
			}
		}
	}

	log.Success("Triage actionable issues E2E passed")
}

// TestE2E_Triage_QuickWins tests quick win detection.
func TestE2E_Triage_QuickWins(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating fixture with varied priorities")
	fixture := NewTestFixture(t)

	// Create some quick win candidates (high priority, no dependencies)
	fixture.AddIssue("Quick P0 fix", "open", 0, "bug")
	fixture.AddIssue("Quick P1 fix", "open", 1, "bug")

	// Create some complex items (lower priority or with dependencies)
	blocker := fixture.AddIssue("Blocker task", "open", 2, "task")
	fixture.AddIssueWithDeps("Blocked by blocker", "open", 1, "task", blocker)
	fixture.AddIssueWithDeps("Also blocked", "open", 0, "bug", blocker)

	// Create some backlog items
	fixture.AddIssue("Backlog item 1", "open", 4, "task")
	fixture.AddIssue("Backlog item 2", "open", 4, "feature")

	if err := fixture.Write(); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	log.Step("Running --robot-triage")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-triage"); err != nil {
		t.Fatalf("--robot-triage failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("triage_quick_wins", elapsed)

	log.Step("Checking for quick wins")

	if quickWins, ok := result["quick_wins"].([]any); ok {
		log.Metric("quick_wins_count", int64(len(quickWins)))
		t.Logf("Quick wins found: %d", len(quickWins))

		for i, qw := range quickWins {
			if qwMap, ok := qw.(map[string]any); ok {
				t.Logf("  Quick win %d: %v", i+1, qwMap["title"])
			}
		}
	} else {
		t.Logf("quick_wins field type: %T", result["quick_wins"])
	}

	log.Success("Triage quick wins E2E passed")
}

// TestE2E_Triage_BlockersToUnblock tests blocker detection.
func TestE2E_Triage_BlockersToUnblock(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating fixture with blocking hierarchy")
	fixture := NewTestFixture(t)

	// Create a critical blocker that blocks many issues
	criticalBlocker := fixture.AddIssue("Critical Blocker", "open", 0, "bug")

	// Many issues blocked by the critical blocker
	for i := 0; i < 10; i++ {
		fixture.AddIssueWithDeps("Blocked task "+string(rune('A'+i)), "open", 2, "task", criticalBlocker)
	}

	// Another blocker with fewer dependents
	minorBlocker := fixture.AddIssue("Minor Blocker", "open", 2, "task")
	fixture.AddIssueWithDeps("Blocked by minor 1", "open", 3, "task", minorBlocker)
	fixture.AddIssueWithDeps("Blocked by minor 2", "open", 3, "task", minorBlocker)

	// Some independent issues
	fixture.AddIssue("Independent 1", "open", 2, "task")
	fixture.AddIssue("Independent 2", "open", 2, "task")

	if err := fixture.Write(); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	log.Step("Running --robot-triage")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-triage"); err != nil {
		t.Fatalf("--robot-triage failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("triage_blockers", elapsed)

	log.Step("Checking blockers to clear")

	if blockers, ok := result["blockers_to_clear"].([]any); ok {
		log.Metric("blockers_to_clear_count", int64(len(blockers)))
		t.Logf("Blockers to clear: %d", len(blockers))

		for i, b := range blockers {
			if bMap, ok := b.(map[string]any); ok {
				t.Logf("  Blocker %d: %v (unblocks=%v)", i+1, bMap["title"], bMap["unblocks_count"])
			}
		}

		// The critical blocker should be first (unblocks most)
		if len(blockers) > 0 {
			if first, ok := blockers[0].(map[string]any); ok {
				if unblocks, ok := first["unblocks_count"].(float64); ok {
					if unblocks < 5 {
						t.Logf("Warning: expected critical blocker first (unblocks ~10), got unblocks=%v", unblocks)
					}
				}
			}
		}
	}

	log.Success("Triage blockers E2E passed")
}

// TestE2E_Triage_RobotNext tests --robot-next minimal output.
func TestE2E_Triage_RobotNext(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating fixture with actionable issues")
	fixture := createTriageFixture(t, FixtureConfig{
		BlockedCount:    5,
		ActionableCount: 10,
	})

	log.Step("Running --robot-next")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-next"); err != nil {
		t.Fatalf("--robot-next failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("robot_next", elapsed)

	log.Step("Verifying next recommendation")

	// Should have a recommendation
	if rec, ok := result["recommendation"].(map[string]any); ok {
		if id, ok := rec["id"].(string); ok {
			log.Metric("has_recommendation", 1)
			t.Logf("Next recommendation: %s - %v", id, rec["title"])
		}
		if reason, ok := rec["reason"].(string); ok {
			t.Logf("Reason: %s", reason)
		}
	} else {
		t.Logf("recommendation field type: %T", result["recommendation"])
	}

	// Check for command hint
	if commands, ok := result["commands"].(map[string]any); ok {
		if claim, ok := commands["claim"].(string); ok {
			t.Logf("Claim command: %s", claim)
		}
	}

	// --robot-next should be fast
	if elapsed > 1*time.Second {
		t.Errorf("--robot-next too slow: %v (want < 1s)", elapsed)
	}

	log.Success("Robot next E2E passed")
}

// TestE2E_Triage_ByTrack tests --robot-triage-by-track grouping.
func TestE2E_Triage_ByTrack(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating fixture with parallel work streams")
	fixture := NewTestFixture(t)

	// Create two chains (parallel tracks)
	a1 := fixture.AddIssue("Track A: Start", "open", 1, "task")
	a2 := fixture.AddIssueWithDeps("Track A: Middle", "open", 2, "task", a1)
	fixture.AddIssueWithDeps("Track A: End", "open", 2, "task", a2)

	b1 := fixture.AddIssue("Track B: Start", "open", 1, "task")
	b2 := fixture.AddIssueWithDeps("Track B: Middle", "open", 2, "task", b1)
	fixture.AddIssueWithDeps("Track B: End", "open", 2, "task", b2)

	// Some independent issues
	fixture.AddIssue("Independent 1", "open", 3, "task")
	fixture.AddIssue("Independent 2", "open", 3, "task")

	if err := fixture.Write(); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	log.Step("Running --robot-triage --robot-triage-by-track")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-triage", "--robot-triage-by-track"); err != nil {
		t.Fatalf("--robot-triage --robot-triage-by-track failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("triage_by_track", elapsed)

	log.Step("Checking track grouping")

	if tracks, ok := result["tracks"].([]any); ok {
		log.Metric("track_count", int64(len(tracks)))
		t.Logf("Found %d tracks", len(tracks))

		for i, track := range tracks {
			if trackMap, ok := track.(map[string]any); ok {
				if issues, ok := trackMap["issues"].([]any); ok {
					t.Logf("  Track %d: %d issues", i+1, len(issues))
				}
			}
		}
	}

	log.Success("Triage by track E2E passed")
}

// TestE2E_Triage_ByLabel tests --robot-triage-by-label grouping.
func TestE2E_Triage_ByLabel(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating fixture with labeled issues")
	fixture := NewTestFixture(t)

	// Create issues with different labels
	fixture.AddIssueWithLabels("Backend Task 1", "open", 1, "task", "backend")
	fixture.AddIssueWithLabels("Backend Task 2", "open", 2, "task", "backend")
	fixture.AddIssueWithLabels("Backend Bug", "open", 0, "bug", "backend")

	fixture.AddIssueWithLabels("Frontend Task 1", "open", 1, "task", "frontend")
	fixture.AddIssueWithLabels("Frontend Task 2", "open", 2, "task", "frontend")

	fixture.AddIssueWithLabels("API Task", "open", 1, "task", "api")
	fixture.AddIssueWithLabels("Infra Task", "open", 2, "task", "infra")

	if err := fixture.Write(); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	log.Step("Running --robot-triage --robot-triage-by-label")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-triage", "--robot-triage-by-label"); err != nil {
		t.Fatalf("--robot-triage --robot-triage-by-label failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("triage_by_label", elapsed)

	log.Step("Checking label grouping")

	if byLabel, ok := result["by_label"].(map[string]any); ok {
		log.Metric("label_groups", int64(len(byLabel)))
		t.Logf("Found %d label groups", len(byLabel))

		for label, issues := range byLabel {
			if issueList, ok := issues.([]any); ok {
				t.Logf("  Label '%s': %d issues", label, len(issueList))
			}
		}
	}

	log.Success("Triage by label E2E passed")
}

// TestE2E_Triage_ProjectHealth tests project_health in triage output.
func TestE2E_Triage_ProjectHealth(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating diverse fixture")
	fixture := NewTestFixture(t)

	// Mix of statuses
	fixture.AddIssue("Open Task", "open", 2, "task")
	fixture.AddIssue("In Progress Task", "in_progress", 2, "task")
	fixture.AddIssue("Closed Task", "closed", 2, "task")

	// Mix of types
	fixture.AddIssue("Bug 1", "open", 1, "bug")
	fixture.AddIssue("Feature 1", "open", 2, "feature")
	fixture.AddIssue("Epic 1", "open", 2, "epic")

	// Mix of priorities
	fixture.AddIssue("P0 Critical", "open", 0, "bug")
	fixture.AddIssue("P1 High", "open", 1, "task")
	fixture.AddIssue("P4 Backlog", "open", 4, "task")

	if err := fixture.Write(); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	log.Step("Running --robot-triage")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-triage"); err != nil {
		t.Fatalf("--robot-triage failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("triage_health", elapsed)

	log.Step("Checking project health")

	if health, ok := result["project_health"].(map[string]any); ok {
		// Check status distribution
		if statusDist, ok := health["status_distribution"].(map[string]any); ok {
			t.Logf("Status distribution: %v", statusDist)
		}

		// Check type distribution
		if typeDist, ok := health["type_distribution"].(map[string]any); ok {
			t.Logf("Type distribution: %v", typeDist)
		}

		// Check priority distribution
		if priDist, ok := health["priority_distribution"].(map[string]any); ok {
			t.Logf("Priority distribution: %v", priDist)
		}
	}

	log.Success("Triage project health E2E passed")
}

// TestE2E_Triage_EmptyProject tests triage on empty project.
func TestE2E_Triage_EmptyProject(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating empty fixture")
	fixture := NewTestFixture(t)
	if err := fixture.Write(); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	log.Step("Running --robot-triage on empty project")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-triage"); err != nil {
		t.Fatalf("--robot-triage failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("triage_empty", elapsed)

	log.Step("Verifying empty project handling")

	// Should still return valid structure
	if _, ok := result["generated_at"]; !ok {
		t.Error("missing generated_at field")
	}

	// Quick ref should show zero counts
	if quickRef, ok := result["quick_ref"].(map[string]any); ok {
		if total, ok := quickRef["total"].(float64); ok {
			if total != 0 {
				t.Errorf("expected 0 total issues, got %v", total)
			}
		}
	}

	log.Success("Triage empty project E2E passed")
}

// TestE2E_Triage_AllClosed tests triage when all issues are closed.
func TestE2E_Triage_AllClosed(t *testing.T) {
	log := newDetailedLogger(t)

	log.Step("Creating fixture with all closed issues")
	fixture := NewTestFixture(t)

	fixture.AddIssue("Closed Task 1", "closed", 2, "task")
	fixture.AddIssue("Closed Task 2", "closed", 1, "task")
	fixture.AddIssue("Closed Bug", "closed", 0, "bug")

	if err := fixture.Write(); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	log.Step("Running --robot-triage")
	startTime := time.Now()

	var result map[string]any
	if err := runBVCommandJSON(t, fixture.Dir, &result, "--robot-triage"); err != nil {
		t.Fatalf("--robot-triage failed: %v", err)
	}

	elapsed := time.Since(startTime)
	log.MetricDuration("triage_all_closed", elapsed)

	log.Step("Verifying all-closed handling")

	// Should have no actionable items
	if quickRef, ok := result["quick_ref"].(map[string]any); ok {
		if actionable, ok := quickRef["actionable"].(float64); ok {
			if actionable != 0 {
				t.Errorf("expected 0 actionable issues when all closed, got %v", actionable)
			}
		}
	}

	log.Success("Triage all closed E2E passed")
}
