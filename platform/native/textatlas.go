//go:build windows && amd64

package native

import (
	"fmt"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/oliverbestmann/webgpu/wgpu"
)

type gpuTextAtlas struct {
	atlas          *tess.Atlas
	texture        *wgpu.Texture
	metricBuffer   *wgpu.Buffer
	view           *wgpu.TextureView
	sampler        *wgpu.Sampler
	glyphBindGroup *wgpu.BindGroup
}

type textAtlasResources struct {
	device            *wgpu.Device
	queue             *wgpu.Queue
	textGlyphPipeline *wgpu.RenderPipeline
	glyphBuffer       *wgpu.Buffer
	labelBuffer       *wgpu.Buffer
	uniformBuffer     *wgpu.Buffer
}

func (r textAtlasResources) textAtlasFor(cache map[tess.AtlasKey]gpuTextAtlas, provider tess.AtlasProvider, key tess.AtlasKey) (gpuTextAtlas, bool, error) {
	atlas, err := provider.AtlasFor(key.Font, key.PX, "")
	if err != nil || atlas == nil {
		return gpuTextAtlas{}, false, err
	}
	if cached, ok := cache[key]; ok && cached.atlas == atlas {
		return cached, true, nil
	}
	if cached, ok := cache[key]; ok {
		cached.release()
		delete(cache, key)
	}
	resource, err := r.createTextAtlas(atlas)
	if err != nil {
		return gpuTextAtlas{}, false, err
	}
	cache[key] = resource
	return resource, true, nil
}

func (r textAtlasResources) createTextAtlas(atlas *tess.Atlas) (gpuTextAtlas, error) {
	if err := r.valid(); err != nil {
		return gpuTextAtlas{}, err
	}
	bounds := atlas.Image.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return gpuTextAtlas{}, fmt.Errorf("text atlas has invalid dimensions %dx%d", width, height)
	}
	texture, err := r.device.TryCreateTexture(&wgpu.TextureDescriptor{
		Label:         "frameforge text atlas",
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
		Dimension:     wgpu.TextureDimension2D,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		Format:        wgpu.TextureFormatR8Unorm,
		MipLevelCount: 1,
		SampleCount:   1,
	})
	if err != nil {
		return gpuTextAtlas{}, fmt.Errorf("create text atlas texture: %w", err)
	}
	result := gpuTextAtlas{atlas: atlas, texture: texture}
	if err := r.queue.TryWriteTexture(
		&wgpu.TexelCopyTextureInfo{Texture: texture, Aspect: wgpu.TextureAspectAll},
		atlas.Image.Pix,
		&wgpu.TexelCopyBufferLayout{BytesPerRow: uint32(width), RowsPerImage: uint32(height)},
		&wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
	); err != nil {
		result.release()
		return gpuTextAtlas{}, fmt.Errorf("write text atlas texture: %w", err)
	}
	view, err := texture.TryCreateView(nil)
	if err != nil {
		result.release()
		return gpuTextAtlas{}, fmt.Errorf("create text atlas view: %w", err)
	}
	result.view = view
	samplerDescriptor := nearestSamplerDescriptor()
	sampler, err := r.device.TryCreateSampler(&samplerDescriptor)
	if err != nil {
		result.release()
		return gpuTextAtlas{}, fmt.Errorf("create text atlas sampler: %w", err)
	}
	result.sampler = sampler
	metricBytes := atlas.GlyphMetricStream()
	if len(metricBytes) == 0 || len(metricBytes)%tess.TextMetricStride != 0 {
		result.release()
		return gpuTextAtlas{}, fmt.Errorf("text atlas has invalid metric stream")
	}
	metricBuffer, err := r.device.TryCreateBuffer(&wgpu.BufferDescriptor{
		Label: "frameforge text metrics",
		Usage: wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst,
		Size:  uint64(len(metricBytes)),
	})
	if err != nil {
		result.release()
		return gpuTextAtlas{}, fmt.Errorf("create text metric buffer: %w", err)
	}
	result.metricBuffer = metricBuffer
	if err := r.queue.TryWriteBuffer(metricBuffer, 0, metricBytes); err != nil {
		result.release()
		return gpuTextAtlas{}, fmt.Errorf("write text metric buffer: %w", err)
	}
	bindGroup, err := r.createTextGlyphBindGroup(view, sampler, metricBuffer)
	if err != nil {
		result.release()
		return gpuTextAtlas{}, err
	}
	result.glyphBindGroup = bindGroup
	return result, nil
}

func (r textAtlasResources) createTextGlyphBindGroup(view *wgpu.TextureView, sampler *wgpu.Sampler, metrics *wgpu.Buffer) (*wgpu.BindGroup, error) {
	layout := r.textGlyphPipeline.GetBindGroupLayout(0)
	defer layout.Release()
	bindGroup, err := r.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "frameforge text glyph bind group",
		Layout: layout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: view},
			{Binding: 1, Sampler: sampler},
			{Binding: 2, Buffer: r.glyphBuffer, Size: wgpu.WholeSize},
			{Binding: 3, Buffer: r.labelBuffer, Size: wgpu.WholeSize},
			{Binding: 4, Buffer: metrics, Size: wgpu.WholeSize},
			{Binding: 5, Buffer: r.uniformBuffer, Size: wgpu.WholeSize},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create text glyph bind group: %w", err)
	}
	return bindGroup, nil
}

func (r textAtlasResources) valid() error {
	if r.device == nil || r.queue == nil || r.textGlyphPipeline == nil || r.glyphBuffer == nil || r.labelBuffer == nil || r.uniformBuffer == nil {
		return fmt.Errorf("text atlas resources are incomplete")
	}
	return nil
}

func (atlas gpuTextAtlas) release() {
	if atlas.glyphBindGroup != nil {
		atlas.glyphBindGroup.Release()
	}
	if atlas.metricBuffer != nil {
		atlas.metricBuffer.Destroy()
		atlas.metricBuffer.Release()
	}
	if atlas.sampler != nil {
		atlas.sampler.Release()
	}
	if atlas.view != nil {
		atlas.view.Release()
	}
	if atlas.texture != nil {
		atlas.texture.Destroy()
		atlas.texture.Release()
	}
}
