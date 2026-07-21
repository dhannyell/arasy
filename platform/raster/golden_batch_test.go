package raster

import (
	"image"
	"image/color"
	"testing"

	"github.com/dhannyell/frameforge/asset/pixel"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// goldenFlipTexture builds an 8x2 flip-book sheet: four 2x2 cells, one
// distinct opaque color each (red, green, blue, yellow), row-major.
func goldenFlipTexture() *image.RGBA {
	colors := [4]color.RGBA{
		{R: 230, G: 40, B: 40, A: 255},
		{R: 40, G: 200, B: 60, A: 255},
		{R: 50, G: 90, B: 230, A: 255},
		{R: 240, G: 220, B: 40, A: 255},
	}
	img := image.NewRGBA(image.Rect(0, 0, 8, 2))
	for cell := 0; cell < 4; cell++ {
		for y := 0; y < 2; y++ {
			for x := 0; x < 2; x++ {
				img.SetRGBA(cell*2+x, y, colors[cell])
			}
		}
	}
	return img
}

// TestGoldenRetainedBatches covers the retained-batch raster semantics in
// one image: static sprites interleaving two textures, tint/rotation, a
// clipped batch, a flip-book row and a bounce/pulse row at a fixed time, an
// offscreen batch (no pixels), and retained text — interleaved in painter
// order with streaming commands.
func TestGoldenRetainedBatches(t *testing.T) {
	r := goldenRenderer()
	r.Textures = pixel.Registry{nil, goldenImageTexture(), goldenFlipTexture()}

	// Batch 1: static sprites, two textures, one rotated, one tinted.
	r.ReplaceSpriteBatch(1, []render.SpriteInstance{
		{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 4, Y: 4}, math2d.Vec2{X: 12, Y: 12})},
		{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 18, Y: 6}, math2d.Vec2{X: 8, Y: 8})},
		{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 28, Y: 4}, math2d.Vec2{X: 10, Y: 10}), Rotation: 0.5},
		{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 40, Y: 6}, math2d.Vec2{X: 8, Y: 8}), Tint: render.RGBA(0.3, 0.8, 1, 0.7)},
	})

	// Batch 2: flip-book row. At Time 0.25 with Rate 1 the phases 0, .25,
	// .5, .75 select frames 0-3 (red, green, blue, yellow).
	r.ReplaceSpriteBatch(2, []render.SpriteInstance{
		{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 4, Y: 24}, math2d.Vec2{X: 8, Y: 8}), Src: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}), Animation: render.SpriteAnimation{Frames: 4, Rate: 1}},
		{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 14, Y: 24}, math2d.Vec2{X: 8, Y: 8}), Src: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}), Animation: render.SpriteAnimation{Frames: 4, Rate: 1, Phase: 0.25}},
		{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 24, Y: 24}, math2d.Vec2{X: 8, Y: 8}), Src: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}), Animation: render.SpriteAnimation{Frames: 4, Rate: 1, Phase: 0.5}},
		{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 34, Y: 24}, math2d.Vec2{X: 8, Y: 8}), Src: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}), Animation: render.SpriteAnimation{Frames: 4, Rate: 1, Phase: 0.75}},
	})

	// Batch 3: bounce row with one pulse. At Time 0.25 and BounceHz 1 the
	// sine peaks (s=1); the pulse centered on the middle sprite doubles its
	// lift relative to the others.
	r.ReplaceSpriteBatch(3, []render.SpriteInstance{
		{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 4, Y: 44}, math2d.Vec2{X: 6, Y: 6}), Src: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}), Animation: render.SpriteAnimation{Bounce: 2}},
		{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 14, Y: 44}, math2d.Vec2{X: 6, Y: 6}), Src: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}), Animation: render.SpriteAnimation{Bounce: 2}},
		{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 24, Y: 44}, math2d.Vec2{X: 6, Y: 6}), Src: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}), Animation: render.SpriteAnimation{Bounce: 2}},
	})

	// Batch 4: clipped sprites (only the strip inside the clip draws).
	r.ReplaceSpriteBatch(4, []render.SpriteInstance{
		{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 54, Y: 20}, math2d.Vec2{X: 16, Y: 16})},
		{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 66, Y: 28}, math2d.Vec2{X: 10, Y: 10})},
	})

	// Batch 5: fully offscreen — must leave no pixels.
	r.ReplaceSpriteBatch(5, []render.SpriteInstance{
		{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 500, Y: 500}, math2d.Vec2{X: 20, Y: 20})},
	})

	// Batch 6: retained text, built-in font, one rotated label.
	r.ReplaceTextBatch(6, []render.TextLabel{
		{Position: math2d.Vec2{X: 4, Y: 66}, Text: "RETAINED", Color: render.RGBA(1, 1, 1, 1), Size: 5},
		{Position: math2d.Vec2{X: 60, Y: 60}, Text: "BATCH", Color: render.RGBA(1, 0.85, 0.2, 1), Size: 4, Rotation: -0.35},
	})

	img := r.Render([]render.Command{
		render.DrawRect{Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}), Fill: render.RGBA(0.09, 0.1, 0.12, 1)},
		render.DrawSpriteBatch{Batch: 1},
		render.DrawSpriteBatch{Batch: 2, Animation: render.SpriteBatchAnimation{Time: 0.25}},
		render.DrawSpriteBatch{Batch: 3, Animation: render.SpriteBatchAnimation{
			Time: 0.25, BounceHz: 1, Intensity: 1,
			Pulse: render.SpritePulse{
				Center: math2d.Vec2{X: 17, Y: 47}, Radius: 0, Width: 4, Boost: 1,
			},
		}},
		render.DrawSpriteBatch{BaseCommand: render.BaseCommand{Clip: math2d.NewRect(math2d.Vec2{X: 58, Y: 24}, math2d.Vec2{X: 14, Y: 10})}, Batch: 4},
		render.DrawSpriteBatch{Batch: 5},
		// Streaming interop: same painter position rules as any command.
		render.DrawImage{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 84, Y: 6}, math2d.Vec2{X: 12, Y: 12})},
		render.DrawTextBatch{Batch: 6},
	})

	assertGolden(t, "retained-batches", img)
}
