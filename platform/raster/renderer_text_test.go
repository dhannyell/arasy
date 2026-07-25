package raster_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
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

func TestDrawTextSupportsRotation(t *testing.T) {
	renderer := raster.Renderer{
		Width:  100,
		Height: 100,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		},
		Clear: render.RGBA(0, 0, 0, 1),
	}

	img := renderer.Render([]render.Command{
		render.DrawText{
			Position: math2d.Vec2{X: 50, Y: 50},
			Text:     "Hi",
			Color:    render.RGBA(1, 1, 1, 1),
			Size:     16,
			Baseline: render.BaselineTop,
			Rotation: math.Pi / 2,
		},
	})

	foundRotatedPixel := false
	for y := 51; y < img.Bounds().Max.Y && !foundRotatedPixel; y++ {
		for x := img.Bounds().Min.X; x < 50; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 > 0 || g>>8 > 0 || b>>8 > 0 {
				foundRotatedPixel = true
				break
			}
		}
	}

	if !foundRotatedPixel {
		t.Fatal("rotated DrawText produced no pixels in the expected rotated quadrant")
	}
}
