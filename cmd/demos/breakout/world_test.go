package breakout

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
)

func TestBrickColorInRange(t *testing.T) {
	colors := make(map[render.Color]struct{})

	for row := range brickRows {
		c := brickColor(row)
		if c.R < 0 || c.R > 1 || c.G < 0 || c.G > 1 || c.B < 0 || c.B > 1 {
			t.Fatalf("row %d color out of range: %+v", row, c)
		}
		colors[c] = struct{}{}
	}

	if len(colors) < brickRows-1 {
		t.Fatalf("brickColor is not producing varied colors: got %d distinct colors from %d rows", len(colors), brickRows)
	}
}

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

func TestBallFallsThroughOpenBottomAndCostsALife(t *testing.T) {
	game := &Breakout{}
	cfg := game.Setup()
	world := scene.NewWorld()
	ctx := runtime.UpdateContext{World: world}
	game.Init(&ctx)

	game.State.state = StatePlaying
	game.State.lives = 3
	game.State.ball.Position = math2d.Vec2{X: worldWidth / 2, Y: worldHeight + ballRadius + 0.5}
	game.State.ball.Velocity = math2d.Vec2{X: 0, Y: ballSpeed}
	world.SetTransform(game.State.ball.id, scene.Transform{Position: game.State.ball.Position})
	world.SetVelocity(game.State.ball.id, scene.Velocity{Linear: game.State.ball.Velocity})

	world.Step(1.0/60.0, scene.StepConfig{
		Bounds:  cfg.Bounds,
		Bounce:  cfg.Bounce,
		Physics: cfg.Physics,
	})
	game.PostStep(&ctx, scene.StepStats{})

	if game.State.lives != 2 {
		t.Fatalf("lives = %d, want 2", game.State.lives)
	}
	if game.State.state != StateReady {
		t.Fatalf("state = %v, want StateReady", game.State.state)
	}
}
