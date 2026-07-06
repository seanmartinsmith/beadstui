package ui

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/model"
)

// wheelTestModel builds a single-pane list model with n open issues, sized and
// focused on the list, for the mouse-wheel ramp tests (bt-citoc).
func wheelTestModel(n int) Model {
	issues := make([]model.Issue, 0, n)
	for i := 0; i < n; i++ {
		issues = append(issues, model.Issue{
			ID:     fmt.Sprintf("bd-%04d", i),
			Title:  "row",
			Status: model.StatusOpen,
		})
	}
	m := NewModel(issues, nil, "", nil, nil)
	m.width = 80
	m.height = 30
	m.mode = ViewList
	m.isSplitView = false
	m.list.SetSize(m.bodyWidth(), 24)
	m.focused = focusList
	return m
}

// TestRampedWheelStep_SingleTickMovesOneRow: an isolated tick (no prior wheel
// state) advances exactly one row, preserving precise control (bt-citoc).
func TestRampedWheelStep_SingleTickMovesOneRow(t *testing.T) {
	m := wheelTestModel(50)
	_, step := m.rampedWheelStep(+1)
	if step != 1 {
		t.Fatalf("isolated tick should move 1 row, got %d", step)
	}
}

// TestRampedWheelStep_RampFormula verifies the rows-per-tick curve for a known
// seeded streak, plus the cap (bt-citoc). State is seeded directly so the
// assertion is independent of wall-clock timing between calls.
func TestRampedWheelStep_RampFormula(t *testing.T) {
	// step = 1 + (seedStreak+1)/wheelRampDivisor, capped at wheelStepMax.
	cases := []struct{ seed, want int }{
		{0, 1},
		{1, 2},
		{3, 3},
		{17, wheelStepMax},
		{100, wheelStepMax},
	}
	for _, c := range cases {
		m := wheelTestModel(5)
		m.lastWheelDir = +1
		m.lastWheelAt = time.Now()
		m.wheelStreak = c.seed
		_, step := m.rampedWheelStep(+1)
		if step != c.want {
			t.Fatalf("seedStreak=%d: step=%d, want %d", c.seed, step, c.want)
		}
	}
}

// TestRampedWheelStep_DirectionChangeResets: reversing direction resets the
// streak so the next tick moves a single row (bt-citoc).
func TestRampedWheelStep_DirectionChangeResets(t *testing.T) {
	m := wheelTestModel(5)
	m.lastWheelDir = +1
	m.lastWheelAt = time.Now()
	m.wheelStreak = 8 // mid-ramp downward
	_, step := m.rampedWheelStep(-1)
	if step != 1 {
		t.Fatalf("direction change should reset to 1 row, got %d", step)
	}
}

// TestRampedWheelStep_ExpiredWindowResets: a pause longer than the ramp window
// resets to a single row (bt-citoc).
func TestRampedWheelStep_ExpiredWindowResets(t *testing.T) {
	m := wheelTestModel(5)
	m.lastWheelDir = +1
	m.lastWheelAt = time.Now().Add(-2 * wheelRampWindow)
	m.wheelStreak = 8
	_, step := m.rampedWheelStep(+1)
	if step != 1 {
		t.Fatalf("expired window should reset to 1 row, got %d", step)
	}
}

// TestHandleMouseWheel_ListRampAdvancesMultipleRows: a wheel tick mid-ramp
// advances the list selection by the ramped step, not just one row (bt-citoc).
func TestHandleMouseWheel_ListRampAdvancesMultipleRows(t *testing.T) {
	m := wheelTestModel(100)
	m.list.Select(0)
	// Seed mid-ramp so the next down-tick -> streak 7 -> step 1 + 7/2 = 4.
	m.lastWheelDir = +1
	m.lastWheelAt = time.Now()
	m.wheelStreak = 6
	m, _ = m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if got := m.list.Index(); got != 4 {
		t.Fatalf("ramped down-tick should advance 4 rows from index 0, got %d", got)
	}
}

// TestHandleMouseWheel_ListSingleTickMovesOneRow: an isolated wheel tick (no
// prior ramp state) advances exactly one row through the real handler
// (bt-citoc).
func TestHandleMouseWheel_ListSingleTickMovesOneRow(t *testing.T) {
	m := wheelTestModel(100)
	m.list.Select(10)
	m, _ = m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if got := m.list.Index(); got != 11 {
		t.Fatalf("isolated down-tick should advance 1 row from index 10, got %d", got)
	}
}

// TestHandleMouseWheel_ListRampClampsAtBottom: the ramped step never selects
// past the last item (bt-citoc).
func TestHandleMouseWheel_ListRampClampsAtBottom(t *testing.T) {
	m := wheelTestModel(20)
	m.list.Select(18)
	m.lastWheelDir = +1
	m.lastWheelAt = time.Now()
	m.wheelStreak = 100 // step capped at wheelStepMax, well past the last index
	m, _ = m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if got := m.list.Index(); got != 19 {
		t.Fatalf("ramped tick at the bottom should clamp to last index 19, got %d", got)
	}
}