package raster_test

import (
	"image/color"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestRendererClipsPolygonToCommandClip(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawPolygon{
			BaseCommand: render.BaseCommand{
				Clip: math2d.NewRect(
					math2d.Vec2{X: 3, Y: 3},
					math2d.Vec2{X: 3, Y: 3},
				),
			},
			Points: []math2d.Vec2{
				{X: 1, Y: 1},
				{X: 8, Y: 1},
				{X: 8, Y: 8},
				{X: 1, Y: 8},
			},
			Fill: render.RGBA(1, 1, 1, 1),
		},
	})

	assertPixel(t, img.At(2, 2), color.RGBA{R: 0, G: 0, B: 0, A: 255})
	assertPixel(t, img.At(3, 3), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(5, 5), color.RGBA{R: 255, G: 255, B: 255, A: 255})
	assertPixel(t, img.At(6, 6), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}
