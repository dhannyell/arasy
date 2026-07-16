package raster

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

var update = flag.Bool("update", false, "rewrite golden files")

func assertGolden(t *testing.T, name string, img *image.RGBA) {
	assertGoldenTolerance(t, name, img, 0)
}

func assertGoldenTolerance(t *testing.T, name string, img *image.RGBA, channelTolerance uint8) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".png")

	if *update {
		writePNG(t, path, img)
		t.Logf("updated %s", path)
		return
	}

	want := decodePNG(t, path)

	diff := countPixelDiff(want, img, channelTolerance)
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

func countPixelDiff(want, got *image.RGBA, channelTolerance uint8) pixelDiff {
	if want.Bounds() != got.Bounds() {
		return pixelDiff{count: 1, first: got.Bounds().Min}
	}

	diff := pixelDiff{first: image.Point{X: -1, Y: -1}}
	bounds := want.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if colorsWithinTolerance(want.RGBAAt(x, y), got.RGBAAt(x, y), channelTolerance) {
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

func colorsWithinTolerance(want, got color.RGBA, tolerance uint8) bool {
	return channelWithinTolerance(want.R, got.R, tolerance) &&
		channelWithinTolerance(want.G, got.G, tolerance) &&
		channelWithinTolerance(want.B, got.B, tolerance) &&
		channelWithinTolerance(want.A, got.A, tolerance)
}

func channelWithinTolerance(want, got, tolerance uint8) bool {
	if want > got {
		return want-got <= tolerance
	}
	return got-want <= tolerance
}

var goldenClear = render.RGBA(0.055, 0.065, 0.075, 1)

func goldenRenderer() *Renderer {
	return &Renderer{
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

func TestCountPixelDiffHonorsChannelTolerance(t *testing.T) {
	want := image.NewRGBA(image.Rect(0, 0, 2, 1))
	got := image.NewRGBA(want.Bounds())
	want.SetRGBA(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 40})
	got.SetRGBA(0, 0, color.RGBA{R: 11, G: 19, B: 30, A: 41})
	want.SetRGBA(1, 0, color.RGBA{R: 100, G: 110, B: 120, A: 130})
	got.SetRGBA(1, 0, color.RGBA{R: 100, G: 112, B: 120, A: 130})

	diff := countPixelDiff(want, got, 1)
	if diff.count != 1 || diff.first != (image.Point{X: 1, Y: 0}) {
		t.Fatalf("countPixelDiff() = %+v, want one diff at (1,0)", diff)
	}
}

func TestCountPixelDiffRemainsExactAtZeroTolerance(t *testing.T) {
	want := image.NewRGBA(image.Rect(0, 0, 1, 1))
	got := image.NewRGBA(want.Bounds())
	want.SetRGBA(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 40})
	got.SetRGBA(0, 0, color.RGBA{R: 11, G: 20, B: 30, A: 40})

	if diff := countPixelDiff(want, got, 0); diff.count != 1 {
		t.Fatalf("countPixelDiff() count = %d, want 1", diff.count)
	}
}
