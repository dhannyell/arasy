package wasmgolden

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

//go:embed testdata/fonts/Go-Regular.ttf
var retainedBatchesTTF []byte

const (
	retainedStaticSprites   render.SpriteBatchID = 1
	retainedStyledSprite    render.SpriteBatchID = 2
	retainedAnimatedSprite  render.SpriteBatchID = 3
	retainedOffscreenSprite render.SpriteBatchID = 4
	retainedEphemeralSprite render.SpriteBatchID = 5

	retainedBuiltinText render.TextBatchID = 1
	retainedTTFText     render.TextBatchID = 2
)

// retainedBatchesGame is the cross-backend fixture for retained sprite/text
// batches. It exercises, in one frame: static sprites split
// across two homogeneous textures, a clipped/rotated/tinted batch, a
// flip-book+bounce+pulse batch evaluated at a fixed time, retained text with
// both the built-in font and an external TTF, a batch published entirely
// outside the camera view, and a batch replaced then deleted before the
// screenshot is captured.
type retainedBatchesGame struct {
	assets      asset.Manifest
	renderCount uint64
}

// NewRetainedBatchesGolden builds the retained-batch fixture used by the
// browser golden harness.
func NewRetainedBatchesGolden() (runtime.Game, error) {
	manifest, err := retainedBatchesManifest()
	if err != nil {
		return nil, err
	}
	return &retainedBatchesGame{assets: manifest}, nil
}

func (g *retainedBatchesGame) Setup() runtime.Config {
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

// Init publishes every stable batch once, matching the plan's preferred
// publication path: stable content in Init, presentation-only changes in
// Render.
func (g *retainedBatchesGame) Init(ctx *runtime.UpdateContext) {
	tileA := g.assets.Texture("tileA")
	tileB := g.assets.Texture("tileB")
	flipbook := g.assets.Texture("flipbook")

	staticInstances := make([]render.SpriteInstance, 0, 6)
	for i := 0; i < 6; i++ {
		tex := tileA
		if i%2 == 1 {
			tex = tileB
		}
		x := 4 + float64(i)*8
		staticInstances = append(staticInstances, render.SpriteInstance{
			Texture: tex,
			Dest:    math2d.NewRect(math2d.Vec2{X: x, Y: 4}, math2d.Vec2{X: 7, Y: 7}),
		})
	}
	ctx.Batches.UpdateSprites(retainedStaticSprites, staticInstances)

	ctx.Batches.UpdateSprites(retainedStyledSprite, []render.SpriteInstance{
		{
			// Straddles the batch clip's left edge; the clip must cut its
			// left portion away.
			Texture: tileA,
			Dest:    math2d.NewRect(math2d.Vec2{X: 52, Y: 10}, math2d.Vec2{X: 22, Y: 14}),
		},
		{
			Texture:  tileB,
			Dest:     math2d.NewRect(math2d.Vec2{X: 66, Y: 12}, math2d.Vec2{X: 18, Y: 14}),
			Rotation: 0.4,
			Tint:     render.RGBA(1, 0.55, 0.35, 0.85),
		},
	})

	animInstances := make([]render.SpriteInstance, 0, 4)
	for i := 0; i < 4; i++ {
		animInstances = append(animInstances, render.SpriteInstance{
			Texture: flipbook,
			Dest:    math2d.NewRect(math2d.Vec2{X: 6 + float64(i)*10, Y: 34}, math2d.Vec2{X: 8, Y: 8}),
			Src:     math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 8, Y: 8}),
			Animation: render.SpriteAnimation{
				Frames: 4,
				Rate:   1,
				Phase:  float32(i) * 0.2,
				Bounce: 4,
			},
		})
	}
	ctx.Batches.UpdateSprites(retainedAnimatedSprite, animInstances)

	// Entirely outside the camera's world rect (0,0)-(100,75); must draw
	// nothing and never crash the batch/culling path.
	ctx.Batches.UpdateSprites(retainedOffscreenSprite, []render.SpriteInstance{{
		Texture: tileA,
		Dest:    math2d.NewRect(math2d.Vec2{X: 5000, Y: 5000}, math2d.Vec2{X: 10, Y: 10}),
	}})

	// Both labels sit well clear of the world's bottom edge (75): Canvas2D
	// paints external TTF glyphs through the browser FontFace API while
	// WebGPU rasterizes them into its own atlas via asset/typeface, and
	// their line-height/descent do not match exactly (see AGENTS.md). A
	// label placed close to the edge can clip on one backend and not the
	// other; the margin here must hold on both.
	ctx.Batches.UpdateText(retainedBuiltinText, []render.TextLabel{{
		Position: math2d.Vec2{X: 4, Y: 54},
		Text:     "BUILTIN",
		Color:    render.RGBA8(235, 244, 255, 255),
		Size:     6,
		Baseline: render.BaselineTop,
	}})
	ctx.Batches.UpdateText(retainedTTFText, []render.TextLabel{{
		Position: math2d.Vec2{X: 4, Y: 63},
		Text:     "TTF Retained",
		Color:    render.RGBA8(255, 214, 120, 255),
		Size:     5,
		Font:     g.assets.Font("retained-ttf"),
		Baseline: render.BaselineTop,
	}})
}

func (*retainedBatchesGame) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (g *retainedBatchesGame) Render(ctx *runtime.RenderContext) []render.Command {
	// Replace on the first frame, then delete before any frame that could be
	// captured; the draw command below keeps referencing the ID so a
	// backend that failed to release the batch would still show its quad.
	if g.renderCount == 0 {
		ctx.Batches.UpdateSprites(retainedEphemeralSprite, []render.SpriteInstance{{
			Texture: g.assets.Texture("tileB"),
			Dest:    math2d.NewRect(math2d.Vec2{X: 84, Y: 54}, math2d.Vec2{X: 12, Y: 10}),
		}})
	} else {
		ctx.Batches.DeleteSprites(retainedEphemeralSprite)
	}
	g.renderCount++

	clip := math2d.NewRect(math2d.Vec2{X: 60, Y: 4}, math2d.Vec2{X: 28, Y: 26})

	return []render.Command{
		render.DrawSpriteBatch{Batch: retainedStaticSprites},
		render.DrawSpriteBatch{
			BaseCommand: render.BaseCommand{Clip: clip},
			Batch:       retainedStyledSprite,
		},
		render.DrawSpriteBatch{
			Batch: retainedAnimatedSprite,
			Animation: render.SpriteBatchAnimation{
				// Fixed render-clock value: the animation must be
				// deterministic regardless of when Playwright captures the
				// frame.
				Time:      0.42,
				BounceHz:  1,
				Intensity: 1,
				Pulse: render.SpritePulse{
					Center: math2d.Vec2{X: 16, Y: 34},
					Radius: 6,
					Width:  6,
					Boost:  1.5,
				},
			},
		},
		render.DrawSpriteBatch{Batch: retainedOffscreenSprite},
		render.DrawSpriteBatch{Batch: retainedEphemeralSprite},
		render.DrawTextBatch{Batch: retainedBuiltinText},
		render.DrawTextBatch{Batch: retainedTTFText},
	}
}

func retainedBatchesManifest() (asset.Manifest, error) {
	tileA, err := encodeGoldenTexture([4]color.RGBA{
		{R: 242, G: 84, B: 105, A: 255},
		{R: 255, G: 213, B: 94, A: 255},
		{R: 105, G: 224, B: 157, A: 255},
		{R: 76, G: 175, B: 255, A: 255},
	})
	if err != nil {
		return asset.Manifest{}, err
	}
	tileB, err := encodeGoldenTexture([4]color.RGBA{
		{R: 116, G: 83, B: 244, A: 255},
		{R: 237, G: 92, B: 190, A: 255},
		{R: 74, G: 215, B: 221, A: 255},
		{R: 245, G: 245, B: 250, A: 255},
	})
	if err != nil {
		return asset.Manifest{}, err
	}
	flipbook, err := encodeRetainedFlipbookTexture()
	if err != nil {
		return asset.Manifest{}, err
	}

	const manifest = `{
		"version": 1,
		"textures": [
			{"name":"tileA","file":"tileA.png"},
			{"name":"tileB","file":"tileB.png"},
			{"name":"flipbook","file":"flipbook.png"}
		],
		"fonts": [
			{"name":"retained-ttf","file":"fonts/Go-Regular.ttf"}
		]
	}`

	return asset.Load(fstest.MapFS{
		"manifest.json":        {Data: []byte(manifest), Mode: 0o644},
		"tileA.png":            {Data: tileA, Mode: 0o644},
		"tileB.png":            {Data: tileB, Mode: 0o644},
		"flipbook.png":         {Data: flipbook, Mode: 0o644},
		"fonts/Go-Regular.ttf": {Data: retainedBatchesTTF, Mode: 0o644},
	})
}

// encodeRetainedFlipbookTexture builds a 32x8 strip of four 8x8 cells with
// distinct flat colors, so the flip-book frame advance in the golden image
// is unambiguous.
func encodeRetainedFlipbookTexture() ([]byte, error) {
	colors := [4]color.RGBA{
		{R: 255, G: 99, B: 99, A: 255},
		{R: 255, G: 214, B: 92, A: 255},
		{R: 120, G: 224, B: 143, A: 255},
		{R: 111, G: 165, B: 255, A: 255},
	}
	img := image.NewRGBA(image.Rect(0, 0, 32, 8))
	for cell, c := range colors {
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				img.SetRGBA(cell*8+x, y, c)
			}
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}
