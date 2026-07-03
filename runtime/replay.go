package runtime

import (
	"log/slog"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
)

// Recording is the exact input log and tick count needed to deterministically
// reproduce a run via Replay.
type Recording struct {
	Events []input.Event
	Ticks  uint64
}

// StartRecording begins capturing every event passed to ApplyInput. Call
// StopRecording to retrieve the log once the run to be replayed is done.
//
// Start it at tick 0 (a fresh driver, or right after a reset): Replay always
// re-runs from tick 0, so a recording started mid-run misses the input that
// shaped the earlier ticks and cannot be reproduced. A reset while recording
// restarts the capture from the new tick 0 for the same reason.
func (d *Driver) StartRecording() {
	if d.tick != 0 {
		d.log.Warn("recording started mid-run; Replay re-runs from tick 0 and will not reproduce it", "tick", d.tick)
	}
	d.recordingEnabled = true
	d.recording = nil
}

// StopRecording ends capture and returns the recorded input log together
// with the tick the driver had reached, ready to hand to Replay.
func (d *Driver) StopRecording() Recording {
	d.recordingEnabled = false
	events := d.recording
	d.recording = nil
	return Recording{Events: events, Ticks: d.tick}
}

// Replay re-runs game on a fresh Driver, injecting rec.Events on the exact
// ticks they were originally applied and advancing with Step — the
// deterministic entry point — rec.Ticks times. Compare the returned
// Driver's Checksum against the original run's to verify the replay
// reproduced it exactly.
//
// Replay resets and mutates the game instance it receives (Setup, Init and
// every Update run again). Do not pass a game still attached to a live
// Driver you want to keep — hand it a fresh instance instead. The internal
// scrubber relies on this same in-place behavior on purpose: it replays the
// live driver's own game so the reconstructed state is adopted directly.
func Replay(game Game, log *slog.Logger, rec Recording) *Driver {
	return replayWithViewport(game, log, rec, math2d.Rect{}, nil)
}

// replayWithViewport is Replay with the surface viewport applied before the
// first step. The viewport is not part of the input log, yet it shapes any
// viewport-dependent Update logic — most importantly UI hit-testing (a button
// click maps through the camera, which uses the viewport). The internal
// scrubber threads the live driver's viewport and viewport change log here so
// replayed UI interactions (e.g. breakout's Play button) land exactly as they
// did live; without it a UI-gated game reconstructs a divergent, frozen world
// during scrub.
//
// viewportLog is a tick-tagged log of SetViewport calls. During replay, each
// entry is applied at its tick (before any events at that tick), so the
// viewport used for UI hit-testing at each tick matches what was active when
// the pointer events were originally recorded. initialViewport is the
// fallback for ticks before the first log entry.
func replayWithViewport(game Game, log *slog.Logger, rec Recording, initialViewport math2d.Rect, viewportLog []viewportEntry) *Driver {
	driver := NewDriver(game, log)

	// Disable snapshot capture during replay: each Capture clones the entire
	// World (every component map), which is O(entities × component-maps) per
	// tick. For the scrubber this multiplies the replay cost by that factor
	// on every tick — e.g. breakout's 74 entities make a 1000-tick scrub take
	// 20+ seconds. The scrubber only needs the final state, not intermediate
	// snapshots, so we suppress capture here and let doScrubToTick capture a
	// single snapshot at the target tick afterward.
	driver.recorder = nil

	// Build a tick → viewport map (last entry per tick wins).
	viewportByTick := make(map[uint64]math2d.Rect)
	for _, ve := range viewportLog {
		if ve.tick < rec.Ticks {
			viewportByTick[ve.tick] = ve.viewport
		}
	}

	// Apply the initial viewport for ticks before the first log entry. If
	// there's an entry at tick 0, it overrides this at tick 0 below.
	if initialViewport.Width() > 0 && initialViewport.Height() > 0 {
		driver.viewport = initialViewport
	}

	byTick := make(map[uint64][]input.Event, len(rec.Events))
	for _, event := range rec.Events {
		byTick[event.Tick] = append(byTick[event.Tick], event)
	}

	for tick := uint64(0); tick < rec.Ticks; tick++ {
		// Apply the viewport change for this tick before any events, matching
		// the live driver's order (SetViewport before ApplyInput before Step).
		if vp, ok := viewportByTick[tick]; ok {
			driver.viewport = vp
		}
		if events, ok := byTick[tick]; ok {
			driver.ApplyInput(events)
		}
		driver.Step()
	}

	return driver
}
