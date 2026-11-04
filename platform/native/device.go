//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

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

// configure updates the surface from physical framebuffer dimensions. A
// minimized window reports a zero size, for which wgpu-native must not be
// configured.
func (d *device) configure(fbW, fbH int) bool {
	if d == nil || d.surface == nil || d.device == nil || d.config == nil || fbW <= 0 || fbH <= 0 {
		return false
	}
	d.config.Width = uint32(fbW)
	d.config.Height = uint32(fbH)
	d.surface.Configure(d.device, d.config)
	return true
}
