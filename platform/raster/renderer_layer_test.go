package raster_test

import (
	"image"
	"image/color"
	"math"
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
)

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

func TestRasterParallaxUsesTheEffectiveCameraPerLayer(t *testing.T) {
	base := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{X: -50, Y: -50}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Parallax: []render.LayerParallax{
			{FromLayer: 0, ToLayer: 0, Factor: math2d.Vec2{X: 0.5, Y: 0.5}},
			{FromLayer: 1, ToLayer: 1, Factor: math2d.Vec2{X: 1, Y: 1}},
		},
	}
	backgroundPoint := math2d.Vec2{X: -15, Y: -10}
	foregroundPoint := math2d.Vec2{X: 15, Y: 10}
	commands := []render.Command{
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: 0},
			Rect:        centeredRect(backgroundPoint, 4),
			Fill:        render.RGBA(1, 0, 0, 1),
		},
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: 1},
			Rect:        centeredRect(foregroundPoint, 4),
			Fill:        render.RGBA(0, 0, 1, 1),
		},
	}

	for _, delta := range []math2d.Vec2{{}, {X: 10, Y: -6}, {X: -20, Y: 12}} {
		camera := base.Pan(delta)
		renderer := raster.Renderer{
			Width: 100, Height: 100, Camera: camera, Clear: render.RGBA(0, 0, 0, 1),
		}
		image := renderer.Render(commands)

		assertWorldPixel(t, image, camera.ForLayer(0), backgroundPoint, color.RGBA{R: 255, A: 255})
		assertWorldPixel(t, image, camera.ForLayer(1), foregroundPoint, color.RGBA{B: 255, A: 255})
		if !reflect.DeepEqual(renderer.Camera, camera) {
			t.Fatalf("Render changed the configured camera: got %#v, want %#v", renderer.Camera, camera)
		}
	}
}

func centeredRect(center math2d.Vec2, size float64) math2d.Rect {
	half := size / 2
	return math2d.NewRect(
		math2d.Vec2{X: center.X - half, Y: center.Y - half},
		math2d.Vec2{X: size, Y: size},
	)
}

func assertWorldPixel(t *testing.T, image *image.RGBA, camera render.Camera, point math2d.Vec2, want color.RGBA) {
	t.Helper()
	screen := camera.WorldToScreen(point)
	x := int(math.Floor(screen.X))
	y := int(math.Floor(screen.Y))
	if got := image.RGBAAt(x, y); got != want {
		t.Fatalf("world point %v at pixel (%d,%d) = %v, want %v", point, x, y, got, want)
	}
}
