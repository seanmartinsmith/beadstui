package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// TestModalChromeAboveItems_MatchesRender renders both modal tabs and asserts
// that the first item lands on the row identified by modalChromeAboveItems.
// Without this guard, modalChromeAboveItems silently drifts from the renderer
// and clicks land one row off (bt-46p6.13 dogfooding). The probe iterates the
// rendered output and finds the first row that contains an item glyph.
func TestModalChromeAboveItems_MatchesRender(t *testing.T) {
	cases := []struct {
		name     string
		buildKey rune
		setup    func(m Model) Model
		marker   string
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
		},
		{
			name:     "alerts tab",
			buildKey: '!',
			setup:   func(m Model) Model { return m },
			// seedModel seeds one stale alert with IssueID="bt-fix"; the alerts
			// renderer prints the message ("fixture") on the cursor row.
			marker: "fixture",
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
			if firstItemRow != modalChromeAboveItems {
				t.Errorf("first item row = %d, modalChromeAboveItems = %d — drift will cause off-by-one clicks",
					firstItemRow, modalChromeAboveItems)
			}
		})
	}
}
