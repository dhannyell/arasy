package raster_test

import (
	"image/color"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/raster"
	"github.com/dhannyell/frameforge/render"
)

func TestDrawTextRendersPixels(t *testing.T) {
	renderer := raster.Renderer{
		Width:  100,
		Height: 50,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 50}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 50}),
		},
		Clear: render.RGBA(0, 0, 0, 1),
	}

	img := renderer.Render([]render.Command{
		render.DrawText{
			Position: math2d.Vec2{X: 10, Y: 10},
			Text:     "Hi",
			Color:    render.RGBA(1, 1, 1, 1),
			Size:     16,
			Baseline: render.BaselineTop,
		},
	})

	foundNonBackground := false
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y && !foundNonBackground; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 > 0 || g>>8 > 0 || b>>8 > 0 {
				foundNonBackground = true
				break
			}
		}
	}

	if !foundNonBackground {
		t.Fatal("DrawText produced no visible pixels — text is being silently dropped")
	}
}

func TestDrawTextEmptyStringDrawsNothing(t *testing.T) {
	renderer := newTestRenderer()

	img := renderer.Render([]render.Command{
		render.DrawText{
			Position: math2d.Vec2{X: 5, Y: 5},
			Text:     "",
			Color:    render.RGBA(1, 1, 1, 1),
			Size:     4,
		},
	})

	assertPixel(t, img.At(5, 5), color.RGBA{R: 0, G: 0, B: 0, A: 255})
}
