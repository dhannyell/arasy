//go:build js && wasm

package wasm

import (
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/wasm/gpu"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// textBatchRange is one consecutive span of a retained text batch's glyph
// records, bound to the GPU atlas its records index into.
type textBatchRange struct {
	key       tess.AtlasKey
	start     int
	count     int
	bindGroup js.Value
}

// gpuTextBatch is one retained text batch living on the GPU: the retained
// high-level labels (kept so atlas/projection invalidation can repack
// without a new UpdateText call), persistent glyph/label storage buffers,
// consecutive draw ranges with their own bind groups, and the CPU metadata
// culling and invalidation need.
type gpuTextBatch struct {
	labels []render.TextLabel
	// dirty is set on every replace and cleared after a successful repack;
	// it forces a repack even when projection and atlas identity are
	// unchanged, since only a repack re-reads labels.
	dirty    bool
	revision uint64

	glyphBuffer js.Value
	labelBuffer js.Value
	glyphCap    int
	labelCap    int
	count       int // total packed glyph records

	ranges  []textBatchRange
	atlases map[tess.AtlasKey]*tess.Atlas

	bounds    math2d.Rect
	hasBounds bool
	projKey   textBatchProjKey
}

type passTextBatchKey struct {
	pass int
	id   render.TextBatchID
}

func (b *gpuTextBatch) destroy() {
	if !b.glyphBuffer.IsUndefined() && !b.glyphBuffer.IsNull() {
		b.glyphBuffer.Call("destroy")
	}
	if !b.labelBuffer.IsUndefined() && !b.labelBuffer.IsNull() {
		b.labelBuffer.Call("destroy")
	}
}

// residentBytes reports the GPU bytes this batch holds, for accounting.
func (b *gpuTextBatch) residentBytes() int {
	return b.glyphCap + b.labelCap
}

// applyTextBatchUpdates stores replaced labels for lazy repack at Render
// time: packing needs the frame's camera/canvas size and a finalized atlas,
// neither available yet when resources drain ahead of Build.
func (r *webgpuRenderer) applyTextBatchUpdates(updates []runtime.TextBatchUpdate) {
	for _, u := range updates {
		if u.Op == runtime.BatchDelete {
			if batch, ok := r.textBatches[u.ID]; ok {
				batch.destroy()
				delete(r.textBatches, u.ID)
			}
			continue
		}
		batch, ok := r.textBatches[u.ID]
		if !ok {
			batch = &gpuTextBatch{}
			r.textBatches[u.ID] = batch
		}
		batch.labels = u.Labels
		batch.dirty = true
		batch.revision++
	}
}

// resolveFinalTextAtlas resolves the already-current atlas for (font, px)
// without growing coverage, falling back to the built-in font like
// resolveFrameAtlases does for streaming text. It is only safe to call after
// prepTextBatches has requested coverage for this frame's dirty batches.
func (r *webgpuRenderer) resolveFinalTextAtlas(font render.FontID, px int) *tess.Atlas {
	if atlas, err := r.fontProvider.AtlasFor(font, px, ""); err == nil && atlas != nil {
		return atlas
	}
	atlas, _ := r.fontProvider.AtlasFor(0, px, "")
	return atlas
}

// prepTextBatches ensures the font atlas provider has already grown coverage
// for every rune a dirty or projection-invalidated retained text batch's
// labels need, before tess.Builder.Build resolves streaming text and
// finalizes atlas pointers for the frame. Batches that are neither dirty nor
// affected by a projection change are skipped: atlas growth only adds runes,
// so their already-covered content stays covered even when the pointer
// changes underneath them from streaming or another batch.
func (r *webgpuRenderer) prepTextBatches(ids []render.TextBatchID, cam render.Camera, w, h int) {
	if len(ids) == 0 {
		return
	}
	proj := currentTextBatchProjKey(cam, w, h)
	for _, id := range ids {
		batch, ok := r.textBatches[id]
		if !ok || len(batch.labels) == 0 {
			continue
		}
		if !batch.dirty && batch.projKey == proj {
			continue
		}
		for key, set := range tess.TextBatchCoverage(cam, w, h, batch.labels) {
			runes := make([]rune, 0, len(set))
			for ch := range set {
				runes = append(runes, ch)
			}
			r.fontProvider.AtlasFor(key.Font, key.PX, string(runes))
		}
	}
}

// finalizeTextBatches repacks every referenced batch whose content,
// projection, or resolved atlas identity changed since its last pack. It
// must run after tess.Builder.Build, once every atlas the frame needs is
// finalized: a repack only consults resolveFinalTextAtlas, it never grows
// coverage itself.
func (r *webgpuRenderer) finalizeTextBatches(ids []render.TextBatchID, cam render.Camera, w, h int) {
	if len(ids) == 0 {
		return
	}
	proj := currentTextBatchProjKey(cam, w, h)
	for _, id := range ids {
		batch, ok := r.textBatches[id]
		if !ok {
			continue
		}
		if r.textBatchNeedsRepack(batch, proj) {
			r.repackTextBatch(batch, cam, w, h, proj)
		}
	}
}

func (r *webgpuRenderer) preparePassTextBatches(passes []render.Pass, w, h int) map[passTextBatchKey]*gpuTextBatch {
	if len(passes) == 0 {
		return nil
	}

	baseIndex := len(passes) - 1
	baseIDs := collectTextBatchIDs(passes[baseIndex].Commands)
	r.finalizeTextBatches(baseIDs, passes[baseIndex].Camera, w, h)
	result := make(map[passTextBatchKey]*gpuTextBatch)
	used := make(map[passTextBatchKey]struct{})
	for passIndex, pass := range passes {
		for _, id := range collectTextBatchIDs(pass.Commands) {
			batch, ok := r.textBatches[id]
			if !ok {
				continue
			}
			if passIndex == baseIndex {
				result[passTextBatchKey{pass: passIndex, id: id}] = batch
				continue
			}
			key := passTextBatchKey{pass: passIndex, id: id}
			passBatch := r.passTextBatches[key]
			if passBatch == nil {
				passBatch = &gpuTextBatch{}
				r.passTextBatches[key] = passBatch
			}
			if passBatch.revision != batch.revision {
				passBatch.labels = append(passBatch.labels[:0], batch.labels...)
				passBatch.dirty = true
				passBatch.revision = batch.revision
			}
			proj := currentTextBatchProjKey(pass.Camera, w, h)
			if r.textBatchNeedsRepack(passBatch, proj) {
				r.repackTextBatch(passBatch, pass.Camera, w, h, proj)
			}
			used[key] = struct{}{}
			result[key] = passBatch
		}
	}
	for key, batch := range r.passTextBatches {
		if _, ok := used[key]; !ok {
			batch.destroy()
			delete(r.passTextBatches, key)
		}
	}
	return result
}

func (r *webgpuRenderer) textBatchNeedsRepack(batch *gpuTextBatch, proj textBatchProjKey) bool {
	if batch.dirty || batch.projKey != proj {
		return true
	}
	for key, oldAtlas := range batch.atlases {
		if r.resolveFinalTextAtlas(key.Font, key.PX) != oldAtlas {
			return true
		}
	}
	return false
}

// repackTextBatch packs and uploads a batch's glyph/label records once,
// grouped into consecutive draw ranges by the atlas each range resolved to.
// Steady-state frames never reach this function: finalizeTextBatches only
// calls it when content, projection, or atlas identity actually changed.
func (r *webgpuRenderer) repackTextBatch(batch *gpuTextBatch, cam render.Camera, w, h int, proj textBatchProjKey) {
	plan := tess.PlanTextBatch(cam, w, h, batch.labels, r.resolveFinalTextAtlas)
	batch.projKey = proj
	batch.dirty = false

	if len(plan.Ranges) == 0 {
		batch.ranges = nil
		batch.atlases = nil
		batch.count = 0
		batch.hasBounds = false
		return
	}

	r.ensureTextBatchBuffers(batch, len(plan.Glyphs), len(plan.Labels))
	r.queue.Call("writeBuffer", batch.glyphBuffer, 0, bytesToJS(plan.Glyphs))
	r.queue.Call("writeBuffer", batch.labelBuffer, 0, bytesToJS(plan.Labels))
	r.frameRetainedContentBytes += uint64(len(plan.Glyphs) + len(plan.Labels))

	ranges := make([]textBatchRange, len(plan.Ranges))
	for i, rg := range plan.Ranges {
		var bindGroup js.Value
		if gpuAtlas, ok := r.textAtlasFor(rg.Key); ok {
			bindGroup = r.createTextGlyphBindGroup(gpuAtlas.view, gpuAtlas.sampler, gpuAtlas.metricBuffer, batch.glyphBuffer, batch.labelBuffer)
		}
		ranges[i] = textBatchRange{key: rg.Key, start: rg.Start, count: rg.Count, bindGroup: bindGroup}
	}
	batch.ranges = ranges
	batch.atlases = plan.Atlases
	batch.count = len(plan.Glyphs) / tess.TextGlyphStride
	batch.bounds, batch.hasBounds = plan.Bounds, plan.HasBounds
}

// ensureTextBatchBuffers (re)allocates a batch's glyph/label storage buffers
// only when the packed content no longer fits, preserving a larger existing
// buffer across a smaller repack instead of shrinking it.
func (r *webgpuRenderer) ensureTextBatchBuffers(batch *gpuTextBatch, glyphSize, labelSize int) {
	if glyphSize > batch.glyphCap || batch.glyphBuffer.IsUndefined() || batch.glyphBuffer.IsNull() {
		if !batch.glyphBuffer.IsUndefined() && !batch.glyphBuffer.IsNull() {
			batch.glyphBuffer.Call("destroy")
		}
		batch.glyphCap = glyphSize
		batch.glyphBuffer = jsHandle(r.dev.CreateBuffer(gpu.BufferDesc(max(1, batch.glyphCap), gpu.BufferUsageStorage|gpu.BufferUsageCopyDst)))
	}
	if labelSize > batch.labelCap || batch.labelBuffer.IsUndefined() || batch.labelBuffer.IsNull() {
		if !batch.labelBuffer.IsUndefined() && !batch.labelBuffer.IsNull() {
			batch.labelBuffer.Call("destroy")
		}
		batch.labelCap = labelSize
		batch.labelBuffer = jsHandle(r.dev.CreateBuffer(gpu.BufferDesc(max(1, batch.labelCap), gpu.BufferUsageStorage|gpu.BufferUsageCopyDst)))
	}
}

func (r *webgpuRenderer) resolveTextBatchDrawsForPasses(drawRuns []gpuDrawRun, plans []gpuPassPlan, passBatches map[passTextBatchKey]*gpuTextBatch, w, h int) {
	r.frameVisibleLabels = 0
	for i := range drawRuns {
		if drawRuns[i].kind != tess.DrawRunTextBatch {
			continue
		}
		cmd := drawRuns[i].textBatch
		batch, ok := passBatches[passTextBatchKey{pass: drawRuns[i].pass, id: cmd.Batch}]
		if !ok {
			r.diagnostics.Log(r.log, slog.LevelWarn,
				diagnostic.Key{Code: "webgpu-text-batch-missing", ID: uint64(cmd.Batch)},
				"webgpu: DrawTextBatch references a missing or deleted batch; command skipped")
			continue
		}
		if batch.count == 0 {
			continue
		}
		plan := plans[drawRuns[i].pass]
		effective := tess.CameraForLayer(plan.camera, plan.layerCameras, drawRuns[i].layer)
		if view, ok := textBatchScreenView(effective, w, h, cmd.Clip); ok {
			bounds := offsetTextBatchBounds(batch.bounds, tess.TextScreenOffset(plan.camera, effective))
			if !renderbatch.IntersectsView(bounds, view) {
				continue
			}
		}
		drawRuns[i].text = batch
		r.frameVisibleLabels += len(batch.labels)
		r.frameRetainedDraws += len(batch.ranges)
	}
}

// offsetTextBatchBounds moves screen-space bounds by the displacement used by
// the text shader when a retained batch follows a parallax layer.
func offsetTextBatchBounds(bounds math2d.Rect, offset math2d.Vec2) math2d.Rect {
	bounds.Min = bounds.Min.Add(offset)
	bounds.Max = bounds.Max.Add(offset)
	return bounds
}
