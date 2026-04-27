package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/seanmartinsmith/beadstui/pkg/drift"
	"github.com/seanmartinsmith/beadstui/pkg/model"
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// pressRune drives a single-character key through handleKeyPress.
func pressRune(m Model, r rune) Model {
	got, _ := m.handleKeyPress(tea.KeyPressMsg{Code: r, Text: string(r)})
	return got
}

// pressTab drives the Tab key through handleKeyPress.
func pressTab(m Model) Model {
	got, _ := m.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyTab})
	return got
}

func seedModel() Model {
	m := NewModel(nil, nil, "", nil)
	m.width = 120
	m.height = 40
	m.mode = ViewList
	m.ready = true
	// Seed one alert so visibleAlerts() is non-empty (prevents the
	// "No active alerts" short-circuit in the `!` handler).
	m.alerts = []drift.Alert{{
		Type:     drift.AlertStale,
		Severity: drift.SeverityWarning,
		Message:  "fixture",
		IssueID:  "bt-fix",
	}}
	return m
}

func TestNotificationModal_BangOpensAlertsTab(t *testing.T) {
	m := seedModel()
	m = pressRune(m, '!')
	if m.activeModal != ModalAlerts {
		t.Fatalf("expected ModalAlerts, got %v", m.activeModal)
	}
	if m.activeTab != TabAlerts {
		t.Fatalf("expected TabAlerts, got %v", m.activeTab)
	}
}

func TestNotificationModal_OneOpensNotificationsTab(t *testing.T) {
	m := seedModel()
	m = pressRune(m, '1')
	if m.activeModal != ModalAlerts {
		t.Fatalf("expected ModalAlerts, got %v", m.activeModal)
	}
	if m.activeTab != TabNotifications {
		t.Fatalf("expected TabNotifications, got %v", m.activeTab)
	}
}

func TestNotificationModal_KeySwitchesTab(t *testing.T) {
	m := seedModel()
	m = pressRune(m, '!') // open on alerts
	m = pressRune(m, '1') // switch to notifications
	if m.activeModal != ModalAlerts {
		t.Fatalf("modal should stay open, got %v", m.activeModal)
	}
	if m.activeTab != TabNotifications {
		t.Fatalf("expected TabNotifications after switch, got %v", m.activeTab)
	}
}

func TestNotificationModal_SameKeyCloses(t *testing.T) {
	m := seedModel()
	m = pressRune(m, '!')
	m = pressRune(m, '!')
	if m.activeModal == ModalAlerts {
		t.Fatalf("second ! should close modal")
	}
}

func TestNotificationModal_TabCycles(t *testing.T) {
	m := seedModel()
	m = pressRune(m, '!')
	if m.activeTab != TabAlerts {
		t.Fatalf("setup: should be on alerts tab")
	}
	m = pressTab(m)
	if m.activeTab != TabNotifications {
		t.Fatalf("tab should flip to notifications")
	}
	m = pressTab(m)
	if m.activeTab != TabAlerts {
		t.Fatalf("tab should flip back to alerts")
	}
}

func TestNotificationModal_PerTabCursorPreserved(t *testing.T) {
	m := seedModel()
	m.events.Append(events.Event{
		ID: "e1", Kind: events.EventClosed, BeadID: "bt-1",
		Title: "t1", At: time.Now(),
	})
	m.events.Append(events.Event{
		ID: "e2", Kind: events.EventClosed, BeadID: "bt-2",
		Title: "t2", At: time.Now(),
	})
	m = pressRune(m, '!') // alerts tab
	m.alertsCursor = 1
	m = pressRune(m, '1') // switch to notifications
	m.notificationsCursor = 1
	m = pressTab(m) // back to alerts
	if m.alertsCursor != 1 {
		t.Fatalf("alertsCursor drifted: expected 1, got %d", m.alertsCursor)
	}
	m = pressTab(m) // notifications again
	if m.notificationsCursor != 1 {
		t.Fatalf("notificationsCursor drifted: expected 1, got %d", m.notificationsCursor)
	}
}

func TestNotificationModal_NotificationsFromRingBuffer(t *testing.T) {
	m := seedModel()
	now := time.Now()
	m.events.Append(events.Event{ID: "a", Kind: events.EventCreated, BeadID: "bt-a", Title: "first", At: now.Add(-2 * time.Minute)})
	m.events.Append(events.Event{ID: "b", Kind: events.EventClosed, BeadID: "bt-b", Title: "second", At: now.Add(-1 * time.Minute)})
	m.events.Append(events.Event{ID: "c", Kind: events.EventEdited, BeadID: "bt-c", Title: "third", At: now})

	got := m.visibleNotifications()
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	if got[0].ID != "c" || got[2].ID != "a" {
		t.Fatalf("expected newest-first [c,b,a], got [%s,%s,%s]", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestNotificationModal_DismissedHidden(t *testing.T) {
	m := seedModel()
	m.events.Append(events.Event{ID: "keep", Kind: events.EventCreated, BeadID: "bt-k", Title: "kept", At: time.Now()})
	m.events.Append(events.Event{ID: "drop", Kind: events.EventCreated, BeadID: "bt-d", Title: "drop", At: time.Now()})
	m.events.Dismiss("drop")

	got := m.visibleNotifications()
	if len(got) != 1 || got[0].ID != "keep" {
		t.Fatalf("expected only 'keep' visible, got %+v", got)
	}
}

func TestNotificationModal_AttentionViewOneKeyNoConflict(t *testing.T) {
	m := seedModel()
	m.mode = ViewAttention
	before := m.activeModal
	m = pressRune(m, '1')
	if m.activeModal != before {
		t.Fatalf("1 in ViewAttention should NOT open alerts modal, got %v", m.activeModal)
	}
}

func TestNotificationModal_RespectsActiveRepoFilter(t *testing.T) {
	m := seedModel()
	m.workspaceMode = true
	m.activeRepos = map[string]bool{"bt": true}

	m.events.Append(events.Event{ID: "a", Kind: events.EventCreated, BeadID: "bt-a", Repo: "bt", Title: "bt thing", At: time.Now()})
	m.events.Append(events.Event{ID: "b", Kind: events.EventCreated, BeadID: "other-b", Repo: "other", Title: "other thing", At: time.Now()})

	got := m.visibleNotifications()
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("active-repo filter should hide non-bt events; got %+v", got)
	}

	// Expanding to include both repos shows both events.
	m.activeRepos["other"] = true
	got = m.visibleNotifications()
	if len(got) != 2 {
		t.Fatalf("two-repo filter should show both events; got %d", len(got))
	}

	// nil activeRepos (global) shows all events regardless of repo.
	m.activeRepos = nil
	got = m.visibleNotifications()
	if len(got) != 2 {
		t.Fatalf("nil activeRepos should show all events; got %d", len(got))
	}
}

// TestSelectIssueByID_NarrowFilterDoesNotPanic reproduces the crash from
// the 2026-04-24 dogfood: pressing enter on a notification whose bead was
// at a high unfiltered index, while an active search filter narrowed the
// visible set to one item, drove Paginator.Page past TotalPages-1 and
// panicked on the next render with "slice bounds out of range".
func TestSelectIssueByID_NarrowFilterDoesNotPanic(t *testing.T) {
	m := seedModel()

	// Replace the list with a larger one so Paginator has multiple pages.
	issues := make([]model.Issue, 60)
	items := make([]list.Item, 60)
	for i := range issues {
		id := "proj-" + randID(i)
		if i == 55 {
			id = "proj-target"
		}
		issues[i] = model.Issue{ID: id, Title: "Issue " + id, Status: model.StatusOpen, CreatedAt: time.Now()}
		items[i] = IssueItem{Issue: issues[i]}
	}
	lst := list.New(items, list.NewDefaultDelegate(), 80, 20)
	lst.SetFilteringEnabled(true)
	m.list = lst

	// Put cursor on the late-index match, then apply a narrowing filter.
	m.list.Select(55)
	m.list.SetFilterText("target")
	m.list.SetFilterState(list.FilterApplied)

	if got := len(m.list.VisibleItems()); got != 1 {
		t.Fatalf("precondition: narrow filter should show 1 item, got %d", got)
	}

	// selectIssueByID must not drive the paginator out of bounds.
	if !m.selectIssueByID("proj-target") {
		t.Fatalf("expected to find proj-target in visible items")
	}

	// View() exercises populatedView -> Paginator.GetSliceBounds, where
	// the pre-fix bug crashed with "slice bounds out of range".
	_ = m.list.View()
}

// TestSelectIssueByID_HiddenByFilterResets verifies that when the target
// bead is filtered out, selectIssueByID resets the filter so the jump
// still lands — user intent is "take me there," which outranks the filter.
func TestSelectIssueByID_HiddenByFilterResets(t *testing.T) {
	m := seedModel()
	issues := []model.Issue{
		{ID: "bt-a", Title: "apple", Status: model.StatusOpen, CreatedAt: time.Now()},
		{ID: "bt-b", Title: "banana", Status: model.StatusOpen, CreatedAt: time.Now()},
	}
	items := []list.Item{IssueItem{Issue: issues[0]}, IssueItem{Issue: issues[1]}}
	lst := list.New(items, list.NewDefaultDelegate(), 80, 20)
	lst.SetFilteringEnabled(true)
	m.list = lst

	m.list.SetFilterText("apple")
	m.list.SetFilterState(list.FilterApplied)
	if got := len(m.list.VisibleItems()); got != 1 {
		t.Fatalf("precondition: filter should narrow to apple only, got %d visible", got)
	}

	// Target bt-b is filtered out; selectIssueByID should reset filter and find it.
	if !m.selectIssueByID("bt-b") {
		t.Fatalf("expected filter-reset fallback to find bt-b")
	}
	if m.list.FilterState() != list.Unfiltered {
		t.Fatalf("expected filter reset after fallback, got state=%v", m.list.FilterState())
	}
}

func TestRenderAlertsPanel_BorderReflectsActiveTab(t *testing.T) {
	m := seedModel()
	m.activeTab = TabAlerts
	panel := m.renderAlertsPanel()
	if !strings.Contains(panel, "Alerts!") {
		t.Fatalf("alerts tab border should contain 'Alerts!' title; panel=\n%s", panel)
	}
	if !strings.Contains(panel, "(1)") {
		t.Fatalf("alerts tab border should contain '(1)' count (seeded fixture); panel=\n%s", panel)
	}
	if strings.Contains(panel, "Notifications") {
		t.Fatalf("alerts tab border should NOT contain 'Notifications' title; panel=\n%s", panel)
	}

	m.activeTab = TabNotifications
	m.events.Append(events.Event{ID: "x", Kind: events.EventCreated, BeadID: "bt-x", Title: "t", At: time.Now()})
	panel = m.renderAlertsPanel()
	if !strings.Contains(panel, "Notifications") {
		t.Fatalf("notifications tab border should contain 'Notifications' title; panel=\n%s", panel)
	}
	if !strings.Contains(panel, "(1)") {
		t.Fatalf("notifications tab border should contain '(1)' count; panel=\n%s", panel)
	}
}

// TestUpdateViewportContent_ScrollsToCommentAt verifies bt-46p6.16: when
// pendingCommentScroll is set to a comment's CreatedAt and the selected
// bead has a matching comment, updateViewportContent renders, advances the
// viewport YOffset off zero, and clears the pending field.
func TestUpdateViewportContent_ScrollsToCommentAt(t *testing.T) {
	commentTime := time.Date(2026, 4, 22, 12, 30, 0, 0, time.UTC)

	issues := []model.Issue{{
		ID: "bt-target", Title: "Target bead", Status: model.StatusOpen,
		CreatedAt:   time.Now(),
		Description: strings.Repeat("Long description paragraph. ", 12),
		Comments: []*model.Comment{
			{ID: "c1", Author: "sms", Text: strings.Repeat("first comment body. ", 6), CreatedAt: commentTime.Add(-time.Hour)},
			{ID: "c2", Author: "sms", Text: "deep-link target comment body", CreatedAt: commentTime},
			{ID: "c3", Author: "sms", Text: "later comment", CreatedAt: commentTime.Add(time.Hour)},
		},
	}}
	m := NewModel(issues, nil, "", nil)
	m.width = 120
	m.height = 40
	m.mode = ViewList
	m.ready = true
	m.list.Select(0)
	m.viewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

	m.pendingCommentScroll = commentTime
	m.updateViewportContent()

	if !m.pendingCommentScroll.IsZero() {
		t.Errorf("pendingCommentScroll should be cleared after render; got %v", m.pendingCommentScroll)
	}
	if m.viewport.YOffset() == 0 {
		t.Errorf("viewport.YOffset() should be > 0 after scrolling to comment; still 0")
	}
}

// TestUpdateViewportContent_NoScrollWhenPendingZero confirms the deep-link
// path is opt-in: with pendingCommentScroll unset, updateViewportContent
// leaves the viewport at the top regardless of comment count.
func TestUpdateViewportContent_NoScrollWhenPendingZero(t *testing.T) {
	commentTime := time.Date(2026, 4, 22, 12, 30, 0, 0, time.UTC)
	issues := []model.Issue{{
		ID: "bt-target", Title: "T", Status: model.StatusOpen, CreatedAt: time.Now(),
		Description: strings.Repeat("padding. ", 30),
		Comments: []*model.Comment{
			{ID: "c1", Author: "sms", Text: "x", CreatedAt: commentTime},
		},
	}}
	m := NewModel(issues, nil, "", nil)
	m.width = 120
	m.height = 40
	m.mode = ViewList
	m.ready = true
	m.list.Select(0)
	m.viewport = viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))

	m.updateViewportContent()
	if m.viewport.YOffset() != 0 {
		t.Errorf("viewport.YOffset() should stay 0 without pendingCommentScroll; got %d", m.viewport.YOffset())
	}
}

// TestActivateAlert_CentralityChangeOpensInsights covers bt-46p6.12 AC1:
// pressing enter on a centrality_change alert (which has no single-issue
// target) routes to the insights view rather than no-op'ing or jumping
// to a hallucinated bead.
func TestActivateAlert_CentralityChangeOpensInsights(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 120
	m.height = 40
	m.mode = ViewList
	m.ready = true
	m.alerts = []drift.Alert{{
		Type:     drift.AlertCentralityChange,
		Severity: drift.SeverityWarning,
		Message:  "3 PageRank changes detected",
		Details:  []string{"bt-x dropped from top", "bt-y entered top"},
		// IssueID intentionally empty: graph-scope alerts don't carry one.
	}}
	m.activeModal = ModalAlerts
	m.activeTab = TabAlerts
	m.alertsCursor = 0

	got, _ := m.activateCurrentModalItem()
	if got.mode != ViewInsights {
		t.Errorf("centrality_change activation should switch to ViewInsights, got %v", got.mode)
	}
	if got.activeModal == ModalAlerts {
		t.Errorf("activation should close the modal")
	}
}

// TestActivateAlert_StaleAlertJumpsToBead guards against the centrality
// routing accidentally swallowing single-issue alerts.
func TestActivateAlert_StaleAlertJumpsToBead(t *testing.T) {
	issues := []model.Issue{{
		ID: "bt-stale", Title: "stale", Status: model.StatusOpen, CreatedAt: time.Now(),
	}}
	m := NewModel(issues, nil, "", nil)
	m.width = 120
	m.height = 40
	m.mode = ViewList
	m.ready = true
	m.alerts = []drift.Alert{{
		Type:     drift.AlertStale,
		Severity: drift.SeverityWarning,
		Message:  "stale",
		IssueID:  "bt-stale",
	}}
	m.activeModal = ModalAlerts
	m.activeTab = TabAlerts
	m.alertsCursor = 0

	got, _ := m.activateCurrentModalItem()
	if got.mode == ViewInsights {
		t.Errorf("stale alert should not route to ViewInsights")
	}
	if got.activeModal == ModalAlerts {
		t.Errorf("activation should close the modal")
	}
}

// TestAlertsRender_DependencyLoopShowsCyclePath covers bt-7ye5: the dep loop
// alert's Details (cycle paths) must reach the modal, otherwise the user sees
// only "N new cycle(s) detected" with no way to know which beads are looping.
func TestAlertsRender_DependencyLoopShowsCyclePath(t *testing.T) {
	m := seedModel()
	m.alerts = []drift.Alert{{
		Type:     drift.AlertDependencyLoop,
		Severity: drift.SeverityCritical,
		Message:  "2 new cycle(s) detected",
		Details:  []string{"bt-x → bt-y → bt-x", "bt-z → bt-w → bt-z"},
		// IssueID empty — graph-scope alert
	}}
	m = pressRune(m, '!')

	rendered := m.renderAlertsPanel()
	if !strings.Contains(rendered, "bt-x → bt-y → bt-x") {
		t.Errorf("expected first cycle path in modal, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "+1 more") {
		t.Errorf("expected '+1 more' suffix when len(Details) > 1, got:\n%s", rendered)
	}
}

// TestAlertsRender_CentralityChangeShowsFirstChange mirrors the dep-loop case
// for centrality_change alerts — same root cause, same fix.
func TestAlertsRender_CentralityChangeShowsFirstChange(t *testing.T) {
	m := seedModel()
	m.alerts = []drift.Alert{{
		Type:     drift.AlertCentralityChange,
		Severity: drift.SeverityWarning,
		Message:  "3 PageRank changes detected",
		Details:  []string{"bt-x dropped from top", "bt-y entered top", "bt-z: 50.0% change"},
	}}
	m = pressRune(m, '!')

	rendered := m.renderAlertsPanel()
	if !strings.Contains(rendered, "bt-x dropped from top") {
		t.Errorf("expected first detail entry in modal, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "+2 more") {
		t.Errorf("expected '+2 more' suffix, got:\n%s", rendered)
	}
}

// TestNotifications_DismissedFilterToggle covers bt-46p6.13's dismissed-events
// filter: pressing `d` on the notifications tab flips visibility of dismissed
// events. v1 hides dismissed unconditionally; v2 lets the user surface them
// for audit without restoring them.
func TestNotifications_DismissedFilterToggle(t *testing.T) {
	m := NewModel(nil, nil, "", nil)
	m.width = 120
	m.height = 40
	m.mode = ViewList
	m.ready = true

	now := time.Now()
	m.events.Append(events.Event{ID: "evt-1", Kind: events.EventClosed, BeadID: "bt-x", Repo: "bt", Title: "live", At: now})
	m.events.Append(events.Event{ID: "evt-2", Kind: events.EventClosed, BeadID: "bt-y", Repo: "bt", Title: "tomb", At: now})
	m.events.Dismiss("evt-2")

	if got := len(m.visibleNotifications()); got != 1 {
		t.Fatalf("v1 default should hide dismissed; got %d visible", got)
	}

	m.activeModal = ModalAlerts
	m.activeTab = TabNotifications
	m.notificationsCursor = 0

	got, _ := m.handleNotificationsKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if !got.notifShowDismissed {
		t.Errorf("expected notifShowDismissed=true after `d`")
	}
	if n := len(got.visibleNotifications()); n != 2 {
		t.Errorf("expected 2 visible after toggle (live + dismissed); got %d", n)
	}
	if got.notificationsCursor != 0 {
		t.Errorf("toggle should reset cursor; got %d", got.notificationsCursor)
	}

	got2, _ := got.handleNotificationsKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if got2.notifShowDismissed {
		t.Errorf("second `d` should toggle off")
	}
	if n := len(got2.visibleNotifications()); n != 1 {
		t.Errorf("expected 1 visible after toggle off; got %d", n)
	}
}

// TestActivateNotification_NonCommentEventDoesNotQueueScroll ensures only
// EventCommented populates pendingCommentScroll. Other event kinds activate
// normally without queueing a comment scroll.
func TestActivateNotification_NonCommentEventDoesNotQueueScroll(t *testing.T) {
	issues := []model.Issue{{
		ID: "bt-x", Title: "X", Status: model.StatusOpen, CreatedAt: time.Now(),
	}}
	m := NewModel(issues, nil, "", nil)
	m.width = 120
	m.height = 40
	m.mode = ViewList
	m.ready = true

	m.events.Append(events.Event{
		ID: "evt-closed", Kind: events.EventClosed,
		BeadID: "bt-x", Repo: "bt", Title: "X", At: time.Now(),
	})
	m.activeModal = ModalAlerts
	m.activeTab = TabNotifications
	m.notificationsCursor = 0

	got, _ := m.activateCurrentModalItem()
	if !got.pendingCommentScroll.IsZero() {
		t.Errorf("non-comment event must not leave pendingCommentScroll set; got %v", got.pendingCommentScroll)
	}
	if got.activeModal == ModalAlerts {
		t.Errorf("activation should close the modal")
	}
}
