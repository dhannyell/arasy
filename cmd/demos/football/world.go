package football

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

const (
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

	ballInvMass       = 1.0
	playerInvMass     = 0.2
	ballRestitution   = 0.8
	playerRestitution = 0.35
	physicsCellSize   = 8.0
)

var (
	worldOrigin     = math2d.Vec2{X: 0, Y: 0}
	worldSize       = math2d.Vec2{X: worldWidth, Y: worldHeight}
	grassColor      = render.RGBA(0.22, 0.49, 0.17, 1)
	darkGrassColor  = render.RGBA(0.14, 0.33, 0.11, 1)
	lineColor       = render.RGBA(1, 1, 1, 1)
	homeTeamColor   = render.RGBA(0.08, 0.22, 0.95, 1)
	awayTeamColor   = render.RGBA(0.95, 0.12, 0.12, 1)
	ballColor       = render.RGBA(0.98, 0.98, 0.94, 1)
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

type DebugConfig struct {
	ShowColliders bool
	ShowContacts  bool

	ColliderColor       render.Color
	ColliderStrokeWidth float64
	ColliderLayer       int

	ContactColor        render.Color
	ContactPointRadius  float64
	ContactLineWidth    float64
	ContactNormalLength float64
	ContactLayer        int

	ShowGrid        bool
	GridColor       render.Color
	GridDashLength  float64
	GridStrokeWidth float64
	GridLayer       int

	ShowActiveGrid        bool
	ActiveGridFill        render.Color
	ActiveGridStroke      render.Color
	ActiveGridStrokeWidth float64
	ActiveGridLayer       int

	ShowCandidateGrid        bool
	CandidateGridFill        render.Color
	CandidateGridStroke      render.Color
	CandidateGridStrokeWidth float64
	CandidateGridLayer       int
}

type side int

const (
	leftSide side = iota
	rightSide
)

func BuildDemoWorld() *scene.World {
	world := scene.NewWorld()

	ball := spawnCircleEntity(
		world,
		fieldCenter,
		0.45,
		0.45,
		scene.Body{InvMass: ballInvMass, Restitution: ballRestitution},
		ballColor,
		layerBall,
	)
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
		player := spawnCircleEntity(
			world,
			position,
			1.2,
			1.1,
			scene.Body{InvMass: playerInvMass, Restitution: playerRestitution},
			homeTeamColor,
			layerPlayers,
		)
		if i == 3 {
			world.SetVelocity(player, scene.Velocity{
				Linear: math2d.Vec2{X: 8, Y: 0},
			})
		}
	}

	for i, position := range awayPositions {
		player := spawnCircleEntity(
			world,
			position,
			1.2,
			1.1,
			scene.Body{InvMass: playerInvMass, Restitution: playerRestitution},
			awayTeamColor,
			layerPlayers,
		)
		if i == 3 {
			world.SetVelocity(player, scene.Velocity{
				Linear: math2d.Vec2{X: -2, Y: 0},
			})
		}
	}

	return world
}

func AppendDebugCommands(commands *[]render.Command, world *scene.World, config DebugConfig) {
	if config.ShowGrid {
		*commands = append(*commands, scene.DebugGridAreaCommands(
			math2d.NewRect(fieldOrigin, fieldSize),
			physicsCellSize,
			config.GridColor,
			config.GridStrokeWidth,
			config.GridDashLength,
			config.GridLayer,
		)...)
	}

	if config.ShowActiveGrid {
		*commands = append(*commands, world.DebugActiveGridCommands(
			config.ActiveGridFill,
			config.ActiveGridStroke,
			config.ActiveGridStrokeWidth,
			config.ActiveGridLayer,
		)...)
	}

	if config.ShowCandidateGrid {
		*commands = append(*commands, world.DebugCandidateGridCommands(
			config.CandidateGridFill,
			config.CandidateGridStroke,
			config.CandidateGridStrokeWidth,
			config.CandidateGridLayer,
		)...)
	}

	if config.ShowColliders {
		*commands = append(
			*commands,
			world.DebugColliderCommands(
				config.ColliderColor,
				config.ColliderStrokeWidth,
				config.ColliderLayer,
			)...,
		)
	}

	if !config.ShowContacts {
		return
	}

	*commands = append(
		*commands,
		world.DebugContactCommands(
			config.ContactColor,
			config.ContactPointRadius,
			config.ContactLineWidth,
			config.ContactNormalLength,
			config.ContactLayer,
		)...,
	)
}

func BuildFieldCommands() []render.Command {
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

func spawnCircleEntity(
	world *scene.World,
	position math2d.Vec2,
	visualRadius float64,
	colliderRadius float64,
	body scene.Body,
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

	world.SetBody(entity, body)

	return entity
}

func backgroundCommand() render.Command {
	return render.DrawRect{
		Rect: math2d.NewRect(worldOrigin, worldSize),
		Fill: grassColor,
		BaseCommand: render.BaseCommand{
			LayerValue: layerGrass,
		},
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
			Fill: darkGrassColor,
			BaseCommand: render.BaseCommand{
				LayerValue: layerStripes,
			},
		})
	}

	return commands
}

func fieldBoundaryCommand() render.Command {
	return render.DrawRect{
		Rect:        math2d.NewRect(fieldOrigin, fieldSize),
		Stroke:      lineColor,
		StrokeWidth: lineWidth,
		BaseCommand: render.BaseCommand{
			LayerValue: layerMarkings,
		},
	}
}

func centerMarkingCommands() []render.Command {
	return []render.Command{
		render.DrawCircle{
			Center:      fieldCenter,
			Radius:      centerCircleRadius,
			Stroke:      lineColor,
			StrokeWidth: lineWidth,
			BaseCommand: render.BaseCommand{
				LayerValue: layerMarkings,
			},
		},
		render.DrawPoint{
			Center: fieldCenter,
			Radius: pointRadius,
			Color:  lineColor,
			BaseCommand: render.BaseCommand{
				LayerValue: layerPoints,
			},
		},
		render.DrawLine{
			From:  math2d.Vec2{X: fieldCenter.X, Y: fieldTop},
			To:    math2d.Vec2{X: fieldCenter.X, Y: fieldTop + fieldHeight},
			Width: lineWidth,
			Color: lineColor,
			BaseCommand: render.BaseCommand{
				LayerValue: layerPoints,
			},
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
			BaseCommand: render.BaseCommand{
				LayerValue: layerMarkings,
			},
		},
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: goalX, Y: goalAreaTop},
				math2d.Vec2{X: goalAreaDepth, Y: goalAreaWidth},
			),
			Stroke:      lineColor,
			StrokeWidth: lineWidth,
			BaseCommand: render.BaseCommand{
				LayerValue: layerMarkings,
			},
		},
		render.DrawArc{
			Center:     spot,
			Radius:     penaltyArcRadius,
			StartAngle: arcStart,
			EndAngle:   arcEnd,
			Width:      lineWidth,
			Color:      lineColor,
			BaseCommand: render.BaseCommand{
				LayerValue: layerMarkings,
			},
		},
		render.DrawPoint{
			Center: spot,
			Radius: pointRadius,
			Color:  lineColor,
			BaseCommand: render.BaseCommand{
				LayerValue: layerPoints,
			},
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
			BaseCommand: render.BaseCommand{
				LayerValue: layerMarkings,
			},
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: fieldRight, Y: fieldTop},
			Radius:     cornerArcRadius,
			StartAngle: math.Pi/2 + cornerArcInset,
			EndAngle:   math.Pi - cornerArcInset,
			Width:      cornerArcWidth,
			Color:      lineColor,
			BaseCommand: render.BaseCommand{
				LayerValue: layerMarkings,
			},
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: fieldRight, Y: fieldBottom},
			Radius:     cornerArcRadius,
			StartAngle: math.Pi + cornerArcInset,
			EndAngle:   3*math.Pi/2 - cornerArcInset,
			Width:      cornerArcWidth,
			Color:      lineColor,
			BaseCommand: render.BaseCommand{
				LayerValue: layerMarkings,
			},
		},
		render.DrawArc{
			Center:     math2d.Vec2{X: fieldLeft, Y: fieldBottom},
			Radius:     cornerArcRadius,
			StartAngle: 3*math.Pi/2 + cornerArcInset,
			EndAngle:   2*math.Pi - cornerArcInset,
			Width:      cornerArcWidth,
			Color:      lineColor,
			BaseCommand: render.BaseCommand{
				LayerValue: layerMarkings,
			},
		},
	}
}

func penaltyArcAngle() float64 {
	return math.Acos((penaltyAreaDepth - penaltyMarkDistance) / penaltyArcRadius)
}
