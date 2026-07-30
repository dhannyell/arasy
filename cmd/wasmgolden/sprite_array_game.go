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

type spriteArrayGame struct {
	assets asset.Manifest
}

// NewSpriteArrayGolden builds a fixture with two equal-sized textures. WebGPU
// must place them in one array resource while retaining single-layer 2D views
// for DrawImageQuad; Canvas2D remains the visual reference.
func NewSpriteArrayGolden() (runtime.Game, error) {
	manifest, err := spriteArrayManifest()
	if err != nil {
		return nil, err
	}
	return &spriteArrayGame{assets: manifest}, nil
}

func (g *spriteArrayGame) Setup() runtime.Config {
	return runtime.Config{
		SimulationHz: 60,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
			Aspect:   render.AspectStretch,
		},
		Assets:               g.assets,
		DisableDefaultSplash: true,
		Clear:                render.RGBA8(11, 16, 28, 255),
	}
}

func (*spriteArrayGame) Init(*runtime.UpdateContext) {}

func (*spriteArrayGame) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (g *spriteArrayGame) Render(ctx *runtime.RenderContext) []render.Command {
	first := g.assets.Texture("first")
	second := g.assets.Texture("second")
	dynamic := g.assets.Texture("third_dynamic")
	ctx.UpdateTexture(dynamic, goldenTextureImage([4]color.RGBA{
		{R: 255, G: 145, B: 77, A: 255},
		{R: 124, G: 242, B: 197, A: 255},
		{R: 105, G: 137, B: 255, A: 255},
		{R: 250, G: 246, B: 235, A: 255},
	}))
	clip := math2d.NewRect(math2d.Vec2{X: 8, Y: 8}, math2d.Vec2{X: 84, Y: 55})

	return []render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: 4, Y: 4}, math2d.Vec2{X: 92, Y: 67}),
			Fill: render.RGBA8(28, 38, 58, 255),
		},
		render.DrawImage{
			Texture: first,
			Dest:    math2d.NewRect(math2d.Vec2{X: 11, Y: 12}, math2d.Vec2{X: 34, Y: 30}),
		},
		render.DrawImage{
			BaseCommand: render.BaseCommand{Clip: clip},
			Texture:     second,
			Dest:        math2d.NewRect(math2d.Vec2{X: 34, Y: 18}, math2d.Vec2{X: 38, Y: 32}),
			Rotation:    0.31,
		},
		render.DrawImage{
			Texture: first,
			Dest:    math2d.NewRect(math2d.Vec2{X: 65, Y: 10}, math2d.Vec2{X: 27, Y: 27}),
			Tint:    render.RGBA(0.72, 1, 0.82, 0.86),
		},
		render.DrawImageQuad{
			Texture: second,
			Corners: [4]math2d.Vec2{
				{X: 13, Y: 48}, {X: 43, Y: 44}, {X: 49, Y: 68}, {X: 9, Y: 66},
			},
		},
		render.DrawText{
			Position: math2d.Vec2{X: 51, Y: 43},
			Text:     "ARRAY",
			Color:    render.RGBA8(245, 248, 255, 255),
			Size:     5,
			Baseline: render.BaselineTop,
		},
		render.DrawImage{
			Texture: second,
			Dest:    math2d.NewRect(math2d.Vec2{X: 58, Y: 47}, math2d.Vec2{X: 31, Y: 20}),
			Src:     math2d.NewRect(math2d.Vec2{X: 4, Y: 0}, math2d.Vec2{X: 8, Y: 12}),
		},
		render.DrawText{
			Position: math2d.Vec2{X: 51, Y: 67},
			Text:     "LAYER",
			Color:    render.RGBA8(255, 222, 112, 255),
			Size:     4,
			Baseline: render.BaselineBottom,
		},
		render.DrawImage{
			Texture: dynamic,
			Dest:    math2d.NewRect(math2d.Vec2{X: 84, Y: 51}, math2d.Vec2{X: 9, Y: 14}),
		},
	}
}

func spriteArrayManifest() (asset.Manifest, error) {
	first, err := encodeGoldenTexture([4]color.RGBA{
		{R: 242, G: 84, B: 105, A: 255},
		{R: 255, G: 213, B: 94, A: 255},
		{R: 105, G: 224, B: 157, A: 255},
		{R: 76, G: 175, B: 255, A: 255},
	})
	if err != nil {
		return asset.Manifest{}, err
	}
	second, err := encodeGoldenTexture([4]color.RGBA{
		{R: 116, G: 83, B: 244, A: 255},
		{R: 237, G: 92, B: 190, A: 255},
		{R: 74, G: 215, B: 221, A: 255},
		{R: 245, G: 245, B: 250, A: 255},
	})
	if err != nil {
		return asset.Manifest{}, err
	}

	return asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"textures":[{"name":"first","file":"first.png"},{"name":"second","file":"second.png"}],"dynamic":[{"name":"third_dynamic","w":16,"h":16}]}`), Mode: 0o644},
		"first.png":     {Data: first, Mode: 0o644},
		"second.png":    {Data: second, Mode: 0o644},
	})
}

func encodeGoldenTexture(colors [4]color.RGBA) ([]byte, error) {
	img := goldenTextureImage(colors)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

func goldenTextureImage(colors [4]color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			img.SetRGBA(x, y, colors[(x/4+y/4)%len(colors)])
		}
	}
	return img
}
