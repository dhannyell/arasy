package raster_test

import (
	"bytes"
	"image"
	"testing"

	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
	"golang.org/x/image/font/gofont/goregular"
)

func TestDrawTypefaceUsesCommandPosition(t *testing.T) {
	renderer := newTypefaceRenderer(t, 120, 80)
	img := renderer.Render([]render.Command{typefaceCommand("Frame", math2d.Vec2{X: 40, Y: 24})})

	bounds, ok := paintedBounds(img)
	if !ok {
		t.Fatal("custom typeface produced no visible pixels")
	}
	if bounds.Min.X < 40 || bounds.Min.Y < 24 {
		t.Fatalf("painted bounds %v precede the top-left anchor (40,24)", bounds)
	}
}

func TestDrawTypefaceCompositesTranslucentGlyphsOnce(t *testing.T) {
	renderer := newTypefaceRenderer(t, 120, 80)
	command := typefaceCommand("MMMM", math2d.Vec2{X: 8, Y: 8})
	command.Size = 40
	command.Color = render.RGBA(1, 1, 1, 0.5)
	img := renderer.Render([]render.Command{command})

	maxAlpha := uint8(0)
	for i := 3; i < len(img.Pix); i += 4 {
		maxAlpha = max(maxAlpha, img.Pix[i])
	}
	if maxAlpha == 0 {
		t.Fatal("translucent custom typeface produced no visible pixels")
	}
	if maxAlpha > 128 {
		t.Fatalf("maximum alpha = %d, want at most 128 from one 50%%-alpha composite", maxAlpha)
	}
}

func TestDrawTypefaceUnknownFontFallsBackToBuiltin(t *testing.T) {
	renderer := newTypefaceRenderer(t, 120, 80)
	command := typefaceCommand("fallback", math2d.Vec2{X: 8, Y: 8})
	command.Font = 99
	got := renderer.Render([]render.Command{command})

	command.Font = 0
	want := renderer.Render([]render.Command{command})
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Fatal("unknown font does not preserve the built-in Font(0) fallback")
	}
}

func TestDrawTypefaceIsProportional(t *testing.T) {
	renderer := newTypefaceRenderer(t, 160, 80)
	narrow := typefaceCommand("iiii", math2d.Vec2{X: 8, Y: 8})
	wide := typefaceCommand("MMMM", math2d.Vec2{X: 8, Y: 8})

	narrowBounds, narrowOK := paintedBounds(renderer.Render([]render.Command{narrow}))
	wideBounds, wideOK := paintedBounds(renderer.Render([]render.Command{wide}))
	if !narrowOK || !wideOK {
		t.Fatal("custom typeface comparison produced an empty image")
	}
	if narrowBounds.Dx() >= wideBounds.Dx() {
		t.Fatalf("painted widths: iiii=%d, MMMM=%d; want proportional narrow text", narrowBounds.Dx(), wideBounds.Dx())
	}
}

func TestDrawTypefaceHonorsAlignAndBaseline(t *testing.T) {
	renderer := newTypefaceRenderer(t, 160, 120)
	anchor := math2d.Vec2{X: 80, Y: 60}

	leftTop := typefaceCommand("Frame", anchor)
	leftTop.Align = render.AlignLeft
	leftTop.Baseline = render.BaselineTop
	leftTopBounds, ok := paintedBounds(renderer.Render([]render.Command{leftTop}))
	if !ok || leftTopBounds.Min.X < int(anchor.X) || leftTopBounds.Min.Y < int(anchor.Y) {
		t.Fatalf("left/top painted bounds = %v, want pixels after anchor %v", leftTopBounds, anchor)
	}

	rightBottom := leftTop
	rightBottom.Align = render.AlignRight
	rightBottom.Baseline = render.BaselineBottom
	rightBottomBounds, ok := paintedBounds(renderer.Render([]render.Command{rightBottom}))
	if !ok || rightBottomBounds.Max.X > int(anchor.X) || rightBottomBounds.Max.Y > int(anchor.Y) {
		t.Fatalf("right/bottom painted bounds = %v, want pixels before anchor %v", rightBottomBounds, anchor)
	}

	centerMiddle := leftTop
	centerMiddle.Align = render.AlignCenter
	centerMiddle.Baseline = render.BaselineMiddle
	centerBounds, ok := paintedBounds(renderer.Render([]render.Command{centerMiddle}))
	if !ok || centerBounds.Min.X >= int(anchor.X) || centerBounds.Max.X <= int(anchor.X) ||
		centerBounds.Min.Y >= int(anchor.Y) || centerBounds.Max.Y <= int(anchor.Y) {
		t.Fatalf("center/middle painted bounds = %v, want bounds spanning anchor %v", centerBounds, anchor)
	}
}

func TestDrawTypefaceRotationHonorsClip(t *testing.T) {
	renderer := newTypefaceRenderer(t, 120, 120)
	command := typefaceCommand("Rotate", math2d.Vec2{X: 60, Y: 60})
	command.Align = render.AlignCenter
	command.Baseline = render.BaselineMiddle
	command.Rotation = 0.7
	command.Clip = math2d.Rect{
		Min: math2d.Vec2{X: 48, Y: 48},
		Max: math2d.Vec2{X: 72, Y: 72},
	}

	img := renderer.Render([]render.Command{command})
	bounds, ok := paintedBounds(img)
	if !ok {
		t.Fatal("rotated clipped custom typeface produced no visible pixels")
	}
	clip := image.Rect(48, 48, 72, 72)
	if bounds != bounds.Intersect(clip) {
		t.Fatalf("painted bounds %v escape clip %v", bounds, clip)
	}
}

func TestDrawTypefaceIsDeterministic(t *testing.T) {
	renderer := newTypefaceRenderer(t, 120, 80)
	command := typefaceCommand("deterministic", math2d.Vec2{X: 8, Y: 8})

	first := renderer.Render([]render.Command{command})
	second := renderer.Render([]render.Command{command})
	if !bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("identical custom typeface renders differ")
	}
}

func newTypefaceRenderer(t *testing.T, width, height int) raster.Renderer {
	t.Helper()
	face, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parse goregular: %v", err)
	}
	return raster.Renderer{
		Width:  width,
		Height: height,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: float64(width), Y: float64(height)}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: float64(width), Y: float64(height)}),
		},
		Fonts: typeface.Registry{nil, face},
	}
}

func typefaceCommand(text string, position math2d.Vec2) render.DrawText {
	return render.DrawText{
		Position: position,
		Text:     text,
		Color:    render.RGBA(1, 1, 1, 1),
		Size:     24,
		Font:     1,
		Align:    render.AlignLeft,
		Baseline: render.BaselineTop,
	}
}

func paintedBounds(img *image.RGBA) (image.Rectangle, bool) {
	minX, minY := img.Bounds().Max.X, img.Bounds().Max.Y
	maxX, maxY := img.Bounds().Min.X, img.Bounds().Min.Y
	found := false
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if img.RGBAAt(x, y).A == 0 {
				continue
			}
			found = true
			minX = min(minX, x)
			minY = min(minY, y)
			maxX = max(maxX, x+1)
			maxY = max(maxY, y+1)
		}
	}
	return image.Rect(minX, minY, maxX, maxY), found
}

func TestDrawTypefaceNonPositiveSizeDoesNotPanic(t *testing.T) {
	renderer := newTypefaceRenderer(t, 120, 80)

	for _, size := range []float64{0, -1, -100} {
		command := typefaceCommand("edge", math2d.Vec2{X: 8, Y: 8})
		command.Size = size

		img := renderer.Render([]render.Command{command})
		if img == nil {
			t.Fatalf("Render() with Size=%v returned nil image", size)
		}
		if _, ok := paintedBounds(img); ok {
			t.Fatalf("Render() with Size=%v painted pixels, want an early return", size)
		}
	}
}

func TestDrawTypefaceRoundsToNonPositivePixelSizeReturnsEarly(t *testing.T) {
	face := newTypefaceRenderer(t, 200, 200)
	// Shrink the camera's screen scale enough that even a normal Size
	// rounds down to a zero pixel face size (px <= 0 in drawText).
	face.Camera = render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 20000, Y: 20000}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 200}),
	}
	command := typefaceCommand("tiny", math2d.Vec2{X: 8, Y: 8})
	command.Size = 24

	img := face.Render([]render.Command{command})
	if img == nil {
		t.Fatal("Render() with a tiny camera scale returned nil image")
	}
	if _, ok := paintedBounds(img); ok {
		t.Fatal("Render() painted pixels despite a camera scale that rounds the pixel size to <= 0")
	}
}
