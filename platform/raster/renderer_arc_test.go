package raster_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

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
			BaseCommand: render.BaseCommand{
				LayerValue: 1,
			},
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: 5, Y: 5},
			Radius:     3,
			StartAngle: 0,
			EndAngle:   math.Pi / 2,
			Width:      1,
			Color:      render.RGBA(0, 0, 1, 1),
			BaseCommand: render.BaseCommand{
				LayerValue: 2,
			},
		},
	})

	assertPixel(t, img.At(7, 5), color.RGBA{R: 0, G: 0, B: 255, A: 255})
}
