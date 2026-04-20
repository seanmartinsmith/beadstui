package view

import (
	"encoding/json"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// CompactIssueSchemaV1 identifies the wire shape produced by CompactAll.
// Emitted on the robot output envelope's `schema` field so agents can
// discover what they received.
const CompactIssueSchemaV1 = "compact.v1"

// CompactIssue is the agent-facing index projection of model.Issue. It omits
// fat text fields (description, design, acceptance_criteria, notes,
// comments, close_reason) and replaces dependency edges with scalar counts
// plus a single parent id. Agents scan lists of CompactIssue, then drill
// into specific issues via `bd show <id>` when they need the full body.
//
// See docs/design/2026-04-20-bt-mhwy-1-compact-output.md for the authority
// on field selection and rationale.
type CompactIssue struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Status     model.Status    `json:"status"`
	Priority   int             `json:"priority"`
	IssueType  model.IssueType `json:"issue_type"`
	Labels     []string        `json:"labels,omitempty"`
	Assignee   string          `json:"assignee,omitempty"`
	SourceRepo string          `json:"source_repo,omitempty"`

	ParentID      string `json:"parent_id,omitempty"`
	BlockersCount int    `json:"blockers_count"`
	UnblocksCount int    `json:"unblocks_count"`
	ChildrenCount int    `json:"children_count"`
	RelatesCount  int    `json:"relates_count"`
	IsBlocked     bool   `json:"is_blocked"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DueDate   *time.Time `json:"due_date,omitempty"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`

	CreatedBySession string `json:"created_by_session,omitempty"`
	ClaimedBySession string `json:"claimed_by_session,omitempty"`
	ClosedBySession  string `json:"closed_by_session,omitempty"`
}

// CompactAll produces the compact projection for an entire issue set.
// Reverse-map dependencies (children, unblocks, is_blocked) are precomputed
// in a single O(n) pass before projecting each issue in O(1).
//
// Per-issue compaction is deliberately absent: a method on model.Issue would
// either be wrong (missing reverse data) or silently O(n²). The reverse-map
// dependency is visible in this signature on purpose.
//
// Nil and empty inputs are safe and return nil.
func CompactAll(issues []model.Issue) []CompactIssue {
	if len(issues) == 0 {
		return nil
	}

	// Reverse maps keyed by issue ID (target of an edge).
	childrenCount := make(map[string]int, len(issues))
	unblocksCount := make(map[string]int, len(issues))
	openBlockers := make(map[string]int, len(issues))
	statusByID := make(map[string]model.Status, len(issues))

	for i := range issues {
		statusByID[issues[i].ID] = issues[i].Status
	}

	for i := range issues {
		src := &issues[i]
		for _, dep := range src.Dependencies {
			if dep == nil {
				continue
			}
			switch {
			case dep.Type == model.DepParentChild:
				// Child -> parent edge stored on the child; increment the
				// parent's children counter.
				childrenCount[dep.DependsOnID]++
			case dep.Type.IsBlocking():
				// src depends on dep.DependsOnID => dep.DependsOnID
				// unblocks src when closed.
				unblocksCount[dep.DependsOnID]++
				if targetStatus, ok := statusByID[dep.DependsOnID]; ok {
					if targetStatus == model.StatusOpen || targetStatus == model.StatusInProgress {
						openBlockers[src.ID]++
					}
				} else {
					// Unknown target (e.g., external/cross-project ref):
					// err on the side of blocked=true so agents see the
					// hazard.
					openBlockers[src.ID]++
				}
			}
		}
	}

	out := make([]CompactIssue, 0, len(issues))
	for i := range issues {
		out = append(out, compactOne(&issues[i], childrenCount, unblocksCount, openBlockers))
	}
	return out
}

func compactOne(
	src *model.Issue,
	childrenCount, unblocksCount, openBlockers map[string]int,
) CompactIssue {
	var (
		parentID      string
		blockersCount int
		relatesCount  int
	)
	for _, dep := range src.Dependencies {
		if dep == nil {
			continue
		}
		switch {
		case dep.Type == model.DepParentChild:
			if parentID == "" {
				parentID = dep.DependsOnID
			}
		case dep.Type == model.DepRelated:
			relatesCount++
		case dep.Type.IsBlocking():
			blockersCount++
		}
	}

	return CompactIssue{
		ID:               src.ID,
		Title:            src.Title,
		Status:           src.Status,
		Priority:         src.Priority,
		IssueType:        src.IssueType,
		Labels:           cloneStrings(src.Labels),
		Assignee:         src.Assignee,
		SourceRepo:       src.SourceRepo,
		ParentID:         parentID,
		BlockersCount:    blockersCount,
		UnblocksCount:    unblocksCount[src.ID],
		ChildrenCount:    childrenCount[src.ID],
		RelatesCount:     relatesCount,
		IsBlocked:        openBlockers[src.ID] > 0,
		CreatedAt:        src.CreatedAt,
		UpdatedAt:        src.UpdatedAt,
		DueDate:          copyTime(src.DueDate),
		ClosedAt:         copyTime(src.ClosedAt),
		CreatedBySession: metadataString(src.Metadata, "created_by_session"),
		ClaimedBySession: metadataString(src.Metadata, "claimed_by_session"),
		ClosedBySession:  src.ClosedBySession,
	}
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func copyTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}

func metadataString(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok || len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}
