package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/input"
)

// TestRecordingIgnoresStagedInputDiscardedByScrub: input applied while
// rewound is staged in the timeline (not yet part of the live run) and can
// be discarded by a later scrub before the driver ever steps past it. The
// explicit recording log (Driver.recording, used by StopRecording/Replay)
// must follow the same staging rule — otherwise the recording captures an
// event the live run never actually applied, and Replay diverges from the
// live driver's final checksum.
func TestRecordingIgnoresStagedInputDiscardedByScrub(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.StartRecording()

	// Idle run out to tick 20, establishing maxTick = 20.
	for range 20 {
		driver.Step()
	}

	// Rewind into the middle of the run.
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 5})
	driver.AdvanceTime(0)

	// Apply input while rewound: this is staged (tick 5 < maxTick 20), not
	// yet part of the live timeline.
	driver.ApplyInput([]input.Event{
		{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
	})

	// Scrub away again before ever stepping past tick 5: the staged event is
	// discarded and the live simulation never applies it.
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 8})
	driver.AdvanceTime(0)

	// Resume the live run (idle, matching the original run) out to tick 20.
	for range 12 {
		driver.Step()
	}
	if driver.State().Tick != 20 {
		t.Fatalf("tick after resume = %d, want 20", driver.State().Tick)
	}
	liveChecksum := driver.Checksum()

	rec := driver.StopRecording()

	replay := Replay(&inputBackGame{}, slog.Default(), rec)
	if got, want := replay.Checksum(), liveChecksum; got != want {
		t.Fatalf("replay checksum = %d, want %d (live) — recording captured a staged event the live run discarded", got, want)
	}
}
