//go:build js && wasm

package wasm

import (
	"syscall/js"

	"github.com/dhannyell/frameforge/platform/wasm/gpu"
)

// jsDevice adapts WebGPU resource setup to the js-free gpu.Device seam.
type jsDevice struct {
	device js.Value
	queue  js.Value
}

func newJSDevice(device, queue js.Value) gpu.Device {
	return jsDevice{device: device, queue: queue}
}

func (d jsDevice) CreateShaderModule(code string) gpu.Handle {
	return d.device.Call("createShaderModule", map[string]any{"code": code})
}

func (d jsDevice) CreateRenderPipeline(desc map[string]any) gpu.Handle {
	return d.device.Call("createRenderPipeline", resolveHandles(desc))
}

func (d jsDevice) CreateSampler(desc map[string]any) gpu.Handle {
	return d.device.Call("createSampler", resolveHandles(desc))
}

func (d jsDevice) CreateTexture(desc map[string]any) gpu.Handle {
	return d.device.Call("createTexture", resolveHandles(desc))
}

func (d jsDevice) CreateBuffer(desc map[string]any) gpu.Handle {
	return d.device.Call("createBuffer", resolveHandles(desc))
}

func (d jsDevice) CreateBindGroup(desc map[string]any) gpu.Handle {
	return d.device.Call("createBindGroup", resolveHandles(desc))
}

func (d jsDevice) CreateView(texture gpu.Handle, desc map[string]any) gpu.Handle {
	value := handleValue(texture)
	if desc == nil {
		return value.Call("createView")
	}
	return value.Call("createView", resolveHandles(desc))
}

func (d jsDevice) BindGroupLayout(pipeline gpu.Handle, index int) gpu.Handle {
	return handleValue(pipeline).Call("getBindGroupLayout", index)
}

func (d jsDevice) WriteBuffer(buffer gpu.Handle, offset int, data []byte) {
	d.queue.Call("writeBuffer", handleValue(buffer), offset, bytesToJS(data))
}

func (d jsDevice) WriteTexture(dst map[string]any, data []byte, layout, size map[string]any) {
	d.queue.Call("writeTexture", resolveHandles(dst), bytesToJS(data), resolveHandles(layout), resolveHandles(size))
}

func handleValue(handle gpu.Handle) js.Value {
	value, ok := handle.(js.Value)
	if !ok {
		panic("wasm gpu device received a non-js handle")
	}
	return value
}

// resolveHandles recursively rebuilds descriptor maps and slices so their
// opaque handles are passed to syscall/js as native js.Value instances.
func resolveHandles(value any) any {
	switch value := value.(type) {
	case js.Value:
		return value
	case map[string]any:
		resolved := make(map[string]any, len(value))
		for key, item := range value {
			resolved[key] = resolveHandles(item)
		}
		return resolved
	case []any:
		resolved := make([]any, len(value))
		for i, item := range value {
			resolved[i] = resolveHandles(item)
		}
		return resolved
	default:
		return value
	}
}

func jsHandle(handle gpu.Handle) js.Value { return handleValue(handle) }

func bytesToJS(data []byte) js.Value {
	array := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(array, data)
	return array
}
