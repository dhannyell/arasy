package runtime

import (
	"context"
	"log/slog"
	"math"

	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/ui"
)

// seek.go handles seeking: moving the driver to a recorded tick by restoring
// the nearest snapshot (or resetting to tick 0) and resimulating forward from
// the timeline journal. Forking the timeline remains exclusive to stepOnce —
// seeking never truncates the journal or keyframes.

func (d *Driver) coalesceSeeks(seeks []seekRequest) (target uint64, ok bool) {
	target = d.tick
	for _, seek := range seeks {
		switch seek.kind {
		case seekBack:
			if target > 0 {
				target--
			}
		case seekToTick:
			if seek.tick < target {
				target = seek.tick
			}
		case seekBackSeconds:
			if seek.seconds <= 0 {
				continue
			}
			back := uint64(math.Ceil(seek.seconds * float64(d.cfg.SimulationHz)))
			if back > target {
				target = 0
			} else {
				target -= back
			}
		case seekScrub:
			if seek.tick <= d.timeline.maxTick {
				target = seek.tick
			}
		}
	}
	return target, target != d.tick
}

// seekTo moves the driver to target ∈ [0, maxTick]. It never truncates the
// timeline journal or keyframes — the fork remains exclusive to stepOnce.
func (d *Driver) seekTo(target uint64) {
	if target > d.timeline.maxTick || target == d.tick {
		return
	}

	surfaceViewport := d.viewport

	// Fast-forward can be incremental only when the current state has not been
	// tainted by staged input/viewport changes that are waiting for a fork.
	if target > d.tick && len(d.timeline.staged) == 0 && !d.timeline.hasStagedViewport {
		d.viewport = d.timeline.viewportAt(d.tick, d.viewport)
		d.resimulate(target)
		d.viewport = surfaceViewport
		return
	}

	restored := false
	if d.recorder != nil && d.canRestoreKeyframes() {
		if snap, ok := d.recorder.KeyframeAtOrBefore(target); ok {
			d.restoreSnapshot(snap)
			restored = true
		}
	}
	if !restored {
		d.resetForResimulation()
	}
	d.viewport = d.timeline.viewportAt(d.tick, d.viewport)
	if !restored && target == 0 {
		d.applyLoggedInputsAtCurrentTick()
	}
	d.resimulate(target)
	d.viewport = surfaceViewport

	if d.log.Enabled(context.Background(), slog.LevelDebug) {
		d.log.Debug("seek", "tick", d.tick, "target", target, "checksum", d.world.Checksum())
	}
}

func (d *Driver) resetForResimulation() {
	d.world = scene.NewWorld()
	d.tick = 0
	d.uiState = ui.State{}
	d.animStore = ui.AnimStore{}
	d.input.Reset()
	d.timeline.discardStaged()
	d.recordingStaged = nil
	d.rng = NewRand(d.cfg.Seed)
	d.audio.reset()
	if game, ok := d.game.(StatefulGame); ok {
		game.RestoreGameState(nil)
	}
	ctx := d.makeUpdateContext(d.newRenderUI())
	d.game.Init(&ctx)
}

func (d *Driver) resimulate(target uint64) {
	// Reconstruction is silent regardless of direction: the cues at these ticks
	// were already heard on the live run (D7 scrub policy). Suppressing here
	// covers both seekTo paths (incremental fast-forward and keyframe restore),
	// and drops any cue left pending from the pre-seek present.
	d.audio.suppressCues = true
	d.audio.pending = nil
	defer func() { d.audio.suppressCues = false }()

	cursor := d.timeline.cursorFrom(d.tick)
	dt := d.clock.DT()

	for d.tick < target {
		for _, viewport := range cursor.viewportsAt(d.tick) {
			d.viewport = viewport
		}
		d.input.Apply(cursor.eventsAt(d.tick), d.tick)
		d.stepCore(dt)
		d.verifyKeyframe(d.tick)
	}
	d.timeline.discardStaged()
	d.recordingStaged = nil
}

func (d *Driver) applyLoggedInputsAtCurrentTick() {
	cursor := d.timeline.cursorFrom(d.tick)
	for _, viewport := range cursor.viewportsAt(d.tick) {
		d.viewport = viewport
	}
	d.input.Apply(cursor.eventsAt(d.tick), d.tick)
}

func (d *Driver) verifyKeyframe(tick uint64) {
	if d.recorder == nil || !d.canRestoreKeyframes() {
		return
	}
	keyframe, ok := d.recorder.KeyframeAt(tick)
	if !ok {
		return
	}
	got := d.world.Checksum()
	if got == keyframe.checksum {
		return
	}
	d.determinismViolated = true
	d.log.Error("resimulation diverged from keyframe", "tick", tick, "got", got, "want", keyframe.checksum)
}
