package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/input"
)

func TestReplayReproducesChecksum(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.StartRecording()

	for tick := range 300 {
		kind := input.EventButtonUp
		if tick%40 < 20 {
			kind = input.EventButtonDown
		}
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: kind},
		})
		driver.Step()
	}

	rec := driver.StopRecording()
	if len(rec.Events) == 0 {
		t.Fatal("recording captured no events")
	}
	if rec.Ticks != driver.State().Tick {
		t.Fatalf("recording.Ticks = %d, want %d", rec.Ticks, driver.State().Tick)
	}

	replay := Replay(&inputBackGame{}, slog.Default(), rec)

	if replay.State().Tick != driver.State().Tick {
		t.Fatalf("replay tick = %d, want %d", replay.State().Tick, driver.State().Tick)
	}
	if got, want := replay.Checksum(), driver.Checksum(); got != want {
		t.Fatalf("replay checksum = %d, want %d", got, want)
	}
}

func TestReplayDivergesWhenInputDiffers(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.StartRecording()
	for tick := range 60 {
		kind := input.EventButtonUp
		if tick >= 20 {
			kind = input.EventButtonDown
		}
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: kind},
		})
		driver.Step()
	}
	rec := driver.StopRecording()
	want := driver.Checksum()

	// Drop the event that presses D, so the replay never moves the player.
	tampered := Recording{Ticks: rec.Ticks}
	for _, e := range rec.Events {
		if e.Kind == input.EventButtonDown {
			continue
		}
		tampered.Events = append(tampered.Events, e)
	}

	replay := Replay(&inputBackGame{}, slog.Default(), tampered)
	if got := replay.Checksum(); got == want {
		t.Fatal("expected checksum to diverge when replayed with different input, but it matched")
	}
}

// TestResetRestartsActiveRecording: a reset restarts tick 0, so keeping the
// pre-reset events would collide with the new run's ticks — Replay would
// apply both runs' events on the same ticks and reproduce a state that never
// existed. The capture must restart with the run, keeping StopRecording
// reproducible.
func TestResetRestartsActiveRecording(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.StartRecording()

	// Pre-reset run: D held on ticks 0..4. If these events survived the
	// reset, the replay below would move the player from tick 0.
	for range 5 {
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
		})
		driver.Step()
	}

	driver.Control(ControlCommand{Kind: ControlReset})
	driver.AdvanceTime(0)

	// Post-reset run: idle on ticks 0..4 (where the stale events would
	// land), then a press/release pair.
	for tick := range 10 {
		switch tick {
		case 5:
			driver.ApplyInput([]input.Event{
				{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
			})
		case 8:
			driver.ApplyInput([]input.Event{
				{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonUp},
			})
		}
		driver.Step()
	}

	rec := driver.StopRecording()
	if rec.Ticks != 10 {
		t.Fatalf("recording ticks = %d, want 10 (post-reset run only)", rec.Ticks)
	}

	replay := Replay(&inputBackGame{}, slog.Default(), rec)
	if got, want := replay.Checksum(), driver.Checksum(); got != want {
		t.Fatalf("replay checksum = %d, want %d — recording kept pre-reset events", got, want)
	}
}
