//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"fmt"
	"log/slog"

	"github.com/dhannyell/arasy/internal/diagnostic"
	gpubuffer "github.com/dhannyell/arasy/internal/render/gpu/buffer"
	"github.com/dhannyell/arasy/internal/render/gpu/particlegpu"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/internal/render/slugcore"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
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

	batches map[render.TextBatchID]*nativeSlugContent
	drawn   map[slugBatchKey]*nativeSlugBatch
}

// nativeSlugContent is the content of a retained text batch. The native host draws
// only its Slug labels.
type nativeSlugContent struct {
	labels   []render.TextLabel
	revision uint64
}

// slugBatchKey has the pass, because each pass of a frame has its own camera.
type slugBatchKey struct {
	pass int
	id   render.TextBatchID
}

// nativeSlugBatch holds the Slug records of a retained text batch in one pass.
type nativeSlugBatch struct {
	revision   uint64
	camera     render.ResolvedCamera
	width      int
	height     int
	generation uint32 // of the Pack that the records refer to
	used       bool

	instances slugStorage
	bindGroup *wgpu.BindGroup
	ranges    []tess.TextBatchRange
}

func (b *nativeSlugBatch) release() {
	releaseBuffer(&b.instances.buffer)
	b.instances.capacity = 0
	if b.bindGroup != nil {
		b.bindGroup.Release()
		b.bindGroup = nil
	}
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
	for key, batch := range s.drawn {
		batch.release()
		delete(s.drawn, key)
	}
}

// applySlugBatchUpdates stores the labels. The camera of a repack is known only at
// the frame, so prepareSlugBatches does the repack.
func (r *Renderer) applySlugBatchUpdates(updates []runtime.TextBatchUpdate) {
	for _, u := range updates {
		if u.Op == runtime.BatchDelete {
			delete(r.slug.batches, u.ID)
			continue
		}
		if r.slug.batches == nil {
			r.slug.batches = make(map[render.TextBatchID]*nativeSlugContent)
		}
		content := r.slug.batches[u.ID]
		if content == nil {
			content = &nativeSlugContent{}
			r.slug.batches[u.ID] = content
		}
		content.labels = u.Labels
		content.revision++
		if r.slug.engine == nil {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "native-slug-batches-off"},
				"native renderer skips retained text because Slug text is off; set Options.GlyphOutlines to slug.Outlines")
			continue
		}
		for i := range u.Labels {
			if u.Labels[i].Strategy != render.GlyphSlug {
				r.diagnostics.Log(r.log, slog.LevelWarn,
					diagnostic.Key{Code: "native-text-batches-unsupported"},
					"native renderer skips retained text labels without the GlyphSlug strategy")
				break
			}
		}
	}
}

// prepareSlugBatches packs the batches of the frame when their content, their
// camera, or the glyph positions in the Pack changed.
func (r *Renderer) prepareSlugBatches(runs []tess.StreamRun, plans []nativePassPlan) error {
	s := &r.slug
	for _, batch := range s.drawn {
		batch.used = false
	}
	for _, run := range runs {
		if run.Kind != tess.DrawRunTextBatch {
			continue
		}
		content := s.batches[run.TextBatch.Batch]
		if content == nil {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "native-text-batch-missing", ID: uint64(run.TextBatch.Batch)},
				"native: DrawTextBatch references a missing or deleted batch; command skipped")
			continue
		}
		key := slugBatchKey{pass: run.Pass, id: run.TextBatch.Batch}
		batch := s.drawn[key]
		if batch == nil {
			if s.drawn == nil {
				s.drawn = make(map[slugBatchKey]*nativeSlugBatch)
			}
			batch = &nativeSlugBatch{revision: content.revision - 1}
			s.drawn[key] = batch
		}
		batch.used = true
		plan := plans[run.Pass]
		resolved, generation := plan.camera.Resolve(), s.engine.Pack().Generation()
		if batch.revision == content.revision && batch.camera == resolved && batch.width == plan.width &&
			batch.height == plan.height && batch.generation == generation {
			continue
		}
		packed := tess.PlanTextBatch(s.engine, plan.camera, plan.width, plan.height, content.labels, nil)
		batch.revision, batch.camera, batch.width, batch.height = content.revision, resolved, plan.width, plan.height
		batch.generation, batch.ranges = generation, packed.Ranges
		if len(packed.SlugInstances) == 0 {
			continue
		}
		grew := false
		if err := r.growSlugStorage(&batch.instances, "slug batch instances", len(packed.SlugInstances), &grew); err != nil {
			return err
		}
		if grew && batch.bindGroup != nil {
			batch.bindGroup.Release()
			batch.bindGroup = nil
		}
		if err := r.device.queue.TryWriteBuffer(batch.instances.buffer, 0, packed.SlugInstances); err != nil {
			return fmt.Errorf("upload slug batch instances: %w", err)
		}
	}
	for key, batch := range s.drawn {
		if !batch.used {
			batch.release()
			delete(s.drawn, key)
		}
	}
	return nil
}

func (r *Renderer) drawSlugBatch(
	pass *wgpu.RenderPassEncoder,
	run tess.StreamRun,
	uniformOffset uint32,
	camera render.Camera,
	viewport math2d.Rect,
	scissor *nativeScissorState,
	width, height int,
	renderTarget bool,
) {
	batch := r.slug.drawn[slugBatchKey{pass: run.Pass, id: run.TextBatch.Batch}]
	if batch == nil || batch.bindGroup == nil {
		return
	}
	x, y, sw, sh, visible := particlegpu.ScissorRectWithin(run.TextBatch.Clip, camera, viewport, 1, width, height)
	setNativeScissor(pass, scissor, uint32(x), uint32(y), uint32(sw), uint32(sh))
	if visible {
		pass.SetPipeline(pick(renderTarget, r.slug.target, r.slug.main))
		pass.SetBindGroup(0, batch.bindGroup, []uint32{uniformOffset})
		for _, rg := range batch.ranges {
			pass.Draw(uint32(rg.Count*6), 1, uint32(rg.Start*6), 0)
		}
	}
	if run.TextBatch.Clip.Width() > 0 && run.TextBatch.Clip.Height() > 0 {
		x, y, sw, sh, _ = particlegpu.ScissorRectWithin(math2d.Rect{}, camera, viewport, 1, width, height)
		setNativeScissor(pass, scissor, uint32(x), uint32(y), uint32(sw), uint32(sh))
	}
}

// uploadSlug uploads the Slug data of the frame. A failure disables Slug text, so
// the next frames draw the commands with the glyph atlas.
func (r *Renderer) uploadSlug(runs []tess.StreamRun, plans []nativePassPlan) {
	if r.slug.engine == nil {
		if r.tess.SlugMissing() {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "slug-text-off"},
				"native: GlyphSlug text uses the glyph atlas because Slug text is off; set Options.GlyphOutlines to slug.Outlines")
		}
		return
	}
	batchErr := r.prepareSlugBatches(runs, plans)
	if err := r.slug.engine.TakeErr(); err != nil {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "slug-text-add"},
			"native: Slug text is incomplete this frame", "error", err)
	}
	if batchErr == nil && r.slug.engine.Pack().Size() == 0 {
		return
	}
	err := batchErr
	if err == nil {
		err = r.createSlugPipelines()
	}
	if err == nil {
		err = r.writeSlugBuffers()
	}
	if err != nil {
		r.log.Warn("native Slug text disabled", "err", err)
		r.tess.SetSlugText(nil)
		r.slug.release()
		r.slug.engine = nil
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
	r.slug.layout, err = r.device.device.TryCreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
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
	r.slug.pipelineLayout, err = r.device.device.TryCreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{r.slug.layout},
	})
	if err != nil {
		return fmt.Errorf("create Slug pipeline layout: %w", err)
	}
	source := slugcore.WGSL()
	surface, targetFormat := r.device.config.Format, wgpu.TextureFormatRGBA8Unorm
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
		if err := r.device.queue.TryWriteBuffer(s.buffer, uint64(start), data[start:]); err != nil {
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
	for _, batch := range s.drawn {
		if batch.bindGroup != nil {
			batch.bindGroup.Release()
			batch.bindGroup = nil
		}
	}
	return r.createSlugBatchBindGroups()
}

// createSlugBatchBindGroups creates the bind groups that a batch does not have.
func (r *Renderer) createSlugBatchBindGroups() error {
	for _, batch := range r.slug.drawn {
		if batch.bindGroup != nil || batch.instances.buffer == nil {
			continue
		}
		bindGroup, err := r.createSlugBindGroup(batch.instances.buffer)
		if err != nil {
			return err
		}
		batch.bindGroup = bindGroup
	}
	return nil
}

func (r *Renderer) createSlugBindGroup(instances *wgpu.Buffer) (*wgpu.BindGroup, error) {
	s := &r.slug
	bindGroup, err := r.device.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
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
