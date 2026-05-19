package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// TestAlertsModal_DriftDiagnostic_2s3a5 is a NON-ASSERTIVE diagnostic that
// dumps the alerts/notifications modal at a known-broken width with every
// non-ASCII rune annotated by both ansi.StringWidth and runewidth.StringWidth.
// Useful for future modal-border drift investigations. Always skipped in
// automated runs; remove the skip and re-run with `-v` when investigating a
// new dimension-sensitive layout bug.
func TestAlertsModal_DriftDiagnostic_2s3a5(t *testing.T) {
	t.Skip("bt-2s3a5: diagnostic only; remove this skip locally when investigating modal width disagreements")

	m := seedModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 118, Height: 48})
	m = updated.(Model)

	day1 := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	kinds := []events.EventKind{
		events.EventCreated, events.EventEdited, events.EventClosed,
		events.EventCommented, events.EventBulk,
	}
	longTitle := "long descriptive notification title that will overflow and be truncated to fit the modal row width"
	for i := 0; i < 50; i++ {
		date := day1
		if i >= 25 {
			date = day2
		}
		m.events.Append(events.Event{
			ID:     fmt.Sprintf("e%d", i),
			Kind:   kinds[i%len(kinds)],
			BeadID: fmt.Sprintf("bt-%04d", i),
			Repo:   "bt",
			Title:  longTitle,
			At:     date.Add(time.Duration(i) * time.Minute),
		})
	}

	m = pressRune(m, '1')
	m.notificationsCursor = 25

	view := m.View().Content
	rows := strings.Split(view, "\n")

	topRow, bottomRow := -1, -1
	for i, r := range rows {
		stripped := ansi.Strip(r)
		if topRow == -1 && strings.Contains(stripped, "╭─ Notifications") {
			topRow = i
		}
		if topRow != -1 && bottomRow == -1 && i > topRow {
			if strings.Contains(stripped, "╰─") &&
				strings.Contains(stripped, "─╯") &&
				!strings.Contains(stripped, " Notifications") {
				bottomRow = i
			}
		}
	}
	if topRow < 0 || bottomRow < 0 {
		t.Fatalf("band not located: top=%d bottom=%d", topRow, bottomRow)
	}

	t.Logf("modal band rows %d..%d (inclusive)", topRow, bottomRow)
	disagreements := map[rune][2]int{}
	for i := topRow; i <= bottomRow; i++ {
		stripped := ansi.Strip(rows[i])
		ansiW := ansi.StringWidth(stripped)
		rwW := runewidth.StringWidth(stripped)
		t.Logf("row %2d  ansi=%3d  rw=%3d  %q",
			i-topRow, ansiW, rwW, stripped)
		for _, r := range stripped {
			if r > unicode.MaxASCII {
				a := ansi.StringWidth(string(r))
				w := runewidth.StringWidth(string(r))
				if a != w {
					disagreements[r] = [2]int{a, w}
				}
			}
		}
	}
	if len(disagreements) == 0 {
		t.Logf("no ansi-vs-runewidth disagreements among non-ASCII runes in band")
	}
	for r, pair := range disagreements {
		t.Logf("DISAGREEMENT: %q (U+%04X)  ansi=%d  runewidth=%d", string(r), r, pair[0], pair[1])
	}
}
