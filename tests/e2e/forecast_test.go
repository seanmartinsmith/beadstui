package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func createForecastRepo(t *testing.T) (string, time.Time) {
	t.Helper()

	now := time.Now().UTC()
	weekAgo := now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	twoWeeksAgo := now.Add(-14 * 24 * time.Hour).Format(time.RFC3339)

	repoDir := t.TempDir()
	beadsDir := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	// Issues:
	// - OPEN-1 (backend, explicit estimate)
	// - OPEN-2 (frontend, no estimate)
	// - CLOSED-* (backend, recent, provides velocity samples)
	beads := fmt.Sprintf(
		`{"id":"OPEN-1","title":"Fix memory leak","description":"Investigate and fix leak","status":"open","priority":2,"issue_type":"task","estimated_minutes":120,"labels":["backend"]}`+"\n"+
			`{"id":"OPEN-2","title":"Polish UI","description":"Improve spacing","status":"open","priority":3,"issue_type":"task","labels":["frontend"]}`+"\n"+
			`{"id":"CLOSED-1","title":"Backend cleanup","status":"closed","priority":3,"issue_type":"task","estimated_minutes":60,"labels":["backend"],"closed_at":"%s"}`+"\n"+
			`{"id":"CLOSED-2","title":"Refactor API","status":"closed","priority":3,"issue_type":"task","estimated_minutes":90,"labels":["backend"],"closed_at":"%s"}`+"\n",
		weekAgo,
		twoWeeksAgo,
	)

	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}

	return repoDir, now
}

func TestRobotForecast_SingleIssueAndAgentsScaling(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir, start := createForecastRepo(t)

	run := func(agents int) map[string]any {
		cmd := exec.Command(bv, "--robot-forecast", "OPEN-1", "--forecast-agents", fmt.Sprintf("%d", agents))
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("--robot-forecast failed: %v\n%s", err, out)
		}
		var payload map[string]any
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("json decode: %v\nout=%s", err, out)
		}
		return payload
	}

	p1 := run(1)
	p2 := run(2)

	getDays := func(p map[string]any) float64 {
		forecasts := p["forecasts"].([]any)
		f0 := forecasts[0].(map[string]any)
		return f0["estimated_days"].(float64)
	}

	d1 := getDays(p1)
	d2 := getDays(p2)
	t.Logf("agents=1 estimated_days=%f, agents=2 estimated_days=%f", d1, d2)

	if d2 >= d1 {
		t.Fatalf("expected agents=2 to reduce estimated_days; d1=%f d2=%f", d1, d2)
	}

	// Validate confidence bounds and ETA ordering.
	forecasts := p1["forecasts"].([]any)
	f0 := forecasts[0].(map[string]any)

	if f0["issue_id"].(string) != "OPEN-1" {
		t.Fatalf("expected issue_id OPEN-1, got %v", f0["issue_id"])
	}

	conf := f0["confidence"].(float64)
	if conf <= 0 || conf > 1 {
		t.Fatalf("confidence out of range: %f", conf)
	}

	eta := mustParseRFC3339(t, f0["eta_date"].(string))
	etaLow := mustParseRFC3339(t, f0["eta_date_low"].(string))
	etaHigh := mustParseRFC3339(t, f0["eta_date_high"].(string))
	if eta.Before(start.Add(-1 * time.Minute)) {
		t.Fatalf("eta_date should be in the future; eta=%v start=%v", eta, start)
	}
	if etaLow.After(eta) || eta.After(etaHigh) {
		t.Fatalf("expected eta_low <= eta <= eta_high; low=%v eta=%v high=%v", etaLow, eta, etaHigh)
	}
}

func TestRobotForecast_AllAndLabelFilter(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir, _ := createForecastRepo(t)

	cmd := exec.Command(bv, "--robot-forecast", "all")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-forecast all failed: %v\n%s", err, out)
	}

	var all struct {
		ForecastCount int `json:"forecast_count"`
		Forecasts     []struct {
			IssueID string `json:"issue_id"`
		} `json:"forecasts"`
		Summary any `json:"summary"`
	}
	if err := json.Unmarshal(out, &all); err != nil {
		t.Fatalf("json decode: %v\nout=%s", err, out)
	}
	if all.ForecastCount != 2 {
		t.Fatalf("expected forecast_count=2, got %d", all.ForecastCount)
	}
	if len(all.Forecasts) != 2 {
		t.Fatalf("expected 2 forecasts, got %d", len(all.Forecasts))
	}
	if all.Summary == nil {
		t.Fatalf("expected summary to be present when forecasting multiple issues")
	}

	// Label filter: only backend issues should remain (OPEN-1).
	cmd = exec.Command(bv, "--robot-forecast", "all", "--forecast-label", "backend")
	cmd.Dir = repoDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-forecast all --forecast-label failed: %v\n%s", err, out)
	}

	var filtered struct {
		ForecastCount int `json:"forecast_count"`
		Forecasts     []struct {
			IssueID string `json:"issue_id"`
		} `json:"forecasts"`
	}
	if err := json.Unmarshal(out, &filtered); err != nil {
		t.Fatalf("json decode: %v\nout=%s", err, out)
	}
	if filtered.ForecastCount != 1 {
		t.Fatalf("expected forecast_count=1, got %d", filtered.ForecastCount)
	}
	if len(filtered.Forecasts) != 1 || filtered.Forecasts[0].IssueID != "OPEN-1" {
		t.Fatalf("unexpected filtered forecasts: %#v", filtered.Forecasts)
	}
}

func mustParseRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse RFC3339 %q: %v", s, err)
	}
	return parsed
}
