package raster_test

import (
	"image/color"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
)

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
