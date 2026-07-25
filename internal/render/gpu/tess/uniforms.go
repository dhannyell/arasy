package tess

import (
	"encoding/binary"
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// SpriteUniformSize is the byte layout shared by the sprite and text WGSL
// camera uniforms.
const SpriteUniformSize = 32

// PackSpriteUniforms packs the camera uniform shared by the sprite and text
// pipelines: world-to-clip scale/offset, the world-to-screen scale used to
// size quads before rotation, and the screen-to-clip scale used to reproject
// the rotated corner.
func PackSpriteUniforms(dst []byte, camera render.Camera, w, h int) {
	scale, offset, screenScale := WorldToClip(camera, w, h)
	values := [8]float64{
		scale.X, scale.Y,
		offset.X, offset.Y,
		screenScale.X, screenScale.Y,
		2 / float64(w), -2 / float64(h),
	}
	for i, value := range values {
		binary.LittleEndian.PutUint32(dst[i*4:], math.Float32bits(float32(value)))
	}
}

// WorldToClip returns the affine coefficients consumed by the WGSL sprite,
// text and particle vertex shaders. It mirrors Camera.WorldToScreen followed
// by tess's screenToClip conversion, including aspect-fit/expand view
// resolution.
func WorldToClip(camera render.Camera, w, h int) (scale, offset, screenScale math2d.Vec2) {
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
