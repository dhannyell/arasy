package runtime

import (
	"log/slog"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

// Recording is the input log and tick count needed for Replay.
type Recording struct {
	Events []input.Event
	Ticks  uint64
	Seed   uint64
	// GamepadDeadzone is the effective deterministic stick-axis deadzone.
	// Zero, or an invalid value in a Recording constructed in memory, means
	// use the fresh game's configured effective deadzone.
	GamepadDeadzone float64
	Anchors         []Anchor

	// Viewport is the surface size active at tick 0 of the recording. A zero
	// value (Width/Height <= 0) means "use the game's Config default", which is
	// the behavior of recordings made before viewport capture existed. Replay
	// applies it before the first tick so viewport-dependent logic (e.g. a UI
	// button hit-test) matches the live run — without it, pointer coordinates
	// captured in a different surface size map to the wrong reference-canvas
	// position and a click that started the game live is missed during replay.
	Viewport math2d.Rect

	// Viewports records SetViewport calls during the recording, each stamped
	// with the tick it was applied. Replay applies them in tick order before
	// the input events of the same tick, mirroring the live
	// SetViewport → ApplyInput → Step order. Without this, pointer coordinates
	// captured in a different surface size map to the wrong reference-canvas
	// position and UI clicks that started the game live are missed during
	// replay.
	Viewports []ViewportChange
}

// ViewportChange is one SetViewport call captured in a Recording, stamped with
// the tick it was applied.
type ViewportChange struct {
	Tick     uint64
	Viewport math2d.Rect
}

type Anchor struct {
	Tick     uint64
	Checksum uint64
}

// StartRecording begins capturing every event passed to ApplyInput.
// Start at tick 0; Replay always reconstructs from the initial state.
func (d *Driver) StartRecording() {
	if d.tick != 0 {
		d.log.Warn("recording started mid-run; Replay re-runs from tick 0 and will not reproduce it", "tick", d.tick)
	}
	d.recordingEnabled = true
	d.recording = nil
	d.recordingStaged = nil
	d.recordingAnchors = nil
	// Capture the viewport active at tick 0 so Replay can restore it; without
	// it, a platform surface different from the Config default (e.g. a
	// responsive browser canvas) shifts pointer coordinates and breaks UI
	// hit-tests during replay.
	d.recordingInitialViewport = d.viewport
}

// StopRecording returns the captured input log and final tick.
func (d *Driver) StopRecording() Recording {
	d.recordingEnabled = false
	events := d.recording
	anchors := d.recordingAnchors
	initial := d.recordingInitialViewport
	viewports := viewportLogToChanges(d.timeline.viewports, d.tick)
	d.recording = nil
	d.recordingStaged = nil
	d.recordingAnchors = nil
	d.recordingInitialViewport = math2d.Rect{}
	return Recording{
		Events:          events,
		Ticks:           d.tick,
		Seed:            d.cfg.Seed,
		GamepadDeadzone: d.cfg.GamepadDeadzone,
		Anchors:         anchors,
		Viewport:        initial,
		Viewports:       viewports,
	}
}

// viewportLogToChanges copies the driver's viewport change log into the
// exportable Recording form, dropping entries at or beyond the recording's
// final tick (they belong to ticks Replay never reaches).
func viewportLogToChanges(log []viewportEntry, ticks uint64) []ViewportChange {
	var changes []ViewportChange
	for _, ve := range log {
		if ve.tick >= ticks {
			continue
		}
		changes = append(changes, ViewportChange{Tick: ve.tick, Viewport: ve.viewport})
	}
	return changes
}

// viewportChangesToLog converts the exportable Recording viewport changes back
// into the driver-internal viewportEntry form used by replayWithViewport.
func viewportChangesToLog(changes []ViewportChange) []viewportEntry {
	log := make([]viewportEntry, len(changes))
	for i, c := range changes {
		log[i] = viewportEntry{tick: c.Tick, viewport: c.Viewport}
	}
	return log
}

// Replay reconstructs a recording on a fresh Driver, returning it at the
// final tick with no keyframes retained. Use it when only the final state
// matters (verification, tests); use ReplayForViewing when the driver will
// be adopted by an interactive host for scrubbing and playback.
// It mutates the game instance it receives; pass a fresh game unless the
// caller intentionally wants replay to adopt the reconstructed state.
func Replay(game Game, log *slog.Logger, rec Recording) *Driver {
	return replayWithViewport(game, log, rec, rec.Viewport, viewportChangesToLog(rec.Viewports), false)
}

// ReplayForViewing reconstructs a recording like Replay, but keeps the
// keyframe recorder populated during reconstruction and returns the driver
// paused at the final tick in watch mode. Use it when the reconstructed
// driver will be adopted by an interactive host for scrubbing and playback;
// use Replay when only the final state matters (verification, tests).
func ReplayForViewing(game Game, log *slog.Logger, rec Recording) *Driver {
	driver := replayWithViewport(game, log, rec, rec.Viewport, viewportChangesToLog(rec.Viewports), true)
	driver.controls.paused = true
	driver.controls.replayMode = true
	return driver
}

// replayWithViewport also replays viewport changes for UI hit-testing.
// Each viewport entry is applied before input events at the same tick. With
// keepRecorder, the keyframe recorder stays populated during reconstruction
// instead of being dropped, at the cost of a world clone per keyframe.
func replayWithViewport(game Game, log *slog.Logger, rec Recording, initialViewport math2d.Rect, viewportLog []viewportEntry, keepRecorder bool) *Driver {
	driver := newDriverFromRecording(game, log, rec)

	if !keepRecorder {
		// The final state is all that's needed; snapshot capture would clone
		// the whole World every tick for nothing.
		driver.recorder = nil
	}
	// Cues at these ticks were already heard on the live run.
	driver.audio.suppressCues = true

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

	// Reconstruction is over: a driver handed to a caller (or adopted by an
	// interactive host) must be able to emit cues again, otherwise it stays
	// mute for the rest of its life.
	driver.audio.suppressCues = false

	return driver
}
