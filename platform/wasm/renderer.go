//go:build js && wasm

package wasm

import (
	"errors"
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

var errWebGPUUnavailable = errors.New("webgpu unavailable")

// browserRenderer connects wgpurender to the browser canvas.
type browserRenderer struct {
	*wgpurender.Renderer
	canvas        js.Value
	width, height int
	readySignaled bool
}

func newBrowserRenderer(
	canvas js.Value,
	clear render.Color,
	manifest asset.Manifest,
	wantGPUTiming bool,
	particleScale float64,
	msaa render.MSAAMode,
	log *slog.Logger,
) (*browserRenderer, error) {
	d, err := openBrowserDevice(canvas, wantGPUTiming)
	if err != nil {
		return nil, err
	}
	renderer, err := wgpurender.New(d, clear, manifest, msaa, particleScale, log)
	if err != nil {
		return nil, err
	}
	observeCanvasDeviceErrors(canvas, log)
	renderer.SetGPUTiming(wantGPUTiming)
	renderer.SetCPUTiming(true)
	return &browserRenderer{
		Renderer: renderer,
		canvas:   canvas,
		width:    int(d.Config.Width),
		height:   int(d.Config.Height),
	}, nil
}

func openBrowserDevice(canvas js.Value, wantGPUTiming bool) (*wgpurender.Device, error) {
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
	// A canvas that already has another context type returns null here.
	if !canvas.Call("getContext", "webgpu").Truthy() {
		return nil, errWebGPUUnavailable
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

// observeCanvasDeviceErrors logs device errors of the configured canvas. The
// binding does not expose the browser device, so this reads it from the context.
// A browser without getConfiguration still prints uncaptured errors itself. The
// callbacks stay for the life of the page, because the device can report an
// error until the page closes.
func observeCanvasDeviceErrors(canvas js.Value, log *slog.Logger) {
	context := canvas.Call("getContext", "webgpu")
	if context.Get("getConfiguration").IsUndefined() {
		return
	}
	device := context.Call("getConfiguration").Get("device")
	device.Call("addEventListener", "uncapturederror", js.FuncOf(func(this js.Value, args []js.Value) any {
		message := "unknown WebGPU error"
		if len(args) > 0 {
			err := args[0].Get("error")
			if !err.IsUndefined() && !err.IsNull() {
				message = err.Get("message").String()
			}
		}
		log.Error("webgpu uncaptured error", "error", message)
		return nil
	}))
	device.Get("lost").Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		reason := "unknown"
		message := ""
		if len(args) > 0 {
			reason = args[0].Get("reason").String()
			message = args[0].Get("message").String()
		}
		log.Error("webgpu device lost", "reason", reason, "message", message)
		return nil
	}))
}

// Render follows the canvas size, because the browser host resizes the canvas
// and does not tell the renderer.
func (r *browserRenderer) Render(frame runtime.Frame) {
	width, height := r.canvas.Get("width").Int(), r.canvas.Get("height").Int()
	if width != r.width || height != r.height {
		r.Resize(width, height)
		r.width, r.height = width, height
	}
	r.Renderer.Render(frame)
	if !r.readySignaled && r.HasPresentedFrame() {
		r.readySignaled = true
		signalCanvasReady(r.canvas)
	}
}

// signalCanvasReady marks the canvas after two animation frames, so the shell
// removes its boot overlay only when the browser has shown the frame.
func signalCanvasReady(canvas js.Value) {
	var firstFrame js.Func
	firstFrame = js.FuncOf(func(this js.Value, args []js.Value) any {
		firstFrame.Release()

		var secondFrame js.Func
		secondFrame = js.FuncOf(func(this js.Value, args []js.Value) any {
			secondFrame.Release()
			canvas.Call("setAttribute", "data-arasy-ready", "webgpu")
			return nil
		})
		js.Global().Call("requestAnimationFrame", secondFrame)
		return nil
	})
	js.Global().Call("requestAnimationFrame", firstFrame)
}

func (r *browserRenderer) retainedBatchMetrics() (RetainedBatchMetrics, bool) {
	return RetainedBatchMetrics(r.RetainedBatchMetrics()), true
}

func (r *browserRenderer) renderCPUTimings() (tessTime, upload, encode time.Duration, ok bool) {
	return r.RenderCPUTimings()
}

func (r *browserRenderer) particleGPUTimings() (prepare, simulate, render time.Duration, ok bool) {
	return r.ParticleGPUTimings()
}

func (r *browserRenderer) particleGPUBufferBytes() uint64 {
	bytes, _ := r.ParticleGPUBufferBytes()
	return bytes
}
