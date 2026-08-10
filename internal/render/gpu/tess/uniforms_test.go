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
	worldToClip, offset, screenScale, screenToClipScale := tess.WorldToClip(camera, 800, 600)

	assertVec2(t, worldToClip[0], math2d.Vec2{X: 0.01})
	assertVec2(t, worldToClip[1], math2d.Vec2{Y: -1.0 / 75})
	assertVec2(t, offset, math2d.Vec2{X: -1.05, Y: 37.0 / 30})
	assertVec2(t, screenScale, math2d.Vec2{X: 4, Y: 4})
	assertVec2(t, screenToClipScale, math2d.Vec2{X: 1.0 / 400, Y: -1.0 / 300})
}

func TestPackSpriteUniformsWritesWGSL48ByteLayout(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRectFromCorners(math2d.Vec2{X: 10, Y: 20}, math2d.Vec2{X: 110, Y: 70}),
		Viewport: math2d.NewRectFromCorners(math2d.Vec2{X: 20, Y: 10}, math2d.Vec2{X: 420, Y: 210}),
	}
	var got [tess.SpriteUniformSize]byte
	tess.PackSpriteUniforms(got[:], camera, 800, 600)

	want := []float32{
		0.01, 0,
		0, -1.0 / 75,
		-1.05, 37.0 / 30,
		4, 4,
		1.0 / 400, -1.0 / 300,
		0, 0,
	}
	for i, expected := range want {
		actual := math.Float32frombits(binary.LittleEndian.Uint32(got[i*4:]))
		if actual != expected {
			t.Fatalf("uniform float %d = %v, want %v", i, actual, expected)
		}
	}
}

func TestWorldToClipMatrixMatchesWorldToScreen(t *testing.T) {
	cameras := []render.Camera{
		characterizationCamera(),
		func() render.Camera {
			camera := characterizationCamera()
			camera.View = math2d.Affine{A: 1, C: -1, B: 0.5, D: 0.5}
			return camera
		}(),
		func() render.Camera {
			camera := characterizationCamera()
			camera.DepthScale = 0.5
			return camera
		}(),
	}

	for cameraIndex, camera := range cameras {
		worldToClip, offset, _, screenToClipScale := tess.WorldToClip(camera, 100, 100)
		for index := 0; index < 100; index++ {
			point := math2d.Vec2{
				X: -47 + float64((index*37)%95),
				Y: -43 + float64((index*53)%91),
			}
			clip := math2d.Vec2{
				X: worldToClip[0].X*point.X + worldToClip[1].X*point.Y + offset.X,
				Y: worldToClip[0].Y*point.X + worldToClip[1].Y*point.Y + offset.Y,
			}
			got := math2d.Vec2{
				X: (clip.X + 1) / screenToClipScale.X,
				Y: (clip.Y - 1) / screenToClipScale.Y,
			}
			want := camera.WorldToScreen(point)
			if math.Abs(got.X-want.X) > 1e-6 || math.Abs(got.Y-want.Y) > 1e-6 {
				t.Fatalf("camera %d point %v = %v, want %v", cameraIndex, point, got, want)
			}
		}
	}
}

func assertVec2(t *testing.T, actual, expected math2d.Vec2) {
	t.Helper()
	if actual != expected {
		t.Fatalf("vector = %+v, want %+v", actual, expected)
	}
}
