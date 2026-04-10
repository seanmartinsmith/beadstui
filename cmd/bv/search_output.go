package main

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/Dicklesworthstone/beads_viewer/pkg/search"
)

type robotSearchResult struct {
	IssueID         string             `json:"issue_id"`
	Score           float64            `json:"score"`
	TextScore       float64            `json:"text_score,omitempty"`
	Title           string             `json:"title,omitempty"`
	ComponentScores map[string]float64 `json:"component_scores,omitempty"`
}

type robotSearchOutput struct {
	GeneratedAt string                `json:"generated_at"`
	DataHash    string                `json:"data_hash"`
	Query       string                `json:"query"`
	Provider    search.Provider       `json:"provider"`
	Model       string                `json:"model,omitempty"`
	Dim         int                   `json:"dim"`
	IndexPath   string                `json:"index_path"`
	Index       search.IndexSyncStats `json:"index"`
	Loaded      bool                  `json:"loaded"`
	Limit       int                   `json:"limit"`
	Mode        search.SearchMode     `json:"mode"`
	Preset      search.PresetName     `json:"preset,omitempty"`
	Weights     *search.Weights       `json:"weights,omitempty"`
	Results     []robotSearchResult   `json:"results"`
	UsageHints  []string              `json:"usage_hints,omitempty"`
}

func writeRobotSearchOutput(w io.Writer, out robotSearchOutput) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func applySearchConfigOverrides(cfg search.SearchConfig, modeFlag, presetFlag, weightsFlag string) (search.SearchConfig, error) {
	if modeFlag != "" {
		switch search.SearchMode(strings.ToLower(modeFlag)) {
		case search.SearchModeText, search.SearchModeHybrid:
			cfg.Mode = search.SearchMode(strings.ToLower(modeFlag))
		default:
			return search.SearchConfig{}, fmt.Errorf("invalid --search-mode: %q (expected text|hybrid)", modeFlag)
		}
	}

	if presetFlag != "" {
		name := search.PresetName(strings.ToLower(presetFlag))
		if _, err := search.GetPreset(name); err != nil {
			return search.SearchConfig{}, err
		}
		cfg.Preset = name
	}

	if weightsFlag != "" {
		weights, err := search.ParseWeightsJSON(weightsFlag)
		if err != nil {
			return search.SearchConfig{}, err
		}
		cfg.Weights = weights
		cfg.HasWeights = true
	}

	return cfg, nil
}

func resolveSearchWeights(cfg search.SearchConfig) (search.Weights, search.PresetName, error) {
	if cfg.HasWeights {
		return cfg.Weights, search.PresetName("custom"), nil
	}

	weights, err := search.GetPreset(cfg.Preset)
	if err != nil {
		return search.Weights{}, "", err
	}
	return weights, cfg.Preset, nil
}

func buildHybridScores(results []search.SearchResult, scorer search.HybridScorer) ([]search.HybridScore, error) {
	out := make([]search.HybridScore, 0, len(results))
	for _, result := range results {
		scored, err := scorer.Score(result.IssueID, result.Score)
		if err != nil {
			return nil, err
		}
		out = append(out, scored)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].FinalScore == out[j].FinalScore {
			return out[i].IssueID < out[j].IssueID
		}
		return out[i].FinalScore > out[j].FinalScore
	})

	return out, nil
}

var issueIDPattern = regexp.MustCompile(`^[A-Za-z]+-[A-Za-z0-9]+$`)

func isLikelyIssueID(query string) bool {
	return issueIDPattern.MatchString(strings.TrimSpace(query))
}

func promoteExactSearchResult(query string, results []search.SearchResult) []search.SearchResult {
	needle := strings.TrimSpace(query)
	if needle == "" || len(results) == 0 {
		return results
	}
	for i := range results {
		if strings.EqualFold(results[i].IssueID, needle) {
			if i == 0 {
				return results
			}
			match := results[i]
			copy(results[1:i+1], results[0:i])
			results[0] = match
			return results
		}
	}
	return results
}

func promoteExactHybridResult(query string, results []search.HybridScore) []search.HybridScore {
	needle := strings.TrimSpace(query)
	if needle == "" || len(results) == 0 {
		return results
	}
	for i := range results {
		if strings.EqualFold(results[i].IssueID, needle) {
			if i == 0 {
				return results
			}
			match := results[i]
			copy(results[1:i+1], results[0:i])
			results[0] = match
			return results
		}
	}
	return results
}
