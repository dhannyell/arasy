//go:build ((windows || linux || darwin) && (amd64 || arm64)) || js

package wgpurender

import (
	"fmt"
	"log/slog"

	"github.com/dhannyell/arasy/internal/diagnostic"
	gpubuffer "github.com/dhannyell/arasy/internal/render/gpu/buffer"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/internal/render/slugcore"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/webgpu/wgpu"
)

// slugStorage is one storage buffer of the Slug shader.
type slugStorage struct {
	buffer   *wgpu.Buffer
	capacity int
}

// nativeSlug holds the GPU state of Slug text. The first Slug run creates it, so a
// game that does not use Slug text does not compile the shader.
type nativeSlug struct {
	engine slugcore.Engine

	pipelineLayout                   *wgpu.PipelineLayout
	layout                           *wgpu.BindGroupLayout
	main, mainAdd, target, targetAdd *wgpu.RenderPipeline
	curves, bands, layers, instances slugStorage
	bindGroup                        *wgpu.BindGroup
}

func (s *nativeSlug) endFrame() {
	if s.engine != nil {
		s.engine.EndFrame()
	}
}

func (s *nativeSlug) release() {
	releasePipeline(&s.main)
	releasePipeline(&s.mainAdd)
	releasePipeline(&s.target)
	releasePipeline(&s.targetAdd)
	if s.pipelineLayout != nil {
		s.pipelineLayout.Release()
		s.pipelineLayout = nil
	}
	if s.layout != nil {
		s.layout.Release()
		s.layout = nil
	}
	if s.bindGroup != nil {
		s.bindGroup.Release()
		s.bindGroup = nil
	}
	for _, storage := range []*slugStorage{&s.curves, &s.bands, &s.layers, &s.instances} {
		releaseBuffer(&storage.buffer)
		storage.capacity = 0
	}
}

// generation returns the generation of the Pack, or 0 if Slug text is off.
func (s *nativeSlug) generation() uint32 {
	if s.engine == nil {
		return 0
	}
	return s.engine.Pack().Generation()
}

// warnSlugBatchOff reports GlyphSlug labels that use the atlas, because Slug text is off.
func (r *Renderer) warnSlugBatchOff(labels []render.TextLabel) {
	if r.slug.engine != nil {
		return
	}
	for i := range labels {
		if labels[i].Strategy == render.GlyphSlug {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "slug-text-off"},
				"native: GlyphSlug text uses the glyph atlas because Slug text is off; set Options.GlyphOutlines to slug.Outlines")
			return
		}
	}
}

// writeSlugBatch uploads the Slug records of a retained text batch.
func (r *Renderer) writeSlugBatch(batch *gpuTextBatch, instances []byte) error {
	if len(instances) == 0 {
		return nil
	}
	grew := false
	if err := r.growSlugStorage(&batch.slugInstances, "slug batch instances", len(instances), &grew); err != nil {
		return err
	}
	if grew && batch.slugBindGroup != nil {
		batch.slugBindGroup.Release()
		batch.slugBindGroup = nil
	}
	if err := r.device.Queue.TryWriteBuffer(batch.slugInstances.buffer, 0, instances); err != nil {
		return fmt.Errorf("upload slug batch instances: %w", err)
	}
	return nil
}

func (r *Renderer) forEachTextBatch(visit func(*gpuTextBatch)) {
	for _, batch := range r.textBatches {
		visit(batch)
	}
	for _, batch := range r.passTextBatches {
		visit(batch)
	}
}

// uploadSlug uploads the Slug data of the frame. A failure disables Slug text, so
// the next frames draw the commands with the glyph atlas.
func (r *Renderer) uploadSlug() {
	if r.slug.engine == nil {
		if r.tess.SlugMissing() {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "slug-text-off"},
				"native: GlyphSlug text uses the glyph atlas because Slug text is off; set Options.GlyphOutlines to slug.Outlines")
		}
		return
	}
	if err := r.slug.engine.TakeErr(); err != nil {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "slug-text-add"},
			"native: Slug text is incomplete this frame", "error", err)
	}
	if r.slug.engine.Pack().Size() == 0 {
		return
	}
	err := r.createSlugPipelines()
	if err == nil {
		err = r.writeSlugBuffers()
	}
	if err != nil {
		r.log.Warn("native Slug text disabled", "err", err)
		r.tess.SetSlugText(nil)
		r.slug.release()
		r.slug.engine = nil
		r.forEachTextBatch((*gpuTextBatch).releaseSlug)
	}
}

func (r *Renderer) createSlugPipelines() error {
	if r.slug.main != nil {
		return nil
	}
	storage := func(binding uint32, visibility wgpu.ShaderStage) wgpu.BindGroupLayoutEntry {
		return wgpu.BindGroupLayoutEntry{Binding: binding, Visibility: visibility, Buffer: wgpu.BufferBindingLayout{
			Type: wgpu.BufferBindingTypeReadOnlyStorage,
		}}
	}
	var err error
	r.slug.layout, err = r.device.Device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []wgpu.BindGroupLayoutEntry{
			{Binding: 0, Visibility: wgpu.ShaderStageVertex, Buffer: wgpu.BufferBindingLayout{
				Type: wgpu.BufferBindingTypeUniform, HasDynamicOffset: true, MinBindingSize: tess.SpriteUniformSize,
			}},
			storage(1, wgpu.ShaderStageFragment),
			storage(2, wgpu.ShaderStageFragment),
			storage(3, wgpu.ShaderStageVertex|wgpu.ShaderStageFragment),
			storage(4, wgpu.ShaderStageVertex),
		},
	})
	if err != nil {
		return fmt.Errorf("create Slug bind group layout: %w", err)
	}
	r.slug.pipelineLayout, err = r.device.Device.TryCreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{r.slug.layout},
	})
	if err != nil {
		return fmt.Errorf("create Slug pipeline layout: %w", err)
	}
	source := slugcore.WGSL()
	surface, targetFormat := r.device.Config.Format, wgpu.TextureFormatRGBA8Unorm
	for _, p := range []struct {
		out     **wgpu.RenderPipeline
		label   string
		format  wgpu.TextureFormat
		samples uint32
		blend   *wgpu.BlendState
	}{
		{&r.slug.main, "slug text", surface, r.msaaSamples, alphaBlend()},
		{&r.slug.mainAdd, "slug text additive", surface, r.msaaSamples, additiveStraightBlend()},
		{&r.slug.target, "target slug text", targetFormat, 1, alphaBlend()},
		{&r.slug.targetAdd, "target slug text additive", targetFormat, 1, additiveStraightBlend()},
	} {
		if *p.out, err = r.createPipeline(p.label, source, p.format, p.samples, nil, p.blend, r.slug.pipelineLayout); err != nil {
			return err
		}
	}
	return nil
}

// writeSlugBuffers writes the data that the Pack added since the last frame, and
// all of the data of a buffer that grew: a new buffer has no old data.
func (r *Renderer) writeSlugBuffers() error {
	pack := r.slug.engine.Pack()
	dirty := pack.TakeDirty()
	grew := false
	write := func(s *slugStorage, label string, data []byte, start int) error {
		if len(data) == 0 {
			return r.growSlugStorage(s, label, gpubuffer.InitialCapacity, &grew)
		}
		wasBuffer := s.buffer
		if err := r.growSlugStorage(s, label, len(data), &grew); err != nil {
			return err
		}
		if s.buffer != wasBuffer {
			start = 0
		}
		if start >= len(data) {
			return nil
		}
		if err := r.device.Queue.TryWriteBuffer(s.buffer, uint64(start), data[start:]); err != nil {
			return fmt.Errorf("upload %s: %w", label, err)
		}
		return nil
	}
	const word = 4 // bytes in one element of each Pack slice
	if err := write(&r.slug.curves, "slug curves", wgpu.ToBytes(pack.Curves()), dirty.Curves.Start*word); err != nil {
		return err
	}
	if err := write(&r.slug.bands, "slug bands", wgpu.ToBytes(pack.Bands()), dirty.Bands.Start*word); err != nil {
		return err
	}
	if err := write(&r.slug.layers, "slug layers", wgpu.ToBytes(pack.Layers()), dirty.Layers.Start*word); err != nil {
		return err
	}
	if err := write(&r.slug.instances, "slug instances", r.tess.SlugInstanceStream(), 0); err != nil {
		return err
	}
	if grew || r.slug.bindGroup == nil {
		return r.rebuildSlugBindGroup()
	}
	return r.createSlugBatchBindGroups()
}

func (r *Renderer) growSlugStorage(s *slugStorage, label string, required int, grew *bool) error {
	if s.buffer != nil && required <= s.capacity {
		return nil
	}
	buffer, capacity, err := r.growBuffer(s.buffer, s.capacity, max(required, gpubuffer.InitialCapacity), label,
		wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst)
	if err != nil {
		return err
	}
	s.buffer, s.capacity = buffer, capacity
	*grew = true
	return nil
}

// rebuildSlugBindGroup does nothing before the first Slug run. The sprite uniform
// buffer can change at any frame, so it also calls this function.
func (r *Renderer) rebuildSlugBindGroup() error {
	s := &r.slug
	if s.layout == nil || s.instances.buffer == nil {
		return nil
	}
	bindGroup, err := r.createSlugBindGroup(s.instances.buffer)
	if err != nil {
		return err
	}
	if s.bindGroup != nil {
		s.bindGroup.Release()
	}
	s.bindGroup = bindGroup
	r.forEachTextBatch(func(batch *gpuTextBatch) {
		if batch.slugBindGroup != nil {
			batch.slugBindGroup.Release()
			batch.slugBindGroup = nil
		}
	})
	return r.createSlugBatchBindGroups()
}

// createSlugBatchBindGroups creates the bind groups that a batch does not have.
func (r *Renderer) createSlugBatchBindGroups() error {
	var err error
	r.forEachTextBatch(func(batch *gpuTextBatch) {
		if err != nil || batch.slugBindGroup != nil || batch.slugInstances.buffer == nil {
			return
		}
		batch.slugBindGroup, err = r.createSlugBindGroup(batch.slugInstances.buffer)
	})
	return err
}

func (r *Renderer) createSlugBindGroup(instances *wgpu.Buffer) (*wgpu.BindGroup, error) {
	s := &r.slug
	bindGroup, err := r.device.Device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Arasy slug text bind group",
		Layout: s.layout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: r.spriteUniform, Size: tess.SpriteUniformSize},
			{Binding: 1, Buffer: s.curves.buffer, Size: wgpu.WholeSize},
			{Binding: 2, Buffer: s.bands.buffer, Size: wgpu.WholeSize},
			{Binding: 3, Buffer: s.layers.buffer, Size: wgpu.WholeSize},
			{Binding: 4, Buffer: instances, Size: wgpu.WholeSize},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create Slug bind group: %w", err)
	}
	return bindGroup, nil
}
