//go:build !js

package wasmgolden

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dhannyell/frameforge/asset/pixel"
	"github.com/dhannyell/frameforge/asset/typeface"
	"github.com/dhannyell/frameforge/platform/raster"
	"github.com/dhannyell/frameforge/runtime"
)

// Cross-backend conformance: render the canonical fixtures through the
// raster reference in-process and diff them against the committed browser
// goldens (Playwright, 320x240, deviceScaleFactor 1 -- platform/wasm's
// wasmgolden goldens). Divergence beyond the calibrated budget means a
// backend drifted from the reference; the failure names which one.
//
// Calibration placeholders (Step 1): loose enough to pass while the matrix
// below is measured; Step 2 replaces these with 2x the measured ratio.
const (
	frameChannelTol     = 32
	frameBudget         = 0.25
	particlesChannelTol = 32
	particlesBudget     = 0.25
)

var calibrationTolerances = []uint8{4, 8, 16, 32}

func TestConformanceFrameCanvas2D(t *testing.T) {
	assertConformance(t, "frame", "canvas2d", 1, frameChannelTol, frameBudget)
}

func TestConformanceFrameWebGPU(t *testing.T) {
	assertConformance(t, "frame", "webgpu", 1, frameChannelTol, frameBudget)
}

func TestConformanceParticlesCanvas2D(t *testing.T) {
	assertConformance(t, "particles", "canvas2d", 3, particlesChannelTol, particlesBudget)
}

func TestConformanceParticlesWebGPU(t *testing.T) {
	assertConformance(t, "particles", "webgpu", 3, particlesChannelTol, particlesBudget)
}

func assertConformance(t *testing.T, fixture, backend string, frames int, channelTol uint8, budget float64) {
	t.Helper()

	game, err := newFixtureGame(fixture)
	if err != nil {
		t.Fatalf("build %s fixture: %v", fixture, err)
	}
	rasterImg := rasterFrame(t, game, frames)
	golden := loadGolden(t, backend, fixture)

	for _, tol := range calibrationTolerances {
		r, _ := diffRatio(rasterImg, golden, tol)
		t.Logf("calibration %s x %s: channelTol=%d ratio=%.4f%%", fixture, backend, tol, r*100)
	}

	ratio, first := diffRatio(rasterImg, golden, channelTol)
	t.Logf("%s x %s: %.4f%% pixels differ (channelTol=%d, budget=%.4f)", fixture, backend, ratio*100, channelTol, budget)

	if ratio > budget {
		failurePath := filepath.Join("testdata", "failures", fixture+"-"+backend+".png")
		writePNG(t, failurePath, rasterImg)
		t.Errorf("%s x %s: %.4f%% pixels differ (first at %v), budget %.4f; raster render saved to %s",
			fixture, backend, ratio*100, first, budget, failurePath)
	}
}

func newFixtureGame(fixture string) (runtime.Game, error) {
	switch fixture {
	case "frame":
		return New()
	case "particles":
		return NewParticleGolden()
	default:
		return nil, fmt.Errorf("unknown fixture %q", fixture)
	}
}

// rasterFrame drives the game through frames Driver.Frame calls on a single
// raster.Renderer (particle fixtures need the same renderer across calls:
// emitter state lives there) and returns the last rendered image.
func rasterFrame(t *testing.T, game runtime.Game, frames int) *image.RGBA {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	driver := runtime.NewDriver(game, log)
	cfg := driver.Config()

	// A texture/font that fails to decode is left nil in the registry
	// (rendered as the missing-texture placeholder) rather than aborting the
	// run -- mirrors platform/raster/run.go. The particles fixture relies on
	// this: it deliberately declares one corrupt texture to characterize the
	// missing-texture placeholder across backends.
	textures, err := pixel.BuildRegistry(cfg.Assets)
	if err != nil {
		t.Logf("build texture registry (non-fatal, missing-texture placeholder expected): %v", err)
	}
	fonts, err := typeface.BuildRegistry(cfg.Assets)
	if err != nil {
		t.Logf("build font registry (non-fatal): %v", err)
	}

	renderer := &raster.Renderer{
		Width:    320,
		Height:   240,
		Camera:   cfg.Camera,
		Clear:    cfg.Clear,
		Textures: textures,
		Fonts:    fonts,
		Assets:   cfg.Assets,
	}

	var img *image.RGBA
	for i := 0; i < frames; i++ {
		frame := driver.Frame(time.Duration(0))
		img = renderer.Render(frame.Commands)
	}
	return img
}

func loadGolden(t *testing.T, backend, name string) *image.RGBA {
	t.Helper()

	path := filepath.Join("..", "..", "platform", "wasm", "testdata", "golden", backend, name+".png")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode golden %s: %v", path, err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 320 || bounds.Dy() != 240 {
		t.Fatalf("golden %s is %dx%d, want 320x240 (Playwright viewport/DPR changed; this plan needs revision, not a resize)",
			path, bounds.Dx(), bounds.Dy())
	}

	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.Set(x, y, img.At(x, y))
		}
	}
	return out
}

// diffRatio mirrors platform/raster/golden_helpers_test.go's countPixelDiff
// (per-channel tolerance), returning the fraction of pixels that differ
// instead of a raw count, since the two backends being compared are never
// pixel-identical (AA, gradient quantization, MSAA).
func diffRatio(a, b *image.RGBA, channelTol uint8) (ratio float64, first image.Point) {
	if a.Bounds() != b.Bounds() {
		return 1, a.Bounds().Min
	}

	bounds := a.Bounds()
	total := bounds.Dx() * bounds.Dy()
	diffCount := 0
	first = image.Point{X: -1, Y: -1}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if colorsWithinTolerance(a.RGBAAt(x, y), b.RGBAAt(x, y), channelTol) {
				continue
			}
			if diffCount == 0 {
				first = image.Point{X: x, Y: y}
			}
			diffCount++
		}
	}
	return float64(diffCount) / float64(total), first
}

func colorsWithinTolerance(a, b color.RGBA, tolerance uint8) bool {
	return channelWithinTolerance(a.R, b.R, tolerance) &&
		channelWithinTolerance(a.G, b.G, tolerance) &&
		channelWithinTolerance(a.B, b.B, tolerance) &&
		channelWithinTolerance(a.A, b.A, tolerance)
}

func channelWithinTolerance(a, b, tolerance uint8) bool {
	if a > b {
		return a-b <= tolerance
	}
	return b-a <= tolerance
}

func writePNG(t *testing.T, path string, img *image.RGBA) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}
