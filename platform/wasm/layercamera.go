package wasm

import (
	"math"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/render"
)

// wasmLayerCamera caches the derived camera of one painter layer for the
// frame, so the per-layer derivation runs once per layer, never per run.
type wasmLayerCamera struct {
	layer  int
	camera render.Camera
}

func wasmLayerCameras(base render.Camera, runs []tess.StreamRun) []wasmLayerCamera {
	if len(base.Parallax) == 0 {
		return nil
	}
	var cameras []wasmLayerCamera
	for _, run := range runs {
		found := false
		for _, entry := range cameras {
			if entry.layer == run.Layer {
				found = true
				break
			}
		}
		if !found {
			cameras = append(cameras, wasmLayerCamera{layer: run.Layer, camera: base.ForLayer(run.Layer)})
		}
	}
	return cameras
}

func wasmCameraForLayer(base render.Camera, cameras []wasmLayerCamera, layer int) render.Camera {
	for _, entry := range cameras {
		if entry.layer == layer {
			return entry.camera
		}
	}
	return base
}

type wasmUniformKind uint8

const (
	wasmUniformSprites wasmUniformKind = iota
	wasmUniformText
)

// wasmUniformKey identifies one packed camera upload. Runs that share a key
// share the upload: without parallax every sprite run collapses to layer 0,
// and text batches key by their exact screen offset.
type wasmUniformKey struct {
	kind    wasmUniformKind
	layer   int
	offsetX uint64
	offsetY uint64
}

// wasmUniformKeyForKind computes the upload key of one run kind. Streaming
// text needs only the shared zero-offset uniform, because its labels were
// tessellated under the derived camera already.
func wasmUniformKeyForKind(kind tess.DrawRunKind, layer int, base render.Camera, cameras []wasmLayerCamera) (wasmUniformKey, bool) {
	switch kind {
	case tess.DrawRunSprites, tess.DrawRunSpriteBatch:
		if len(base.Parallax) == 0 {
			layer = 0
		}
		return wasmUniformKey{kind: wasmUniformSprites, layer: layer}, true
	case tess.DrawRunText:
		return wasmUniformKey{kind: wasmUniformText}, true
	case tess.DrawRunTextBatch:
		effective := wasmCameraForLayer(base, cameras, layer)
		offset := tess.TextScreenOffset(base, effective)
		return wasmUniformKey{
			kind:    wasmUniformText,
			offsetX: math.Float64bits(offset.X),
			offsetY: math.Float64bits(offset.Y),
		}, true
	default:
		return wasmUniformKey{}, false
	}
}

func wasmUniformKeyIndex(keys []wasmUniformKey, key wasmUniformKey) int {
	for index, entry := range keys {
		if entry == key {
			return index
		}
	}
	return -1
}
