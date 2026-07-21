package raster

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestGoldenTransforms(t *testing.T) {
	img := goldenRenderer().Render([]render.Command{
		// Rect rotated 30° around its center
		render.DrawRect{
			BaseCommand: render.BaseCommand{
				Transform: math2d.Rotate(30 * math.Pi / 180).About(math2d.Vec2{X: 18, Y: 15}),
			},
			Rect: math2d.NewRect(math2d.Vec2{X: 10, Y: 8}, math2d.Vec2{X: 26, Y: 22}),
			Fill: render.RGBA(0.9, 0.25, 0.18, 1),
		},
		// Circle with non-uniform scale (becomes ellipse)
		render.DrawCircle{
			BaseCommand: render.BaseCommand{
				Transform: math2d.Scale(math2d.Vec2{X: 2, Y: 0.5}),
			},
			Center: math2d.Vec2{X: 55, Y: 18},
			Radius: 8,
			Fill:   render.RGBA(0.22, 0.74, 0.36, 1),
		},
		// Line with skew
		render.DrawLine{
			BaseCommand: render.BaseCommand{
				Transform: math2d.Shear(0.5, 0),
			},
			From:  math2d.Vec2{X: 70, Y: 8},
			To:    math2d.Vec2{X: 90, Y: 26},
			Width: 2,
			Color: render.RGBA(0.95, 0.95, 0.35, 1),
		},
		// Polygon translated
		render.DrawPolygon{
			BaseCommand: render.BaseCommand{
				Transform: math2d.Translate(math2d.Vec2{X: 5, Y: 0}),
			},
			Points: []math2d.Vec2{
				{X: 5, Y: 38},
				{X: 20, Y: 44},
				{X: 14, Y: 58},
			},
			Fill:        render.RGBA(1, 0.35, 0.7, 1),
			Stroke:      render.RGBA(0.88, 0.95, 1, 1),
			StrokeWidth: 1,
		},
		// Ellipse rotated
		render.DrawEllipse{
			BaseCommand: render.BaseCommand{
				Transform: math2d.Rotate(45 * math.Pi / 180).About(math2d.Vec2{X: 55, Y: 50}),
			},
			Center:  math2d.Vec2{X: 55, Y: 50},
			RadiusX: 15,
			RadiusY: 6,
			Fill:    render.RGBA(0.12, 0.63, 0.72, 1),
		},
		// Point with translation
		render.DrawPoint{
			BaseCommand: render.BaseCommand{
				Transform: math2d.Translate(math2d.Vec2{X: 10, Y: 5}),
			},
			Center: math2d.Vec2{X: 70, Y: 40},
			Radius: 3,
			Color:  render.RGBA(1, 0.55, 0.2, 0.7),
		},
		// Arc with rotation
		render.DrawArc{
			BaseCommand: render.BaseCommand{
				Transform: math2d.Rotate(-20 * math.Pi / 180).About(math2d.Vec2{X: 85, Y: 58}),
			},
			Center:     math2d.Vec2{X: 85, Y: 58},
			Radius:     10,
			StartAngle: 0,
			EndAngle:   math.Pi * 1.2,
			Width:      2,
			Color:      render.RGBA(0.4, 0.75, 1, 1),
		},
		// Rect with zero transform (identity) — proves no regression
		render.DrawRect{
			BaseCommand: render.BaseCommand{
				LayerValue: 1,
			},
			Rect:   math2d.NewRect(math2d.Vec2{X: 12, Y: 58}, math2d.Vec2{X: 30, Y: 70}),
			Stroke: render.RGBA(0.55, 1, 0.38, 0.62),
			Fill:   render.RGBA(0.55, 1, 0.38, 0.15),
		},
	})

	assertGolden(t, "transforms", img)
}
