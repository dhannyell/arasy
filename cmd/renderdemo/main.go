package main

import (
	"image/png"
	"math"
	"os"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/raster"
	"github.com/dhannyell/frameforge/render"
)

func main() {
	const cornerArcInset = 0.08

	penaltyMarkDistance := 11.00
	penaltyArcRadius := 9.15
	fieldLeft := 5.25
	fieldTop := 3.4
	fieldRight := 110.25

	leftPenaltySpot := math2d.Vec2{
		X: fieldLeft + penaltyMarkDistance,
		Y: fieldTop + 34,
	}
	centerSpot := math2d.Vec2{
		X: 57.75,
		Y: fieldTop + 34,
	}

	rightPenaltySpot := math2d.Vec2{
		X: fieldRight - penaltyMarkDistance,
		Y: fieldTop + 34,
	}

	renderer := raster.Renderer{
		Width:  1920,
		Height: 1080,
		Camera: render.Camera{
			World: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 0},
				math2d.Vec2{X: 115.5, Y: 74.8},
			),
			Viewport: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 0},
				math2d.Vec2{X: 1920, Y: 1080},
			),
			Aspect: render.AspectExpand,
		},
		Clear: render.RGBA(0.05, 0.08, 0.10, 1),
	}

	img := renderer.Render([]render.Command{
		// Background
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 0},
				math2d.Vec2{X: 115.5, Y: 74.8},
			),
			Fill:  render.RGBA(0.22, 0.49, 0.17, 1),
			Layer: 0,
		},
		// Group of small rectangles to simulate grass height difference effect
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 3},
				math2d.Vec2{X: 115.5, Y: 9.715},
			),
			Fill:  render.RGBA(0.14, 0.33, 0.11, 1),
			Layer: 1,
		},
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 22.43},
				math2d.Vec2{X: 115.5, Y: 9.715},
			),
			Fill:  render.RGBA(0.14, 0.33, 0.11, 1),
			Layer: 1,
		},
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 41.86},
				math2d.Vec2{X: 115.5, Y: 9.715},
			),
			Fill:  render.RGBA(0.14, 0.33, 0.11, 1),
			Layer: 1,
		},
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 61.29},
				math2d.Vec2{X: 115.5, Y: 9.715},
			),
			Fill:  render.RGBA(0.14, 0.33, 0.11, 1),
			Layer: 1,
		},
		// Field with margin
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 5.25, Y: 3.4},
				math2d.Vec2{X: 105, Y: 68},
			),
			Stroke:      render.RGBA(1, 1, 1, 1),
			StrokeWidth: 0.3,
			Layer:       2,
		},
		// Center Field Circle
		render.DrawCircle{
			Radius:      9.15,
			Center:      centerSpot,
			Stroke:      render.RGBA(1, 1, 1, 1),
			StrokeWidth: 0.3,
			Layer:       2,
		},
		// Center mark
		render.DrawPoint{
			Center: centerSpot,
			Radius: 0.35,
			Color:  render.RGBA(1, 1, 1, 1),
			Layer:  3,
		},
		// Left Penalty area
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 5.25, Y: 16.24},
				math2d.Vec2{X: 16.5, Y: 40.32},
			),
			Stroke:      render.RGBA(1, 1, 1, 1),
			StrokeWidth: 0.3,
			Layer:       2,
		},
		// Left Goal area
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 5.25, Y: 28.24},
				math2d.Vec2{X: 5.5, Y: 18.32},
			),
			Stroke:      render.RGBA(1, 1, 1, 1),
			StrokeWidth: 0.3,
			Layer:       2,
		},
		// Lef penalty area Arc
		render.DrawArc{
			Center:     leftPenaltySpot,
			Radius:     penaltyArcRadius,
			StartAngle: -0.95,
			EndAngle:   0.95,
			Width:      0.3,
			Color:      render.RGBA(1, 1, 1, 1),
			Layer:      2,
		},
		// Left penalty mark
		render.DrawPoint{
			Center: leftPenaltySpot,
			Radius: 0.35,
			Color:  render.RGBA(1, 1, 1, 1),
			Layer:  3,
		},
		// Right Penalty area
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 93.75, Y: 16.24},
				math2d.Vec2{X: 16.5, Y: 40.32},
			),
			Stroke:      render.RGBA(1, 1, 1, 1),
			StrokeWidth: 0.3,
			Layer:       2,
		},
		// Right Penalty Arc
		render.DrawArc{
			Center:     rightPenaltySpot,
			Radius:     penaltyArcRadius,
			StartAngle: math.Pi - 0.95,
			EndAngle:   math.Pi + 0.95,
			Width:      0.3,
			Color:      render.RGBA(1, 1, 1, 1),
			Layer:      2,
		},
		// Right penalty mark
		render.DrawPoint{
			Center: rightPenaltySpot,
			Radius: 0.35,
			Color:  render.RGBA(1, 1, 1, 1),
			Layer:  3,
		},
		// Right Goal area
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 104.75, Y: 28.24},
				math2d.Vec2{X: 5.5, Y: 18.32},
			),
			Stroke:      render.RGBA(1, 1, 1, 1),
			StrokeWidth: 0.3,
			Layer:       2,
		},
		// top-left corner
		render.DrawArc{
			Center:     math2d.Vec2{X: 5.25, Y: 3.4},
			Radius:     2.5,
			StartAngle: 0 + cornerArcInset,
			EndAngle:   math.Pi/2 - cornerArcInset,
			Width:      0.35,
			Color:      render.RGBA(1, 1, 1, 1),
			Layer:      2,
		},
		// top-right corner
		render.DrawArc{
			Center:     math2d.Vec2{X: 110.25, Y: 3.4},
			Radius:     2.5,
			StartAngle: math.Pi/2 + cornerArcInset,
			EndAngle:   math.Pi - cornerArcInset,
			Width:      0.35,
			Color:      render.RGBA(1, 1, 1, 1),
			Layer:      2,
		},
		// bottom-right corner
		render.DrawArc{
			Center:     math2d.Vec2{X: 110.25, Y: 71.4},
			Radius:     2.5,
			StartAngle: math.Pi + cornerArcInset,
			EndAngle:   3*math.Pi/2 - cornerArcInset,
			Width:      0.35,
			Color:      render.RGBA(1, 1, 1, 1),
			Layer:      2,
		},
		// bottom-left corner
		render.DrawArc{
			Center:     math2d.Vec2{X: 5.25, Y: 71.4},
			Radius:     2.5,
			StartAngle: 3*math.Pi/2 + cornerArcInset,
			EndAngle:   2*math.Pi - cornerArcInset,
			Width:      0.35,
			Color:      render.RGBA(1, 1, 1, 1),
			Layer:      2,
		},
		// Center line
		render.DrawLine{
			From:  math2d.Vec2{X: 57.75, Y: 3.4},
			To:    math2d.Vec2{X: 57.75, Y: 71},
			Width: 0.3,
			Color: render.RGBA(1, 1, 1, 1),
			Layer: 3,
		},
	})

	if err := os.MkdirAll("output", 0755); err != nil {
		panic(err)
	}

	file, err := os.Create("output/renderdemo.png")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		panic(err)
	}

	println("wrote output/renderdemo.png")
}
