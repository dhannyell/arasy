package raster_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
)

func BenchmarkRendererRender(b *testing.B) {
	renderer, commands := benchmarkRendererCommands()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = renderer.Render(commands)
	}
}

func BenchmarkRendererRenderInto(b *testing.B) {
	renderer, commands := benchmarkRendererCommands()
	target := renderer.NewTarget()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		renderer.RenderInto(target, commands)
	}
}

// BenchmarkRendererRenderPostChain measures the CPU and allocation cost of a
// representative four-effect post-processing chain at 1280x720.
func BenchmarkRendererRenderPostChain(b *testing.B) {
	renderer, commands := benchmarkRendererCommands()
	commands = append(commands, render.PostChain{
		Effects: []render.PostEffect{
			render.ColorGrade{Contrast: 0.08, Saturation: 0.1},
			render.Bloom{Threshold: 0.62, Intensity: 0.45, Passes: 2},
			render.Vignette{Strength: 0.35, Softness: 0.6},
			render.LensRain{Density: 0.4, Size: 32, Speed: 0.15, Seed: 0x0885eed},
		},
	})

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = renderer.Render(commands)
	}
}

func benchmarkRendererCommands() (raster.Renderer, []render.Command) {
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

	return renderer, commands
}
