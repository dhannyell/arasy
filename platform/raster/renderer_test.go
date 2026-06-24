package raster_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/raster"
	"github.com/dhannyell/frameforge/render"
)

func TestRendererClearsImage(t *testing.T) {
	renderer := newTestRenderer()
	renderer.Clear = render.RGBA(0.1, 0.2, 0.3, 1)

	img := renderer.Render(nil)

	assertPixel(t, img.At(0, 0), color.RGBA{R: 26, G: 51, B: 77, A: 255})
	assertPixel(t, img.At(9, 9), color.RGBA{R: 26, G: 51, B: 77, A: 255})
}

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

func TestRendererAppliesCameraScale(t *testing.T) {
	renderer := raster.Renderer{
		Width:  20,
		Height: 20,
		Camera: render.Camera{
			World: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 0},
				math2d.Vec2{X: 10, Y: 10},
			),
			Viewport: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 0},
				math2d.Vec2{X: 20, Y: 20},
			),
		},
		Clear: render.RGBA(0, 0, 0, 1),
	}

	img := renderer.Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 1, Y: 1},
				math2d.Vec2{X: 2, Y: 2},
			),
			Fill: render.RGBA(0, 0, 1, 1),
		},
	})

	assertPixel(t, img.At(1, 2), color.RGBA{R: 0, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(2, 2), color.RGBA{R: 0, G: 0, B: 255, A: 255})
	assertPixel(t, img.At(5, 5), color.RGBA{R: 0, G: 0, B: 255, A: 255})
	assertPixel(t, img.At(6, 6), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererDrawsCircle(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawCircle{
			Center: math2d.Vec2{X: 5, Y: 5},
			Radius: 2,
			Fill:   render.RGBA(1, 1, 0, 1),
		},
	})

	assertPixel(t, img.At(5, 5), color.RGBA{R: 255, G: 255, B: 0, A: 255})
	assertPixel(t, img.At(4, 4), color.RGBA{R: 255, G: 255, B: 0, A: 255})
	assertPixel(t, img.At(2, 2), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererClipsCircleToImageBounds(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawCircle{
			Center: math2d.Vec2{X: 0, Y: 0},
			Radius: 3,
			Fill:   render.RGBA(1, 0, 1, 1),
		},
	})

	assertPixel(t, img.At(0, 0), color.RGBA{R: 255, G: 0, B: 255, A: 255})
	assertPixel(t, img.At(2, 0), color.RGBA{R: 255, G: 0, B: 255, A: 255})
	assertPixel(t, img.At(3, 3), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererDrawsPoint(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawPoint{
			Center: math2d.Vec2{X: 5, Y: 5},
			Radius: 1,
			Color:  render.RGBA(1, 1, 1, 1),
		},
	})

	assertPixel(t, img.At(5, 5), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(4, 4), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(3, 3), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererDrawsPointLayerOnTop(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 4, Y: 4},
				math2d.Vec2{X: 3, Y: 3},
			),
			Fill:  render.RGBA(1, 0, 0, 1),
			Layer: 1,
		},
		render.DrawPoint{
			Center: math2d.Vec2{X: 5, Y: 5},
			Radius: 1,
			Color:  render.RGBA(0, 0, 1, 1),
			Layer:  2,
		},
	})

	assertPixel(t, img.At(5, 5), color.RGBA{R: 0, G: 0, B: 255, A: 255})
}

func TestRendererDrawsHorizontalLine(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawLine{
			From:  math2d.Vec2{X: 2, Y: 5},
			To:    math2d.Vec2{X: 7, Y: 5},
			Width: 1,
			Color: render.RGBA(0, 1, 1, 1),
		},
	})

	assertPixel(t, img.At(2, 5), color.RGBA{R: 0, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(5, 5), color.RGBA{R: 0, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(6, 5), color.RGBA{R: 0, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(7, 5), color.RGBA{R: 0, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(5, 3), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererDrawsVerticalLine(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawLine{
			From:  math2d.Vec2{X: 5, Y: 2},
			To:    math2d.Vec2{X: 5, Y: 7},
			Width: 1,
			Color: render.RGBA(1, 0.5, 0, 1),
		},
	})

	assertPixel(t, img.At(5, 2), color.RGBA{R: 255, G: 128, B: 0, A: 255})
	assertPixel(t, img.At(5, 5), color.RGBA{R: 255, G: 128, B: 0, A: 255})
	assertPixel(t, img.At(5, 6), color.RGBA{R: 255, G: 128, B: 0, A: 255})
	assertPixel(t, img.At(5, 7), color.RGBA{R: 0, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(3, 5), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererClipsLineToImageBounds(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawLine{
			From:  math2d.Vec2{X: -5, Y: 5},
			To:    math2d.Vec2{X: 5, Y: 5},
			Width: 1,
			Color: render.RGBA(1, 1, 1, 1),
		},
	})

	assertPixel(t, img.At(0, 5), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(4, 5), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(5, 5), color.RGBA{R: 0, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(6, 5), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func newTestRenderer() raster.Renderer {
	return raster.Renderer{
		Width:  10,
		Height: 10,
		Camera: render.Camera{
			World: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 0},
				math2d.Vec2{X: 10, Y: 10},
			),
			Viewport: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 0},
				math2d.Vec2{X: 10, Y: 10},
			),
		},
		Clear: render.RGBA(0, 0, 0, 1),
	}
}

func TestRendererPreservesOrderWithinSameLayer(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 2, Y: 2},
				math2d.Vec2{X: 5, Y: 5},
			),
			Fill:  render.RGBA(1, 0, 0, 1),
			Layer: 1,
		},
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 3, Y: 3},
				math2d.Vec2{X: 5, Y: 5},
			),
			Fill:  render.RGBA(0, 0, 1, 1),
			Layer: 1,
		},
	})

	assertPixel(t, img.At(3, 3), color.RGBA{R: 0, G: 0, B: 255, A: 255})
}

func TestRendererDrawsHigherLayerOnTop(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 2, Y: 2},
				math2d.Vec2{X: 5, Y: 5},
			),
			Fill:  render.RGBA(1, 0, 0, 1),
			Layer: 10,
		},
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 3, Y: 3},
				math2d.Vec2{X: 5, Y: 5},
			),
			Fill:  render.RGBA(0, 0, 1, 1),
			Layer: 1,
		},
	})

	assertPixel(t, img.At(3, 3), color.RGBA{R: 255, G: 0, B: 0, A: 255})
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

func TestRendererDrawsCircleWithStroke(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawCircle{
			Center:      math2d.Vec2{X: 5, Y: 5},
			Radius:      3,
			Fill:        render.RGBA(1, 0, 1, 1),
			Stroke:      render.RGBA(0, 0, 1, 1),
			StrokeWidth: 1,
		},
	})

	assertPixel(t, img.At(5, 5), color.RGBA{R: 255, G: 0, B: 255, A: 255})
	assertPixel(t, img.At(7, 5), color.RGBA{R: 0, G: 0, B: 255, A: 255})
	assertPixel(t, img.At(8, 5), color.RGBA{R: 0, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(9, 5), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererDrawsArc(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawArc{
			Center:     math2d.Vec2{X: 5, Y: 5},
			Radius:     3,
			StartAngle: 0,
			EndAngle:   1.5707963267948966,
			Width:      1,
			Color:      render.RGBA(1, 1, 1, 1),
		},
	})

	assertPixel(t, img.At(7, 5), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(5, 7), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(3, 5), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererDrawsArcAcrossZeroAngle(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawArc{
			Center:     math2d.Vec2{X: 5, Y: 5},
			Radius:     3,
			StartAngle: 3 * math.Pi / 2,
			EndAngle:   0,
			Width:      1,
			Color:      render.RGBA(1, 0, 0, 1),
		},
	})

	assertPixel(t, img.At(7, 4), color.RGBA{R: 255, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(5, 2), color.RGBA{R: 255, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(5, 7), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererDoesNotDrawArcWithInvalidSize(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawArc{
			Center:     math2d.Vec2{X: 5, Y: 5},
			Radius:     0,
			StartAngle: 0,
			EndAngle:   math.Pi / 2,
			Width:      1,
			Color:      render.RGBA(1, 1, 1, 1),
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: 5, Y: 5},
			Radius:     3,
			StartAngle: 0,
			EndAngle:   math.Pi / 2,
			Width:      0,
			Color:      render.RGBA(1, 1, 1, 1),
		},
	})

	assertPixel(t, img.At(7, 5), color.RGBA{R: 0, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(5, 7), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}

func TestRendererDrawsArcLayerOnTop(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawArc{
			Center:     math2d.Vec2{X: 5, Y: 5},
			Radius:     3,
			StartAngle: 0,
			EndAngle:   math.Pi / 2,
			Width:      1,
			Color:      render.RGBA(1, 0, 0, 1),
			Layer:      1,
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: 5, Y: 5},
			Radius:     3,
			StartAngle: 0,
			EndAngle:   math.Pi / 2,
			Width:      1,
			Color:      render.RGBA(0, 0, 1, 1),
			Layer:      2,
		},
	})

	assertPixel(t, img.At(7, 5), color.RGBA{R: 0, G: 0, B: 255, A: 255})
}

func assertPixel(t *testing.T, got color.Color, want color.RGBA) {
	t.Helper()

	r, g, b, a := got.RGBA()
	gotRGBA := color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}

	if gotRGBA != want {
		t.Fatalf("got %v, want %v", gotRGBA, want)
	}
}
