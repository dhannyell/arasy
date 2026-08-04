package raster

import (
	"image/color"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// The additive command path composes through a scratch layer instead of the
// source-over rasterizers; this pins its defining property — overlapping
// light sums toward white instead of saturating at the source color.
func TestAdditiveCommandsAccumulateLight(t *testing.T) {
	r := &Renderer{
		Width: 4, Height: 4,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}),
		},
	}

	glow := render.DrawRect{
		BaseCommand: render.BaseCommand{BlendMode: render.BlendAdditive},
		Rect:        math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}),
		Fill:        render.RGBA(0.5, 0, 0, 1),
	}
	img := r.Render([]render.Command{glow, glow})

	want := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	if got := img.RGBAAt(1, 1); got != want {
		t.Fatalf("two additive half-red rects = %#v, want saturated %#v", got, want)
	}

	// The same pair under source-over never exceeds the source color.
	over := glow
	over.BlendMode = render.BlendAlpha
	img = r.Render([]render.Command{over, over})
	if got := img.RGBAAt(1, 1); got.R != 128 {
		t.Fatalf("two source-over half-red rects R = %d, want 128", got.R)
	}
}
