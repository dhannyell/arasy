package football

import (
	"math"
	"math/rand"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
)

const (
	playerSpeed = 18.0
	goalWidth   = 7.32
	goalDepth   = 1.0
)

type Football struct {
	controlledPlayer scene.EntityID
	ball             scene.EntityID

	leftGoal  math2d.Rect
	rightGoal math2d.Rect

	shake       CameraShake
	shakeOffset math2d.Vec2
}

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

func (c *CameraShake) Step(dt float64) math2d.Vec2 {
	if c.Time <= 0 || c.Duration <= 0 {
		return math2d.Vec2{}
	}

	t := c.Time / c.Duration
	offset := math2d.Vec2{
		X: math.Sin(float64(c.Seed)*12.9898+c.Time*90) * c.Amplitude * t,
		Y: math.Cos(float64(c.Seed)*78.233+c.Time*110) * c.Amplitude * t,
	}

	c.Time -= dt
	if c.Time < 0 {
		c.Time = 0
	}

	return offset
}

func (f *Football) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 115.5, Y: 74.8}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080}),
			Aspect:   render.AspectExpand,
		},
		Clear:        render.RGBA(0.05, 0.08, 0.10, 1),
		Bounds:       math2d.NewRect(fieldOrigin, fieldSize),
		Physics:      scene.PhysicsConfig{CellSize: physicsCellSize},
		SimulationHz: 60, FPS: 60,
		InputBindings: movementBindings(),
		HistoryLen:    600,
	}
}

func (f *Football) Init(ctx *runtime.Context) {
	demo := BuildPlayableDemoWorldIn(ctx.World)
	f.controlledPlayer = demo.ControlledPlayer
	f.ball = demo.Ball
	f.shake = CameraShake{}
	f.shakeOffset = math2d.Vec2{}

	goalTop := fieldTop + fieldHeight/2 - goalWidth/2
	f.leftGoal = math2d.NewRect(
		math2d.Vec2{X: fieldLeft, Y: goalTop},
		math2d.Vec2{X: goalDepth, Y: goalWidth},
	)
	f.rightGoal = math2d.NewRect(
		math2d.Vec2{X: fieldLeft + fieldWidth - goalDepth, Y: goalTop},
		math2d.Vec2{X: goalDepth, Y: goalWidth},
	)
}

func (f *Football) Update(ctx *runtime.Context, dt float64) {
	f.shakeOffset = f.shake.Step(dt)
	previousBallTransform, _ := ctx.World.Transform(f.ball)

	applyPlayerInput(ctx.World, f.controlledPlayer, ctx.Input)
	stats := ctx.World.Step(dt, ctx.StepConfig)
	for _, c := range ctx.World.Contacts() {
		ctx.Log.Debug("contact", "a", c.A, "b", c.B, "depth", c.Depth)
		if c.A == f.ball && c.B == f.controlledPlayer {
			playerSprite, ok := ctx.World.CircleSprite(f.controlledPlayer)
			if ok {
				playerSprite.Color = render.RGBA(rand.Float64(), rand.Float64(), rand.Float64(), 1)
				ctx.World.SetCircleSprite(f.controlledPlayer, playerSprite)
			}
		}
	}

	ballTransform, ok := ctx.World.Transform(f.ball)
	if !ok {
		return
	}
	ballCollider, ok := ctx.World.CircleCollider(f.ball)
	if !ok {
		return
	}

	goalTop := fieldTop + fieldHeight/2 - goalWidth/2
	goalBottom := goalTop + goalWidth
	leftGoalScored := ballIntersectsGoal(ballTransform.Position, ballCollider.Radius, f.leftGoal) ||
		ballCrossedGoalLine(previousBallTransform.Position, ballTransform.Position, ballCollider.Radius, fieldLeft, goalTop, goalBottom, true)
	rightGoalScored := ballIntersectsGoal(ballTransform.Position, ballCollider.Radius, f.rightGoal) ||
		ballCrossedGoalLine(previousBallTransform.Position, ballTransform.Position, ballCollider.Radius, fieldLeft+fieldWidth, goalTop, goalBottom, false)

	switch {
	case leftGoalScored:
		ctx.Log.Debug("goal", "side", "left")
		f.shake.Start(0.3, 0.45)
		resetBall(ctx.World, f.ball)
	case rightGoalScored:
		ctx.Log.Debug("goal", "side", "right")
		f.shake.Start(0.3, 0.45)
		resetBall(ctx.World, f.ball)
	}

	_ = stats
}

func (f *Football) Render(ctx *runtime.Context) []render.Command {
	if f.shakeOffset != (math2d.Vec2{}) {
		ctx.Camera.World = math2d.NewRect(worldOrigin.Add(f.shakeOffset), worldSize)
	}

	cmds := BuildFieldCommands()
	cmds = append(cmds, ctx.World.RenderCommands()...)
	AppendDebugCommands(&cmds, ctx.World, ctx.Debug, DefaultDebugStyle())
	return cmds
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

func keyboard(code input.SourceCode) input.Source {
	return input.Source{
		Device: input.DeviceKeyboard,
		Code:   code,
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

func resetBall(world *scene.World, ball scene.EntityID) {
	ballTransform, ok := world.Transform(ball)
	if ok {
		ballTransform.Position = fieldCenter
		world.SetTransform(ball, ballTransform)
	}

	ballVelocity, ok := world.Velocity(ball)
	if ok {
		ballVelocity.Linear = math2d.Vec2{}
		world.SetVelocity(ball, ballVelocity)
	}
}

func ballIntersectsGoal(center math2d.Vec2, radius float64, rect math2d.Rect) bool {
	closest := math2d.Vec2{
		X: clamp(center.X, rect.Min.X, rect.Max.X),
		Y: clamp(center.Y, rect.Min.Y, rect.Max.Y),
	}

	return center.DistanceSq(closest) <= radius*radius
}

func ballCrossedGoalLine(previous, current math2d.Vec2, radius float64, lineX float64, goalTop float64, goalBottom float64, leftSide bool) bool {
	crossX := lineX
	if leftSide {
		crossX += radius
	} else {
		crossX -= radius
	}

	deltaX := current.X - previous.X
	if deltaX == 0 {
		return false
	}

	t := (crossX - previous.X) / deltaX
	if t < 0 || t > 1 {
		return false
	}

	y := previous.Y + (current.Y-previous.Y)*t

	return y >= goalTop-radius && y <= goalBottom+radius
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
