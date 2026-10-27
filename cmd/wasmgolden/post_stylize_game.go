package wasmgolden

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// postStylizeGame is the fixture for Halftone and FocusBlur. Halftone runs
// first on flat fills, so a threshold flip can only happen on a shape edge;
// FocusBlur then softens everything outside one rounded focus box.
type postStylizeGame struct{}

// NewPostStylizeGolden builds the Halftone and FocusBlur fixture used by the
// browser golden and raster-to-browser conformance tests.
func NewPostStylizeGolden() (runtime.Game, error) {
	return postStylizeGame{}, nil
}

func (postStylizeGame) Setup() runtime.Config {
	return runtime.Config{
		SimulationHz: 60,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
			Aspect:   render.AspectStretch,
		},
		DisableDefaultSplash: true,
		Clear:                render.RGBA8(9, 13, 24, 255),
	}
}

func (postStylizeGame) Init(*runtime.UpdateContext) {}

func (postStylizeGame) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (postStylizeGame) Render(*runtime.RenderContext) []render.Command {
	return []render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: 4, Y: 4}, math2d.Vec2{X: 92, Y: 67}),
			Fill: render.RGBA8(64, 64, 64, 255),
		},
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 36, Y: 28}),
			Fill: render.RGBA8(160, 96, 32, 255),
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 68, Y: 44},
			Radius: 18,
			Fill:   render.RGBA8(32, 128, 192, 255),
		},
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: 14, Y: 48}, math2d.Vec2{X: 30, Y: 14}),
			Fill: render.RGBA8(224, 224, 224, 255),
		},
		render.PostChain{
			Effects: []render.PostEffect{
				render.Halftone{CellSize: 4, Levels: 2, Shape: render.HalftonePlus, Strength: 1},
				render.FocusBlur{
					Points:   []math2d.Vec2{{X: 28, Y: 24}},
					Extent:   math2d.Vec2{X: 20, Y: 16},
					Rounding: 6,
					Falloff:  10,
					Strength: 1,
				},
			},
		},
	}
}
