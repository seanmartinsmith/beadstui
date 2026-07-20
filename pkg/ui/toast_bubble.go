package ui

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Yazi-style floating notification bubble (bt-kuvzj), replacing the
// statusline-embedded toast (Phase 4 of bt-a3zi3.1, docs/design/2026-06-22-
// footer-phase4-notifications.md). model_footer.go's Render() no longer
// borrows the footer's right zone for notification content ("Toast
// override" block, removed); this file owns the bubble that took its place.
//
// Footer-speaks policy (bt-c3gpe): this bubble is the SINGLE surface for a
// transient status. The footer renders StatusMsg only as the non-inline
// full-width banner (errors / explicit confirmations); an inline status
// (StatusIsInline) has zero footer footprint. So a transient paints here and
// nowhere else - no double surface.
//
// This is a PLACEMENT redesign only. The trigger conditions, data
// (m.statusMsg / m.statusSeverity / m.statusIsInline), and timing/dismiss
// semantics (statusDismissAge, the Degraded recovery path) are all
// unchanged — see model_footer.go's setStatus/setFailure/setDegraded/
// setNotice and model.go's handleStatusTick/handleStatusClear. bt-msxk
// (mutation-feedback pattern) and bt-p4p8 (notification-center data model)
// own any future changes to what triggers a toast or what data it carries.
//
// Fixes bt-8scek's root cause: the embedded toast borrowed the footer's
// right zone and was ansi.Truncate'd to whatever columns the badge stack
// left over, clipping long bdroute refusal messages mid-remedy. The bubble
// instead wraps at a fixed content width via ansi.Wrap and grows vertically
// — no line is ever cut.

const (
	// toastBubbleMaxContentWidth caps the bubble's interior (wrap) width so
	// it reads as a compact yazi-style notification rather than spanning
	// wide terminals. Message text wraps inside this budget instead of
	// truncating (bt-8scek).
	toastBubbleMaxContentWidth = 40
	// toastBubbleMinContentWidth floors the interior width so short
	// messages ("reloaded +3 -1") still get a legible bordered bubble, not a
	// sliver.
	toastBubbleMinContentWidth = 12
	// toastBubbleMarginRight/Bottom position the bubble off the terminal's
	// bottom-right corner, just above the footer row (bt-kuvzj: "floating
	// above content just over the footer line").
	toastBubbleMarginRight  = 2
	toastBubbleMarginBottom = 0
)

// toastBorderGlyph returns the type-signaling glyph for the bubble's top
// border (bt-kuvzj acceptance: "border encodes type/count"). Mirrors
// StatusSeverity.glyph() (model_footer.go) but additionally gives Notice an
// icon: the removed embedded toast rendered Notice glyph-less (plain inline
// text was enough there), but a glyphless bubble title reads as broken
// chrome rather than "low urgency" on a freestanding bordered panel. Uses
// activeGlyphs.Info — an existing table field, previously unused by any
// severity mapping — so this needs no new glyph role.
func toastBorderGlyph(sev StatusSeverity) string {
	switch sev {
	case SeveritySuccess:
		return activeGlyphs.Success
	case SeverityFailure:
		return activeGlyphs.Cross
	case SeverityDegraded:
		return activeGlyphs.Warning
	case SeverityNotice:
		return activeGlyphs.Info
	default:
		return ""
	}
}

// toastBorderColor maps severity to the bubble's border/title color,
// mirroring the color switch in the removed embedded-toast render path.
func toastBorderColor(sev StatusSeverity) color.Color {
	switch sev {
	case SeverityFailure:
		return ColorPrioCritical
	case SeverityDegraded:
		return ColorWarning
	case SeverityNotice:
		return ColorMuted
	default: // Success
		return ColorSuccess
	}
}

// toastBubbleContentWidth picks the wrap width for a bubble on a terminal
// termWidth cells wide: the max budget, shrunk to fit narrow terminals, never
// below the min legibility floor.
func toastBubbleContentWidth(termWidth int) int {
	avail := termWidth - toastBubbleMarginRight - 2 /* borders */ - 2 /* left breathing room */
	w := toastBubbleMaxContentWidth
	if avail < w {
		w = avail
	}
	if w < toastBubbleMinContentWidth {
		w = toastBubbleMinContentWidth
	}
	return w
}

// buildToastBubble renders msg as a bordered bubble: the border encodes
// severity (title glyph) and the unseen-bell count (right label), and the
// message wraps at the terminal-appropriate width rather than truncating
// (bt-8scek). The bubble shrinks to the longest wrapped line's actual width
// instead of always claiming the full wrap budget, so short messages
// ("reloaded +3 -1") stay compact.
func buildToastBubble(msg string, sev StatusSeverity, bellCount, termWidth int) string {
	wrapLimit := toastBubbleContentWidth(termWidth)
	wrapped := ansi.Wrap(msg, wrapLimit, "")

	innerWidth := 0
	for _, line := range strings.Split(wrapped, "\n") {
		if w := ansi.StringWidth(line); w > innerWidth {
			innerWidth = w
		}
	}
	if innerWidth < toastBubbleMinContentWidth {
		innerWidth = toastBubbleMinContentWidth
	}
	if innerWidth > wrapLimit {
		innerWidth = wrapLimit
	}

	borderColor := toastBorderColor(sev)
	rightLabel := ""
	if bellCount > 0 {
		rightLabel = fmt.Sprintf("%s%d", activeGlyphs.Bell, bellCount)
	}

	return RenderTitledPanel(wrapped, PanelOpts{
		Title:       toastBorderGlyph(sev),
		RightLabel:  rightLabel,
		Width:       innerWidth + 2,
		BorderColor: borderColor,
		TitleColor:  borderColor,
	})
}

// renderToastBubble returns the floating notification bubble for the
// Model's active toast, or "" when no toast is active — the same trigger
// condition the removed footer override used (StatusMsg non-empty and
// inline). bgWidth sizes the wrap; the caller composites the result via
// OverlayBottomRight over the body region, just above the footer line.
func (m Model) renderToastBubble(bgWidth int) string {
	if m.statusMsg == "" || !m.statusIsInline {
		return ""
	}
	bellCount := 0
	if m.events != nil {
		bellCount = m.unseenNotificationCount(m.alertsSeenAt)
	}
	return buildToastBubble(m.statusMsg, m.statusSeverity, bellCount, bgWidth)
}
