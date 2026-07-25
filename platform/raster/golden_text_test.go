package raster

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"golang.org/x/image/font/gofont/goregular"
)

func TestGoldenText(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawLine{
			From:  math2d.Vec2{X: 8, Y: 20},
			To:    math2d.Vec2{X: 92, Y: 20},
			Width: 0.35,
			Color: render.RGBA(1, 1, 1, 0.28),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 8, Y: 44},
			To:    math2d.Vec2{X: 92, Y: 44},
			Width: 0.35,
			Color: render.RGBA(1, 1, 1, 0.28),
		},
		render.DrawText{
			Position: math2d.Vec2{X: 10, Y: 20},
			Text:     "Left",
			Color:    render.RGBA(0.95, 0.95, 0.95, 1),
			Size:     6.5,
			Align:    render.AlignLeft,
			Baseline: render.BaselineTop,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 50, Y: 20},
			Text:     "Mid",
			Color:    render.RGBA(0.35, 0.9, 1, 1),
			Size:     6.5,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 90, Y: 20},
			Text:     "Right",
			Color:    render.RGBA(1, 0.76, 0.3, 1),
			Size:     6.5,
			Align:    render.AlignRight,
			Baseline: render.BaselineBottom,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 16, Y: 50},
			Text:     "2x",
			Color:    render.RGBA(1, 0.4, 0.55, 0.86),
			Size:     13,
			Align:    render.AlignLeft,
			Baseline: render.BaselineAlphabetic,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 80, Y: 58},
			Text:     "",
			Color:    render.RGBA(1, 1, 1, 1),
			Size:     13,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 64, Y: 58},
			Text:     "Tilt",
			Color:    render.RGBA(1, 0.85, 0.25, 1),
			Size:     7.5,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
			Rotation: -math.Pi / 5,
		},
	})

	assertGolden(t, "text", img)
}

func TestGoldenTypeface(t *testing.T) {
	face, err := typeface.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parse goregular: %v", err)
	}
	renderer := goldenRenderer()
	renderer.Fonts = typeface.Registry{nil, face}
	img := renderer.Render([]render.Command{
		render.DrawText{
			Position: math2d.Vec2{X: 7, Y: 8},
			Text:     "Go Regular 6",
			Color:    render.RGBA(0.92, 0.95, 1, 1),
			Size:     6,
			Font:     1,
			Baseline: render.BaselineTop,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 50, Y: 24},
			Text:     "Center 10",
			Color:    render.RGBA(0.3, 0.88, 1, 1),
			Size:     10,
			Font:     1,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 93, Y: 45},
			Text:     "Right 16",
			Color:    render.RGBA(1, 0.72, 0.25, 1),
			Size:     16,
			Font:     1,
			Align:    render.AlignRight,
			Baseline: render.BaselineBottom,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 23, Y: 49},
			Text:     "Rotate",
			Color:    render.RGBA(1, 0.42, 0.63, 0.9),
			Size:     9,
			Font:     1,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
			Rotation: -math.Pi / 7,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 7, Y: 68},
			Text:     "builtin 0",
			Color:    render.RGBA(0.62, 1, 0.48, 1),
			Size:     6,
			Baseline: render.BaselineBottom,
		},
		render.DrawText{
			Position: math2d.Vec2{X: 93, Y: 67},
			Text:     "fallback 99",
			Color:    render.RGBA(0.82, 0.72, 1, 1),
			Size:     6,
			Font:     99,
			Align:    render.AlignRight,
			Baseline: render.BaselineBottom,
		},
	})

	// x/image's anti-aliased glyph rasterizer can round edge coverage by one
	// channel value across platforms. Keep this tolerance local to the external
	// typeface golden; the remaining raster goldens stay pixel-exact.
	assertGoldenTolerance(t, "typeface", img, 1)
}
