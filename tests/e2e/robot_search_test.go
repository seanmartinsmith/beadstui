package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestRobotSearchContract(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()
	// Use a very distinctive token with many repeats to make hashed-vector ranking stable.
	writeBeads(t, env, `{"id":"A","title":"Semantic search target","description":"interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken interstellarkraken","status":"open","priority":1,"issue_type":"task"}
{"id":"B","title":"Unrelated docs","description":"readme changelog docs","status":"open","priority":2,"issue_type":"task"}`)

	cmd := exec.Command(bv, "--search", "interstellarkraken", "--robot-search")
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
		GeneratedAt string `json:"generated_at"`
		DataHash    string `json:"data_hash"`
		Query       string `json:"query"`
		Provider    string `json:"provider"`
		Dim         int    `json:"dim"`
		IndexPath   string `json:"index_path"`
		Loaded      bool   `json:"loaded"`
		Limit       int    `json:"limit"`
		Index       struct {
			Total    int `json:"total"`
			Added    int `json:"added"`
			Updated  int `json:"updated"`
			Removed  int `json:"removed"`
			Skipped  int `json:"skipped"`
			Embedded int `json:"embedded"`
		} `json:"index"`
		Results []struct {
			IssueID string  `json:"issue_id"`
			Score   float64 `json:"score"`
			Title   string  `json:"title"`
		} `json:"results"`
		UsageHints []string `json:"usage_hints"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("robot-search json decode: %v\nout=%s", err, out)
	}

	if payload.GeneratedAt == "" || payload.DataHash == "" {
		t.Fatalf("robot-search missing metadata: generated_at=%q data_hash=%q", payload.GeneratedAt, payload.DataHash)
	}
	if payload.Query != "interstellarkraken" {
		t.Fatalf("unexpected query: %q", payload.Query)
	}
	if payload.Provider != "hash" {
		t.Fatalf("unexpected provider: %q", payload.Provider)
	}
	if payload.Dim != 2048 {
		t.Fatalf("unexpected dim: %d", payload.Dim)
	}
	if payload.IndexPath == "" {
		t.Fatalf("missing index_path")
	}
	if payload.Limit <= 0 {
		t.Fatalf("missing/invalid limit: %d", payload.Limit)
	}
	if len(payload.Results) == 0 {
		t.Fatalf("expected at least one result")
	}
	if payload.Results[0].IssueID != "A" {
		t.Fatalf("expected top match A, got %s (%+v)", payload.Results[0].IssueID, payload.Results)
	}
	if len(payload.UsageHints) == 0 {
		t.Fatalf("expected usage_hints")
	}
}
