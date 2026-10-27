package raster

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/render"
)

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
	changed := image.NewRGBA(source.Bounds())
	copy(changed.Pix, source.Pix)
	changed.SetRGBA(32, 32, color.RGBA{R: 255, G: 10, B: 20, A: 255})
	effect := render.LensRain{Density: 1, Size: 16, Speed: 0.15, Seed: 44}
	first := applyLensRain(source, effect, 0.5)
	second := applyLensRain(changed, effect, 0.5)

	if first.RGBAAt(0, 0) != second.RGBAAt(0, 0) {
		t.Fatal("N1 lens rain read a neighboring pixel")
	}
}

func TestLensRainDropsCrossCellBorders(t *testing.T) {
	bounds := image.Rect(0, 0, 160, 64)
	size := 32.0
	layer := buildRainLayer(bounds, 1, size, 0.15, 7, 0, 0.8, 1, rainStatic)

	// Find a droplet whose disk crosses a vertical cell border.
	var borderX, centerY float64
	found := false
	for c := 0; c < layer.colCount-1 && !found; c++ {
		for r := 0; r < layer.rowCount && !found; r++ {
			cell := layer.cells[r*layer.colCount+c]
			if !cell.active {
				continue
			}
			border := float64(c+1) * size
			if math.Abs(cell.centerX-border) < cell.radius*0.9 {
				borderX, centerY, found = border, cell.centerY, true
			}
		}
	}
	if !found {
		t.Skip("no droplet near a cell border for this seed")
	}

	src := solidPostImage(160, 64, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	x := min(int(math.Round(borderX))+1, 159)
	y := min(max(int(centerY), 0), 63)
	before := pixelAt(src, x, y)
	if after := layer.apply(src, before, x, y, 0); after == before {
		t.Fatal("droplet clipped at cell border: neighbor-cell pixel unaffected")
	}
}

func TestLensRainErodeLayerHasLifecycle(t *testing.T) {
	bounds := image.Rect(0, 0, 160, 160)
	speed := 0.15
	seed := uint64(11)

	// Enumerate density-active cells with a static build, then drive one
	// droplet to a known lifecycle phase by solving the cycle time.
	enumerate := buildRainLayer(bounds, 1, 32, speed, seed, 0, 1.25, 1, rainStatic)
	cellX, cellY := -1.0, -1.0
	for idx, cell := range enumerate.cells {
		if cell.active {
			cellX = float64(idx % enumerate.colCount)
			cellY = float64(idx / enumerate.colCount)
			break
		}
	}
	if cellX < 0 {
		t.Fatal("no active cell at full density")
	}
	idx := int(cellY)*enumerate.colCount + int(cellX)

	hash := particle.Hash3([3]uint32{uint32(int32(cellX)), uint32(int32(cellY)), uint32(seed)})
	h0 := particle.Float01(hash[0])
	rate := 0.6 + 0.8*particle.Float01(hash[1])
	timeAtPhase := func(phase float64) float64 {
		return (math.Ceil(h0-phase) + phase - h0) / (speed * rate)
	}

	full := buildRainLayer(bounds, 1, 32, speed, seed, timeAtPhase(0.2), 1.25, 1, rainErode)
	if !full.cells[idx].active {
		t.Fatal("droplet inactive during the hold phase of its lifecycle")
	}
	eroded := buildRainLayer(bounds, 1, 32, speed, seed, timeAtPhase(0.99), 1.25, 1, rainErode)
	if eroded.cells[idx].active {
		t.Fatal("droplet still active at the end of erosion")
	}
}

func TestBlurZeroStrengthIsIdentity(t *testing.T) {
	src := solidPostImage(8, 8, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	got := applyBlur(src, render.Blur{})

	if !bytes.Equal(src.Pix, got.Pix) {
		t.Fatal("zero-strength blur changed the source")
	}
}

func TestBlurSoftensAHardEdge(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if x < 8 {
				src.SetRGBA(x, y, color.RGBA{A: 255})
			} else {
				src.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}

	got := applyBlur(src, render.Blur{Strength: 1, Passes: 2})
	if got.RGBAAt(8, 8).R == 255 {
		t.Fatal("blur left the edge as hard as the source")
	}
}

func TestFadeZeroAmountIsIdentity(t *testing.T) {
	src := solidPostImage(4, 4, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	got := applyFade(src, render.Fade{Color: render.RGBA(1, 1, 1, 1)})

	if !bytes.Equal(src.Pix, got.Pix) {
		t.Fatal("zero-amount fade changed the source")
	}
}

func TestFadeFullAmountMatchesTheTargetColor(t *testing.T) {
	src := solidPostImage(2, 2, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	got := applyFade(src, render.Fade{Color: render.RGBA(1, 0, 0, 1), Amount: 1})

	if pixel := got.RGBAAt(0, 0); pixel.R != 255 || pixel.G != 0 || pixel.B != 0 {
		t.Fatalf("full fade = %#v, want pure red", pixel)
	}
}

func TestCircleWipeShrinksTowardTheCenter(t *testing.T) {
	src := solidPostImage(64, 64, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	camera := render.Camera{World: imageRect(0, 0, 64, 64), Viewport: imageRect(0, 0, 64, 64)}
	effect := render.CircleWipe{Center: math2d.Vec2{X: 32, Y: 32}, Color: render.RGBA(0, 0, 0, 1), Progress: 0.5}

	corner := applyCircleWipe(src, effect, camera).RGBAAt(0, 0)
	center := applyCircleWipe(src, effect, camera).RGBAAt(32, 32)
	if corner.R >= center.R {
		t.Fatalf("corner red = %d, center red = %d; wipe should cover the corner before the center", corner.R, center.R)
	}
}

func TestDissolveZeroAmountIsIdentity(t *testing.T) {
	src := solidPostImage(16, 16, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	got := applyDissolve(src, render.Dissolve{})

	if !bytes.Equal(src.Pix, got.Pix) {
		t.Fatal("zero-amount dissolve changed the source")
	}
}

func TestDissolveFullAmountCoversEveryCell(t *testing.T) {
	src := solidPostImage(16, 16, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	got := applyDissolve(src, render.Dissolve{Amount: 1, Color: render.RGBA(1, 0, 0, 1), CellSize: 4})

	if pixel := got.RGBAAt(0, 0); pixel.R != 255 || pixel.G != 0 {
		t.Fatalf("fully dissolved pixel = %#v, want pure red", pixel)
	}
}

func TestFilmGrainZeroIntensityIsIdentity(t *testing.T) {
	src := solidPostImage(8, 8, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	got := applyFilmGrain(src, render.FilmGrain{}, 1)

	if !bytes.Equal(src.Pix, got.Pix) {
		t.Fatal("zero-intensity grain changed the source")
	}
}

func TestFilmGrainAnimatesWithTime(t *testing.T) {
	src := solidPostImage(16, 16, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	effect := render.FilmGrain{Intensity: 0.2, Seed: 9}
	first := applyFilmGrain(src, effect, 0)
	second := applyFilmGrain(src, effect, 1)

	if bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("film grain did not change between ticks")
	}
}

func TestScanlinesZeroIntensityIsIdentity(t *testing.T) {
	src := solidPostImage(16, 16, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	got := applyScanlines(src, render.Scanlines{})

	if !bytes.Equal(src.Pix, got.Pix) {
		t.Fatal("zero-intensity scanlines changed pixels")
	}
}

// Scanlines must only remove light: any brightening would wash the image out
// instead of darkening lines.
func TestScanlinesNeverBrighten(t *testing.T) {
	src := solidPostImage(8, 8, color.RGBA{R: 200, G: 200, B: 200, A: 255})
	got := applyScanlines(src, render.Scanlines{Intensity: 0.5, Thickness: 3})

	darkened := false
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if got.RGBAAt(x, y).R > 200 {
				t.Fatalf("pixel (%d,%d) brightened to %d", x, y, got.RGBAAt(x, y).R)
			}
			if got.RGBAAt(x, y).R < 200 {
				darkened = true
			}
		}
	}
	if !darkened {
		t.Fatal("scanlines darkened nothing")
	}
}

// The aperture grille must keep each column's own triad channel untouched
// while dimming the other two.
func TestScanlinesMaskFavorsOneChannelPerColumn(t *testing.T) {
	src := solidPostImage(3, 1, color.RGBA{R: 200, G: 200, B: 200, A: 255})
	got := applyScanlines(src, render.Scanlines{MaskStrength: 0.5, Thickness: 100})

	first := got.RGBAAt(0, 0)
	if first.R <= first.G || first.R <= first.B {
		t.Fatalf("column 0 = %#v, want the red channel dominant", first)
	}
	second := got.RGBAAt(1, 0)
	if second.G <= second.R || second.G <= second.B {
		t.Fatalf("column 1 = %#v, want the green channel dominant", second)
	}
}

func TestRadialBlurZeroStrengthIsIdentity(t *testing.T) {
	src := solidPostImage(16, 16, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	got := applyRadialBlur(src, render.RadialBlur{Strength: 0}, render.Camera{})

	if !bytes.Equal(src.Pix, got.Pix) {
		t.Fatal("zero-strength radial blur changed pixels")
	}
}

// Averaging taps of a uniform image must reproduce it; any drift means the
// tap weights do not sum to one.
func TestRadialBlurKeepsUniformImageUnchanged(t *testing.T) {
	src := solidPostImage(16, 16, color.RGBA{R: 90, G: 120, B: 150, A: 255})
	got := applyRadialBlur(src, render.RadialBlur{
		Center: math2d.Vec2{X: 8, Y: 8}, Strength: 0.8,
	}, render.Camera{})

	if !bytes.Equal(src.Pix, got.Pix) {
		t.Fatal("radial blur changed a uniform image")
	}
}

func TestHalftoneZeroStrengthIsIdentity(t *testing.T) {
	src := solidPostImage(16, 16, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	got := applyHalftone(src, render.Halftone{Strength: 0}, 0)

	if !bytes.Equal(src.Pix, got.Pix) {
		t.Fatal("zero-strength halftone changed pixels")
	}
}

// An ordered dither must keep a flat tone's brightness: a quarter gray turns
// a quarter of each cell white and the rest black, with nothing in between.
func TestHalftoneDithersAFlatGrayToTwoTonesAtItsMean(t *testing.T) {
	src := solidPostImage(16, 16, color.RGBA{R: 64, G: 64, B: 64, A: 255})
	got := applyHalftone(src, render.Halftone{CellSize: 8, Levels: 2, Strength: 1}, 0)

	white := 0
	for i := 0; i < len(got.Pix); i += 4 {
		switch got.Pix[i] {
		case 255:
			white++
		case 0:
		default:
			t.Fatalf("pixel %d = %d, want 0 or 255", i/4, got.Pix[i])
		}
	}
	if white != 16*16/4 {
		t.Fatalf("white pixels = %d, want %d", white, 16*16/4)
	}
}

func TestFocusBlurZeroStrengthIsIdentity(t *testing.T) {
	src := solidPostImage(16, 16, color.RGBA{R: 80, G: 110, B: 140, A: 255})
	got := applyFocusBlur(src, render.FocusBlur{Strength: 0}, render.Camera{})

	if !bytes.Equal(src.Pix, got.Pix) {
		t.Fatal("zero-strength focus blur changed pixels")
	}
}

// The focus region must stay pixel-exact while the frame outside it blurs.
func TestFocusBlurKeepsTheFocusRegionSharp(t *testing.T) {
	src := checkerPostImage(32, 32)
	camera := render.Camera{World: imageRect(0, 0, 32, 32), Viewport: imageRect(0, 0, 32, 32)}
	got := applyFocusBlur(src, render.FocusBlur{
		Points:   []math2d.Vec2{{X: 8, Y: 8}},
		Extent:   math2d.Vec2{X: 4, Y: 4},
		Falloff:  4,
		Strength: 1,
	}, camera)

	for y := 4; y < 12; y++ {
		for x := 4; x < 12; x++ {
			if got.RGBAAt(x, y) != src.RGBAAt(x, y) {
				t.Fatalf("pixel (%d, %d) inside the focus region changed", x, y)
			}
		}
	}
	if got.RGBAAt(28, 28) == src.RGBAAt(28, 28) {
		t.Fatal("pixel (28, 28) far from the focus point stayed sharp")
	}
}

func checkerPostImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			value := uint8(0)
			if (x+y)%2 == 0 {
				value = 255
			}
			img.SetRGBA(x, y, color.RGBA{R: value, G: value, B: value, A: 255})
		}
	}
	return img
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
