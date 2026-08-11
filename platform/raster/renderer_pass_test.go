package raster_test

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
)

func targetRenderer(t *testing.T) (raster.Renderer, render.TextureID) {
	t.Helper()
	manifest, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"dynamic":[{"name":"target","w":4,"h":4}]}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	textures, err := pixel.BuildRegistry(manifest)
	if err != nil {
		t.Fatal(err)
	}
	world := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4})
	return raster.Renderer{
		Width: 4, Height: 4, Camera: passCamera(world, world),
		Clear: render.RGBA(0, 0, 0, 1), Textures: textures, Assets: manifest,
	}, manifest.Texture("target")
}

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

func TestRendererPassOutsideTargetDrawsNothing(t *testing.T) {
	renderer := raster.Renderer{
		Width:  8,
		Height: 8,
		Clear:  render.RGBA(0, 0, 0, 1),
	}
	world := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 8, Y: 8})
	outside := math2d.NewRect(math2d.Vec2{X: 20, Y: 20}, math2d.Vec2{X: 8, Y: 8})
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))

	renderer.RenderPassesInto(img, []render.Pass{{
		Camera:   passCamera(world, outside),
		Commands: []render.Command{passRect(world, render.RGBA(1, 0, 0, 1))},
	}}, render.Pass{Camera: passCamera(world, math2d.Rect{})})

	if got := img.RGBAAt(4, 4); got != (color.RGBA{A: 255}) {
		t.Fatalf("outside pass pixel = %v, want clear black", got)
	}
}

func TestRendererTargetPassFeedsTheBaseStreamInDeclarationOrder(t *testing.T) {
	renderer, target := targetRenderer(t)
	base := render.Pass{Camera: renderer.Camera, Commands: []render.Command{
		render.DrawImage{Texture: target, Dest: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}), Tint: render.RGBA(1, 1, 1, 1)},
	}}
	producer := render.Pass{Target: target, Camera: renderer.Camera, Commands: []render.Command{
		passRect(math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 2, Y: 2}), render.RGBA(1, 0, 0, 1)),
	}}

	image := image.NewRGBA(image.Rect(0, 0, 4, 4))
	renderer.RenderPassesInto(image, []render.Pass{producer}, base)
	if got := image.RGBAAt(2, 2); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("target sampled in base = %v, want red", got)
	}
	if got := renderer.Textures.Lookup(target).RGBAAt(0, 0); got != (color.RGBA{}) {
		t.Fatalf("target clear pixel = %v, want transparent", got)
	}
}

func TestRendererTargetPassKeepsPreviousFrameUntilItsDeclaration(t *testing.T) {
	renderer, target := targetRenderer(t)
	texture := renderer.Textures.Lookup(target)
	draw.Draw(texture, texture.Bounds(), &image.Uniform{C: color.RGBA{B: 255, A: 255}}, image.Point{}, draw.Src)
	consumer := render.Pass{Camera: renderer.Camera, Commands: []render.Command{
		render.DrawImage{Texture: target, Dest: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}), Tint: render.RGBA(1, 1, 1, 1)},
	}}
	producer := render.Pass{Target: target, Camera: renderer.Camera, Commands: []render.Command{
		passRect(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}), render.RGBA(1, 0, 0, 1)),
	}}
	image := image.NewRGBA(image.Rect(0, 0, 4, 4))
	renderer.RenderPassesInto(image, []render.Pass{consumer, producer}, render.Pass{Camera: renderer.Camera})
	if got := image.RGBAAt(2, 2); got != (color.RGBA{B: 255, A: 255}) {
		t.Fatalf("consumer before producer = %v, want previous blue frame", got)
	}
	if got := texture.RGBAAt(2, 2); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("target after producer = %v, want red", got)
	}
}

func TestRendererTargetPassKeepsFirstOwner(t *testing.T) {
	renderer, target := targetRenderer(t)
	world := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4})
	passes := []render.Pass{
		{Target: target, Camera: renderer.Camera, Commands: []render.Command{passRect(world, render.RGBA(1, 0, 0, 1))}},
		{Target: target, Camera: renderer.Camera, Commands: []render.Command{passRect(world, render.RGBA(0, 1, 0, 1))}},
	}

	renderer.RenderPassesInto(image.NewRGBA(image.Rect(0, 0, 4, 4)), passes, render.Pass{Camera: renderer.Camera})
	if got := renderer.Textures.Lookup(target).RGBAAt(2, 2); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("target pixel = %v, want first owner's red", got)
	}
}
