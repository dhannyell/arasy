package wasm

import (
	"encoding/binary"
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// packSpriteUniforms packs the 32-byte camera uniform shared by the sprite,
// text and retained-batch pipelines: world-to-clip scale/offset, the
// world-to-screen scale used to size quads before rotation, and the
// screen-to-clip scale used to reproject the rotated corner.
func packSpriteUniforms(dst []byte, camera render.Camera, w, h int) {
	scale, offset, screenScale := worldToClip(camera, w, h)
	writer := particleUniformWriter{dst: dst}
	writer.vec2(scale)
	writer.vec2(offset)
	writer.vec2(screenScale)
	writer.vec2(math2d.Vec2{X: 2 / float64(w), Y: -2 / float64(h)})
}

type particleUniformWriter struct {
	dst    []byte
	offset int
}

func (w *particleUniformWriter) f32(value float64) {
	binary.LittleEndian.PutUint32(w.dst[w.offset:], math.Float32bits(float32(value)))
	w.offset += 4
}

func (w *particleUniformWriter) u32(value uint32) {
	binary.LittleEndian.PutUint32(w.dst[w.offset:], value)
	w.offset += 4
}

func (w *particleUniformWriter) vec2(value math2d.Vec2) {
	w.f32(value.X)
	w.f32(value.Y)
}

// worldToClip returns the affine coefficients consumed by the particle
// vertex shader. It mirrors Camera.WorldToScreen followed by tess's
// screenToClip conversion, including aspect-fit/expand view resolution.
func worldToClip(camera render.Camera, w, h int) (scale, offset, screenScale math2d.Vec2) {
	if w <= 0 || h <= 0 {
		return math2d.Vec2{}, math2d.Vec2{}, math2d.Vec2{}
	}
	resolved := camera.Resolve()
	screenScale = math2d.Vec2{X: resolved.ScaleX, Y: resolved.ScaleY}
	scale = math2d.Vec2{
		X: 2 * resolved.ScaleX / float64(w),
		Y: -2 * resolved.ScaleY / float64(h),
	}
	offset = math2d.Vec2{
		X: 2*(resolved.Viewport.Min.X-resolved.World.Min.X*resolved.ScaleX)/float64(w) - 1,
		Y: 1 - 2*(resolved.Viewport.Min.Y-resolved.World.Min.Y*resolved.ScaleY)/float64(h),
	}
	return scale, offset, screenScale
}
