package wasmgolden

import (
	"image/color"
	"math"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// flipConformanceGame covers the four source-flip states in streaming and
// retained image paths. Rotated cells pin the flip-before-rotation contract.
type flipConformanceGame struct {
	assets asset.Manifest
}

// NewFlipConformanceGolden returns the sprite-flip conformance fixture.
func NewFlipConformanceGolden() (runtime.Game, error) {
	manifest, err := flipConformanceManifest()
	if err != nil {
		return nil, err
	}
	return &flipConformanceGame{assets: manifest}, nil
}

func (g *flipConformanceGame) Setup() runtime.Config {
	return runtime.Config{
		SimulationHz: 60,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
			Aspect:   render.AspectStretch,
		},
		Assets:               g.assets,
		DisableDefaultSplash: true,
		Clear:                render.RGBA8(12, 16, 24, 255),
	}
}

func (g *flipConformanceGame) Init(ctx *runtime.UpdateContext) {
	texture := g.assets.Texture("quadrants")
	ctx.Batches.UpdateSprites(1, []render.SpriteInstance{
		{Texture: texture, Dest: math2d.NewRect(math2d.Vec2{X: 8, Y: 42}, math2d.Vec2{X: 5, Y: 5})},
		{Texture: texture, Dest: math2d.NewRect(math2d.Vec2{X: 16, Y: 42}, math2d.Vec2{X: 5, Y: 5}), Flip: render.FlipX},
		{Texture: texture, Dest: math2d.NewRect(math2d.Vec2{X: 24, Y: 42}, math2d.Vec2{X: 5, Y: 5}), Flip: render.FlipY},
		{Texture: texture, Dest: math2d.NewRect(math2d.Vec2{X: 32, Y: 42}, math2d.Vec2{X: 5, Y: 5}), Flip: render.FlipX | render.FlipY, Rotation: math.Pi / 2},
	})
}

func (*flipConformanceGame) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (g *flipConformanceGame) Render(ctx *runtime.RenderContext) []render.Command {
	texture := g.assets.Texture("quadrants")
	return []render.Command{
		render.DrawImage{Texture: texture, Dest: math2d.NewRect(math2d.Vec2{X: 8, Y: 12}, math2d.Vec2{X: 5, Y: 5})},
		render.DrawImage{Texture: texture, Dest: math2d.NewRect(math2d.Vec2{X: 16, Y: 12}, math2d.Vec2{X: 5, Y: 5}), Flip: render.FlipX},
		render.DrawImage{Texture: texture, Dest: math2d.NewRect(math2d.Vec2{X: 24, Y: 12}, math2d.Vec2{X: 5, Y: 5}), Flip: render.FlipY},
		render.DrawImage{Texture: texture, Dest: math2d.NewRect(math2d.Vec2{X: 32, Y: 12}, math2d.Vec2{X: 5, Y: 5}), Flip: render.FlipX | render.FlipY, Rotation: math.Pi / 2},
		render.DrawSpriteBatch{BaseCommand: render.BaseCommand{LayerValue: 1}, Batch: 1},
	}
}

func flipConformanceManifest() (asset.Manifest, error) {
	quadrants, err := encodeGoldenTexture([4]color.RGBA{
		{R: 242, G: 84, B: 105, A: 255},
		{R: 255, G: 213, B: 94, A: 255},
		{R: 105, G: 224, B: 157, A: 255},
		{R: 76, G: 175, B: 255, A: 255},
	})
	if err != nil {
		return asset.Manifest{}, err
	}
	const manifest = `{"version":1,"textures":[{"name":"quadrants","file":"quadrants.png"}]}`
	return asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(manifest), Mode: 0o644},
		"quadrants.png": {Data: quadrants, Mode: 0o644},
	})
}
