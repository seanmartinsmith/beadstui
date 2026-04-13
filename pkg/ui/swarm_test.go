package ui

import (
	"encoding/json"
	"image/color"
	"testing"
)

func TestSwarmValidateResultParsing(t *testing.T) {
	input := `{
		"ready_fronts": [
			{"wave": 0, "issues": ["bt-001", "bt-002"]},
			{"wave": 1, "issues": ["bt-003"]},
			{"wave": 2, "issues": ["bt-004", "bt-005"]}
		],
		"max_parallelism": 3,
		"estimated_sessions": 2
	}`

	var result swarmValidateResult
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(result.ReadyFronts) != 3 {
		t.Errorf("expected 3 fronts, got %d", len(result.ReadyFronts))
	}
	if result.MaxParallelism != 3 {
		t.Errorf("expected max_parallelism 3, got %d", result.MaxParallelism)
	}
	if result.EstimatedSessions != 2 {
		t.Errorf("expected estimated_sessions 2, got %d", result.EstimatedSessions)
	}

	if result.ReadyFronts[0].Wave != 0 {
		t.Errorf("expected wave 0, got %d", result.ReadyFronts[0].Wave)
	}
	if len(result.ReadyFronts[0].Issues) != 2 {
		t.Errorf("expected 2 issues in wave 0, got %d", len(result.ReadyFronts[0].Issues))
	}
}

func TestSwarmWaveMapBuilding(t *testing.T) {
	input := `{
		"ready_fronts": [
			{"wave": 0, "issues": ["bt-001", "bt-002"]},
			{"wave": 1, "issues": ["bt-003"]},
			{"wave": 2, "issues": ["bt-004"]}
		],
		"max_parallelism": 2,
		"estimated_sessions": 3
	}`

	var result swarmValidateResult
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	waves := make(map[string]int)
	for _, front := range result.ReadyFronts {
		for _, id := range front.Issues {
			waves[id] = front.Wave
		}
	}

	tests := []struct {
		id       string
		wantWave int
	}{
		{"bt-001", 0},
		{"bt-002", 0},
		{"bt-003", 1},
		{"bt-004", 2},
	}

	for _, tc := range tests {
		got, ok := waves[tc.id]
		if !ok {
			t.Errorf("issue %s not in wave map", tc.id)
			continue
		}
		if got != tc.wantWave {
			t.Errorf("issue %s: want wave %d, got %d", tc.id, tc.wantWave, got)
		}
	}

	// Issue not in any wave
	if _, ok := waves["bt-999"]; ok {
		t.Error("bt-999 should not be in wave map")
	}
}

func TestSwarmColorForWave(t *testing.T) {
	tests := []struct {
		wave      int
		wantColor color.Color
	}{
		{0, ColorSuccess},
		{1, ColorWarning},
		{2, ColorInfo},
		{5, ColorInfo},
	}

	for _, tc := range tests {
		got := swarmColorForWave(tc.wave)
		if got != tc.wantColor {
			t.Errorf("wave %d: want %v, got %v", tc.wave, tc.wantColor, got)
		}
	}
}

func TestSwarmClearData(t *testing.T) {
	g := &GraphModel{
		swarmWaves:   map[string]int{"bt-001": 0},
		swarmEnabled: true,
		maxParallel:  3,
		estSessions:  2,
	}
	g.clearSwarmData()

	if g.swarmWaves != nil {
		t.Error("swarmWaves should be nil after clear")
	}
	if g.swarmEnabled {
		t.Error("swarmEnabled should be false after clear")
	}
	if g.maxParallel != 0 {
		t.Error("maxParallel should be 0 after clear")
	}
	if g.estSessions != 0 {
		t.Error("estSessions should be 0 after clear")
	}
}
