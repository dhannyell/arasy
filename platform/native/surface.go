//go:build windows && amd64

package native

import (
	"fmt"
	"log/slog"

	"github.com/oliverbestmann/webgpu/wgpu"
)

type surfaceAction uint8

const (
	actionRender surfaceAction = iota
	actionSkip
	actionReconfigure
	actionFatal
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
	for _, mode := range modes {
		if mode == want {
			return true
		}
	}
	return false
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
