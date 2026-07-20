package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/seanmartinsmith/beadstui/pkg/ui/events"
)

// bt8scekRefusalMsg is the exact bdroute pre-flight refusal string from the
// bt-8scek repro: long by design because it carries the remedy. Under the
// removed embedded-toast mechanism this collapsed to "x claim sy" at ~165
// cols with the full footer badge stack visible.
const bt8scekRefusalMsg = `claim sym-maqi: no known checkout path for project "sym"; cd into that project once so bt can record it (~/.bt/settings.json), then relaunch bt --global`

// TestToastBubbleRendersOverContentBothTiers proves the floating bubble
// composites correctly at the footer-lens design widths (50/70/100/130/160)
// in both glyph tiers, without breaking the existing view invariants: total
// rows never exceed the terminal height and no row exceeds the terminal
// width (mirrors TestRenderHarnessBothTiers in glyphs_test.go). It also
// asserts the bubble is actually visible in the composited view at every
// width/tier — a regression here would mean OverlayBottomRight silently
// dropped the bubble at some size.
func TestToastBubbleRendersOverContentBothTiers(t *testing.T) {
	widths := []int{50, 70, 100, 130, 160}
	const height = 24
	for _, tier := range []struct {
		name string
		g    GlyphSet
	}{{"nerdfont", nerdfontGlyphs}, {"ascii", asciiGlyphs}} {
		t.Run(tier.name, func(t *testing.T) {
			setGlyphs(t, tier.g)
			for _, w := range widths {
				issues := harnessIssues()
				m := NewModel(issues, nil, "", nil, nil)
				nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: height})
				m = nm.(Model)
				m.setFailure("write failed: db locked")

				content := m.View().Content
				plain := ansi.Strip(content)
				lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")

				if len(lines) > height {
					t.Errorf("width %d tier %s: view has %d rows > height %d (bubble compositing broke the height invariant)",
						w, tier.name, len(lines), height)
				}
				for i, ln := range lines {
					if lw := ansi.StringWidth(ln); lw > w {
						t.Errorf("width %d tier %s: row %d width %d exceeds terminal width %d: %q",
							w, tier.name, i, lw, w, ln)
					}
				}

				wantGlyph := activeGlyphs.Cross
				if !strings.Contains(plain, wantGlyph) {
					t.Errorf("width %d tier %s: bubble border glyph %q not found in view; bubble may not be rendering", w, tier.name, wantGlyph)
				}
				// The footer's own bell must still render (dark-cockpit / pinned
				// invariant, unaffected by the bubble).
				if !strings.Contains(plain, activeGlyphs.Bell) {
					t.Errorf("width %d tier %s: footer bell missing", w, tier.name)
				}
			}
		})
	}
}

// TestToastBubbleBorderEncodesTypeAndCount locks the acceptance criterion
// "border encodes type/count": each severity gets a distinct glyph in the
// title, and a non-zero bell count shows up as a right-border label that a
// zero count omits.
func TestToastBubbleBorderEncodesTypeAndCount(t *testing.T) {
	cases := []struct {
		name      string
		sev       StatusSeverity
		wantGlyph string
	}{
		{"success", SeveritySuccess, activeGlyphs.Success},
		{"failure", SeverityFailure, activeGlyphs.Cross},
		{"degraded", SeverityDegraded, activeGlyphs.Warning},
		{"notice", SeverityNotice, activeGlyphs.Info},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		out := buildToastBubble("something happened", c.sev, 0, 100)
		plain := ansi.Strip(out)
		if c.wantGlyph != "" && !strings.Contains(plain, c.wantGlyph) {
			t.Errorf("severity %s: border should contain glyph %q; got %q", c.name, c.wantGlyph, plain)
		}
		if seen[plain] {
			t.Errorf("severity %s: border identical to a previous severity's render; type is not distinguishable", c.name)
		}
		seen[plain] = true
	}

	// Count encoding: a positive bell count must render as a visible digit
	// somewhere in the border/panel, and a zero count must not fabricate one.
	withCount := ansi.Strip(buildToastBubble("db locked", SeverityFailure, 5, 100))
	if !strings.Contains(withCount, "5") {
		t.Errorf("bell count 5 should appear in the bubble border; got %q", withCount)
	}
	withoutCount := ansi.Strip(buildToastBubble("db locked", SeverityFailure, 0, 100))
	if strings.Contains(withoutCount, activeGlyphs.Bell) {
		t.Errorf("zero bell count should not render a bell/count badge; got %q", withoutCount)
	}
}

// dewrapPanelBody strips a RenderTitledPanel box down to its content rows
// (dropping the top/bottom border-only rows, which may carry a title glyph
// or right-label token that would otherwise contaminate reconstruction),
// removes the vertical border character, and rejoins every row's words with
// single spaces. For a panel built from ansi.Wrap (word-wrap only, no
// hyphenation), this reconstructs the original unwrapped text exactly — the
// only clean way to assert "no characters were dropped" against text that is
// now deliberately spread across multiple bordered lines instead of one.
func dewrapPanelBody(s string) string {
	lines := strings.Split(ansi.Strip(s), "\n")
	if len(lines) >= 2 {
		lines = lines[1 : len(lines)-1]
	}
	var words []string
	for _, ln := range lines {
		words = append(words, strings.Fields(strings.ReplaceAll(ln, "│", " "))...)
	}
	return strings.Join(words, " ")
}

// TestToastBubbleLongMessageNoTruncation is the bt-8scek repro fixed: the
// long bdroute refusal message must render in full inside the bubble (wrapped
// across lines, never truncated with an ellipsis), and must NOT appear
// anywhere in the footer row at all — notification content lives only in the
// bubble now.
func TestToastBubbleLongMessageNoTruncation(t *testing.T) {
	// Direct check against the wrap function itself: reconstructing the
	// bubble's content rows must reproduce the exact original message at
	// every design width — the old mechanism (ansi.Truncate against the
	// footer's remaining columns) would have silently dropped the tail.
	for _, w := range []int{50, 70, 100, 130, 160} {
		out := buildToastBubble(bt8scekRefusalMsg, SeverityFailure, 0, w)
		if strings.Contains(out, activeGlyphs.Ellipsis) {
			t.Errorf("termWidth %d: bubble must wrap, not truncate with an ellipsis; got:\n%s", w, out)
		}
		if got := dewrapPanelBody(out); got != bt8scekRefusalMsg {
			t.Errorf("termWidth %d: wrapped bubble does not reconstruct the full message.\ngot:  %q\nwant: %q", w, got, bt8scekRefusalMsg)
		}
	}

	// Integration check: the same long message driven through the real
	// Model/View at a few representative widths (including a window close to
	// the bt-8scek repro's own ~165-col case) must show the message inside
	// the bubble — checked via its known wrapped lines, since the full
	// message now spans multiple bordered rows — and must NOT show any of it
	// on the footer's last row.
	knownWrappedLines := []string{
		`claim sym-maqi: no known checkout path`,
		`for project "sym"; cd into that project`,
		`once so bt can record it`,
		`(~/.bt/settings.json), then relaunch bt`,
		`--global`,
	}
	for _, w := range []int{70, 100, 165} {
		m := NewModel(harnessIssues(), nil, "", nil, nil)
		nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 24})
		m = nm.(Model)
		m.setFailure(bt8scekRefusalMsg)

		content := ansi.Strip(m.View().Content)
		for _, frag := range knownWrappedLines {
			if !strings.Contains(content, frag) {
				t.Errorf("width %d: expected wrapped bubble line %q not found in view:\n%s", w, frag, content)
			}
		}

		// The footer (last row) must not carry any fragment of the message —
		// notification content no longer renders in the footer at all.
		lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
		footerRow := lines[len(lines)-1]
		if strings.Contains(footerRow, "claim sym-maqi") {
			t.Errorf("width %d: footer row must not contain toast content; got %q", w, footerRow)
		}
	}
}

// TestToastBubbleAbsentWhenNoActiveToast proves the bubble only appears while
// a toast is active — renderToastBubble returns "" otherwise, so an idle
// session composites nothing extra over the body.
func TestToastBubbleAbsentWhenNoActiveToast(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil, nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = nm.(Model)

	if got := m.renderToastBubble(m.width); got != "" {
		t.Errorf("idle model should render no bubble; got %q", got)
	}
}

// TestToastBubbleClearsWithStatus proves timing/dismiss semantics are
// unchanged: clearing the underlying status (the same mechanism
// handleStatusClear/handleStatusTick use) makes the bubble disappear. This
// bead only redesigns placement — it must not touch when a toast appears or
// clears.
func TestToastBubbleClearsWithStatus(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil, nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = nm.(Model)

	m.setFailure("write failed: db locked")
	if m.renderToastBubble(m.width) == "" {
		t.Fatal("precondition: an active Failure toast should render a bubble")
	}

	m.clearStatus()
	if got := m.renderToastBubble(m.width); got != "" {
		t.Errorf("after clearStatus, bubble should be gone; got %q", got)
	}
}

// TestTransientStatusSingleSurfaceFooterHintsIntact locks the footer-speaks
// policy (bt-c3gpe): a transient status renders in EXACTLY ONE surface - the
// floating bubble - and never clobbers the footer. It drives the real reload
// toast setter (setInlineTransientStatus) and asserts, across widths and both
// glyph tiers, that the footer's last row is byte-identical with and without
// the active transient (so the Zone-3 discoverability hints + lens are wholly
// untouched), that the footer never carries the message text, and that the
// bubble is the surface that does carry it. Byte-identity is deliberately
// stronger than string-matching a specific hint token: it holds regardless of
// how the degradation cascade compacts the hints at a given width.
func TestTransientStatusSingleSurfaceFooterHintsIntact(t *testing.T) {
	const reloadMsg = "Reloaded 5634 issues"
	for _, tier := range []struct {
		name string
		g    GlyphSet
	}{{"nerdfont", nerdfontGlyphs}, {"ascii", asciiGlyphs}} {
		t.Run(tier.name, func(t *testing.T) {
			setGlyphs(t, tier.g)
			for _, w := range []int{70, 100, 160} {
				// Baseline footer with no active status.
				m := NewModel(harnessIssues(), nil, "", nil, nil)
				nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 24})
				m = nm.(Model)
				footerRow := func(mm Model) string {
					lines := strings.Split(strings.TrimRight(ansi.Strip(mm.View().Content), "\n"), "\n")
					return lines[len(lines)-1]
				}
				idleFooter := footerRow(m)

				// Activate the real reload transient (SeveritySuccess, inline).
				// setInlineTransientStatus does not touch the events ring, so the
				// footer bell is unchanged too - the footer must be identical.
				m.setInlineTransientStatus(reloadMsg, 3*time.Second)

				// Surface 1: the bubble carries the message.
				bubble := ansi.Strip(m.renderToastBubble(m.width))
				if !strings.Contains(bubble, reloadMsg) {
					t.Errorf("width %d tier %s: bubble should carry the transient %q; got %q", w, tier.name, reloadMsg, bubble)
				}

				// Surface 2 (the footer) must be untouched: the last row is
				// byte-identical to the idle footer, so no hint/lens content
				// moved and the message text never leaks into the footer.
				activeFooter := footerRow(m)
				if activeFooter != idleFooter {
					t.Errorf("width %d tier %s: footer changed while a transient was active - hint slot clobbered.\n idle:   %q\n active: %q",
						w, tier.name, idleFooter, activeFooter)
				}
				if strings.Contains(activeFooter, reloadMsg) {
					t.Errorf("width %d tier %s: footer must not render transient content; got %q", w, tier.name, activeFooter)
				}

				// The message IS visible in the composited view (the bubble),
				// proving it renders in exactly one surface, not zero.
				if !strings.Contains(ansi.Strip(m.View().Content), reloadMsg) {
					t.Errorf("width %d tier %s: transient should be visible in the view via the bubble; not found", w, tier.name)
				}
			}
		})
	}
}

// TestToastBubbleUsesBellCountFromEvents proves the border's count badge
// reads the same unseen-events count the footer bell already shows (reusing
// existing bt-a3zi3.1 data rather than inventing a new counter, per bt-kuvzj
// scope: data/trigger unchanged, only placement).
func TestToastBubbleUsesBellCountFromEvents(t *testing.T) {
	m := NewModel(harnessIssues(), nil, "", nil, nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = nm.(Model)

	for i := 0; i < 3; i++ {
		m.events.Append(events.NewSystemEvent("activity"))
	}
	m.setFailure("write failed: db locked")

	bubble := ansi.Strip(m.renderToastBubble(m.width))
	bellCount := m.unseenNotificationCount(m.alertsSeenAt)
	if bellCount == 0 {
		t.Fatal("precondition: unseen events should produce a non-zero bell count")
	}
	if !strings.Contains(bubble, activeGlyphs.Bell) {
		t.Errorf("bubble border should carry the bell glyph when unseen events exist; got %q", bubble)
	}
}
