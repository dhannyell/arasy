package wasmgolden

import (
	"image/color"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

const cameraPassBatch render.SpriteBatchID = 1

// cameraPassesGame keeps one retained batch visible through two cameras.
// Its edge sprite must stay inside the zoomed pass viewport.
type cameraPassesGame struct {
	assets asset.Manifest
}

// NewCameraPassesGolden returns the two-camera browser conformance fixture.
func NewCameraPassesGolden() (runtime.Game, error) {
	manifest, err := cameraPassesManifest()
	if err != nil {
		return nil, err
	}
	return &cameraPassesGame{assets: manifest}, nil
}

func (g *cameraPassesGame) Setup() runtime.Config {
	return runtime.Config{
		SimulationHz:         60,
		Camera:               cameraPassBaseCamera(),
		Assets:               g.assets,
		DisableDefaultSplash: true,
		Clear:                render.RGBA8(12, 16, 24, 255),
	}
}

func (g *cameraPassesGame) Init(ctx *runtime.UpdateContext) {
	ctx.Batches.UpdateSprites(cameraPassBatch, []render.SpriteInstance{
		{
			Texture: g.assets.Texture("tile"),
			Dest:    math2d.NewRect(math2d.Vec2{X: -6, Y: 8}, math2d.Vec2{X: 20, Y: 16}),
		},
		{
			Texture: g.assets.Texture("tile"),
			Dest:    math2d.NewRect(math2d.Vec2{X: 32, Y: 28}, math2d.Vec2{X: 18, Y: 18}),
		},
	})
}

func (*cameraPassesGame) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (*cameraPassesGame) Render(ctx *runtime.RenderContext) []render.Command {
	ctx.SetCamera(cameraPassBaseCamera())
	ctx.AddPass(cameraPassZoomCamera(), cameraPassZoomCommands())
	return cameraPassBaseCommands()
}

func cameraPassBaseCamera() render.Camera {
	return render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
		Aspect:   render.AspectStretch,
	}
}

func cameraPassZoomCamera() render.Camera {
	return render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 60, Y: 52}),
		Viewport: math2d.NewRect(math2d.Vec2{X: 200, Y: 16}, math2d.Vec2{X: 112, Y: 96}),
		Aspect:   render.AspectStretch,
	}
}

func cameraPassBaseCommands() []render.Command {
	return []render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 60, Y: 75}),
			Fill: render.RGBA8(29, 49, 72, 255),
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: 1},
			Center:      math2d.Vec2{X: 42, Y: 49},
			Radius:      14,
			Fill:        render.RGBA8(89, 193, 176, 255),
		},
		render.DrawSpriteBatch{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Batch:       cameraPassBatch,
		},
	}
}

func cameraPassZoomCommands() []render.Command {
	return []render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: -12, Y: -12}, math2d.Vec2{X: 84, Y: 76}),
			Fill: render.RGBA8(50, 32, 77, 255),
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: 1},
			Center:      math2d.Vec2{X: 42, Y: 49},
			Radius:      14,
			Fill:        render.RGBA8(242, 132, 96, 255),
		},
		render.DrawSpriteBatch{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Batch:       cameraPassBatch,
		},
	}
}

func cameraPassesManifest() (asset.Manifest, error) {
	tile, err := encodeGoldenTexture([4]color.RGBA{
		{R: 242, G: 84, B: 105, A: 255},
		{R: 242, G: 84, B: 105, A: 255},
		{R: 242, G: 84, B: 105, A: 255},
		{R: 242, G: 84, B: 105, A: 255},
	})
	if err != nil {
		return asset.Manifest{}, err
	}

	return asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"textures":[{"name":"tile","file":"tile.png"}]}`), Mode: 0o644},
		"tile.png":      {Data: tile, Mode: 0o644},
	})
}
