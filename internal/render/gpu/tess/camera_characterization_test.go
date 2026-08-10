package tess_test

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestRetainedSpriteBatchCameraCharacterization(t *testing.T) {
	const width, height = 100, 100
	center := math2d.Vec2{X: 10, Y: 10}

	tests := []struct {
		name        string
		camera      render.Camera
		wantCamera  math2d.Vec2
		wantUniform math2d.Vec2
	}{
		{
			name:        "identity",
			camera:      characterizationCamera(),
			wantCamera:  math2d.Vec2{X: 60, Y: 60},
			wantUniform: math2d.Vec2{X: 60, Y: 60},
		},
		{
			name: "isometric view",
			camera: func() render.Camera {
				camera := characterizationCamera()
				camera.View = math2d.Affine{A: 1, C: -1, B: 0.5, D: 0.5}
				return camera
			}(),
			wantCamera:  math2d.Vec2{X: 50, Y: 60},
			wantUniform: math2d.Vec2{X: 50, Y: 60},
		},
		{
			name: "depth scale",
			camera: func() render.Camera {
				camera := characterizationCamera()
				camera.DepthScale = 0.5
				return camera
			}(),
			wantCamera:  math2d.Vec2{X: 60, Y: 55},
			wantUniform: math2d.Vec2{X: 60, Y: 55},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCamera := tt.camera.WorldToScreen(center)
			assertVecNear(t, gotCamera, tt.wantCamera)

			var builder tess.Builder
			runs := builder.Build(tess.Frame{
				Camera:   tt.camera,
				Commands: []render.Command{render.DrawSpriteBatch{Batch: 1}},
			}, width, height, nil, nil)
			if len(runs) != 1 || runs[0].Kind != tess.DrawRunSpriteBatch {
				t.Fatalf("runs = %+v, want one retained sprite-batch run", runs)
			}

			// Retained batches store the center in the same 64-byte record that
			// the shader consumes. Project that record through the current
			// shared uniform to characterize the emitted sprite position.
			instance := render.SpriteInstance{
				Texture: 1,
				Dest:    math2d.NewRect(math2d.Vec2{X: 7, Y: 8}, math2d.Vec2{X: 6, Y: 4}),
			}
			record := tess.AppendRetainedSpriteRecord(nil, instance, 1, 1, 0, false)
			recordCenter := math2d.Vec2{
				X: float64(readF32(record, 0)),
				Y: float64(readF32(record, 4)),
			}
			gotUniform := packedUniformCenter(tt.camera, recordCenter, width, height)
			assertVecNear(t, gotUniform, tt.wantUniform)

			if !vectorsNear(gotUniform, gotCamera) {
				t.Fatalf("retained sprite center = %v, want Camera.WorldToScreen center %v",
					gotUniform, gotCamera)
			}
		})
	}
}

func characterizationCamera() render.Camera {
	return render.Camera{
		World:    math2d.NewRect(math2d.Vec2{X: -50, Y: -50}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Aspect:   render.AspectStretch,
	}
}

func packedUniformCenter(camera render.Camera, center math2d.Vec2, width, height int) math2d.Vec2 {
	var packed [tess.SpriteUniformSize]byte
	tess.PackSpriteUniforms(packed[:], camera, width, height)

	worldToClip := [2]math2d.Vec2{
		{X: float64(readF32(packed[:], 0)), Y: float64(readF32(packed[:], 4))},
		{X: float64(readF32(packed[:], 8)), Y: float64(readF32(packed[:], 12))},
	}
	offset := math2d.Vec2{X: float64(readF32(packed[:], 16)), Y: float64(readF32(packed[:], 20))}
	clip := math2d.Vec2{
		X: float64(float32(center.X)*float32(worldToClip[0].X) +
			float32(center.Y)*float32(worldToClip[1].X) + float32(offset.X)),
		Y: float64(float32(center.X)*float32(worldToClip[0].Y) +
			float32(center.Y)*float32(worldToClip[1].Y) + float32(offset.Y)),
	}
	return math2d.Vec2{
		X: (clip.X + 1) * float64(width) / 2,
		Y: (1 - clip.Y) * float64(height) / 2,
	}
}

func readF32(data []byte, offset int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(data[offset:]))
}

func assertVecNear(t *testing.T, got, want math2d.Vec2) {
	t.Helper()
	if !vectorsNear(got, want) {
		t.Fatalf("vector = %v, want %v", got, want)
	}
}

func vectorsNear(got, want math2d.Vec2) bool {
	return math.Abs(got.X-want.X) <= 1e-5 && math.Abs(got.Y-want.Y) <= 1e-5
}
