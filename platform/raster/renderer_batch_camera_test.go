package raster_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
)

func TestSpriteBatchCameraCharacterization(t *testing.T) {
	center := math2d.Vec2{X: 10, Y: 10}
	instance := render.SpriteInstance{
		Texture: 1,
		Dest:    math2d.NewRect(math2d.Vec2{X: 7, Y: 8}, math2d.Vec2{X: 6, Y: 4}),
	}

	tests := []struct {
		name   string
		camera render.Camera
	}{
		{name: "identity", camera: characterizationRasterCamera()},
		{
			name: "isometric view",
			camera: func() render.Camera {
				camera := characterizationRasterCamera()
				camera.View = math2d.Affine{A: 1, C: -1, B: 0.5, D: 0.5}
				return camera
			}(),
		},
		{
			name: "depth scale",
			camera: func() render.Camera {
				camera := characterizationRasterCamera()
				camera.DepthScale = 0.5
				return camera
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := characterizationRasterRenderer(tt.camera)
			renderer.ReplaceSpriteBatch(1, []render.SpriteInstance{instance})
			image := renderer.Render([]render.Command{render.DrawSpriteBatch{Batch: 1}})

			want := tt.camera.WorldToScreen(center)
			assertPixel(t, image.At(int(math.Floor(want.X)), int(math.Floor(want.Y))), color.RGBA{
				R: 255, G: 255, B: 255, A: 255,
			})

			// The isometric case must not fall back to the unprojected center.
			if tt.camera.View != (math2d.Affine{}) {
				stale := tt.camera
				stale.View = math2d.Affine{}
				staleCenter := stale.WorldToScreen(center)
				assertPixel(t, image.At(int(math.Floor(staleCenter.X)), int(math.Floor(staleCenter.Y))), color.RGBA{
					R: 0, G: 0, B: 0, A: 255,
				})
			}
		})
	}
}

func characterizationRasterCamera() render.Camera {
	return render.Camera{
		World:    math2d.NewRect(math2d.Vec2{X: -50, Y: -50}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Aspect:   render.AspectStretch,
	}
}

func characterizationRasterRenderer(camera render.Camera) raster.Renderer {
	return raster.Renderer{
		Width:    100,
		Height:   100,
		Camera:   camera,
		Clear:    render.RGBA(0, 0, 0, 1),
		Textures: pixel.Registry{nil, makeTexWhite()},
	}
}
