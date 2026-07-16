package raster

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestGoldenRects(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: 8, Y: 8}, math2d.Vec2{X: 22, Y: 16}),
			Fill: render.RGBA(0.9, 0.25, 0.18, 1),
		},
		render.DrawRect{
			Rect:        math2d.NewRect(math2d.Vec2{X: 38, Y: 10}, math2d.Vec2{X: 20, Y: 20}),
			Stroke:      render.RGBA(0.88, 0.95, 1, 1),
			StrokeWidth: 2,
		},
		render.DrawRect{
			Rect:         math2d.NewRect(math2d.Vec2{X: 68, Y: 9}, math2d.Vec2{X: 22, Y: 18}),
			Fill:         render.RGBA(0.12, 0.63, 0.72, 1),
			Stroke:       render.RGBA(1, 0.85, 0.25, 0.85),
			StrokeWidth:  3,
			CornerRadius: 4,
		},
		render.DrawRect{
			Rect:        math2d.NewRect(math2d.Vec2{X: 18, Y: 42}, math2d.Vec2{X: 0, Y: 20}),
			Fill:        render.RGBA(1, 1, 1, 1),
			Stroke:      render.RGBA(1, 0, 1, 1),
			StrokeWidth: 1,
		},
		render.DrawRect{
			BaseCommand: render.BaseCommand{
				Clip: math2d.NewRect(math2d.Vec2{X: 56, Y: 42}, math2d.Vec2{X: 22, Y: 14}),
			},
			Rect:         math2d.NewRect(math2d.Vec2{X: 48, Y: 36}, math2d.Vec2{X: 40, Y: 26}),
			Fill:         render.RGBA(0.35, 0.9, 1, 0.78),
			Stroke:       render.RGBA(1, 1, 1, 1),
			StrokeWidth:  1.4,
			CornerRadius: 5,
		},
	})

	assertGolden(t, "rects", img)
}

func TestGoldenCirclesPoints(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawCircle{
			Center: math2d.Vec2{X: 22, Y: 21},
			Radius: 10,
			Fill:   render.RGBA(0.22, 0.74, 0.36, 1),
		},
		render.DrawCircle{
			Center:      math2d.Vec2{X: 52, Y: 22},
			Radius:      10,
			Stroke:      render.RGBA(0.95, 0.95, 0.35, 1),
			StrokeWidth: 2,
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 77, Y: 21},
			Radius: 0.45,
			Fill:   render.RGBA(1, 0.35, 0.7, 1),
		},
		render.DrawPoint{
			Center: math2d.Vec2{X: 28, Y: 54},
			Radius: 2.25,
			Color:  render.RGBA(0.4, 0.75, 1, 1),
		},
		render.DrawPoint{
			Center: math2d.Vec2{X: 63, Y: 52},
			Radius: 4,
			Color:  render.RGBA(1, 0.55, 0.2, 0.7),
		},
	})

	assertGolden(t, "circles_points", img)
}

func TestGoldenLinesArcs(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawLine{
			From:  math2d.Vec2{X: 10, Y: 12},
			To:    math2d.Vec2{X: 42, Y: 36},
			Width: 1.4,
			Color: render.RGBA(0.94, 0.35, 0.2, 1),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 90, Y: 12},
			To:    math2d.Vec2{X: 58, Y: 36},
			Width: 2.4,
			Color: render.RGBA(0.28, 0.8, 0.95, 1),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 10, Y: 63},
			To:    math2d.Vec2{X: 42, Y: 39},
			Width: 3.4,
			Color: render.RGBA(0.82, 0.72, 1, 0.78),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 90, Y: 63},
			To:    math2d.Vec2{X: 58, Y: 39},
			Width: 0.8,
			Color: render.RGBA(0.7, 1, 0.45, 1),
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: 32, Y: 53},
			Radius:     12,
			StartAngle: math.Pi * 1.72,
			EndAngle:   math.Pi * 0.28,
			Width:      2,
			Color:      render.RGBA(1, 0.82, 0.22, 1),
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: 71, Y: 53},
			Radius:     13,
			StartAngle: math.Pi * 0.18,
			EndAngle:   math.Pi * 1.38,
			Width:      3,
			Color:      render.RGBA(1, 0.4, 0.72, 0.82),
		},
	})

	assertGolden(t, "lines_arcs", img)
}

func TestGoldenPolygons(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawPolygon{
			Points: []math2d.Vec2{
				{X: 16.67, Y: 6.75},
				{X: 27.06, Y: 24.75},
				{X: 6.28, Y: 24.75},
			},
			Fill:        render.RGBA(0.95, 0.95, 0.35, 1),
			Stroke:      render.Color{R: 0.1, G: 0.7, B: 0.1, A: 1.0},
			StrokeWidth: 1.0,
		},
		render.DrawPolygon{
			Points: []math2d.Vec2{
				{X: 50.00, Y: 6.75},
				{X: 62.00, Y: 18.75},
				{X: 50.00, Y: 30.75},
				{X: 38.00, Y: 18.75},
			},
			Fill:        render.RGBA(0.23, 0.54, 0.82, 1),
			Stroke:      render.RGBA(0.43, 0.15, 0.95, 1),
			StrokeWidth: 2.0,
		},
		render.DrawPolygon{
			Points: []math2d.Vec2{
				{X: 83.33, Y: 6.75},
				{X: 94.74, Y: 15.04},
				{X: 90.38, Y: 28.46},
				{X: 76.28, Y: 28.46},
				{X: 71.92, Y: 15.04},
			},
			Fill:        render.RGBA(1, 0.35, 0.7, 1),
			Stroke:      render.RGBA(0.88, 0.95, 1, 1),
			StrokeWidth: 3,
		},
		render.DrawPolygon{
			Points: []math2d.Vec2{
				{X: 25.00, Y: 44.25},
				{X: 35.39, Y: 50.25},
				{X: 35.39, Y: 62.25},
				{X: 25.00, Y: 68.25},
				{X: 14.61, Y: 62.25},
				{X: 14.61, Y: 50.25},
			},
			Fill:        render.RGBA(0.4, 0.75, 1, 1),
			Stroke:      render.RGBA(1, 0.85, 0.25, 0.85),
			StrokeWidth: 4,
		},
		render.DrawPolygon{
			Points: []math2d.Vec2{
				{X: 75.00, Y: 44.25},
				{X: 83.49, Y: 47.76},
				{X: 87.00, Y: 56.25},
				{X: 83.49, Y: 64.74},
				{X: 75.00, Y: 68.25},
				{X: 66.51, Y: 64.74},
				{X: 63.00, Y: 56.25},
				{X: 66.51, Y: 47.76},
			},
			Fill:        render.RGBA(1, 0.55, 0.2, 0.7),
			Stroke:      render.RGBA(1, 0, 1, 1),
			StrokeWidth: 5,
		},
		render.DrawPolygon{
			BaseCommand: render.BaseCommand{
				Clip: math2d.NewRect(math2d.Vec2{X: 42, Y: 44}, math2d.Vec2{X: 16, Y: 24}),
			},
			Points: []math2d.Vec2{
				{X: 39, Y: 42},
				{X: 60, Y: 47},
				{X: 56, Y: 70},
				{X: 40, Y: 66},
			},
			Fill:        render.RGBA(0.55, 1, 0.38, 0.62),
			Stroke:      render.RGBA(0.95, 0.95, 1, 0.9),
			StrokeWidth: 1.2,
		},
	})

	assertGolden(t, "polygons", img)
}

func TestGoldenAlpha(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 60, Y: 44}),
			Fill: render.RGBA(0.12, 0.62, 0.78, 0.55),
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 54, Y: 33},
			Radius: 22,
			Fill:   render.RGBA(1, 0.36, 0.18, 0.45),
		},
		render.DrawRect{
			Rect:        math2d.NewRect(math2d.Vec2{X: 24, Y: 22}, math2d.Vec2{X: 42, Y: 31}),
			Stroke:      render.RGBA(1, 1, 1, 0.42),
			StrokeWidth: 4,
			BaseCommand: render.BaseCommand{LayerValue: 2},
		},
		render.DrawRect{
			Rect:        math2d.NewRect(math2d.Vec2{X: 37, Y: 16}, math2d.Vec2{X: 20, Y: 20}),
			Fill:        render.RGBA(1, 0.55, 0.05, 0.22),
			Stroke:      render.RGBA(1, 0.75, 0.05, 0.9),
			StrokeWidth: 0.12,
			BaseCommand: render.BaseCommand{LayerValue: 3},
		},
	})

	assertGolden(t, "alpha", img)
}
