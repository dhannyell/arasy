// Package vectoranim demonstrates generated and hand-authored vector animation.
package vectoranim

import (
	"math"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/ease"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

const duration = 3.0

type Game struct{}

func (g *Game) Setup() runtime.Config {
	world := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 180})
	return runtime.Config{
		Camera: render.Camera{
			World: world, Viewport: world, Aspect: render.AspectFit,
		},
		Clear:        render.RGBA8(16, 24, 39, 255),
		SimulationHz: 60,
		FPS:          60,
	}
}

func (g *Game) Init(*runtime.UpdateContext) {}

func (g *Game) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	timeline := anim.At(math.Mod(ctx.Time, duration))
	return AppendVectorScene(nil, timeline, ctx.Camera().World, VectorSceneResources{})
}

// AppendManualAccent proves the public animation and composition APIs can be
// used independently of generated code. It is deliberately kept separate
// from Game.Render so replacing the generated SVG cannot inherit demo-only
// decoration.
func AppendManualAccent(dst []render.Command, timeline anim.Timeline) []render.Command {
	pulse := timeline.Track(1.8, 2.6, ease.InOutQuad)
	var builder render.Builder
	builder.Reset(dst)
	builder.Push(render.Group{
		Transform: math2d.Scale(math2d.Vec2{
			X: pulse.Lerp(0.5, 1),
			Y: pulse.Lerp(0.5, 1),
		}).About(math2d.Vec2{X: 160, Y: 90}),
		Opacity: pulse.Lerp(0, 0.75),
	})
	builder.Add(render.DrawCircle{
		Center: math2d.Vec2{X: 160, Y: 90},
		Radius: 11,
		Fill:   render.RGBA8(255, 255, 255, 255),
	})
	builder.Pop()
	return builder.Commands()
}
