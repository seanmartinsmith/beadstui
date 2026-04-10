package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestRobotSearchHybridMode(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	writeBeads(t, env, `{"id":"AUTH-1","title":"Authentication bug","description":"auth failure login oauth","status":"open","priority":0,"issue_type":"bug"}
{"id":"AUTH-2","title":"Authentication typo","description":"auth typo in error text","status":"closed","priority":4,"issue_type":"task"}
{"id":"DEP-1","title":"Depends on AUTH-1","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"DEP-1","depends_on_id":"AUTH-1","type":"blocks"}]}
{"id":"DEP-2","title":"Depends on AUTH-1","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"DEP-2","depends_on_id":"AUTH-1","type":"blocks"}]}`)

	cmd := exec.Command(bv,
		"--search", "authentication",
		"--search-mode", "hybrid",
		"--search-preset", "default",
		"--robot-search",
	)
	cmd.Dir = env
	cmd.Env = append(os.Environ(),
		"BV_SEMANTIC_EMBEDDER=hash",
		"BV_SEMANTIC_DIM=2048",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("robot-search hybrid failed: %v\n%s", err, out)
	}

	var payload struct {
		Mode    string `json:"mode"`
		Preset  string `json:"preset"`
		Weights struct {
			Text     float64 `json:"text"`
			PageRank float64 `json:"pagerank"`
		} `json:"weights"`
		Results []struct {
			IssueID         string             `json:"issue_id"`
			Score           float64            `json:"score"`
			TextScore       float64            `json:"text_score"`
			ComponentScores map[string]float64 `json:"component_scores"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("hybrid json decode: %v\nout=%s", err, out)
	}

	if payload.Mode != "hybrid" {
		t.Fatalf("expected mode hybrid, got %q", payload.Mode)
	}
	if payload.Preset != "default" {
		t.Fatalf("expected preset default, got %q", payload.Preset)
	}
	if payload.Weights.Text == 0 || payload.Weights.PageRank == 0 {
		t.Fatalf("expected non-zero weights in hybrid output")
	}
	if len(payload.Results) == 0 {
		t.Fatalf("expected hybrid results")
	}
	if payload.Results[0].ComponentScores == nil {
		t.Fatalf("expected component scores in hybrid results")
	}
}

func TestRobotSearchHybridPresetAffectsScores(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	writeBeads(t, env, `{"id":"BUG-1","title":"Auth bug","description":"auth failure","status":"open","priority":0,"issue_type":"bug"}
{"id":"BUG-2","title":"Auth bug docs","description":"auth typo docs","status":"open","priority":4,"issue_type":"task"}
{"id":"DEP-1","title":"Depends on BUG-1","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"DEP-1","depends_on_id":"BUG-1","type":"blocks"}]}
{"id":"DEP-2","title":"Depends on BUG-1","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"DEP-2","depends_on_id":"BUG-1","type":"blocks"}]}`)

	run := func(preset string) float64 {
		cmd := exec.Command(bv,
			"--search", "auth",
			"--search-mode", "hybrid",
			"--search-preset", preset,
			"--robot-search",
		)
		cmd.Dir = env
		cmd.Env = append(os.Environ(),
			"BV_SEMANTIC_EMBEDDER=hash",
			"BV_SEMANTIC_DIM=2048",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("robot-search preset %s failed: %v\n%s", preset, err, out)
		}
		var payload struct {
			Results []struct {
				IssueID string  `json:"issue_id"`
				Score   float64 `json:"score"`
			} `json:"results"`
		}
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("decode preset %s: %v\nout=%s", preset, err, out)
		}
		if len(payload.Results) == 0 {
			t.Fatalf("no results for preset %s", preset)
		}
		if payload.Results[0].IssueID != "BUG-1" {
			t.Fatalf("expected BUG-1 to remain top for preset %s, got %s", preset, payload.Results[0].IssueID)
		}
		return payload.Results[0].Score
	}

	defaultScore := run("default")
	bugScore := run("bug-hunting")
	if defaultScore == bugScore {
		t.Fatalf("expected preset scores to differ; got %f vs %f", defaultScore, bugScore)
	}
}

func TestRobotSearchHybridBackwardCompatibility(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()

	writeBeads(t, env, `{"id":"A","title":"Search target","description":"token token token","status":"open","priority":1,"issue_type":"task"}
{"id":"B","title":"Other","description":"noise","status":"open","priority":2,"issue_type":"task"}`)

	cmd := exec.Command(bv, "--search", "token", "--robot-search")
	cmd.Dir = env
	cmd.Env = append(os.Environ(),
		"BV_SEMANTIC_EMBEDDER=hash",
		"BV_SEMANTIC_DIM=2048",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("robot-search failed: %v\n%s", err, out)
	}

	var payload struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("decode backward compat: %v\nout=%s", err, out)
	}
	if payload.Mode != "text" {
		t.Fatalf("expected default mode text, got %q", payload.Mode)
	}
}
