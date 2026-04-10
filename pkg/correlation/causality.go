// Package correlation provides temporal causality analysis for beads.
package correlation

import (
	"sort"
	"time"
)

// CausalEventType categorizes events in the causal chain
type CausalEventType string

const (
	// CausalCreated indicates the bead was created
	CausalCreated CausalEventType = "created"
	// CausalClaimed indicates the bead was claimed (status -> in_progress)
	CausalClaimed CausalEventType = "claimed"
	// CausalCommit indicates a code commit related to the bead
	CausalCommit CausalEventType = "commit"
	// CausalBlocked indicates the bead became blocked
	CausalBlocked CausalEventType = "blocked"
	// CausalUnblocked indicates the bead was unblocked
	CausalUnblocked CausalEventType = "unblocked"
	// CausalClosed indicates the bead was closed
	CausalClosed CausalEventType = "closed"
	// CausalReopened indicates the bead was reopened
	CausalReopened CausalEventType = "reopened"
)

// CausalEvent represents a single event in the causal chain
type CausalEvent struct {
	ID           int             `json:"id"`                      // Unique within chain
	Type         CausalEventType `json:"type"`                    // Event type
	Timestamp    time.Time       `json:"timestamp"`               // When it happened
	Description  string          `json:"description"`             // Human-readable description
	CommitSHA    string          `json:"commit_sha,omitempty"`    // For commit events
	BlockerID    string          `json:"blocker_id,omitempty"`    // For blocked/unblocked events
	CausedByID   *int            `json:"caused_by_id,omitempty"`  // ID of event that caused this
	EnablesIDs   []int           `json:"enables_ids,omitempty"`   // IDs of events this enables
	DurationNext *time.Duration  `json:"duration_next,omitempty"` // Time until next event
}

// CausalChain represents the full causal flow for a bead
type CausalChain struct {
	BeadID     string        `json:"bead_id"`
	Title      string        `json:"title"`
	Status     string        `json:"status"`
	Events     []CausalEvent `json:"events"`      // All events in chronological order
	EdgeCount  int           `json:"edge_count"`  // Number of causal links
	StartTime  time.Time     `json:"start_time"`  // First event time
	EndTime    time.Time     `json:"end_time"`    // Last event time (or now if open)
	TotalTime  time.Duration `json:"total_time"`  // Total elapsed time
	IsComplete bool          `json:"is_complete"` // True if bead is closed
}

// BlockedPeriod represents a contiguous period when the bead was blocked
type BlockedPeriod struct {
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	BlockerID string        `json:"blocker_id,omitempty"` // What blocked it
}

// CausalInsights contains derived analysis from the causal chain
type CausalInsights struct {
	TotalDuration     time.Duration   `json:"total_duration"`     // Total time from create to close/now
	BlockedDuration   time.Duration   `json:"blocked_duration"`   // Total time spent blocked
	ActiveDuration    time.Duration   `json:"active_duration"`    // Time not blocked
	BlockedPercentage float64         `json:"blocked_percentage"` // % of time blocked
	BlockedPeriods    []BlockedPeriod `json:"blocked_periods"`    // Each blocked period
	CriticalPath      []int           `json:"critical_path"`      // Event IDs on critical path
	CriticalPathDesc  string          `json:"critical_path_desc"` // Human-readable critical path
	CommitCount       int             `json:"commit_count"`       // Number of commits
	AvgTimeBetween    *time.Duration  `json:"avg_time_between"`   // Avg time between events
	LongestGap        *time.Duration  `json:"longest_gap"`        // Longest gap between events
	LongestGapDesc    string          `json:"longest_gap_desc"`   // Description of longest gap
	EstimatedWithout  *time.Duration  `json:"estimated_without"`  // Est. time without blocks
	Summary           string          `json:"summary"`            // One-line summary
	Recommendations   []string        `json:"recommendations"`    // Actionable insights
}

// CausalityResult is the top-level output for --robot-causality
type CausalityResult struct {
	GeneratedAt time.Time       `json:"generated_at"`
	DataHash    string          `json:"data_hash"`
	Chain       *CausalChain    `json:"chain"`
	Insights    *CausalInsights `json:"insights"`
}

// CausalityOptions configures causality analysis
type CausalityOptions struct {
	IncludeCommits bool              // Include commit events in chain (default true)
	BlockerTitles  map[string]string // BeadID -> Title for blocker descriptions
}

// DefaultCausalityOptions returns sensible defaults
func DefaultCausalityOptions() CausalityOptions {
	return CausalityOptions{
		IncludeCommits: true,
	}
}

// BuildCausalityChain constructs the causal chain for a bead
func (hr *HistoryReport) BuildCausalityChain(beadID string, opts CausalityOptions) *CausalityResult {
	history, exists := hr.Histories[beadID]
	if !exists {
		return nil
	}

	chain := &CausalChain{
		BeadID: beadID,
		Title:  history.Title,
		Status: history.Status,
		Events: []CausalEvent{},
	}

	// Collect all events with their timestamps
	type rawEvent struct {
		timestamp   time.Time
		eventType   CausalEventType
		description string
		commitSHA   string
		blockerID   string
	}
	var rawEvents []rawEvent

	// Add lifecycle events
	for _, event := range history.Events {
		var causalType CausalEventType
		var desc string

		switch event.EventType {
		case EventCreated:
			causalType = CausalCreated
			desc = "Bead created"
		case EventClaimed:
			causalType = CausalClaimed
			desc = "Work started (claimed)"
		case EventClosed:
			causalType = CausalClosed
			desc = "Work completed (closed)"
		case EventReopened:
			causalType = CausalReopened
			desc = "Bead reopened"
		default:
			continue // Skip modified events for now
		}

		rawEvents = append(rawEvents, rawEvent{
			timestamp:   event.Timestamp,
			eventType:   causalType,
			description: desc,
		})
	}

	// Add commit events if requested
	if opts.IncludeCommits {
		for _, commit := range history.Commits {
			desc := commit.Message
			// Truncate by runes (not bytes) to avoid splitting multi-byte characters
			runes := []rune(desc)
			if len(runes) > 50 {
				desc = string(runes[:47]) + "..."
			}
			rawEvents = append(rawEvents, rawEvent{
				timestamp:   commit.Timestamp,
				eventType:   CausalCommit,
				description: "Commit: " + desc,
				commitSHA:   commit.ShortSHA,
			})
		}
	}

	// Sort by timestamp
	sort.Slice(rawEvents, func(i, j int) bool {
		return rawEvents[i].timestamp.Before(rawEvents[j].timestamp)
	})

	// Convert to CausalEvents with IDs and link causality
	var prevEventID *int
	for i, raw := range rawEvents {
		event := CausalEvent{
			ID:          i,
			Type:        raw.eventType,
			Timestamp:   raw.timestamp,
			Description: raw.description,
			CommitSHA:   raw.commitSHA,
			BlockerID:   raw.blockerID,
		}

		// Link to previous event (simple linear causality for now)
		if prevEventID != nil {
			event.CausedByID = prevEventID
			// Update previous event's enables
			if len(chain.Events) > 0 {
				chain.Events[*prevEventID].EnablesIDs = append(
					chain.Events[*prevEventID].EnablesIDs, i)
			}
		}

		// Calculate duration to next event
		if i > 0 && len(chain.Events) > 0 {
			dur := raw.timestamp.Sub(chain.Events[i-1].Timestamp)
			chain.Events[i-1].DurationNext = &dur
		}

		chain.Events = append(chain.Events, event)
		id := i
		prevEventID = &id
	}

	// Set chain metadata
	if len(chain.Events) > 0 {
		chain.StartTime = chain.Events[0].Timestamp
		chain.EndTime = chain.Events[len(chain.Events)-1].Timestamp
		chain.IsComplete = history.Status == "closed"
		if !chain.IsComplete {
			chain.EndTime = time.Now()
		}
		chain.TotalTime = chain.EndTime.Sub(chain.StartTime)
	}

	// Count edges
	for _, event := range chain.Events {
		chain.EdgeCount += len(event.EnablesIDs)
	}

	// Build insights
	insights := buildInsights(chain, history)

	return &CausalityResult{
		GeneratedAt: time.Now(),
		DataHash:    hr.DataHash,
		Chain:       chain,
		Insights:    insights,
	}
}

// buildInsights derives analytical insights from the causal chain
func buildInsights(chain *CausalChain, history BeadHistory) *CausalInsights {
	insights := &CausalInsights{
		TotalDuration:   chain.TotalTime,
		BlockedPeriods:  []BlockedPeriod{},
		CriticalPath:    []int{},
		Recommendations: []string{},
	}

	// Count commits
	for _, event := range chain.Events {
		if event.Type == CausalCommit {
			insights.CommitCount++
		}
	}

	// Find blocked periods by looking at status changes
	// Note: This is a simplified approach - actual blocked state would need
	// to be inferred from dependency resolution or explicit status
	var inBlockedState bool
	var blockedStart time.Time
	var currentBlocker string

	for _, event := range chain.Events {
		if event.Type == CausalBlocked {
			inBlockedState = true
			blockedStart = event.Timestamp
			currentBlocker = event.BlockerID
		} else if event.Type == CausalUnblocked && inBlockedState {
			period := BlockedPeriod{
				StartTime: blockedStart,
				EndTime:   event.Timestamp,
				Duration:  event.Timestamp.Sub(blockedStart),
				BlockerID: currentBlocker,
			}
			insights.BlockedPeriods = append(insights.BlockedPeriods, period)
			insights.BlockedDuration += period.Duration
			inBlockedState = false
		}
	}

	// Calculate active duration and blocked percentage
	insights.ActiveDuration = insights.TotalDuration - insights.BlockedDuration
	if insights.TotalDuration > 0 {
		insights.BlockedPercentage = float64(insights.BlockedDuration) / float64(insights.TotalDuration) * 100
	}

	// Build critical path (for now, it's the full linear path)
	for _, event := range chain.Events {
		insights.CriticalPath = append(insights.CriticalPath, event.ID)
	}

	// Build critical path description
	if len(chain.Events) > 0 {
		var pathParts []string
		for _, event := range chain.Events {
			pathParts = append(pathParts, string(event.Type))
		}
		if len(pathParts) > 5 {
			insights.CriticalPathDesc = pathParts[0] + " → ... → " + pathParts[len(pathParts)-1]
		} else {
			desc := ""
			for i, p := range pathParts {
				if i > 0 {
					desc += " → "
				}
				desc += p
			}
			insights.CriticalPathDesc = desc
		}
	}

	// Calculate average time between events and find longest gap
	if len(chain.Events) > 1 {
		var totalGap time.Duration
		var longestGap time.Duration
		longestGapIdx := 1 // Initialize to 1 (first valid gap index), not 0

		for i := 1; i < len(chain.Events); i++ {
			gap := chain.Events[i].Timestamp.Sub(chain.Events[i-1].Timestamp)
			totalGap += gap
			if gap > longestGap {
				longestGap = gap
				longestGapIdx = i
			}
		}

		avgGap := totalGap / time.Duration(len(chain.Events)-1)
		insights.AvgTimeBetween = &avgGap
		insights.LongestGap = &longestGap
		// longestGapIdx is always >= 1 since we initialize to 1 and only update with i >= 1
		insights.LongestGapDesc = formatGapDescription(chain.Events[longestGapIdx-1], chain.Events[longestGapIdx], longestGap)
	}

	// Estimate time without blocks
	if insights.BlockedDuration > 0 {
		estimated := insights.ActiveDuration
		insights.EstimatedWithout = &estimated
	}

	// Build summary
	insights.Summary = buildSummary(chain, insights)

	// Generate recommendations
	insights.Recommendations = generateRecommendations(chain, insights)

	return insights
}

// formatGapDescription creates a human-readable description of a gap
func formatGapDescription(from, to CausalEvent, gap time.Duration) string {
	return formatDurationShort(gap) + " between " + string(from.Type) + " and " + string(to.Type)
}

// buildSummary creates a one-line summary of the bead's causal history
func buildSummary(chain *CausalChain, insights *CausalInsights) string {
	if !chain.IsComplete {
		if insights.BlockedPercentage > 50 {
			return "In progress, mostly blocked (" + formatDurationShort(insights.TotalDuration) + " total, " +
				formatPercent(insights.BlockedPercentage) + " blocked)"
		}
		return "In progress for " + formatDurationShort(insights.TotalDuration) +
			" with " + formatInt(insights.CommitCount) + " commits"
	}

	if insights.BlockedPercentage > 30 {
		return "Completed in " + formatDurationShort(insights.TotalDuration) +
			" (" + formatPercent(insights.BlockedPercentage) + " blocked)"
	}

	return "Completed in " + formatDurationShort(insights.TotalDuration) +
		" with " + formatInt(insights.CommitCount) + " commits"
}

// generateRecommendations creates actionable insights
func generateRecommendations(chain *CausalChain, insights *CausalInsights) []string {
	var recs []string

	// High blocked percentage
	if insights.BlockedPercentage > 50 {
		recs = append(recs, "High blocked percentage ("+formatPercent(insights.BlockedPercentage)+
			") - consider addressing blockers earlier in the process")
	}

	// Long gaps
	if insights.LongestGap != nil && *insights.LongestGap > 7*24*time.Hour {
		recs = append(recs, "Longest gap of "+formatDurationShort(*insights.LongestGap)+
			" - consider breaking work into smaller pieces")
	}

	// Few commits for long duration
	if insights.TotalDuration > 7*24*time.Hour && insights.CommitCount < 3 {
		recs = append(recs, "Few commits over "+formatDurationShort(insights.TotalDuration)+
			" - consider more frequent incremental commits")
	}

	// Still in progress for a long time
	if !chain.IsComplete && insights.TotalDuration > 14*24*time.Hour {
		recs = append(recs, "Open for "+formatDurationShort(insights.TotalDuration)+
			" - consider breaking into subtasks or closing if complete")
	}

	if len(recs) == 0 {
		recs = append(recs, "No significant issues detected in the causal flow")
	}

	return recs
}

// Helper functions

func formatDurationShort(d time.Duration) string {
	if d < time.Hour {
		return formatInt(int(d.Minutes())) + "m"
	}
	if d < 24*time.Hour {
		return formatInt(int(d.Hours())) + "h"
	}
	days := int(d.Hours() / 24)
	if days < 7 {
		return formatInt(days) + "d"
	}
	weeks := days / 7
	if weeks < 4 {
		return formatInt(weeks) + "w"
	}
	months := days / 30
	return formatInt(months) + "mo"
}

func formatPercent(p float64) string {
	return formatInt(int(p)) + "%"
}

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if neg {
		result = "-" + result
	}
	return result
}

// Note: appendUnique and normalizePath are defined in other files in this package
