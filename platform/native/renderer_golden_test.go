//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"bytes"
	"flag"
	"image"
	"image/draw"
	"image/png"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhannyell/arasy/cmd/wasmgolden"
	"github.com/dhannyell/arasy/platform/internal/wgpurender"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/webgpu/wgpu"
)

var updateGoldens = flag.Bool("update", false, "rewrite the native renderer goldens")

const (
	goldenWidth  = 320
	goldenHeight = 240
	// These limits are the same as the Playwright limits for browser goldens.
	goldenChannelTol = 51
	goldenBudget     = 0.005
)

// Goldens match only the GPU that recorded them, and CI has a software adapter.
// So this test runs only when a developer asks for it.
func TestRendererMatchesGoldens(t *testing.T) {
	if os.Getenv("ARASY_GPU_GOLDEN") == "" {
		t.Skip("set ARASY_GPU_GOLDEN=1 on the GPU that recorded the goldens")
	}
	cases := []struct {
		name, fixture string
		frames        int
		msaa          render.MSAAMode
	}{
		{name: "frame", frames: 1},
		{name: "particles", fixture: "particles", frames: 3},
		{name: "sprite-array", fixture: "sprite-array", frames: 1},
		{name: "retained-batches", fixture: "retained-batches", frames: 2},
		{name: "view", fixture: "view", frames: 3},
		{name: "parallax", fixture: "parallax", frames: 1},
		{name: "flip", fixture: "flip", frames: 1},
		{name: "passes", fixture: "passes", frames: 1},
		{name: "passes-msaa-off", fixture: "passes", frames: 1, msaa: render.MSAAOff},
		{name: "arena-growth", fixture: "arena-growth", frames: 1},
		{name: "post", fixture: "post", frames: 2},
		{name: "post-stylize", fixture: "post-stylize", frames: 1},
		{name: "rtt", fixture: "rtt", frames: 1},
		{name: "rtt-array", fixture: "rtt-array", frames: 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			game, err := wasmgolden.NewFixture(c.fixture)
			if err != nil {
				t.Fatal(err)
			}
			compareGolden(t, c.name, renderOffscreen(t, game, c.frames, c.msaa))
		})
	}
}

func renderOffscreen(t *testing.T, game runtime.Game, frames int, msaa render.MSAAMode) *image.RGBA {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	driver := runtime.NewDriver(game, log)
	cfg := driver.Config()

	d := &wgpurender.Device{Instance: wgpu.CreateInstance(nil)}
	if err := openDevice(d, false, log); err != nil {
		d.Destroy()
		t.Skipf("no GPU adapter: %v", err)
	}
	d.Config = &wgpu.SurfaceConfiguration{
		Format: wgpu.TextureFormatBGRA8Unorm,
		Width:  goldenWidth,
		Height: goldenHeight,
	}
	texture, err := d.Device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "Arasy golden target",
		Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageCopySrc,
		Dimension:     wgpu.TextureDimension2D,
		Size:          wgpu.Extent3D{Width: goldenWidth, Height: goldenHeight, DepthOrArrayLayers: 1},
		Format:        d.Config.Format,
		MipLevelCount: 1,
		SampleCount:   1,
	})
	if err != nil {
		d.Destroy()
		t.Fatal(err)
	}
	defer texture.Release()
	view, err := texture.TryCreateView(nil)
	if err != nil {
		d.Destroy()
		t.Fatal(err)
	}
	defer view.Release()
	d.OffscreenView = view

	renderer, err := wgpurender.New(d, cfg.Clear, cfg.Assets, msaa, cfg.ParticleOffscreenScale, nil, log)
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()

	for range frames {
		driver.RenderFrame(0, renderer)
		if err := renderer.Err(); err != nil {
			t.Fatal(err)
		}
	}
	return readTexture(t, d, texture)
}

// readTexture copies a BGRA8 texture to an image and blocks until the GPU ends.
func readTexture(t *testing.T, d *wgpurender.Device, texture *wgpu.Texture) *image.RGBA {
	t.Helper()
	const bytesPerRow = goldenWidth * 4 // 1280 is a multiple of the required 256.
	size := uint64(bytesPerRow * goldenHeight)
	buffer, err := d.Device.TryCreateBuffer(&wgpu.BufferDescriptor{
		Label: "Arasy golden readback",
		Usage: wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapRead,
		Size:  size,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer buffer.Release()

	encoder := d.Device.CreateCommandEncoder(nil)
	defer encoder.Release()
	encoder.CopyTextureToBuffer(
		&wgpu.TexelCopyTextureInfo{Texture: texture, Aspect: wgpu.TextureAspectAll},
		&wgpu.TexelCopyBufferInfo{
			Layout: wgpu.TexelCopyBufferLayout{BytesPerRow: bytesPerRow, RowsPerImage: goldenHeight},
			Buffer: buffer,
		},
		&wgpu.Extent3D{Width: goldenWidth, Height: goldenHeight, DepthOrArrayLayers: 1},
	)
	commands := encoder.Finish(nil)
	defer commands.Release()
	d.Queue.Submit(commands)

	status := wgpu.MapAsyncStatus(0)
	if err := buffer.TryMapAsync(wgpu.MapModeRead, 0, size, func(s wgpu.MapAsyncStatus) { status = s }); err != nil {
		t.Fatal(err)
	}
	d.Device.Poll(true, nil)
	if status != wgpu.MapAsyncStatusSuccess {
		t.Fatalf("map readback buffer: %v", status)
	}
	defer buffer.Unmap()

	img := image.NewRGBA(image.Rect(0, 0, goldenWidth, goldenHeight))
	copy(img.Pix, buffer.GetMappedRange(0, uint(size)))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+2] = img.Pix[i+2], img.Pix[i]
	}
	return img
}

func compareGolden(t *testing.T, name string, got *image.RGBA) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".png")
	if *updateGoldens {
		writeGolden(t, path, got)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to record it)", err)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	want := image.NewRGBA(decoded.Bounds())
	draw.Draw(want, want.Bounds(), decoded, image.Point{}, draw.Src)
	differ := 0
	for i := 0; i < len(got.Pix); i += 4 {
		for c := range 4 {
			if delta := int(got.Pix[i+c]) - int(want.Pix[i+c]); delta > goldenChannelTol || delta < -goldenChannelTol {
				differ++
				break
			}
		}
	}
	ratio := float64(differ) / float64(goldenWidth*goldenHeight)
	if ratio > goldenBudget {
		failure := filepath.Join("testdata", "failures", name+".png")
		writeGolden(t, failure, got)
		t.Errorf("%s: %.3f%% of pixels differ, budget %.3f%%; render saved to %s", name, ratio*100, goldenBudget*100, failure)
	}
}

func writeGolden(t *testing.T, path string, img *image.RGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
