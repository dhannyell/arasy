package tess_test

import (
	"testing"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestUniformKeysShareCameraWithoutParallax(t *testing.T) {
	base := layerCameraTestCamera()
	runs := []tess.StreamRun{
		{Kind: tess.DrawRunSprites, Layer: 0},
		{Kind: tess.DrawRunSprites, Layer: 10},
		{Kind: tess.DrawRunSpriteBatch, Layer: 20},
	}

	keys := collectUniformKeys(t, runs, base, nil)
	if len(keys) != 1 {
		t.Fatalf("uniform keys = %d, want one shared entry", len(keys))
	}
}

func TestUniformKeysSeparateParallaxLayers(t *testing.T) {
	base := layerCameraTestCamera()
	base.Parallax = []render.LayerParallax{
		{FromLayer: 0, ToLayer: 0, Factor: math2d.Vec2{X: 1, Y: 1}},
		{FromLayer: 10, ToLayer: 10, Factor: math2d.Vec2{X: 0.5, Y: 0.5}},
	}
	runs := []tess.StreamRun{
		{Kind: tess.DrawRunSprites, Layer: 0},
		{Kind: tess.DrawRunSprites, Layer: 10},
	}

	keys := collectUniformKeys(t, runs, base, tess.LayerCameras(base, runs))
	if len(keys) != 2 {
		t.Fatalf("uniform keys = %d, want one per parallax layer", len(keys))
	}
}

func TestUniformKeysSeparateTextBatchesByScreenOffset(t *testing.T) {
	base := layerCameraTestCamera()
	base.Parallax = []render.LayerParallax{
		{FromLayer: 3, ToLayer: 3, Factor: math2d.Vec2{X: 0.5, Y: 0.5}},
	}
	runs := []tess.StreamRun{
		{Kind: tess.DrawRunTextBatch, Layer: 3},
		{Kind: tess.DrawRunTextBatch, Layer: 4},
	}
	cameras := tess.LayerCameras(base, runs)

	parallaxKey, ok := tess.UniformKeyForKind(runs[0].Kind, runs[0].Layer, base, cameras)
	if !ok {
		t.Fatal("text batch on a parallax layer got no key")
	}
	baseKey, ok := tess.UniformKeyForKind(runs[1].Kind, runs[1].Layer, base, cameras)
	if !ok {
		t.Fatal("text batch on the base layer got no key")
	}
	if parallaxKey == baseKey {
		t.Fatal("text batches on different layers share a key; offsets must differ")
	}

	streamingKey, ok := tess.UniformKeyForKind(tess.DrawRunText, 3, base, cameras)
	if !ok {
		t.Fatal("streaming text got no key")
	}
	if streamingKey != baseKey {
		t.Fatal("streaming text must share the zero-offset key of the base camera")
	}
}

func collectUniformKeys(t *testing.T, runs []tess.StreamRun, base render.Camera, cameras []tess.LayerCamera) []tess.UniformKey {
	t.Helper()
	keys := make([]tess.UniformKey, 0, len(runs))
	for _, run := range runs {
		key, ok := tess.UniformKeyForKind(run.Kind, run.Layer, base, cameras)
		if !ok {
			continue
		}
		if tess.UniformKeyIndex(keys, key) < 0 {
			keys = append(keys, key)
		}
	}
	return keys
}

func layerCameraTestCamera() render.Camera {
	return render.Camera{
		// The world center is off the origin so a parallax factor below 1
		// produces a non-zero translation against the default anchor.
		World:    math2d.NewRect(math2d.Vec2{X: 10, Y: 20}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 640, Y: 360}),
		Aspect:   render.AspectStretch,
	}
}
