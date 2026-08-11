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
	start     int
	count     int
	bindGroup *wgpu.BindGroup
}

// nativeSpriteBatch stores the GPU records published by one retained batch.
type nativeSpriteBatch struct {
	buffer *wgpu.Buffer
	ranges []nativeSpriteBatchRange
	count  int
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
	releaseBuffer(&b.buffer)
}

type nativeSpriteBatchPlan struct {
	records   []byte
	ranges    []nativeSpriteBatchRange
	count     int
	nonFinite int
	animated  int
}

// planNativeSpriteBatch serializes static retained records without reordering
// them. Each texture change starts a new draw range because native WebGPU
// binds one texture view per draw.
func planNativeSpriteBatch(instances []render.SpriteInstance, sizes tess.TexSizes) nativeSpriteBatchPlan {
	var plan nativeSpriteBatchPlan
	for _, inst := range instances {
		if !renderbatch.IsFiniteSpriteGeometry(inst) {
			plan.nonFinite++
			continue
		}
		if inst.Animation != (render.SpriteAnimation{}) {
			plan.animated++
		}
		texW, texH, found := sizes.Lookup(inst.Texture)
		plan.records = tess.AppendRetainedSpriteRecord(plan.records, inst, texW, texH, 0, !found)
		if len(plan.ranges) == 0 || plan.ranges[len(plan.ranges)-1].texture != inst.Texture {
			plan.ranges = append(plan.ranges, nativeSpriteBatchRange{
				texture: inst.Texture,
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
	if plan.animated > 0 {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "native-sprite-batch-animation-unsupported", ID: uint64(id)},
			"native: retained sprite animation is unsupported; base cells will be drawn",
			"instances", plan.animated)
	}

	batch := &nativeSpriteBatch{count: plan.count, ranges: plan.ranges}
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
		for i := range batch.ranges {
			texture, ok := textureFor(r.textures, batch.ranges[i].texture)
			if !ok || nativeTextureSampleView(texture) == nil {
				continue
			}
			bindGroup, err := r.createSpriteBatchBindGroup(nativeTextureSampleView(texture), batch.buffer)
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

func (r *Renderer) createSpriteBatchBindGroup(view *wgpu.TextureView, buffer *wgpu.Buffer) (*wgpu.BindGroup, error) {
	layout := r.spritePipeline.GetBindGroupLayout(0)
	defer layout.Release()
	bindGroup, err := r.device.device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "Arasy retained sprite batch bind group",
		Layout: layout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: view},
			{Binding: 1, Sampler: r.textureSampler},
			{Binding: 2, Buffer: buffer, Size: wgpu.WholeSize},
			{Binding: 3, Buffer: r.spriteUniform, Size: tess.SpriteUniformSize},
		},
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
		texture, ok := textureFor(r.textures, batch.ranges[i].texture)
		if !ok || nativeTextureSampleView(texture) == nil {
			continue
		}
		bindGroup, err := r.createSpriteBatchBindGroup(nativeTextureSampleView(texture), batch.buffer)
		if err != nil {
			return err
		}
		batch.ranges[i].bindGroup = bindGroup
	}
	return nil
}

func nativeTextureSampleView(texture gpuTexture) *wgpu.TextureView {
	if texture.rtt {
		return texture.targetView
	}
	return texture.view
}
