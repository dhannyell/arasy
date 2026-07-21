//go:build js && wasm

package wasm

import (
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/frameforge/internal/diagnostic"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/wasm/gpu"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

// gpuSpriteBatch is one retained sprite batch living on the GPU: a
// persistent storage buffer of 64-byte records written only on replace,
// consecutive draw ranges with their own bind groups, the optional
// per-instance animation buffer, and the CPU metadata culling and
// accounting need. Static batches reuse the streaming sprite pipelines;
// animated batches use the animation variants with dynamic-offset uniforms.
type gpuSpriteBatch struct {
	buffer     js.Value
	animBuffer js.Value
	cap        int
	animCap    int
	count      int
	animated   bool
	ranges     []spriteBatchRange
	bindGroups []js.Value

	bounds      math2d.Rect
	hasBounds   bool
	maxBounce   float64
	hasRotation bool
}

func (b *gpuSpriteBatch) destroy() {
	if !b.buffer.IsUndefined() && !b.buffer.IsNull() {
		b.buffer.Call("destroy")
	}
	if !b.animBuffer.IsUndefined() && !b.animBuffer.IsNull() {
		b.animBuffer.Call("destroy")
	}
}

// residentBytes reports the GPU bytes this batch holds, for accounting.
func (b *gpuSpriteBatch) residentBytes() int {
	return b.cap + b.animCap
}

// applySpriteBatchUpdates drains retained sprite batch updates after
// textures, so replaced batches can bind freshly uploaded textures.
func (r *webgpuRenderer) applySpriteBatchUpdates(updates []runtime.SpriteBatchUpdate) {
	for _, u := range updates {
		if u.Op == runtime.BatchDelete {
			if batch, ok := r.spriteBatches[u.ID]; ok {
				batch.destroy()
				delete(r.spriteBatches, u.ID)
			}
			continue
		}
		r.replaceSpriteBatch(u.ID, u.Instances)
	}
}

// replaceSpriteBatch packs and uploads a batch once. Superseded buffers are
// destroyed immediately — WebGPU destroys them on the queue timeline after
// previously submitted work, the same ordering the text atlas replacement
// already relies on, so no new asynchronous wait is introduced.
func (r *webgpuRenderer) replaceSpriteBatch(id render.SpriteBatchID, instances []render.SpriteInstance) {
	plan := planSpriteBatch(instances, r.texSizes, r.spriteArrayEnabled)
	if plan.nonFinite > 0 {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "webgpu-sprite-batch-non-finite", ID: uint64(id)},
			"webgpu: sprite batch replace dropped non-finite instances", "dropped", plan.nonFinite)
	}

	old := r.spriteBatches[id]
	if plan.count == 0 {
		// An empty replace keeps an empty batch; draws of it are silent
		// no-ops and only a delete removes the resource.
		if old != nil {
			old.destroy()
		}
		r.spriteBatches[id] = &gpuSpriteBatch{}
		return
	}

	batch := &gpuSpriteBatch{
		count:       plan.count,
		animated:    plan.animated,
		ranges:      plan.ranges,
		bounds:      plan.bounds,
		hasBounds:   plan.hasBounds,
		maxBounce:   plan.maxBounce,
		hasRotation: plan.hasRotation,
	}
	reuse := old != nil && old.animated == plan.animated &&
		old.cap >= len(plan.records) && old.animCap >= len(plan.anims)
	if reuse {
		batch.buffer, batch.cap = old.buffer, old.cap
		batch.animBuffer, batch.animCap = old.animBuffer, old.animCap
	} else if old != nil {
		old.destroy()
	}

	if batch.buffer.IsUndefined() || batch.buffer.IsNull() {
		batch.cap = len(plan.records)
		batch.buffer = jsHandle(r.dev.CreateBuffer(gpu.BufferDesc(batch.cap, gpu.BufferUsageStorage|gpu.BufferUsageCopyDst)))
	}
	r.queue.Call("writeBuffer", batch.buffer, 0, bytesToJS(plan.records))
	r.frameRetainedContentBytes += uint64(len(plan.records))

	if plan.animated {
		if batch.animBuffer.IsUndefined() || batch.animBuffer.IsNull() {
			batch.animCap = len(plan.anims)
			batch.animBuffer = jsHandle(r.dev.CreateBuffer(gpu.BufferDesc(batch.animCap, gpu.BufferUsageStorage|gpu.BufferUsageCopyDst)))
		}
		r.queue.Call("writeBuffer", batch.animBuffer, 0, bytesToJS(plan.anims))
		r.frameRetainedContentBytes += uint64(len(plan.anims))
	} else if !batch.animBuffer.IsUndefined() && !batch.animBuffer.IsNull() {
		batch.animBuffer.Call("destroy")
		batch.animBuffer = js.Undefined()
		batch.animCap = 0
	}

	batch.bindGroups = r.createSpriteBatchBindGroups(batch)
	r.spriteBatches[id] = batch
}

// createSpriteBatchBindGroups builds one bind group per range. Static
// batches reuse the streaming sprite pipeline layouts (auto layout: texture,
// sampler, records, shared camera uniform); animated batches use the
// explicit dynamic-offset layouts with the records, one shared uniform
// arena, and the animation records.
func (r *webgpuRenderer) createSpriteBatchBindGroups(batch *gpuSpriteBatch) []js.Value {
	groups := make([]js.Value, len(batch.ranges))
	for i, rg := range batch.ranges {
		if batch.animated {
			// The bind group's texture view dimension must match its
			// layout: spriteRangeTexture returns a 2d-array view on the
			// homogeneous array path (except for the magenta placeholder,
			// always plain 2d), so the layout choice must follow the same
			// condition the draw-time pipeline selection uses below.
			layout := r.spriteBatchAnimLayout
			if r.spriteArrayEnabled && !rg.magenta {
				layout = r.spriteBatchAnimArrayLayout
			}
			tex := r.spriteRangeTexture(rg)
			groups[i] = r.device.Call("createBindGroup", map[string]any{
				"layout": layout,
				"entries": []any{
					map[string]any{"binding": 0, "resource": tex},
					map[string]any{"binding": 1, "resource": r.texSampler},
					map[string]any{"binding": 2, "resource": map[string]any{"buffer": batch.buffer}},
					map[string]any{"binding": 3, "resource": map[string]any{
						"buffer": r.batchUniformBuffer, "offset": 0, "size": spriteBatchAnimUniformSize}},
					map[string]any{"binding": 4, "resource": map[string]any{"buffer": batch.animBuffer}},
				},
			})
			continue
		}
		if r.spriteArrayEnabled && !rg.magenta {
			groups[i] = r.device.Call("createBindGroup", map[string]any{
				"layout": r.spriteArrayPipeline.Call("getBindGroupLayout", 0),
				"entries": []any{
					map[string]any{"binding": 0, "resource": r.spriteArrayTexture.Call("createView", map[string]any{"dimension": "2d-array"})},
					map[string]any{"binding": 1, "resource": r.texSampler},
					map[string]any{"binding": 2, "resource": map[string]any{"buffer": batch.buffer}},
					map[string]any{"binding": 3, "resource": map[string]any{"buffer": r.spriteUniform}},
				},
			})
			continue
		}
		groups[i] = r.device.Call("createBindGroup", map[string]any{
			"layout": r.spritePipeline.Call("getBindGroupLayout", 0),
			"entries": []any{
				map[string]any{"binding": 0, "resource": r.spriteRangeTexture(rg)},
				map[string]any{"binding": 1, "resource": r.texSampler},
				map[string]any{"binding": 2, "resource": map[string]any{"buffer": batch.buffer}},
				map[string]any{"binding": 3, "resource": map[string]any{"buffer": r.spriteUniform}},
			},
		})
	}
	return groups
}

// spriteRangeTexture resolves the texture view one range binds: the shared
// 2d-array view on the homogeneous array path, the declared texture on the
// fallback path, or the 1x1 magenta placeholder for missing textures.
func (r *webgpuRenderer) spriteRangeTexture(rg spriteBatchRange) js.Value {
	if rg.magenta {
		return r.spritePlaceholderView()
	}
	if r.spriteArrayEnabled {
		return r.spriteArrayTexture.Call("createView", map[string]any{"dimension": "2d-array"})
	}
	if gt, ok := r.textureFor(rg.texture); ok {
		return gt.view
	}
	return r.spritePlaceholderView()
}

// spritePlaceholderView lazily creates the 1x1 opaque-magenta texture that
// stands in for missing textures in retained batches, preserving the
// streaming placeholder without solid-geometry fallbacks.
func (r *webgpuRenderer) spritePlaceholderView() js.Value {
	if !r.spritePlaceholder.IsUndefined() && !r.spritePlaceholder.IsNull() {
		return r.spritePlaceholder
	}
	texture := jsHandle(r.dev.CreateTexture(gpu.TextureDesc(1, 1, 1)))
	r.dev.WriteTexture(
		map[string]any{"texture": texture}, []byte{255, 0, 255, 255},
		gpu.TextureWriteLayout(4, 1), gpu.TextureWriteSize(1, 1))
	r.spritePlaceholder = jsHandle(r.dev.CreateView(texture, nil))
	return r.spritePlaceholder
}

// createSpriteBatchAnimPipelines builds the animated retained-sprite
// pipelines and their explicit bind group layouts. The uniform binding
// carries hasDynamicOffset, which "auto" layouts never produce, so the
// layout is explicit — one slot per draw command lives in the shared arena.
func (r *webgpuRenderer) createSpriteBatchAnimPipelines() {
	vertex := js.Global().Get("GPUShaderStage").Get("VERTEX").Int()
	fragment := js.Global().Get("GPUShaderStage").Get("FRAGMENT").Int()
	readOnly := map[string]any{"type": "read-only-storage"}

	layoutFor := func(viewDimension string) js.Value {
		texture := map[string]any{}
		if viewDimension != "" {
			texture["viewDimension"] = viewDimension
		}
		return r.device.Call("createBindGroupLayout", map[string]any{
			"entries": []any{
				map[string]any{"binding": 0, "visibility": fragment, "texture": texture},
				map[string]any{"binding": 1, "visibility": fragment, "sampler": map[string]any{}},
				map[string]any{"binding": 2, "visibility": vertex, "buffer": readOnly},
				map[string]any{"binding": 3, "visibility": vertex, "buffer": map[string]any{
					"type": "uniform", "hasDynamicOffset": true, "minBindingSize": spriteBatchAnimUniformSize}},
				map[string]any{"binding": 4, "visibility": vertex, "buffer": readOnly},
			},
		})
	}
	pipelineFor := func(module string, layout js.Value) js.Value {
		pipelineLayout := r.device.Call("createPipelineLayout", map[string]any{
			"bindGroupLayouts": []any{layout},
		})
		shader := r.device.Call("createShaderModule", map[string]any{"code": module})
		return r.device.Call("createRenderPipeline", map[string]any{
			"layout":      pipelineLayout,
			"vertex":      map[string]any{"module": shader, "entryPoint": "vs_main"},
			"fragment":    map[string]any{"module": shader, "entryPoint": "fs_main", "targets": []any{map[string]any{"format": r.format, "blend": gpu.PremultipliedBlend()}}},
			"primitive":   map[string]any{"topology": "triangle-list"},
			"multisample": map[string]any{"count": msaaSampleCount},
		})
	}

	r.spriteBatchAnimLayout = layoutFor("")
	r.spriteBatchAnimArrayLayout = layoutFor("2d-array")
	r.spriteBatchAnimPipeline = pipelineFor(spriteBatchAnimWGSL, r.spriteBatchAnimLayout)
	r.spriteBatchAnimArrayPipeline = pipelineFor(spriteBatchAnimArrayWGSL, r.spriteBatchAnimArrayLayout)
}

// ensureBatchUniformArena grows the shared uniform arena before any slots are
// assigned or written for the frame. Replacing the buffer after writes would
// redirect earlier bind groups to a new buffer without their uniform data.
func (r *webgpuRenderer) ensureBatchUniformArena(required int) {
	slots := uniformArenaSlotCapacity(r.batchArena.slots, required)
	if slots == r.batchArena.slots {
		return
	}
	old := r.batchUniformBuffer
	r.batchArena = newUniformArena(spriteBatchAnimUniformSize, r.batchArenaAlignment, slots)
	r.batchUniformBuffer = jsHandle(r.dev.CreateBuffer(gpu.BufferDesc(
		r.batchArena.slotSize*r.batchArena.slots, gpu.BufferUsageUniform|gpu.BufferUsageCopyDst)))
	for _, batch := range r.spriteBatches {
		if batch.animated {
			batch.bindGroups = r.createSpriteBatchBindGroups(batch)
		}
	}
	if !old.IsUndefined() && !old.IsNull() {
		old.Call("destroy")
	}
}
