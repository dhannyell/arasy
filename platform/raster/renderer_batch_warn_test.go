package raster

import (
	"bytes"
	"image"
	"image/color"
	"log/slog"
	"math"
	"strings"
	"testing"

	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// batchWarnRenderer returns a 16x16 renderer whose warnings are captured.
func batchWarnRenderer(log *slog.Logger) *Renderer {
	return &Renderer{
		Width:  16,
		Height: 16,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 16, Y: 16}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 16, Y: 16}),
		},
		Clear:    render.RGBA(0, 0, 0, 1),
		Textures: pixel.Registry{nil, nil},
		log:      log,
	}
}

func whiteTexture(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	return img
}

func TestSpriteBatchMissingWarnsOnce(t *testing.T) {
	var buf bytes.Buffer
	r := batchWarnRenderer(slog.New(slog.NewTextHandler(&buf, nil)))

	base := r.Render(nil)
	commands := []render.Command{render.DrawSpriteBatch{Batch: 42}}
	got := r.Render(commands)
	r.Render(commands) // second draw must not warn again

	if count := strings.Count(buf.String(), "level=WARN"); count != 1 {
		t.Fatalf("warnings = %d, want 1; log:\n%s", count, buf.String())
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if got.RGBAAt(x, y) != base.RGBAAt(x, y) {
				t.Fatalf("missing batch changed pixel (%d,%d)", x, y)
			}
		}
	}
}

func TestTextBatchMissingWarnsOnce(t *testing.T) {
	var buf bytes.Buffer
	r := batchWarnRenderer(slog.New(slog.NewTextHandler(&buf, nil)))

	commands := []render.Command{render.DrawTextBatch{Batch: 42}}
	r.Render(commands)
	r.Render(commands)

	if count := strings.Count(buf.String(), "level=WARN"); count != 1 {
		t.Fatalf("warnings = %d, want 1; log:\n%s", count, buf.String())
	}
}

// TestSpriteBatchDeleteRemovesContent: after a delete, the command falls
// back to the missing-batch path (warn once, draw nothing).
func TestSpriteBatchDeleteRemovesContent(t *testing.T) {
	var buf bytes.Buffer
	r := batchWarnRenderer(slog.New(slog.NewTextHandler(&buf, nil)))

	r.ReplaceSpriteBatch(1, []render.SpriteInstance{
		{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 2, Y: 2}, math2d.Vec2{X: 8, Y: 8})},
	})
	r.DeleteSpriteBatch(1)

	base := r.Render(nil)
	got := r.Render([]render.Command{render.DrawSpriteBatch{Batch: 1}})

	if count := strings.Count(buf.String(), "level=WARN"); count != 1 {
		t.Fatalf("warnings = %d, want 1; log:\n%s", count, buf.String())
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if got.RGBAAt(x, y) != base.RGBAAt(x, y) {
				t.Fatalf("deleted batch changed pixel (%d,%d)", x, y)
			}
		}
	}
}

// TestSpriteBatchNonFiniteSkipped: a non-finite instance is dropped with a
// single warning; finite siblings still draw.
func TestSpriteBatchNonFiniteSkipped(t *testing.T) {
	var buf bytes.Buffer
	r := batchWarnRenderer(slog.New(slog.NewTextHandler(&buf, nil)))
	r.Textures = pixel.Registry{nil, whiteTexture(2)}

	r.ReplaceSpriteBatch(1, []render.SpriteInstance{
		{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: math.NaN(), Y: 2}, math2d.Vec2{X: 4, Y: 4})},
		{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 8, Y: 8}, math2d.Vec2{X: 4, Y: 4})},
	})

	got := r.Render([]render.Command{render.DrawSpriteBatch{Batch: 1}})
	r.Render([]render.Command{render.DrawSpriteBatch{Batch: 1}})

	if count := strings.Count(buf.String(), "level=WARN"); count != 1 {
		t.Fatalf("warnings = %d, want 1; log:\n%s", count, buf.String())
	}
	if got.RGBAAt(10, 10) != (color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatal("finite sibling instance was not drawn")
	}
	if got.RGBAAt(2, 2) != (color.RGBA{A: 255}) {
		t.Fatal("non-finite instance drew pixels")
	}
}
