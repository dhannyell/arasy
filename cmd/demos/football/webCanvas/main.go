package main

import (
	"fmt"

	"github.com/dhannyell/frameforge/cmd/demos/football"
	"github.com/dhannyell/frameforge/debug"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/web/canvas"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

const (
	imageWidth      = 1920
	imageHeight     = 1080
	renderFPS       = 60
	simulationHz    = 60
	worldWidth      = 115.5
	worldHeight     = 74.8
	fieldWidth      = 105.0
	fieldHeight     = 68.0
	fieldLeft       = 5.25
	fieldTop        = 3.4
	physicsCellSize = 8.0
	playerSpeed     = 18.0
)

var (
	worldOrigin    = math2d.Vec2{X: 0, Y: 0}
	worldSize      = math2d.Vec2{X: worldWidth, Y: worldHeight}
	clearColor     = render.RGBA(0.05, 0.08, 0.10, 1)
	viewportOrigin = math2d.Vec2{X: 0, Y: 0}
	viewportSize   = math2d.Vec2{X: imageWidth, Y: imageHeight}
	fieldOrigin    = math2d.Vec2{X: fieldLeft, Y: fieldTop}
	fieldSize      = math2d.Vec2{X: fieldWidth, Y: fieldHeight}

	colliderColor = render.RGBA(1, 1, 0, 1)
	contactColor  = render.RGBA(1, 0, 1, 1)
)

var debugConfig = football.DebugConfig{
	ShowColliders: true,
	ShowContacts:  true,

	ColliderColor:       colliderColor,
	ColliderStrokeWidth: 0.08,
	ColliderLayer:       100,

	ContactColor:        contactColor,
	ContactPointRadius:  0.18,
	ContactLineWidth:    0.12,
	ContactNormalLength: 1.25,
	ContactLayer:        101,

	ShowGrid:        true,
	GridColor:       render.RGBA(0.5, 0.5, 0.5, 0.4), // cinza semi-transparente
	GridDashLength:  1.5,
	GridStrokeWidth: 0.2,
	GridLayer:       200,

	ShowActiveGrid:        true,
	ActiveGridFill:        render.RGBA(1, 0.55, 0.05, 0.22),
	ActiveGridStroke:      render.RGBA(1, 0.75, 0.05, 0.9),
	ActiveGridStrokeWidth: 0.12,
	ActiveGridLayer:       201,

	ShowCandidateGrid:        true,
	CandidateGridFill:        render.RGBA(1, 0.95, 0.05, 0.35),
	CandidateGridStroke:      render.RGBA(1, 1, 0.15, 1),
	CandidateGridStrokeWidth: 0.18,
	CandidateGridLayer:       202,
}

func main() {
	demo := football.BuildPlayableDemoWorld()
	world := demo.World
	controlledPlayer := demo.ControlledPlayer
	recorder := debug.NewRecorder(600)

	server := canvas.Server{
		Addr:         ":8080",
		FPS:          renderFPS,
		SimulationHz: simulationHz,
		Camera: render.Camera{
			World:    math2d.NewRect(worldOrigin, worldSize),
			Viewport: math2d.NewRect(viewportOrigin, viewportSize),
			Aspect:   render.AspectExpand,
		},
		Clear:         clearColor,
		InputBindings: movementBindings(),
		Input: func(state input.State) {
			applyPlayerInput(world, controlledPlayer, state)
		},
		Update: func(dt float64) {
			recorder.Capture(world)
			world.Step(dt, scene.StepConfig{
				Bounds: math2d.NewRect(fieldOrigin, fieldSize),
				Physics: scene.PhysicsConfig{
					CellSize: physicsCellSize,
				},
			})
		},
		Commands: func(debug bool) []render.Command {
			commands := football.BuildFieldCommands()
			commands = append(commands, world.RenderCommands()...)

			if debug {
				football.AppendDebugCommands(&commands, world, debugConfig)
			}

			return commands
		},
		HistoryLen: func() int {
			return recorder.Len()
		},
		Reset: func() {
			demo = football.BuildPlayableDemoWorld()
			world = demo.World
			controlledPlayer = demo.ControlledPlayer
			recorder.Clear()
		},
		Back: func() bool {
			snapshot, ok := recorder.Back()
			if !ok {
				return false
			}

			world = snapshot

			return true
		},
	}

	fmt.Println("Starting canvas server on: localhost:8080")

	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}

func movementBindings() []input.Binding {
	return []input.Binding{
		{Source: keyboard(input.KeyA), Action: input.ActionMoveX, Scale: -1},
		{Source: keyboard(input.KeyD), Action: input.ActionMoveX, Scale: 1},
		{Source: keyboard(input.KeyArrowLeft), Action: input.ActionMoveX, Scale: -1},
		{Source: keyboard(input.KeyArrowRight), Action: input.ActionMoveX, Scale: 1},
		{Source: keyboard(input.KeyW), Action: input.ActionMoveY, Scale: -1},
		{Source: keyboard(input.KeyS), Action: input.ActionMoveY, Scale: 1},
		{Source: keyboard(input.KeyArrowUp), Action: input.ActionMoveY, Scale: -1},
		{Source: keyboard(input.KeyArrowDown), Action: input.ActionMoveY, Scale: 1},
	}
}

func keyboard(code input.SourceCode) input.Source {
	return input.Source{
		Device: input.DeviceKeyboard,
		Code:   code,
	}
}

func applyPlayerInput(world *scene.World, player scene.EntityID, state input.State) {
	direction := math2d.Vec2{
		X: state.Value(input.ActionMoveX),
		Y: state.Value(input.ActionMoveY),
	}
	if direction.LenSq() > 1 {
		direction = direction.Normalize()
	}

	world.SetVelocity(player, scene.Velocity{
		Linear: direction.Mul(playerSpeed),
	})
}
