package raster_test

import (
	"image/color"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestRendererDrawsRect(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 2, Y: 3},
				math2d.Vec2{X: 4, Y: 2},
			),
			Fill: render.RGBA(1, 0, 0, 1),
		},
	})

	assertPixel(t, img.At(1, 3), color.RGBA{R: 0, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(2, 3), color.RGBA{R: 255, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(5, 4), color.RGBA{R: 255, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(6, 4), color.RGBA{R: 0, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(2, 5), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererClipsRectToImageBounds(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: -2, Y: -2},
				math2d.Vec2{X: 5, Y: 5},
			),
			Fill: render.RGBA(0, 1, 0, 1),
		},
	})

	assertPixel(t, img.At(0, 0), color.RGBA{R: 0, G: 255, B: 0, A: 255})
	assertPixel(t, img.At(2, 2), color.RGBA{R: 0, G: 255, B: 0, A: 255})
	assertPixel(t, img.At(3, 3), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererDrawsRectWithStroke(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 2, Y: 2},
				math2d.Vec2{X: 5, Y: 5},
			),
			Stroke:      render.RGBA(1, 1, 1, 1),
			StrokeWidth: 1,
		},
	})

	assertPixel(t, img.At(2, 2), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(6, 2), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(2, 6), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(6, 6), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(4, 4), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestDrawRectAlphaBlends(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
			Fill: render.RGBA(1, 1, 1, 1),
			BaseCommand: render.BaseCommand{
				LayerValue: 1,
			},
		},
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
			Fill: render.RGBA(0, 0, 0, 0.5),
			BaseCommand: render.BaseCommand{
				LayerValue: 2,
			},
		},
	})

	// 127 not 128: alpha 0.5 quantizes to byte 128/255 ≈ 0.502, so the
	// blended channel rounds down by one from the mathematically exact half.
	assertPixel(t, img.At(5, 5), color.RGBA{R: 127, G: 127, B: 127, A: 255})
}

// TestStrokeRectTranslucentCornersBlendOnce guards against the four stroke
// bands overlapping: fills composite with "over", so a corner covered by two
// bands (top ∩ left in the naive full-height layout) would blend the
// translucent stroke twice and come out darker than the edges.
func TestStrokeRectTranslucentCornersBlendOnce(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
			Fill: render.RGBA(1, 1, 1, 1),
			BaseCommand: render.BaseCommand{
				LayerValue: 1,
			},
		},
		render.DrawRect{
			Rect:        math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 8, Y: 8}),
			Stroke:      render.RGBA(0, 0, 0, 0.5),
			StrokeWidth: 2,
			BaseCommand: render.BaseCommand{
				LayerValue: 2,
			},
		},
	})

	// Same quantization as TestDrawRectAlphaBlends: 50% black over white → 127.
	want := color.RGBA{R: 127, G: 127, B: 127, A: 255}
	assertPixel(t, img.At(5, 1), want) // top edge: single band
	assertPixel(t, img.At(1, 5), want) // left edge: single band
	assertPixel(t, img.At(1, 1), want) // corner: was 63 when two bands overlapped
	assertPixel(t, img.At(8, 8), want) // opposite corner
}
