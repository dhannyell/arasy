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

func TestResumeAfterScrubForksTimeline(t *testing.T) {
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

// Regression: pointer movement while paused must not fork the future.
func TestInputWhilePausedAfterScrubKeepsFutureScrubbable(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{
		{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
	})
	for range 30 {
		driver.Step()
	}
	originalTick30 := driver.Checksum()

	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 10})
	driver.AdvanceTime(0)

	// Pointer moves over the canvas while paused; no step runs.
	driver.ApplyInput([]input.Event{{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseX},
		Kind:   input.EventPointerMove,
	}})
	driver.AdvanceTime(0) // paused: processes controls only, no steps

	if got := driver.State().MaxRecordedTick; got != 30 {
		t.Fatalf("MaxRecordedTick after paused pointer move = %d, want 30", got)
	}

	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 30})
	driver.AdvanceTime(0)
	if got := driver.Checksum(); got != originalTick30 {
		t.Fatalf("scrub forward after paused pointer move: checksum = %d, want original %d", got, originalTick30)
	}
}

// Regression: resuming from a rewound tick forks even without new input.
func TestResumeWithoutInputForksTimeline(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	// Original timeline: D held from tick 0, released at tick 20.
	driver.ApplyInput([]input.Event{
		{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
	})
	for tick := 0; tick < 30; tick++ {
		if tick == 20 {
			driver.ApplyInput([]input.Event{
				{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonUp},
			})
		}
		driver.Step()
	}
	originalTick30 := driver.Checksum()

	// Scrub to 10 and resume with no input at all: D stays held past tick
	// 20 (the recorded release is not re-injected), so this is a new branch.
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 10})
	driver.AdvanceTime(0)
	for range 20 {
		driver.Step()
	}
	liveTick30 := driver.Checksum()
	if liveTick30 == originalTick30 {
		t.Fatal("expected the input-less resume to diverge from the original timeline")
	}

	// Scrubbing back must land on the branch just watched, not the old one.
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 30})
	driver.AdvanceTime(0)
	if got := driver.Checksum(); got != liveTick30 {
		t.Fatalf("scrub to 30 after input-less resume: checksum = %d, want live branch %d (original was %d)",
			got, liveTick30, originalTick30)
	}
}

// TestStagedInputDiscardedByNextScrub: input applied while rewound is staged,
// and a scrub before any step must discard it — both its mapper effect (the
// scrub replaces the mapper wholesale) and its log entry. If the discard
// leaked the staged event into the log, replaying would move the player and
// the reconstructed state would diverge from the live branch.
func TestStagedInputDiscardedByNextScrub(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	for range 30 {
		driver.Step()
	}

	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 10})
	driver.AdvanceTime(0)

	// Staged while rewound, then discarded by the next scrub.
	driver.ApplyInput([]input.Event{
		{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
	})
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 12})
	driver.AdvanceTime(0)

	for range 3 {
		driver.Step()
	}
	if driver.Tick() != 15 {
		t.Fatalf("tick after resume = %d, want 15", driver.Tick())
	}
	liveTick15 := driver.Checksum()

	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 15})
	driver.AdvanceTime(0)
	if got := driver.Checksum(); got != liveTick15 {
		t.Fatalf("scrub to 15: checksum = %d, want %d — staged input leaked into the scrub log", got, liveTick15)
	}
}

// TestResumeAfterScrubShrinksMaxTickOnPartialResume covers a case a
// same-source-overwrite scenario can mask: the fork must lower maxTick to
// wherever the new branch actually reached, not leave it at the old run's
// tip. Otherwise, scrubbing past the new branch's end silently resurrects
// discarded input from the original future instead of refusing the jump.
func TestResumeAfterScrubShrinksMaxTickOnPartialResume(t *testing.T) {
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
