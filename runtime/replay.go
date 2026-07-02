package runtime

import (
	"log/slog"

	"github.com/dhannyell/frameforge/input"
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
	driver := NewDriver(game, log)

	byTick := make(map[uint64][]input.Event, len(rec.Events))
	for _, event := range rec.Events {
		byTick[event.Tick] = append(byTick[event.Tick], event)
	}

	for tick := uint64(0); tick < rec.Ticks; tick++ {
		if events, ok := byTick[tick]; ok {
			driver.ApplyInput(events)
		}
		driver.Step()
	}

	return driver
}
