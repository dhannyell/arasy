package raster_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/raster"
	"github.com/dhannyell/frameforge/render"
)

func BenchmarkRendererRender(b *testing.B) {
	renderer := raster.Renderer{
		Width:  1280,
		Height: 720,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720}),
		},
		Clear: render.RGBA(0, 0, 0, 1),
	}

	commands := make([]render.Command, 0, 500)
	for i := range 500 {
		x := float64(i % 128)
		y := float64(i / 128)
		commands = append(commands, render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: x * 10, Y: y * 10},
				math2d.Vec2{X: 8, Y: 8},
			),
			Fill: render.RGBA(1, 0, 0, 0.8),
		})
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = renderer.Render(commands)
	}
}
