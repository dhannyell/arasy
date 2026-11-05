//go:build ((windows || linux || darwin) && (amd64 || arm64)) || js

package wgpurender

import (
	"fmt"
	"log/slog"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/asset/pixel"
	internalrender "github.com/dhannyell/arasy/internal/render"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/webgpu/wgpu"
)

// gpuTexture is one declared texture uploaded to the GPU with its complete
// mip chain. TextureID zero remains unavailable so tess can emit its magenta
// missing-texture placeholder. A layer of the shared sprite array has
// arrayLayer >= 0 and does not own texture; a standalone texture has -1.
type gpuTexture struct {
	texture         *wgpu.Texture
	targetTexture   *wgpu.Texture
	view            *wgpu.TextureView
	targetView      *wgpu.TextureView
	bindGroup       *wgpu.BindGroup
	targetBindGroup *wgpu.BindGroup
	spriteBindGroup *wgpu.BindGroup
	arrayLayer      int
	mipLevels       int
	dynamic         bool
	rtt             bool
}

// spriteTextureArray is the one resource that holds every layer of the sprite
// array. The renderer owns it; the gpuTexture layers only reference it.
type spriteTextureArray struct {
	texture *wgpu.Texture
	view    *wgpu.TextureView
}

func (a *spriteTextureArray) release() {
	releaseTextureView(&a.view)
	releaseTexture(&a.texture)
}

type textureResources struct {
	device                 *wgpu.Device
	queue                  *wgpu.Queue
	texturedPipeline       *wgpu.RenderPipeline
	targetTexturedPipeline *wgpu.RenderPipeline
	mipPipeline            *wgpu.RenderPipeline
	textureSampler         *wgpu.Sampler
	mipSampler             *wgpu.Sampler
}

func buildTextures(manifest asset.Manifest, resources textureResources, log *slog.Logger) ([]gpuTexture, tess.TexSizes, spriteTextureArray) {
	if log == nil {
		log = slog.Default()
	}
	registry, err := pixel.BuildRegistry(manifest)
	if err != nil {
		log.Error("build texture registry", "error", err)
	}

	sizes := make(tess.TexSizes, len(registry))
	for _, declaration := range manifest.Textures() {
		image := registry.Lookup(declaration.ID)
		if image == nil {
			continue
		}
		bounds := image.Bounds()
		if bounds.Dx() > 0 && bounds.Dy() > 0 {
			sizes[declaration.ID] = [2]int{bounds.Dx(), bounds.Dy()}
		}
	}

	var array spriteTextureArray
	plan := tess.PlanSpriteTextureArray(sizes, int(resources.device.GetLimits().MaxTextureArrayLayers))
	if plan.Enabled {
		array, err = resources.createTextureArray(plan)
		if err != nil {
			log.Error("create sprite texture array", "error", err)
		} else {
			log.Info("sprite texture array enabled", "layers", plan.Layers, "width", plan.Width, "height", plan.Height)
		}
	} else {
		log.Info("sprite texture array disabled", "reason", plan.Reason)
	}

	textures := make([]gpuTexture, len(registry))
	for _, declaration := range manifest.Textures() {
		width, height, ok := sizes.Lookup(declaration.ID)
		if !ok {
			continue
		}
		pixels := registry.Lookup(declaration.ID).Pix
		mipLevels := mipLevelCount(width, height)
		var texture gpuTexture
		if array.texture != nil {
			texture, err = resources.createTextureLayer(array.texture, int(declaration.ID)-1, pixels, width, height, mipLevels, declaration.Dynamic)
		} else {
			texture, err = resources.createGPUTexture(pixels, width, height, mipLevels, declaration.Dynamic)
		}
		if err != nil {
			log.Error("upload texture", "texture", declaration.Name, "error", err)
			sizes[declaration.ID] = [2]int{}
			continue
		}
		textures[declaration.ID] = texture
	}
	return textures, sizes, array
}

func buildMissingTexture(resources textureResources) (gpuTexture, error) {
	placeholder := internalrender.MissingTextureRGBA()
	return resources.createGPUTexture(
		placeholder.Pix,
		internalrender.MissingTextureSize,
		internalrender.MissingTextureSize,
		1,
		false,
	)
}

// createTextureArray creates the shared resource. A failure leaves the caller
// on standalone textures.
func (r textureResources) createTextureArray(plan tess.SpriteTextureArrayPlan) (spriteTextureArray, error) {
	if err := r.valid(); err != nil {
		return spriteTextureArray{}, err
	}
	levels := uint32(mipLevelCount(plan.Width, plan.Height))
	texture, err := r.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "Arasy sprite texture array",
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst | wgpu.TextureUsageRenderAttachment,
		Dimension:     wgpu.TextureDimension2D,
		Size:          wgpu.Extent3D{Width: uint32(plan.Width), Height: uint32(plan.Height), DepthOrArrayLayers: uint32(plan.Layers)},
		Format:        wgpu.TextureFormatRGBA8Unorm,
		MipLevelCount: levels,
		SampleCount:   1,
	})
	if err != nil {
		return spriteTextureArray{}, fmt.Errorf("create texture: %w", err)
	}
	array := spriteTextureArray{texture: texture}
	array.view, err = texture.TryCreateView(&wgpu.TextureViewDescriptor{
		Dimension: wgpu.TextureViewDimension2DArray, MipLevelCount: levels,
		ArrayLayerCount: uint32(plan.Layers), Aspect: wgpu.TextureAspectAll,
	})
	if err != nil {
		array.release()
		return spriteTextureArray{}, fmt.Errorf("create view: %w", err)
	}
	return array, nil
}

// createTextureLayer uploads one texture into the shared array. Its 2D view
// serves DrawImageQuad and particles; sprites sample the array view.
func (r textureResources) createTextureLayer(array *wgpu.Texture, layer int, pixels []byte, width, height, levels int, dynamic bool) (gpuTexture, error) {
	result := gpuTexture{texture: array, arrayLayer: layer, mipLevels: levels, dynamic: dynamic}
	if err := r.writeRGBA(array, layer, pixels, width, height); err != nil {
		return gpuTexture{}, err
	}
	if err := r.generateMipmaps(array, levels, layer); err != nil {
		return gpuTexture{}, err
	}
	view, err := array.TryCreateView(&wgpu.TextureViewDescriptor{
		Dimension: wgpu.TextureViewDimension2D, MipLevelCount: uint32(levels),
		BaseArrayLayer: uint32(layer), ArrayLayerCount: 1, Aspect: wgpu.TextureAspectAll,
	})
	if err != nil {
		return gpuTexture{}, fmt.Errorf("create texture layer view: %w", err)
	}
	result.view = view
	if err := r.bindTextureView(&result); err != nil {
		result.release()
		return gpuTexture{}, err
	}
	return result, nil
}

func (r textureResources) createGPUTexture(pixels []byte, width, height, levels int, dynamic bool) (gpuTexture, error) {
	if err := r.valid(); err != nil {
		return gpuTexture{}, err
	}
	texture, err := r.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "Arasy texture",
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst | wgpu.TextureUsageRenderAttachment,
		Dimension:     wgpu.TextureDimension2D,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		Format:        wgpu.TextureFormatRGBA8Unorm,
		MipLevelCount: uint32(levels),
		SampleCount:   1,
	})
	if err != nil {
		return gpuTexture{}, fmt.Errorf("create texture: %w", err)
	}
	result := gpuTexture{texture: texture, arrayLayer: -1, mipLevels: levels, dynamic: dynamic}
	if err := r.writeRGBA(texture, -1, pixels, width, height); err != nil {
		result.release()
		return gpuTexture{}, err
	}
	if err := r.generateMipmaps(texture, levels, -1); err != nil {
		result.release()
		return gpuTexture{}, err
	}
	view, err := texture.TryCreateView(nil)
	if err != nil {
		result.release()
		return gpuTexture{}, fmt.Errorf("create texture view: %w", err)
	}
	result.view = view
	if dynamic {
		targetView, err := texture.TryCreateView(&wgpu.TextureViewDescriptor{
			Dimension: wgpu.TextureViewDimension2D, BaseMipLevel: 0, MipLevelCount: 1,
			ArrayLayerCount: 1, Aspect: wgpu.TextureAspectAll,
		})
		if err != nil {
			result.release()
			return gpuTexture{}, fmt.Errorf("create target texture view: %w", err)
		}
		result.targetView = targetView
	}
	if err := r.bindTextureView(&result); err != nil {
		result.release()
		return gpuTexture{}, err
	}
	return result, nil
}

// bindTextureView creates the textured bind groups for texture.view.
func (r textureResources) bindTextureView(texture *gpuTexture) error {
	layout := r.texturedPipeline.GetBindGroupLayout(0)
	defer layout.Release()
	bindGroup, err := r.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Arasy textured bind group",
		Layout: layout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: texture.view},
			{Binding: 1, Sampler: r.textureSampler},
		},
	})
	if err != nil {
		return fmt.Errorf("create texture bind group: %w", err)
	}
	texture.bindGroup = bindGroup
	if r.targetTexturedPipeline == nil {
		return nil
	}
	targetLayout := r.targetTexturedPipeline.GetBindGroupLayout(0)
	defer targetLayout.Release()
	targetBindGroup, err := r.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Label: "Arasy target textured bind group", Layout: targetLayout,
		Entries: []wgpu.BindGroupEntry{{Binding: 0, TextureView: texture.view}, {Binding: 1, Sampler: r.textureSampler}},
	})
	if err != nil {
		return fmt.Errorf("create target texture bind group: %w", err)
	}
	texture.targetBindGroup = targetBindGroup
	return nil
}

func (r textureResources) updateTextures(updates []runtime.TextureUpdate, textures []gpuTexture) error {
	if err := r.valid(); err != nil {
		return err
	}
	for _, update := range updates {
		if update.Image == nil || update.ID == 0 || int(update.ID) >= len(textures) {
			continue
		}
		texture := textures[update.ID]
		if texture.texture == nil {
			continue
		}
		bounds := update.Image.Bounds()
		width, height := bounds.Dx(), bounds.Dy()
		if width <= 0 || height <= 0 {
			continue
		}
		if err := r.writeRGBA(texture.texture, texture.arrayLayer, update.Image.Pix, width, height); err != nil {
			return fmt.Errorf("update texture %d: %w", update.ID, err)
		}
		levels := texture.mipLevels
		if texture.rtt && texture.arrayLayer < 0 {
			levels = 1
		}
		if err := r.generateMipmaps(texture.texture, levels, texture.arrayLayer); err != nil {
			return fmt.Errorf("generate texture %d mipmaps: %w", update.ID, err)
		}
	}
	return nil
}

// writeRGBA fills mip 0 of texture. A negative layer selects a standalone
// texture.
func (r textureResources) writeRGBA(texture *wgpu.Texture, layer int, pixels []byte, width, height int) error {
	if err := r.queue.TryWriteTexture(
		&wgpu.TexelCopyTextureInfo{Texture: texture, Origin: wgpu.Origin3D{Z: uint32(max(layer, 0))}, Aspect: wgpu.TextureAspectAll},
		pixels,
		&wgpu.TexelCopyBufferLayout{BytesPerRow: uint32(4 * width), RowsPerImage: uint32(height)},
		&wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
	); err != nil {
		return fmt.Errorf("write texture: %w", err)
	}
	return nil
}

func (r textureResources) generateMipmaps(texture *wgpu.Texture, levels, layer int) error {
	if levels <= 1 {
		return nil
	}
	encoder, err := r.device.TryCreateCommandEncoder(nil)
	if err != nil {
		return fmt.Errorf("create mipmap command encoder: %w", err)
	}
	defer encoder.Release()

	if err := r.encodeMipmaps(encoder, texture, levels, layer); err != nil {
		return err
	}
	commandBuffer, err := encoder.TryFinish(nil)
	if err != nil {
		return fmt.Errorf("finish mipmap command encoder: %w", err)
	}
	defer commandBuffer.Release()
	r.queue.Submit(commandBuffer)
	return nil
}

// encodeMipmaps records mip generation into an existing encoder, so a later
// pass of the same frame samples the new chain of a target layer.
func (r textureResources) encodeMipmaps(encoder *wgpu.CommandEncoder, texture *wgpu.Texture, levels, layer int) error {
	for level := 1; level < levels; level++ {
		if err := r.generateMipLevel(encoder, texture, uint32(level), layer); err != nil {
			return err
		}
	}
	return nil
}

func (r textureResources) generateMipLevel(encoder *wgpu.CommandEncoder, texture *wgpu.Texture, level uint32, layer int) error {
	source, err := texture.TryCreateView(&wgpu.TextureViewDescriptor{
		Dimension:       wgpu.TextureViewDimension2D,
		BaseMipLevel:    level - 1,
		MipLevelCount:   1,
		BaseArrayLayer:  uint32(max(layer, 0)),
		ArrayLayerCount: 1,
		Aspect:          wgpu.TextureAspectAll,
	})
	if err != nil {
		return fmt.Errorf("create source mip view: %w", err)
	}
	defer source.Release()
	destination, err := texture.TryCreateView(&wgpu.TextureViewDescriptor{
		Dimension:       wgpu.TextureViewDimension2D,
		BaseMipLevel:    level,
		MipLevelCount:   1,
		BaseArrayLayer:  uint32(max(layer, 0)),
		ArrayLayerCount: 1,
		Aspect:          wgpu.TextureAspectAll,
	})
	if err != nil {
		return fmt.Errorf("create destination mip view: %w", err)
	}
	defer destination.Release()
	layout := r.mipPipeline.GetBindGroupLayout(0)
	defer layout.Release()
	bindGroup, err := r.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Arasy mipmap bind group",
		Layout: layout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: source},
			{Binding: 1, Sampler: r.mipSampler},
		},
	})
	if err != nil {
		return fmt.Errorf("create mipmap bind group: %w", err)
	}
	defer bindGroup.Release()
	pass, err := encoder.TryBeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "Arasy mipmap pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       destination,
			LoadOp:     wgpu.LoadOpClear,
			StoreOp:    wgpu.StoreOpStore,
			ClearValue: wgpu.Color{},
		}},
	})
	if err != nil {
		return fmt.Errorf("begin mipmap render pass: %w", err)
	}
	defer pass.Release()
	pass.SetPipeline(r.mipPipeline)
	pass.SetBindGroup(0, bindGroup, nil)
	pass.Draw(3, 1, 0, 0)
	if err := pass.TryEnd(); err != nil {
		return fmt.Errorf("end mipmap render pass: %w", err)
	}
	return nil
}

func (r textureResources) valid() error {
	if r.device == nil || r.queue == nil || r.texturedPipeline == nil || r.mipPipeline == nil || r.textureSampler == nil || r.mipSampler == nil {
		return fmt.Errorf("texture resources are incomplete")
	}
	return nil
}

func (t gpuTexture) release() {
	if t.spriteBindGroup != nil {
		t.spriteBindGroup.Release()
	}
	if t.bindGroup != nil {
		t.bindGroup.Release()
	}
	if t.targetBindGroup != nil {
		t.targetBindGroup.Release()
	}
	if t.targetView != nil {
		t.targetView.Release()
	}
	if t.view != nil {
		t.view.Release()
	}
	if t.targetTexture != nil {
		t.targetTexture.Destroy()
		t.targetTexture.Release()
	}
	if t.texture != nil && t.arrayLayer < 0 {
		t.texture.Destroy()
		t.texture.Release()
	}
}

func textureSamplerDescriptor() wgpu.SamplerDescriptor {
	return wgpu.SamplerDescriptor{
		Label:         "Arasy texture sampler",
		MagFilter:     wgpu.FilterModeLinear,
		MinFilter:     wgpu.FilterModeLinear,
		MipmapFilter:  wgpu.MipmapFilterModeLinear,
		MaxAnisotropy: 16,
	}
}

func mipSamplerDescriptor() wgpu.SamplerDescriptor {
	return wgpu.SamplerDescriptor{
		Label:         "Arasy mipmap sampler",
		MagFilter:     wgpu.FilterModeLinear,
		MinFilter:     wgpu.FilterModeLinear,
		MaxAnisotropy: 1,
	}
}

func nearestSamplerDescriptor() wgpu.SamplerDescriptor {
	return wgpu.SamplerDescriptor{
		Label:         "Arasy text atlas sampler",
		MagFilter:     wgpu.FilterModeNearest,
		MinFilter:     wgpu.FilterModeNearest,
		MaxAnisotropy: 1,
	}
}

func linearTextSamplerDescriptor() wgpu.SamplerDescriptor {
	return wgpu.SamplerDescriptor{
		Label:         "Arasy smooth text atlas sampler",
		MagFilter:     wgpu.FilterModeLinear,
		MinFilter:     wgpu.FilterModeLinear,
		MaxAnisotropy: 1,
	}
}

func mipLevelCount(width, height int) int {
	maximum := width
	if height > maximum {
		maximum = height
	}
	levels := 1
	for maximum > 1 {
		maximum /= 2
		levels++
	}
	return levels
}
