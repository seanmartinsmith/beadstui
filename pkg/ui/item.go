package ui

import (
	"fmt"
	"strings"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// DiffStatus represents the diff state of an issue in time-travel mode
type DiffStatus int

const (
	DiffStatusNone     DiffStatus = iota // No diff or not in time-travel mode
	DiffStatusNew                        // Issue was added since comparison point
	DiffStatusClosed                     // Issue was closed since comparison point
	DiffStatusModified                   // Issue was modified since comparison point
)

// DiffBadge returns the badge string for a diff status
func (s DiffStatus) Badge() string {
	switch s {
	case DiffStatusNew:
		return "🆕"
	case DiffStatusClosed:
		return "✅"
	case DiffStatusModified:
		return "~"
	default:
		return ""
	}
}

// IssueItem wraps model.Issue to implement list.Item
type IssueItem struct {
	Issue      model.Issue
	GraphScore float64
	Impact     float64
	DiffStatus DiffStatus // Diff state for time-travel mode
	RepoPrefix string     // Repository prefix for workspace mode (e.g., "api", "web")

	// Semantic/hybrid search scores (set when search is active)
	SearchScore      float64
	SearchTextScore  float64
	SearchComponents map[string]float64
	SearchScoreSet   bool

	// Triage insights (bv-151)
	TriageScore   float64  // Unified triage score (0-1)
	TriageReason  string   // Primary reason for recommendation
	TriageReasons []string // All triage reasons
	IsQuickWin    bool     // True if identified as a quick win
	IsBlocker     bool     // True if this item blocks significant downstream work
	UnblocksCount int      // Number of items this unblocks

	// Epic progress (bt-waeh) - only populated for epic-type issues with children
	EpicDone  int // Number of closed children
	EpicTotal int // Total number of children

	// Gate badge - pre-computed from blockers that are gate-type issues
	GateAwaitType string // await_type from a blocking gate issue (empty = no gate)
}

func (i IssueItem) Title() string {
	return i.Issue.Title
}

func (i IssueItem) Description() string {
	return fmt.Sprintf("%s %s • %s", i.Issue.ID, i.Issue.Status, i.Issue.Assignee)
}

func (i IssueItem) FilterValue() string {
	// ID is emitted first so the id-priority filter wrapper can extract it
	// as the first whitespace-separated token of the target string (bt-i4yn).
	// Putting ID first also nudges fuzzy scoring for ID-shaped queries, since
	// sahilm/fuzzy weights earlier matches higher.
	var sb strings.Builder
	sb.WriteString(i.Issue.ID)
	sb.WriteString(" ")
	sb.WriteString(i.Issue.Title)
	sb.WriteString(" ")
	sb.WriteString(string(i.Issue.Status))
	sb.WriteString(" ")
	sb.WriteString(string(i.Issue.IssueType))

	if i.Issue.Assignee != "" {
		sb.WriteString(" ")
		sb.WriteString(i.Issue.Assignee)
	}

	if len(i.Issue.Labels) > 0 {
		sb.WriteString(" ")
		sb.WriteString(strings.Join(i.Issue.Labels, " "))
	}

	// Include repo prefix for filtering
	if i.RepoPrefix != "" {
		sb.WriteString(" ")
		sb.WriteString(i.RepoPrefix)
	}

	return sb.String()
}

// IssueRepoKey returns the normalized repo key for filtering. Thin
// wrapper around model.RepoKey, kept here so pkg/ui callers don't need
// to plumb a different import path. See pkg/model/repokey.go for the
// canonical derivation logic and the rationale for SourceRepo-first
// lookup (database name "marketplace" vs ID prefix "mkt-xxx").
func IssueRepoKey(issue model.Issue) string {
	return model.RepoKey(issue)
}

// ExtractRepoPrefix extracts the repository prefix from a namespaced
// issue ID. Thin wrapper around model.ExtractRepoPrefix.
func ExtractRepoPrefix(id string) string {
	return model.ExtractRepoPrefix(id)
}
