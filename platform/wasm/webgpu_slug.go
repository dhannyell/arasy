//go:build js && wasm

package wasm

import (
	"log/slog"
	"syscall/js"
	"unsafe"

	"github.com/dhannyell/arasy/internal/diagnostic"
	gpubuffer "github.com/dhannyell/arasy/internal/render/gpu/buffer"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/internal/render/slugcore"
	"github.com/dhannyell/arasy/platform/wasm/gpu"
	"github.com/dhannyell/arasy/render"
)

// slugStorage is one storage buffer of the Slug shader.
type slugStorage struct {
	buffer   js.Value
	capacity int
}

// wasmSlug holds the GPU state of Slug text. The first Slug run creates it, so a
// game that does not use Slug text does not compile the shader.
type wasmSlug struct {
	engine slugcore.Engine

	layout, pipelineLayout           js.Value
	main, mainAdd, target, targetAdd js.Value
	curves, bands, layers, instances slugStorage
	bindGroup                        js.Value
}

func (s *wasmSlug) endFrame() {
	if s.engine != nil {
		s.engine.EndFrame()
	}
}

// generation returns the Pack generation. Without an engine, it never changes.
func (s *wasmSlug) generation() uint32 {
	if s.engine == nil {
		return 0
	}
	return s.engine.Pack().Generation()
}

func (r *webgpuRenderer) warnSlugOff() {
	r.diagnostics.Log(r.log, slog.LevelWarn,
		diagnostic.Key{Code: "slug-text-off"},
		"webgpu: GlyphSlug text uses the glyph atlas; set Options.GlyphOutlines to slug.Outlines")
}

// warnSlugBatchOff warns if a batch has a GlyphSlug label that no engine can draw.
func (r *webgpuRenderer) warnSlugBatchOff(labels []render.TextLabel) {
	if r.slug.engine != nil {
		return
	}
	for i := range labels {
		if labels[i].Strategy == render.GlyphSlug {
			r.warnSlugOff()
			return
		}
	}
}

// uploadSlug uploads the Slug data of the frame.
func (r *webgpuRenderer) uploadSlug() {
	if r.slug.engine == nil {
		if r.tess.SlugMissing() {
			r.warnSlugOff()
		}
		return
	}
	if err := r.slug.engine.TakeErr(); err != nil {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "slug-text-add"},
			"webgpu: Slug text is incomplete this frame", "error", err)
	}
	if r.slug.engine.Pack().Size() == 0 {
		return
	}
	r.createSlugPipelines()
	r.writeSlugBuffers()
	r.forEachTextBatch(func(batch *gpuTextBatch) {
		if batch.slugBuffer.IsUndefined() || !batch.slugBindGroup.IsUndefined() {
			return
		}
		batch.slugBindGroup = r.createSlugBindGroup(batch.slugBuffer)
	})
}

// writeSlugBatch replaces the Slug records of a retained batch.
func (r *webgpuRenderer) writeSlugBatch(batch *gpuTextBatch, instances []byte) {
	if len(instances) == 0 {
		return
	}
	if len(instances) > batch.slugCap {
		if !batch.slugBuffer.IsUndefined() {
			batch.slugBuffer.Call("destroy")
		}
		batch.slugBuffer, batch.slugCap = r.createSpriteBuffer(gpubuffer.GrowCapacity(batch.slugCap, len(instances)))
		batch.slugBindGroup = js.Undefined()
	}
	r.queue.Call("writeBuffer", batch.slugBuffer, 0, bytesToJS(instances))
}

func (r *webgpuRenderer) forEachTextBatch(visit func(*gpuTextBatch)) {
	for _, batch := range r.textBatches {
		visit(batch)
	}
	for _, batch := range r.passTextBatches {
		visit(batch)
	}
}

func (r *webgpuRenderer) createSlugPipelines() {
	if !r.slug.main.IsUndefined() {
		return
	}
	vertex := js.Global().Get("GPUShaderStage").Get("VERTEX").Int()
	fragment := js.Global().Get("GPUShaderStage").Get("FRAGMENT").Int()
	readOnly := map[string]any{"type": "read-only-storage"}
	r.slug.layout = r.device.Call("createBindGroupLayout", map[string]any{
		"entries": []any{
			map[string]any{"binding": 0, "visibility": vertex, "buffer": map[string]any{
				"type": "uniform", "hasDynamicOffset": true, "minBindingSize": tess.SpriteUniformSize,
			}},
			map[string]any{"binding": 1, "visibility": fragment, "buffer": readOnly},
			map[string]any{"binding": 2, "visibility": fragment, "buffer": readOnly},
			map[string]any{"binding": 3, "visibility": vertex | fragment, "buffer": readOnly},
			map[string]any{"binding": 4, "visibility": vertex, "buffer": readOnly},
		},
	})
	r.slug.pipelineLayout = r.device.Call("createPipelineLayout", map[string]any{
		"bindGroupLayouts": []any{r.slug.layout},
	})
	module := r.dev.CreateShaderModule(slugcore.WGSL())
	pipeline := func(format string, samples int, blend map[string]any) js.Value {
		return jsHandle(r.dev.CreateRenderPipeline(gpu.TextGlyphPipelineDescWithLayout(module, format, samples, blend, r.slug.pipelineLayout)))
	}
	r.slug.main = pipeline(r.format.String(), r.msaaSamples, gpu.AlphaBlend())
	r.slug.mainAdd = pipeline(r.format.String(), r.msaaSamples, gpu.AdditiveStraightBlend())
	r.slug.target = pipeline("rgba8unorm", 1, gpu.AlphaBlend())
	r.slug.targetAdd = pipeline("rgba8unorm", 1, gpu.AdditiveStraightBlend())
}

// writeSlugBuffers writes the data that the Pack added since the last frame, and
// all of the data of a buffer that grew: a new buffer has no old data.
func (r *webgpuRenderer) writeSlugBuffers() {
	pack := r.slug.engine.Pack()
	dirty := pack.TakeDirty()
	grew := false
	write := func(s *slugStorage, data []byte, start int) {
		if required := max(len(data), gpubuffer.InitialCapacity); s.buffer.IsUndefined() || required > s.capacity {
			if !s.buffer.IsUndefined() {
				s.buffer.Call("destroy")
			}
			s.buffer, s.capacity = r.createSpriteBuffer(gpubuffer.GrowCapacity(s.capacity, required))
			grew = true
			start = 0
		}
		if start < len(data) {
			r.queue.Call("writeBuffer", s.buffer, start, bytesToJS(data[start:]))
		}
	}
	const word = 4 // bytes in one element of each Pack slice
	write(&r.slug.curves, wordBytes(pack.Curves()), dirty.Curves.Start*word)
	write(&r.slug.bands, wordBytes(pack.Bands()), dirty.Bands.Start*word)
	write(&r.slug.layers, wordBytes(pack.Layers()), dirty.Layers.Start*word)
	write(&r.slug.instances, r.tess.SlugInstanceStream(), 0)
	if grew || r.slug.bindGroup.IsUndefined() {
		r.rebuildSlugBindGroup()
	}
}

// rebuildSlugBindGroup does nothing before the first Slug run. The sprite uniform
// buffer can change at any frame, so it also calls this function.
func (r *webgpuRenderer) rebuildSlugBindGroup() {
	s := &r.slug
	if s.layout.IsUndefined() || s.instances.buffer.IsUndefined() {
		return
	}
	s.bindGroup = r.createSlugBindGroup(s.instances.buffer)
	// uploadSlug creates them again, after the writes of the frame.
	r.forEachTextBatch(func(batch *gpuTextBatch) { batch.slugBindGroup = js.Undefined() })
}

func (r *webgpuRenderer) createSlugBindGroup(instances js.Value) js.Value {
	s := &r.slug
	storage := func(binding int, buffer js.Value) map[string]any {
		return map[string]any{"binding": binding, "resource": map[string]any{"buffer": buffer}}
	}
	return r.device.Call("createBindGroup", map[string]any{
		"layout": s.layout,
		"entries": []any{
			map[string]any{"binding": 0, "resource": map[string]any{"buffer": r.spriteUniform, "size": tess.SpriteUniformSize}},
			storage(1, s.curves.buffer),
			storage(2, s.bands.buffer),
			storage(3, s.layers.buffer),
			storage(4, instances),
		},
	})
}

// wordBytes views a slice of 4-byte words as bytes. Wasm is little-endian, so the
// bytes already have the order of the GPU buffer.
func wordBytes[T float32 | uint32](words []T) []byte {
	if len(words) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), len(words)*4)
}
