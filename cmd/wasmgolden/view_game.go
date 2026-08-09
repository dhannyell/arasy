package wasmgolden

import (
	"image/color"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// viewConformanceGame is the camera fixture for retained sprites, text, and
// particles. Raster and browser tests use the same scene.
type viewConformanceGame struct {
	assets      asset.Manifest
	renderCount uint64
}

// NewViewConformanceGolden returns the camera conformance fixture.
func NewViewConformanceGolden() (runtime.Game, error) {
	manifest, err := viewConformanceManifest()
	if err != nil {
		return nil, err
	}
	return &viewConformanceGame{assets: manifest}, nil
}

func (g *viewConformanceGame) Setup() runtime.Config {
	return runtime.Config{
		SimulationHz: 60,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
			Aspect:   render.AspectStretch,
			View:     math2d.Affine{A: 0.5, B: 0.25, C: -0.5, D: 0.25, E: 50, F: 25},
		},
		Assets:               g.assets,
		DisableDefaultSplash: true,
		Clear:                render.RGBA8(12, 16, 24, 255),
	}
}

func (g *viewConformanceGame) Init(ctx *runtime.UpdateContext) {
	ctx.Batches.UpdateSprites(1, []render.SpriteInstance{
		{
			Texture: g.assets.Texture("tile"),
			Dest:    math2d.NewRect(math2d.Vec2{X: 42, Y: 37}, math2d.Vec2{X: 12, Y: 8}),
		},
		{
			Texture:  g.assets.Texture("tile"),
			Dest:     math2d.NewRect(math2d.Vec2{X: 58, Y: 37}, math2d.Vec2{X: 12, Y: 8}),
			Rotation: 0.3,
			Tint:     render.RGBA(1, 0.65, 0.3, 0.9),
		},
	})
	ctx.Batches.UpdateText(1, []render.TextLabel{{
		Position: math2d.Vec2{X: 38, Y: 61},
		Text:     "VIEW BATCH",
		Color:    render.RGBA8(235, 244, 255, 255),
		Size:     6,
		Baseline: render.BaselineTop,
	}})
}

func (*viewConformanceGame) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (g *viewConformanceGame) Render(ctx *runtime.RenderContext) []render.Command {
	burst := uint32(0)
	dt := 0.0
	if g.renderCount > 0 {
		burst = 1
	}
	if g.renderCount == 1 {
		dt = 0.2
	}
	g.renderCount++

	return []render.Command{
		render.DrawPolygon{
			BaseCommand: render.BaseCommand{LayerValue: 0},
			Points: []math2d.Vec2{
				{X: 20, Y: 50}, {X: 50, Y: 20}, {X: 80, Y: 50}, {X: 50, Y: 80},
			},
			Fill:        render.RGBA8(25, 35, 52, 255),
			Stroke:      render.RGBA8(72, 89, 116, 255),
			StrokeWidth: 1,
		},
		render.DrawSpriteBatch{BaseCommand: render.BaseCommand{LayerValue: 1}, Batch: 1},
		render.DrawTextBatch{BaseCommand: render.BaseCommand{LayerValue: 2}, Batch: 1},
		render.DrawParticles{
			BaseCommand: render.BaseCommand{LayerValue: 3},
			Effect:      ctx.Assets.Effect("spark"),
			Emitter:     1,
			Pos:         math2d.Vec2{X: 50, Y: 50},
			Burst:       burst,
			Dt:          dt,
			Seed:        151,
		},
	}
}

func viewConformanceManifest() (asset.Manifest, error) {
	tile, err := encodeGoldenTexture([4]color.RGBA{
		{R: 242, G: 84, B: 105, A: 255},
		{R: 255, G: 213, B: 94, A: 255},
		{R: 105, G: 224, B: 157, A: 255},
		{R: 76, G: 175, B: 255, A: 255},
	})
	if err != nil {
		return asset.Manifest{}, err
	}
	white, err := encodeGoldenTexture([4]color.RGBA{
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 255, B: 255, A: 255},
		{R: 255, G: 255, B: 255, A: 255},
	})
	if err != nil {
		return asset.Manifest{}, err
	}

	const manifest = `{
		"version": 1,
		"textures": [
			{"name":"tile","file":"tile.png"},
			{"name":"particle-white","file":"white.png"}
		],
		"effects": [{
			"name":"spark","texture":"particle-white","capacity":8,
			"emit_rate":0,"burst_size":1,"lifetime":[1,1],"speed":[0,0],
			"size_start":[10,10],"size_end":[10,10],"drag":1,
			"color_start":[1,0.2,0.05,1],"color_end":[0.1,0.45,1,1]
		}]
	}`

	return asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(manifest), Mode: 0o644},
		"tile.png":      {Data: tile, Mode: 0o644},
		"white.png":     {Data: white, Mode: 0o644},
	})
}
