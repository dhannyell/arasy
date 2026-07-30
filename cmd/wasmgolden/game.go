// Package wasmgolden defines the deterministic visual fixture used by the
// Canvas2D and WebGPU browser golden tests.
package wasmgolden

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/render/vector"
	"github.com/dhannyell/arasy/runtime"
)

type game struct {
	assets               asset.Manifest
	disableDefaultSplash bool
}

var curvedStrokeCommand = func() render.DrawTriangles {
	var builder vector.Builder
	builder.MoveTo(math2d.Vec2{X: 54, Y: 29})
	builder.CubicTo(
		math2d.Vec2{X: 63, Y: 13},
		math2d.Vec2{X: 86, Y: 13},
		math2d.Vec2{X: 96, Y: 29},
	)
	path, err := builder.Build()
	if err != nil {
		panic(err)
	}
	commands, err := path.AppendStroke(nil, render.BaseCommand{LayerValue: 4}, vector.StrokeStyle{
		Color: render.RGBA8(159, 132, 246, 255), Width: 2.5,
		LineCap: vector.CapRound, LineJoin: vector.JoinMiter, Tolerance: 0.1,
	})
	if err != nil {
		panic(err)
	}
	return commands[0].(render.DrawTriangles)
}()

// New builds the deterministic game fixture shared by both WASM renderers.
func New() (runtime.Game, error) {
	return newGoldenGame(true)
}

// NewBoot builds the canonical fixture with the default engine splash enabled.
// The browser boot test uses it to exercise the poster-to-canvas handoff while
// the screenshot fixtures keep skipping boot for deterministic capture speed.
func NewBoot() (runtime.Game, error) {
	return newGoldenGame(false)
}

func newGoldenGame(disableDefaultSplash bool) (runtime.Game, error) {
	manifest, err := goldenManifest()
	if err != nil {
		return nil, err
	}
	return &game{
		assets:               manifest,
		disableDefaultSplash: disableDefaultSplash,
	}, nil
}

func (g *game) Setup() runtime.Config {
	return runtime.Config{
		SimulationHz: 60,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
			Aspect:   render.AspectStretch,
		},
		Assets:               g.assets,
		DisableDefaultSplash: g.disableDefaultSplash,
		Clear:                render.RGBA8(14, 18, 27, 255),
	}
}

func (*game) Init(*runtime.UpdateContext) {}

func (*game) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (g *game) Render(ctx *runtime.RenderContext) []render.Command {
	texture := ctx.Assets.Texture("checker")
	clip := math2d.NewRect(math2d.Vec2{X: 4, Y: 4}, math2d.Vec2{X: 96, Y: 71})

	return []render.Command{
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: 0},
			Rect:        math2d.NewRect(math2d.Vec2{X: 4, Y: 4}, math2d.Vec2{X: 96, Y: 71}),
			Fill:        render.RGBA8(25, 35, 52, 255),
		},
		render.DrawRect{
			BaseCommand:  render.BaseCommand{LayerValue: 1, Clip: clip},
			Rect:         math2d.NewRect(math2d.Vec2{X: 8, Y: 8}, math2d.Vec2{X: 38, Y: 27}),
			Fill:         render.RGBA(0.12, 0.62, 0.78, 0.7),
			Stroke:       render.RGBA8(235, 244, 255, 255),
			StrokeWidth:  1.25,
			CornerRadius: 3,
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Center:      math2d.Vec2{X: 27, Y: 18},
			Radius:      8,
			Fill:        render.RGBA(1, 0.32, 0.18, 0.72),
			Stroke:      render.RGBA8(255, 210, 70, 255),
			StrokeWidth: 1,
		},
		render.DrawPoint{
			BaseCommand: render.BaseCommand{LayerValue: 3},
			Center:      math2d.Vec2{X: 27, Y: 18},
			Radius:      1.4,
			Color:       render.RGBA8(255, 255, 255, 255),
		},
		render.DrawLine{
			BaseCommand: render.BaseCommand{LayerValue: 4, Clip: clip},
			From:        math2d.Vec2{X: 6, Y: 35},
			To:          math2d.Vec2{X: 47, Y: 49},
			Width:       1.5,
			Color:       render.RGBA8(175, 135, 255, 230),
		},
		render.DrawArc{
			BaseCommand: render.BaseCommand{LayerValue: 5},
			Center:      math2d.Vec2{X: 28, Y: 48},
			Radius:      10,
			StartAngle:  5.25,
			EndAngle:    0.9,
			Width:       2,
			Color:       render.RGBA8(255, 93, 184, 255),
		},
		render.DrawPolygon{
			BaseCommand: render.BaseCommand{LayerValue: 6},
			Points: []math2d.Vec2{
				{X: 8, Y: 55}, {X: 19, Y: 51}, {X: 25, Y: 59},
				{X: 18, Y: 67}, {X: 7, Y: 64},
			},
			Fill:        render.RGBA(0.25, 0.82, 0.46, 0.68),
			Stroke:      render.RGBA8(210, 255, 220, 255),
			StrokeWidth: 1,
		},
		render.DrawEllipse{
			BaseCommand: render.BaseCommand{LayerValue: 7},
			Center:      math2d.Vec2{X: 38, Y: 63},
			RadiusX:     8,
			RadiusY:     4,
			Fill:        render.RGBA(0.82, 0.72, 1, 0.78),
			Stroke:      render.RGBA8(255, 255, 255, 255),
			StrokeWidth: 1.2,
		},
		render.DrawTriangles{
			BaseCommand: render.BaseCommand{LayerValue: 8, Clip: clip},
			Vertices: []math2d.Vec2{
				{X: 69, Y: 61}, {X: 83, Y: 61}, {X: 88, Y: 70}, {X: 64, Y: 70},
			},
			Indices: []uint32{0, 1, 2, 0, 2, 3},
			Color:   render.RGBA(0.95, 0.48, 0.18, 0.82),
		},
		// The inner circle sits exactly on the clip-space horizontal axis.
		// A non-identical 0/2pi fan closure shows as a radial MSAA seam.
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: 8},
			Center:      math2d.Vec2{X: 44, Y: 37.5},
			Radius:      8,
			Fill:        render.RGBA8(243, 235, 221, 255),
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: 9},
			Center:      math2d.Vec2{X: 44, Y: 37.5},
			Radius:      6,
			Fill:        render.RGBA8(2, 11, 20, 255),
		},
		curvedStrokeCommand,
		render.DrawImage{
			BaseCommand: render.BaseCommand{LayerValue: 1, Clip: clip},
			Texture:     texture,
			Dest:        math2d.NewRect(math2d.Vec2{X: 55, Y: 8}, math2d.Vec2{X: 18, Y: 18}),
		},
		render.DrawImage{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Texture:     texture,
			Dest:        math2d.NewRect(math2d.Vec2{X: 78, Y: 8}, math2d.Vec2{X: 16, Y: 18}),
			Src:         math2d.NewRect(math2d.Vec2{X: 2, Y: 0}, math2d.Vec2{X: 8, Y: 6}),
			Tint:        render.RGBA(1, 0.5, 0.65, 0.82),
			Rotation:    0.28,
		},
		render.DrawImageQuad{
			BaseCommand: render.BaseCommand{LayerValue: 3},
			Texture:     texture,
			Corners: [4]math2d.Vec2{
				{X: 59, Y: 34}, {X: 88, Y: 31}, {X: 95, Y: 58}, {X: 53, Y: 62},
			},
			Tint: render.RGBA(0.72, 0.92, 1, 0.9),
		},
	}
}

func goldenManifest() (asset.Manifest, error) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	colors := [4]color.RGBA{
		{R: 245, G: 76, B: 91, A: 255},
		{R: 61, G: 198, B: 255, A: 255},
		{R: 255, G: 211, B: 79, A: 255},
		{R: 113, G: 235, B: 151, A: 255},
	}
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, colors[(x/2+y/2)%len(colors)])
		}
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return asset.Manifest{}, err
	}

	return asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"textures":[{"name":"checker","file":"checker.png"}]}`), Mode: 0o644},
		"checker.png":   {Data: encoded.Bytes(), Mode: 0o644},
	})
}
