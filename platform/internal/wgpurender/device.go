//go:build ((windows || linux || darwin) && (amd64 || arm64)) || js

package wgpurender

import (
	"fmt"

	"github.com/dhannyell/webgpu/wgpu"
)

type surfaceAction uint8

const (
	actionRender surfaceAction = iota
	actionSkip
	actionReconfigure
	actionFatal
)

// Device holds the GPU objects that a host opens for the renderer.
type Device struct {
	Instance *wgpu.Instance
	Surface  *wgpu.Surface
	Adapter  *wgpu.Adapter
	Device   *wgpu.Device
	Queue    *wgpu.Queue
	Config   *wgpu.SurfaceConfiguration

	// OffscreenView replaces the surface when it is set. The renderer then
	// draws here and presents nothing. The owner of the view releases it.
	OffscreenView *wgpu.TextureView

	// VendorID/DeviceID identify the adapter to the OS probes, so the dev
	// panel's VRAM reading lands on the same GPU the game renders to.
	VendorID uint32
	DeviceID uint32

	// TimestampQuery reports whether the device was created with the
	// timestamp-query feature, which is what gates per-pass GPU timing.
	TimestampQuery bool

	// FormatFeatures reports whether the device was created with wgpu's
	// adapter-specific format features, which is what makes 2x and 8x MSAA
	// available. WebGPU itself only guarantees 1 and 4.
	FormatFeatures bool
}

// validateMSAASampleCount probes the selected surface format with the real
// multisample target descriptor.
func (d *Device) validateMSAASampleCount(samples uint32) error {
	if d == nil || d.Device == nil || d.Config == nil {
		return fmt.Errorf("native WebGPU device is not configured")
	}
	texture, err := d.Device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "Arasy MSAA capability probe",
		Usage:         wgpu.TextureUsageRenderAttachment,
		Dimension:     wgpu.TextureDimension2D,
		Size:          wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		Format:        d.Config.Format,
		MipLevelCount: 1,
		SampleCount:   samples,
	})
	if err != nil {
		return err
	}
	texture.Release()
	return nil
}

// Destroy releases the GPU objects of d.
func (d *Device) Destroy() {
	if d.Adapter != nil {
		d.Adapter.Release()
		d.Adapter = nil
	}
	if d.Queue != nil {
		d.Queue.Release()
		d.Queue = nil
	}
	if d.Device != nil {
		d.Device.Release()
		d.Device = nil
	}
	if d.Surface != nil {
		d.Surface.Release()
		d.Surface = nil
	}
	if d.Instance != nil {
		d.Instance.Release()
		d.Instance = nil
	}
	d.Config = nil
}

// decideSurfaceStatus maps a surface texture status to a frame-loop action.
func decideSurfaceStatus(status wgpu.SurfaceGetCurrentTextureStatus) surfaceAction {
	switch status {
	case wgpu.SurfaceGetCurrentTextureStatusSuccessOptimal,
		wgpu.SurfaceGetCurrentTextureStatusSuccessSuboptimal:
		return actionRender
	case wgpu.SurfaceGetCurrentTextureStatusTimeout,
		wgpu.SurfaceGetCurrentTextureStatusOccluded:
		return actionSkip
	case wgpu.SurfaceGetCurrentTextureStatusOutdated,
		wgpu.SurfaceGetCurrentTextureStatusLost:
		return actionReconfigure
	default:
		return actionFatal
	}
}

// Configure updates the surface from physical framebuffer dimensions. A
// minimized window reports a zero size, for which the surface must not be
// configured.
func (d *Device) Configure(fbW, fbH int) bool {
	if d == nil || d.Surface == nil || d.Device == nil || d.Config == nil || fbW <= 0 || fbH <= 0 {
		return false
	}
	d.Config.Width = uint32(fbW)
	d.Config.Height = uint32(fbH)
	d.Surface.Configure(d.Device, d.Config)
	return true
}
