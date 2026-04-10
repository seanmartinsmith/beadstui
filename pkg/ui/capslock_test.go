package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewCapsLockTracker(t *testing.T) {
	tracker := NewCapsLockTracker()
	if tracker == nil {
		t.Fatal("NewCapsLockTracker returned nil")
	}
	if tracker.threshold != 300*time.Millisecond {
		t.Errorf("Expected 300ms threshold, got %v", tracker.threshold)
	}
	if tracker.pending {
		t.Error("New tracker should not be pending")
	}
}

func TestNewCapsLockTrackerWithThreshold(t *testing.T) {
	threshold := 500 * time.Millisecond
	tracker := NewCapsLockTrackerWithThreshold(threshold)
	if tracker.threshold != threshold {
		t.Errorf("Expected %v threshold, got %v", threshold, tracker.threshold)
	}
}

func TestCapsLockTracker_SinglePress(t *testing.T) {
	tracker := NewCapsLockTracker()

	// First press should start timer, not trigger anything immediately
	trigger, cmd := tracker.HandlePress()
	if trigger != TriggerNone {
		t.Errorf("First press should return TriggerNone, got %v", trigger)
	}
	if cmd == nil {
		t.Error("First press should return a timer command")
	}
	if !tracker.pending {
		t.Error("Tracker should be pending after first press")
	}
}

func TestCapsLockTracker_TimerExpired(t *testing.T) {
	tracker := NewCapsLockTracker()

	// Press and wait for timer
	tracker.HandlePress()

	// Timer expires
	trigger := tracker.HandleTimerExpired()
	if trigger != TriggerFullTutorial {
		t.Errorf("Timer expiration should trigger full tutorial, got %v", trigger)
	}
	if tracker.pending {
		t.Error("Tracker should not be pending after timer expiration")
	}
}

func TestCapsLockTracker_DoubleTap(t *testing.T) {
	// Use a longer threshold for test reliability
	tracker := NewCapsLockTrackerWithThreshold(500 * time.Millisecond)

	// First press
	tracker.HandlePress()

	// Second press quickly (should be double-tap)
	trigger, cmd := tracker.HandlePress()
	if trigger != TriggerContextHelp {
		t.Errorf("Double-tap should trigger context help, got %v", trigger)
	}
	if cmd != nil {
		t.Error("Double-tap should not return a timer command")
	}
	if tracker.pending {
		t.Error("Tracker should not be pending after double-tap")
	}
}

func TestCapsLockTracker_SlowDoubleTap(t *testing.T) {
	// Use a very short threshold for test
	tracker := NewCapsLockTrackerWithThreshold(1 * time.Millisecond)

	// First press
	tracker.HandlePress()

	// Wait longer than threshold
	time.Sleep(5 * time.Millisecond)

	// Second press after threshold - should be new single tap, not double
	// First handle the expired timer
	tracker.HandleTimerExpired()

	// New press
	trigger, cmd := tracker.HandlePress()
	if trigger != TriggerNone {
		t.Errorf("Slow second press should be new single tap, got %v", trigger)
	}
	if cmd == nil {
		t.Error("Should return timer command for new single tap")
	}
}

func TestCapsLockTracker_Reset(t *testing.T) {
	tracker := NewCapsLockTracker()

	// Press to make pending
	tracker.HandlePress()
	if !tracker.pending {
		t.Error("Should be pending")
	}

	// Reset
	tracker.Reset()
	if tracker.pending {
		t.Error("Should not be pending after reset")
	}
	if !tracker.lastPress.IsZero() {
		t.Error("lastPress should be zero after reset")
	}
}

func TestCapsLockTracker_IsPending(t *testing.T) {
	tracker := NewCapsLockTracker()

	if tracker.IsPending() {
		t.Error("New tracker should not be pending")
	}

	tracker.HandlePress()
	if !tracker.IsPending() {
		t.Error("Should be pending after press")
	}

	tracker.HandleTimerExpired()
	if tracker.IsPending() {
		t.Error("Should not be pending after timer expired")
	}
}

func TestCapsLockTracker_TimerExpiredWhenNotPending(t *testing.T) {
	tracker := NewCapsLockTracker()

	// Timer expires when not pending (shouldn't happen but handle gracefully)
	trigger := tracker.HandleTimerExpired()
	if trigger != TriggerNone {
		t.Errorf("Timer expired when not pending should return TriggerNone, got %v", trigger)
	}
}

func TestTutorialTrigger_String(t *testing.T) {
	tests := []struct {
		trigger  TutorialTrigger
		expected string
	}{
		{TriggerNone, "none"},
		{TriggerFullTutorial, "full tutorial"},
		{TriggerContextHelp, "context help"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.trigger.String(); got != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestIsCapsLock(t *testing.T) {
	// CapsLock is not reliably detectable, so function should be conservative
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	if IsCapsLock(msg) {
		t.Error("Regular key should not be detected as CapsLock")
	}

	// Empty runes case
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{}}
	if IsCapsLock(msg) {
		t.Error("Empty runes should not be detected as CapsLock (too ambiguous)")
	}
}

func TestIsTutorialTrigger(t *testing.T) {
	// Backtick should be the trigger
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'`'}}
	if !IsTutorialTrigger(msg) {
		t.Error("Backtick should be tutorial trigger")
	}

	// Other keys should not
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	if IsTutorialTrigger(msg) {
		t.Error("'a' should not be tutorial trigger")
	}
}

func TestIsContextHelpTrigger(t *testing.T) {
	// Tilde should be the trigger
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'~'}}
	if !IsContextHelpTrigger(msg) {
		t.Error("Tilde should be context help trigger")
	}

	// Other keys should not
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	if IsContextHelpTrigger(msg) {
		t.Error("'a' should not be context help trigger")
	}
}

func TestGetTriggerKeyHint(t *testing.T) {
	hint := GetTriggerKeyHint()
	if hint == "" {
		t.Error("Trigger key hint should not be empty")
	}
	// Should contain both trigger keys
	if !stringContains(hint, TutorialTriggerKey) {
		t.Error("Hint should contain tutorial trigger key")
	}
	if !stringContains(hint, ContextHelpTriggerKey) {
		t.Error("Hint should contain context help trigger key")
	}
}

func TestDefaultTutorialKeyBindings(t *testing.T) {
	bindings := DefaultTutorialKeyBindings()
	if bindings.DirectTutorial != TutorialTriggerKey {
		t.Errorf("Expected direct tutorial key %q, got %q", TutorialTriggerKey, bindings.DirectTutorial)
	}
	if bindings.ContextHelp != ContextHelpTriggerKey {
		t.Errorf("Expected context help key %q, got %q", ContextHelpTriggerKey, bindings.ContextHelp)
	}
	if !bindings.HelpModalSpace {
		t.Error("HelpModalSpace should be enabled by default")
	}
	if !bindings.DoubleTapEnable {
		t.Error("DoubleTapEnable should be enabled by default")
	}
}

func TestTutorialKeyBindings_IsDirectTutorial(t *testing.T) {
	bindings := DefaultTutorialKeyBindings()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'`'}}
	if !bindings.IsDirectTutorial(msg) {
		t.Error("Backtick should match direct tutorial binding")
	}

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	if bindings.IsDirectTutorial(msg) {
		t.Error("'x' should not match direct tutorial binding")
	}
}

func TestTutorialKeyBindings_IsContextHelp(t *testing.T) {
	bindings := DefaultTutorialKeyBindings()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'~'}}
	if !bindings.IsContextHelp(msg) {
		t.Error("Tilde should match context help binding")
	}

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	if bindings.IsContextHelp(msg) {
		t.Error("'x' should not match context help binding")
	}
}

func TestTutorialKeyBindings_CustomBinding(t *testing.T) {
	bindings := TutorialKeyBindings{
		DirectTutorial: "t",
		ContextHelp:    "T",
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
	if !bindings.IsDirectTutorial(msg) {
		t.Error("Custom 't' binding should work")
	}

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}}
	if !bindings.IsContextHelp(msg) {
		t.Error("Custom 'T' binding should work")
	}
}

// Helper function
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
