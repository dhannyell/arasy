//go:build (windows || linux || darwin) && (amd64 || arm64)

package wgpurender

import (
	"bytes"
	"image"
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/render/slug"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/webgpu/wgpu"
	"golang.org/x/image/font/gofont/goregular"
)

// The raster is the oracle: the GPU path must draw each Slug case as the raster
// does, through the same frame code as a window. The cases share one renderer, so
// they also upload new glyphs to a Pack that has old glyphs.
func TestSlugTextMatchesRaster(t *testing.T) {
	const width, height = 320, 180 // 320*4 meets the row alignment of a texture copy
	manifest, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"fonts":[{"name":"go","file":"go.ttf"}],"dynamic":[{"name":"label","w":160,"h":48}]}`)},
		"go.ttf":        {Data: goregular.TTF},
	})
	if err != nil {
		t.Fatal(err)
	}
	font, label := manifest.Font("go"), manifest.Texture("label")
	screen := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: width, Y: height})
	camera := render.Camera{World: screen, Viewport: screen}
	labelRect := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 160, Y: 48})

	text := func(s string, x, y, size float64, color render.Color) render.DrawText {
		return render.DrawText{
			Position: math2d.Vec2{X: x, Y: y}, Text: s, Size: size, Font: font, Color: color, Strategy: render.GlyphSlug,
		}
	}
	background := render.DrawRect{Rect: screen, Fill: render.RGBA(0.12, 0.2, 0.35, 1)}
	white := render.RGBA(1, 1, 1, 1)

	additive := text("Additive", 20, 100, 40, render.RGBA(0.8, 0.5, 0.2, 0.8))
	additive.BlendMode = render.BlendAdditive
	clipped := text("Clipped text", 20, 100, 40, white)
	clipped.Clip = math2d.NewRect(math2d.Vec2{X: 60, Y: 70}, math2d.Vec2{X: 120, Y: 25})
	rotated := text("Rotated", 160, 90, 32, white)
	rotated.Rotation, rotated.Align, rotated.Baseline = math.Pi/7, render.AlignCenter, render.BaselineMiddle
	transformed := text("Sheared", 40, 100, 32, white)
	transformed.Transform = math2d.Affine{A: 1, B: 0.2, C: -0.3, D: 1, E: 30, F: -10}
	targetText := text("Target", 8, 36, 32, render.RGBA(1, 0.8, 0.3, 0.9))

	cases := map[string]runtime.Frame{
		"alpha":     {Commands: []render.Command{background, text("Half alpha", 20, 100, 40, render.RGBA(1, 1, 1, 0.5))}},
		"additive":  {Commands: []render.Command{background, additive}},
		"clip":      {Commands: []render.Command{background, clipped}},
		"rotation":  {Commands: []render.Command{background, rotated}},
		"transform": {Commands: []render.Command{background, transformed}},
		"target": {
			Passes: []render.Pass{{
				Target:   label,
				Camera:   render.Camera{World: labelRect, Viewport: labelRect},
				Commands: []render.Command{targetText},
			}},
			Commands: []render.Command{background, render.DrawImage{
				Texture: label, Dest: math2d.NewRect(math2d.Vec2{X: 40, Y: 60}, math2d.Vec2{X: 160, Y: 48}),
			}},
		},
		"post": {Commands: []render.Command{
			background,
			text("Graded", 20, 100, 40, white),
			render.PostChain{Effects: []render.PostEffect{render.ColorGrade{Brightness: 0.05, Contrast: 0.1, Tint: white}}},
		}},
	}

	textures, err := pixel.BuildRegistry(manifest)
	if err != nil {
		t.Fatal(err)
	}
	fonts, err := typeface.BuildRegistry(manifest)
	if err != nil {
		t.Fatal(err)
	}
	oracle := raster.Renderer{
		Width: width, Height: height, Camera: camera, Clear: render.RGBA(0, 0, 0, 1),
		Textures: textures, Fonts: fonts, Assets: manifest, GlyphOutlines: slug.Outlines,
	}
	modes := map[string]render.MSAAMode{"msaa-off": render.MSAAOff, "msaa-4x": render.MSAA4x}
	for modeName, mode := range modes {
		renderer := newHeadlessRenderer(t, manifest, mode, width, height)
		for name, frame := range cases {
			t.Run(modeName+"/"+name, func(t *testing.T) {
				frame.Camera = camera
				want := oracle.NewTarget()
				oracle.RenderPassesInto(want, frame.Passes, render.Pass{Camera: camera, Commands: frame.Commands})
				got := renderer.renderOffscreen(t, frame, width, height)
				compareSlugFrames(t, renderer, got, want)
			})
		}
	}
}

// compareSlugFrames fails if got and want differ by more than sampling noise.
func compareSlugFrames(t *testing.T, renderer *Renderer, got, want *image.RGBA) {
	t.Helper()
	slugBatch := false
	renderer.forEachTextBatch(func(batch *gpuTextBatch) {
		for _, rg := range batch.ranges {
			slugBatch = slugBatch || rg.slug
		}
	})
	if len(renderer.tess.SlugInstanceStream()) == 0 && !slugBatch {
		t.Fatal("the frame has no Slug instance")
	}
	if len(renderer.textAtlases) != 0 {
		t.Fatal("a Slug command built a glyph atlas")
	}

	const threshold = 8 // of 255: MSAA and rounding move some edge pixels
	var total, far, ink int
	for i := 0; i < len(got.Pix); i += 4 {
		worst := 0
		for c := range 3 {
			d := int(got.Pix[i+c]) - int(want.Pix[i+c])
			total += max(d, -d)
			worst = max(worst, d, -d)
		}
		if worst > threshold {
			far++
		}
		if int(want.Pix[i])-int(want.Pix[0]) > threshold {
			ink++
		}
	}
	mean := float64(total) / float64(len(got.Pix)/4*3)
	t.Logf("mean difference %.3f of 255, %d pixels over %d, %d pixels of ink", mean, far, threshold, ink)
	// A frame with only the background also matches, so the frame must have ink.
	if ink < 100 {
		t.Errorf("the raster drew only %d pixels of ink", ink)
	}
	if mean > 0.1 || far*20 > ink {
		t.Errorf("mean difference %.3f, %d pixels over %d, %d pixels of ink", mean, far, threshold, ink)
	}
}

// newHeadlessRenderer returns a renderer on a device without a surface. It skips
// the test without a hardware adapter: the tolerance of the comparison does not
// hold on a software adapter, and the CI runners have only that kind.
func newHeadlessRenderer(t *testing.T, manifest asset.Manifest, msaa render.MSAAMode, width, height int) *Renderer {
	t.Helper()
	d := &Device{Instance: wgpu.CreateInstance(nil)}
	var err error
	d.Adapter, err = d.Instance.RequestAdapter(&wgpu.RequestAdapterOptions{PowerPreference: wgpu.PowerPreferenceHighPerformance})
	if err != nil {
		d.Destroy()
		t.Skipf("no WebGPU adapter: %v", err)
	}
	if info := d.Adapter.GetInfo(); info.AdapterType == wgpu.AdapterTypeCPU {
		d.Destroy()
		t.Skipf("software WebGPU adapter: %s", info.Description)
	}
	if d.Device, err = d.Adapter.RequestDevice(nil); err != nil {
		d.Destroy()
		t.Skipf("no WebGPU device: %v", err)
	}
	d.Queue = d.Device.GetQueue()
	d.Config = &wgpu.SurfaceConfiguration{
		Format: wgpu.TextureFormatRGBA8Unorm, Width: uint32(width), Height: uint32(height),
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	renderer, err := New(d, render.RGBA(0, 0, 0, 1), manifest, msaa, 0, slug.Outlines, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(renderer.Close)
	return renderer
}

// renderOffscreen draws frame as a window does, and reads the result back.
func (r *Renderer) renderOffscreen(t *testing.T, frame runtime.Frame, width, height int) *image.RGBA {
	t.Helper()
	size := wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}
	texture, err := r.device.Device.TryCreateTexture(&wgpu.TextureDescriptor{
		Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageCopySrc,
		Dimension:     wgpu.TextureDimension2D,
		Size:          size,
		Format:        wgpu.TextureFormatRGBA8Unorm,
		MipLevelCount: 1,
		SampleCount:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer texture.Release()
	view, err := texture.TryCreateView(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer view.Release()

	prepared, ok := r.prepareFrame(frame, width, height)
	if !ok {
		t.Fatalf("prepare frame: %v", r.err)
	}
	if err := r.encodeFrame(prepared, view, width, height); err != nil {
		t.Fatal(err)
	}
	if r.postDisabled {
		t.Fatal("the renderer disabled post-processing")
	}
	r.slug.endFrame()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	readback, err := r.newBuffer("test readback", len(img.Pix), wgpu.BufferUsageCopyDst|wgpu.BufferUsageMapRead)
	if err != nil {
		t.Fatal(err)
	}
	defer readback.Release()
	encoder, err := r.device.Device.TryCreateCommandEncoder(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Release()
	if err := encoder.TryCopyTextureToBuffer(
		&wgpu.TexelCopyTextureInfo{Texture: texture, Aspect: wgpu.TextureAspectAll},
		&wgpu.TexelCopyBufferInfo{Buffer: readback, Layout: wgpu.TexelCopyBufferLayout{
			BytesPerRow: uint32(img.Stride), RowsPerImage: uint32(height),
		}},
		&size,
	); err != nil {
		t.Fatal(err)
	}
	commands, err := encoder.TryFinish(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer commands.Release()
	r.device.Queue.Submit(commands)

	status := wgpu.MapAsyncStatus(0)
	if err := readback.TryMapAsync(wgpu.MapModeRead, 0, uint64(len(img.Pix)), func(s wgpu.MapAsyncStatus) { status = s }); err != nil {
		t.Fatal(err)
	}
	r.device.Device.Poll(true, nil)
	if status != wgpu.MapAsyncStatusSuccess {
		t.Fatalf("map readback: status %v", status)
	}
	copy(img.Pix, readback.GetMappedRange(0, uint(len(img.Pix))))
	readback.Unmap()
	return img
}

func TestResolveFramePassesSkipsUnavailableTargetOnce(t *testing.T) {
	manifest, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"dynamic":[{"name":"target","w":4,"h":4}]}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	renderer := Renderer{
		manifest: manifest,
		log:      slog.New(slog.NewTextHandler(&logs, nil)),
	}
	frame := runtime.Frame{Passes: []render.Pass{{Target: manifest.Texture("target")}}}

	for range 2 {
		passes := renderer.resolveFramePasses(frame, 8, 8)
		if len(passes) != 1 || passes[0].Target != 0 {
			t.Fatalf("resolved passes = %#v, want base pass only", passes)
		}
	}
	if got := strings.Count(logs.String(), "pass target activation failed"); got != 1 {
		t.Fatalf("unavailable-target warnings = %d, want 1; logs: %s", got, logs.String())
	}
}

func TestTextureSampleViewUsesMipZeroAfterRTT(t *testing.T) {
	full := &wgpu.TextureView{}
	target := &wgpu.TextureView{}
	texture := gpuTexture{view: full, targetView: target, arrayLayer: -1}
	if got := textureSampleView(texture); got != full {
		t.Fatalf("static sample view = %p, want full view %p", got, full)
	}
	texture.rtt = true
	if got := textureSampleView(texture); got != target {
		t.Fatalf("RTT sample view = %p, want mip-zero view %p", got, target)
	}
}

func TestCPUFrameTimingUsesSubmitBoundaryAndExcludesAcquireWait(t *testing.T) {
	base := time.Unix(100, 0)
	tickEnd := base.Add(2 * time.Millisecond)
	inputAt := base.Add(-3 * time.Millisecond)
	timing := CPUFrameTiming{
		submittedAt: base.Add(8 * time.Millisecond),
		cpuEndAt:    base.Add(10 * time.Millisecond),
		acquireWait: 4 * time.Millisecond,
		Valid:       true,
	}

	if got, want := timing.RenderDurationSince(tickEnd), 4*time.Millisecond; got != want {
		t.Fatalf("CPU render duration = %v, want %v", got, want)
	}
	latency, ok := timing.InputToSubmit(inputAt)
	if !ok {
		t.Fatal("input-to-submit timing unavailable")
	}
	if want := 11 * time.Millisecond; latency != want {
		t.Fatalf("input-to-submit = %v, want %v", latency, want)
	}
}

func TestDecideSurfaceStatus(t *testing.T) {
	tests := []struct {
		name   string
		status wgpu.SurfaceGetCurrentTextureStatus
		want   surfaceAction
	}{
		{name: "optimal", status: wgpu.SurfaceGetCurrentTextureStatusSuccessOptimal, want: actionRender},
		{name: "suboptimal", status: wgpu.SurfaceGetCurrentTextureStatusSuccessSuboptimal, want: actionRender},
		{name: "timeout", status: wgpu.SurfaceGetCurrentTextureStatusTimeout, want: actionSkip},
		{name: "occluded", status: wgpu.SurfaceGetCurrentTextureStatusOccluded, want: actionSkip},
		{name: "outdated", status: wgpu.SurfaceGetCurrentTextureStatusOutdated, want: actionReconfigure},
		{name: "lost", status: wgpu.SurfaceGetCurrentTextureStatusLost, want: actionReconfigure},
		{name: "error", status: wgpu.SurfaceGetCurrentTextureStatusError, want: actionFatal},
		{name: "unknown", status: wgpu.SurfaceGetCurrentTextureStatus(99), want: actionFatal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := decideSurfaceStatus(test.status); got != test.want {
				t.Fatalf("decideSurfaceStatus(%v) = %v, want %v", test.status, got, test.want)
			}
		})
	}
}

// A Pack reset moves the glyphs, so a retained batch must pack its records again.
func TestSlugTextBatchMatchesRaster(t *testing.T) {
	const width, height = 320, 180 // 320*4 meets the row alignment of a texture copy
	manifest, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"fonts":[{"name":"go","file":"go.ttf"}]}`)},
		"go.ttf":        {Data: goregular.TTF},
	})
	if err != nil {
		t.Fatal(err)
	}
	font := manifest.Font("go")
	screen := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: width, Y: height})
	camera := render.Camera{World: screen, Viewport: screen}
	background := render.DrawRect{Rect: screen, Fill: render.RGBA(0.12, 0.2, 0.35, 1)}
	white := render.RGBA(1, 1, 1, 1)
	label := func(s string, y float64) render.TextLabel {
		return render.TextLabel{
			Position: math2d.Vec2{X: 20, Y: y}, Text: s, Size: 32, Font: font, Color: white, Strategy: render.GlyphSlug,
		}
	}
	const id render.TextBatchID = 1
	batch := render.DrawTextBatch{Batch: id}
	clippedBatch := batch
	clippedBatch.Clip = math2d.NewRect(math2d.Vec2{X: 40, Y: 30}, math2d.Vec2{X: 120, Y: 60})
	streaming := render.DrawText{
		Position: math2d.Vec2{X: 20, Y: 160}, Text: "XYZ qwjk", Size: 24, Font: font, Color: white, Strategy: render.GlyphSlug,
	}

	textures, err := pixel.BuildRegistry(manifest)
	if err != nil {
		t.Fatal(err)
	}
	fonts, err := typeface.BuildRegistry(manifest)
	if err != nil {
		t.Fatal(err)
	}
	oracle := raster.Renderer{
		Width: width, Height: height, Camera: camera, Clear: render.RGBA(0, 0, 0, 1),
		Textures: textures, Fonts: fonts, Assets: manifest, GlyphOutlines: slug.Outlines,
	}
	renderer := newHeadlessRenderer(t, manifest, render.MSAAOff, width, height)

	first := []render.TextLabel{label("Retained", 50), label("batch", 100)}
	steps := []struct {
		name     string
		labels   []render.TextLabel
		reset    bool
		commands []render.Command
	}{
		{name: "first", labels: first, commands: []render.Command{background, batch}},
		{name: "clip", commands: []render.Command{background, clippedBatch}},
		// The streaming text takes the first positions of the empty Pack.
		{name: "reset", reset: true, commands: []render.Command{background, streaming, batch}},
		{name: "update", labels: []render.TextLabel{label("Changed", 70)}, commands: []render.Command{background, batch}},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			if step.labels != nil {
				oracle.ReplaceTextBatch(id, step.labels)
				renderer.UpdateResources(runtime.FrameUpdates{
					TextBatches: []runtime.TextBatchUpdate{{ID: id, Op: runtime.BatchReplace, Labels: step.labels}},
				})
			}
			if step.reset {
				renderer.slug.engine.Pack().Reset()
			}
			frame := runtime.Frame{Camera: camera, Commands: step.commands}
			want := oracle.NewTarget()
			oracle.RenderPassesInto(want, nil, render.Pass{Camera: camera, Commands: frame.Commands})
			got := renderer.renderOffscreen(t, frame, width, height)
			compareSlugFrames(t, renderer, got, want)
		})
	}
}

func TestMipLevelCount(t *testing.T) {
	for _, tc := range []struct{ w, h, want int }{{1, 1, 1}, {8, 8, 4}, {3, 7, 3}} {
		if got := mipLevelCount(tc.w, tc.h); got != tc.want {
			t.Errorf("mipLevelCount(%d, %d) = %d, want %d", tc.w, tc.h, got, tc.want)
		}
	}
}
