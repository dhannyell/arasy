package tess

import (
	"encoding/binary"
	"math"

	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// Retained sprite records keep their clip always open: a retained batch
// applies its command clip as a draw-time scissor, so the per-record clip
// must never discard. The sentinel bounds any finite screen position.
var retainedSpriteClipMin = [2]float32{-math.MaxFloat32, -math.MaxFloat32}
var retainedSpriteClipMax = [2]float32{math.MaxFloat32, math.MaxFloat32}

const (
	projectedSpriteFlag uint32 = 1 << iota
	flipXSpriteFlag
	flipYSpriteFlag
)

func spriteFlags(projectCorners bool, flip render.SpriteFlip) uint32 {
	flags := uint32(0)
	if projectCorners {
		flags |= projectedSpriteFlag
	}
	if flip&render.FlipX != 0 {
		flags |= flipXSpriteFlag
	}
	if flip&render.FlipY != 0 {
		flags |= flipYSpriteFlag
	}
	return flags
}

// AppendRetainedSpriteRecord appends one 64-byte sprite storage record for a
// retained batch instance, serialized exactly like the streaming records of
// appendSpriteRecord: world center/half-size, rotation, corner mode, UVs from
// the (base) Src cell, packed tint and texture layer — but with the always-open
// clip sentinel instead of a per-command clip.
//
// whiteTint forces the opaque-white tint and full UV range used for missing
// textures, so source rectangles cannot hide or recolor the diagnostic
// placeholder. A degenerate Src (clamped empty, as streaming's srcUV reports)
// zeroes the quad size instead of skipping, since retained records cannot be
// dropped without reordering the batch.
func AppendRetainedSpriteRecord(dst []byte, inst render.SpriteInstance, texW, texH int, textureLayer uint32, whiteTint bool) []byte {
	minX, maxX := inst.Dest.Min.X, inst.Dest.Max.X
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := inst.Dest.Min.Y, inst.Dest.Max.Y
	if minY > maxY {
		minY, maxY = maxY, minY
	}

	uvMin := [2]float32{}
	uvMax := [2]float32{1, 1}
	srcOK := true
	if !whiteTint && inst.Src != (math2d.Rect{}) {
		uv, ok := srcUV(inst.Src, texW, texH)
		if !ok {
			srcOK = false
		} else {
			uvMin, uvMax = uv[0], uv[2]
		}
	}

	halfWidth := float32((maxX - minX) * 0.5)
	halfHeight := float32((maxY - minY) * 0.5)
	if !srcOK {
		halfWidth, halfHeight = 0, 0
	}

	tint := uint32(0xffffffff)
	if !whiteTint {
		tint = packSpriteTint(inst.Tint)
	}
	flags := spriteFlags(inst.ProjectCorners, inst.Flip)

	record := spriteRecord{
		centerX:    float32((minX + maxX) * 0.5),
		centerY:    float32((minY + maxY) * 0.5),
		halfWidth:  halfWidth,
		halfHeight: halfHeight,
		rotation:   float32(inst.Rotation), flags: flags,
		uvMinX: uvMin[0], uvMinY: uvMin[1],
		uvMaxX: uvMax[0], uvMaxY: uvMax[1],
		clipMinX: retainedSpriteClipMin[0], clipMinY: retainedSpriteClipMin[1],
		clipMaxX: retainedSpriteClipMax[0], clipMaxY: retainedSpriteClipMax[1],
		tint: tint, textureLayer: textureLayer,
	}
	return append(dst, spriteBytes([]spriteRecord{record})...)
}

// SpriteAnimStride is the byte size of one retained animation record. Keep it
// in sync with the WGSL SpriteAnim struct of the animated batch shaders.
const SpriteAnimStride = 32

// AppendSpriteAnimRecord appends the per-instance animation record the
// animated sprite batch shaders read next to the base record. An invalid
// flip-book sequence packs Frames=0, the same static-Src degradation as the
// CPU reference in renderbatch.AnimatedSrc.
func AppendSpriteAnimRecord(dst []byte, inst render.SpriteInstance, texW, texH int) []byte {
	anim := inst.Animation
	frames := uint32(0)
	cellsPerRow := uint32(0)
	baseIndex := uint32(0)
	if cpr, base, ok := renderbatch.FlipBookCells(inst, texW, texH); ok {
		frames = uint32(anim.Frames)
		cellsPerRow = uint32(cpr)
		baseIndex = uint32(base)
	}
	var buf [SpriteAnimStride]byte
	binary.LittleEndian.PutUint32(buf[0:], frames)
	binary.LittleEndian.PutUint32(buf[4:], cellsPerRow)
	binary.LittleEndian.PutUint32(buf[8:], baseIndex)
	binary.LittleEndian.PutUint32(buf[16:], math.Float32bits(float32(anim.Rate)))
	binary.LittleEndian.PutUint32(buf[20:], math.Float32bits(float32(anim.Phase)))
	binary.LittleEndian.PutUint32(buf[24:], math.Float32bits(float32(anim.Bounce)))
	return append(dst, buf[:]...)
}

// SpriteBatchRange is one consecutive span of batch records sharing a bind
// target. The array path binds one 2d-array group for every non-missing
// texture (Texture is zero); the fallback binds per consecutive texture.
// Missing selects the shared diagnostic placeholder texture (Texture is zero).
type SpriteBatchRange struct {
	Texture render.TextureID
	Missing bool
	Start   int
	Count   int
}

// SpriteBatchPlan is the CPU-side result of packing one retained sprite
// batch: the 64-byte records to upload once, the optional animation records,
// consecutive draw ranges, and the metadata needed for culling and
// accounting. Content is render-only.
type SpriteBatchPlan struct {
	Records     []byte
	Anims       []byte
	Animated    bool
	Ranges      []SpriteBatchRange
	Bounds      math2d.Rect
	HasBounds   bool
	MaxBounce   float64
	HasRotation bool
	Count       int
	NonFinite   int
}

// PlanSpriteBatch packs instances into retained records without reordering:
// Src is normalized (zero = whole texture), tint and texture layer are
// packed, and the clip sentinel stays always open (the retained clip is the
// command's draw-time scissor). Non-finite instances are dropped (counted in
// NonFinite so the adapter can warn once). Instances whose texture is
// missing join a diagnostic range with a forced white tint, preserving the
// streaming placeholder without solid-geometry fallbacks.
func PlanSpriteBatch(instances []render.SpriteInstance, sizes TexSizes, arrayEnabled bool) SpriteBatchPlan {
	plan := SpriteBatchPlan{}
	for i := range instances {
		if instances[i].Animation != (render.SpriteAnimation{}) {
			plan.Animated = true
			break
		}
	}

	for i := range instances {
		inst := instances[i]
		if !renderbatch.IsFiniteSpriteGeometry(inst) {
			plan.NonFinite++
			continue
		}
		plan.HasRotation = plan.HasRotation || inst.Rotation != 0

		texW, texH, found := sizes.Lookup(inst.Texture)
		missing := !found
		layer := uint32(0)
		if arrayEnabled && !missing {
			layer = uint32(inst.Texture - 1)
		}
		plan.Records = AppendRetainedSpriteRecord(plan.Records, inst, texW, texH, layer, missing)

		if plan.Animated {
			plan.Anims = AppendSpriteAnimRecord(plan.Anims, inst, texW, texH)
		}

		if b := float64(inst.Animation.Bounce); math.Abs(b) > plan.MaxBounce {
			plan.MaxBounce = math.Abs(b)
		}
		if bounds, ok := renderbatch.SpriteBounds(inst); ok {
			if !plan.HasBounds {
				plan.Bounds, plan.HasBounds = bounds, true
			} else {
				plan.Bounds.Min.X = min(plan.Bounds.Min.X, bounds.Min.X)
				plan.Bounds.Min.Y = min(plan.Bounds.Min.Y, bounds.Min.Y)
				plan.Bounds.Max.X = max(plan.Bounds.Max.X, bounds.Max.X)
				plan.Bounds.Max.Y = max(plan.Bounds.Max.Y, bounds.Max.Y)
			}
		}

		texture := inst.Texture
		if arrayEnabled || missing {
			texture = 0
		}
		if last := len(plan.Ranges) - 1; last >= 0 && plan.Ranges[last].Texture == texture && plan.Ranges[last].Missing == missing {
			plan.Ranges[last].Count++
		} else {
			plan.Ranges = append(plan.Ranges, SpriteBatchRange{Texture: texture, Missing: missing, Start: plan.Count, Count: 1})
		}
		plan.Count++
	}
	return plan
}

// SpriteBatchInflatedBounds returns the batch bounds inflated by the
// command's animation, for whole-batch culling. ok is false when the batch
// has no finite geometry at all.
func SpriteBatchInflatedBounds(bounds math2d.Rect, hasBounds bool, maxBounce float64, anim render.SpriteBatchAnimation) (math2d.Rect, bool) {
	if !hasBounds {
		return math2d.Rect{}, false
	}
	inflation := renderbatch.MaxVerticalInflation(maxBounce, anim)
	bounds.Min.Y -= inflation
	bounds.Max.Y += inflation
	return bounds, true
}

// SpriteTextureArrayPlan selects the sprite texture resource layout. Reason
// says why the plan left the array disabled.
type SpriteTextureArrayPlan struct {
	Enabled bool
	Width   int
	Height  int
	Layers  int
	Reason  string
}

// PlanSpriteTextureArray selects the shared-array resource layout only when
// at least two decoded textures have identical dimensions and the complete
// TextureID range fits the device's layer limit. Zero-size entries represent
// failed decodes and do not prevent valid textures from sharing the array.
func PlanSpriteTextureArray(sizes TexSizes, maxLayers int) SpriteTextureArrayPlan {
	layers := len(sizes) - 1
	if layers <= 1 {
		return SpriteTextureArrayPlan{Reason: "fewer than two declared textures"}
	}
	if maxLayers <= 0 || layers > maxLayers {
		return SpriteTextureArrayPlan{Reason: "texture count exceeds device layer limit"}
	}

	width, height, available := 0, 0, 0
	for _, size := range sizes[1:] {
		if size[0] <= 0 || size[1] <= 0 {
			continue
		}
		available++
		if width == 0 {
			width, height = size[0], size[1]
			continue
		}
		if size[0] != width || size[1] != height {
			return SpriteTextureArrayPlan{Reason: "texture dimensions differ"}
		}
	}
	if available < 2 {
		return SpriteTextureArrayPlan{Reason: "fewer than two decoded textures"}
	}

	return SpriteTextureArrayPlan{Enabled: true, Width: width, Height: height, Layers: layers}
}
