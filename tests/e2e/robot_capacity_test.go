package main_test

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"time"
)

func TestRobotCapacity_EstimatedDaysDropsWithMoreAgents(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	now := time.Now().UTC().Format(time.RFC3339)

	// Three independent tasks; estimated_minutes drives total_minutes deterministically.
	writeBeads(t, env, fmt.Sprintf(
		`{"id":"A","title":"A","status":"open","priority":1,"issue_type":"task","estimated_minutes":480,"labels":["backend"],"created_at":"%s","updated_at":"%s"}
{"id":"B","title":"B","status":"open","priority":1,"issue_type":"task","estimated_minutes":480,"labels":["backend"],"created_at":"%s","updated_at":"%s"}
{"id":"C","title":"C","status":"open","priority":1,"issue_type":"task","estimated_minutes":480,"labels":["frontend"],"created_at":"%s","updated_at":"%s"}`,
		now, now, now, now, now, now,
	))

	run := func(args ...string) struct {
		Agents         int     `json:"agents"`
		Label          string  `json:"label"`
		OpenIssueCount int     `json:"open_issue_count"`
		EstimatedDays  float64 `json:"estimated_days"`
		TotalMinutes   int     `json:"total_minutes"`
	} {
		t.Helper()
		cmd := exec.Command(bv, args...)
		cmd.Dir = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
		var payload struct {
			Agents         int     `json:"agents"`
			Label          string  `json:"label"`
			OpenIssueCount int     `json:"open_issue_count"`
			EstimatedDays  float64 `json:"estimated_days"`
			TotalMinutes   int     `json:"total_minutes"`
		}
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("json decode: %v\nout=%s", err, out)
		}
		return payload
	}

	one := run("--robot-capacity", "--agents=1")
	three := run("--robot-capacity", "--agents=3")

	if one.OpenIssueCount != 3 || three.OpenIssueCount != 3 {
		t.Fatalf("open_issue_count mismatch: one=%d three=%d", one.OpenIssueCount, three.OpenIssueCount)
	}
	if one.TotalMinutes <= 0 || three.TotalMinutes != one.TotalMinutes {
		t.Fatalf("total_minutes mismatch: one=%d three=%d", one.TotalMinutes, three.TotalMinutes)
	}
	if !(three.EstimatedDays < one.EstimatedDays) {
		t.Fatalf("expected estimated_days to drop with more agents: one=%.3f three=%.3f", one.EstimatedDays, three.EstimatedDays)
	}

	// Label filter.
	backend := run("--robot-capacity", "--capacity-label=backend", "--agents=1")
	if backend.Label != "backend" {
		t.Fatalf("label=%q; want backend", backend.Label)
	}
	if backend.OpenIssueCount != 2 {
		t.Fatalf("backend open_issue_count=%d; want 2", backend.OpenIssueCount)
	}
}
