package raster

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestClonePixelsDoesNotAliasTheSource(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2, 1))
	source.SetRGBA(0, 0, color.RGBA{R: 20, A: 255})
	clone := clonePixels(source)
	clone.SetRGBA(0, 0, color.RGBA{B: 30, A: 255})

	if got := source.RGBAAt(0, 0); got != (color.RGBA{R: 20, A: 255}) {
		t.Fatalf("source pixel changed through clone: %#v", got)
	}
}

func TestSampleBilinearInterpolatesAndClampsToTheImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{R: 0, A: 255})
	img.SetRGBA(1, 0, color.RGBA{R: 255, A: 255})
	img.SetRGBA(0, 1, color.RGBA{R: 255, A: 255})
	img.SetRGBA(1, 1, color.RGBA{R: 0, A: 255})

	if got := sampleBilinear(img, 0.5, 0.5); got.r != 0.5 {
		t.Fatalf("center red = %v, want 0.5", got.r)
	}
	if got := sampleBilinear(img, -10, 10); got.r != 1 {
		t.Fatalf("clamped red = %v, want 1", got.r)
	}
}

func TestPostChainPartitionsWorldAndAboveLayerCommands(t *testing.T) {
	r := Renderer{Width: 2, Height: 1, Camera: render.Camera{
		World:    imageRect(0, 0, 2, 1),
		Viewport: imageRect(0, 0, 2, 1),
	}, Clear: render.RGBA(0, 0, 0, 1)}
	commands := []render.Command{
		render.DrawRect{BaseCommand: render.BaseCommand{LayerValue: 1}, Rect: imageRect(0, 0, 2, 1), Fill: render.RGBA(1, 0, 0, 1)},
		render.PostChain{BelowLayer: 2},
		render.DrawRect{BaseCommand: render.BaseCommand{LayerValue: 2}, Rect: imageRect(0, 0, 2, 1), Fill: render.RGBA(0, 0, 1, 1)},
	}

	got := r.Render(commands)
	if pixel := got.RGBAAt(0, 0); pixel != (color.RGBA{B: 255, A: 255}) {
		t.Fatalf("above-layer command = %#v, want opaque blue", pixel)
	}
}

func TestVignetteGetsStrongerTowardTheCorner(t *testing.T) {
	src := solidPostImage(4, 4, color.RGBA{R: 200, G: 200, B: 200, A: 255})
	got := applyVignette(src, render.Vignette{Color: render.RGBA(0, 0, 0, 1), Strength: 1, Softness: 1})

	center := pixelAt(got, 2, 2).r
	corner := pixelAt(got, 0, 0).r
	if corner >= center {
		t.Fatalf("corner red = %v, center red = %v; vignette is not monotonic", corner, center)
	}
}

func TestColorGradeMinusOneSaturationProducesGrayscale(t *testing.T) {
	src := solidPostImage(1, 1, color.RGBA{R: 200, G: 80, B: 20, A: 255})
	got := applyColorGrade(src, render.ColorGrade{Saturation: -1})
	pixel := got.RGBAAt(0, 0)
	if pixel.R != pixel.G || pixel.G != pixel.B {
		t.Fatalf("grayscale pixel = %#v, want equal RGB channels", pixel)
	}
}

func TestPixelateKeepsEachBlockUniform(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for x := 0; x < 4; x++ {
		for y := 0; y < 2; y++ {
			src.SetRGBA(x, y, color.RGBA{R: uint8(x * 60), A: 255})
		}
	}

	got := applyPixelate(src, render.Pixelate{Size: 2})
	if got.RGBAAt(0, 0) != got.RGBAAt(1, 0) || got.RGBAAt(2, 0) != got.RGBAAt(3, 0) {
		t.Fatal("pixelate did not keep each 2x2 block uniform")
	}
}

func TestChromaticAberrationMovesRedAndBlueInOppositeDirections(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 8, 1))
	for x := 0; x < 8; x++ {
		src.SetRGBA(x, 0, color.RGBA{R: uint8(x * 30), B: uint8(x * 30), A: 255})
	}

	got := applyChromaticAberration(src, render.ChromaticAberration{Strength: 2})
	center := got.RGBAAt(5, 0)
	if center.R <= src.RGBAAt(5, 0).R || center.B >= src.RGBAAt(5, 0).B {
		t.Fatalf("aberrated center = %#v, source = %#v; channels did not move oppositely", center, src.RGBAAt(5, 0))
	}
}

func TestBloomThresholdAboveEveryPixelIsIdentity(t *testing.T) {
	src := solidPostImage(4, 4, color.RGBA{R: 180, G: 120, B: 40, A: 255})
	got := applyBloom(src, render.Bloom{Threshold: 1, Intensity: 1, Passes: 3})

	if !bytes.Equal(src.Pix, got.Pix) {
		t.Fatal("bloom leaked pixels below its threshold")
	}
}

func TestBloomLeakageScalesWithIntensity(t *testing.T) {
	src := solidPostImage(16, 16, color.RGBA{R: 80, G: 20, A: 255})
	base := src.RGBAAt(8, 8).R
	half := applyBloom(src, render.Bloom{Threshold: 0.2, Intensity: 0.5, Passes: 1}).RGBAAt(8, 8).R
	full := applyBloom(src, render.Bloom{Threshold: 0.2, Intensity: 1, Passes: 1}).RGBAAt(8, 8).R

	if full <= half || half <= base {
		t.Fatalf("bloom intensities produced base=%d half=%d full=%d", base, half, full)
	}
}

func TestShockwaveRingMovesWithRadius(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 32, 1))
	for x := 0; x < 32; x++ {
		src.SetRGBA(x, 0, color.RGBA{R: uint8(x * 8), A: 255})
	}
	camera := render.Camera{
		World:    imageRect(0, 0, 32, 1),
		Viewport: imageRect(0, 0, 32, 1),
	}
	first := applyShockwave(src, render.Shockwave{Center: math2d.Vec2{X: 16, Y: 0.5}, Radius: 5, Width: 3, Strength: 2}, camera)
	second := applyShockwave(src, render.Shockwave{Center: math2d.Vec2{X: 16, Y: 0.5}, Radius: 10, Width: 3, Strength: 2}, camera)

	if bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("moving shockwave radius did not move the refracted ring")
	}
}

func TestLensRainZeroDensityIsIdentity(t *testing.T) {
	src := solidPostImage(8, 8, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	got := applyLensRain(src, render.LensRain{}, 2)

	if !bytes.Equal(src.Pix, got.Pix) {
		t.Fatal("zero-density lens rain changed the source")
	}
}

func TestLensRainIsPureForTheSameInputs(t *testing.T) {
	src := solidPostImage(64, 64, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	effect := render.LensRain{Density: 1, Size: 16, Speed: 0.15, Seed: 44}
	first := applyLensRain(src, effect, 1.25)
	second := applyLensRain(src, effect, 1.25)

	if !bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("same lens-rain inputs produced different pixels")
	}
}

func TestLensRainMovesWithTime(t *testing.T) {
	src := solidPostImage(64, 64, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	effect := render.LensRain{Density: 1, Size: 16, Speed: 0.15, Seed: 44}
	first := applyLensRain(src, effect, 0)
	second := applyLensRain(src, effect, 1)

	if bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("lens rain did not move with time")
	}
}

func TestLensRainN1DoesNotReadNeighborPixels(t *testing.T) {
	source := solidPostImage(64, 64, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	changed := clonePixels(source)
	changed.SetRGBA(32, 32, color.RGBA{R: 255, G: 10, B: 20, A: 255})
	effect := render.LensRain{Density: 1, Size: 16, Speed: 0.15, Seed: 44}
	first := applyLensRain(source, effect, 0.5)
	second := applyLensRain(changed, effect, 0.5)

	if first.RGBAAt(0, 0) != second.RGBAAt(0, 0) {
		t.Fatal("N1 lens rain read a neighboring pixel")
	}
}

func solidPostImage(width, height int, value color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, value)
		}
	}
	return img
}

func imageRect(minX, minY, maxX, maxY float64) math2d.Rect {
	return math2d.NewRect(
		math2d.Vec2{X: minX, Y: minY},
		math2d.Vec2{X: maxX, Y: maxY},
	)
}
