package main_test

import "testing"

func TestRobotSuggestContract(t *testing.T) {
	bv := buildBvBinary(t)
	env := t.TempDir()
	// Two similar issues to exercise suggestion pipeline (duplicates/labels may or may not trigger).
	writeBeads(t, env, `{"id":"A","title":"Login OAuth bug","status":"open","priority":1,"issue_type":"task","description":"OAuth login fails with 500 in auth handler"}
{"id":"B","title":"OAuth login failure","status":"open","priority":2,"issue_type":"task","description":"Login via OAuth returns error; auth flow seems broken"}`)

	var first struct {
		GeneratedAt string `json:"generated_at"`
		DataHash    string `json:"data_hash"`
		Suggestions struct {
			Suggestions []struct {
				Type       string  `json:"type"`
				TargetBead string  `json:"target_bead"`
				Confidence float64 `json:"confidence"`
			} `json:"suggestions"`
			Stats struct {
				Total int `json:"total"`
			} `json:"stats"`
		} `json:"suggestions"`
		UsageHints []string `json:"usage_hints"`
	}
	runRobotJSON(t, bv, env, "--robot-suggest", &first)

	if first.GeneratedAt == "" {
		t.Fatalf("suggest missing generated_at")
	}
	if first.DataHash == "" {
		t.Fatalf("suggest missing data_hash")
	}
	if len(first.UsageHints) == 0 {
		t.Fatalf("suggest missing usage_hints")
	}
	if first.Suggestions.Stats.Total != len(first.Suggestions.Suggestions) {
		t.Fatalf("suggest stats.total mismatch: %d vs %d", first.Suggestions.Stats.Total, len(first.Suggestions.Suggestions))
	}

	// Determinism: second call should share the same data_hash
	var second struct {
		DataHash string `json:"data_hash"`
	}
	runRobotJSON(t, bv, env, "--robot-suggest", &second)
	if first.DataHash != second.DataHash {
		t.Fatalf("suggest data_hash changed between calls: %v vs %v", first.DataHash, second.DataHash)
	}
}
