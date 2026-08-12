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
	"github.com/dhannyell/arasy/runtime"
)

const rttRetainedBatch render.SpriteBatchID = 41

type rttGoldenGame struct {
	assets asset.Manifest
}

// NewRTTGolden returns the render-to-texture conformance fixture using
// individual GPU texture resources.
func NewRTTGolden() (runtime.Game, error) {
	return newRTTGolden(false)
}

// NewRTTArrayGolden returns the render-to-texture conformance fixture with a
// matching static texture that keeps the WebGPU sprite-array path enabled.
func NewRTTArrayGolden() (runtime.Game, error) {
	return newRTTGolden(true)
}

func newRTTGolden(array bool) (runtime.Game, error) {
	manifest, err := rttManifest(array)
	if err != nil {
		return nil, err
	}
	return &rttGoldenGame{assets: manifest}, nil
}

func (g *rttGoldenGame) Setup() runtime.Config {
	return runtime.Config{
		SimulationHz: 60,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
			Aspect:   render.AspectStretch,
		},
		Assets:               g.assets,
		DisableDefaultSplash: true,
		Clear:                render.RGBA8(10, 15, 27, 255),
	}
}

func (g *rttGoldenGame) Init(ctx *runtime.UpdateContext) {
	ctx.Batches.UpdateSprites(rttRetainedBatch, []render.SpriteInstance{{
		Texture: g.assets.Texture("target"),
		Dest:    math2d.NewRect(math2d.Vec2{X: 84, Y: 54}, math2d.Vec2{X: 8, Y: 8}),
	}})
}

func (*rttGoldenGame) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (g *rttGoldenGame) Render(ctx *runtime.RenderContext) []render.Command {
	target := g.assets.Texture("target")
	ctx.AddTargetPass(rttTargetCamera(), rttTargetCommands(), target)

	return []render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: 4, Y: 4}, math2d.Vec2{X: 92, Y: 67}),
			Fill: render.RGBA8(27, 39, 63, 255),
		},
		render.DrawImage{
			Texture: target,
			Dest:    math2d.NewRect(math2d.Vec2{X: 10, Y: 11}, math2d.Vec2{X: 7, Y: 7}),
		},
		render.DrawImageQuad{
			Texture: target,
			Corners: [4]math2d.Vec2{
				{X: 30, Y: 10}, {X: 72, Y: 15}, {X: 67, Y: 46}, {X: 25, Y: 39},
			},
		},
		render.DrawSpriteBatch{Batch: rttRetainedBatch},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 101},
			Position:    math2d.Vec2{X: 10, Y: 66},
			Text:        "RTT / SPRITE / QUAD / BATCH",
			Color:       render.RGBA8(239, 246, 255, 255),
			Size:        4,
			Baseline:    render.BaselineBottom,
		},
		render.PostChain{
			BaseCommand: render.BaseCommand{LayerValue: 100},
			Effects: []render.PostEffect{
				render.ColorGrade{Brightness: 0.01, Contrast: 0.04, Saturation: 0.05},
			},
			BelowLayer: 100,
		},
	}
}

func rttTargetCamera() render.Camera {
	return render.Camera{
		World:  math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 64, Y: 64}),
		Aspect: render.AspectStretch,
	}
}

func rttTargetCommands() []render.Command {
	commands := make([]render.Command, 0, 66)
	commands = append(commands, render.DrawRect{
		Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 64, Y: 64}),
		Fill: render.RGBA8(18, 25, 43, 255),
	})
	colors := [...]render.Color{
		render.RGBA8(246, 86, 108, 255),
		render.RGBA8(74, 215, 221, 255),
		render.RGBA8(255, 211, 92, 255),
		render.RGBA8(116, 83, 244, 255),
	}
	for y := range 8 {
		for x := range 8 {
			commands = append(commands, render.DrawRect{
				Rect: math2d.NewRect(
					math2d.Vec2{X: float64(x * 8), Y: float64(y * 8)},
					math2d.Vec2{X: 8, Y: 8},
				),
				Fill: colors[(x+y)%len(colors)],
			})
		}
	}
	commands = append(commands, render.DrawCircle{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Center:      math2d.Vec2{X: 32, Y: 32},
		Radius:      13,
		Fill:        render.RGBA8(245, 248, 255, 224),
	})
	return commands
}

func rttManifest(array bool) (asset.Manifest, error) {
	size := 32
	if array {
		size = 64
	}
	peer, err := encodeRTTPeer(size)
	if err != nil {
		return asset.Manifest{}, err
	}
	return asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"textures":[{"name":"peer","file":"peer.png"}],"dynamic":[{"name":"target","w":64,"h":64}]}`), Mode: 0o644},
		"peer.png":      {Data: peer, Mode: 0o644},
	})
}

func encodeRTTPeer(size int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			img.SetRGBA(x, y, color.RGBA{R: uint8(40 + x%32), G: uint8(60 + y%32), B: 90, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}
