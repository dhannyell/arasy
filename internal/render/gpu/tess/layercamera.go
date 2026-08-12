package tess

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// LayerCamera caches the derived camera of one painter layer for the frame,
// so the per-layer derivation runs once per layer, never per run.
type LayerCamera struct {
	// Layer is the painter layer the camera belongs to.
	Layer int
	// Camera is base.ForLayer(Layer), resolved once for the frame.
	Camera render.Camera
}

// LayerCameras derives the camera of every layer that appears in runs. A
// camera without parallax returns nil: every layer uses the base camera.
func LayerCameras(base render.Camera, runs []StreamRun) []LayerCamera {
	if len(base.Parallax) == 0 {
		return nil
	}
	var cameras []LayerCamera
	for _, run := range runs {
		found := false
		for _, entry := range cameras {
			if entry.Layer == run.Layer {
				found = true
				break
			}
		}
		if !found {
			cameras = append(cameras, LayerCamera{Layer: run.Layer, Camera: base.ForLayer(run.Layer)})
		}
	}
	return cameras
}

// CameraForLayer returns the cached derived camera of layer, or the base
// camera when the layer has no cache entry.
func CameraForLayer(base render.Camera, cameras []LayerCamera, layer int) render.Camera {
	for _, entry := range cameras {
		if entry.Layer == layer {
			return entry.Camera
		}
	}
	return base
}

// UniformKind selects which packing a camera uniform upload uses.
type UniformKind uint8

const (
	// UniformKindSprites packs the full world-to-clip camera mapping.
	UniformKindSprites UniformKind = iota
	// UniformKindText packs a screen-space displacement for text records.
	UniformKindText
)

// UniformKey identifies one packed camera upload. Runs that share a key
// share the upload: without parallax every sprite run collapses to layer 0,
// and text batches key by their exact screen offset.
type UniformKey struct {
	// Kind selects the packing.
	Kind UniformKind
	// Layer is the painter layer for sprite uploads; zero for text.
	Layer int
	// OffsetX and OffsetY carry the text screen offset as float bits, so
	// the key stays comparable.
	OffsetX uint64
	OffsetY uint64
	// Pass separates identical layer keys belonging to different cameras.
	Pass int
}

// UniformKeyForKind computes the upload key of one run kind. Streaming text
// needs only the shared zero-offset uniform, because its glyphs were
// tessellated under the derived camera already.
func UniformKeyForKind(kind DrawRunKind, layer int, base render.Camera, cameras []LayerCamera) (UniformKey, bool) {
	switch kind {
	case DrawRunSprites, DrawRunSpriteBatch:
		if len(base.Parallax) == 0 {
			layer = 0
		}
		return UniformKey{Kind: UniformKindSprites, Layer: layer}, true
	case DrawRunText:
		return UniformKey{Kind: UniformKindText}, true
	case DrawRunTextBatch:
		effective := CameraForLayer(base, cameras, layer)
		offset := TextScreenOffset(base, effective)
		return UniformKey{
			Kind:    UniformKindText,
			OffsetX: math.Float64bits(offset.X),
			OffsetY: math.Float64bits(offset.Y),
		}, true
	default:
		return UniformKey{}, false
	}
}

// UniformKeyIndex returns the position of key in keys, or -1.
func UniformKeyIndex(keys []UniformKey, key UniformKey) int {
	for index, entry := range keys {
		if entry == key {
			return index
		}
	}
	return -1
}

// PackUniform writes the camera upload key describes into dst.
func PackUniform(dst []byte, key UniformKey, base render.Camera, cameras []LayerCamera, w, h int) {
	switch key.Kind {
	case UniformKindSprites:
		PackSpriteUniforms(dst, CameraForLayer(base, cameras, key.Layer), w, h)
	case UniformKindText:
		PackTextUniforms(dst, math2d.Vec2{
			X: math.Float64frombits(key.OffsetX),
			Y: math.Float64frombits(key.OffsetY),
		}, w, h)
	}
}
