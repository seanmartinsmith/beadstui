package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// TestModalChromeAboveItems_MatchesRender renders both modal tabs and asserts
// that the first item lands on the row identified by modalChromeAboveItems
// (plus an optional intra-page row offset for tabs that render a leading
// non-item row inside the item area, e.g. the notifications tab's day
// separator added in bt-l5zk). Without this guard, modalChromeAboveItems
// silently drifts from the renderer and clicks land one row off (bt-46p6.13
// dogfooding). The probe iterates the rendered output and finds the first
// row that contains an item glyph.
func TestModalChromeAboveItems_MatchesRender(t *testing.T) {
	cases := []struct {
		name           string
		buildKey       rune
		setup          func(m Model) Model
		marker         string
		intraPageShift int // rows inserted between chrome and first event
	}{
		{
			name:     "notifications tab",
			buildKey: '1',
			setup: func(m Model) Model {
				m.events.Append(events.Event{ID: "e1", Kind: events.EventCreated, BeadID: "bt-1", Repo: "bt", Title: "one", At: time.Now()})
				return m
			},
			// formatNotificationRow always emits "HH:MM kind id • title".
			// Looking for "bt-1" picks up the first item exactly.
			marker: "bt-1",
			// bt-l5zk: a day separator always precedes the first event of the
			// page. The mouse handler models this as a chrome-equivalent row
			// (returns -1 for clicks on separators), so the click math still
			// uses modalChromeAboveItems as the base — the probe just needs
			// to know the visible event lands one row deeper.
			intraPageShift: 1,
		},
		{
			name:     "alerts tab",
			buildKey: '!',
			setup:   func(m Model) Model { return m },
			// seedModel seeds one stale alert with IssueID="bt-fix"; the alerts
			// renderer prints the message ("fixture") on the cursor row.
			marker:         "fixture",
			intraPageShift: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := seedModel()
			m = tc.setup(m)
			m = pressRune(m, tc.buildKey)

			rows := strings.Split(m.renderAlertsPanel(), "\n")
			firstItemRow := -1
			for i, r := range rows {
				if strings.Contains(r, tc.marker) {
					firstItemRow = i
					break
				}
			}
			if firstItemRow == -1 {
				t.Fatalf("marker %q not found in rendered modal", tc.marker)
			}
			want := modalChromeAboveItems + tc.intraPageShift
			if firstItemRow != want {
				t.Errorf("first item row = %d, want %d (modalChromeAboveItems=%d, intraPageShift=%d) — drift will cause off-by-one clicks",
					firstItemRow, want, modalChromeAboveItems, tc.intraPageShift)
			}
		})
	}
}
