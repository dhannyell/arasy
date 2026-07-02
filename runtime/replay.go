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
func (d *Driver) StartRecording() {
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
