//go:build js && wasm

package wasm

import (
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/platform/wasm/gpu"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// gpuTexture is one declared texture uploaded to the GPU, with its full mip
// chain generated at boot.
type gpuTexture struct {
	texture         js.Value
	view            js.Value
	bindGroup       js.Value
	spriteBindGroup js.Value
	arrayLayer      int
}

// buildTextures decodes every texture in manifest via asset/pixel and
// uploads it to the GPU, indexed like pixel.Registry (index 0 unused,
// TextureID 0 means "missing texture"). tess already draws a magenta
// placeholder for any DrawImage/DrawImageQuad whose texture id is not found
// in the returned sizes, so a decode failure here is not fatal: BuildRegistry
// already leaves the failed texture's slot nil and reports it in the joined
// error, so only that texture is missing -- the rest of the manifest still
// builds normally.
func (r *webgpuRenderer) buildTextures(manifest asset.Manifest) ([]gpuTexture, tess.TexSizes) {
	reg, err := pixel.BuildRegistry(manifest)
	if err != nil {
		slog.Default().Error("build texture registry", "error", err)
	}

	sizes := make(tess.TexSizes, len(reg))
	for _, decl := range manifest.Textures() {
		img := reg.Lookup(decl.ID)
		if img == nil {
			continue
		}

		bounds := img.Bounds()
		w, h := bounds.Dx(), bounds.Dy()
		if w <= 0 || h <= 0 {
			continue
		}
		sizes[decl.ID] = [2]int{w, h}
	}

	maxLayers := r.device.Get("limits").Get("maxTextureArrayLayers").Int()
	plan := planSpriteTextureArray(sizes, maxLayers)
	if plan.Enabled {
		textures := r.createGPUTextureArray(reg, plan)
		slog.Default().Info("sprite texture array enabled",
			"layers", plan.Layers, "width", plan.Width, "height", plan.Height,
		)
		return textures, sizes
	}

	textures := make([]gpuTexture, len(reg))
	for _, decl := range manifest.Textures() {
		img := reg.Lookup(decl.ID)
		if img == nil {
			continue
		}
		bounds := img.Bounds()
		textures[decl.ID] = r.createGPUTexture(img.Pix, bounds.Dx(), bounds.Dy())
	}
	slog.Default().Info("sprite texture array disabled", "reason", plan.Reason)
	return textures, sizes
}

// createGPUTextureArray uploads matching textures into one resource. Each
// texture keeps a single-layer 2D view for DrawImageQuad, while instanced
// sprites share the full array view. This replaces the individual resources
// instead of duplicating their pixels in GPU memory.
func (r *webgpuRenderer) createGPUTextureArray(reg pixel.Registry, plan spriteTextureArrayPlan) []gpuTexture {
	levels := gpu.MipLevelCount(plan.Width, plan.Height)
	texture := jsHandle(r.dev.CreateTexture(gpu.TextureArrayDesc(plan.Width, plan.Height, plan.Layers, levels)))
	textures := make([]gpuTexture, len(reg))
	for index, img := range reg[1:] {
		if img == nil {
			continue
		}
		r.dev.WriteTexture(
			map[string]any{"texture": texture, "origin": map[string]any{"z": index}}, img.Pix,
			gpu.TextureWriteLayout(4*plan.Width, plan.Height), gpu.TextureWriteSize(plan.Width, plan.Height),
		)
		r.generateMipmapsLayer(texture, plan.Width, plan.Height, levels, index)

		view := jsHandle(texture.Call("createView", map[string]any{
			"dimension":       "2d",
			"baseArrayLayer":  index,
			"arrayLayerCount": 1,
		}))
		layout := r.texturedPipeline.Call("getBindGroupLayout", 0)
		bindGroup := jsHandle(r.dev.CreateBindGroup(gpu.TextureBindGroupDesc(layout, view, r.texSampler)))
		textures[index+1] = gpuTexture{
			texture: texture, view: view, bindGroup: bindGroup, arrayLayer: index,
		}
	}

	r.spriteArrayTexture = texture
	r.spriteArrayEnabled = true
	return textures
}

// createGPUTexture uploads premultiplied RGBA pixels to a new rgba8unorm
// texture at mip level 0, generates the remaining mip levels, and builds
// the bind group the textured pipeline expects (binding 0: view, binding 1:
// the shared linear/anisotropic sampler).
func (r *webgpuRenderer) createGPUTexture(pix []byte, w, h int) gpuTexture {
	resources := gpu.BuildTextureResources(r.dev, pix, w, h, r.texturedPipeline, r.texSampler)
	texture := jsHandle(resources.Texture)
	r.generateMipmaps(texture, w, h, gpu.MipLevelCount(w, h))
	return gpuTexture{
		texture: texture, view: jsHandle(resources.View), bindGroup: jsHandle(resources.BindGroup),
		arrayLayer: -1,
	}
}

// rebuildSpriteBindGroups connects each declared texture to the current
// sprite storage buffer and camera uniform. It runs only after texture setup
// and on the rare storage-buffer growth path.
func (r *webgpuRenderer) rebuildSpriteBindGroups() {
	if r.spriteArrayEnabled {
		layout := r.spriteArrayLayout
		view := r.spriteArrayTexture.Call("createView", map[string]any{"dimension": "2d-array"})
		r.spriteArrayBindGroup = r.device.Call("createBindGroup", map[string]any{
			"layout": layout,
			"entries": []any{
				map[string]any{"binding": 0, "resource": view},
				map[string]any{"binding": 1, "resource": r.texSampler},
				map[string]any{"binding": 2, "resource": map[string]any{"buffer": r.spriteBuffer}},
				map[string]any{"binding": 3, "resource": map[string]any{"buffer": r.spriteUniform, "size": tess.SpriteUniformSize}},
			},
		})
		return
	}
	layout := r.spriteLayout
	for i := range r.textures {
		texture := &r.textures[i]
		if texture.view.IsUndefined() || texture.view.IsNull() {
			continue
		}
		texture.spriteBindGroup = r.device.Call("createBindGroup", map[string]any{
			"layout": layout,
			"entries": []any{
				map[string]any{"binding": 0, "resource": texture.view},
				map[string]any{"binding": 1, "resource": r.texSampler},
				map[string]any{"binding": 2, "resource": map[string]any{"buffer": r.spriteBuffer}},
				map[string]any{"binding": 3, "resource": map[string]any{"buffer": r.spriteUniform, "size": tess.SpriteUniformSize}},
			},
		})
	}
}

// generateMipmaps fills mip levels 1..levels-1 of tex by rendering a
// fullscreen triangle that samples level N into level N+1, one render pass
// per level, via r.mipPipeline. It is a method on the renderer (rather than
// a free function) so a future dynamic-texture feature can call it again
// after rewriting level 0 of an existing texture.
func (r *webgpuRenderer) generateMipmaps(tex js.Value, w, h, levels int) {
	r.generateMipmapsLayer(tex, w, h, levels, -1)
}

// generateMipmapsLayer is generateMipmaps for one texture-array layer. A
// single-layer 2D view keeps the existing mip shader and sampler unchanged.
// layer < 0 selects a regular non-array texture.
func (r *webgpuRenderer) generateMipmapsLayer(tex js.Value, w, h, levels, layer int) {
	if levels <= 1 {
		return
	}

	encoder := r.device.Call("createCommandEncoder")

	for level := 1; level < levels; level++ {
		srcDesc := map[string]any{
			"dimension":     "2d",
			"baseMipLevel":  level - 1,
			"mipLevelCount": 1,
		}
		dstDesc := map[string]any{
			"dimension":     "2d",
			"baseMipLevel":  level,
			"mipLevelCount": 1,
		}
		if layer >= 0 {
			srcDesc["baseArrayLayer"] = layer
			srcDesc["arrayLayerCount"] = 1
			dstDesc["baseArrayLayer"] = layer
			dstDesc["arrayLayerCount"] = 1
		}
		srcView := tex.Call("createView", srcDesc)
		dstView := tex.Call("createView", dstDesc)

		bindGroup := r.device.Call("createBindGroup", map[string]any{
			"layout": r.mipPipeline.Call("getBindGroupLayout", 0),
			"entries": []any{
				map[string]any{"binding": 0, "resource": srcView},
				map[string]any{"binding": 1, "resource": r.mipSampler},
			},
		})

		pass := encoder.Call("beginRenderPass", map[string]any{
			"colorAttachments": []any{map[string]any{
				"view":       dstView,
				"loadOp":     "clear",
				"storeOp":    "store",
				"clearValue": map[string]any{"r": 0, "g": 0, "b": 0, "a": 0},
			}},
		})
		pass.Call("setPipeline", r.mipPipeline)
		pass.Call("setBindGroup", 0, bindGroup)
		pass.Call("draw", 3)
		pass.Call("end")
	}

	r.queue.Call("submit", []any{encoder.Call("finish")})
}

// textureFor resolves id to its uploaded GPU texture. ok is false when id is
// out of range or was never built (e.g. a decode failure logged in
// buildTextures); tess.Build already substitutes a solid magenta placeholder
// for such ids, so the caller should simply skip drawing the run.
func (r *webgpuRenderer) UpdateResources(updates runtime.FrameUpdates) {
	// UpdateResources runs before Render's own CPU timing window opens
	// (Driver.RenderFrame calls it between Frame() and Render()), but a
	// sprite batch replace packs and uploads its records right here. Time it
	// so that cost is folded into the frame's reported upload instead of
	// disappearing outside every measured phase.
	var start float64
	if r.sampleCPUTiming {
		start = performanceNowMillis()
	}
	// UpdateResources is the frame boundary every host calls right before
	// Render (Driver.RenderFrame), so this is where the retained-batch
	// content-bytes-this-frame counter resets.
	r.frameRetainedContentBytes = 0
	for _, u := range updates.Textures {
		if u.Image == nil {
			continue
		}
		gt, ok := r.textureFor(u.ID)
		if !ok {
			continue
		}
		b := u.Image.Bounds()
		w, h := b.Dx(), b.Dy()
		destination := map[string]any{"texture": gt.texture}
		if gt.arrayLayer >= 0 {
			destination["origin"] = map[string]any{"z": gt.arrayLayer}
		}
		r.dev.WriteTexture(destination, u.Image.Pix, gpu.TextureWriteLayout(4*w, h), gpu.TextureWriteSize(w, h))
		if gt.arrayLayer >= 0 {
			r.generateMipmapsLayer(gt.texture, w, h, gpu.MipLevelCount(w, h), gt.arrayLayer)
		} else {
			r.generateMipmaps(gt.texture, w, h, gpu.MipLevelCount(w, h))
		}
	}
	// Textures first: retained batches may reference them.
	r.applySpriteBatchUpdates(updates.SpriteBatches)
	r.applyTextBatchUpdates(updates.TextBatches)
	if r.sampleCPUTiming {
		r.cpuResourceUpdate = durationFromMillis(performanceNowMillis() - start)
	}
}

func (r *webgpuRenderer) textureFor(id render.TextureID) (gpuTexture, bool) {
	if id == 0 || int(id) >= len(r.textures) {
		return gpuTexture{}, false
	}
	t := r.textures[id]
	if t.bindGroup.IsUndefined() || t.bindGroup.IsNull() {
		return gpuTexture{}, false
	}
	return t, true
}
