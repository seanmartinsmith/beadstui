# TUI modal compositing

All modal overlays must dim the backdrop via `OverlayCenterDimBackdrop`
(in `pkg/ui/panel.go`). This makes the modal read as a true pop-up
rather than a content-shaped panel embedded in the underlying view, and
is the unified aesthetic across alerts, pickers, and prompts (bt-v8he,
bt-o1hs).

The non-dim `OverlayCenter` is reserved for non-modal overlays only -
debug panels, transient hints, or anything where the user is meant to
keep reading the underlying view through the overlay.

## Adding a new modal

1. Expose the modal's content as a `View()` method on its struct (no
   centering, no overlay logic - just the bordered/styled panel).
2. In `pkg/ui/model_view.go`, leave the `activeModal` switch case as a
   fall-through comment ("Handled as overlay after background renders
   (below)") so the underlying view renders into `body` first.
3. Add an overlay block at the bottom of `View()` that calls
   `OverlayCenterDimBackdrop(body, m.foo.View(), m.width, m.height-1)`.

This matches the alerts/notifications modal shape and keeps a single
canonical compositor for modal pop-ups.
