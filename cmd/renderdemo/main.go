package main

import (
	"image/png"
	"math"
	"os"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/raster"
	"github.com/dhannyell/frameforge/render"
)

const (
	imageWidth  = 1920
	imageHeight = 1080

	worldWidth  = 115.5
	worldHeight = 74.8

	fieldWidth  = 105.0
	fieldHeight = 68.0
	fieldLeft   = 5.25
	fieldTop    = 3.4

	lineWidth           = 0.3
	pointRadius         = 0.35
	centerCircleRadius  = 9.15
	penaltyMarkDistance = 11.0
	penaltyArcRadius    = 9.15
	penaltyAreaDepth    = 16.5
	penaltyAreaWidth    = 40.32
	goalAreaDepth       = 5.5
	goalAreaWidth       = 18.32
	cornerArcRadius     = 2.5
	cornerArcWidth      = 0.35
	cornerArcInset      = 0.08

	layerGrass    = 0
	layerStripes  = 1
	layerMarkings = 2
	layerPoints   = 3
)

var (
	grassColor      = render.RGBA(0.22, 0.49, 0.17, 1)
	darkGrassColor  = render.RGBA(0.14, 0.33, 0.11, 1)
	lineColor       = render.RGBA(1, 1, 1, 1)
	clearColor      = render.RGBA(0.05, 0.08, 0.10, 1)
	worldOrigin     = math2d.Vec2{X: 0, Y: 0}
	worldSize       = math2d.Vec2{X: worldWidth, Y: worldHeight}
	viewportOrigin  = math2d.Vec2{X: 0, Y: 0}
	viewportSize    = math2d.Vec2{X: imageWidth, Y: imageHeight}
	fieldOrigin     = math2d.Vec2{X: fieldLeft, Y: fieldTop}
	fieldSize       = math2d.Vec2{X: fieldWidth, Y: fieldHeight}
	fieldCenter     = math2d.Vec2{X: fieldLeft + fieldWidth/2, Y: fieldTop + fieldHeight/2}
	leftPenaltySpot = math2d.Vec2{
		X: fieldLeft + penaltyMarkDistance,
		Y: fieldCenter.Y,
	}
	rightPenaltySpot = math2d.Vec2{
		X: fieldLeft + fieldWidth - penaltyMarkDistance,
		Y: fieldCenter.Y,
	}
)

type side int

const (
	leftSide side = iota
	rightSide
)

func main() {
	renderer := raster.Renderer{
		Width:  imageWidth,
		Height: imageHeight,
		Camera: render.Camera{
			World:    math2d.NewRect(worldOrigin, worldSize),
			Viewport: math2d.NewRect(viewportOrigin, viewportSize),
			Aspect:   render.AspectExpand,
		},
		Clear: clearColor,
	}

	img := renderer.Render(buildFieldCommands())

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

func buildFieldCommands() []render.Command {
	commands := make([]render.Command, 0, 24)

	commands = append(commands, backgroundCommand())
	commands = append(commands, grassStripeCommands()...)
	commands = append(commands, fieldBoundaryCommand())
	commands = append(commands, centerMarkingCommands()...)
	commands = append(commands, penaltyAreaCommands(leftSide)...)
	commands = append(commands, penaltyAreaCommands(rightSide)...)
	commands = append(commands, cornerArcCommands()...)

	return commands
}

func backgroundCommand() render.Command {
	return render.DrawRect{
		Rect:  math2d.NewRect(worldOrigin, worldSize),
		Fill:  grassColor,
		Layer: layerGrass,
	}
}

func grassStripeCommands() []render.Command {
	const stripeHeight = 9.715

	stripeStarts := []float64{3, 22.43, 41.86, 61.29}
	commands := make([]render.Command, 0, len(stripeStarts))

	for _, y := range stripeStarts {
		commands = append(commands, render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 0, Y: y},
				math2d.Vec2{X: worldWidth, Y: stripeHeight},
			),
			Fill:  darkGrassColor,
			Layer: layerStripes,
		})
	}

	return commands
}

func fieldBoundaryCommand() render.Command {
	return render.DrawRect{
		Rect:        math2d.NewRect(fieldOrigin, fieldSize),
		Stroke:      lineColor,
		StrokeWidth: lineWidth,
		Layer:       layerMarkings,
	}
}

func centerMarkingCommands() []render.Command {
	return []render.Command{
		render.DrawCircle{
			Center:      fieldCenter,
			Radius:      centerCircleRadius,
			Stroke:      lineColor,
			StrokeWidth: lineWidth,
			Layer:       layerMarkings,
		},
		render.DrawPoint{
			Center: fieldCenter,
			Radius: pointRadius,
			Color:  lineColor,
			Layer:  layerPoints,
		},
		render.DrawLine{
			From:  math2d.Vec2{X: fieldCenter.X, Y: fieldTop},
			To:    math2d.Vec2{X: fieldCenter.X, Y: fieldTop + fieldHeight},
			Width: lineWidth,
			Color: lineColor,
			Layer: layerPoints,
		},
	}
}

func penaltyAreaCommands(s side) []render.Command {
	areaX := fieldLeft
	goalX := fieldLeft
	spot := leftPenaltySpot
	arcStart := -penaltyArcAngle()
	arcEnd := penaltyArcAngle()

	if s == rightSide {
		areaX = fieldLeft + fieldWidth - penaltyAreaDepth
		goalX = fieldLeft + fieldWidth - goalAreaDepth
		spot = rightPenaltySpot
		arcStart = math.Pi - penaltyArcAngle()
		arcEnd = math.Pi + penaltyArcAngle()
	}

	penaltyAreaTop := fieldCenter.Y - penaltyAreaWidth/2
	goalAreaTop := fieldCenter.Y - goalAreaWidth/2

	return []render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: areaX, Y: penaltyAreaTop},
				math2d.Vec2{X: penaltyAreaDepth, Y: penaltyAreaWidth},
			),
			Stroke:      lineColor,
			StrokeWidth: lineWidth,
			Layer:       layerMarkings,
		},
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: goalX, Y: goalAreaTop},
				math2d.Vec2{X: goalAreaDepth, Y: goalAreaWidth},
			),
			Stroke:      lineColor,
			StrokeWidth: lineWidth,
			Layer:       layerMarkings,
		},
		render.DrawArc{
			Center:     spot,
			Radius:     penaltyArcRadius,
			StartAngle: arcStart,
			EndAngle:   arcEnd,
			Width:      lineWidth,
			Color:      lineColor,
			Layer:      layerMarkings,
		},
		render.DrawPoint{
			Center: spot,
			Radius: pointRadius,
			Color:  lineColor,
			Layer:  layerPoints,
		},
	}
}

func cornerArcCommands() []render.Command {
	fieldRight := fieldLeft + fieldWidth
	fieldBottom := fieldTop + fieldHeight

	return []render.Command{
		render.DrawArc{
			Center:     math2d.Vec2{X: fieldLeft, Y: fieldTop},
			Radius:     cornerArcRadius,
			StartAngle: cornerArcInset,
			EndAngle:   math.Pi/2 - cornerArcInset,
			Width:      cornerArcWidth,
			Color:      lineColor,
			Layer:      layerMarkings,
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: fieldRight, Y: fieldTop},
			Radius:     cornerArcRadius,
			StartAngle: math.Pi/2 + cornerArcInset,
			EndAngle:   math.Pi - cornerArcInset,
			Width:      cornerArcWidth,
			Color:      lineColor,
			Layer:      layerMarkings,
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: fieldRight, Y: fieldBottom},
			Radius:     cornerArcRadius,
			StartAngle: math.Pi + cornerArcInset,
			EndAngle:   3*math.Pi/2 - cornerArcInset,
			Width:      cornerArcWidth,
			Color:      lineColor,
			Layer:      layerMarkings,
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: fieldLeft, Y: fieldBottom},
			Radius:     cornerArcRadius,
			StartAngle: 3*math.Pi/2 + cornerArcInset,
			EndAngle:   2*math.Pi - cornerArcInset,
			Width:      cornerArcWidth,
			Color:      lineColor,
			Layer:      layerMarkings,
		},
	}
}

func penaltyArcAngle() float64 {
	return math.Acos((penaltyAreaDepth - penaltyMarkDistance) / penaltyArcRadius)
}
