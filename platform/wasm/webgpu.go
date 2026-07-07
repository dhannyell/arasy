//go:build js && wasm

package wasm

import (
	"errors"
	"fmt"
	"syscall/js"

	"github.com/dhannyell/frameforge/platform/wasm/tess"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

var errWebGPUUnavailable = errors.New("webgpu unavailable")

type webgpuRenderer struct {
	canvas js.Value
	clear  render.Color

	gpu     js.Value
	device  js.Value
	queue   js.Value
	context js.Value
	format  js.Value

	solidPipeline js.Value
	textPipeline  js.Value
	atlasBind     js.Value

	solidBuffer js.Value
	textBuffer  js.Value
	solidCap    int
	textCap     int

	atlas *tess.Atlas
}

func newWebGPU(canvas js.Value, clear render.Color) (*webgpuRenderer, error) {
	gpu := js.Global().Get("navigator").Get("gpu")
	if gpu.IsUndefined() || gpu.IsNull() {
		return nil, errWebGPUUnavailable
	}

	adapter, err := awaitPromise(gpu.Call("requestAdapter"))
	if err != nil {
		return nil, fmt.Errorf("request webgpu adapter: %w", err)
	}
	if adapter.IsUndefined() || adapter.IsNull() {
		return nil, errWebGPUUnavailable
	}

	device, err := awaitPromise(adapter.Call("requestDevice"))
	if err != nil {
		return nil, fmt.Errorf("request webgpu device: %w", err)
	}

	context := canvas.Call("getContext", "webgpu")
	if context.IsUndefined() || context.IsNull() {
		return nil, errWebGPUUnavailable
	}

	format := gpu.Call("getPreferredCanvasFormat")
	context.Call("configure", map[string]any{
		"device":    device,
		"format":    format,
		"alphaMode": "opaque",
	})

	r := &webgpuRenderer{
		canvas:  canvas,
		clear:   clear,
		gpu:     gpu,
		device:  device,
		queue:   device.Get("queue"),
		context: context,
		format:  format,
		atlas:   tess.NewAtlas(),
	}

	r.solidPipeline = r.createSolidPipeline()
	r.textPipeline = r.createTextPipeline()
	r.atlasBind = r.createAtlasBindGroup()
	r.solidBuffer, r.solidCap = r.createVertexBuffer(64 * 1024)
	r.textBuffer, r.textCap = r.createVertexBuffer(64 * 1024)

	return r, nil
}

func (r *webgpuRenderer) Render(frame runtime.Frame) {
	w := r.canvas.Get("width").Int()
	h := r.canvas.Get("height").Int()
	if w <= 0 || h <= 0 {
		return
	}

	mesh, textMesh := tess.Build(frame, w, h, r.atlas)

	solidBytes := tess.Bytes(mesh)
	textBytes := tess.TextBytes(textMesh)

	r.ensureSolidBuffer(len(solidBytes))
	r.ensureTextBuffer(len(textBytes))

	if len(solidBytes) > 0 {
		r.queue.Call("writeBuffer", r.solidBuffer, 0, bytesToJS(solidBytes))
	}
	if len(textBytes) > 0 {
		r.queue.Call("writeBuffer", r.textBuffer, 0, bytesToJS(textBytes))
	}

	encoder := r.device.Call("createCommandEncoder")
	textureView := r.context.Call("getCurrentTexture").Call("createView")

	pass := encoder.Call("beginRenderPass", map[string]any{
		"colorAttachments": []any{map[string]any{
			"view":       textureView,
			"loadOp":     "clear",
			"storeOp":    "store",
			"clearValue": clearValue(r.clear),
		}},
	})

	if len(mesh.Vertices) > 0 {
		pass.Call("setPipeline", r.solidPipeline)
		pass.Call("setVertexBuffer", 0, r.solidBuffer)
		pass.Call("draw", len(mesh.Vertices))
	}

	if len(textMesh.Vertices) > 0 {
		pass.Call("setPipeline", r.textPipeline)
		pass.Call("setBindGroup", 0, r.atlasBind)
		pass.Call("setVertexBuffer", 0, r.textBuffer)
		pass.Call("draw", len(textMesh.Vertices))
	}

	pass.Call("end")
	commandBuffer := encoder.Call("finish")
	r.queue.Call("submit", []any{commandBuffer})
}

func (r *webgpuRenderer) createSolidPipeline() js.Value {
	module := r.device.Call("createShaderModule", map[string]any{"code": solidWGSL})

	return r.device.Call("createRenderPipeline", map[string]any{
		"layout": "auto",
		"vertex": map[string]any{
			"module":     module,
			"entryPoint": "vs_main",
			"buffers": []any{map[string]any{
				"arrayStride": 24,
				"attributes": []any{
					map[string]any{"shaderLocation": 0, "offset": 0, "format": "float32x2"},
					map[string]any{"shaderLocation": 1, "offset": 8, "format": "float32x4"},
				},
			}},
		},
		"fragment": map[string]any{
			"module":     module,
			"entryPoint": "fs_main",
			"targets": []any{map[string]any{
				"format": r.format,
				"blend":  alphaBlend(),
			}},
		},
		"primitive": map[string]any{
			"topology": "triangle-list",
		},
	})
}

func (r *webgpuRenderer) createTextPipeline() js.Value {
	module := r.device.Call("createShaderModule", map[string]any{"code": textWGSL})

	return r.device.Call("createRenderPipeline", map[string]any{
		"layout": "auto",
		"vertex": map[string]any{
			"module":     module,
			"entryPoint": "vs_main",
			"buffers": []any{map[string]any{
				"arrayStride": 32,
				"attributes": []any{
					map[string]any{"shaderLocation": 0, "offset": 0, "format": "float32x2"},
					map[string]any{"shaderLocation": 1, "offset": 8, "format": "float32x4"},
					map[string]any{"shaderLocation": 2, "offset": 24, "format": "float32x2"},
				},
			}},
		},
		"fragment": map[string]any{
			"module":     module,
			"entryPoint": "fs_main",
			"targets": []any{map[string]any{
				"format": r.format,
				"blend":  alphaBlend(),
			}},
		},
		"primitive": map[string]any{
			"topology": "triangle-list",
		},
	})
}

func (r *webgpuRenderer) createAtlasBindGroup() js.Value {
	bounds := r.atlas.Image.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	texture := r.device.Call("createTexture", map[string]any{
		"size":   []any{width, height},
		"format": "r8unorm",
		"usage":  textureUsageTextureBinding() | textureUsageCopyDst(),
	})

	r.queue.Call(
		"writeTexture",
		map[string]any{"texture": texture},
		bytesToJS(r.atlas.Image.Pix),
		map[string]any{
			"bytesPerRow":  width,
			"rowsPerImage": height,
		},
		map[string]any{
			"width":              width,
			"height":             height,
			"depthOrArrayLayers": 1,
		},
	)

	sampler := r.device.Call("createSampler", map[string]any{
		"magFilter": "nearest",
		"minFilter": "nearest",
	})

	layout := r.textPipeline.Call("getBindGroupLayout", 0)

	return r.device.Call("createBindGroup", map[string]any{
		"layout": layout,
		"entries": []any{
			map[string]any{"binding": 0, "resource": texture.Call("createView")},
			map[string]any{"binding": 1, "resource": sampler},
		},
	})
}

func (r *webgpuRenderer) createVertexBuffer(size int) (js.Value, int) {
	usage := js.Global().Get("GPUBufferUsage")
	return r.device.Call("createBuffer", map[string]any{
		"size":  size,
		"usage": usage.Get("VERTEX").Int() | usage.Get("COPY_DST").Int(),
	}), size
}

func (r *webgpuRenderer) ensureSolidBuffer(size int) {
	if size <= r.solidCap {
		return
	}

	newCap := growBufferCap(r.solidCap, size)
	r.solidBuffer, r.solidCap = r.createVertexBuffer(newCap)
}

func (r *webgpuRenderer) ensureTextBuffer(size int) {
	if size <= r.textCap {
		return
	}

	newCap := growBufferCap(r.textCap, size)
	r.textBuffer, r.textCap = r.createVertexBuffer(newCap)
}

func awaitPromise(promise js.Value) (js.Value, error) {
	resolveChan := make(chan js.Value, 1)
	rejectChan := make(chan js.Value, 1)

	onResolve := js.FuncOf(func(this js.Value, args []js.Value) any {
		var val js.Value
		if len(args) > 0 {
			val = args[0]
		}
		resolveChan <- val
		return nil
	})
	defer onResolve.Release()

	onReject := js.FuncOf(func(this js.Value, args []js.Value) any {
		var val js.Value
		if len(args) > 0 {
			val = args[0]
		}
		rejectChan <- val
		return nil
	})
	defer onReject.Release()

	promise.Call("then", onResolve).Call("catch", onReject)

	select {
	case res := <-resolveChan:
		return res, nil
	case err := <-rejectChan:
		return js.Undefined(), fmt.Errorf("JS Error: %s", err.Call("toString").String())
	}
}

func growBufferCap(current, required int) int {
	if current <= 0 {
		current = 64 * 1024
	}

	for current < required {
		current *= 2
	}

	return current
}

func bytesToJS(data []byte) js.Value {
	array := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(array, data)
	return array
}

func alphaBlend() map[string]any {
	return map[string]any{
		"color": map[string]any{
			"srcFactor": "src-alpha",
			"dstFactor": "one-minus-src-alpha",
			"operation": "add",
		},
		"alpha": map[string]any{
			"srcFactor": "one",
			"dstFactor": "one-minus-src-alpha",
			"operation": "add",
		},
	}
}

func textureUsageTextureBinding() int {
	return js.Global().Get("GPUTextureUsage").Get("TEXTURE_BINDING").Int()
}

func textureUsageCopyDst() int {
	return js.Global().Get("GPUTextureUsage").Get("COPY_DST").Int()
}

func clearValue(clear render.Color) map[string]any {
	return map[string]any{
		"r": clear.R * clear.A,
		"g": clear.G * clear.A,
		"b": clear.B * clear.A,
		"a": 1,
	}
}
