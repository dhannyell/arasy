package raster_test

import (
	"image/color"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
)

func TestRendererClearsImage(t *testing.T) {
	renderer := newTestRenderer()
	renderer.Clear = render.RGBA(0.1, 0.2, 0.3, 1)

	img := renderer.Render(nil)

	assertPixel(t, img.At(0, 0), color.RGBA{R: 26, G: 51, B: 77, A: 255})
	assertPixel(t, img.At(9, 9), color.RGBA{R: 26, G: 51, B: 77, A: 255})
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
