package raster_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// assertImagesIdentical requires pixel-exact equality: retained batches are
// specified to be pixel-equivalent to their streaming counterparts.
func assertImagesIdentical(t *testing.T, want, got *image.RGBA) {
	t.Helper()
	if want.Bounds() != got.Bounds() {
		t.Fatalf("bounds = %v, want %v", got.Bounds(), want.Bounds())
	}
	for y := want.Bounds().Min.Y; y < want.Bounds().Max.Y; y++ {
		for x := want.Bounds().Min.X; x < want.Bounds().Max.X; x++ {
			if got.RGBAAt(x, y) != want.RGBAAt(x, y) {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got.RGBAAt(x, y), want.RGBAAt(x, y))
			}
		}
	}
}

func worldRect(x, y, w, h float64) math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: x, Y: y}, math2d.Vec2{X: w, Y: h})
}

func TestSpriteBatchStaticMatchesDrawImage(t *testing.T) {
	renderer := imageRenderer(16, 16)
	renderer.Textures = pixel.Registry{nil, makeTex4x4Quadrants()}

	tint := render.RGBA(1, 0, 0, 0.5)
	src := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2})
	direct := renderer.Render([]render.Command{
		render.DrawImage{Texture: 1, Dest: worldRect(2, 2, 6, 6), Src: src},
		render.DrawImage{Texture: 1, Dest: worldRect(8, 4, 4, 4), Rotation: math.Pi / 6, Tint: tint},
	})

	renderer.ReplaceSpriteBatch(1, []render.SpriteInstance{
		{Texture: 1, Dest: worldRect(2, 2, 6, 6), Src: src},
		{Texture: 1, Dest: worldRect(8, 4, 4, 4), Rotation: math.Pi / 6, Tint: tint},
	})
	retained := renderer.Render([]render.Command{
		render.DrawSpriteBatch{Batch: 1},
	})

	assertImagesIdentical(t, direct, retained)
}

func TestProjectedSpriteBatchMatchesDrawImageQuad(t *testing.T) {
	renderer := imageRenderer(32, 32)
	renderer.Textures = pixel.Registry{nil, makeTexWhite()}
	renderer.Camera.View = math2d.Affine{A: 1, C: -1, B: 0.5, D: 0.5}
	instance := render.SpriteInstance{
		Texture:        1,
		Dest:           worldRect(12, 12, 8, 8),
		ProjectCorners: true,
	}

	direct := renderer.Render([]render.Command{
		render.DrawImageQuad{Texture: 1, Corners: renderbatch.SpriteCorners(instance)},
	})
	renderer.ReplaceSpriteBatch(1, []render.SpriteInstance{instance})
	retained := renderer.Render([]render.Command{
		render.DrawSpriteBatch{Batch: 1},
	})

	assertImagesIdentical(t, direct, retained)
}

// TestSpriteBatchPreservesSliceOrder draws two overlapping translucent
// sprites: the batch must blend them in slice order, like two DrawImages.
func TestSpriteBatchPreservesSliceOrder(t *testing.T) {
	renderer := imageRenderer(16, 16)
	renderer.Textures = pixel.Registry{nil, makeTexWhite()}

	redHalf := render.RGBA(1, 0, 0, 0.5)
	blueHalf := render.RGBA(0, 0, 1, 0.5)
	direct := renderer.Render([]render.Command{
		render.DrawImage{Texture: 1, Dest: worldRect(2, 2, 8, 8), Tint: redHalf},
		render.DrawImage{Texture: 1, Dest: worldRect(6, 6, 8, 8), Tint: blueHalf},
	})

	renderer.ReplaceSpriteBatch(1, []render.SpriteInstance{
		{Texture: 1, Dest: worldRect(2, 2, 8, 8), Tint: redHalf},
		{Texture: 1, Dest: worldRect(6, 6, 8, 8), Tint: blueHalf},
	})
	retained := renderer.Render([]render.Command{
		render.DrawSpriteBatch{Batch: 1},
	})

	assertImagesIdentical(t, direct, retained)
}

// TestSpriteBatchPainterBoundary pins the batch command as a painter-order
// boundary relative to other commands: same layer sequence, same pixels.
func TestSpriteBatchPainterBoundary(t *testing.T) {
	renderer := imageRenderer(16, 16)
	renderer.Textures = pixel.Registry{nil, makeTexWhite()}

	whiteHalf := render.RGBA(1, 1, 1, 0.5)
	red := render.RGBA(1, 0, 0, 1)
	blue := render.RGBA(0, 0, 1, 1)
	compose := func(imageCommand func(layer int) render.Command) *image.RGBA {
		return renderer.Render([]render.Command{
			render.DrawRect{BaseCommand: render.BaseCommand{LayerValue: 0}, Rect: worldRect(0, 0, 16, 16), Fill: red},
			imageCommand(1),
			render.DrawRect{BaseCommand: render.BaseCommand{LayerValue: 2}, Rect: worldRect(8, 0, 8, 16), Fill: blue},
		})
	}

	direct := compose(func(layer int) render.Command {
		return render.DrawImage{
			BaseCommand: render.BaseCommand{LayerValue: layer},
			Texture:     1, Dest: worldRect(4, 4, 8, 8), Tint: whiteHalf,
		}
	})

	renderer.ReplaceSpriteBatch(7, []render.SpriteInstance{
		{Texture: 1, Dest: worldRect(4, 4, 8, 8), Tint: whiteHalf},
	})
	retained := compose(func(layer int) render.Command {
		return render.DrawSpriteBatch{BaseCommand: render.BaseCommand{LayerValue: layer}, Batch: 7}
	})

	assertImagesIdentical(t, direct, retained)
}

func TestTextBatchMatchesDrawText(t *testing.T) {
	renderer := imageRenderer(64, 32)

	white := render.RGBA(1, 1, 1, 1)
	green := render.RGBA(0, 1, 0, 1)
	direct := renderer.Render([]render.Command{
		render.DrawText{Position: math2d.Vec2{X: 2, Y: 12}, Text: "HP 42", Color: white, Size: 8},
		render.DrawText{Position: math2d.Vec2{X: 32, Y: 24}, Text: "OK", Color: green, Size: 6, Align: render.AlignCenter, Rotation: math.Pi / 8},
	})

	renderer.ReplaceTextBatch(9, []render.TextLabel{
		{Position: math2d.Vec2{X: 2, Y: 12}, Text: "HP 42", Color: white, Size: 8},
		{Position: math2d.Vec2{X: 32, Y: 24}, Text: "OK", Color: green, Size: 6, Align: render.AlignCenter, Rotation: math.Pi / 8},
	})
	retained := renderer.Render([]render.Command{
		render.DrawTextBatch{Batch: 9},
	})

	assertImagesIdentical(t, direct, retained)
}

// TestSpriteBatchOffscreenDrawsNothing: a batch fully outside the camera
// view is culled before the instance loop and leaves no pixels behind.
func TestSpriteBatchOffscreenDrawsNothing(t *testing.T) {
	renderer := imageRenderer(16, 16)
	renderer.Textures = pixel.Registry{nil, makeTexWhite()}

	base := renderer.Render(nil)

	renderer.ReplaceSpriteBatch(1, []render.SpriteInstance{
		{Texture: 1, Dest: worldRect(1000, 1000, 8, 8)},
		{Texture: 1, Dest: worldRect(-100, 0, 8, 8)},
	})
	got := renderer.Render([]render.Command{
		render.DrawSpriteBatch{Batch: 1},
	})

	assertImagesIdentical(t, base, got)
}

// TestSpriteBatchClipCull: a batch whose bounds intersect the view but not
// the command clip draws nothing.
func TestSpriteBatchClipCull(t *testing.T) {
	renderer := imageRenderer(16, 16)
	renderer.Textures = pixel.Registry{nil, makeTexWhite()}

	base := renderer.Render(nil)

	renderer.ReplaceSpriteBatch(1, []render.SpriteInstance{
		{Texture: 1, Dest: worldRect(8, 8, 8, 8)},
	})
	got := renderer.Render([]render.Command{
		render.DrawSpriteBatch{
			BaseCommand: render.BaseCommand{Clip: worldRect(0, 0, 4, 4)},
			Batch:       1,
		},
	})

	assertImagesIdentical(t, base, got)
}

// TestSpriteBatchClipMatchesDrawImage: a visible batch under a command clip
// is pixel-equivalent to a clipped DrawImage.
func TestSpriteBatchClipMatchesDrawImage(t *testing.T) {
	renderer := imageRenderer(16, 16)
	renderer.Textures = pixel.Registry{nil, makeTexWhite()}

	clip := worldRect(2, 2, 6, 6)
	direct := renderer.Render([]render.Command{
		render.DrawImage{
			BaseCommand: render.BaseCommand{Clip: clip},
			Texture:     1, Dest: worldRect(0, 0, 8, 8),
		},
	})

	renderer.ReplaceSpriteBatch(1, []render.SpriteInstance{
		{Texture: 1, Dest: worldRect(0, 0, 8, 8)},
	})
	retained := renderer.Render([]render.Command{
		render.DrawSpriteBatch{BaseCommand: render.BaseCommand{Clip: clip}, Batch: 1},
	})

	assertImagesIdentical(t, direct, retained)
}

// TestSpriteBatchAnimationMatchesHelpers: at a fixed time, the flip-book
// frame and bounce offset drawn by the batch equal a streaming DrawImage
// using the canonical helper outputs.
func TestSpriteBatchAnimationMatchesHelpers(t *testing.T) {
	renderer := imageRenderer(16, 16)
	renderer.Textures = pixel.Registry{nil, makeTex4x4Quadrants()}

	inst := render.SpriteInstance{
		Texture:   1,
		Dest:      worldRect(4, 6, 4, 4),
		Src:       math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2}),
		Animation: render.SpriteAnimation{Frames: 4, Rate: 1, Phase: 0.25, Bounce: 2},
	}
	anim := render.SpriteBatchAnimation{Time: 1, BounceHz: 1, Intensity: 1}

	wantSrc := renderbatch.AnimatedSrc(inst, anim.Time, 4, 4)
	if wantSrc == inst.Src {
		t.Fatal("test premise: flip-book must advance past the base cell at this time")
	}
	wantOffset := renderbatch.BounceOffsetY(inst, anim)
	if wantOffset == 0 {
		t.Fatal("test premise: bounce must be active at this time")
	}

	dest := inst.Dest
	dest.Min.Y += wantOffset
	dest.Max.Y += wantOffset
	direct := renderer.Render([]render.Command{
		render.DrawImage{Texture: 1, Dest: dest, Src: wantSrc},
	})

	renderer.ReplaceSpriteBatch(1, []render.SpriteInstance{inst})
	retained := renderer.Render([]render.Command{
		render.DrawSpriteBatch{Batch: 1, Animation: anim},
	})

	assertImagesIdentical(t, direct, retained)
}

// TestSpriteBatchMissingTexturePlaceholder: an instance with an unknown
// texture draws the same magenta placeholder as a streaming DrawImage.
func TestSpriteBatchMissingTexturePlaceholder(t *testing.T) {
	renderer := imageRenderer(16, 16)
	renderer.Textures = pixel.Registry{nil} // no textures loaded

	direct := renderer.Render([]render.Command{
		render.DrawImage{Texture: 1, Dest: worldRect(4, 4, 8, 8)},
	})

	renderer.ReplaceSpriteBatch(1, []render.SpriteInstance{
		{Texture: 1, Dest: worldRect(4, 4, 8, 8)},
	})
	retained := renderer.Render([]render.Command{
		render.DrawSpriteBatch{Batch: 1},
	})

	assertImagesIdentical(t, direct, retained)
	if got := retained.RGBAAt(8, 8); got != (color.RGBA{R: 255, G: 0, B: 255, A: 255}) {
		t.Fatalf("placeholder pixel = %v, want magenta", got)
	}
}

// TestSpriteBatchEmptyReplaceDrawsNothing: an empty replace keeps an empty
// batch — drawing it is a silent no-op (no missing-batch warning).
func TestSpriteBatchEmptyReplaceDrawsNothing(t *testing.T) {
	renderer := imageRenderer(16, 16)
	renderer.Textures = pixel.Registry{nil, makeTexWhite()}

	base := renderer.Render(nil)

	renderer.ReplaceSpriteBatch(1, nil)
	got := renderer.Render([]render.Command{
		render.DrawSpriteBatch{Batch: 1},
	})

	assertImagesIdentical(t, base, got)
}
