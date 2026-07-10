package runtime

import (
	"time"

	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
)

// FrameTimer turns host timestamps into frame deltas. The first sample after
// construction or Reset deliberately returns zero so a newly active host
// cannot inject the idle time accumulated before it had a client or canvas.
type FrameTimer struct {
	last    time.Time
	started bool
}

func (t *FrameTimer) Elapsed(now time.Time) time.Duration {
	if !t.started {
		t.last = now
		t.started = true
		return 0
	}
	elapsed := now.Sub(t.last)
	t.last = now
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

// ElapsedMillis is the equivalent helper for requestAnimationFrame's
// monotonic DOMHighResTimeStamp, expressed in milliseconds.
func (t *FrameTimer) ElapsedMillis(timestamp float64) time.Duration {
	now := time.Unix(0, int64(timestamp*float64(time.Millisecond)))
	return t.Elapsed(now)
}

func (t *FrameTimer) Reset() {
	t.last = time.Time{}
	t.started = false
}

// TickBatch is the host-owned input/control batch applied before a frame's
// fixed-step simulation advance.
type TickBatch struct {
	Viewport *math2d.Rect
	Controls []ControlCommand
	Events   []input.Event
}

// RunTick applies one host frame in the canonical order: viewport, controls,
// input, fixed-step advance, and audio drain.
func (d *Driver) RunTick(batch TickBatch, elapsed time.Duration) (AdvanceStats, audio.Output) {
	if batch.Viewport != nil {
		d.SetViewport(*batch.Viewport)
	}
	for _, command := range batch.Controls {
		d.Control(command)
	}
	d.ApplyInput(batch.Events)
	stats := d.AdvanceTime(elapsed)
	return stats, d.ConsumeAudio()
}
