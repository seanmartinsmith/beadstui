package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// sizedFullscreenModel builds a Model sized to w x h, driving the WindowSizeMsg
// that sets isSplitView so the two-stage (wide) vs direct-toggle (narrow)
// branches of toggleFullscreenPane are exercised through the real sizing path.
// Sidebar is off by default, so bodyWidth == width and SplitViewThreshold (100)
// governs directly: 120 -> split, 90 -> narrow.
func sizedFullscreenModel(t *testing.T, w, h int) Model {
	t.Helper()
	m := NewModel(harnessIssues(), nil, "", nil, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return updated.(Model)
}

// TestToggleFullscreenPane_SplitTwoStage covers the wide-view two-stage model
// (bt-566fk): press a number to focus, again to maximize, again to exit back to
// the focused split (Q2).
func TestToggleFullscreenPane_SplitTwoStage(t *testing.T) {
	m := sizedFullscreenModel(t, 120, 32)
	if !m.isSplitView {
		t.Fatalf("expected split view at width 120")
	}
	if m.focused != focusList || m.fullscreen != fullscreenNone {
		t.Fatalf("initial: focused=%d fullscreen=%d, want focusList/None", m.focused, m.fullscreen)
	}

	// Stage 1: details not focused -> focus it, no fullscreen.
	m.toggleFullscreenPane(fullscreenDetails)
	if m.focused != focusDetail || m.fullscreen != fullscreenNone {
		t.Fatalf("stage1: focused=%d fullscreen=%d, want focusDetail/None", m.focused, m.fullscreen)
	}
	// Stage 2: details already focused -> maximize.
	m.toggleFullscreenPane(fullscreenDetails)
	if m.focused != focusDetail || m.fullscreen != fullscreenDetails {
		t.Fatalf("stage2: focused=%d fullscreen=%d, want focusDetail/Details", m.focused, m.fullscreen)
	}
	// Stage 3: re-press exits fullscreen, focus stays on details (Q2).
	m.toggleFullscreenPane(fullscreenDetails)
	if m.focused != focusDetail || m.fullscreen != fullscreenNone {
		t.Fatalf("stage3: focused=%d fullscreen=%d, want focusDetail/None", m.focused, m.fullscreen)
	}
}

// TestToggleFullscreenPane_FocusedPaneMaximizesImmediately: pressing the number
// of the pane you are already focused on skips stage 1 and maximizes it. The
// list is focused by default, so "2" (issues) fullscreens on the first press.
func TestToggleFullscreenPane_FocusedPaneMaximizesImmediately(t *testing.T) {
	m := sizedFullscreenModel(t, 120, 32)
	m.toggleFullscreenPane(fullscreenIssues)
	if m.focused != focusList || m.fullscreen != fullscreenIssues {
		t.Fatalf("focused-pane press: focused=%d fullscreen=%d, want focusList/Issues", m.focused, m.fullscreen)
	}
}

// TestToggleFullscreenPane_OtherKeyOutOfFullscreen covers Q1: while one pane is
// fullscreen, pressing the OTHER number exits fullscreen and focuses that pane
// in the restored split; a second press maximizes it.
func TestToggleFullscreenPane_OtherKeyOutOfFullscreen(t *testing.T) {
	m := sizedFullscreenModel(t, 120, 32)
	// Maximize details (list focused by default -> press details twice).
	m.toggleFullscreenPane(fullscreenDetails)
	m.toggleFullscreenPane(fullscreenDetails)
	if m.fullscreen != fullscreenDetails {
		t.Fatalf("precondition: want details fullscreen, got %d", m.fullscreen)
	}
	// Press issues: exit fullscreen, focus issues in the split (Q1).
	m.toggleFullscreenPane(fullscreenIssues)
	if m.focused != focusList || m.fullscreen != fullscreenNone {
		t.Fatalf("Q1 first press: focused=%d fullscreen=%d, want focusList/None", m.focused, m.fullscreen)
	}
	// Second press of issues maximizes it.
	m.toggleFullscreenPane(fullscreenIssues)
	if m.focused != focusList || m.fullscreen != fullscreenIssues {
		t.Fatalf("Q1 second press: focused=%d fullscreen=%d, want focusList/Issues", m.focused, m.fullscreen)
	}
}

// TestToggleFullscreenPane_NarrowDirect covers Q3: with no split the focus stage
// collapses - the first press maximizes directly, the other key swaps straight
// over, and re-press exits.
func TestToggleFullscreenPane_NarrowDirect(t *testing.T) {
	m := sizedFullscreenModel(t, 90, 24)
	if m.isSplitView {
		t.Fatalf("expected narrow (no split) at width 90")
	}
	// First press maximizes details directly (no intermediate focus stage).
	m.toggleFullscreenPane(fullscreenDetails)
	if m.focused != focusDetail || m.fullscreen != fullscreenDetails {
		t.Fatalf("narrow first press: focused=%d fullscreen=%d, want focusDetail/Details", m.focused, m.fullscreen)
	}
	// Other key swaps straight to issues fullscreen.
	m.toggleFullscreenPane(fullscreenIssues)
	if m.focused != focusList || m.fullscreen != fullscreenIssues {
		t.Fatalf("narrow swap: focused=%d fullscreen=%d, want focusList/Issues", m.focused, m.fullscreen)
	}
	// Re-press exits fullscreen.
	m.toggleFullscreenPane(fullscreenIssues)
	if m.fullscreen != fullscreenNone {
		t.Fatalf("narrow re-press: fullscreen=%d, want None", m.fullscreen)
	}
}
