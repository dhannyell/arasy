package raster_test

import (
	"image/color"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

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
