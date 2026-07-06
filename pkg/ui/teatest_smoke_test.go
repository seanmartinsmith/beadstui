package ui_test

// Tier 2 of the TUI dev SOP: teatest drives the REAL bt Model through the
// actual Bubble Tea event loop — Init, Update, async cmds, the deferred
// resize-settle render — none of which the static render harness exercises
// (it sets fields directly). Use this layer to lock interaction sequences
// (open detail -> scroll -> filter -> resize) and catch async regressions.
//
// This is a smoke test: it proves the plumbing works end to end. Richer
// golden-frame tests build on the same NewTestModel scaffold.

import (
	"bytes"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/seanmartinsmith/beadstui/pkg/ui"
)

func TestTeatestSmoke(t *testing.T) {
	issues := createTestIssues(8)
	m := ui.NewModel(issues, nil, "", nil, nil)

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 32))

	// Init + first render happened in the real program loop; wait for the list
	// column header (TYPE) plus an actual data row (test-* IDs) to prove the
	// issues pane rendered with data. Don't key on "TITLE": at 120 wide the
	// model is in split view and the list header clips before that column.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("TYPE")) && bytes.Contains(b, []byte("test-"))
	}, teatest.WithDuration(8*time.Second))

	// Drive a real interaction through the loop (open/focus detail). We assert
	// only that the loop keeps running and shuts down cleanly — content
	// assertions belong in dedicated golden tests, not this smoke check.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// teatest's own Quit terminates the program loop deterministically. Generous
	// final timeout absorbs cold-start teardown variance.
	tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(15*time.Second))
}
