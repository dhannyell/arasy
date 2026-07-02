package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/input"
)

func TestScrubToTickIsBidirectional(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	checksums := make([]uint64, 0, 61)
	checksums = append(checksums, driver.Checksum()) // tick 0

	for tick := range 60 {
		kind := input.EventButtonUp
		if tick%20 < 10 {
			kind = input.EventButtonDown
		}
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: kind},
		})
		driver.Step()
		checksums = append(checksums, driver.Checksum())
	}

	// Scrub backward, forward, and backward again — none of it should be
	// destructive, and every jump should land on the exact original state.
	for _, target := range []uint64{15, 45, 5, 60, 30} {
		driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: target})
		driver.AdvanceTime(0)

		if driver.Tick() != target {
			t.Fatalf("scrub to %d: tick = %d, want %d", target, driver.Tick(), target)
		}
		if got, want := driver.Checksum(), checksums[target]; got != want {
			t.Fatalf("scrub to %d: checksum = %d, want %d", target, got, want)
		}
	}
}

func TestScrubToTickIgnoresOutOfRangeTick(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	for range 10 {
		driver.Step()
	}

	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 999})
	driver.AdvanceTime(0)

	if driver.Tick() != 10 {
		t.Fatalf("tick after out-of-range scrub = %d, want unchanged 10", driver.Tick())
	}
}

func TestApplyInputAfterScrubForksTimeline(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	for range 30 {
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
		})
		driver.Step()
	}
	originalTick30 := driver.Checksum()

	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 10})
	driver.AdvanceTime(0)
	if driver.Tick() != 10 {
		t.Fatalf("tick after scrub = %d, want 10", driver.Tick())
	}

	// Resume with different input than the original run had at these ticks.
	for range 20 {
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonUp},
		})
		driver.Step()
	}
	if driver.Tick() != 30 {
		t.Fatalf("tick after resuming = %d, want 30", driver.Tick())
	}
	if driver.Checksum() == originalTick30 {
		t.Fatal("expected the forked future to diverge from the original timeline")
	}

	// The old future beyond the fork point is gone — scrubbing to tick 30
	// now reproduces the new branch, not the discarded original one.
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 30})
	driver.AdvanceTime(0)
	if got := driver.Checksum(); got == originalTick30 {
		t.Fatal("scrubbing to tick 30 after a fork should reflect the new branch, not the old one")
	}

	// The shared past before the fork point is still intact.
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 5})
	driver.AdvanceTime(0)
	if driver.Tick() != 5 {
		t.Fatalf("tick after scrub = %d, want 5", driver.Tick())
	}
}

// TestApplyInputAfterScrubShrinksMaxTickOnPartialResume covers a case a
// same-source-overwrite scenario can mask: the fork must lower maxTick to
// wherever the new branch actually reached, not leave it at the old run's
// tip. Otherwise scrubbing past the new branch's end silently resurrects
// discarded input from the original future instead of refusing the jump.
func TestApplyInputAfterScrubShrinksMaxTickOnPartialResume(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	for range 30 {
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
		})
		driver.Step()
	}

	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 10})
	driver.AdvanceTime(0)

	// Resume with a different source (A instead of D) for only 5 ticks —
	// shorter than the original future, and touching a key the original
	// run never used, so there's no overwrite to accidentally mask a bug.
	for range 5 {
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyA}, Kind: input.EventButtonDown},
		})
		driver.Step()
	}
	if driver.Tick() != 15 {
		t.Fatalf("tick after partial resume = %d, want 15", driver.Tick())
	}

	// The old future's tick 30 must no longer be reachable: it belonged to
	// a timeline that no longer exists past the fork point.
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 30})
	driver.AdvanceTime(0)
	if driver.Tick() != 15 {
		t.Fatalf("scrub to stale tick 30 should be a no-op, but tick = %d", driver.Tick())
	}
}
