package tess

import (
	"encoding/binary"
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// SpriteUniformSize is the size of the camera data shared by sprite and text
// shaders. Keep it in sync with their WGSL uniform blocks.
const SpriteUniformSize = 48

// PackSpriteUniforms writes camera data for the sprite and text shaders.
// Positions use the full camera mapping. Sprite records choose between
// screen-space billboard corners and projected world corners.
func PackSpriteUniforms(dst []byte, camera render.Camera, w, h int) {
	worldToClip, offset, screenScale, screenToClipScale := WorldToClip(camera, w, h)
	values := [12]float64{
		worldToClip[0].X, worldToClip[0].Y,
		worldToClip[1].X, worldToClip[1].Y,
		offset.X, offset.Y,
		screenScale.X, screenScale.Y,
		screenToClipScale.X, screenToClipScale.Y,
	}
	for i, value := range values {
		binary.LittleEndian.PutUint32(dst[i*4:], math.Float32bits(float32(value)))
	}
}

// PackTextUniforms writes the screen-space camera data used by text records.
// offset is the additional screen displacement for a retained batch whose
// labels were packed under the base camera.
func PackTextUniforms(dst []byte, offset math2d.Vec2, w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	screenToClipScale := math2d.Vec2{X: 2 / float64(w), Y: -2 / float64(h)}
	values := [12]float64{
		offset.X * screenToClipScale.X,
		offset.Y * screenToClipScale.Y,
		0, 0,
		0, 0,
		0, 0,
		screenToClipScale.X, screenToClipScale.Y,
	}
	for i, value := range values {
		binary.LittleEndian.PutUint32(dst[i*4:], math.Float32bits(float32(value)))
	}
}

// TextScreenOffset returns the constant screen displacement between two
// cameras that differ only by a parallax translation.
func TextScreenOffset(base, derived render.Camera) math2d.Vec2 {
	origin := math2d.Vec2{}
	return derived.WorldToScreen(origin).Sub(base.WorldToScreen(origin))
}

// WorldToClip returns the camera mapping used by the sprite and text shaders.
// It uses the same world-to-screen rules as Camera.WorldToScreen, including
// aspect-fit and aspect-expand resolution.
func WorldToClip(camera render.Camera, w, h int) (worldToClip [2]math2d.Vec2, offset, screenScale, screenToClipScale math2d.Vec2) {
	if w <= 0 || h <= 0 {
		return [2]math2d.Vec2{}, math2d.Vec2{}, math2d.Vec2{}, math2d.Vec2{}
	}
	resolved := camera.Resolve()
	screenScale = math2d.Vec2{X: resolved.ScaleX, Y: resolved.ScaleY}
	screenToClipScale = math2d.Vec2{X: 2 / float64(w), Y: -2 / float64(h)}
	view := camera.View
	if view.IsIdentity() {
		view = math2d.Identity()
	}
	worldToClip[0] = math2d.Vec2{
		X: screenToClipScale.X * resolved.ScaleX * view.A,
		Y: screenToClipScale.Y * resolved.ScaleY * view.B,
	}
	worldToClip[1] = math2d.Vec2{
		X: screenToClipScale.X * resolved.ScaleX * view.C,
		Y: screenToClipScale.Y * resolved.ScaleY * view.D,
	}
	offset = math2d.Vec2{
		X: -1 + screenToClipScale.X*(resolved.Viewport.Min.X+
			resolved.ScaleX*(view.E-resolved.World.Min.X)),
		Y: 1 + screenToClipScale.Y*(resolved.Viewport.Min.Y+
			resolved.ScaleY*(view.F-resolved.World.Min.Y)),
	}
	return worldToClip, offset, screenScale, screenToClipScale
}
