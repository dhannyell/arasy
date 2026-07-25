package tess_test

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestWorldToClipUsesDefaultAspectWithNonTrivialViewport(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRectFromCorners(math2d.Vec2{X: 10, Y: 20}, math2d.Vec2{X: 110, Y: 70}),
		Viewport: math2d.NewRectFromCorners(math2d.Vec2{X: 20, Y: 10}, math2d.Vec2{X: 420, Y: 210}),
	}
	scale, offset, screenScale := tess.WorldToClip(camera, 800, 600)

	assertVec2(t, scale, math2d.Vec2{X: 0.01, Y: -1.0 / 75})
	assertVec2(t, offset, math2d.Vec2{X: -1.05, Y: 37.0 / 30})
	assertVec2(t, screenScale, math2d.Vec2{X: 4, Y: 4})
}

func TestPackSpriteUniformsWritesWGSL32ByteLayout(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRectFromCorners(math2d.Vec2{X: 10, Y: 20}, math2d.Vec2{X: 110, Y: 70}),
		Viewport: math2d.NewRectFromCorners(math2d.Vec2{X: 20, Y: 10}, math2d.Vec2{X: 420, Y: 210}),
	}
	var got [tess.SpriteUniformSize]byte
	tess.PackSpriteUniforms(got[:], camera, 800, 600)

	want := []float32{0.01, -1.0 / 75, -1.05, 37.0 / 30, 4, 4, 1.0 / 400, -1.0 / 300}
	for i, expected := range want {
		actual := math.Float32frombits(binary.LittleEndian.Uint32(got[i*4:]))
		if actual != expected {
			t.Fatalf("uniform float %d = %v, want %v", i, actual, expected)
		}
	}
}

func assertVec2(t *testing.T, actual, expected math2d.Vec2) {
	t.Helper()
	if actual != expected {
		t.Fatalf("vector = %+v, want %+v", actual, expected)
	}
}
