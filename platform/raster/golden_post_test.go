package raster

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestGoldenPostVignette(t *testing.T) {
	assertGolden(t, "post_vignette", postRenderer().Render(postCommands(render.Vignette{
		Color: render.RGBA(0.01, 0.03, 0.08, 1), Strength: 0.85, Softness: 0.65,
	})))
}

func TestGoldenPostColorGrade(t *testing.T) {
	assertGolden(t, "post_color_grade", postRenderer().Render(postCommands(render.ColorGrade{
		Brightness: 0.04, Contrast: 0.18, Saturation: -0.2, Tint: render.RGBA(1, 0.92, 0.78, 1),
	})))
}

func TestGoldenPostPixelate(t *testing.T) {
	assertGolden(t, "post_pixelate", postRenderer().Render(postCommands(render.Pixelate{Size: 5})))
}

func TestGoldenPostChromaticAberration(t *testing.T) {
	assertGolden(t, "post_chromatic", postRenderer().Render(postCommands(render.ChromaticAberration{Strength: 3})))
}

func TestGoldenPostBloom(t *testing.T) {
	assertGolden(t, "post_bloom", postRenderer().Render(postCommands(render.Bloom{
		Threshold: 0.55, Intensity: 0.85, Passes: 3,
	})))
}

func TestGoldenPostShockwave(t *testing.T) {
	assertGolden(t, "post_shockwave", postRenderer().Render(postCommands(render.Shockwave{
		Center: math2d.Vec2{X: 50, Y: 37}, Radius: 22, Width: 7, Strength: 4,
	})))
}

func TestGoldenPostLensRain(t *testing.T) {
	assertGolden(t, "post_lensrain", postRenderer().Render(postCommandsAt(
		render.LensRain{Density: 0.7, Size: 24, Speed: 0.15, Refraction: 0.025, Seed: 77},
		0.65,
	)))
}

func TestGoldenPostChainAndBelowLayer(t *testing.T) {
	commands := []render.Command{
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: 1},
			Rect:        math2d.NewRect(math2d.Vec2{X: 8, Y: 8}, math2d.Vec2{X: 92, Y: 67}),
			Fill:        render.RGBA(0.2, 0.38, 0.65, 1),
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Center:      math2d.Vec2{X: 50, Y: 38},
			Radius:      15,
			Fill:        render.RGBA(1, 0.64, 0.18, 0.95),
		},
		render.PostChain{
			Effects: []render.PostEffect{
				render.ColorGrade{Brightness: 0.04, Contrast: 0.12, Tint: render.RGBA(1, 0.95, 0.82, 1)},
				render.Vignette{Color: render.RGBA(0.02, 0.03, 0.08, 1), Strength: 0.55, Softness: 0.7},
			},
			BelowLayer: 100,
		},
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: 100},
			Rect:        math2d.NewRect(math2d.Vec2{X: 10, Y: 69}, math2d.Vec2{X: 90, Y: 73}),
			Fill:        render.RGBA(0.95, 0.95, 0.98, 1),
		},
	}

	assertGolden(t, "post_chain", postRenderer().Render(commands))
}

func postRenderer() *Renderer {
	r := goldenRenderer()
	r.Camera.World = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75})
	return r
}

func postCommands(effect render.PostEffect) []render.Command {
	return postCommandsAt(effect, 0)
}

func postCommandsAt(effect render.PostEffect, time float64) []render.Command {
	return []render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: 5, Y: 5}, math2d.Vec2{X: 95, Y: 70}),
			Fill: render.RGBA(0.12, 0.2, 0.42, 1),
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 28, Y: 28}, Radius: 12,
			Fill: render.RGBA(1, 0.3, 0.08, 0.9),
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 72, Y: 42}, Radius: 18,
			Fill: render.RGBA(0.1, 0.9, 0.75, 0.8),
		},
		render.DrawRect{
			BaseCommand: render.BaseCommand{Transform: math2d.Rotate(math.Pi / 8).About(math2d.Vec2{X: 50, Y: 37})},
			Rect:        math2d.NewRect(math2d.Vec2{X: 45, Y: 12}, math2d.Vec2{X: 55, Y: 62}),
			Fill:        render.RGBA(0.92, 0.85, 0.3, 0.75),
		},
		render.PostChain{Effects: []render.PostEffect{effect}, Time: time},
	}
}
