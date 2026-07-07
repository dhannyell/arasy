package runtime

import (
	"log/slog"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
)

// Recording is the input log and tick count needed for Replay.
type Recording struct {
	Events []input.Event
	Ticks  uint64
	Seed   uint64
}

// StartRecording begins capturing every event passed to ApplyInput.
// Start at tick 0; Replay always reconstructs from the initial state.
func (d *Driver) StartRecording() {
	if d.tick != 0 {
		d.log.Warn("recording started mid-run; Replay re-runs from tick 0 and will not reproduce it", "tick", d.tick)
	}
	d.recordingEnabled = true
	d.recording = nil
}

// StopRecording returns the captured input log and final tick.
func (d *Driver) StopRecording() Recording {
	d.recordingEnabled = false
	events := d.recording
	d.recording = nil
	return Recording{Events: events, Ticks: d.tick, Seed: d.cfg.Seed}
}

// Replay reconstructs a recording on a fresh Driver.
// It mutates the game instance it receives; pass a fresh game unless the
// caller intentionally wants replay to adopt the reconstructed state.
func Replay(game Game, log *slog.Logger, rec Recording) *Driver {
	return replayWithViewport(game, log, rec, math2d.Rect{}, nil)
}

// replayWithViewport also replays viewport changes for UI hit-testing.
// Each viewport entry is applied before input events at the same tick.
func replayWithViewport(game Game, log *slog.Logger, rec Recording, initialViewport math2d.Rect, viewportLog []viewportEntry) *Driver {
	driver := newDriver(game, log, &rec.Seed)

	// Replay only needs the final state; snapshot capture would clone the
	// whole World every tick.
	driver.recorder = nil

	viewportByTick := make(map[uint64]math2d.Rect)
	for _, ve := range viewportLog {
		if ve.tick < rec.Ticks {
			viewportByTick[ve.tick] = ve.viewport
		}
	}

	// Tick 0 entries override this initial viewport below.
	if initialViewport.Width() > 0 && initialViewport.Height() > 0 {
		driver.viewport = initialViewport
	}

	byTick := make(map[uint64][]input.Event, len(rec.Events))
	for _, event := range rec.Events {
		byTick[event.Tick] = append(byTick[event.Tick], event)
	}

	for tick := uint64(0); tick < rec.Ticks; tick++ {
		// Match live order: SetViewport, ApplyInput, Step.
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
