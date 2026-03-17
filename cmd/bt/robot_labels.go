package main

import (
	"fmt"
	"os"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
)

// runLabelHealth handles --robot-label-health.
func (rc *robotCtx) runLabelHealth() {
	cfg := analysis.DefaultLabelHealthConfig()
	results := analysis.ComputeAllLabelHealth(rc.issues, cfg, time.Now().UTC(), nil)

	output := struct {
		GeneratedAt    string                       `json:"generated_at"`
		DataHash       string                       `json:"data_hash"`
		AnalysisConfig analysis.LabelHealthConfig   `json:"analysis_config"`
		Results        analysis.LabelAnalysisResult `json:"results"`
		UsageHints     []string                     `json:"usage_hints"`
	}{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		DataHash:       rc.dataHash,
		AnalysisConfig: cfg,
		Results:        results,
		UsageHints: []string{
			"jq '.results.summaries | sort_by(.health) | .[:3]' - Critical labels",
			"jq '.results.labels[] | select(.health_level == \"critical\")' - Critical details",
			"jq '.results.cross_label_flow.bottleneck_labels' - Bottleneck labels",
			"jq '.results.attention_needed' - Labels needing attention",
		},
	}
	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding label health: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runLabelFlow handles --robot-label-flow.
func (rc *robotCtx) runLabelFlow() {
	cfg := analysis.DefaultLabelHealthConfig()
	flow := analysis.ComputeCrossLabelFlow(rc.issues, cfg)
	output := struct {
		GeneratedAt string                     `json:"generated_at"`
		DataHash    string                     `json:"data_hash"`
		Flow        analysis.CrossLabelFlow    `json:"flow"`
		Config      analysis.LabelHealthConfig `json:"analysis_config"`
		UsageHints  []string                   `json:"usage_hints"`
	}{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		DataHash:    rc.dataHash,
		Flow:        flow,
		Config:      cfg,
		UsageHints: []string{
			"jq '.flow.bottleneck_labels' - labels blocking the most others",
			"jq '.flow.dependencies[] | select(.issue_count > 0) | {from:.from_label,to:.to_label,count:.issue_count}'",
			"jq '.flow.flow_matrix' - raw matrix (row=from, col=to, align with .flow.labels)",
		},
	}
	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding label flow: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// runLabelAttention handles --robot-label-attention.
func (rc *robotCtx) runLabelAttention(attentionLimit int) {
	cfg := analysis.DefaultLabelHealthConfig()
	result := analysis.ComputeLabelAttentionScores(rc.issues, cfg, time.Now().UTC())

	// Apply limit
	limit := attentionLimit
	if limit <= 0 {
		limit = 5
	}
	if limit > len(result.Labels) {
		limit = len(result.Labels)
	}

	// Build limited output
	type AttentionOutput struct {
		GeneratedAt string `json:"generated_at"`
		DataHash    string `json:"data_hash"`
		Limit       int    `json:"limit"`
		TotalLabels int    `json:"total_labels"`
		Labels      []struct {
			Rank            int     `json:"rank"`
			Label           string  `json:"label"`
			AttentionScore  float64 `json:"attention_score"`
			NormalizedScore float64 `json:"normalized_score"`
			Reason          string  `json:"reason"`
			OpenCount       int     `json:"open_count"`
			BlockedCount    int     `json:"blocked_count"`
			StaleCount      int     `json:"stale_count"`
			PageRankSum     float64 `json:"pagerank_sum"`
			VelocityFactor  float64 `json:"velocity_factor"`
		} `json:"labels"`
		UsageHints []string `json:"usage_hints"`
	}

	output := AttentionOutput{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		DataHash:    rc.dataHash,
		Limit:       limit,
		TotalLabels: result.TotalLabels,
		UsageHints: []string{
			"jq '.labels[0]' - top attention label details",
			"jq '.labels[] | select(.blocked_count > 0)' - labels with blocked issues",
			"jq '.labels[] | {label:.label,score:.attention_score,reason:.reason}'",
		},
	}

	for i := 0; i < limit; i++ {
		score := result.Labels[i]
		// Build human-readable reason
		reason := buildAttentionReason(score)
		output.Labels = append(output.Labels, struct {
			Rank            int     `json:"rank"`
			Label           string  `json:"label"`
			AttentionScore  float64 `json:"attention_score"`
			NormalizedScore float64 `json:"normalized_score"`
			Reason          string  `json:"reason"`
			OpenCount       int     `json:"open_count"`
			BlockedCount    int     `json:"blocked_count"`
			StaleCount      int     `json:"stale_count"`
			PageRankSum     float64 `json:"pagerank_sum"`
			VelocityFactor  float64 `json:"velocity_factor"`
		}{
			Rank:            score.Rank,
			Label:           score.Label,
			AttentionScore:  score.AttentionScore,
			NormalizedScore: score.NormalizedScore,
			Reason:          reason,
			OpenCount:       score.OpenCount,
			BlockedCount:    score.BlockedCount,
			StaleCount:      score.StaleCount,
			PageRankSum:     score.PageRankSum,
			VelocityFactor:  score.VelocityFactor,
		})
	}

	encoder := rc.newEncoder()
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding label attention: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
