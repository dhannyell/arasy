package wasmgolden

import (
	"image/color"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// Parallax fixture layers: a pinned sky, a half-speed hill band, and the
// gameplay plane at full scroll.
const (
	parallaxSkyLayer  = 0
	parallaxHillLayer = 1
	parallaxGameLayer = 2
)

// parallaxConformanceGame is the layer-parallax fixture: streaming shapes, a
// retained sprite batch and a retained text batch, each on a layer with its
// own scroll factor, under a camera scrolled away from the parallax anchor.
// Raster and browser tests use the same scene.
type parallaxConformanceGame struct {
	assets asset.Manifest
}

// NewParallaxConformanceGolden returns the parallax conformance fixture.
func NewParallaxConformanceGolden() (runtime.Game, error) {
	manifest, err := parallaxConformanceManifest()
	if err != nil {
		return nil, err
	}
	return &parallaxConformanceGame{assets: manifest}, nil
}

func (g *parallaxConformanceGame) Setup() runtime.Config {
	return runtime.Config{
		SimulationHz: 60,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{X: 40, Y: 10}, math2d.Vec2{X: 100, Y: 100}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
			Aspect:   render.AspectStretch,
			Parallax: []render.LayerParallax{
				{FromLayer: parallaxSkyLayer, ToLayer: parallaxSkyLayer, Factor: math2d.Vec2{X: 0, Y: 0}},
				{FromLayer: parallaxHillLayer, ToLayer: parallaxHillLayer, Factor: math2d.Vec2{X: 0.5, Y: 1}},
			},
		},
		Assets:               g.assets,
		DisableDefaultSplash: true,
		Clear:                render.RGBA8(16, 22, 34, 255),
	}
}

func (g *parallaxConformanceGame) Init(ctx *runtime.UpdateContext) {
	ctx.Batches.UpdateSprites(1, []render.SpriteInstance{
		{
			Texture: g.assets.Texture("tile"),
			Dest:    math2d.NewRect(math2d.Vec2{X: 55, Y: 45}, math2d.Vec2{X: 16, Y: 12}),
		},
		{
			Texture:  g.assets.Texture("tile"),
			Dest:     math2d.NewRect(math2d.Vec2{X: 100, Y: 52}, math2d.Vec2{X: 12, Y: 9}),
			Rotation: 0.25,
			Tint:     render.RGBA(0.6, 0.9, 1, 0.9),
		},
	})
	ctx.Batches.UpdateText(1, []render.TextLabel{{
		Position: math2d.Vec2{X: 58, Y: 84},
		Text:     "PARALLAX",
		Color:    render.RGBA8(255, 230, 160, 255),
		Size:     7,
		Baseline: render.BaselineTop,
	}})
}

func (*parallaxConformanceGame) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (g *parallaxConformanceGame) Render(ctx *runtime.RenderContext) []render.Command {
	return []render.Command{
		// Pinned sky: stays put no matter how far the camera scrolls.
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: parallaxSkyLayer},
			Rect:        math2d.NewRect(math2d.Vec2{X: -20, Y: -10}, math2d.Vec2{X: 80, Y: 60}),
			Fill:        render.RGBA8(38, 54, 84, 255),
		},
		// Half-speed hill band: streaming shape plus retained sprites.
		render.DrawEllipse{
			BaseCommand: render.BaseCommand{LayerValue: parallaxHillLayer},
			Center:      math2d.Vec2{X: 80, Y: 75},
			RadiusX:     60,
			RadiusY:     28,
			Fill:        render.RGBA8(52, 84, 62, 255),
		},
		render.DrawSpriteBatch{BaseCommand: render.BaseCommand{LayerValue: parallaxHillLayer}, Batch: 1},
		// Gameplay plane at full scroll.
		render.DrawPolygon{
			BaseCommand: render.BaseCommand{LayerValue: parallaxGameLayer},
			Points: []math2d.Vec2{
				{X: 40, Y: 110}, {X: 140, Y: 110}, {X: 140, Y: 80}, {X: 40, Y: 80},
			},
			Fill:        render.RGBA8(94, 70, 52, 255),
			Stroke:      render.RGBA8(140, 110, 80, 255),
			StrokeWidth: 1,
		},
		render.DrawTextBatch{BaseCommand: render.BaseCommand{LayerValue: parallaxGameLayer}, Batch: 1},
	}
}

func parallaxConformanceManifest() (asset.Manifest, error) {
	tile, err := encodeGoldenTexture([4]color.RGBA{
		{R: 242, G: 84, B: 105, A: 255},
		{R: 255, G: 213, B: 94, A: 255},
		{R: 105, G: 224, B: 157, A: 255},
		{R: 76, G: 175, B: 255, A: 255},
	})
	if err != nil {
		return asset.Manifest{}, err
	}

	const manifest = `{
		"version": 1,
		"textures": [
			{"name":"tile","file":"tile.png"}
		]
	}`

	return asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(manifest), Mode: 0o644},
		"tile.png":      {Data: tile, Mode: 0o644},
	})
}
