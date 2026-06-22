# Footer Phase 4 - Notifications Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the footer notification system - severity-tiered transient toasts and a permanent unread bell (`🔔N`) sourced from the events ring buffer.

**Architecture:** Two channels. (1) A severity-driven *toast* reuses the existing `statusMsg` path but renders inline in the footer's right zone (replacing key hints, yielding back when it clears); severity drives glyph, lifetime, and whether the event is recorded. (2) A permanent *bell* counts ring-buffer events unseen since `alertsSeenAt` (a session high-water-mark set when the notifications view opens). Failure/Degraded toasts append an `EventSystem` to the ring buffer so they survive in the alerts modal.

**Tech Stack:** Go 1.25+, Charm Bracelet v2 (Bubble Tea / Lipgloss), the existing `pkg/ui/events` ring buffer.

**Spec:** [docs/design/2026-06-22-footer-phase4-notifications.md](../design/2026-06-22-footer-phase4-notifications.md)

## Global Constraints

- `go build ./...` and `go vet ./...` must pass after every task (AGENTS.md rule 7).
- No file deletion without explicit permission (AGENTS.md rule 1). `renderStatusBar` becomes unreachable after this work - leave it in place, note it; do not delete.
- All edits manual or via subagents - no script-based code changes (AGENTS.md rule 3).
- Bead ref for commits: `bt-a3zi3.1`. Commit format: `type(scope): description (bt-a3zi3.1)`, scope `tui`.
- Module path: `github.com/seanmartinsmith/beadstui`.
- TUI invariants this work must preserve: the footer is always exactly one row (bt-yyked footer-pin) and never wraps/clips (Phase 1 degradation engine). Toast and bell must slot into both.
- Run `go install ./cmd/bt/` after a successful build (user runs `bt` from PATH).

---

## File structure

| File | Responsibility | Change |
|---|---|---|
| `pkg/ui/events/events.go` | Event type + constructors | Add `NewSystemEvent` |
| `pkg/ui/events/ring.go` | Ring buffer | Add `UnseenCount(since)` |
| `pkg/ui/model_footer.go` | Status setters, `StatusSeverity`, `FooterData`, `Render` | Severity type, setters, toast right-zone + bell render |
| `pkg/ui/model.go` | Model struct + `NewModel` | Replace `statusIsError`→`statusSeverity`; add `alertsSeenAt` |
| `pkg/ui/model_update_analysis.go` | Status tick/clear handlers | Severity-based auto-dismiss |
| `pkg/ui/model_update_data.go` | Snapshot/reload handlers | Clear Degraded toast on recovery |
| `pkg/ui/model_update_input.go` | Key dispatch, status reset | Severity reset; `markNotificationsSeen` calls |
| `pkg/ui/*_keys.go`, `model_editor.go`, `model_export.go`, `model_modes.go`, `model_update_*.go` | Error call sites | Reclassify 41 `setStatusError` calls |
| `pkg/ui/model_footer_test.go` | Footer unit tests | Severity/toast/bell tests |
| `pkg/ui/render_harness_test.go` | Render harness | Notification scenarios |
| `pkg/ui/events/*_test.go` | Events unit tests | `NewSystemEvent`, `UnseenCount` tests |

---

### Task 1: events package - system-event constructor + unseen count

**Files:**
- Modify: `pkg/ui/events/events.go` (add `NewSystemEvent` after `computeID`, ~line 97)
- Modify: `pkg/ui/events/ring.go` (add `UnseenCount` after `UnreadCount`, ~line 136)
- Test: `pkg/ui/events/events_test.go`, `pkg/ui/events/ring_test.go`

**Interfaces:**
- Produces: `func NewSystemEvent(summary string) Event` - an `EventSystem` event with `Summary` set, `At = time.Now()`, a non-empty `ID`, empty `BeadID`/`Repo`.
- Produces: `func (r *RingBuffer) UnseenCount(since time.Time) int` - count of events with `At.After(since)` and `!Dismissed`.

- [ ] **Step 1: Write the failing test for NewSystemEvent**

In `pkg/ui/events/events_test.go`:

```go
func TestNewSystemEvent(t *testing.T) {
	e := NewSystemEvent("write failed: db locked")
	if e.Kind != EventSystem {
		t.Errorf("Kind = %v, want EventSystem", e.Kind)
	}
	if e.Summary != "write failed: db locked" {
		t.Errorf("Summary = %q, want the message", e.Summary)
	}
	if e.ID == "" {
		t.Error("ID must be non-empty for dedup/dismissal")
	}
	if e.BeadID != "" {
		t.Errorf("BeadID = %q, want empty for a system event", e.BeadID)
	}
	if e.At.IsZero() {
		t.Error("At must be set")
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./pkg/ui/events/ -run TestNewSystemEvent -v`
Expected: FAIL - `undefined: NewSystemEvent`.

- [ ] **Step 3: Implement NewSystemEvent**

In `pkg/ui/events/events.go`, after `computeID` (line ~96):

```go
// NewSystemEvent builds an ambient, non-bead EventSystem (e.g. a write
// failure or a degraded-service notice surfaced by the footer). BeadID and
// Repo are empty; consumers that key off bead identity must guard for that.
func NewSystemEvent(summary string) Event {
	at := time.Now()
	return Event{
		ID:      computeID("", EventSystem, at),
		Kind:    EventSystem,
		Summary: summary,
		At:      at,
		Source:  SourceDolt,
	}
}
```

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./pkg/ui/events/ -run TestNewSystemEvent -v`
Expected: PASS.

- [ ] **Step 5: Write the failing test for UnseenCount**

In `pkg/ui/events/ring_test.go`:

```go
func TestUnseenCount(t *testing.T) {
	r := NewRingBuffer(10)
	base := time.Now()
	r.Append(Event{ID: "a", Kind: EventCreated, At: base.Add(-2 * time.Minute)})
	r.Append(Event{ID: "b", Kind: EventCreated, At: base.Add(-1 * time.Minute)})
	r.Append(Event{ID: "c", Kind: EventSystem, At: base})

	// Everything is newer than the zero time.
	if got := r.UnseenCount(time.Time{}); got != 3 {
		t.Errorf("UnseenCount(zero) = %d, want 3", got)
	}
	// Only events strictly after the cutoff count.
	if got := r.UnseenCount(base.Add(-90 * time.Second)); got != 2 {
		t.Errorf("UnseenCount(cutoff) = %d, want 2", got)
	}
	// Dismissed events never count, even when newer than the cutoff.
	r.Dismiss("c")
	if got := r.UnseenCount(base.Add(-90 * time.Second)); got != 1 {
		t.Errorf("UnseenCount after dismiss = %d, want 1", got)
	}
}
```

- [ ] **Step 6: Run it, verify it fails**

Run: `go test ./pkg/ui/events/ -run TestUnseenCount -v`
Expected: FAIL - `r.UnseenCount undefined`.

- [ ] **Step 7: Implement UnseenCount**

In `pkg/ui/events/ring.go`, after `UnreadCount` (line ~136):

```go
// UnseenCount returns the number of non-dismissed events whose timestamp is
// strictly after `since`. The footer bell uses this with a session
// high-water-mark so opening the notifications view (which advances the
// mark) clears the badge without dismissing anything.
func (r *RingBuffer) UnseenCount(since time.Time) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for i := range r.events {
		if !r.events[i].Dismissed && r.events[i].At.After(since) {
			n++
		}
	}
	return n
}
```

- [ ] **Step 8: Run it, verify it passes**

Run: `go test ./pkg/ui/events/ -v`
Expected: PASS (all events tests).

- [ ] **Step 9: Commit**

```bash
git add pkg/ui/events/events.go pkg/ui/events/ring.go pkg/ui/events/events_test.go pkg/ui/events/ring_test.go
git commit -m "feat(tui): events NewSystemEvent + UnseenCount for footer bell (bt-a3zi3.1)"
```

---

### Task 2: StatusSeverity type + field migration (behavior-preserving)

Replace the `statusIsError bool` with a 4-way `statusSeverity`, threading it through every read/write site. Behavior stays equivalent (`setStatus`→Success, `setStatusError`→Failure for now, reclassified in Task 4). Adds the unused `alertsSeenAt` field for later tasks.

**Files:**
- Modify: `pkg/ui/model_footer.go:70` (add type), `:22-48` (setters), `:96-98` (FooterData), `:198-200` (footerData), `:663-675` (Render top), `:1095-1114` (renderStatusBar)
- Modify: `pkg/ui/model.go:694` (field), `:1196-1205,1322` (init), add `alertsSeenAt`
- Modify: `pkg/ui/model_update_analysis.go:71-94` (handlers)
- Modify: `pkg/ui/model_update_input.go:50-52` (keypress reset)
- Test: `pkg/ui/model_footer_test.go`

**Interfaces:**
- Produces: `type StatusSeverity int` with `SeverityNone, SeveritySuccess, SeverityNotice, SeverityFailure, SeverityDegraded`.
- Produces: `func (s StatusSeverity) glyph() string` → `""`/`"✓"`/`""`/`"✗"`/`"⚠"`.
- Produces: Model field `statusSeverity StatusSeverity`, `alertsSeenAt time.Time`.
- Produces: `FooterData.StatusSeverity StatusSeverity` (replaces `StatusIsErr`).

- [ ] **Step 1: Write the failing test for the severity glyph + FooterData wiring**

In `pkg/ui/model_footer_test.go` (replace the existing `TestFooterData_StatusBarOverride` body's `StatusIsErr` usage and add a glyph test):

```go
func TestStatusSeverityGlyph(t *testing.T) {
	cases := map[StatusSeverity]string{
		SeveritySuccess:  "✓",
		SeverityNotice:   "",
		SeverityFailure:  "✗",
		SeverityDegraded: "⚠",
	}
	for sev, want := range cases {
		if got := sev.glyph(); got != want {
			t.Errorf("severity %d glyph = %q, want %q", sev, got, want)
		}
	}
}
```

Also update `TestFooterData_StatusBarOverride` (line ~13): change `StatusIsErr: false` to `StatusSeverity: SeveritySuccess`.

- [ ] **Step 2: Run it, verify it fails to compile**

Run: `go test ./pkg/ui/ -run TestStatusSeverityGlyph -v`
Expected: FAIL - `undefined: SeveritySuccess` / `glyph`.

- [ ] **Step 3: Add the StatusSeverity type**

In `pkg/ui/model_footer.go`, after the imports / before `WorkerLevel` (line ~69):

```go
// StatusSeverity classifies a footer toast (bt-a3zi3.1). It drives the
// glyph, the auto-dismiss lifetime (see statusDismissAge), and whether the
// toast is also recorded in the events ring buffer (Failure/Degraded are).
type StatusSeverity int

const (
	SeverityNone     StatusSeverity = iota // no status message
	SeveritySuccess                        // ✓ confirmation; ~3s; no bell
	SeverityNotice                         // rejection/validation; ~3s; no bell
	SeverityFailure                        // ✗ one-shot failure; ~8s; bell
	SeverityDegraded                       // ⚠ live condition; sticky; bell
)

// glyph is the leading symbol for a toast of this severity ("" = none).
func (s StatusSeverity) glyph() string {
	switch s {
	case SeveritySuccess:
		return "✓"
	case SeverityFailure:
		return "✗"
	case SeverityDegraded:
		return "⚠"
	default:
		return ""
	}
}
```

- [ ] **Step 4: Migrate the Model field + init sites**

In `pkg/ui/model.go:694`, replace `statusIsError  bool` with:

```go
	statusSeverity StatusSeverity // severity of the active toast (bt-a3zi3.1)
```

Add directly below the `events *events.RingBuffer` field (line ~703):

```go
	// alertsSeenAt is the session high-water-mark for the footer bell. Events
	// newer than this and not dismissed count toward 🔔N; opening the
	// notifications view advances it to now, clearing the badge (bt-a3zi3.1).
	alertsSeenAt time.Time
```

In `pkg/ui/model.go:1196`, replace `var initialStatusErr bool` with `var initialStatusSeverity StatusSeverity`. At lines 1199/1202/1205, replace the boolean assignments:
- line 1199: `initialStatusSeverity = SeveritySuccess`
- line 1202: `initialStatusSeverity = SeverityFailure`
- line 1205: `initialStatusSeverity = SeverityFailure`

In `pkg/ui/model.go:1322`, replace `statusIsError: initialStatusErr,` with `statusSeverity: initialStatusSeverity,`.

- [ ] **Step 5: Migrate the setters**

In `pkg/ui/model_footer.go`, rewrite the three setters (lines 22-48) so they set `statusSeverity` and render inline (the toast now lives in the right zone, so all toasts are inline - the bell must stay visible beside them):

```go
func (m *Model) setInlineTransientStatus(msg string, d time.Duration) tea.Cmd {
	m.statusMsg = msg
	m.statusSeverity = SeveritySuccess
	m.statusIsInline = true
	m.statusSetAt = time.Now()
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(d, func(time.Time) tea.Msg {
		return statusClearMsg{seq: seq}
	})
}

// setStatus sets a success/info confirmation toast (✓, ~3s auto-fade, no bell).
func (m *Model) setStatus(msg string) {
	m.statusMsg = msg
	m.statusSeverity = SeveritySuccess
	m.statusIsInline = true
	m.statusSetAt = time.Now()
}

// setStatusError sets a failure toast. TEMPORARY: Task 4 reclassifies callers
// to setNotice/setFailure/setDegraded; until then this maps to Failure to
// preserve "error" behavior.
func (m *Model) setStatusError(msg string) {
	m.statusMsg = msg
	m.statusSeverity = SeverityFailure
	m.statusIsInline = true
	m.statusSetAt = time.Now()
}
```

- [ ] **Step 6: Migrate FooterData + footerData + Render glyph sites + handlers + keypress reset**

In `pkg/ui/model_footer.go:97`, replace `StatusIsErr    bool` with `StatusSeverity StatusSeverity`.

In `pkg/ui/model_footer.go:199`, replace `StatusIsErr:    m.statusIsError,` with `StatusSeverity: m.statusSeverity,`.

In `pkg/ui/model_footer.go`, the inline override block (lines 669-675) is removed entirely in Task 6; for now, to keep this task behavior-preserving and compiling, replace it with the severity glyph:

```go
	if fd.StatusMsg != "" && fd.StatusIsInline {
		prefix := fd.StatusSeverity.glyph()
		if prefix != "" {
			prefix += " "
		}
		fd.HintText = prefix + fd.StatusMsg
	}
```

In `renderStatusBar` (lines 1095-1114), replace the two `if fd.StatusIsErr` checks with `if fd.StatusSeverity >= SeverityFailure` (error styling for Failure/Degraded), and replace the `prefix` block (lines 1110-1113) with:

```go
	prefix := fd.StatusSeverity.glyph()
	if prefix != "" {
		prefix += " "
	}
	displayMsg := prefix + fd.StatusMsg
```

In `pkg/ui/model_update_analysis.go`, update `handleStatusClear` (line 75) - replace `m.statusIsError = false` with `m.statusSeverity = SeverityNone`. Update `handleStatusTick` (line 85) - replace `!m.statusIsError` with `m.statusSeverity != SeverityDegraded` (Degraded is the only sticky severity).

In `pkg/ui/model_update_input.go:51`, replace `m.statusIsError = false` with `m.statusSeverity = SeverityNone`.

- [ ] **Step 7: Build, vet, and run footer tests**

Run: `go build ./... && go vet ./... && go test ./pkg/ui/ -run 'TestStatusSeverityGlyph|TestFooterData_StatusBarOverride' -v`
Expected: build/vet clean; both tests PASS.

- [ ] **Step 8: Commit**

```bash
git add pkg/ui/model_footer.go pkg/ui/model.go pkg/ui/model_update_analysis.go pkg/ui/model_update_input.go pkg/ui/model_footer_test.go
git commit -m "refactor(tui): statusIsError -> 4-way StatusSeverity, add alertsSeenAt (bt-a3zi3.1)"
```

---

### Task 3: Severity behaviors - per-severity timing + bell append

Add the severity-driven lifetime (Success/Notice 3s, Failure 8s, Degraded sticky) and the new error setters; Failure/Degraded append an `EventSystem` to the ring buffer.

**Files:**
- Modify: `pkg/ui/model_footer.go` (add `statusDismissAge`, new setters, `clearStatus`)
- Modify: `pkg/ui/model_update_analysis.go:82-94` (handleStatusTick uses `statusDismissAge`)
- Test: `pkg/ui/model_footer_test.go`

**Interfaces:**
- Produces: `func statusDismissAge(s StatusSeverity) time.Duration`.
- Produces: `func (m *Model) setNotice(msg string)`, `setFailure(msg string)`, `setDegraded(msg string)`, `clearStatus()`.
- Consumes: `events.NewSystemEvent` (Task 1), `m.events` ring buffer.

- [ ] **Step 1: Write failing tests for the new setters' bell behavior**

In `pkg/ui/model_footer_test.go`:

```go
func TestErrorSettersBellAppend(t *testing.T) {
	newM := func() *Model {
		m := NewModel(harnessIssues(), nil, "", nil)
		return &m
	}
	t.Run("notice does not touch the bell", func(t *testing.T) {
		m := newM()
		before := m.events.Len()
		m.setNotice("No issue selected")
		if m.events.Len() != before {
			t.Errorf("Notice appended an event; bell must stay clean")
		}
		if m.statusSeverity != SeverityNotice {
			t.Errorf("severity = %d, want Notice", m.statusSeverity)
		}
	})
	t.Run("failure appends one system event", func(t *testing.T) {
		m := newM()
		before := m.events.Len()
		m.setFailure("Export failed: disk full")
		if m.events.Len() != before+1 {
			t.Fatalf("Failure appended %d events, want 1", m.events.Len()-before)
		}
		if m.statusSeverity != SeverityFailure {
			t.Errorf("severity = %d, want Failure", m.statusSeverity)
		}
	})
	t.Run("degraded appends one system event and is sticky", func(t *testing.T) {
		m := newM()
		before := m.events.Len()
		m.setDegraded("Dolt server unreachable (retrying)")
		if m.events.Len() != before+1 {
			t.Fatalf("Degraded appended %d events, want 1", m.events.Len()-before)
		}
		if statusDismissAge(SeverityDegraded) != 0 {
			t.Errorf("Degraded must be sticky (dismiss age 0)")
		}
	})
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./pkg/ui/ -run TestErrorSettersBellAppend -v`
Expected: FAIL - `m.setNotice undefined`.

- [ ] **Step 3: Implement statusDismissAge, the setters, and clearStatus**

In `pkg/ui/model_footer.go`, near the other status constants (after line 56):

```go
// statusDismissAge is how long a toast of the given severity stays before
// the idle tick clears it. Degraded returns 0 (sticky - cleared only by the
// recovery path; see handleSnapshotReady).
func statusDismissAge(s StatusSeverity) time.Duration {
	switch s {
	case SeverityFailure:
		return 8 * time.Second
	case SeverityDegraded:
		return 0
	default: // Success, Notice
		return 3 * time.Second
	}
}
```

After the existing setters (after line 48):

```go
// setNotice sets a Notice toast (rejection/validation; ~3s; no bell entry).
func (m *Model) setNotice(msg string) {
	m.statusMsg = msg
	m.statusSeverity = SeverityNotice
	m.statusIsInline = true
	m.statusSetAt = time.Now()
}

// setFailure sets a Failure toast (one-shot op failure; ~8s) and records it
// in the events ring buffer so it survives in the alerts modal.
func (m *Model) setFailure(msg string) {
	m.statusMsg = msg
	m.statusSeverity = SeverityFailure
	m.statusIsInline = true
	m.statusSetAt = time.Now()
	if m.events != nil {
		m.events.Append(events.NewSystemEvent(msg))
	}
}

// setDegraded sets a Degraded toast (live condition; sticky until the
// recovery path clears it) and records it in the ring buffer.
func (m *Model) setDegraded(msg string) {
	m.statusMsg = msg
	m.statusSeverity = SeverityDegraded
	m.statusIsInline = true
	m.statusSetAt = time.Now()
	if m.events != nil {
		m.events.Append(events.NewSystemEvent(msg))
	}
}

// clearStatus clears any active toast (used by the recovery path to drop a
// sticky Degraded toast once the condition resolves).
func (m *Model) clearStatus() {
	m.statusMsg = ""
	m.statusSeverity = SeverityNone
	m.statusIsInline = false
}
```

Add the events import to `pkg/ui/model_footer.go` if not present:

```go
	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
```

- [ ] **Step 4: Wire severity timing into handleStatusTick**

In `pkg/ui/model_update_analysis.go`, replace the body of `handleStatusTick` (lines 84-92) with:

```go
	if m.statusMsg != "" && m.statusSeverity != SeverityDegraded {
		age := statusDismissAge(m.statusSeverity)
		if m.statusSetAt.IsZero() {
			m.statusSetAt = time.Now()
		} else if age > 0 && time.Since(m.statusSetAt) > age {
			m.statusMsg = ""
			m.statusSeverity = SeverityNone
			m.statusIsInline = false
		}
	}
```

Remove the now-unused `statusAutoDismissAge` constant in `pkg/ui/model_footer.go:52` ONLY if `go vet` flags it as unused; otherwise leave it (it may be referenced in tests). Verify with `grep -rn statusAutoDismissAge pkg/ui/`.

- [ ] **Step 5: Run the tests, verify they pass**

Run: `go test ./pkg/ui/ -run 'TestErrorSettersBellAppend|TestStatusSeverityGlyph' -v && go build ./... && go vet ./...`
Expected: PASS; build/vet clean.

- [ ] **Step 6: Commit**

```bash
git add pkg/ui/model_footer.go pkg/ui/model_update_analysis.go pkg/ui/model_footer_test.go
git commit -m "feat(tui): severity timing + Failure/Degraded bell append (bt-a3zi3.1)"
```

---

### Task 4: Reclassify the 41 setStatusError call sites

Mechanically replace each `m.setStatusError(...)` with the severity-correct setter per spec section 3. After this, `setStatusError` has no callers - leave the function (Task 2 marked it temporary); it is now dead but harmless. Do NOT delete it (AGENTS.md rule 1) unless you have explicit permission.

**Files (all under `pkg/ui/`):** `board_keys.go`, `graph_keys.go`, `history_keys.go`, `list_keys.go`, `model_editor.go`, `model_export.go`, `model_modes.go`, `model_update_analysis.go`, `model_update_data.go`, `model_update_input.go`.

**Classification (from spec section 3):**

- **setNotice** (validation/rejection): `No issue selected`, `No commit selected`, `No bead selected`, `select an issue to enable swarm view`, `No git remote configured`, `Could not open browser`, every `Clipboard error: %v`, `❌ No issue selected` (export), `❌ Invalid item type`.
- **setFailure** (one-shot op failure): `Export failed`, `Time-travel failed`/`Time-travel requires...`/`No beads history at...` (model_modes), `Failed to open editor`, the `$EDITOR`/`.beads`/`No GUI editor` config errors (model_editor), `Failed to update <file>` (model_update_input), `Semantic search unavailable`, `Hybrid search unavailable`, `History load failed`, `Semantic search unavailable`/`Refresh unavailable` (model_update_input), `swarm: %v` (graph), `Cannot get working directory for history`.
- **setDegraded** (live retrying condition, model_update_data.go): `Reload error (retrying)` (line ~406), `Dolt server unreachable (retrying in %ds)` (line ~471). NOTE: the non-retry `Reload error`/`Reload failed` variants (lines ~408, 423, 571) are **setFailure** (one-shot, not retrying).

- [ ] **Step 1: List every call site**

Run: `grep -rn "setStatusError" pkg/ui/ | grep -v "_test.go" | grep -v "func "`
Expected: 41 call sites across the files above. Work through each, applying the classification. Most map to `setNotice` or `setFailure`; only the two retrying conditions in `model_update_data.go` map to `setDegraded`.

- [ ] **Step 2: Apply replacements**

For each site, replace `m.setStatusError(X)` with `m.setNotice(X)` / `m.setFailure(X)` / `m.setDegraded(X)` per the classification. Example (`pkg/ui/list_keys.go:145`):

```go
// before: m.setStatusError("No issue selected")
m.setNotice("No issue selected")
```

Example (`pkg/ui/model_update_data.go:471`):

```go
// before: m.setStatusError(fmt.Sprintf("Dolt server unreachable (retrying in %ds)", msg.BackoffSeconds))
m.setDegraded(fmt.Sprintf("Dolt server unreachable (retrying in %ds)", msg.BackoffSeconds))
```

- [ ] **Step 3: Verify no setStatusError callers remain**

Run: `grep -rn "m.setStatusError" pkg/ui/ | grep -v "_test.go"`
Expected: no output (zero callers).

- [ ] **Step 4: Build, vet, full test**

Run: `go build ./... && go vet ./... && go test ./pkg/ui/`
Expected: build/vet clean; tests pass.

- [ ] **Step 5: Commit**

```bash
git add pkg/ui/
git commit -m "refactor(tui): reclassify 41 error sites into severity tiers (bt-a3zi3.1)"
```

---

### Task 5: Degraded self-clear on recovery

A Degraded toast (e.g. `Dolt server unreachable (retrying)`) is sticky; it must clear when the condition resolves. A successful snapshot is the recovery signal.

**Files:**
- Modify: `pkg/ui/model_update_data.go` (`handleSnapshotReady`, starts line 25)
- Test: `pkg/ui/model_update_data_test.go` (or `model_footer_test.go` if no data test file exists)

**Interfaces:**
- Consumes: `clearStatus()` (Task 3), `m.statusSeverity` (Task 2).

- [ ] **Step 1: Write the failing test**

Add to `pkg/ui/model_footer_test.go`:

```go
func TestDegradedClearsOnSnapshot(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.setDegraded("Dolt server unreachable (retrying)")
	if m.statusMsg == "" {
		t.Fatal("precondition: degraded toast should be set")
	}
	nm, _ := m.handleSnapshotReady(SnapshotReadyMsg{})
	if nm.statusSeverity != SeverityNone || nm.statusMsg != "" {
		t.Errorf("degraded toast should clear on snapshot; got severity=%d msg=%q",
			nm.statusSeverity, nm.statusMsg)
	}
}
```

NOTE: if `SnapshotReadyMsg{}` requires fields to avoid a nil panic in `handleSnapshotReady`, inspect the struct and the handler's early lines and populate the minimum (e.g. a non-nil snapshot). Adjust the test to the real message shape before running.

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./pkg/ui/ -run TestDegradedClearsOnSnapshot -v`
Expected: FAIL - the degraded toast persists.

- [ ] **Step 3: Clear the degraded toast in handleSnapshotReady**

In `pkg/ui/model_update_data.go`, near the top of `handleSnapshotReady` (after the snapshot is accepted as valid, before returning), add:

```go
	// A successful snapshot means the data layer recovered; drop any sticky
	// Degraded toast (e.g. "Dolt unreachable, retrying"). bt-a3zi3.1.
	if m.statusSeverity == SeverityDegraded {
		m.clearStatus()
	}
```

Place this where `m` is the local Model value being mutated and returned (handleSnapshotReady has a value receiver `(m Model)`, so mutate the local `m`).

- [ ] **Step 4: Run it, verify it passes**

Run: `go test ./pkg/ui/ -run TestDegradedClearsOnSnapshot -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/ui/model_update_data.go pkg/ui/model_footer_test.go
git commit -m "feat(tui): clear sticky Degraded toast on snapshot recovery (bt-a3zi3.1)"
```

---

### Task 6: Footer rendering - toast right-zone override + bell badge

Move the toast from the left hint slot to the right zone (replacing key hints, yielding back when cleared) and add the permanent bell badge. This is the visible Phase 4 deliverable.

**Files:**
- Modify: `pkg/ui/model_footer.go` (`FooterData` add `BellCount`; `footerData` populate; `Render` toast + bell)
- Test: `pkg/ui/model_footer_test.go`

**Interfaces:**
- Produces: `FooterData.BellCount int`.
- Consumes: `m.events.UnseenCount` (Task 1), `m.alertsSeenAt` (Task 2), `StatusSeverity.glyph` (Task 2).

- [ ] **Step 1: Write failing tests for toast right-zone + bell**

In `pkg/ui/model_footer_test.go`:

```go
func TestFooterToastRightZone(t *testing.T) {
	fd := FooterData{
		Width:          100,
		StatusMsg:      "write failed: db locked",
		StatusSeverity: SeverityFailure,
		StatusIsInline: true,
		FilterText:     "ALL",
		FilterIcon:     "🌐",
		TotalItems:     42,
		Hints:          []FooterHint{{Key: "⏎", Desc: "open"}, {Key: "?", Desc: "help"}},
	}
	out := ansiStripForTest(fd.Render())
	if !strings.Contains(out, "✗ write failed: db locked") {
		t.Errorf("failure toast (with ✗) should appear; got %q", out)
	}
	if strings.Contains(out, "open") {
		t.Errorf("key hints should yield to the toast; 'open' should be gone")
	}
}

func TestFooterBellBadge(t *testing.T) {
	withN := FooterData{Width: 100, FilterText: "ALL", FilterIcon: "🌐", BellCount: 3,
		Hints: []FooterHint{{Key: "?", Desc: "help"}}}
	out := ansiStripForTest(withN.Render())
	if !strings.Contains(out, "🔔3") {
		t.Errorf("bell should show 🔔3; got %q", out)
	}
	zero := withN
	zero.BellCount = 0
	out0 := ansiStripForTest(zero.Render())
	if !strings.Contains(out0, "🔔") {
		t.Errorf("bell glyph should always render; got %q", out0)
	}
	if strings.Contains(out0, "🔔0") {
		t.Errorf("zero count should render bare 🔔, not 🔔0; got %q", out0)
	}
}
```

If a strip helper does not already exist in the test file, add:

```go
func ansiStripForTest(s string) string { return ansi.Strip(s) }
```

(import `github.com/charmbracelet/x/ansi` in the test file if needed.)

- [ ] **Step 2: Run them, verify they fail**

Run: `go test ./pkg/ui/ -run 'TestFooterToastRightZone|TestFooterBellBadge' -v`
Expected: FAIL - `fd.BellCount undefined` and/or toast still in left slot.

- [ ] **Step 3: Add the FooterData field + footerData population**

In `pkg/ui/model_footer.go`, add to `FooterData` (after `CenterOverride`, line ~180):

```go
	// Unread bell (Phase 4): events newer than alertsSeenAt and not dismissed.
	// Always rendered as 🔔; the count suffix appears only when > 0.
	BellCount int
```

In `footerData()` (after the CenterOverride line ~292), add:

```go
	// Footer bell: unseen-since-last-look count from the ring buffer.
	if m.events != nil {
		fd.BellCount = m.events.UnseenCount(m.alertsSeenAt)
	}
```

- [ ] **Step 4: Remove the left-slot inline override**

In `Render()`, delete the inline `HintText` override block (current lines 669-675, the `if fd.StatusMsg != "" && fd.StatusIsInline { ... fd.HintText = prefix + fd.StatusMsg }`). The toast now renders in the right zone (Step 5). Keep the full-width banner branch (lines 666-668) as the defensive fallback for `!StatusIsInline`.

- [ ] **Step 5: Build the toast + bell sections and wire them into the right zone**

In `Render()`, after `keysSection := renderKeys(...)` (line ~1046), add the toast override and the bell:

```go
	// Toast override (Phase 4): an active inline toast borrows the right zone,
	// replacing the key hints; it yields back when the toast clears.
	rightZone := keysSection
	if fd.StatusMsg != "" && fd.StatusIsInline {
		glyph := fd.StatusSeverity.glyph()
		text := fd.StatusMsg
		if glyph != "" {
			text = glyph + " " + text
		}
		var toastStyle lipgloss.Style
		switch fd.StatusSeverity {
		case SeverityFailure:
			toastStyle = lipgloss.NewStyle().Foreground(ColorPrioCritical).Bold(true).Padding(0, 1)
		case SeverityDegraded:
			toastStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Padding(0, 1)
		case SeverityNotice:
			toastStyle = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
		default: // Success
			toastStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Padding(0, 1)
		}
		avail := fd.Width - nonKeyWidth()
		toast := toastStyle.Render(text)
		if avail > 0 && lipgloss.Width(toast) > avail {
			toast = ansi.Truncate(toast, avail, "")
		}
		rightZone = toast
	}

	// Bell badge (Phase 4): always rendered; the count appears only when > 0.
	// Pinned (last to drop) alongside the ? hint.
	bellText := "🔔"
	if fd.BellCount > 0 {
		bellText = fmt.Sprintf("🔔%d", fd.BellCount)
	}
	bellStyle := lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
	if fd.BellCount > 0 {
		bellStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Padding(0, 1)
	}
	bellSection := bellStyle.Render(bellText)
```

Then update the filler/parts assembly (lines 1048-1082):
- Add the bell width into the reserved right-zone width so the filler accounts for it. Replace the `remaining` calculation (line 1049) with:

```go
	remaining := fd.Width - nonKeyWidth() - lipgloss.Width(rightZone) - lipgloss.Width(bellSection)
	if remaining < 0 {
		remaining = 0
	}
```

- In the parts assembly, replace the final two `addIf(countBadge); addIf(keysSection)` (lines 1081-1082) with:

```go
	addIf(countBadge)
	addIf(rightZone)
	addIf(bellSection)
```

NOTE: `nonKeyWidth()` (line 999) does not include the bell; the bell is reserved via the `remaining` subtraction above and appended last, so it is never dropped (it survives the final ansi truncate only in pathological widths, matching the spec's "pinned, last to drop").

- [ ] **Step 6: Run the tests, verify they pass**

Run: `go test ./pkg/ui/ -run 'TestFooterToastRightZone|TestFooterBellBadge|TestFooterData_StatusBarOverride' -v && go build ./... && go vet ./...`
Expected: PASS; build/vet clean.

- [ ] **Step 7: Commit**

```bash
git add pkg/ui/model_footer.go pkg/ui/model_footer_test.go
git commit -m "feat(tui): footer toast right-zone override + permanent bell badge (bt-a3zi3.1)"
```

---

### Task 7: Clear the bell on opening the notifications view

Opening the notifications view advances `alertsSeenAt`, zeroing the footer bell without dismissing modal items.

**Files:**
- Modify: `pkg/ui/model_footer.go` (add `markNotificationsSeen` near the setters) or `model.go`
- Modify: `pkg/ui/model_update_input.go` (3 activation sites: ~253, ~270, ~1226)
- Test: `pkg/ui/model_footer_test.go`

**Interfaces:**
- Produces: `func (m *Model) markNotificationsSeen()` - sets `m.alertsSeenAt = time.Now()`.

- [ ] **Step 1: Write the failing test**

In `pkg/ui/model_footer_test.go`:

```go
func TestMarkNotificationsSeenClearsBell(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil)
	m.events.Append(events.NewSystemEvent("something happened"))
	if m.footerData().BellCount == 0 {
		t.Fatal("precondition: an unseen event should bump the bell")
	}
	m.markNotificationsSeen()
	if got := m.footerData().BellCount; got != 0 {
		t.Errorf("BellCount after mark-seen = %d, want 0", got)
	}
}
```

(import `github.com/seanmartinsmith/beadstui/pkg/ui/events` in the test file if needed.)

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./pkg/ui/ -run TestMarkNotificationsSeenClearsBell -v`
Expected: FAIL - `m.markNotificationsSeen undefined`.

- [ ] **Step 3: Implement markNotificationsSeen**

In `pkg/ui/model_footer.go`, after `clearStatus` (Task 3):

```go
// markNotificationsSeen advances the footer bell's high-water-mark so the
// badge clears, without dismissing any modal items (seen != dismissed,
// bt-a3zi3.1).
func (m *Model) markNotificationsSeen() {
	m.alertsSeenAt = time.Now()
}
```

- [ ] **Step 4: Call it at the three TabNotifications activation sites**

In `pkg/ui/model_update_input.go`, add `m.markNotificationsSeen()` immediately after each `m.activeTab = TabNotifications` assignment:
- line ~253 (mouse / tab click path)
- line ~270 (tab-toggle key path)
- line ~1226 (the global "1" key handler)

Example (line ~1226):

```go
			m.activeTab = TabNotifications
			m.markNotificationsSeen()
			m.openModal(ModalAlerts)
```

Run `grep -n "activeTab = TabNotifications" pkg/ui/model_update_input.go` to confirm all three are covered.

- [ ] **Step 5: Run the test + build/vet**

Run: `go test ./pkg/ui/ -run TestMarkNotificationsSeenClearsBell -v && go build ./... && go vet ./...`
Expected: PASS; build/vet clean.

- [ ] **Step 6: Commit**

```bash
git add pkg/ui/model_footer.go pkg/ui/model_update_input.go pkg/ui/model_footer_test.go
git commit -m "feat(tui): clear footer bell on opening notifications view (bt-a3zi3.1)"
```

---

### Task 8: Render-harness scenarios + never-wrap integration

Lock the visual behavior across widths and confirm the toast + bell never break the one-row invariant.

**Files:**
- Modify: `pkg/ui/render_harness_test.go` (add scenarios)
- Modify: `pkg/ui/model_footer_test.go` (add a never-wrap-with-notifications test)

- [ ] **Step 1: Add a never-wrap integration test**

In `pkg/ui/model_footer_test.go`, mirroring `TestFooterPinnedToBottomRow`:

```go
func TestFooterNotificationsNeverWrap(t *testing.T) {
	widths := []int{60, 70, 80, 100, 120, 160}
	setups := map[string]func(*Model){
		"idle": func(m *Model) {},
		"success": func(m *Model) { m.setStatus("reloaded +3 -1") },
		"failure": func(m *Model) { m.setFailure("write failed: db locked") },
		"degraded": func(m *Model) { m.setDegraded("Dolt server unreachable (retrying in 5s)") },
		"bell": func(m *Model) {
			for i := 0; i < 4; i++ {
				m.events.Append(events.NewSystemEvent("event"))
			}
		},
	}
	for name, setup := range setups {
		for _, w := range widths {
			t.Run(fmt.Sprintf("%s_%d", name, w), func(t *testing.T) {
				m := NewModel(harnessIssues(), nil, "", nil)
				nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 24})
				m = nm.(Model)
				setup(&m)
				footer := ansi.Strip(m.renderFooter())
				if strings.Contains(footer, "\n") {
					t.Fatalf("footer wrapped at width %d: %q", w, footer)
				}
				if got := lipgloss.Width(m.renderFooter()); got > w {
					t.Errorf("footer width %d exceeds terminal %d", got, w)
				}
			})
		}
	}
}
```

(imports: `fmt`, `strings`, `tea "charm.land/bubbletea/v2"`, `lipgloss "charm.land/lipgloss/v2"`, `github.com/charmbracelet/x/ansi`, `github.com/seanmartinsmith/beadstui/pkg/ui/events` - add any missing.)

- [ ] **Step 2: Run it, verify it passes**

Run: `go test ./pkg/ui/ -run TestFooterNotificationsNeverWrap -v`
Expected: PASS at all widths and severities. If a width fails, the degradation/truncate in Task 6 needs adjustment - fix before proceeding.

- [ ] **Step 3: Add render-harness dump scenarios**

In `pkg/ui/render_harness_test.go`, add to the `scenarios` table (after line ~269):

```go
		{"footer_success_100x24", 100, 24, func(m *Model) { m.setStatus("reloaded +3 -1") }},
		{"footer_failure_100x24", 100, 24, func(m *Model) { m.setFailure("write failed: db locked") }},
		{"footer_degraded_80x24", 80, 24, func(m *Model) { m.setDegraded("Dolt server unreachable (retrying in 5s)") }},
		{"footer_bell_100x24", 100, 24, func(m *Model) {
			for i := 0; i < 3; i++ {
				m.events.Append(events.NewSystemEvent("activity"))
			}
		}},
		{"footer_bell_60x24", 60, 24, func(m *Model) {
			for i := 0; i < 3; i++ {
				m.events.Append(events.NewSystemEvent("activity"))
			}
		}},
```

(ensure `events` is imported in the harness test file.)

- [ ] **Step 4: Generate and eyeball the dumps**

Run: `BT_RENDER_DUMP=1 go test ./pkg/ui/ -run TestRenderHarness -v`
Expected: writes `footer_*.txt` to the dump dir. Read each: confirm the toast sits in the right zone with the correct glyph, the bell is rightmost, `?` survives, and nothing clips.

- [ ] **Step 5: Full suite + install**

Run: `go test ./... && go build ./... && go vet ./... && go install ./cmd/bt/`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add pkg/ui/render_harness_test.go pkg/ui/model_footer_test.go
git commit -m "test(tui): footer notification render scenarios + never-wrap sweep (bt-a3zi3.1)"
```

---

## Self-Review

**Spec coverage:**
- Two channels (toast vs bell) → Tasks 3, 6. ✓
- Toast taxonomy (Success/Notice/Failure/Degraded; glyph/lifetime/bell) → Tasks 2 (glyph), 3 (timing + bell). ✓
- Severity classification of 41 sites → Task 4. ✓
- Sticky-until-resolved (Degraded self-clear) → Task 5. ✓
- Bell seen-vs-dismissed via `alertsSeenAt`, boot-init 0, open clears footer not modal → Tasks 1 (UnseenCount), 2 (field), 6 (render), 7 (mark-seen). ✓
- Always render 🔔, N only when >0, pinned → Task 6. ✓
- Never-wrap / footer-pin invariants → Task 8. ✓
- Out-of-scope (cross-session `bt-vhzia`, multi-select `bt-s2duc`) → not built here, tracked. ✓

**Deferred decisions resolved in this plan:**
- Setter API shape: separate `setNotice`/`setFailure`/`setDegraded` (not a single `setToast(sev)`) - matches the existing setter style and keeps the 41-site reclassification a one-token change per site.
- Degraded recovery hook: `handleSnapshotReady` (Task 5).
- Tab clearing: the notifications-tab activation sites (Task 7); the alerts tab does not clear the bell (the bell counts notifications).

**Placeholder scan:** none - every code step has concrete code. Two steps carry explicit "inspect the real struct shape before running" notes (Task 5 `SnapshotReadyMsg`, Task 3 unused-constant check) because those depend on code not fully quoted here; both give the exact check to run.

**Type consistency:** `StatusSeverity`/`SeverityNone..Degraded`, `statusSeverity`, `alertsSeenAt`, `BellCount`, `UnseenCount`, `NewSystemEvent`, `statusDismissAge`, `setNotice/setFailure/setDegraded/clearStatus/markNotificationsSeen` are used identically across tasks.
