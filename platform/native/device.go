//go:build windows && amd64

package native

import (
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/go-gl/glfw/v3.4/glfw"
	"github.com/go-webgpu/webgpu/wgpu"
)

type device struct {
	instance *wgpu.Instance
	surface  *wgpu.Surface
	adapter  *wgpu.Adapter
	device   *wgpu.Device
	queue    *wgpu.Queue
	config   *wgpu.SurfaceConfiguration
}

func newDevice(window *glfw.Window, vsync VSyncMode, log *slog.Logger) (d *device, err error) {
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

	if err = wgpu.Init(); err != nil {
		return d, fmt.Errorf("initialize WebGPU: %w", err)
	}
	d.instance, err = wgpu.CreateInstance(nil)
	if err != nil {
		return d, fmt.Errorf("create WebGPU instance: %w", err)
	}
	d.surface, err = d.instance.CreateSurfaceFromWindowsHWND(0, uintptr(unsafe.Pointer(window.GetWin32Window())))
	if err != nil {
		return d, fmt.Errorf("create WebGPU surface: %w", err)
	}

	d.adapter, err = d.instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		CompatibleSurface: d.surface,
	})
	if err != nil {
		return d, fmt.Errorf("request WebGPU adapter: %w", err)
	}
	info, err := d.adapter.Info()
	if err != nil {
		return d, fmt.Errorf("get WebGPU adapter info: %w", err)
	}
	log.Info("native WebGPU adapter selected",
		"backend", info.BackendType,
		"vendor", info.Vendor,
		"device", info.Device,
	)

	d.device, err = d.adapter.RequestDevice(&wgpu.DeviceDescriptor{
		Label: "frameforge-device",
	})
	if err != nil {
		return d, fmt.Errorf("request WebGPU device: %w", err)
	}

	d.queue = d.device.Queue()

	caps, err := d.surface.GetCapabilities(d.adapter)
	if err != nil {
		return d, fmt.Errorf("get surface capabilities: %w", err)
	}
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
