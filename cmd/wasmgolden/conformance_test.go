//go:build !js

package wasmgolden

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// These tests compare the raster reference with the repository browser
// goldens. A difference above the budget reports the fixture and backend.
const (
	frameChannelTol           = 32
	frameCanvas2DBudget       = 0.2454
	frameWebGPUBudget         = 0.2002
	particlesChannelTol       = 32
	particlesCanvas2DBudget   = 0.1520
	particlesWebGPUBudget     = 0.0754
	viewChannelTol            = 32
	viewCanvas2DBudget        = 0.0186
	viewWebGPUBudget          = 0.0509
	parallaxChannelTol        = 32
	parallaxCanvas2DBudget    = 0.0200
	parallaxWebGPUBudget      = 0.0150
	flipChannelTol            = 32
	flipCanvas2DBudget        = 0.0200
	flipWebGPUBudget          = 0.0150
	passesChannelTol          = 32
	passesCanvas2DBudget      = 0.0200
	passesWebGPUBudget        = 0.0150
	passesWebGPUMSAAOffBudget = 0.0150
	postWebGPUBudget          = 0.075
	rttWebGPUBudget           = 0.050
)

func TestConformanceFrameCanvas2D(t *testing.T) {
	assertConformance(t, "frame", "canvas2d", 1, frameChannelTol, frameCanvas2DBudget)
}

func TestConformanceFrameWebGPU(t *testing.T) {
	assertConformance(t, "frame", "webgpu", 1, frameChannelTol, frameWebGPUBudget)
}

func TestConformanceParticlesCanvas2D(t *testing.T) {
	assertConformance(t, "particles", "canvas2d", 3, particlesChannelTol, particlesCanvas2DBudget)
}

func TestConformanceParticlesWebGPU(t *testing.T) {
	assertConformance(t, "particles", "webgpu", 3, particlesChannelTol, particlesWebGPUBudget)
}

func TestConformancePostWebGPU(t *testing.T) {
	// Canvas2D intentionally uses documented approximations for this fixture;
	// the tight conformance comparison is reserved for the GPU mirror.
	assertConformance(t, "post", "webgpu", 2, frameChannelTol, postWebGPUBudget)
}

func TestConformanceViewCanvas2D(t *testing.T) {
	assertConformance(t, "view", "canvas2d", 3, viewChannelTol, viewCanvas2DBudget)
}

func TestConformanceViewWebGPU(t *testing.T) {
	assertConformance(t, "view", "webgpu", 3, viewChannelTol, viewWebGPUBudget)
}

func TestConformanceParallaxCanvas2D(t *testing.T) {
	assertConformance(t, "parallax", "canvas2d", 1, parallaxChannelTol, parallaxCanvas2DBudget)
}

func TestConformanceParallaxWebGPU(t *testing.T) {
	assertConformance(t, "parallax", "webgpu", 1, parallaxChannelTol, parallaxWebGPUBudget)
}

func TestConformanceFlipCanvas2D(t *testing.T) {
	assertConformance(t, "flip", "canvas2d", 1, flipChannelTol, flipCanvas2DBudget)
}

func TestConformanceFlipWebGPU(t *testing.T) {
	assertConformance(t, "flip", "webgpu", 1, flipChannelTol, flipWebGPUBudget)
}

func TestConformancePassesCanvas2D(t *testing.T) {
	assertConformance(t, "passes", "canvas2d", 1, passesChannelTol, passesCanvas2DBudget)
}

func TestConformancePassesWebGPU(t *testing.T) {
	assertConformance(t, "passes", "webgpu", 1, passesChannelTol, passesWebGPUBudget)
}

func TestConformancePassesWebGPUMSAAOff(t *testing.T) {
	assertConformance(t, "passes-msaa-off", "webgpu", 1, passesChannelTol, passesWebGPUMSAAOffBudget)
}

func TestConformanceRTTWebGPU(t *testing.T) {
	assertConformance(t, "rtt", "webgpu", 1, frameChannelTol, rttWebGPUBudget)
}

func TestConformanceRTTArrayWebGPU(t *testing.T) {
	assertConformance(t, "rtt-array", "webgpu", 1, frameChannelTol, rttWebGPUBudget)
}

func TestRTTFixturesShareRasterOracle(t *testing.T) {
	individual, err := NewRTTGolden()
	if err != nil {
		t.Fatal(err)
	}
	array, err := NewRTTArrayGolden()
	if err != nil {
		t.Fatal(err)
	}

	individualFrame := rasterFrame(t, individual, 1)
	arrayFrame := rasterFrame(t, array, 1)
	if !bytes.Equal(individualFrame.Pix, arrayFrame.Pix) {
		t.Fatal("individual and sprite-array RTT fixtures have different raster output")
	}
	colors := make(map[color.RGBA]struct{})
	for y := 0; y < individualFrame.Bounds().Dy(); y += 8 {
		for x := 0; x < individualFrame.Bounds().Dx(); x += 8 {
			colors[individualFrame.RGBAAt(x, y)] = struct{}{}
		}
	}
	if len(colors) < 8 {
		t.Fatalf("RTT fixture produced only %d sampled colors, want a non-empty conformance scene", len(colors))
	}
}

func assertConformance(t *testing.T, fixture, backend string, frames int, channelTol uint8, budget float64) {
	t.Helper()

	game, err := newFixtureGame(fixture)
	if err != nil {
		t.Fatalf("build %s fixture: %v", fixture, err)
	}
	rasterImg := rasterFrame(t, game, frames)
	golden := loadGolden(t, backend, fixture)

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
	case "post":
		return NewPostGolden()
	case "view":
		return NewViewConformanceGolden()
	case "parallax":
		return NewParallaxConformanceGolden()
	case "flip":
		return NewFlipConformanceGolden()
	case "passes", "passes-msaa-off":
		return NewCameraPassesGolden()
	case "rtt":
		return NewRTTGolden()
	case "rtt-array":
		return NewRTTArrayGolden()
	default:
		return nil, fmt.Errorf("unknown fixture %q", fixture)
	}
}

// rasterFrame renders a fixture for the requested number of frames. It keeps
// one renderer so particle state survives between frames.
func rasterFrame(t *testing.T, game runtime.Game, frames int) *image.RGBA {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	driver := runtime.NewDriver(game, log)
	cfg := driver.Config()

	// A decode failure leaves that entry nil (missing-texture placeholder)
	// instead of aborting; the particles fixture relies on this for its
	// deliberately corrupt texture.
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
		applyRasterUpdates(renderer, driver.ConsumeFrameUpdates())
		img = renderer.NewTarget()
		renderer.RenderPassesInto(img, frame.Passes, render.Pass{Camera: frame.Camera, Commands: frame.Commands})
	}
	return img
}

func applyRasterUpdates(renderer *raster.Renderer, updates runtime.FrameUpdates) {
	for _, update := range updates.Textures {
		if int(update.ID) < len(renderer.Textures) && renderer.Textures[update.ID] != nil && update.Image != nil {
			draw.Draw(renderer.Textures[update.ID], renderer.Textures[update.ID].Bounds(), update.Image, image.Point{}, draw.Src)
		}
	}
	for _, update := range updates.SpriteBatches {
		if update.Op == runtime.BatchDelete {
			renderer.DeleteSpriteBatch(update.ID)
		} else {
			renderer.ReplaceSpriteBatch(update.ID, update.Instances)
		}
	}
	for _, update := range updates.TextBatches {
		if update.Op == runtime.BatchDelete {
			renderer.DeleteTextBatch(update.ID)
		} else {
			renderer.ReplaceTextBatch(update.ID, update.Labels)
		}
	}
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

// diffRatio returns the fraction of pixels outside the channel tolerance.
// The raster and browser paths can differ because of AA, gradients, and MSAA.
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
