//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/internal/render/slugcore"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/webgpu/wgpu"
	"github.com/dhannyell/webgpu/wgpuglfw"
	"github.com/go-gl/glfw/v3.4/glfw"
)

func chooseSurfaceFormat(formats []wgpu.TextureFormat) (wgpu.TextureFormat, error) {
	for _, format := range formats {
		if format == wgpu.TextureFormatBGRA8Unorm {
			return format, nil
		}
	}
	for _, format := range formats {
		if format == wgpu.TextureFormatRGBA8Unorm {
			return format, nil
		}
	}
	return 0, fmt.Errorf("surface requires BGRA8Unorm or RGBA8Unorm")
}

func chooseAlphaMode(modes []wgpu.CompositeAlphaMode, log *slog.Logger) (wgpu.CompositeAlphaMode, error) {
	if len(modes) == 0 {
		return 0, fmt.Errorf("surface reports no alpha modes")
	}
	for _, mode := range modes {
		if mode == wgpu.CompositeAlphaModeOpaque {
			return mode, nil
		}
	}
	if log == nil {
		log = slog.Default()
	}
	log.Warn("surface does not support opaque alpha mode; using fallback", "alpha_mode", modes[0])
	return modes[0], nil
}

func containsPresentMode(modes []wgpu.PresentMode, want wgpu.PresentMode) bool {
	return slices.Contains(modes, want)
}

// choosePresentMode selects a present mode from the surface capabilities. FIFO
// is required because it is the only mode the WebGPU spec guarantees. When
// VSyncOff is requested, Immediate is preferred because it presents without
// waiting for vertical blank. Mailbox remains a lower-latency synchronized
// fallback; if neither is available, rendering continues with FIFO and a
// warning.
func choosePresentMode(modes []wgpu.PresentMode, vsync VSyncMode, log *slog.Logger) (wgpu.PresentMode, error) {
	if !containsPresentMode(modes, wgpu.PresentModeFifo) {
		return 0, fmt.Errorf("fifo present mode support is required")
	}
	if vsync == VSyncOn {
		return wgpu.PresentModeFifo, nil
	}
	switch {
	case containsPresentMode(modes, wgpu.PresentModeImmediate):
		return wgpu.PresentModeImmediate, nil
	case containsPresentMode(modes, wgpu.PresentModeMailbox):
		return wgpu.PresentModeMailbox, nil
	default:
		if log == nil {
			log = slog.Default()
		}
		log.Warn("vsync-off requested but no non-vsync present mode is available; using fifo")
		return wgpu.PresentModeFifo, nil
	}
}

// NewRenderer creates a native streaming renderer for window using vsync to
// select its surface presentation mode.
func NewRenderer(
	window *glfw.Window,
	clear render.Color,
	manifest asset.Manifest,
	vsync VSyncMode,
	msaa render.MSAAMode,
	particleScale float64,
	wantGPUTiming bool,
	outlines slugcore.Factory,
	log *slog.Logger,
) (*Renderer, error) {
	d, err := newDevice(window, vsync, wantGPUTiming, log)
	if err != nil {
		return nil, err
	}
	return newRenderer(d, clear, manifest, msaa, particleScale, outlines, log)
}

func newDevice(window *glfw.Window, vsync VSyncMode, wantGPUTiming bool, log *slog.Logger) (d *device, err error) {
	if log == nil {
		log = slog.Default()
	}
	d = &device{}
	defer func() {
		if err != nil {
			d.destroy()
			d = nil
		}
	}()

	d.instance = wgpu.CreateInstance(nil)
	if !d.instance.IsValid() {
		return d, fmt.Errorf("create WebGPU instance")
	}

	// CreateSurface bridges the GLFW window to wgpu-native.
	d.surface = d.instance.CreateSurface(wgpuglfw.GetSurfaceDescriptor(window))
	if !d.surface.IsValid() {
		return d, fmt.Errorf("create WebGPU surface")
	}

	d.adapter, err = d.instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		CompatibleSurface: d.surface,
		PowerPreference:   wgpu.PowerPreferenceHighPerformance,
	})
	if err != nil {
		return d, fmt.Errorf("request WebGPU adapter: %w", err)
	}
	info := d.adapter.GetInfo()
	d.vendorID = info.VendorId
	d.deviceID = info.DeviceId
	log.Info("native WebGPU adapter selected",
		"backend", info.BackendType,
		"vendor", info.Vendor,
		"device", info.Device,
	)

	// This binding panics when DeviceLostCallback crosses the cgo boundary, so
	// device loss is observed through surface and submission errors instead
	// (DeviceLostCallback is deliberately left unset below).
	//
	// The browser caps WebGPU limits well below what desktop hardware and
	// drivers actually support; requesting the adapter's own limits removes
	// that browser-shaped ceiling (e.g. max storage buffer binding size).
	// Still falls back to RequestDevice(nil) on any error, so the host is never left
	// without a device.
	//
	// Timestamp queries power the per-pass GPU telemetry, so the feature is
	// only requested when a metrics consumer asked for it; without one,
	// sampling every frame would be pure overhead for every game.
	//
	// Adapter-specific format features are asked for unconditionally: they cost
	// nothing to hold and they are what makes 2x and 8x MSAA reachable at all.
	// WebGPU guarantees only 1 and 4 samples per format, so without the feature
	// a game asking for 8x has its pipelines rejected by the driver.
	limits := d.adapter.GetLimits()
	desc := &wgpu.DeviceDescriptor{RequiredLimits: &limits}
	wantTimestamps := wantGPUTiming && d.adapter.HasFeature(wgpu.FeatureNameTimestampQuery)
	if wantTimestamps {
		desc.RequiredFeatures = append(desc.RequiredFeatures, wgpu.FeatureNameTimestampQuery)
	}
	wantFormatFeatures := d.adapter.HasFeature(wgpu.NativeFeatureTextureAdapterSpecificFormatFeatures)
	if wantFormatFeatures {
		desc.RequiredFeatures = append(desc.RequiredFeatures, wgpu.NativeFeatureTextureAdapterSpecificFormatFeatures)
	}
	d.device, err = d.adapter.RequestDevice(desc)
	requestedAdapterLimits := err == nil
	if err != nil {
		log.Warn("request WebGPU device with adapter limits failed, falling back to defaults", "err", err)
		wantTimestamps = false
		wantFormatFeatures = false
		d.device, err = d.adapter.RequestDevice(nil)
	}
	if err != nil {
		return d, fmt.Errorf("request WebGPU device: %w", err)
	}
	d.timestampQuery = wantTimestamps
	d.formatFeatures = wantFormatFeatures
	if requestedAdapterLimits {
		log.Info("native WebGPU device limits requested",
			"max_storage_buffer_binding_size", limits.MaxStorageBufferBindingSize,
			"max_buffer_size", limits.MaxBufferSize,
			"max_compute_workgroups_per_dimension", limits.MaxComputeWorkgroupsPerDimension,
		)
	}

	d.queue = d.device.GetQueue()

	caps := d.surface.GetCapabilities(d.adapter)
	format, err := chooseSurfaceFormat(caps.Formats)
	if err != nil {
		return d, err
	}

	alpha, err := chooseAlphaMode(caps.AlphaModes, log)
	if err != nil {
		return d, err
	}

	present, err := choosePresentMode(caps.PresentModes, vsync, log)
	if err != nil {
		return d, err
	}

	d.config = &wgpu.SurfaceConfiguration{
		Usage:       wgpu.TextureUsageRenderAttachment,
		Format:      format,
		PresentMode: present,
		AlphaMode:   alpha,
	}
	log.Info("native surface present mode selected",
		"requested_vsync_off", vsync == VSyncOff,
		"present_mode", present,
		"available_present_modes", caps.PresentModes,
	)
	fbW, fbH := window.GetFramebufferSize()
	d.configure(fbW, fbH)

	d.adapter.Release()
	d.adapter = nil

	return d, nil
}
