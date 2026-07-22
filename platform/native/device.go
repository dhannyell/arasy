//go:build windows && amd64

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
	})
	if err != nil {
		return d, fmt.Errorf("request WebGPU adapter: %w", err)
	}
	info := d.adapter.GetInfo()
	log.Info("native WebGPU adapter selected",
		"backend", info.BackendType,
		"vendor", info.Vendor,
		"device", info.Device,
	)

	// This binding panics when DeviceLostCallback crosses the cgo boundary, so
	// device loss is observed through surface and submission errors instead.
	d.device, err = d.adapter.RequestDevice(nil)
	if err != nil {
		return d, fmt.Errorf("request WebGPU device: %w", err)
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
