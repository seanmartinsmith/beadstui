package view

import (
	"math"
	"sort"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// PortfolioRecordSchemaV1 identifies the wire shape produced by
// ComputePortfolioRecord. Emitted on the robot output envelope's `schema`
// field. Unlike CompactIssue, portfolio has no full-mode alternate — the
// payload is compact-by-construction — so the schema is set unconditionally.
const PortfolioRecordSchemaV1 = "portfolio.v1"

// PortfolioRecord is a per-project health aggregate for agent consumption.
// One record per project, carrying counts, priority breakdown, velocity with
// trend, composite health score, top blocker, and stalest issue. Callers
// sort/filter as needed; the projection itself is flat.
//
// See docs/design/2026-04-20-bt-mhwy-4-portfolio.md for field rationale.
type PortfolioRecord struct {
	Project     string             `json:"project"`
	Counts      PortfolioCounts    `json:"counts"`
	Priority    PortfolioPriority  `json:"priority"`
	Velocity    PortfolioVelocity  `json:"velocity"`
	HealthScore float64            `json:"health_score"`
	TopBlocker  *PortfolioBeadRef  `json:"top_blocker,omitempty"`
	Stalest     *PortfolioStaleRef `json:"stalest,omitempty"`
}

// PortfolioCounts holds the basic issue-status rollups.
type PortfolioCounts struct {
	Open       int `json:"open"`
	Blocked    int `json:"blocked"`
	InProgress int `json:"in_progress"`
	Closed30d  int `json:"closed_30d"`
}

// PortfolioPriority counts open issues by priority band (P0 and P1 only in v1).
type PortfolioPriority struct {
	P0 int `json:"p0"`
	P1 int `json:"p1"`
}

// PortfolioVelocity summarizes throughput with a simple trend classifier.
type PortfolioVelocity struct {
	Closures7d  int    `json:"closures_7d"`
	Closures30d int    `json:"closures_30d"`
	Trend       string `json:"trend"`
	Estimated   bool   `json:"estimated,omitempty"`
}

// PortfolioBeadRef is a lightweight reference to the project's top-blocker.
type PortfolioBeadRef struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Priority int     `json:"priority"`
	Score    float64 `json:"pagerank,omitempty"`
}

// PortfolioStaleRef is a lightweight reference to the project's stalest issue.
type PortfolioStaleRef struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority int    `json:"priority"`
	AgeDays  int    `json:"age_days"`
}

// ComputePortfolioRecord builds one record for a single project's issue slice.
// `projectIssues` is already filtered to this project's SourceRepo. `allIssues`
// is the full set used for cross-project blocker lookup under --global (a bt
// issue can be blocked by an open issue in another project after external dep
// resolution). `pagerank` is the analyzer's PageRank map (nil-safe). `now` is
// injected for deterministic testing.
func ComputePortfolioRecord(
	project string,
	projectIssues, allIssues []model.Issue,
	pagerank map[string]float64,
	now time.Time,
) PortfolioRecord {
	rec := PortfolioRecord{Project: project}
	rec.Counts, rec.Priority = computePortfolioCounts(projectIssues, allIssues)
	rec.Velocity = computePortfolioVelocity(projectIssues, now)
	rec.Counts.Closed30d = rec.Velocity.Closures30d
	rec.Stalest = computeStalest(projectIssues, now)
	rec.TopBlocker = computeTopBlocker(projectIssues, allIssues, pagerank)
	rec.HealthScore = computeHealthScore(rec.Counts, rec.Stalest)
	return rec
}

func computePortfolioCounts(projectIssues, allIssues []model.Issue) (PortfolioCounts, PortfolioPriority) {
	var counts PortfolioCounts
	var prio PortfolioPriority

	blockedSet := buildOpenBlockersMap(allIssues)

	for i := range projectIssues {
		iss := &projectIssues[i]
		switch iss.Status {
		case model.StatusOpen:
			counts.Open++
			if iss.Priority == 0 {
				prio.P0++
			} else if iss.Priority == 1 {
				prio.P1++
			}
			if blockedSet[iss.ID] > 0 {
				counts.Blocked++
			}
		case model.StatusInProgress:
			counts.InProgress++
			if blockedSet[iss.ID] > 0 {
				counts.Blocked++
			}
		}
	}
	return counts, prio
}

// computePortfolioVelocity delegates to analysis.ComputeProjectVelocity for
// the canonical 7d/30d/weekly numbers, then classifies trend from the 8 most
// recent weekly buckets.
func computePortfolioVelocity(projectIssues []model.Issue, now time.Time) PortfolioVelocity {
	v := analysis.ComputeProjectVelocity(projectIssues, now, 8)
	return PortfolioVelocity{
		Closures7d:  v.ClosedLast7Days,
		Closures30d: v.ClosedLast30Days,
		Trend:       classifyTrend(v.Weekly),
		Estimated:   v.Estimated,
	}
}

// classifyTrend compares the most recent 2-week window to the prior 4-week
// window normalized to a 2-week equivalent. The asymmetric windows smooth out
// a single outlier week without the noisy week-over-week comparison.
//
// Weekly is newest-first; entries are analysis.VelocityWeek.
func classifyTrend(weekly []analysis.VelocityWeek) string {
	if len(weekly) < 6 {
		return "flat"
	}
	recent := weekly[0].Closed + weekly[1].Closed
	priorTotal := weekly[2].Closed + weekly[3].Closed + weekly[4].Closed + weekly[5].Closed
	prior := float64(priorTotal) / 2.0
	denom := prior
	if denom < 1 {
		denom = 1
	}
	delta := (float64(recent) - prior) / denom
	switch {
	case prior == 0:
		// No prior activity — report flat rather than divide-by-zero "up".
		return "flat"
	case delta >= 0.20:
		return "up"
	case delta <= -0.20:
		return "down"
	default:
		return "flat"
	}
}

// computeStalest returns the open/in_progress issue with the oldest UpdatedAt.
// Returns nil when no open issues exist. Ties break by ID for determinism.
func computeStalest(projectIssues []model.Issue, now time.Time) *PortfolioStaleRef {
	var stalest *model.Issue
	for i := range projectIssues {
		iss := &projectIssues[i]
		if iss.Status != model.StatusOpen && iss.Status != model.StatusInProgress {
			continue
		}
		if iss.UpdatedAt.IsZero() {
			continue
		}
		if stalest == nil {
			stalest = iss
			continue
		}
		if iss.UpdatedAt.Before(stalest.UpdatedAt) {
			stalest = iss
			continue
		}
		if iss.UpdatedAt.Equal(stalest.UpdatedAt) && iss.ID < stalest.ID {
			stalest = iss
		}
	}
	if stalest == nil {
		return nil
	}
	age := int(now.Sub(stalest.UpdatedAt).Hours() / 24.0)
	if age < 0 {
		age = 0
	}
	return &PortfolioStaleRef{
		ID:       stalest.ID,
		Title:    stalest.Title,
		Priority: stalest.Priority,
		AgeDays:  age,
	}
}

// computeTopBlocker picks the project-scoped open/in_progress issue with the
// highest PageRank among those blocking at least one other issue. Isolated
// leaves (no unblocks) are excluded even when their PageRank is high — they
// aren't holding anyone hostage. Ties break by ID for determinism. Returns
// nil when no candidates exist.
func computeTopBlocker(projectIssues, allIssues []model.Issue, pagerank map[string]float64) *PortfolioBeadRef {
	unblocks := buildUnblocksMap(allIssues)

	type candidate struct {
		iss   *model.Issue
		score float64
	}
	var candidates []candidate
	for i := range projectIssues {
		iss := &projectIssues[i]
		if iss.Status != model.StatusOpen && iss.Status != model.StatusInProgress {
			continue
		}
		if unblocks[iss.ID] == 0 {
			continue
		}
		candidates = append(candidates, candidate{iss: iss, score: pagerank[iss.ID]})
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].iss.ID < candidates[j].iss.ID
	})
	top := candidates[0]
	return &PortfolioBeadRef{
		ID:       top.iss.ID,
		Title:    top.iss.Title,
		Priority: top.iss.Priority,
		Score:    top.score,
	}
}

// computeHealthScore implements the equal-weight mean documented in the
// design doc. Clamps to [0,1] and rounds to 3 decimals so wire output is
// stable across go-json/encoding changes.
func computeHealthScore(counts PortfolioCounts, stalest *PortfolioStaleRef) float64 {
	closureRatio := 1.0
	if denom := counts.Closed30d + counts.Open; denom > 0 {
		closureRatio = float64(counts.Closed30d) / float64(denom)
	}

	blockerRatio := 0.0
	if counts.Open > 0 {
		blockerRatio = float64(counts.Blocked) / float64(counts.Open)
	}

	staleNorm := 0.0
	if stalest != nil {
		age := stalest.AgeDays
		if age > 180 {
			age = 180
		}
		staleNorm = float64(age) / 180.0
	}

	score := (closureRatio + (1.0 - blockerRatio) + (1.0 - staleNorm)) / 3.0
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return math.Round(score*1000) / 1000
}
