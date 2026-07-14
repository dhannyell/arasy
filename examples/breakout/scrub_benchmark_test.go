package breakout

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/runtime"
)

// BenchmarkBreakoutScrub measures the time to scrub from a high tick back to a
// lower one with the real Breakout game (72 bricks + paddle + ball = 74
// entities). This is the operation that was taking 20+ seconds before the fix
// (suppressing snapshot capture during replay).
func BenchmarkBreakoutScrub(b *testing.B) {
	game := &Breakout{}
	driver := runtime.NewDriver(game, slog.Default())
	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 960}))

	const playX, playY = 640.0, 510.0

	// Start the game (click Play + Space).
	driver.ApplyInput([]input.Event{
		mouseMove(playX, playY), mouseDown(playX, playY), mouseUp(playX, playY),
	})
	driver.Step()
	driver.ApplyInput([]input.Event{keySpaceDown()})
	driver.Step()

	// Run to tick 2000.
	for driver.State().Tick < 2000 {
		driver.Step()
	}

	fromTick := driver.State().Tick
	target := uint64(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Scrub back to target.
		driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: target})
		driver.AdvanceTime(0)

		// Scrub forward to fromTick for the next iteration.
		driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: fromTick})
		driver.AdvanceTime(0)
	}
}
