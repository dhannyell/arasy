package main

import (
	"image"
	"image/color/palette"
	"image/gif"
	"image/png"
	"math"
	"os"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/raster"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

const (
	imageWidth  = 1920
	imageHeight = 1080
	gifWidth    = 640
	gifHeight   = 360
	frameCount  = 250
	frameDelay  = 6
	frameDT     = 1.0 / 30.0

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
	layerPlayers  = 10
	layerBall     = 11
)

var (
	grassColor      = render.RGBA(0.22, 0.49, 0.17, 1)
	darkGrassColor  = render.RGBA(0.14, 0.33, 0.11, 1)
	lineColor       = render.RGBA(1, 1, 1, 1)
	homeTeamColor   = render.RGBA(0.08, 0.22, 0.95, 1)
	awayTeamColor   = render.RGBA(0.95, 0.12, 0.12, 1)
	ballColor       = render.RGBA(0.98, 0.98, 0.94, 1)
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

	world := buildDemoWorld()

	commands := buildFieldCommands()
	commands = append(commands, world.RenderCommands()...)

	img := renderer.Render(commands)

	if err := os.MkdirAll("output", 0755); err != nil {
		panic(err)
	}

	if err := savePNG("output/renderdemo.png", img); err != nil {
		panic(err)
	}

	gifRenderer := renderer
	gifRenderer.Width = gifWidth
	gifRenderer.Height = gifHeight
	gifRenderer.Camera.Viewport = math2d.NewRect(
		viewportOrigin,
		math2d.Vec2{X: gifWidth, Y: gifHeight},
	)

	if err := saveGIF("output/renderdemo.gif", gifRenderer, buildDemoWorld()); err != nil {
		panic(err)
	}

	println("wrote output/renderdemo.png")
	println("wrote output/renderdemo.gif")
}

func savePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	return png.Encode(file, img)
}

func saveGIF(path string, renderer raster.Renderer, world *scene.World) error {
	animation := &gif.GIF{
		Image: make([]*image.Paletted, 0, frameCount),
		Delay: make([]int, 0, frameCount),
	}

	for range frameCount {
		commands := buildFieldCommands()
		commands = append(commands, world.RenderCommands()...)

		img := renderer.Render(commands)
		animation.Image = append(animation.Image, palettedImage(img))
		animation.Delay = append(animation.Delay, frameDelay)

		world.Step(frameDT, scene.StepConfig{
			Bounds: math2d.NewRect(fieldOrigin, fieldSize),
		})
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
		}
	}(file)

	return gif.EncodeAll(file, animation)
}

func palettedImage(img image.Image) *image.Paletted {
	bounds := img.Bounds()
	paletted := image.NewPaletted(bounds, palette.Plan9)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			paletted.Set(x, y, img.At(x, y))
		}
	}

	return paletted
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

func buildDemoWorld() *scene.World {
	world := scene.NewWorld()

	ball := spawnCircleEntity(world, fieldCenter, 0.45, 0.45, ballColor, layerBall)
	world.SetVelocity(ball, scene.Velocity{
		Linear: math2d.Vec2{X: 15.0, Y: -2.5},
	})
	world.SetDamping(ball, scene.Damping{
		Linear: 0.15,
	})

	homePositions := []math2d.Vec2{
		{X: fieldLeft + 18, Y: fieldCenter.Y},
		{X: fieldLeft + 34, Y: fieldTop + 20},
		{X: fieldLeft + 34, Y: fieldTop + 48},
		{X: fieldLeft + 48, Y: fieldCenter.Y},
	}

	awayPositions := []math2d.Vec2{
		{X: fieldLeft + fieldWidth - 18, Y: fieldCenter.Y},
		{X: fieldLeft + fieldWidth - 34, Y: fieldTop + 20},
		{X: fieldLeft + fieldWidth - 34, Y: fieldTop + 48},
		{X: fieldLeft + fieldWidth - 48, Y: fieldCenter.Y},
	}

	for i, position := range homePositions {
		player := spawnCircleEntity(world, position, 1.2, 1.1, homeTeamColor, layerPlayers)
		if i == 3 {
			world.SetVelocity(player, scene.Velocity{
				Linear: math2d.Vec2{X: 8, Y: 0},
			})
		}
	}

	for i, position := range awayPositions {
		player := spawnCircleEntity(world, position, 1.2, 1.1, awayTeamColor, layerPlayers)
		if i == 3 {
			world.SetVelocity(player, scene.Velocity{
				Linear: math2d.Vec2{X: -2, Y: 0},
			})
		}
	}

	return world
}

func spawnCircleEntity(
	world *scene.World,
	position math2d.Vec2,
	visualRadius float64,
	colliderRadius float64,
	color render.Color,
	layer int,
) scene.EntityID {
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})

	world.SetCircleSprite(entity, scene.CircleSprite{
		Radius: visualRadius,
		Color:  color,
		Layer:  layer,
	})

	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: colliderRadius,
	})

	return entity
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
