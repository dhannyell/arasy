package raster_test

import (
	"image/color"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

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
