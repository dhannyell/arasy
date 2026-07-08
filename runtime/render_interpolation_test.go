package runtime_test

import (
	"testing"
	"time"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
)

// interpolationGame tracks whether RenderContext.Alpha was non-zero.
type interpolationGame struct {
	setupCfg runtime.Config
	alpha    float64
}

func (g *interpolationGame) Setup() runtime.Config {
	g.setupCfg.RenderInterpolation = true
	g.setupCfg.Camera = render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
	}
	return g.setupCfg
}

func (g *interpolationGame) Init(ctx *runtime.UpdateContext) {}

func (g *interpolationGame) Update(ctx *runtime.UpdateContext, dt float64) {}

func (g *interpolationGame) Render(ctx *runtime.RenderContext) []render.Command {
	g.alpha = ctx.Alpha
	return nil
}

// noInterpGame has RenderInterpolation off.
type noInterpGame struct {
	alpha float64
}

func (g *noInterpGame) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		},
	}
}

func (g *noInterpGame) Init(ctx *runtime.UpdateContext) {}

func (g *noInterpGame) Update(ctx *runtime.UpdateContext, dt float64) {}

func (g *noInterpGame) Render(ctx *runtime.RenderContext) []render.Command {
	g.alpha = ctx.Alpha
	return nil
}

func TestRenderContextAlphaZeroWhenInterpolationDisabled(t *testing.T) {
	game := &noInterpGame{}
	driver := runtime.NewDriver(game, testLogger())
	driver.AdvanceTime(time.Second / 120) // less than one tick
	driver.Frame(time.Second / 120)
	if game.alpha != 0 {
		t.Fatalf("alpha = %v, want 0 (interpolation disabled)", game.alpha)
	}
}

func TestRenderContextAlphaNonZeroWithFractionalElapsed(t *testing.T) {
	game := &interpolationGame{}
	player := runtime.NewDriver(game, testLogger())
	// Advance half a tick so accumulator > 0.
	player.AdvanceTime(time.Second / 120) // 0.00833s, half of 1/60
	player.Frame(time.Second / 120)
	if game.alpha <= 0 {
		t.Fatalf("alpha = %v, want > 0 (fractional elapsed with interpolation)", game.alpha)
	}
}

func TestStepPreservesAccumulatorAndAlpha(t *testing.T) {
	game := &interpolationGame{}
	player := runtime.NewDriver(game, testLogger())
	// Step runs a full tick, no accumulation.
	stats := player.Step()
	if stats.Alpha != 0 {
		t.Fatalf("Step() alpha = %v, want 0", stats.Alpha)
	}
}

func TestDriverAutoStepSyncsRenderState(t *testing.T) {
	world := scene.NewWorld()
	id := world.Spawn()
	world.SetTransform(id, scene.Transform{
		Position: math2d.Vec2{X: 0, Y: 0},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleSprite(id, scene.CircleSprite{
		Radius: 2,
		BaseSprite: scene.BaseSprite{
			Color: render.RGBA(1, 0, 0, 1),
			Layer: 5,
		},
	})
	world.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 10, Y: 10}})
	world.SyncRenderState()
	world.Step(1, scene.StepConfig{})

	// After step, current transform is (10,10) and renderTransforms is (0,0).
	// At alpha=0.5, interpolated position should be (5,5).
	cmds := world.RenderCommandsInterpolated(0.5)
	circle := cmds[0].(render.DrawCircle)
	want := math2d.Vec2{X: 5, Y: 5}
	if circle.Center != want {
		t.Fatalf("interpolated center = %+v, want %+v", circle.Center, want)
	}
}
