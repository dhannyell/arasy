package breakout

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/runtime"
)

func TestBreakoutRewindRestoresBricks(t *testing.T) {
	game := &Breakout{}
	driver := runtime.NewDriver(game, slog.Default())

	game.State.state = StatePlaying
	brickIndex := 5
	aliveBefore := game.State.bricks[brickIndex].Alive
	if !aliveBefore {
		t.Fatalf("expected brick %d to start alive", brickIndex)
	}

	driver.Step() // snapshot captured with brick still alive

	game.State.bricks[brickIndex].Alive = false
	game.State.bricksLeft--

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlBack})
	driver.AdvanceTime(0)

	if !game.State.bricks[brickIndex].Alive {
		t.Fatalf("brick %d not restored on rewind: got Alive=false, want true", brickIndex)
	}
}
