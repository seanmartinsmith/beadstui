package ui

import (
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// EpicStatusMode selects which epics the overview lists. It reinterprets the
// status filter as "which epics to show" - children are ALWAYS counted in full
// for the progress bar, regardless of this mode (see the epics-view design's
// "status filter override").
type EpicStatusMode int

const (
	// EpicsActive lists epics with >=1 non-closed child (the default).
	EpicsActive EpicStatusMode = iota
	// EpicsAll lists every epic.
	EpicsAll
	// EpicsCompleted lists epics whose children are all closed (and Total>0).
	EpicsCompleted
)

// epicStaleThreshold is how long an in-progress child can go without an update
// before it counts as at-risk.
const epicStaleThreshold = 3 * 24 * time.Hour

// EpicRow is one epic's overview projection: the epic itself plus child counts.
// Children are counted in full (not status-filtered) so the progress bar is
// accurate even when the underlying list is scoped to open issues.
type EpicRow struct {
	Epic                      model.Issue
	Done, Total               int // progress; children ALWAYS counted in full
	InProgress, Blocked, Open int
	AtRisk                    int // children in_progress with >= epicStaleThreshold no update
}

// epicsOverviewRows partitions epics out of `all` (the full issue set, NOT
// status-filtered), computes per-epic counts via parent-child deps, and keeps
// only epics matching statusMode. `all` should already be scope/label filtered.
func epicsOverviewRows(all []model.Issue, statusMode EpicStatusMode, now time.Time) []EpicRow {
	var rows []EpicRow
	for i := range all {
		if all[i].IssueType != model.TypeEpic {
			continue
		}
		row := EpicRow{Epic: all[i]}
		for _, child := range epicChildrenSorted(all[i].ID, all) {
			row.Total++
			switch {
			case isClosedLikeStatus(child.Status):
				row.Done++
			case child.Status == model.StatusInProgress:
				row.InProgress++
			case child.Status == model.StatusBlocked:
				row.Blocked++
			case child.Status == model.StatusOpen:
				row.Open++
			}
			if child.Status == model.StatusInProgress && now.Sub(child.UpdatedAt) >= epicStaleThreshold {
				row.AtRisk++
			}
		}
		switch statusMode {
		case EpicsActive:
			// Keep epics with >=1 non-closed child.
			if row.Done >= row.Total {
				continue
			}
		case EpicsCompleted:
			// Keep epics whose children are all closed (and there is >=1 child).
			if row.Total == 0 || row.Done < row.Total {
				continue
			}
		case EpicsAll:
			// Keep every epic.
		}
		rows = append(rows, row)
	}
	return rows
}
