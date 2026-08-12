//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"fmt"
	"log/slog"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/runtime"
	"github.com/oliverbestmann/webgpu/wgpu"
)

// gpuTexture is one declared texture uploaded to the GPU with its complete
// mip chain. TextureID zero remains unavailable so tess can emit its magenta
// missing-texture placeholder.
type gpuTexture struct {
	texture         *wgpu.Texture
	view            *wgpu.TextureView
	targetView      *wgpu.TextureView
	bindGroup       *wgpu.BindGroup
	targetBindGroup *wgpu.BindGroup
	spriteBindGroup *wgpu.BindGroup
	mipLevels       int
	dynamic         bool
	rtt             bool
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

func buildTextures(manifest asset.Manifest, resources textureResources, log *slog.Logger) ([]gpuTexture, tess.TexSizes) {
	if log == nil {
		log = slog.Default()
	}
	registry, err := pixel.BuildRegistry(manifest)
	if err != nil {
		log.Error("build texture registry", "error", err)
	}

	textures := make([]gpuTexture, len(registry))
	sizes := make(tess.TexSizes, len(registry))
	for _, declaration := range manifest.Textures() {
		image := registry.Lookup(declaration.ID)
		if image == nil {
			continue
		}
		bounds := image.Bounds()
		width, height := bounds.Dx(), bounds.Dy()
		if width <= 0 || height <= 0 {
			continue
		}
		mipLevels := mipLevelCount(width, height)
		texture, err := resources.createGPUTexture(image.Pix, width, height, mipLevels, declaration.Dynamic)
		if err != nil {
			log.Error("upload texture", "texture", declaration.Name, "error", err)
			continue
		}
		textures[declaration.ID] = texture
		sizes[declaration.ID] = [2]int{width, height}
	}
	return textures, sizes
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
	result := gpuTexture{texture: texture, mipLevels: levels, dynamic: dynamic}
	if err := r.writeRGBA(texture, pixels, width, height); err != nil {
		result.release()
		return gpuTexture{}, err
	}
	if err := r.generateMipmaps(texture, width, height, levels); err != nil {
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
	layout := r.texturedPipeline.GetBindGroupLayout(0)
	defer layout.Release()
	bindGroup, err := r.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Arasy textured bind group",
		Layout: layout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: view},
			{Binding: 1, Sampler: r.textureSampler},
		},
	})
	if err != nil {
		result.release()
		return gpuTexture{}, fmt.Errorf("create texture bind group: %w", err)
	}
	result.bindGroup = bindGroup
	if r.targetTexturedPipeline != nil {
		targetLayout := r.targetTexturedPipeline.GetBindGroupLayout(0)
		defer targetLayout.Release()
		targetBindGroup, err := r.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
			Label: "Arasy target textured bind group", Layout: targetLayout,
			Entries: []wgpu.BindGroupEntry{{Binding: 0, TextureView: view}, {Binding: 1, Sampler: r.textureSampler}},
		})
		if err != nil {
			result.release()
			return gpuTexture{}, fmt.Errorf("create target texture bind group: %w", err)
		}
		result.targetBindGroup = targetBindGroup
	}
	return result, nil
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
		if err := r.writeRGBA(texture.texture, update.Image.Pix, width, height); err != nil {
			return fmt.Errorf("update texture %d: %w", update.ID, err)
		}
		levels := texture.mipLevels
		if texture.rtt {
			levels = 1
		}
		if err := r.generateMipmaps(texture.texture, width, height, levels); err != nil {
			return fmt.Errorf("generate texture %d mipmaps: %w", update.ID, err)
		}
	}
	return nil
}

func (r textureResources) writeRGBA(texture *wgpu.Texture, pixels []byte, width, height int) error {
	if err := r.queue.TryWriteTexture(
		&wgpu.TexelCopyTextureInfo{Texture: texture, Aspect: wgpu.TextureAspectAll},
		pixels,
		&wgpu.TexelCopyBufferLayout{BytesPerRow: uint32(4 * width), RowsPerImage: uint32(height)},
		&wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
	); err != nil {
		return fmt.Errorf("write texture: %w", err)
	}
	return nil
}

func (r textureResources) generateMipmaps(texture *wgpu.Texture, width, height, levels int) error {
	if levels <= 1 {
		return nil
	}
	encoder, err := r.device.TryCreateCommandEncoder(nil)
	if err != nil {
		return fmt.Errorf("create mipmap command encoder: %w", err)
	}
	defer encoder.Release()

	for level := 1; level < levels; level++ {
		if err := r.generateMipLevel(encoder, texture, uint32(level)); err != nil {
			return err
		}
	}
	commandBuffer, err := encoder.TryFinish(nil)
	if err != nil {
		return fmt.Errorf("finish mipmap command encoder: %w", err)
	}
	defer commandBuffer.Release()
	r.queue.Submit(commandBuffer)
	return nil
}

func (r textureResources) generateMipLevel(encoder *wgpu.CommandEncoder, texture *wgpu.Texture, level uint32) error {
	source, err := texture.TryCreateView(&wgpu.TextureViewDescriptor{
		Dimension:       wgpu.TextureViewDimension2D,
		BaseMipLevel:    level - 1,
		MipLevelCount:   1,
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
	if t.texture != nil {
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
