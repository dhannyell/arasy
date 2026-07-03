package raster_test

import (
	"image/color"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

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
			Fill: render.RGBA(1, 0, 0, 1),
			BaseCommand: render.BaseCommand{
				LayerValue: 1,
			},
		},
		render.DrawPoint{
			Center: math2d.Vec2{X: 5, Y: 5},
			Radius: 1,
			Color:  render.RGBA(0, 0, 1, 1),
			BaseCommand: render.BaseCommand{
				LayerValue: 2,
			},
		},
	})

	assertPixel(t, img.At(5, 5), color.RGBA{R: 0, G: 0, B: 255, A: 255})
}

func TestRendererPreservesOrderWithinSameLayer(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 2, Y: 2},
				math2d.Vec2{X: 5, Y: 5},
			),
			Fill: render.RGBA(1, 0, 0, 1),
			BaseCommand: render.BaseCommand{
				LayerValue: 1,
			},
		},
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 3, Y: 3},
				math2d.Vec2{X: 5, Y: 5},
			),
			Fill: render.RGBA(0, 0, 1, 1),
			BaseCommand: render.BaseCommand{
				LayerValue: 1,
			},
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
			Fill: render.RGBA(1, 0, 0, 1),
			BaseCommand: render.BaseCommand{
				LayerValue: 10,
			},
		},
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 3, Y: 3},
				math2d.Vec2{X: 5, Y: 5},
			),
			Fill: render.RGBA(0, 0, 1, 1),
			BaseCommand: render.BaseCommand{
				LayerValue: 1,
			},
		},
	})

	assertPixel(t, img.At(3, 3), color.RGBA{R: 255, G: 0, B: 0, A: 255})
}
