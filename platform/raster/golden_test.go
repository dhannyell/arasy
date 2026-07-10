package raster

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhannyell/frameforge/asset/pixel"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

var update = flag.Bool("update", false, "rewrite golden files")

func assertGolden(t *testing.T, name string, img *image.RGBA) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".png")

	if *update {
		writePNG(t, path, img)
		t.Logf("updated %s", path)
		return
	}

	want := decodePNG(t, path)

	diff := countPixelDiff(want, img)
	if diff.count == 0 {
		return
	}

	failurePath := filepath.Join("testdata", "failures", name+".png")
	writePNG(t, failurePath, img)
	t.Errorf("%s: %d pixels differ (first at %v); got image saved to %s",
		name, diff.count, diff.first, failurePath)
}

func decodePNG(t *testing.T, path string) *image.RGBA {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run go test ./platform/raster -run TestGolden -update)", path, err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode golden %s: %v", path, err)
	}

	rgba, ok := img.(*image.RGBA)
	if ok {
		return rgba
	}

	bounds := img.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.Set(x, y, img.At(x, y))
		}
	}
	return out
}

func writePNG(t *testing.T, path string, img *image.RGBA) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

type pixelDiff struct {
	count int
	first image.Point
}

func countPixelDiff(want, got *image.RGBA) pixelDiff {
	if want.Bounds() != got.Bounds() {
		return pixelDiff{count: 1, first: got.Bounds().Min}
	}

	diff := pixelDiff{first: image.Point{X: -1, Y: -1}}
	bounds := want.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if want.RGBAAt(x, y) == got.RGBAAt(x, y) {
				continue
			}
			if diff.count == 0 {
				diff.first = image.Point{X: x, Y: y}
			}
			diff.count++
		}
	}
	return diff
}

func TestGoldenRects(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: 8, Y: 8}, math2d.Vec2{X: 22, Y: 16}),
			Fill: render.RGBA(0.9, 0.25, 0.18, 1),
		},
		render.DrawRect{
			Rect:        math2d.NewRect(math2d.Vec2{X: 38, Y: 10}, math2d.Vec2{X: 20, Y: 20}),
			Stroke:      render.RGBA(0.88, 0.95, 1, 1),
			StrokeWidth: 2,
		},
		render.DrawRect{
			Rect:         math2d.NewRect(math2d.Vec2{X: 68, Y: 9}, math2d.Vec2{X: 22, Y: 18}),
			Fill:         render.RGBA(0.12, 0.63, 0.72, 1),
			Stroke:       render.RGBA(1, 0.85, 0.25, 0.85),
			StrokeWidth:  3,
			CornerRadius: 4,
		},
		render.DrawRect{
			Rect:        math2d.NewRect(math2d.Vec2{X: 18, Y: 42}, math2d.Vec2{X: 0, Y: 20}),
			Fill:        render.RGBA(1, 1, 1, 1),
			Stroke:      render.RGBA(1, 0, 1, 1),
			StrokeWidth: 1,
		},
		render.DrawRect{
			BaseCommand: render.BaseCommand{
				Clip: math2d.NewRect(math2d.Vec2{X: 56, Y: 42}, math2d.Vec2{X: 22, Y: 14}),
			},
			Rect:         math2d.NewRect(math2d.Vec2{X: 48, Y: 36}, math2d.Vec2{X: 40, Y: 26}),
			Fill:         render.RGBA(0.35, 0.9, 1, 0.78),
			Stroke:       render.RGBA(1, 1, 1, 1),
			StrokeWidth:  1.4,
			CornerRadius: 5,
		},
	})

	assertGolden(t, "rects", img)
}

func TestGoldenCirclesPoints(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawCircle{
			Center: math2d.Vec2{X: 22, Y: 21},
			Radius: 10,
			Fill:   render.RGBA(0.22, 0.74, 0.36, 1),
		},
		render.DrawCircle{
			Center:      math2d.Vec2{X: 52, Y: 22},
			Radius:      10,
			Stroke:      render.RGBA(0.95, 0.95, 0.35, 1),
			StrokeWidth: 2,
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 77, Y: 21},
			Radius: 0.45,
			Fill:   render.RGBA(1, 0.35, 0.7, 1),
		},
		render.DrawPoint{
			Center: math2d.Vec2{X: 28, Y: 54},
			Radius: 2.25,
			Color:  render.RGBA(0.4, 0.75, 1, 1),
		},
		render.DrawPoint{
			Center: math2d.Vec2{X: 63, Y: 52},
			Radius: 4,
			Color:  render.RGBA(1, 0.55, 0.2, 0.7),
		},
	})

	assertGolden(t, "circles_points", img)
}

func TestGoldenLinesArcs(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawLine{
			From:  math2d.Vec2{X: 10, Y: 12},
			To:    math2d.Vec2{X: 42, Y: 36},
			Width: 1.4,
			Color: render.RGBA(0.94, 0.35, 0.2, 1),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 90, Y: 12},
			To:    math2d.Vec2{X: 58, Y: 36},
			Width: 2.4,
			Color: render.RGBA(0.28, 0.8, 0.95, 1),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 10, Y: 63},
			To:    math2d.Vec2{X: 42, Y: 39},
			Width: 3.4,
			Color: render.RGBA(0.82, 0.72, 1, 0.78),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 90, Y: 63},
			To:    math2d.Vec2{X: 58, Y: 39},
			Width: 0.8,
			Color: render.RGBA(0.7, 1, 0.45, 1),
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: 32, Y: 53},
			Radius:     12,
			StartAngle: math.Pi * 1.72,
			EndAngle:   math.Pi * 0.28,
			Width:      2,
			Color:      render.RGBA(1, 0.82, 0.22, 1),
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: 71, Y: 53},
			Radius:     13,
			StartAngle: math.Pi * 0.18,
			EndAngle:   math.Pi * 1.38,
			Width:      3,
			Color:      render.RGBA(1, 0.4, 0.72, 0.82),
		},
	})

	assertGolden(t, "lines_arcs", img)
}

func TestGoldenText(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawLine{
			From:  math2d.Vec2{X: 8, Y: 20},
			To:    math2d.Vec2{X: 92, Y: 20},
			Width: 0.35,
			Color: render.RGBA(1, 1, 1, 0.28),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 8, Y: 44},
			To:    math2d.Vec2{X: 92, Y: 44},
			Width: 0.35,
			Color: render.RGBA(1, 1, 1, 0.28),
		},
		render.DrawText{
			Position: math2d.Vec2{X: 10, Y: 20},
			Text:     "Left",
			Color:    render.RGBA(0.95, 0.95, 0.95, 1),
			Size:     6.5,
			Align:    render.AlignLeft,
			Baseline: render.BaselineTop,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 50, Y: 20},
			Text:     "Mid",
			Color:    render.RGBA(0.35, 0.9, 1, 1),
			Size:     6.5,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 90, Y: 20},
			Text:     "Right",
			Color:    render.RGBA(1, 0.76, 0.3, 1),
			Size:     6.5,
			Align:    render.AlignRight,
			Baseline: render.BaselineBottom,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 16, Y: 50},
			Text:     "2x",
			Color:    render.RGBA(1, 0.4, 0.55, 0.86),
			Size:     13,
			Align:    render.AlignLeft,
			Baseline: render.BaselineAlphabetic,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 80, Y: 58},
			Text:     "",
			Color:    render.RGBA(1, 1, 1, 1),
			Size:     13,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 64, Y: 58},
			Text:     "Tilt",
			Color:    render.RGBA(1, 0.85, 0.25, 1),
			Size:     7.5,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
			Rotation: -math.Pi / 5,
		},
	})

	assertGolden(t, "text", img)
}

func TestGoldenPolygons(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawPolygon{
			Points: []math2d.Vec2{
				{X: 16.67, Y: 6.75},
				{X: 27.06, Y: 24.75},
				{X: 6.28, Y: 24.75},
			},
			Fill:        render.RGBA(0.95, 0.95, 0.35, 1),
			Stroke:      render.Color{R: 0.1, G: 0.7, B: 0.1, A: 1.0},
			StrokeWidth: 1.0,
		},
		render.DrawPolygon{
			Points: []math2d.Vec2{
				{X: 50.00, Y: 6.75},
				{X: 62.00, Y: 18.75},
				{X: 50.00, Y: 30.75},
				{X: 38.00, Y: 18.75},
			},
			Fill:        render.RGBA(0.23, 0.54, 0.82, 1),
			Stroke:      render.RGBA(0.43, 0.15, 0.95, 1),
			StrokeWidth: 2.0,
		},
		render.DrawPolygon{
			Points: []math2d.Vec2{
				{X: 83.33, Y: 6.75},
				{X: 94.74, Y: 15.04},
				{X: 90.38, Y: 28.46},
				{X: 76.28, Y: 28.46},
				{X: 71.92, Y: 15.04},
			},
			Fill:        render.RGBA(1, 0.35, 0.7, 1),
			Stroke:      render.RGBA(0.88, 0.95, 1, 1),
			StrokeWidth: 3,
		},
		render.DrawPolygon{
			Points: []math2d.Vec2{
				{X: 25.00, Y: 44.25},
				{X: 35.39, Y: 50.25},
				{X: 35.39, Y: 62.25},
				{X: 25.00, Y: 68.25},
				{X: 14.61, Y: 62.25},
				{X: 14.61, Y: 50.25},
			},
			Fill:        render.RGBA(0.4, 0.75, 1, 1),
			Stroke:      render.RGBA(1, 0.85, 0.25, 0.85),
			StrokeWidth: 4,
		},
		render.DrawPolygon{
			Points: []math2d.Vec2{
				{X: 75.00, Y: 44.25},
				{X: 83.49, Y: 47.76},
				{X: 87.00, Y: 56.25},
				{X: 83.49, Y: 64.74},
				{X: 75.00, Y: 68.25},
				{X: 66.51, Y: 64.74},
				{X: 63.00, Y: 56.25},
				{X: 66.51, Y: 47.76},
			},
			Fill:        render.RGBA(1, 0.55, 0.2, 0.7),
			Stroke:      render.RGBA(1, 0, 1, 1),
			StrokeWidth: 5,
		},
		render.DrawPolygon{
			BaseCommand: render.BaseCommand{
				Clip: math2d.NewRect(math2d.Vec2{X: 42, Y: 44}, math2d.Vec2{X: 16, Y: 24}),
			},
			Points: []math2d.Vec2{
				{X: 39, Y: 42},
				{X: 60, Y: 47},
				{X: 56, Y: 70},
				{X: 40, Y: 66},
			},
			Fill:        render.RGBA(0.55, 1, 0.38, 0.62),
			Stroke:      render.RGBA(0.95, 0.95, 1, 0.9),
			StrokeWidth: 1.2,
		},
	})

	assertGolden(t, "polygons", img)
}

func TestGoldenAlpha(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 60, Y: 44}),
			Fill: render.RGBA(0.12, 0.62, 0.78, 0.55),
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 54, Y: 33},
			Radius: 22,
			Fill:   render.RGBA(1, 0.36, 0.18, 0.45),
		},
		render.DrawRect{
			Rect:        math2d.NewRect(math2d.Vec2{X: 24, Y: 22}, math2d.Vec2{X: 42, Y: 31}),
			Stroke:      render.RGBA(1, 1, 1, 0.42),
			StrokeWidth: 4,
			BaseCommand: render.BaseCommand{LayerValue: 2},
		},
		render.DrawRect{
			Rect:        math2d.NewRect(math2d.Vec2{X: 37, Y: 16}, math2d.Vec2{X: 20, Y: 20}),
			Fill:        render.RGBA(1, 0.55, 0.05, 0.22),
			Stroke:      render.RGBA(1, 0.75, 0.05, 0.9),
			StrokeWidth: 0.12,
			BaseCommand: render.BaseCommand{LayerValue: 3},
		},
	})

	assertGolden(t, "alpha", img)
}

func TestGoldenDebugOverlay(t *testing.T) {
	world := goldenDebugWorld()
	const expectedChecksum uint64 = 4172259945519101670
	if got := world.Checksum(); got != expectedChecksum {
		t.Fatalf("debug world checksum = %d, want %d", got, expectedChecksum)
	}

	commands := world.RenderCommands()
	commands = append(commands, world.DebugCommands(
		scene.DebugGrid|scene.DebugActiveGrid|scene.DebugCandidateGrid|scene.DebugColliders|
			scene.DebugContacts|scene.DebugVelocities|scene.DebugEntityIDs,
		scene.WithDebugBounds(goldenDebugBounds()),
		scene.WithDebugCellSize(8),
	)...)

	assertGolden(t, "debug_overlay", goldenRenderer().Render(commands))
}

func TestGoldenCameraAspect(t *testing.T) {
	commands := []render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Fill: render.RGBA(0.16, 0.19, 0.22, 1),
		},
		render.DrawCircle{
			Center:      math2d.Vec2{X: 50, Y: 37.5},
			Radius:      22,
			Fill:        render.RGBA(0.25, 0.72, 0.5, 0.8),
			Stroke:      render.RGBA(1, 0.95, 0.55, 1),
			StrokeWidth: 1.2,
		},
		render.DrawRect{
			Rect:        math2d.NewRect(math2d.Vec2{X: 6, Y: 6}, math2d.Vec2{X: 88, Y: 63}),
			Stroke:      render.RGBA(0.95, 0.95, 1, 1),
			StrokeWidth: 1,
		},
	}

	img := image.NewRGBA(image.Rect(0, 0, 260, 150))
	left := Renderer{
		Width:  260,
		Height: 150,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 125, Y: 150}),
			Aspect:   render.AspectFit,
		},
		Clear: goldenClear,
	}.Render(commands)
	right := Renderer{
		Width:  260,
		Height: 150,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{X: 135, Y: 0}, math2d.Vec2{X: 125, Y: 150}),
			Aspect:   render.AspectExpand,
		},
		Clear: render.RGBA(0, 0, 0, 0),
	}.Render(commands)

	copy(img.Pix, left.Pix)
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 135; x < img.Bounds().Dx(); x++ {
			img.SetRGBA(x, y, right.RGBAAt(x, y))
		}
	}

	assertGolden(t, "camera_aspect", img)
}

var goldenClear = render.RGBA(0.055, 0.065, 0.075, 1)

func goldenRenderer() Renderer {
	return Renderer{
		Width:  200,
		Height: 150,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 150}),
			Aspect:   render.AspectFit,
		},
		Clear: goldenClear,
	}
}

func goldenDebugBounds() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75})
}

func goldenDebugWorld() *scene.World {
	world := scene.NewWorld()

	spawnGoldenCircle(world, math2d.Vec2{X: 26, Y: 32}, math2d.Vec2{X: 8, Y: -4}, 9, render.RGBA(0.2, 0.65, 1, 0.82), 1)
	spawnGoldenCircle(world, math2d.Vec2{X: 41, Y: 33}, math2d.Vec2{X: -6, Y: 5}, 8, render.RGBA(1, 0.45, 0.28, 0.82), 2)
	spawnGoldenCircle(world, math2d.Vec2{X: 74, Y: 48}, math2d.Vec2{X: -3, Y: -8}, 6, render.RGBA(0.62, 1, 0.42, 0.82), 3)
	spawnGoldenRect(world, math2d.Vec2{X: 57, Y: 29}, math2d.Vec2{X: 9, Y: 7}, math2d.Vec2{X: 2, Y: 4}, render.RGBA(0.92, 0.76, 0.22, 0.78), 0)
	spawnGoldenRect(world, math2d.Vec2{X: 66, Y: 35}, math2d.Vec2{X: 8, Y: 8}, math2d.Vec2{X: -2, Y: -3}, render.RGBA(0.76, 0.46, 1, 0.72), 0)

	world.Step(1.0/60.0, scene.StepConfig{
		Bounds: goldenDebugBounds(),
		Physics: scene.PhysicsConfig{
			CellSize: 8,
		},
	})

	return world
}

func spawnGoldenCircle(world *scene.World, position, velocity math2d.Vec2, radius float64, color render.Color, layer int) {
	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{Position: position, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetVelocity(entity, scene.Velocity{Linear: velocity})
	world.SetBody(entity, scene.Body{InvMass: 1, Restitution: 0.65})
	world.SetCircleCollider(entity, scene.CircleCollider{Radius: radius})
	world.SetCircleSprite(entity, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Layer: layer, Color: color},
		Radius:     radius,
	})
}

func spawnGoldenRect(world *scene.World, position, halfExtents, velocity math2d.Vec2, color render.Color, layer int) {
	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{Position: position, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetVelocity(entity, scene.Velocity{Linear: velocity})
	world.SetBody(entity, scene.Body{InvMass: 1, Restitution: 0.4})
	world.SetRectCollider(entity, scene.RectCollider{HalfExtends: halfExtents})
	world.SetRectSprite(entity, scene.RectSprite{
		BaseSprite:  scene.BaseSprite{Layer: layer, Color: color},
		Rect:        math2d.NewRectFromCorners(position.Sub(halfExtents), position.Add(halfExtents)),
		Stroke:      render.RGBA(1, 1, 1, 0.58),
		StrokeWidth: 0.8,
	})
}

// goldenImageTexture builds a deterministic 8×8 texture: a 2×2 checkerboard
// of 4 opaque colours, with the bottom-right quadrant semi-transparent (alpha
// ≈ 50 %, premultiplied).
//
//	red          teal
//	(0,0)-(4,4)  (4,0)-(8,4)
//
//	purple       gold (50 %)
//	(0,4)-(4,8)  (4,4)-(8,8)
func goldenImageTexture() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))

	// Top-left: red opaque.
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 220, G: 50, B: 32, A: 255})
		}
	}
	// Top-right: teal opaque.
	for y := 0; y < 4; y++ {
		for x := 4; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 30, G: 180, B: 190, A: 255})
		}
	}
	// Bottom-left: purple opaque.
	for y := 4; y < 8; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 140, G: 60, B: 200, A: 255})
		}
	}
	// Bottom-right: gold semi-transparent (premultiplied: straight {200,170,40} × A 128/255).
	for y := 4; y < 8; y++ {
		for x := 4; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 100, G: 85, B: 20, A: 128})
		}
	}
	return img
}

func TestGoldenImages(t *testing.T) {
	r := goldenRenderer()
	r.Textures = pixel.Registry{nil, goldenImageTexture()}

	img := r.Render([]render.Command{
		// 1 — Scaled up 2.5× (8×8 texture → 20×20 world rect).
		render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{X: 5, Y: 5}, math2d.Vec2{X: 20, Y: 20}),
		},
		// 2 — Src sub-rect: bottom-left quadrant of the texture (purple).
		render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{X: 30, Y: 5}, math2d.Vec2{X: 20, Y: 20}),
			Src:     math2d.Rect{Min: math2d.Vec2{X: 0, Y: 4}, Max: math2d.Vec2{X: 4, Y: 8}},
		},
		// 3 — Tinted: green-blue tint at 70 % opacity over the full texture.
		render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{X: 55, Y: 5}, math2d.Vec2{X: 20, Y: 20}),
			Tint:    render.RGBA(0.2, 0.8, 0.4, 0.7),
		},
		// 4 — Rotated ~30° around the center of Dest.
		render.DrawImage{
			Texture:  1,
			Dest:     math2d.NewRect(math2d.Vec2{X: 5, Y: 35}, math2d.Vec2{X: 25, Y: 25}),
			Rotation: math.Pi / 6,
		},
		// 5 — Missing texture (ID 99) → magenta placeholder.
		render.DrawImage{
			Texture: 99,
			Dest:    math2d.NewRect(math2d.Vec2{X: 38, Y: 35}, math2d.Vec2{X: 20, Y: 20}),
		},
	})

	assertGolden(t, "images", img)
}

func TestGoldenImageQuads(t *testing.T) {
	r := goldenRenderer()
	r.Textures = pixel.Registry{nil, goldenImageTexture()}

	img := r.Render([]render.Command{
		// 1 — Plain rectangle quad (corners of an axis-aligned rect):
		// reproduces a DrawImage blit through the homography path.
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 5, Y: 5}, {X: 25, Y: 5}, {X: 25, Y: 25}, {X: 5, Y: 25},
			},
		},
		// 2 — Trapezoid: narrow top edge, wide bottom edge.
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 38, Y: 8}, {X: 52, Y: 8}, {X: 60, Y: 28}, {X: 30, Y: 28},
			},
		},
		// 3 — Quad with genuine perspective (non-parallelogram): converging
		// left edge, to exercise the projective (non-affine) homography path.
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 68, Y: 5}, {X: 92, Y: 10}, {X: 92, Y: 26}, {X: 68, Y: 28},
			},
		},
		// 4 — Src sub-rect + Tint, on a plain rectangle quad.
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 5, Y: 35}, {X: 25, Y: 35}, {X: 25, Y: 55}, {X: 5, Y: 55},
			},
			Src:  math2d.Rect{Min: math2d.Vec2{X: 4, Y: 0}, Max: math2d.Vec2{X: 8, Y: 4}},
			Tint: render.RGBA(0.3, 0.9, 0.6, 0.75),
		},
		// 5 — Missing texture (ID 99) on a rotated (non-rectangular)
		// quad -> magenta placeholder polygon.
		render.DrawImageQuad{
			Texture: 99,
			Corners: [4]math2d.Vec2{
				{X: 45, Y: 40}, {X: 65, Y: 34}, {X: 72, Y: 54}, {X: 50, Y: 60},
			},
		},
		// 6 — Degenerate corners (three collinear points): skipped entirely,
		// nothing drawn here.
		render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: 80, Y: 40}, {X: 86, Y: 40}, {X: 92, Y: 40}, {X: 86, Y: 60},
			},
		},
	})

	assertGolden(t, "image_quads", img)
}
