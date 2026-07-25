package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/input"
)

// TestScrubCoalescesMultipleCommands verifies that when multiple ScrubToTick
// commands are queued before a single AdvanceTime (as happens when the web
// canvas slider sends one every 50ms during a drag), only the LAST target is
// processed. Without coalescing, each command triggers a full replay from
// tick 0 — N commands × average-target ticks = N replays, causing multi-second
// freezes during slider drags.
func TestScrubCoalescesMultipleCommands(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	// Run to tick 50.
	for range 50 {
		driver.Step()
	}
	originalTick50 := driver.Checksum()

	// Queue 10 scrub commands with decreasing targets (simulating a slider
	// drag from tick 50 down to tick 5). Without coalescing, this would
	// trigger 10 full replays (45+40+35+...+5 = 250 ticks). With coalescing,
	// only the last target (5) is processed (5 ticks).
	for target := uint64(45); target >= 5; target -= 5 {
		driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: target})
	}
	driver.AdvanceTime(0)

	// The driver should be at the LAST queued target (5), not any intermediate.
	if driver.State().Tick != 5 {
		t.Fatalf("tick after coalesced scrub = %d, want 5 (last queued target)", driver.State().Tick)
	}

	// Scrub back to 50 and verify the original state is intact.
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 50})
	driver.AdvanceTime(0)
	if got := driver.Checksum(); got != originalTick50 {
		t.Fatalf("checksum at tick 50 after coalesced scrub = %d, want %d", got, originalTick50)
	}
}

// TestScrubAfterForkStillFast verifies the specific scenario the user reported:
// scrub → resume (fork) → advance → scrub again. The second scrub must be just
// as fast as the first, confirming no state accumulation slows down replays.
func TestScrubAfterForkStillFast(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	// Run to tick 30 with input.
	for range 30 {
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
		})
		driver.Step()
	}
	checksums := make([]uint64, 31)
	for i := uint64(0); i <= 30; i++ {
		if i > 0 {
			driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: i})
			driver.AdvanceTime(0)
		}
		checksums[i] = driver.Checksum()
	}
	// Go back to tick 30.
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 30})
	driver.AdvanceTime(0)

	// First scrub to 10.
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 10})
	driver.AdvanceTime(0)
	if driver.State().Tick != 10 {
		t.Fatalf("first scrub: tick = %d, want 10", driver.State().Tick)
	}
	if got, want := driver.Checksum(), checksums[10]; got != want {
		t.Fatalf("first scrub: checksum = %d, want %d", got, want)
	}

	// Fork: resume with DIFFERENT input (KeyD up = stop moving) for 20 ticks.
	for range 20 {
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonUp},
		})
		driver.Step()
	}
	if driver.State().Tick != 30 {
		t.Fatalf("after fork+advance: tick = %d, want 30", driver.State().Tick)
	}
	forkedTick30 := driver.Checksum()
	if forkedTick30 == checksums[30] {
		t.Fatal("forked timeline should diverge from original")
	}

	// Second scrub to 10 — must land on the SAME state as the first scrub
	// (the shared past before the fork point is intact).
	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 10})
	driver.AdvanceTime(0)
	if driver.State().Tick != 10 {
		t.Fatalf("second scrub: tick = %d, want 10", driver.State().Tick)
	}
	if got, want := driver.Checksum(), checksums[10]; got != want {
		t.Fatalf("second scrub after fork: checksum = %d, want %d (shared past should be intact)", got, want)
	}
}
