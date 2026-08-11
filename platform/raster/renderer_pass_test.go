package raster_test

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
)

func passCamera(world, viewport math2d.Rect) render.Camera {
	return render.Camera{World: world, Viewport: viewport}
}

func passRect(rect math2d.Rect, fill render.Color) render.Command {
	return render.DrawRect{Rect: rect, Fill: fill}
}

func TestRendererPassesUseIndependentCamerasAndViewportClips(t *testing.T) {
	world := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10})
	renderer := raster.Renderer{
		Width:  20,
		Height: 10,
		Camera: passCamera(world, math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 20, Y: 10})),
		Clear:  render.RGBA(0, 0, 0, 1),
	}
	viewportLeft := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10})
	viewportRight := math2d.NewRect(math2d.Vec2{X: 10}, math2d.Vec2{X: 10, Y: 10})
	base := render.Pass{Camera: renderer.Camera}
	passes := []render.Pass{
		{
			Camera:   passCamera(world, viewportLeft),
			Commands: []render.Command{passRect(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 20, Y: 10}), render.RGBA(1, 0, 0, 1))},
		},
		{
			Camera:   passCamera(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 5, Y: 10}), viewportRight),
			Commands: []render.Command{passRect(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 5, Y: 10}), render.RGBA(0, 1, 0, 1))},
		},
	}

	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	renderer.RenderPassesInto(img, passes, base)

	if got := img.RGBAAt(9, 5); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("left viewport pixel = %v, want red", got)
	}
	if got := img.RGBAAt(10, 5); got != (color.RGBA{G: 255, A: 255}) {
		t.Fatalf("right viewport pixel = %v, want green from the second camera", got)
	}
	if got := img.RGBAAt(19, 5); got != (color.RGBA{G: 255, A: 255}) {
		t.Fatalf("right viewport edge pixel = %v, want green", got)
	}

	base.Commands = []render.Command{passRect(math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 1, Y: 1}), render.RGBA(0, 0, 1, 1))}
	renderer.RenderPassesInto(img, passes, base)
	if got := img.RGBAAt(3, 1); got != (color.RGBA{B: 255, A: 255}) {
		t.Fatalf("base pass did not draw last: %v", got)
	}
}

func TestRendererEmptyPassesMatchSingleStream(t *testing.T) {
	renderer := raster.Renderer{
		Width:  8,
		Height: 8,
		Camera: passCamera(
			math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 8, Y: 8}),
			math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 8, Y: 8}),
		),
		Clear: render.RGBA(0.1, 0.2, 0.3, 1),
	}
	commands := []render.Command{
		passRect(math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 4, Y: 3}), render.RGBA(1, 0, 0, 1)),
	}
	want := renderer.Render(commands)
	got := image.NewRGBA(want.Bounds())
	renderer.RenderPassesInto(got, nil, render.Pass{Camera: renderer.Camera, Commands: commands})

	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("empty Passes changed the raster output")
	}
}

func TestRendererPassesKeepExtraContentInFramePostChain(t *testing.T) {
	renderer := raster.Renderer{
		Width:  4,
		Height: 4,
		Camera: passCamera(
			math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}),
			math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}),
		),
		Clear: render.RGBA(0, 0, 0, 1),
	}
	passes := []render.Pass{{
		Camera:   renderer.Camera,
		Commands: []render.Command{passRect(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}), render.RGBA(1, 0, 0, 1))},
	}}
	base := render.Pass{
		Camera: renderer.Camera,
		Commands: []render.Command{render.PostChain{Effects: []render.PostEffect{
			render.Fade{Color: render.RGBA(0, 0, 0, 1), Amount: 0.5},
		}}},
	}

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	renderer.RenderPassesInto(img, passes, base)
	if got := img.RGBAAt(2, 2); got != (color.RGBA{R: 128, A: 255}) {
		t.Fatalf("post-processed extra pass pixel = %v, want half-red", got)
	}
}
