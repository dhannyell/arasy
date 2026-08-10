package wasmgolden

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// postGame is the deterministic post-processing fixture. It keeps the HUD
// above BelowLayer so the browser golden also proves the compositor split.
type postGame struct {
	renderCount uint64
}

// NewPostGolden builds the post-processing fixture used by the browser golden
// and raster-to-browser conformance tests.
func NewPostGolden() (runtime.Game, error) {
	return &postGame{}, nil
}

func (*postGame) Setup() runtime.Config {
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

func (*postGame) Init(*runtime.UpdateContext) {}

func (*postGame) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (g *postGame) Render(*runtime.RenderContext) []render.Command {
	time := 0.0
	if g.renderCount > 0 {
		time = 0.25
	}
	g.renderCount++

	return []render.Command{
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: 0},
			Rect:        math2d.NewRect(math2d.Vec2{X: 4, Y: 4}, math2d.Vec2{X: 92, Y: 67}),
			Fill:        render.RGBA8(24, 35, 61, 255),
			Stroke:      render.RGBA8(76, 103, 155, 255),
			StrokeWidth: 1,
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: 1},
			Center:      math2d.Vec2{X: 29, Y: 26},
			Radius:      13,
			Fill:        render.RGBA(0.12, 0.55, 1, 0.9),
			Stroke:      render.RGBA8(204, 236, 255, 255),
			StrokeWidth: 1.5,
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: 1},
			Center:      math2d.Vec2{X: 67, Y: 43},
			Radius:      16,
			Fill:        render.RGBA(1, 0.16, 0.06, 0.88),
			Stroke:      render.RGBA8(255, 220, 107, 255),
			StrokeWidth: 1.5,
		},
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Rect:        math2d.NewRect(math2d.Vec2{X: 43, Y: 12}, math2d.Vec2{X: 15, Y: 12}),
			Fill:        render.RGBA(1, 0.94, 0.45, 1),
		},
		render.DrawLine{
			BaseCommand: render.BaseCommand{LayerValue: 3},
			From:        math2d.Vec2{X: 9, Y: 61},
			To:          math2d.Vec2{X: 88, Y: 12},
			Width:       2,
			Color:       render.RGBA(0.47, 0.95, 0.78, 0.85),
		},
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: 100},
			Rect:        math2d.NewRect(math2d.Vec2{X: 8, Y: 61}, math2d.Vec2{X: 84, Y: 7}),
			Fill:        render.RGBA(0.03, 0.05, 0.1, 0.94),
			Stroke:      render.RGBA8(235, 244, 255, 255),
			StrokeWidth: 1,
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 101},
			Text:        "POST FX / HUD",
			Position:    math2d.Vec2{X: 13, Y: 65.5},
			Size:        4.5,
			Color:       render.RGBA8(235, 244, 255, 255),
		},
		render.PostChain{
			BaseCommand: render.BaseCommand{LayerValue: 99},
			Effects: []render.PostEffect{
				render.ColorGrade{Brightness: 0.02, Contrast: 0.08, Saturation: 0.1},
				render.Bloom{Threshold: 0.62, Intensity: 0.45, Passes: 2},
				render.Vignette{Color: render.RGBA8(0, 0, 0, 255), Strength: 0.35, Softness: 0.6},
				render.LensRain{Density: 0.4, Size: 32, Speed: 0.15, Refraction: 0.2, Seed: 0x0885eed},
			},
			BelowLayer: 100,
			Time:       time,
		},
	}
}
