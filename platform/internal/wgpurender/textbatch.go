//go:build ((windows || linux || darwin) && (amd64 || arm64)) || js

package wgpurender

import (
	"fmt"
	"log/slog"

	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/internal/render/frameplan"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/webgpu/wgpu"
)

// textBatchRange is one consecutive span of glyph records bound to the atlas
// that the records index. A nil bind group skips the range.
type textBatchRange struct {
	slug      bool // the range reads the Slug records of the batch
	key       tess.AtlasKey
	start     int
	count     int
	bindGroup *wgpu.BindGroup
}

// gpuTextBatch is one retained text batch on the GPU. It keeps the labels so
// a projection or atlas change can repack without a new UpdateText call.
type gpuTextBatch struct {
	labels []render.TextLabel
	// dirty forces a repack after a replace, because only a repack reads labels.
	dirty    bool
	revision uint64

	glyphBuffer *wgpu.Buffer
	labelBuffer *wgpu.Buffer
	glyphCap    int
	labelCap    int
	count       int // packed glyph records

	ranges  []textBatchRange
	atlases map[tess.AtlasKey]*tess.Atlas

	bounds    math2d.Rect
	hasBounds bool
	projKey   tess.TextBatchProjKey

	// The Slug records. uploadSlug creates the bind group, because it also reads the
	// buffers of the Pack.
	slugInstances  slugStorage
	slugBindGroup  *wgpu.BindGroup
	slugGeneration uint32 // of the Pack that the records refer to
}

// passTextBatchKey selects the private copy of a batch that a second pass
// packs against its own camera.
type passTextBatchKey struct {
	pass int
	id   render.TextBatchID
}

func (b *gpuTextBatch) release() {
	b.releaseRanges()
	releaseBuffer(&b.glyphBuffer)
	releaseBuffer(&b.labelBuffer)
	b.releaseSlug()
}

func (b *gpuTextBatch) releaseSlug() {
	releaseBuffer(&b.slugInstances.buffer)
	b.slugInstances.capacity = 0
	if b.slugBindGroup != nil {
		b.slugBindGroup.Release()
		b.slugBindGroup = nil
	}
}

func (b *gpuTextBatch) releaseRanges() {
	for i := range b.ranges {
		if b.ranges[i].bindGroup != nil {
			b.ranges[i].bindGroup.Release()
			b.ranges[i].bindGroup = nil
		}
	}
	b.ranges = nil
}

// applyTextBatchUpdates stores labels for a repack at render time. A repack
// needs the frame camera and a final atlas, which do not exist yet here.
func (r *Renderer) applyTextBatchUpdates(updates []runtime.TextBatchUpdate) {
	for _, update := range updates {
		if update.Op == runtime.BatchDelete {
			if batch, ok := r.textBatches[update.ID]; ok {
				batch.release()
				delete(r.textBatches, update.ID)
			}
			continue
		}
		batch, ok := r.textBatches[update.ID]
		if !ok {
			batch = &gpuTextBatch{}
			r.textBatches[update.ID] = batch
		}
		batch.labels = update.Labels
		batch.dirty = true
		batch.revision++
	}
}

// resolveFinalTextAtlas returns the current atlas without growing it, with the
// fallback that streaming text uses. It is safe only after prepTextBatches.
func (r *Renderer) resolveFinalTextAtlas(font render.FontID, px int) *tess.Atlas {
	if atlas, err := r.fontProvider.AtlasFor(font, px, ""); err == nil && atlas != nil {
		return atlas
	}
	atlas, _ := r.fontProvider.AtlasFor(0, px, "")
	return atlas
}

// prepTextBatches grows the atlases for each batch that needs a repack, before
// the builder finalizes the atlas pointers of the frame. Other batches skip
// this step: growth only adds runes, so their content stays covered.
func (r *Renderer) prepTextBatches(ids []render.TextBatchID, camera render.Camera, width, height int) {
	if len(ids) == 0 {
		return
	}
	proj := tess.CurrentTextBatchProjKey(camera, width, height)
	for _, id := range ids {
		batch, ok := r.textBatches[id]
		if !ok || len(batch.labels) == 0 || (!batch.dirty && batch.projKey == proj && batch.slugGeneration == r.slug.generation()) {
			continue
		}
		for key, set := range tess.TextBatchCoverage(r.slug.engine, camera, width, height, batch.labels) {
			runes := make([]rune, 0, len(set))
			for ch := range set {
				runes = append(runes, ch)
			}
			if _, err := r.fontProvider.AtlasFor(key.Font, key.PX, string(runes)); err != nil {
				r.diagnostics.Log(r.log, slog.LevelWarn,
					diagnostic.Key{Code: "text-batch-coverage", ID: uint64(id)},
					"renderer: text batch atlas coverage failed", "err", err)
			}
		}
	}
}

// preparePassTextBatches repacks the batches of this frame into passTextResult.
// The base pass owns a batch that it draws; each other pass that draws the
// batch gets a private copy, because records are in screen space.
func (r *Renderer) preparePassTextBatches(passes []frameplan.Pass, built []tess.BuiltPass) error {
	clear(r.passTextPrimary)
	clear(r.passTextResult)
	clear(r.passTextUsed)
	if len(passes) == 0 {
		return nil
	}
	for passIndex, ids := range r.passTextIDs {
		for _, id := range ids {
			if _, exists := r.passTextPrimary[id]; !exists {
				r.passTextPrimary[id] = passIndex
			}
		}
	}
	for _, id := range r.passTextIDs[len(passes)-1] {
		r.passTextPrimary[id] = len(passes) - 1
	}
	for passIndex, pass := range passes {
		width, height := built[passIndex].Width, built[passIndex].Height
		proj := tess.CurrentTextBatchProjKey(pass.Camera, width, height)
		for _, id := range r.passTextIDs[passIndex] {
			batch, ok := r.textBatches[id]
			if !ok {
				continue
			}
			key := passTextBatchKey{pass: passIndex, id: id}
			target := batch
			if r.passTextPrimary[id] != passIndex {
				target = r.passTextBatches[key]
				if target == nil {
					target = &gpuTextBatch{}
					r.passTextBatches[key] = target
				}
				if target.revision != batch.revision {
					target.labels = append(target.labels[:0], batch.labels...)
					target.dirty = true
					target.revision = batch.revision
				}
				r.passTextUsed[key] = struct{}{}
			}
			if r.textBatchNeedsRepack(target, proj) {
				if err := r.repackTextBatch(target, pass.Camera, width, height, proj); err != nil {
					return err
				}
			}
			r.passTextResult[key] = target
		}
	}
	for key, batch := range r.passTextBatches {
		if _, ok := r.passTextUsed[key]; !ok {
			batch.release()
			delete(r.passTextBatches, key)
		}
	}
	return nil
}

func (r *Renderer) textBatchNeedsRepack(batch *gpuTextBatch, proj tess.TextBatchProjKey) bool {
	// A Pack reset moves the glyphs, so the old records would draw wrong glyphs.
	if batch.dirty || batch.projKey != proj || batch.slugGeneration != r.slug.generation() {
		return true
	}
	for key, atlas := range batch.atlases {
		if r.resolveFinalTextAtlas(key.Font, key.PX) != atlas {
			return true
		}
	}
	return false
}

// repackTextBatch packs and uploads the records once. A steady frame does not
// reach this function.
func (r *Renderer) repackTextBatch(batch *gpuTextBatch, camera render.Camera, width, height int, proj tess.TextBatchProjKey) error {
	r.warnSlugBatchOff(batch.labels)
	plan := tess.PlanTextBatch(r.slug.engine, camera, width, height, batch.labels, r.resolveFinalTextAtlas)
	batch.slugGeneration = r.slug.generation()
	batch.projKey = proj
	batch.dirty = false
	batch.releaseRanges()
	batch.atlases = nil
	batch.count = 0
	batch.hasBounds = false
	if len(plan.Ranges) == 0 {
		return nil
	}

	if err := r.ensureTextBatchBuffers(batch, len(plan.Glyphs), len(plan.Labels)); err != nil {
		return err
	}
	if len(plan.Glyphs) > 0 {
		if err := r.device.Queue.TryWriteBuffer(batch.glyphBuffer, 0, plan.Glyphs); err != nil {
			return fmt.Errorf("upload retained text glyphs: %w", err)
		}
		if err := r.device.Queue.TryWriteBuffer(batch.labelBuffer, 0, plan.Labels); err != nil {
			return fmt.Errorf("upload retained text labels: %w", err)
		}
	}
	if err := r.writeSlugBatch(batch, plan.SlugInstances); err != nil {
		return err
	}
	r.frameRetainedContentBytes += uint64(len(plan.Glyphs) + len(plan.Labels) + len(plan.SlugInstances))

	batch.ranges = make([]textBatchRange, len(plan.Ranges))
	for i, rg := range plan.Ranges {
		batch.ranges[i] = textBatchRange{slug: rg.Slug, key: rg.Key, start: rg.Start, count: rg.Count}
	}
	if err := r.rebuildTextBatchBindGroups(batch); err != nil {
		return err
	}
	batch.atlases = plan.Atlases
	batch.count = len(plan.Glyphs)/tess.TextGlyphStride + len(plan.SlugInstances)/tess.SlugInstanceStride
	batch.bounds, batch.hasBounds = plan.Bounds, plan.HasBounds
	return nil
}

// rebuildTextBatchBindGroups binds each range to the current atlas and the
// current uniform arena. The packed records do not change.
func (r *Renderer) rebuildTextBatchBindGroups(batch *gpuTextBatch) error {
	resources := r.textAtlasResources()
	for i := range batch.ranges {
		if batch.ranges[i].slug {
			continue
		}
		atlas, ok, err := resources.textAtlasFor(r.textAtlases, r.fontProvider, batch.ranges[i].key)
		if err != nil {
			return err
		}
		var bindGroup *wgpu.BindGroup
		if ok {
			bindGroup, err = resources.createTextGlyphBindGroup(atlas.view, atlas.sampler, atlas.metricBuffer, batch.glyphBuffer, batch.labelBuffer)
			if err != nil {
				return err
			}
		}
		if batch.ranges[i].bindGroup != nil {
			batch.ranges[i].bindGroup.Release()
		}
		batch.ranges[i].bindGroup = bindGroup
	}
	return nil
}

// rebuildRetainedTextBindGroups moves every retained text range to a new
// uniform arena. A range that keeps the old arena draws from a destroyed buffer.
func (r *Renderer) rebuildRetainedTextBindGroups() error {
	for _, batch := range r.textBatches {
		if err := r.rebuildTextBatchBindGroups(batch); err != nil {
			return err
		}
	}
	for _, batch := range r.passTextBatches {
		if err := r.rebuildTextBatchBindGroups(batch); err != nil {
			return err
		}
	}
	return nil
}

// ensureTextBatchBuffers replaces a buffer only when the content does not fit.
func (r *Renderer) ensureTextBatchBuffers(batch *gpuTextBatch, glyphSize, labelSize int) error {
	var err error
	if batch.glyphBuffer == nil || glyphSize > batch.glyphCap {
		releaseBuffer(&batch.glyphBuffer)
		batch.glyphCap = glyphSize
		batch.glyphBuffer, err = r.newBuffer("retained text glyphs", max(1, glyphSize), wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst)
		if err != nil {
			return err
		}
	}
	if batch.labelBuffer == nil || labelSize > batch.labelCap {
		releaseBuffer(&batch.labelBuffer)
		batch.labelCap = labelSize
		batch.labelBuffer, err = r.newBuffer("retained text labels", max(1, labelSize), wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst)
		if err != nil {
			return err
		}
	}
	return nil
}

// resolveTextBatchRuns fills textBatchRuns with the batch each run draws. A nil
// entry skips the run: its batch is missing, empty or out of view.
func (r *Renderer) resolveTextBatchRuns(runs []tess.StreamRun, plans []passPlan) {
	if cap(r.textBatchRuns) < len(runs) {
		r.textBatchRuns = make([]*gpuTextBatch, len(runs))
	} else {
		r.textBatchRuns = r.textBatchRuns[:len(runs)]
		clear(r.textBatchRuns)
	}
	r.frameVisibleLabels = 0
	for index, run := range runs {
		if run.Kind != tess.DrawRunTextBatch {
			continue
		}
		cmd := run.TextBatch
		batch, ok := r.passTextResult[passTextBatchKey{pass: run.Pass, id: cmd.Batch}]
		if !ok {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "text-batch-missing", ID: uint64(cmd.Batch)},
				"renderer: DrawTextBatch references a missing or deleted batch; command skipped")
			continue
		}
		if batch.count == 0 {
			continue
		}
		plan := plans[run.Pass]
		camera := tess.CameraForLayer(plan.camera, plan.layerCameras, run.Layer)
		if view, ok := tess.TextBatchScreenView(camera, plan.width, plan.height, cmd.Clip); ok {
			// The text shader moves a batch that follows a parallax layer.
			offset := tess.TextScreenOffset(plan.camera, camera)
			bounds := math2d.Rect{Min: batch.bounds.Min.Add(offset), Max: batch.bounds.Max.Add(offset)}
			if !renderbatch.IntersectsView(bounds, view) {
				continue
			}
		}
		r.textBatchRuns[index] = batch
		r.frameVisibleLabels += len(batch.labels)
		r.frameRetainedDraws += len(batch.ranges)
	}
}
