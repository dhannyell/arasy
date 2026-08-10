package tess

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// Retained sprite records keep their clip always open: a retained batch
// applies its command clip as a draw-time scissor, so the per-record clip
// must never discard. The sentinel bounds any finite screen position.
var retainedSpriteClipMin = [2]float32{-math.MaxFloat32, -math.MaxFloat32}
var retainedSpriteClipMax = [2]float32{math.MaxFloat32, math.MaxFloat32}

const projectedSpriteFlag uint32 = 1

// AppendRetainedSpriteRecord appends one 64-byte sprite storage record for a
// retained batch instance, serialized exactly like the streaming records of
// appendSpriteRecord: world center/half-size, rotation, corner mode, UVs from
// the (base) Src cell, packed tint and texture layer — but with the always-open
// clip sentinel instead of a per-command clip.
//
// whiteTint forces the opaque-white tint used for missing textures, so the
// backend's 1x1 magenta placeholder texture paints the same magenta the
// streaming missing-texture path draws. A degenerate Src (clamped empty, as
// streaming's srcUV reports) zeroes the quad size instead of skipping, since
// retained records cannot be dropped without reordering the batch.
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
	if inst.Src != (math2d.Rect{}) {
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
	flags := uint32(0)
	if inst.ProjectCorners {
		flags = projectedSpriteFlag
	}

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
