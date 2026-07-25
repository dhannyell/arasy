package wasm

import (
	"encoding/binary"
	"math"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// Retained sprite batch GPU layouts. The record stride is fixed by
// tess.SpriteStride; the animation record and uniform layouts must match the
// WGSL SpriteAnim/BatchUniforms structs. tess.SpriteUniformSize is the shared
// camera uniform layout used by every sprite/text pipeline.
const (
	spriteBatchAnimStride      = 32
	spriteBatchAnimUniformSize = 80
)

type spriteUniformWriter struct {
	dst    []byte
	offset int
}

func (w *spriteUniformWriter) f32(value float64) {
	binary.LittleEndian.PutUint32(w.dst[w.offset:], math.Float32bits(float32(value)))
	w.offset += 4
}

func (w *spriteUniformWriter) u32(value uint32) {
	binary.LittleEndian.PutUint32(w.dst[w.offset:], value)
	w.offset += 4
}

func (w *spriteUniformWriter) skip(bytes int) { w.offset += bytes }

// spriteBatchRange is one consecutive span of batch records sharing a bind
// target. The array path binds one 2d-array group for every non-missing
// texture (texture is zero); the fallback binds per consecutive texture.
// magenta selects the 1x1 missing-texture placeholder texture.
type spriteBatchRange struct {
	texture render.TextureID
	magenta bool
	start   int
	count   int
}

// spriteBatchPlan is the CPU-side result of packing one retained sprite
// batch: the 64-byte records to upload once, the optional animation records,
// consecutive draw ranges, and the metadata needed for culling and
// accounting. Content is render-only.
type spriteBatchPlan struct {
	records     []byte
	anims       []byte
	animated    bool
	ranges      []spriteBatchRange
	bounds      math2d.Rect
	hasBounds   bool
	maxBounce   float64
	hasRotation bool
	count       int
	nonFinite   int
}

// planSpriteBatch packs instances into retained records without reordering:
// Src is normalized (zero = whole texture), tint and texture layer are
// packed, and the clip sentinel stays always open (the retained clip is the
// command's draw-time scissor). Non-finite instances are dropped (counted in
// nonFinite so the adapter can warn once). Instances whose texture is
// missing join a magenta range with a forced white tint, preserving the
// streaming placeholder without solid-geometry fallbacks.
func planSpriteBatch(instances []render.SpriteInstance, sizes tess.TexSizes, arrayEnabled bool) spriteBatchPlan {
	plan := spriteBatchPlan{}
	animated := false
	for i := range instances {
		if instances[i].Animation != (render.SpriteAnimation{}) {
			animated = true
			break
		}
	}
	plan.animated = animated

	type rangeKey struct {
		texture render.TextureID
		magenta bool
	}
	lastKey := rangeKey{}
	hasLast := false

	for i := range instances {
		inst := instances[i]
		if !renderbatch.IsFiniteSpriteGeometry(inst) {
			plan.nonFinite++
			continue
		}
		plan.hasRotation = plan.hasRotation || inst.Rotation != 0

		texW, texH, found := sizes.Lookup(inst.Texture)
		magenta := !found
		layer := uint32(0)
		if arrayEnabled && !magenta {
			layer = uint32(inst.Texture - 1)
		}
		plan.records = tess.AppendRetainedSpriteRecord(plan.records, inst, texW, texH, layer, magenta)

		if animated {
			plan.anims = appendSpriteBatchAnimRecord(plan.anims, inst, texW, texH)
		}

		if b := float64(inst.Animation.Bounce); math.Abs(b) > plan.maxBounce {
			plan.maxBounce = math.Abs(b)
		}
		if bounds, ok := renderbatch.SpriteBounds(inst); ok {
			if !plan.hasBounds {
				plan.bounds, plan.hasBounds = bounds, true
			} else {
				plan.bounds.Min.X = min(plan.bounds.Min.X, bounds.Min.X)
				plan.bounds.Min.Y = min(plan.bounds.Min.Y, bounds.Min.Y)
				plan.bounds.Max.X = max(plan.bounds.Max.X, bounds.Max.X)
				plan.bounds.Max.Y = max(plan.bounds.Max.Y, bounds.Max.Y)
			}
		}

		key := rangeKey{texture: inst.Texture, magenta: magenta}
		if arrayEnabled && !magenta {
			key.texture = 0
		}
		recordIndex := plan.count
		if hasLast && key == lastKey {
			plan.ranges[len(plan.ranges)-1].count++
		} else {
			plan.ranges = append(plan.ranges, spriteBatchRange{
				texture: key.texture, magenta: key.magenta,
				start: recordIndex, count: 1,
			})
			lastKey, hasLast = key, true
		}
		plan.count++
	}
	return plan
}

// appendSpriteBatchAnimRecord serializes the per-instance animation data the
// animated sprite shaders read next to the base record: flip-book grid
// (validated against the texture; an invalid sequence packs Frames=0, the
// same static-Src degradation as the CPU reference), rate, phase and bounce.
func appendSpriteBatchAnimRecord(dst []byte, inst render.SpriteInstance, texW, texH int) []byte {
	anim := inst.Animation
	frames := uint32(0)
	cellsPerRow := uint32(0)
	baseIndex := uint32(0)
	if cpr, base, ok := renderbatch.FlipBookCells(inst, texW, texH); ok {
		frames = uint32(anim.Frames)
		cellsPerRow = uint32(cpr)
		baseIndex = uint32(base)
	}
	var buf [spriteBatchAnimStride]byte
	writer := spriteUniformWriter{dst: buf[:]}
	writer.u32(frames)
	writer.u32(cellsPerRow)
	writer.u32(baseIndex)
	writer.skip(4) // alignment pad
	writer.f32(float64(anim.Rate))
	writer.f32(float64(anim.Phase))
	writer.f32(float64(anim.Bounce))
	return append(dst, buf[:]...)
}

// spriteBatchInflatedBounds returns the batch bounds inflated by the
// command's animation, for whole-batch culling. ok is false when the batch
// has no finite geometry at all.
func spriteBatchInflatedBounds(bounds math2d.Rect, hasBounds bool, maxBounce float64, anim render.SpriteBatchAnimation) (math2d.Rect, bool) {
	if !hasBounds {
		return math2d.Rect{}, false
	}
	inflation := renderbatch.MaxVerticalInflation(maxBounce, anim)
	bounds.Min.Y -= inflation
	bounds.Max.Y += inflation
	return bounds, true
}

// uniformArena hands out aligned uniform slots for one frame's animated
// batch commands. Slots are never reused within a frame: queue.writeBuffer
// calls land before the command buffer, so two draws sharing one slot would
// both observe the last packed value.
type uniformArena struct {
	dataSize int
	slotSize int
	slots    int
	used     int
}

func newUniformArena(dataSize, alignment, slots int) uniformArena {
	slotSize := dataSize
	if rem := slotSize % alignment; rem != 0 {
		slotSize += alignment - rem
	}
	return uniformArena{dataSize: dataSize, slotSize: slotSize, slots: slots}
}

// alloc reserves the next slot and returns its byte offset. ok is false when
// the arena is full; the caller grows the backing buffer and rebuilds the
// bind groups that reference it.
func (a *uniformArena) alloc() (offset int, ok bool) {
	if a.used >= a.slots {
		return 0, false
	}
	offset = a.used * a.slotSize
	a.used++
	return offset, true
}

func (a *uniformArena) reset() {
	a.used = 0
}

// uniformArenaSlotCapacity returns the power-of-two slot capacity needed for
// required draws without shrinking current. The renderer calls this before it
// writes any per-frame uniforms so replacing the GPU buffer never invalidates
// slots already assigned to earlier draws in the same frame.
func uniformArenaSlotCapacity(current, required int) int {
	if current < 1 {
		current = 1
	}
	for current < required {
		current *= 2
	}
	return current
}

// packSpriteBatchAnimUniforms packs one arena slot: the shared 32-byte
// camera uniform followed by the 48-byte animation uniform block (wrapped
// time, bounce parameters, pulse) consumed by the animated sprite shaders.
func packSpriteBatchAnimUniforms(dst []byte, camera render.Camera, w, h int, anim render.SpriteBatchAnimation) {
	tess.PackSpriteUniforms(dst, camera, w, h)
	writer := spriteUniformWriter{dst: dst, offset: tess.SpriteUniformSize}
	writer.f32(float64(renderbatch.WrapTime(anim.Time)))
	writer.f32(anim.BounceHz)
	writer.f32(anim.Intensity)
	writer.f32(0)
	writer.f32(anim.Pulse.Center.X)
	writer.f32(anim.Pulse.Center.Y)
	writer.f32(anim.Pulse.Radius)
	writer.f32(anim.Pulse.Width)
	writer.f32(anim.Pulse.Boost)
	writer.f32(0)
	writer.f32(0)
	writer.f32(0)
}
