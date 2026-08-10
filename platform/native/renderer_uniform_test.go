//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"testing"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestNativeUniformKeysShareCameraWithoutParallax(t *testing.T) {
	base := nativeTestCamera()
	runs := []tess.StreamRun{
		{Kind: tess.DrawRunSprites, Layer: 0},
		{Kind: tess.DrawRunSprites, Layer: 10},
		{Kind: tess.DrawRunSpriteBatch, Layer: 20},
	}

	entries := make([]nativeUniformEntry, 0, len(runs))
	for _, run := range runs {
		key, ok := nativeUniformKeyForRun(run, base, nil)
		if !ok {
			continue
		}
		if nativeUniformEntryIndex(entries, key) < 0 {
			entries = append(entries, nativeUniformEntry{key: key})
		}
	}
	if len(entries) != 1 {
		t.Fatalf("uniform entries = %d, want one shared entry", len(entries))
	}
}

func TestNativeUniformKeysSeparateParallaxLayers(t *testing.T) {
	base := nativeTestCamera()
	base.Parallax = []render.LayerParallax{
		{FromLayer: 0, ToLayer: 0, Factor: math2d.Vec2{X: 1, Y: 1}},
		{FromLayer: 10, ToLayer: 10, Factor: math2d.Vec2{X: 0.5, Y: 0.5}},
	}
	runs := []tess.StreamRun{
		{Kind: tess.DrawRunSprites, Layer: 0},
		{Kind: tess.DrawRunSprites, Layer: 10},
	}

	entries := make([]nativeUniformEntry, 0, len(runs))
	for _, run := range runs {
		key, ok := nativeUniformKeyForRun(run, base, nativeLayerCameras(base, runs))
		if !ok {
			continue
		}
		if nativeUniformEntryIndex(entries, key) < 0 {
			entries = append(entries, nativeUniformEntry{key: key})
		}
	}
	if len(entries) != 2 {
		t.Fatalf("uniform entries = %d, want one per parallax layer", len(entries))
	}
}

func nativeTestCamera() render.Camera {
	return render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 600}),
	}
}
