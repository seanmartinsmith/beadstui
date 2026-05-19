package ui

import (
	"fmt"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/seanmartinsmith/beadstui/pkg/debug"
)

// ResizeCoalescer suppresses intermediate tea.WindowSizeMsg events during a
// terminal-resize burst and re-emits only the final size once the burst
// settles. See bt-kfkrb for the back-pressure analysis: Bubble Tea's
// eventLoop runs p.renderer.resize synchronously per WindowSizeMsg before
// any user-supplied filter, which forces the next renderer flush to do a
// full-screen redraw (cursed_renderer.go:289). For bt's full-screen
// content (~22000 cells at full WT width) each redraw pushes ~100 KB
// through ConPTY, and the resulting back-pressure throttles incoming
// resize events, producing the visible 25-45 s drag-fill latency.
//
// The Filter method runs BEFORE p.renderer.resize, so returning nil
// suppresses both the engine-level redraw and our handleWindowSize work
// for that event. RunFlushPump runs a background ticker that re-emits the
// latest stashed size via p.Send once a quiet window of `burstSettle` has
// passed; this guarantees the final size always reaches the model so the
// app never sticks at a stale dimension.
type ResizeCoalescer struct {
	mu           sync.Mutex
	burstSettle  time.Duration
	pumpEvery    time.Duration
	latestW      int
	latestH      int
	lastSeen     time.Time
	initialized  bool
	pending      bool
	passThroughN int // pump increments before re-emitting; filter decrements + lets the next N WindowSizeMsgs through
	send         func(tea.Msg)

	// debug counters
	droppedCount uint64
	passedCount  uint64
	pumpedCount  uint64
}

// NewResizeCoalescer constructs a coalescer with the given burst-settle
// window (how long to wait after the last incoming WindowSizeMsg before
// re-emitting the latest stashed size) and pump interval (how often the
// flush pump goroutine wakes to check).
func NewResizeCoalescer(burstSettle, pumpEvery time.Duration) *ResizeCoalescer {
	return &ResizeCoalescer{burstSettle: burstSettle, pumpEvery: pumpEvery}
}

// SetSender wires the tea.Program's Send method. Called once after
// tea.NewProgram so the pump can re-emit dropped events back into the
// program's message channel.
func (c *ResizeCoalescer) SetSender(send func(tea.Msg)) {
	c.mu.Lock()
	c.send = send
	c.mu.Unlock()
}

// Filter is the tea.WithFilter callback. The first WindowSizeMsg of a
// session passes through unchanged so the model receives its initial
// dimensions; subsequent WindowSizeMsgs are dropped and the latest
// dimensions are stashed for the pump to re-emit after the burst settles.
// Pump-originated re-emissions (signaled by passThroughN > 0) are passed
// through unmodified so the model never gets stuck at a stale size.
// Non-WindowSizeMsg messages pass through untouched.
func (c *ResizeCoalescer) Filter(_ tea.Model, msg tea.Msg) tea.Msg {
	ws, ok := msg.(tea.WindowSizeMsg)
	if !ok {
		return msg
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.passThroughN > 0 {
		c.passThroughN--
		c.latestW = ws.Width
		c.latestH = ws.Height
		c.pending = false
		c.passedCount++
		debug.LogTiming(fmt.Sprintf("coalesce.pumped w=%d h=%d totalPassed=%d", ws.Width, ws.Height, c.passedCount), 0)
		return msg
	}
	c.latestW = ws.Width
	c.latestH = ws.Height
	c.lastSeen = time.Now()
	if !c.initialized {
		c.initialized = true
		c.pending = false
		c.passedCount++
		debug.LogTiming(fmt.Sprintf("coalesce.initial w=%d h=%d", ws.Width, ws.Height), 0)
		return msg
	}
	c.pending = true
	c.droppedCount++
	if c.droppedCount%10 == 1 {
		// Log every 10th drop so the log isn't flooded but we still see the rate.
		debug.LogTiming(fmt.Sprintf("coalesce.dropped w=%d h=%d totalDropped=%d", ws.Width, ws.Height, c.droppedCount), 0)
	}
	return nil
}

// RunFlushPump starts the background re-emit goroutine. It runs until
// stop is closed. Each tick, if a dropped WindowSizeMsg is pending and
// burstSettle has elapsed since the last incoming event, the pump
// re-emits the latest stashed dimensions via the sender wired by
// SetSender.
func (c *ResizeCoalescer) RunFlushPump(stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(c.pumpEvery)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				c.mu.Lock()
				if !c.pending || now.Sub(c.lastSeen) < c.burstSettle {
					c.mu.Unlock()
					continue
				}
				w, h := c.latestW, c.latestH
				c.pending = false
				c.passThroughN++ // tell the filter to let our re-emit through
				c.pumpedCount++
				send := c.send
				pumped := c.pumpedCount
				c.mu.Unlock()
				debug.LogTiming(fmt.Sprintf("coalesce.pump_emit w=%d h=%d totalPumped=%d", w, h, pumped), 0)
				if send != nil {
					send(tea.WindowSizeMsg{Width: w, Height: h})
				}
			}
		}
	}()
}
