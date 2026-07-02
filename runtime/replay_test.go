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
	if rec.Ticks != driver.Tick() {
		t.Fatalf("recording.Ticks = %d, want %d", rec.Ticks, driver.Tick())
	}

	replay := Replay(&inputBackGame{}, slog.Default(), rec)

	if replay.Tick() != driver.Tick() {
		t.Fatalf("replay tick = %d, want %d", replay.Tick(), driver.Tick())
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
