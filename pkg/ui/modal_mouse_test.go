package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// modalMouseModel returns a Model configured for alerts-modal mouse tests:
// known terminal size, a seeded set of alerts, and the modal opened on the
// alerts tab. Dimensions chosen so the centering math is simple:
//
//	width=120, height=40
//	panelWidth  = min(80, width-4)        = 80
//	panelHeight = height * 7 / 10         = 28
//	startCol    = (width - 80) / 2        = 20
//	startRow    = (height-1 - 28) / 2     = 5
//	first item row on screen = startRow + modalChromeAboveItems = 10
func modalMouseModel(t *testing.T) Model {
	t.Helper()
	m := seedModel()
	m.alerts = []drift.Alert{
		{Type: drift.AlertStale, Severity: drift.SeverityWarning, Message: "stale a", IssueID: "bt-a"},
		{Type: drift.AlertStale, Severity: drift.SeverityWarning, Message: "stale b", IssueID: "bt-b"},
		{Type: drift.AlertStale, Severity: drift.SeverityWarning, Message: "stale c", IssueID: "bt-c"},
		{Type: drift.AlertStale, Severity: drift.SeverityWarning, Message: "stale d", IssueID: "bt-d"},
	}
	m = pressRune(m, '!') // open alerts tab
	if m.activeModal != ModalAlerts {
		t.Fatalf("setup: modal did not open")
	}
	if m.activeTab != TabAlerts {
		t.Fatalf("setup: expected TabAlerts")
	}
	return m
}

func TestAlertsModal_ClickAlertRowMovesCursor(t *testing.T) {
	m := modalMouseModel(t)
	// cursor starts at 0; seedModel items map bt-fix first, then a/b/c/d.
	// visibleAlerts order = slice order, so index 2 targets a non-cursor row.
	firstItemY := 5 + modalChromeAboveItems // 10
	// Need to click past the selected-detail line. Cursor is 0; the detail
	// row for item 0 sits at firstItemY+1 when bt-fix has a list title — it
	// doesn't in this fixture (m.list is empty), so detail is absent and
	// items occupy consecutive rows.
	targetRow := firstItemY + 2
	msg := tea.MouseClickMsg{X: 40, Y: targetRow, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)
	if got.activeModal != ModalAlerts {
		t.Fatalf("modal should remain open after single click, got %v", got.activeModal)
	}
	// Non-double click should not activate; cursor should land on the
	// clicked row index within the visible slice.
	if got.alertsCursor != 2 {
		t.Fatalf("expected alertsCursor=2, got %d", got.alertsCursor)
	}
}

func TestAlertsModal_DoubleClickActivates(t *testing.T) {
	m := modalMouseModel(t)
	firstItemY := 5 + modalChromeAboveItems
	// Click the first item twice at the same coordinates within 500ms.
	msg := tea.MouseClickMsg{X: 40, Y: firstItemY, Button: tea.MouseLeft}
	m, _ = m.handleMouseClick(msg)
	if m.activeModal != ModalAlerts {
		t.Fatalf("first click should not close modal")
	}
	m, _ = m.handleMouseClick(msg)
	if m.activeModal == ModalAlerts {
		t.Fatalf("double-click on item should activate + close modal")
	}
}

func TestAlertsModal_BackdropClickIsNoOp(t *testing.T) {
	m := modalMouseModel(t)
	priorCursor := m.alertsCursor
	// (0, 0) is clearly outside the centered modal region.
	msg := tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)
	if got.activeModal != ModalAlerts {
		t.Fatalf("backdrop click must not close the modal")
	}
	if got.alertsCursor != priorCursor {
		t.Fatalf("backdrop click must not move cursor (was %d, now %d)", priorCursor, got.alertsCursor)
	}
}

func TestAlertsModal_ClickOnChromeNoOp(t *testing.T) {
	m := modalMouseModel(t)
	priorCursor := m.alertsCursor
	// Y at the panel's top border row — inside the modal but on chrome.
	msg := tea.MouseClickMsg{X: 40, Y: 5, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)
	if got.activeModal != ModalAlerts {
		t.Fatalf("chrome click must not close modal")
	}
	if got.alertsCursor != priorCursor {
		t.Fatalf("chrome click must not move cursor (was %d, now %d)", priorCursor, got.alertsCursor)
	}
}

func TestAlertsModal_RightClickIgnored(t *testing.T) {
	m := modalMouseModel(t)
	priorCursor := m.alertsCursor
	firstItemY := 5 + modalChromeAboveItems
	msg := tea.MouseClickMsg{X: 40, Y: firstItemY, Button: tea.MouseRight}
	got, _ := m.handleMouseClick(msg)
	if got.alertsCursor != priorCursor {
		t.Fatalf("right-click must not move cursor")
	}
	if got.activeModal != ModalAlerts {
		t.Fatalf("right-click must not close modal")
	}
}

func TestNotificationsModal_ClickRowMovesCursor(t *testing.T) {
	m := seedModel()
	m.events = events.NewRingBuffer(10)
	m.events.AppendMany([]events.Event{
		{ID: "e1", Kind: events.EventCreated, BeadID: "bt-1", Repo: "bt", Title: "one", At: time.Now()},
		{ID: "e2", Kind: events.EventCreated, BeadID: "bt-2", Repo: "bt", Title: "two", At: time.Now()},
		{ID: "e3", Kind: events.EventCreated, BeadID: "bt-3", Repo: "bt", Title: "three", At: time.Now()},
	})
	m = pressRune(m, '1') // open notifications tab
	if m.activeModal != ModalAlerts || m.activeTab != TabNotifications {
		t.Fatalf("setup: modal/tab not as expected (%v/%v)", m.activeModal, m.activeTab)
	}

	firstItemY := 5 + modalChromeAboveItems
	// Cursor starts at 0; click the third visible row.
	// Selected item writes a 1-row summary only if Summary is non-empty; our
	// fixtures have empty Summary, so rows are consecutive.
	msg := tea.MouseClickMsg{X: 40, Y: firstItemY + 2, Button: tea.MouseLeft}
	got, _ := m.handleMouseClick(msg)
	if got.notificationsCursor != 2 {
		t.Fatalf("expected notificationsCursor=2, got %d", got.notificationsCursor)
	}
	if got.activeModal != ModalAlerts {
		t.Fatalf("single click should not close modal")
	}
}

func TestAlertsModal_DoubleClickWindowExpires(t *testing.T) {
	m := modalMouseModel(t)
	firstItemY := 5 + modalChromeAboveItems
	msg := tea.MouseClickMsg{X: 40, Y: firstItemY, Button: tea.MouseLeft}
	m, _ = m.handleMouseClick(msg)
	// Simulate an old first click by pushing lastModalClickAt past the window.
	m.lastModalClickAt = time.Now().Add(-2 * modalDoubleClickWindow)
	m, _ = m.handleMouseClick(msg)
	if m.activeModal != ModalAlerts {
		t.Fatalf("second click after expiry should remain a single click (no activate), got modal=%v", m.activeModal)
	}
}

func TestAlertsModal_DoubleClickDifferentPositionDoesNotActivate(t *testing.T) {
	m := modalMouseModel(t)
	firstItemY := 5 + modalChromeAboveItems
	m, _ = m.handleMouseClick(tea.MouseClickMsg{X: 40, Y: firstItemY, Button: tea.MouseLeft})
	// Second click on a different row within the window → not a double-click.
	m, _ = m.handleMouseClick(tea.MouseClickMsg{X: 40, Y: firstItemY + 1, Button: tea.MouseLeft})
	if m.activeModal != ModalAlerts {
		t.Fatalf("different-position fast click should not activate, modal=%v", m.activeModal)
	}
}

func TestAlertsModalItemAtY_ChromeGuard(t *testing.T) {
	m := modalMouseModel(t)
	// Y values below modalChromeAboveItems must never return an item.
	for y := 0; y < modalChromeAboveItems; y++ {
		if idx, ok := m.alertsModalItemAtY(y); ok {
			t.Errorf("chrome row y=%d must return false, got idx=%d", y, idx)
		}
	}
	// First item row should map to index start (0 with fresh cursor).
	if idx, ok := m.alertsModalItemAtY(modalChromeAboveItems); !ok || idx != 0 {
		t.Errorf("first item row should map to index 0, got (%d, %v)", idx, ok)
	}
}

// TestAlertsModal_MouseWheelMovesCursor confirms the wheel handler advances
// alertsCursor when the modal is open on the alerts tab. Regression baseline
// for bt-tftj — the notifications equivalent below mirrors this contract.
func TestAlertsModal_MouseWheelMovesCursor(t *testing.T) {
	m := modalMouseModel(t)
	if m.alertsCursor != 0 {
		t.Fatalf("setup: expected alertsCursor=0, got %d", m.alertsCursor)
	}
	// Wheel down advances cursor by one (alerts has 4 items, so up to idx 3).
	got, _ := m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if got.alertsCursor != 1 {
		t.Fatalf("wheel down: expected alertsCursor=1, got %d", got.alertsCursor)
	}
	// Wheel up retreats by one.
	got, _ = got.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if got.alertsCursor != 0 {
		t.Fatalf("wheel up: expected alertsCursor=0, got %d", got.alertsCursor)
	}
	// Wheel up at top stays at 0 (no underflow).
	got, _ = got.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if got.alertsCursor != 0 {
		t.Fatalf("wheel up at top: expected alertsCursor=0, got %d", got.alertsCursor)
	}
}

// TestNotificationsModal_MouseWheelMovesCursor is the bt-tftj fix coverage.
// Mouse wheel in the notifications tab must move notificationsCursor (not
// alertsCursor) and respect the visibleNotifications bounds.
func TestNotificationsModal_MouseWheelMovesCursor(t *testing.T) {
	m := seedModel()
	m.events = events.NewRingBuffer(10)
	m.events.AppendMany([]events.Event{
		{ID: "e1", Kind: events.EventCreated, BeadID: "bt-1", Repo: "bt", Title: "one", At: time.Now()},
		{ID: "e2", Kind: events.EventCreated, BeadID: "bt-2", Repo: "bt", Title: "two", At: time.Now()},
		{ID: "e3", Kind: events.EventCreated, BeadID: "bt-3", Repo: "bt", Title: "three", At: time.Now()},
	})
	m = pressRune(m, '1') // open notifications tab
	if m.activeModal != ModalAlerts || m.activeTab != TabNotifications {
		t.Fatalf("setup: modal/tab not as expected (%v/%v)", m.activeModal, m.activeTab)
	}
	if m.notificationsCursor != 0 {
		t.Fatalf("setup: expected notificationsCursor=0, got %d", m.notificationsCursor)
	}
	priorAlertsCursor := m.alertsCursor

	// Wheel down advances notificationsCursor.
	got, _ := m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if got.notificationsCursor != 1 {
		t.Fatalf("wheel down: expected notificationsCursor=1, got %d", got.notificationsCursor)
	}
	if got.alertsCursor != priorAlertsCursor {
		t.Fatalf("wheel down on notifications tab must not move alertsCursor (was %d, now %d)", priorAlertsCursor, got.alertsCursor)
	}
	// Two more wheel-downs should land on idx 2 then clamp at 2 (last index).
	got, _ = got.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	got, _ = got.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if got.notificationsCursor != 2 {
		t.Fatalf("wheel down clamp: expected notificationsCursor=2, got %d", got.notificationsCursor)
	}
	// Wheel up retreats by one.
	got, _ = got.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if got.notificationsCursor != 1 {
		t.Fatalf("wheel up: expected notificationsCursor=1, got %d", got.notificationsCursor)
	}
	// Wheel up past 0 clamps.
	got, _ = got.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	got, _ = got.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if got.notificationsCursor != 0 {
		t.Fatalf("wheel up clamp: expected notificationsCursor=0, got %d", got.notificationsCursor)
	}
}
