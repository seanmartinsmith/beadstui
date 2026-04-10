package search

import "fmt"

// PresetName identifies a named weight configuration.
type PresetName string

const (
	PresetDefault        PresetName = "default"
	PresetBugHunting     PresetName = "bug-hunting"
	PresetSprintPlanning PresetName = "sprint-planning"
	PresetImpactFirst    PresetName = "impact-first"
	PresetTextOnly       PresetName = "text-only"
)

var presets = map[PresetName]Weights{
	PresetDefault: {
		TextRelevance: 0.40,
		PageRank:      0.20,
		Status:        0.15,
		Impact:        0.10,
		Priority:      0.10,
		Recency:       0.05,
	},
	PresetBugHunting: {
		TextRelevance: 0.30,
		PageRank:      0.15,
		Status:        0.15,
		Impact:        0.15,
		Priority:      0.20,
		Recency:       0.05,
	},
	PresetSprintPlanning: {
		TextRelevance: 0.30,
		PageRank:      0.20,
		Status:        0.25,
		Impact:        0.15,
		Priority:      0.05,
		Recency:       0.05,
	},
	PresetImpactFirst: {
		TextRelevance: 0.25,
		PageRank:      0.30,
		Status:        0.10,
		Impact:        0.20,
		Priority:      0.10,
		Recency:       0.05,
	},
	PresetTextOnly: {
		TextRelevance: 1.00,
		PageRank:      0.00,
		Status:        0.00,
		Impact:        0.00,
		Priority:      0.00,
		Recency:       0.00,
	},
}

// GetPreset returns the weights for a named preset.
func GetPreset(name PresetName) (Weights, error) {
	weights, ok := presets[name]
	if !ok {
		return Weights{}, fmt.Errorf("unknown preset %q", name)
	}
	return weights, nil
}

// ListPresets returns all available preset names.
func ListPresets() []PresetName {
	return []PresetName{
		PresetDefault,
		PresetBugHunting,
		PresetSprintPlanning,
		PresetImpactFirst,
		PresetTextOnly,
	}
}
