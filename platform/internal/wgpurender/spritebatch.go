//go:build ((windows || linux || darwin) && (amd64 || arm64)) || js

package wgpurender

import (
	"fmt"
	"log/slog"

	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/webgpu/wgpu"
)

// nativeSpriteBatch stores the GPU records published by one retained batch.
// animated batches carry a parallel animation-record buffer and bind the
// animated pipeline's uniform slot instead of the shared camera slot.
type nativeSpriteBatch struct {
	buffer     *wgpu.Buffer
	animBuffer *wgpu.Buffer
	animated   bool
	ranges     []tess.SpriteBatchRange
	// bindGroups runs parallel to ranges. A nil entry skips its range.
	bindGroups []*wgpu.BindGroup
	count      int
}

func (b *nativeSpriteBatch) release() {
	if b == nil {
		return
	}
	b.releaseBindGroups()
	releaseBuffer(&b.animBuffer)
	releaseBuffer(&b.buffer)
}

func (b *nativeSpriteBatch) releaseBindGroups() {
	for i, bindGroup := range b.bindGroups {
		if bindGroup != nil {
			bindGroup.Release()
			b.bindGroups[i] = nil
		}
	}
}

func (r *Renderer) applySpriteBatchUpdates(updates []runtime.SpriteBatchUpdate) error {
	for _, update := range updates {
		if update.Op == runtime.BatchDelete {
			r.deleteSpriteBatch(update.ID)
			continue
		}
		if err := r.replaceSpriteBatch(update.ID, update.Instances); err != nil {
			return err
		}
	}
	return nil
}

func (r *Renderer) replaceSpriteBatch(id render.SpriteBatchID, instances []render.SpriteInstance) error {
	if r.spriteBatches == nil {
		r.spriteBatches = make(map[render.SpriteBatchID]*nativeSpriteBatch)
	}
	plan := tess.PlanSpriteBatch(instances, r.texSizes, r.spriteArrayEnabled)
	if plan.NonFinite > 0 {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "native-sprite-batch-non-finite", ID: uint64(id)},
			"native: sprite batch replace dropped non-finite instances", "dropped", plan.NonFinite)
	}
	batch := &nativeSpriteBatch{count: plan.Count, ranges: plan.Ranges, animated: plan.Animated}
	if len(plan.Records) > 0 {
		var err error
		batch.buffer, err = r.newBuffer(
			"retained sprite batch",
			len(plan.Records),
			wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
		)
		if err != nil {
			return err
		}
		if err := r.device.Queue.TryWriteBuffer(batch.buffer, 0, plan.Records); err != nil {
			batch.release()
			return fmt.Errorf("upload retained sprite batch: %w", err)
		}
		if batch.animated {
			batch.animBuffer, err = r.newBuffer(
				"retained sprite batch animations",
				len(plan.Anims),
				wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
			)
			if err != nil {
				batch.release()
				return err
			}
			if err := r.device.Queue.TryWriteBuffer(batch.animBuffer, 0, plan.Anims); err != nil {
				batch.release()
				return fmt.Errorf("upload retained sprite batch animations: %w", err)
			}
		}
		if err := r.rebuildSpriteBatchBindGroups(batch); err != nil {
			batch.release()
			return err
		}
	}

	old := r.spriteBatches[id]
	r.spriteBatches[id] = batch
	old.release()
	return nil
}

func (r *Renderer) deleteSpriteBatch(id render.SpriteBatchID) {
	batch := r.spriteBatches[id]
	delete(r.spriteBatches, id)
	batch.release()
}

// createSpriteBatchBindGroup binds one draw range. Animated batches bind the
// animated pipeline's layout: the per-draw animation uniform slot plus the
// batch's animation records at binding 4.
func (r *Renderer) createSpriteBatchBindGroup(view *wgpu.TextureView, batch *nativeSpriteBatch, array bool) (*wgpu.BindGroup, error) {
	pipeline := r.spritePipelineFor(array, false, false)
	if batch.animated {
		pipeline = r.spriteBatchAnimPipelineFor(array, false)
	}
	layout := pipeline.GetBindGroupLayout(0)
	defer layout.Release()
	entries := []wgpu.BindGroupEntry{
		{Binding: 0, TextureView: view},
		{Binding: 1, Sampler: r.textureSampler},
		{Binding: 2, Buffer: batch.buffer, Size: wgpu.WholeSize},
		{Binding: 3, Buffer: r.spriteUniform, Size: tess.SpriteUniformSize},
	}
	if batch.animated {
		entries[3] = wgpu.BindGroupEntry{Binding: 3, Buffer: r.batchAnimUniform, Size: tess.SpriteBatchAnimUniformSize}
		entries = append(entries, wgpu.BindGroupEntry{Binding: 4, Buffer: batch.animBuffer, Size: wgpu.WholeSize})
	}
	bindGroup, err := r.device.Device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:   "Arasy retained sprite batch bind group",
		Layout:  layout,
		Entries: entries,
	})
	if err != nil {
		return nil, fmt.Errorf("create retained sprite batch bind group: %w", err)
	}
	return bindGroup, nil
}

func (r *Renderer) rebuildSpriteBatchBindGroups(batch *nativeSpriteBatch) error {
	if batch == nil {
		return nil
	}
	batch.releaseBindGroups()
	if len(batch.bindGroups) != len(batch.ranges) {
		batch.bindGroups = make([]*wgpu.BindGroup, len(batch.ranges))
	}
	for i, rg := range batch.ranges {
		view := r.spriteBatchRangeView(rg)
		if view == nil {
			continue
		}
		bindGroup, err := r.createSpriteBatchBindGroup(view, batch, r.spriteArrayEnabled && !rg.Missing)
		if err != nil {
			return err
		}
		batch.bindGroups[i] = bindGroup
	}
	return nil
}

// spriteBatchRangeView returns the view a range samples, or nil when the
// texture is unavailable. A missing texture never uses the array: its
// placeholder has its own size.
func (r *Renderer) spriteBatchRangeView(rg tess.SpriteBatchRange) *wgpu.TextureView {
	if rg.Missing {
		return nativeTextureSampleView(r.missingTexture)
	}
	if r.spriteArrayEnabled {
		return r.spriteArray.view
	}
	texture, ok := textureFor(r.textures, rg.Texture)
	if !ok {
		return nil
	}
	return nativeTextureSampleView(texture)
}

// spritePipelineFor selects the sprite pipeline of one draw.
func (r *Renderer) spritePipelineFor(array, additive, target bool) *wgpu.RenderPipeline {
	switch {
	case array && target:
		return pick(additive, r.targetSpriteArrayAddPipeline, r.targetSpriteArrayPipeline)
	case array:
		return pick(additive, r.spriteArrayAddPipeline, r.spriteArrayPipeline)
	case target:
		return pick(additive, r.targetSpriteAddPipeline, r.targetSpritePipeline)
	}
	return pick(additive, r.spriteAddPipeline, r.spritePipeline)
}

// spriteBatchAnimPipelineFor selects the animated batch pipeline of one draw.
// It has no additive variant.
func (r *Renderer) spriteBatchAnimPipelineFor(array, target bool) *wgpu.RenderPipeline {
	switch {
	case array && target:
		return r.targetSpriteBatchAnimArrayPipeline
	case array:
		return r.spriteBatchAnimArrayPipeline
	case target:
		return r.targetSpriteBatchAnimPipeline
	}
	return r.spriteBatchAnimPipeline
}

func nativeTextureSampleView(texture gpuTexture) *wgpu.TextureView {
	if texture.rtt && texture.arrayLayer < 0 {
		return texture.targetView
	}
	return texture.view
}
