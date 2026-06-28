package main

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"

	"github.com/dhannyell/frameforge/cmd/demos/football"
	"github.com/dhannyell/frameforge/debug"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/web/canvas"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

type CameraShake struct {
	Time      float64
	Duration  float64
	Amplitude float64
	Seed      uint64
}

func (c *CameraShake) Start(duration, amplitude float64) {
	if c.Time > 0 {
		return
	}

	c.Time = duration
	c.Duration = duration
	c.Amplitude = amplitude
}

func (s *CameraShake) Step(dt float64) math2d.Vec2 {
	if s.Time <= 0 {
		return math2d.Vec2{}
	}

	t := s.Time / s.Duration
	offset := math2d.Vec2{
		X: math.Sin(float64(s.Seed)*12.9898+s.Time*90) * s.Amplitude * t,
		Y: math.Cos(float64(s.Seed)*78.233+s.Time*110) * s.Amplitude * t,
	}

	s.Time -= dt
	if s.Time < 0 {
		s.Time = 0
	}

	return offset
}

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
	goalWidth       = 7.32
	goalDepth       = 1.0
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
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	demo := football.BuildPlayableDemoWorld()
	world := demo.World
	controlledPlayer := demo.ControlledPlayer
	ball := demo.Ball
	recorder := debug.NewRecorder(600)
	playerSprite, _ := world.CircleSprite(controlledPlayer)
	baseCamera := render.Camera{
		World:    math2d.NewRect(worldOrigin, worldSize),
		Viewport: math2d.NewRect(viewportOrigin, viewportSize),
		Aspect:   render.AspectExpand,
	}
	shake := CameraShake{}
	goalTop := fieldTop + fieldHeight/2 - goalWidth/2
	goalBottom := goalTop + goalWidth
	leftGoal := math2d.NewRect(
		math2d.Vec2{X: fieldLeft, Y: goalTop},
		math2d.Vec2{X: goalDepth, Y: goalWidth},
	)
	rightGoal := math2d.NewRect(
		math2d.Vec2{X: fieldLeft + fieldWidth - goalDepth, Y: goalTop},
		math2d.Vec2{X: goalDepth, Y: goalWidth},
	)

	server := canvas.Server{
		Addr:         ":8080",
		FPS:          renderFPS,
		SimulationHz: simulationHz,
		Camera:       baseCamera,
		CameraForFrame: func() render.Camera {
			camera := baseCamera
			offset := shake.Step(1.0 / float64(simulationHz))
			camera.World = math2d.NewRect(
				worldOrigin.Add(offset),
				worldSize,
			)

			return camera
		},
		Clear:         clearColor,
		InputBindings: movementBindings(),
		Input: func(state input.State) {
			applyPlayerInput(world, controlledPlayer, state)
		},
		Update: func(dt float64) {
			previousBallTransform, _ := world.Transform(ball)
			recorder.Capture(world)
			world.Step(dt, scene.StepConfig{
				Bounds: math2d.NewRect(fieldOrigin, fieldSize),
				Physics: scene.PhysicsConfig{
					CellSize: physicsCellSize,
				},
			})

			for _, contact := range world.Contacts() {

				slog.Debug("contact",
					"a", contact.A,
					"b", contact.B,
					"player", controlledPlayer,
					"ball", ball,
				)
				if contact.A == ball && contact.B == controlledPlayer {
					slog.Debug("Player Touched Ball")

					playerSprite.Color = render.RGBA(rand.Float64(), rand.Float64(), rand.Float64(), 1)
					world.SetCircleSprite(controlledPlayer, playerSprite)
				}
			}

			ballTransform, _ := world.Transform(ball)
			ballCollider, _ := world.CircleCollider(ball)

			leftGoalScored := ballIntersectsGoal(ballTransform.Position, ballCollider.Radius, leftGoal) ||
				ballCrossedGoalLine(previousBallTransform.Position, ballTransform.Position, ballCollider.Radius, fieldLeft, goalTop, goalBottom, true)
			rightGoalScored := ballIntersectsGoal(ballTransform.Position, ballCollider.Radius, rightGoal) ||
				ballCrossedGoalLine(previousBallTransform.Position, ballTransform.Position, ballCollider.Radius, fieldLeft+fieldWidth, goalTop, goalBottom, false)

			switch {
			case leftGoalScored:
				slog.Debug("Goal", "side", "left")
				shake.Start(0.3, 0.45)
				resetBall(world, ball)
			case rightGoalScored:
				slog.Debug("Goal", "side", "right")
				shake.Start(0.3, 0.45)
				resetBall(world, ball)
			}
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
			ball = demo.Ball
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

func resetBall(world *scene.World, ball scene.EntityID) {
	ballTransform, ok := world.Transform(ball)
	if ok {
		ballTransform.Position = math2d.Vec2{
			X: fieldLeft + fieldWidth/2,
			Y: fieldTop + fieldHeight/2,
		}
		world.SetTransform(ball, ballTransform)
	}

	ballVelocity, ok := world.Velocity(ball)
	if ok {
		ballVelocity.Linear = math2d.Vec2{}
		world.SetVelocity(ball, ballVelocity)
	}
}
