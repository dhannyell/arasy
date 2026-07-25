package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// viewportProbeGame records the viewport width it observes each Update into an
// entity's position, so the simulated world state depends on the driver
// viewport — exactly the way breakout depends on the viewport to hit-test its
// UI Play button. It is the minimal stand-in for any UI-gated game.
type viewportProbeGame struct {
	id scene.EntityID
}

func (g *viewportProbeGame) Setup() Config {
	return Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		},
		SimulationHz: 60,
		HistoryLen:   100,
	}
}

func (g *viewportProbeGame) Init(ctx *UpdateContext) {
	g.id = ctx.World.Spawn()
	ctx.World.SetTransform(g.id, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
}

func (g *viewportProbeGame) Update(ctx *UpdateContext, dt float64) {
	transform, _ := ctx.World.Transform(g.id)
	transform.Position.X += ctx.Camera().Viewport.Width()
	ctx.World.SetTransform(g.id, transform)
}

func (g *viewportProbeGame) Render(ctx *RenderContext) []render.Command { return nil }

// TestScrubReconstructsViewportDependentState is the regression test for the
// breakout scrubber bug: the live driver runs with a viewport set by the
// platform (SetViewport), but ScrubToTick replays on a fresh driver. If the
// replay does not inherit the viewport, any viewport-dependent logic (a UI
// button hit-test) diverges and the reconstructed world is wrong — the ball
// and bricks never move during scrub.
func TestScrubReconstructsViewportDependentState(t *testing.T) {
	game := &viewportProbeGame{}
	driver := NewDriver(game, slog.Default())

	// Platform viewport differs from the config default (100x100), just like a
	// responsive browser canvas.
	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 640, Y: 480}))

	checksums := make([]uint64, 0, 11)
	checksums = append(checksums, driver.Checksum())
	for range 10 {
		driver.Step()
		checksums = append(checksums, driver.Checksum())
	}

	// Sanity: the live run accumulated the live viewport width (640), not the
	// config default (100).
	liveTransform, _ := driver.world.Transform(game.id)
	if pos := liveTransform.Position.X; pos != 6400 {
		t.Fatalf("live position after 10 ticks = %v, want 6400 (10*640)", pos)
	}

	for _, target := range []uint64{5, 8, 3, 10} {
		driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: target})
		driver.AdvanceTime(0)

		if got, want := driver.Checksum(), checksums[target]; got != want {
			t.Fatalf("scrub to %d: checksum = %d, want %d — replay did not inherit the viewport",
				target, got, want)
		}
	}
}
