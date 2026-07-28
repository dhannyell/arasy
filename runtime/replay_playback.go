package runtime

// playbackSteps advances up to steps recorded ticks by consuming the
// journal, the same source seek reconstruction reads from. It is the
// non-destructive counterpart to stepOnce: it never forks the timeline,
// truncates the recording, or commits staged input — the journal is read,
// not written.
func (d *Driver) playbackSteps(steps int) {
	if steps <= 0 {
		// Nothing requested this frame (e.g. the clock plan hasn't accumulated
		// a full tick yet) — not a reason to pause, just nothing to do.
		return
	}
	if d.tick >= d.timeline.maxTick {
		// Already at the end: nothing to watch.
		d.controls.paused = true
		return
	}

	remaining := uint64(steps)
	if available := d.timeline.maxTick - d.tick; remaining > available {
		remaining = available
	}

	cursor := d.timeline.cursorFrom(d.tick)
	dt := d.clock.DT()
	for range remaining {
		// Keyframes are captured while watching, so scrubbing a recording
		// loaded without a dense window still gets cheap afterward.
		if d.recorder != nil && d.recorder.WantsCapture(d.tick) {
			d.recorder.Capture(d.captureSnapshot())
		}
		// Playback is presentation: render interpolation needs a previous pose.
		d.world.SyncRenderState()
		d.journalStep(&cursor, dt)
	}

	// Live input that arrived while watching is inert, exactly like during a
	// seek: it never reaches the journal, so it dies here instead of forking.
	d.timeline.discardStaged()
	d.recordingStaged = nil

	if d.tick >= d.timeline.maxTick {
		d.controls.paused = true
	}
}
