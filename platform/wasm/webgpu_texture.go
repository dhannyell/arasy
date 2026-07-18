//go:build js && wasm

package wasm

import (
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/asset/pixel"
	"github.com/dhannyell/frameforge/platform/wasm/gpu"
	"github.com/dhannyell/frameforge/platform/wasm/tess"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

// gpuTexture is one declared texture uploaded to the GPU, with its full mip
// chain generated at boot.
type gpuTexture struct {
	texture   js.Value
	view      js.Value
	bindGroup js.Value
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

	decls := manifest.Textures()
	textures := make([]gpuTexture, len(reg))
	sizes := make(tess.TexSizes, len(reg))

	for _, decl := range decls {
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
		textures[decl.ID] = r.createGPUTexture(img.Pix, w, h)
	}

	return textures, sizes
}

// createGPUTexture uploads premultiplied RGBA pixels to a new rgba8unorm
// texture at mip level 0, generates the remaining mip levels, and builds
// the bind group the textured pipeline expects (binding 0: view, binding 1:
// the shared linear/anisotropic sampler).
func (r *webgpuRenderer) createGPUTexture(pix []byte, w, h int) gpuTexture {
	resources := gpu.BuildTextureResources(r.dev, pix, w, h, r.texturedPipeline, r.texSampler)
	texture := jsHandle(resources.Texture)
	r.generateMipmaps(texture, w, h, gpu.MipLevelCount(w, h))
	return gpuTexture{texture: texture, view: jsHandle(resources.View), bindGroup: jsHandle(resources.BindGroup)}
}

// generateMipmaps fills mip levels 1..levels-1 of tex by rendering a
// fullscreen triangle that samples level N into level N+1, one render pass
// per level, via r.mipPipeline. It is a method on the renderer (rather than
// a free function) so a future dynamic-texture feature can call it again
// after rewriting level 0 of an existing texture.
func (r *webgpuRenderer) generateMipmaps(tex js.Value, w, h, levels int) {
	if levels <= 1 {
		return
	}

	encoder := r.device.Call("createCommandEncoder")

	for level := 1; level < levels; level++ {
		srcView := tex.Call("createView", map[string]any{
			"baseMipLevel":  level - 1,
			"mipLevelCount": 1,
		})
		dstView := tex.Call("createView", map[string]any{
			"baseMipLevel":  level,
			"mipLevelCount": 1,
		})

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
func (r *webgpuRenderer) UpdateTextures(updates []runtime.TextureUpdate) {
	for _, u := range updates {
		if u.Image == nil {
			continue
		}
		gt, ok := r.textureFor(u.ID)
		if !ok {
			continue
		}
		b := u.Image.Bounds()
		w, h := b.Dx(), b.Dy()
		r.dev.WriteTexture(map[string]any{"texture": gt.texture}, u.Image.Pix, gpu.TextureWriteLayout(4*w, h), gpu.TextureWriteSize(w, h))
		r.generateMipmaps(gt.texture, w, h, gpu.MipLevelCount(w, h))
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
