//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"fmt"
	"log/slog"

	"github.com/go-gl/glfw/v3.4/glfw"
	"github.com/oliverbestmann/webgpu/wgpu"
	"github.com/oliverbestmann/webgpu/wgpuglfw"
)

type device struct {
	instance *wgpu.Instance
	surface  *wgpu.Surface
	adapter  *wgpu.Adapter
	device   *wgpu.Device
	queue    *wgpu.Queue
	config   *wgpu.SurfaceConfiguration

	// vendorID/deviceID identify the adapter to the OS probes, so the dev
	// panel's VRAM reading lands on the same GPU the game renders to.
	vendorID uint32
	deviceID uint32

	// timestampQuery reports whether the device was created with the
	// timestamp-query feature, which is what gates per-pass GPU timing.
	timestampQuery bool

	// formatFeatures reports whether the device was created with wgpu's
	// adapter-specific format features, which is what makes 2x and 8x MSAA
	// available. WebGPU itself only guarantees 1 and 4.
	formatFeatures bool
}

// validateMSAASampleCount probes the selected surface format with the real
// multisample target descriptor.
func (d *device) validateMSAASampleCount(samples uint32) error {
	if d == nil || d.device == nil || d.config == nil {
		return fmt.Errorf("native WebGPU device is not configured")
	}
	texture, err := d.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "Arasy MSAA capability probe",
		Usage:         wgpu.TextureUsageRenderAttachment,
		Dimension:     wgpu.TextureDimension2D,
		Size:          wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		Format:        d.config.Format,
		MipLevelCount: 1,
		SampleCount:   samples,
	})
	if err != nil {
		return err
	}
	texture.Release()
	return nil
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
	log.Info("native WebGPU device limits requested",
		"max_storage_buffer_binding_size", limits.MaxStorageBufferBindingSize,
		"max_buffer_size", limits.MaxBufferSize,
		"max_compute_workgroups_per_dimension", limits.MaxComputeWorkgroupsPerDimension,
	)

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

func (d *device) destroy() {
	if d.adapter != nil {
		d.adapter.Release()
		d.adapter = nil
	}
	if d.queue != nil {
		d.queue.Release()
		d.queue = nil
	}
	if d.device != nil {
		d.device.Release()
		d.device = nil
	}
	if d.surface != nil {
		d.surface.Release()
		d.surface = nil
	}
	if d.instance != nil {
		d.instance.Release()
		d.instance = nil
	}
	d.config = nil
}
