// pkg/ui/events/events_test.go
package events

import "testing"

func TestEventKindString(t *testing.T) {
	cases := []struct {
		kind EventKind
		want string
	}{
		{EventCreated, "created"},
		{EventEdited, "edited"},
		{EventClosed, "closed"},
		{EventCommented, "commented"},
	}
	for _, c := range cases {
		if got := c.kind.String(); got != c.want {
			t.Errorf("EventKind(%d).String() = %q, want %q", c.kind, got, c.want)
		}
	}
}

func TestEventSourceString(t *testing.T) {
	if SourceDolt.String() != "dolt" {
		t.Errorf("SourceDolt.String() = %q, want %q", SourceDolt.String(), "dolt")
	}
	if SourceCass.String() != "cass" {
		t.Errorf("SourceCass.String() = %q, want %q", SourceCass.String(), "cass")
	}
}
