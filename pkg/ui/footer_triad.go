package ui

import (
	"charm.land/bubbles/v2/list"

	"github.com/seanmartinsmith/beadstui/pkg/analysis"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// FooterTriad is the footer's default center-zone content (bt-p8y2f,
// replacing the raw per-status "o/p/c dot" tallies bt-9gjt0/bt-ujwiq
// shipped): an "actionable triad" answering what the current lens (active
// filters) makes possible right now, rather than a status breakdown of what
// exists. It is a mutually-exclusive partition of the NON-CLOSED issues in
// scope:
//
//   - InFlight: status == in_progress, regardless of graph-blocked state.
//     Already being worked is the fact that matters here, not a re-derived
//     actionability call — an in-progress issue is never also Ready.
//   - Ready: not in-flight, not closed, and actionable per the SAME
//     graph-based definition pkg/analysis uses for `bt robot triage`
//     (Analyzer.GetActionableIssues — closed/tombstone excluded, blocking
//     deps resolved against the full corpus, parent-child block
//     propagation applied). This replaces the footer's prior inline
//     reimplementation (classifyItemCounts), which only walked an issue's
//     own direct dependencies and missed transitive parent-child
//     propagation — a real correctness gap relative to `bt robot triage`.
//   - Blocked: not in-flight, not closed, and NOT actionable per the same
//     definition.
//
// Deferred/pinned/hooked/review issues are not special-cased — like
// pkg/analysis, only closed-like status and the dependency graph decide
// Ready vs Blocked for them, so they still land in one of the three
// buckets (never a silent fourth category).
type FooterTriad struct {
	Ready    int
	InFlight int
	Blocked  int
}

// computeFooterTriad partitions issues — whatever slice the caller's lens
// currently shows, from the full corpus down to a heavily filtered subset —
// into the footer triad. analyzer supplies the actionable-issue graph
// computation and should be built over the FULL loaded corpus rather than
// the filtered subset, so a blocker outside the current view still counts
// (the contract the footer's prior classifyItemCounts documented and this
// preserves). A nil analyzer (no data loaded yet) degrades every non-closed,
// non-in-flight issue to Blocked=0/Ready=0 — conservative, but panic-free.
func computeFooterTriad(issues []model.Issue, analyzer *analysis.Analyzer) FooterTriad {
	var t FooterTriad
	if len(issues) == 0 {
		return t
	}

	var actionable map[string]bool
	if analyzer != nil {
		set := analyzer.GetActionableIssues()
		actionable = make(map[string]bool, len(set))
		for _, iss := range set {
			actionable[iss.ID] = true
		}
	}

	for _, issue := range issues {
		if isClosedLikeStatus(issue.Status) {
			continue
		}
		switch {
		case issue.Status == model.StatusInProgress:
			t.InFlight++
		case actionable[issue.ID]:
			t.Ready++
		default:
			t.Blocked++
		}
	}
	return t
}

// lensTriad computes the triad scoped to whatever list items setListItems is
// currently holding — the single chokepoint for "what the list shows" (see
// setListItems doc). Readiness is resolved against m.data.analyzer, which is
// built over the full loaded corpus, not just items, so a blocker outside
// the current filtered view still counts.
func (m *Model) lensTriad(items []list.Item) FooterTriad {
	issues := make([]model.Issue, 0, len(items))
	for _, it := range items {
		if issueItem, ok := it.(IssueItem); ok {
			issues = append(issues, issueItem.Issue)
		}
	}
	return computeFooterTriad(issues, m.data.analyzer)
}
