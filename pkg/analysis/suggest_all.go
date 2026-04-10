package analysis

import (
	"sort"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// SuggestAllConfig configures the unified suggestion generator
type SuggestAllConfig struct {
	// Duplicates detection config
	Duplicates DuplicateConfig

	// Dependencies suggestion config
	Dependencies DependencySuggestionConfig

	// Labels suggestion config
	Labels LabelSuggestionConfig

	// Cycles warning config
	Cycles CycleWarningConfig

	// EnableDuplicates enables duplicate detection
	EnableDuplicates bool

	// EnableDependencies enables dependency suggestions
	EnableDependencies bool

	// EnableLabels enables label suggestions
	EnableLabels bool

	// EnableCycles enables cycle warnings
	EnableCycles bool

	// MinConfidence filters suggestions below this threshold
	MinConfidence float64

	// MaxSuggestions limits total suggestions
	MaxSuggestions int

	// FilterType only includes this suggestion type (empty = all)
	FilterType SuggestionType

	// FilterBead only includes suggestions for this bead ID
	FilterBead string
}

// DefaultSuggestAllConfig returns sensible defaults with all features enabled
func DefaultSuggestAllConfig() SuggestAllConfig {
	return SuggestAllConfig{
		Duplicates:         DefaultDuplicateConfig(),
		Dependencies:       DefaultDependencySuggestionConfig(),
		Labels:             DefaultLabelSuggestionConfig(),
		Cycles:             DefaultCycleWarningConfig(),
		EnableDuplicates:   true,
		EnableDependencies: true,
		EnableLabels:       true,
		EnableCycles:       true,
		MinConfidence:      0.0,
		MaxSuggestions:     50,
	}
}

// GenerateAllSuggestions runs all suggestion detectors and returns aggregated results
func GenerateAllSuggestions(issues []model.Issue, config SuggestAllConfig, dataHash string) SuggestionSet {
	var allSuggestions []Suggestion

	// Run enabled detectors
	if config.EnableDuplicates && (config.FilterType == "" || config.FilterType == SuggestionPotentialDuplicate) {
		duplicates := DetectDuplicates(issues, config.Duplicates)
		allSuggestions = append(allSuggestions, duplicates...)
	}

	if config.EnableDependencies && (config.FilterType == "" || config.FilterType == SuggestionMissingDependency) {
		dependencies := DetectMissingDependencies(issues, config.Dependencies)
		allSuggestions = append(allSuggestions, dependencies...)
	}

	if config.EnableLabels && (config.FilterType == "" || config.FilterType == SuggestionLabelSuggestion) {
		labels := SuggestLabels(issues, config.Labels)
		allSuggestions = append(allSuggestions, labels...)
	}

	if config.EnableCycles && (config.FilterType == "" || config.FilterType == SuggestionCycleWarning) {
		cycles := DetectCycleWarnings(issues, config.Cycles)
		allSuggestions = append(allSuggestions, cycles...)
	}

	// Apply filters
	filtered := make([]Suggestion, 0, len(allSuggestions))
	for _, sug := range allSuggestions {
		// Min confidence filter
		if config.MinConfidence > 0 && sug.Confidence < config.MinConfidence {
			continue
		}

		// Type filter
		if config.FilterType != "" && sug.Type != config.FilterType {
			continue
		}

		// Bead filter
		if config.FilterBead != "" && sug.TargetBead != config.FilterBead && sug.RelatedBead != config.FilterBead {
			continue
		}

		filtered = append(filtered, sug)
	}

	// Sort by confidence (highest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Confidence > filtered[j].Confidence
	})

	// Apply max limit
	if config.MaxSuggestions > 0 && len(filtered) > config.MaxSuggestions {
		filtered = filtered[:config.MaxSuggestions]
	}

	return NewSuggestionSet(filtered, dataHash)
}

// RobotSuggestOutput is the JSON output structure for --robot-suggest
type RobotSuggestOutput struct {
	GeneratedAt string        `json:"generated_at"`
	DataHash    string        `json:"data_hash"`
	Filters     SuggestFilter `json:"filters"`
	Set         SuggestionSet `json:"suggestions"`
	UsageHints  []string      `json:"usage_hints"`
}

// SuggestFilter describes applied filters
type SuggestFilter struct {
	Type          string  `json:"type,omitempty"`
	MinConfidence float64 `json:"min_confidence,omitempty"`
	BeadID        string  `json:"bead_id,omitempty"`
}

// GenerateRobotSuggestOutput creates the full robot-suggest output
func GenerateRobotSuggestOutput(issues []model.Issue, config SuggestAllConfig, dataHash string) RobotSuggestOutput {
	set := GenerateAllSuggestions(issues, config, dataHash)

	return RobotSuggestOutput{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		DataHash:    dataHash,
		Filters: SuggestFilter{
			Type:          string(config.FilterType),
			MinConfidence: config.MinConfidence,
			BeadID:        config.FilterBead,
		},
		Set: set,
		UsageHints: []string{
			"jq '.suggestions.suggestions[:5]' - Top 5 suggestions by confidence",
			"jq '.suggestions.suggestions[] | select(.type==\"potential_duplicate\")' - Filter duplicates",
			"jq '.suggestions.suggestions[] | select(.confidence >= 0.8)' - High-confidence only",
			"jq '.suggestions.stats.by_type' - Count by suggestion type",
			"jq '.suggestions.suggestions[].action_command' - All action commands",
			"--suggest-type=dependency - Filter to dependency suggestions",
			"--suggest-confidence=0.7 - Minimum confidence threshold",
			"--suggest-bead=<id> - Suggestions for specific bead",
		},
	}
}
