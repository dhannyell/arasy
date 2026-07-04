package football

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"

	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

const (
	playerSpeed = 18.0
	goalWidth   = 7.32
	goalDepth   = 1.0
)

const (
	goalBannerWidth  = 60.0
	goalBannerHeight = 16.0
)

const (
	shakeDurationTicks = 18
	shakeAmplitude     = 0.45
	shakeSeed          = 0
	shakeSimDt         = 1.0 / 60.0
	goalBlinkDuration  = 1.5
	goalBlinkPulseTime = 0.12
	goalBlinkSimHz     = 60.0
)

type Football struct {
	runtime.StateSlot[MatchState]

	controlledPlayer scene.EntityID
	ball             scene.EntityID
	leftGoal         math2d.Rect
	rightGoal        math2d.Rect
	previousBallPos  math2d.Vec2
}

// shakeOffset is derived from tick state, so rewind restores it automatically.
func shakeOffset(elapsedTicks uint64) (math2d.Vec2, bool) {
	if elapsedTicks >= shakeDurationTicks {
		return math2d.Vec2{}, false
	}

	remaining := float64(shakeDurationTicks-elapsedTicks) / float64(shakeDurationTicks)
	t := float64(elapsedTicks) * shakeSimDt

	return math2d.Vec2{
		X: math.Sin(float64(shakeSeed)*12.9898+t*90) * shakeAmplitude * remaining,
		Y: math.Cos(float64(shakeSeed)*78.233+t*110) * shakeAmplitude * remaining,
	}, true
}

func menuLayout() (panel, play math2d.Rect) {
	panel = math2d.NewRect(
		math2d.Vec2{X: viewportWidth/2 - 160, Y: viewportHeight/2 - 100},
		math2d.Vec2{X: 320, Y: 200},
	)
	play = math2d.NewRect(
		math2d.Vec2{X: viewportWidth/2 - 60, Y: viewportHeight/2 + 10},
		math2d.Vec2{X: 120, Y: 36},
	)
	return
}

func hudLayout() (score, reset math2d.Rect) {
	score = math2d.NewRect(
		math2d.Vec2{X: viewportWidth/2 - 60, Y: 8},
		math2d.Vec2{X: 120, Y: 28},
	)
	reset = math2d.NewRect(
		math2d.Vec2{X: viewportWidth - 100, Y: 8},
		math2d.Vec2{X: 92, Y: 28})
	return
}

func goalBannerRect(y float64) math2d.Rect {
	return math2d.NewRect(
		math2d.Vec2{X: fieldCenter.X - goalBannerWidth/2, Y: y},
		math2d.Vec2{X: goalBannerWidth, Y: goalBannerHeight},
	)
}

func (f *Football) resetMatch(world *scene.World) {
	slog.Debug("Reset Button")
	stopBallBlink(world, f.ball)
	resetBall(world, f.ball)
	f.State = MatchState{}
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
		DebugOverlay: scene.DebugOverlayConfig{
			Bounds:   math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 115.5, Y: 74.8}),
			CellSize: physicsCellSize,
		},
	}
}

func (f *Football) Init(ctx *runtime.UpdateContext) {
	f.State = MatchState{}

	demo := BuildPlayableDemoWorldIn(ctx.World)
	f.controlledPlayer = demo.ControlledPlayer
	f.ball = demo.Ball

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

func (f *Football) Update(ctx *runtime.UpdateContext, dt float64) {
	if !f.State.Started {
		ctx.HoldSimulation()
		_, play := menuLayout()
		if ctx.UI.Clicked("btn_play", play) {
			f.State.Started = true
		}

		return
	}

	_, reset := hudLayout()
	if ctx.UI.Clicked("btn_reset", reset) {
		f.resetMatch(ctx.World)
	}
	if f.State.HasGoal && goalBlinkDone(ctx.Tick, f.State.LastGoalTick) {
		stopBallBlink(ctx.World, f.ball)
		f.State.HasGoal = false
	}

	previousBallTransform, _ := ctx.World.Transform(f.ball)
	f.previousBallPos = previousBallTransform.Position

	applyPlayerInput(ctx.World, f.controlledPlayer, ctx.Input)
}

func (f *Football) PostStep(ctx *runtime.UpdateContext, _ scene.StepStats) {
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
		ballCrossedGoalLine(f.previousBallPos, ballTransform.Position, ballCollider.Radius, fieldLeft, goalTop, goalBottom, true)
	rightGoalScored := ballIntersectsGoal(ballTransform.Position, ballCollider.Radius, f.rightGoal) ||
		ballCrossedGoalLine(f.previousBallPos, ballTransform.Position, ballCollider.Radius, fieldLeft+fieldWidth, goalTop, goalBottom, false)

	switch {
	case leftGoalScored:
		ctx.Log.Debug("goal", "side", "left")
		f.State.ScoreLeft++
		f.State.HasGoal = true
		f.State.LastGoalTick = ctx.Tick
		resetBall(ctx.World, f.ball)
		startBallBlink(ctx.World, f.ball)
	case rightGoalScored:
		ctx.Log.Debug("goal", "side", "right")
		f.State.ScoreRight++
		f.State.HasGoal = true
		f.State.LastGoalTick = ctx.Tick
		resetBall(ctx.World, f.ball)
		startBallBlink(ctx.World, f.ball)
	}
}

func (f *Football) Render(ctx *runtime.RenderContext) []render.Command {
	if !f.State.Started {
		panel, play := menuLayout()
		ctx.UI.Panel(panel)
		ctx.UI.Label("Football", panel.Center())
		ctx.UI.Button("btn_play", "Play", play)
	}

	cam := ctx.Camera()
	if f.State.HasGoal {
		elapsed := ctx.Tick - f.State.LastGoalTick
		if offset, shaking := shakeOffset(elapsed); shaking {
			cam.World = math2d.NewRect(worldOrigin.Add(offset), worldSize)
		}
	}
	ctx.SetCamera(cam)

	cmds := BuildFieldCommands()
	cmds = append(cmds, ctx.World.RenderCommands()...)

	score, reset := hudLayout()

	ctx.UI.Panel(score)
	ctx.UI.Label(fmt.Sprintf("%d — %d", f.State.ScoreLeft, f.State.ScoreRight), score.Center())
	ctx.UI.Button("btn_reset", "Reset", reset)

	goalKey := uint64(0)
	if f.State.HasGoal {
		goalKey = f.State.LastGoalTick
	}
	goalBanner := ctx.UI.Effect("goal_banner", ui.EffectOptions{Key: goalKey, Duration: 2.0})
	if f.State.HasGoal {
		if goalBanner.Done() {
			return cmds
		}

		bannerRect := goalBanner.SlideY(goalBannerRect(12), -60, 12, 0.0, 0.35, ease.OutBack)
		alpha := goalBanner.FadeOut(1.6, 2.0, ease.InQuad)
		pulse := goalBanner.Pulse(0.35, 1.6, 0.05, 8)
		cmds = append(cmds,
			render.DrawRect{
				BaseCommand: render.BaseCommand{LayerValue: 100},
				Rect:        bannerRect,
				Fill:        render.Color{R: 0.1, G: 0.7, B: 0.1, A: alpha},
			},
			render.DrawText{
				BaseCommand: render.BaseCommand{LayerValue: 101},
				Position:    bannerRect.Center(),
				Text:        "GOL!",
				Color:       render.Color{R: 1, G: 1, B: 1, A: alpha},
				Size:        14 * pulse,
				Align:       render.AlignCenter,
				Baseline:    render.BaselineMiddle,
			},
		)
		return cmds
	}

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
	if !ok {
		return
	}

	world.TweenTransform(ball, scene.Transform{
		Position: fieldCenter,
		Rotation: ballTransform.Rotation,
		Scale:    ballTransform.Scale,
	}, scene.TransformTweenOptions{
		Duration:         0.5,
		Ease:             ease.OutCubic,
		Loop:             scene.LoopOnce,
		RemoveOnComplete: true,
		StopVelocity:     true,
	})
}

func startBallBlink(world *scene.World, ball scene.EntityID) {
	sprite, ok := world.CircleSprite(ball)
	if !ok {
		return
	}

	target := sprite
	target.Color = render.RGBA(1, 0.94, 0.2, 1)
	target.Radius = ballVisualRadius * 1.35
	world.TweenCircleSprite(ball, target, scene.CircleSpriteTweenOptions{
		Duration: goalBlinkPulseTime,
		Ease:     ease.Linear,
		Loop:     scene.LoopPingPong,
	})
}

func stopBallBlink(world *scene.World, ball scene.EntityID) {
	sprite, ok := world.CircleSprite(ball)
	if !ok {
		return
	}

	world.RemoveCircleSpriteTween(ball)
	sprite.Color = ballColor
	sprite.Radius = ballVisualRadius
	world.SetCircleSprite(ball, sprite)
}

func goalBlinkDone(tick, lastGoalTick uint64) bool {
	return float64(tick-lastGoalTick) >= goalBlinkDuration*goalBlinkSimHz
}

func ballIntersectsGoal(center math2d.Vec2, radius float64, rect math2d.Rect) bool {
	closest := math2d.Vec2{
		X: math2d.Clamp(center.X, rect.Min.X, rect.Max.X),
		Y: math2d.Clamp(center.Y, rect.Min.Y, rect.Max.Y),
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
