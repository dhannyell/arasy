//go:build js && wasm

package wasm

import (
	"fmt"
	"log/slog"
	"syscall/js"
	"time"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/platform/internal/wgpurender"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/webgpu/wgpu"
)

// sharedRenderer connects the shared renderer to the browser host.
type sharedRenderer struct {
	*wgpurender.Renderer
	canvas        js.Value
	width, height int
	readySignaled bool
}

func newSharedRenderer(
	canvas js.Value,
	clear render.Color,
	manifest asset.Manifest,
	wantGPUTiming bool,
	particleScale float64,
	msaa render.MSAAMode,
	log *slog.Logger,
) (*sharedRenderer, error) {
	d, err := openSharedDevice(canvas, wantGPUTiming)
	if err != nil {
		return nil, err
	}
	renderer, err := wgpurender.New(d, clear, manifest, msaa, particleScale, log)
	if err != nil {
		return nil, err
	}
	renderer.SetGPUTiming(wantGPUTiming)
	renderer.SetCPUTiming(true)
	return &sharedRenderer{
		Renderer: renderer,
		canvas:   canvas,
		width:    int(d.Config.Width),
		height:   int(d.Config.Height),
	}, nil
}

func openSharedDevice(canvas js.Value, wantGPUTiming bool) (*wgpurender.Device, error) {
	instance := wgpu.CreateInstance(nil)
	if instance == nil {
		return nil, errWebGPUUnavailable
	}
	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{})
	if err != nil {
		return nil, fmt.Errorf("request webgpu adapter: %w", err)
	}
	desc := &wgpu.DeviceDescriptor{}
	timestamps := wantGPUTiming && adapter.HasFeature(wgpu.FeatureNameTimestampQuery)
	if timestamps {
		desc.RequiredFeatures = []wgpu.FeatureName{wgpu.FeatureNameTimestampQuery}
	}
	device, err := adapter.RequestDevice(desc)
	if err != nil {
		return nil, fmt.Errorf("request webgpu device: %w", err)
	}
	format := wgpu.TextureFormatBGRA8Unorm
	if js.Global().Get("navigator").Get("gpu").Call("getPreferredCanvasFormat").String() == "rgba8unorm" {
		format = wgpu.TextureFormatRGBA8Unorm
	}
	d := &wgpurender.Device{
		Instance: instance,
		Surface:  instance.CreateSurface(&wgpu.SurfaceDescriptor{Canvas: canvas}),
		Device:   device,
		Queue:    device.GetQueue(),
		Config: &wgpu.SurfaceConfiguration{
			Usage:     wgpu.TextureUsageRenderAttachment,
			Format:    format,
			AlphaMode: wgpu.CompositeAlphaModeOpaque,
		},
		TimestampQuery: timestamps,
	}
	d.Configure(canvas.Get("width").Int(), canvas.Get("height").Int())
	return d, nil
}

// Render follows the canvas size, because the browser host resizes the canvas
// and does not tell the renderer.
func (r *sharedRenderer) Render(frame runtime.Frame) {
	width, height := r.canvas.Get("width").Int(), r.canvas.Get("height").Int()
	if width != r.width || height != r.height {
		r.Resize(width, height)
		r.width, r.height = width, height
	}
	r.Renderer.Render(frame)
	if !r.readySignaled && r.HasPresentedFrame() {
		r.readySignaled = true
		signalCanvasReady(r.canvas, "wgpu", func() {})
	}
}

func (r *sharedRenderer) retainedBatchMetrics() (RetainedBatchMetrics, bool) {
	return RetainedBatchMetrics(r.RetainedBatchMetrics()), true
}

func (r *sharedRenderer) renderCPUTimings() (tessTime, upload, encode time.Duration, ok bool) {
	return r.RenderCPUTimings()
}

func (r *sharedRenderer) particleGPUTimings() (prepare, simulate, render time.Duration, ok bool) {
	return r.ParticleGPUTimings()
}

func (r *sharedRenderer) particleGPUBufferBytes() uint64 {
	bytes, _ := r.ParticleGPUBufferBytes()
	return bytes
}
