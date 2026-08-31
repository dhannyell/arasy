//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"fmt"
	"log/slog"

	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/oliverbestmann/webgpu/wgpu"
)

// nativeSpriteBatchRange keeps a consecutive span of records that shares one
// texture. The ranges preserve painter order when a batch uses more than one
// texture.
type nativeSpriteBatchRange struct {
	texture   render.TextureID
	missing   bool
	start     int
	count     int
	bindGroup *wgpu.BindGroup
}

// nativeSpriteBatch stores the GPU records published by one retained batch.
// animated batches carry a parallel animation-record buffer and bind the
// animated pipeline's uniform slot instead of the shared camera slot.
type nativeSpriteBatch struct {
	buffer     *wgpu.Buffer
	animBuffer *wgpu.Buffer
	animated   bool
	ranges     []nativeSpriteBatchRange
	count      int
}

func (b *nativeSpriteBatch) release() {
	if b == nil {
		return
	}
	for i := range b.ranges {
		if b.ranges[i].bindGroup != nil {
			b.ranges[i].bindGroup.Release()
			b.ranges[i].bindGroup = nil
		}
	}
	releaseBuffer(&b.animBuffer)
	releaseBuffer(&b.buffer)
}

type nativeSpriteBatchPlan struct {
	records   []byte
	anims     []byte
	animated  bool
	ranges    []nativeSpriteBatchRange
	count     int
	nonFinite int
}

// planNativeSpriteBatch serializes static retained records without reordering
// them. Each texture change starts a new draw range because native WebGPU
// binds one texture view per draw. One animated instance switches the whole
// batch to the animated pipeline, so every record gets an animation record.
func planNativeSpriteBatch(instances []render.SpriteInstance, sizes tess.TexSizes) nativeSpriteBatchPlan {
	var plan nativeSpriteBatchPlan
	for _, inst := range instances {
		if inst.Animation != (render.SpriteAnimation{}) {
			plan.animated = true
			break
		}
	}
	for _, inst := range instances {
		if !renderbatch.IsFiniteSpriteGeometry(inst) {
			plan.nonFinite++
			continue
		}
		texW, texH, found := sizes.Lookup(inst.Texture)
		plan.records = tess.AppendRetainedSpriteRecord(plan.records, inst, texW, texH, 0, !found)
		if plan.animated {
			plan.anims = tess.AppendSpriteAnimRecord(plan.anims, inst, texW, texH)
		}
		texture := inst.Texture
		if !found {
			texture = 0
		}
		if len(plan.ranges) == 0 || plan.ranges[len(plan.ranges)-1].texture != texture || plan.ranges[len(plan.ranges)-1].missing != !found {
			plan.ranges = append(plan.ranges, nativeSpriteBatchRange{
				texture: texture,
				missing: !found,
				start:   plan.count,
				count:   1,
			})
		} else {
			plan.ranges[len(plan.ranges)-1].count++
		}
		plan.count++
	}
	return plan
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
	plan := planNativeSpriteBatch(instances, r.texSizes)
	if plan.nonFinite > 0 {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "native-sprite-batch-non-finite", ID: uint64(id)},
			"native: sprite batch replace dropped non-finite instances", "dropped", plan.nonFinite)
	}
	batch := &nativeSpriteBatch{count: plan.count, ranges: plan.ranges, animated: plan.animated}
	if len(plan.records) > 0 {
		var err error
		batch.buffer, err = r.newBuffer(
			"retained sprite batch",
			len(plan.records),
			wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
		)
		if err != nil {
			return err
		}
		if err := r.device.queue.TryWriteBuffer(batch.buffer, 0, plan.records); err != nil {
			batch.release()
			return fmt.Errorf("upload retained sprite batch: %w", err)
		}
		if batch.animated {
			batch.animBuffer, err = r.newBuffer(
				"retained sprite batch animations",
				len(plan.anims),
				wgpu.BufferUsageStorage|wgpu.BufferUsageCopyDst,
			)
			if err != nil {
				batch.release()
				return err
			}
			if err := r.device.queue.TryWriteBuffer(batch.animBuffer, 0, plan.anims); err != nil {
				batch.release()
				return fmt.Errorf("upload retained sprite batch animations: %w", err)
			}
		}
		for i := range batch.ranges {
			texture, ok := r.spriteBatchTexture(batch.ranges[i])
			if !ok || nativeTextureSampleView(texture) == nil {
				continue
			}
			bindGroup, err := r.createSpriteBatchBindGroup(nativeTextureSampleView(texture), batch)
			if err != nil {
				batch.release()
				return err
			}
			batch.ranges[i].bindGroup = bindGroup
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
func (r *Renderer) createSpriteBatchBindGroup(view *wgpu.TextureView, batch *nativeSpriteBatch) (*wgpu.BindGroup, error) {
	pipeline := r.spritePipeline
	if batch.animated {
		pipeline = r.spriteBatchAnimPipeline
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
	bindGroup, err := r.device.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
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
	for i := range batch.ranges {
		if batch.ranges[i].bindGroup != nil {
			batch.ranges[i].bindGroup.Release()
			batch.ranges[i].bindGroup = nil
		}
		texture, ok := r.spriteBatchTexture(batch.ranges[i])
		if !ok || nativeTextureSampleView(texture) == nil {
			continue
		}
		bindGroup, err := r.createSpriteBatchBindGroup(nativeTextureSampleView(texture), batch)
		if err != nil {
			return err
		}
		batch.ranges[i].bindGroup = bindGroup
	}
	return nil
}

func (r *Renderer) spriteBatchTexture(rg nativeSpriteBatchRange) (gpuTexture, bool) {
	if rg.missing {
		return r.missingTexture, nativeTextureSampleView(r.missingTexture) != nil
	}
	return textureFor(r.textures, rg.texture)
}

func nativeTextureSampleView(texture gpuTexture) *wgpu.TextureView {
	if texture.rtt {
		return texture.targetView
	}
	return texture.view
}
