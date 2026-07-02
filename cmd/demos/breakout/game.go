package breakout

import (
	"fmt"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

type Hit struct {
	Normal math2d.Vec2
	Point  math2d.Vec2
}

func (b *Breakout) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldWidth, Y: worldHeight}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 960}),
			Aspect:   render.AspectExpand,
		},
		Clear:         render.RGBA(0.03, 0.035, 0.045, 1),
		Bounds:        breakoutPhysicsBounds(),
		SimulationHz:  60,
		FPS:           60,
		InputBindings: breakoutBindings(),
		HistoryLen:    1000,
		DebugOverlay: scene.DebugOverlayConfig{
			Bounds:   math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldWidth, Y: worldHeight}),
			CellSize: physicsCellSize,
		},
	}
}

func (b *Breakout) Init(ctx *runtime.UpdateContext) {
	b.State = MatchState{}
	b.State.bricks = buildBricks()
	b.State.bricksLeft = len(b.State.bricks)

	b.State.paddle.Rect = math2d.NewRect(math2d.Vec2{X: (worldWidth - paddleWidth) / 2, Y: paddleY}, math2d.Vec2{X: paddleWidth, Y: paddleHeight})
	b.State.paddle.Speed = paddleSpeed
	b.State.ball.Velocity = math2d.Vec2{}
	b.State.ball.Radius = ballRadius

	b.State.ball.id = ctx.World.SpawnBundle(scene.Bundle{
		Transform:      &scene.Transform{Position: math2d.Vec2{X: 0, Y: 0}, Scale: math2d.Vec2{X: 1, Y: 1}},
		Velocity:       &scene.Velocity{Linear: b.State.ball.Velocity},
		CircleSprite:   &scene.CircleSprite{Radius: ballRadius, Color: render.RGBA(0.5, 1, 1, 1)},
		CircleCollider: &scene.CircleCollider{Radius: ballRadius},
		Body:           &scene.Body{InvMass: 1.0, Restitution: 0.8},
	})

	b.State.lives = 3

}

func (b *Breakout) Update(ctx *runtime.UpdateContext, dt float64) {
	ctx.Log.Debug("game", "bricks", b.State.bricksLeft)

	_, reset := hudLayout()
	if ctx.UI.ClickedIn("btn_reset", reset, ui.SpacePlayfield) {
		b.State.state = StateMenu
	}

	if b.State.state == StateMenu || b.State.state == StateGameOver || b.State.state == StateWon {
		_, _, play := menuLayout()
		if ctx.UI.Clicked("btn_play", play) {
			b.State.state = StateReady
			b.State.lives = 3
			b.State.bricksLeft = len(b.State.bricks)

			for i := range b.State.bricks {
				b.State.bricks[i].Alive = true
			}
		}
		return
	}

	if b.State.bricksLeft <= 0 {
		b.State.state = StateWon
	}

	b.movePaddle(ctx.Input, dt)

	if b.State.state == StateReady {
		b.attachBallToPaddle()
		ctx.World.SetTransform(b.State.ball.id,
			scene.Transform{
				Position: b.State.ball.Position,
			},
		)
		ctx.World.SetVelocity(b.State.ball.id, scene.Velocity{Linear: b.State.ball.Velocity})
	}

	if ctx.Input.JustPressed(input.ActionJump) && b.State.state == StateReady {
		b.State.state = StatePlaying
		b.State.ball.Velocity = math2d.Vec2{X: 0.35, Y: -1}.Normalize().Mul(ballSpeed)
		ctx.World.SetVelocity(b.State.ball.id, scene.Velocity{Linear: b.State.ball.Velocity})
	}

	if b.State.state == StatePlaying {
		ctx.World.Step(dt, ctx.StepConfig)
		b.syncBallFromWorld(ctx.World)

		if b.State.ball.Position.Y-b.State.ball.Radius > worldHeight {
			b.State.lives--
			b.attachBallToPaddle()
			b.writeBallToWorld(ctx.World)
			if b.State.lives <= 0 {
				b.State.state = StateGameOver
				return
			}
			b.State.state = StateReady
			return
		}
		b.resolveBallRectCollisions(ctx.World)
	}

}

func (b *Breakout) Render(ctx *runtime.RenderContext) []render.Command {
	if b.State.state == StateWon {
		panel, label, play := menuLayout()
		ctx.UI.Panel(panel)
		ctx.UI.Label("You Win!", label)
		ctx.UI.Button("btn_play", "Play Again", play)
	}
	if b.State.state == StateGameOver {
		panel, label, play := menuLayout()
		ctx.UI.Panel(panel)
		ctx.UI.Label("Game Over!", label)
		ctx.UI.Button("btn_play", "Play Again", play)
	}
	if b.State.state == StateMenu {
		panel, label, play := menuLayout()
		ctx.UI.Panel(panel)
		ctx.UI.Label("Breakout", label)
		ctx.UI.Button("btn_play", "Play", play)
	}

	cmds := make([]render.Command, 0)

	for _, brick := range b.State.bricks {
		if !brick.Alive {
			continue
		}

		cmds = append(cmds, render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: 10},
			Rect:        brick.Rect,
			Fill:        brick.Color,
		})
	}

	cmds = append(cmds, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: 20},
		Rect:        b.State.paddle.Rect,
		Fill:        render.RGBA(0.85, 0.88, 0.95, 1),
	}, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: 30},
		Rect:        math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldWidth, Y: worldHeight}),
		Stroke:      render.RGBA(1, 1, 1, 1),
		StrokeWidth: 0.5,
	},
	)

	score, reset := hudLayout()

	ctx.UI.PanelIn(score, ui.SpacePlayfield)
	ctx.UI.LabelIn(fmt.Sprintf("Score: %v  Lives: %v", b.State.score, b.State.lives), score.Center(), ui.SpacePlayfield)
	ctx.UI.ButtonIn("btn_reset", "Reset", reset, ui.SpacePlayfield)

	cmds = append(cmds, ctx.World.RenderCommands()...)

	return cmds
}

func hudLayout() (score, reset math2d.Rect) {
	score = math2d.NewRect(
		math2d.Vec2{X: viewportWidth/2 - 60, Y: 8},
		math2d.Vec2{X: 150, Y: 50},
	)
	reset = math2d.NewRect(
		math2d.Vec2{X: viewportWidth - 100, Y: 8},
		math2d.Vec2{X: 92, Y: 28})
	return
}

func menuLayout() (panel math2d.Rect, label math2d.Vec2, play math2d.Rect) {
	panelSize := math2d.Vec2{X: 320, Y: 220}
	panel = math2d.NewRect(
		math2d.Vec2{X: viewportWidth/2 - panelSize.X/2, Y: viewportHeight/2 - panelSize.Y/2},
		panelSize,
	)

	center := panel.Center()
	label = math2d.Vec2{X: center.X, Y: center.Y - 30}

	playSize := math2d.Vec2{X: 120, Y: 36}
	play = math2d.NewRect(
		math2d.Vec2{X: center.X - playSize.X/2, Y: center.Y + 30 - playSize.Y/2},
		playSize,
	)
	return
}

func breakoutPhysicsBounds() math2d.Rect {
	return math2d.NewRect(
		math2d.Vec2{},
		math2d.Vec2{X: worldWidth, Y: worldHeight + ballSpeed},
	)
}

func (b *Breakout) attachBallToPaddle() {
	center := b.State.paddle.Rect.Center()
	b.State.ball.Position = math2d.Vec2{
		X: center.X,
		Y: b.State.paddle.Rect.Min.Y - b.State.ball.Radius - 0.2,
	}
	b.State.ball.Velocity = math2d.Vec2{}
}

func (b *Breakout) bounceFromPaddle() {
	paddleCenter := b.State.paddle.Rect.Center()
	halfWidth := b.State.paddle.Rect.Width() * 0.5
	offset := math2d.Clamp((b.State.ball.Position.X-paddleCenter.X)/halfWidth, -1, 1)

	speed := b.State.ball.Velocity.Len()
	direction := math2d.Vec2{
		X: offset * 0.85,
		Y: -1,
	}.Normalize()

	b.State.ball.Velocity = direction.Mul(speed)
}

func (b *Breakout) syncBallFromWorld(world *scene.World) {
	if transform, ok := world.Transform(b.State.ball.id); ok {
		b.State.ball.Position = transform.Position
	}
	if velocity, ok := world.Velocity(b.State.ball.id); ok {
		b.State.ball.Velocity = velocity.Linear
	}
}

func (b *Breakout) resolveBallRectCollisions(world *scene.World) {
	if hit, ok := circleVsRect(b.State.ball.Position, b.State.ball.Radius, b.State.paddle.Rect); ok {
		b.resolveBallHit(world, hit)
		b.bounceFromPaddle()
		b.writeBallToWorld(world)
		return
	}

	for i := range b.State.bricks {
		if !b.State.bricks[i].Alive {
			continue
		}

		hit, ok := circleVsRect(b.State.ball.Position, b.State.ball.Radius, b.State.bricks[i].Rect)
		if !ok {
			continue
		}

		b.State.bricks[i].Alive = false
		b.State.bricksLeft--
		b.State.ball.Velocity.Y -= 1.5
		b.State.score += 100
		b.resolveBallHit(world, hit)
		b.writeBallToWorld(world)
		return
	}
}

func (b *Breakout) resolveBallHit(world *scene.World, hit Hit) {
	b.State.ball.Position = hit.Point.Add(hit.Normal.Mul(b.State.ball.Radius + 0.01))
	b.State.ball.Velocity = reflectVelocity(b.State.ball.Velocity, hit.Normal)
	b.writeBallToWorld(world)
}

func (b *Breakout) writeBallToWorld(world *scene.World) {
	world.SetTransform(b.State.ball.id, scene.Transform{Position: b.State.ball.Position})
	world.SetVelocity(b.State.ball.id, scene.Velocity{Linear: b.State.ball.Velocity})
}

func reflectVelocity(velocity, normal math2d.Vec2) math2d.Vec2 {
	if velocity.Dot(normal) >= 0 {
		return velocity
	}

	return velocity.Sub(normal.Mul(2 * velocity.Dot(normal)))
}

func circleVsRect(center math2d.Vec2, radius float64, rect math2d.Rect) (Hit, bool) {
	closest := math2d.Vec2{
		X: math2d.Clamp(center.X, rect.Min.X, rect.Max.X),
		Y: math2d.Clamp(center.Y, rect.Min.Y, rect.Max.Y),
	}

	delta := center.Sub(closest)
	distSq := delta.LenSq()
	if distSq > radius*radius {
		return Hit{}, false
	}

	if distSq == 0 {
		return Hit{Normal: rectExitNormal(center, rect), Point: closest}, true
	}

	return Hit{Normal: delta.Normalize(), Point: closest}, true
}

func rectExitNormal(point math2d.Vec2, rect math2d.Rect) math2d.Vec2 {
	left := point.X - rect.Min.X
	right := rect.Max.X - point.X
	top := point.Y - rect.Min.Y
	bottom := rect.Max.Y - point.Y

	minDist := left
	normal := math2d.Vec2{X: -1}

	if right < minDist {
		minDist = right
		normal = math2d.Vec2{X: 1}
	}
	if top < minDist {
		minDist = top
		normal = math2d.Vec2{Y: -1}
	}
	if bottom < minDist {
		normal = math2d.Vec2{Y: 1}
	}

	return normal
}

func (b *Breakout) movePaddle(i input.State, dt float64) {
	dx := i.Value(input.ActionMoveX) * b.State.paddle.Speed * dt
	b.State.paddle.Rect = translateRect(b.State.paddle.Rect, math2d.Vec2{X: dx})
	b.State.paddle.Rect = math2d.ClampRect(b.State.paddle.Rect, 0, worldWidth)
}

func translateRect(rect math2d.Rect, delta math2d.Vec2) math2d.Rect {
	return math2d.Rect{
		Min: rect.Min.Add(delta),
		Max: rect.Max.Add(delta),
	}
}

func breakoutBindings() []input.Binding {
	return []input.Binding{
		{Source: input.Keyboard(input.KeyA), Action: input.ActionMoveX, Scale: -1},
		{Source: input.Keyboard(input.KeyD), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowLeft), Action: input.ActionMoveX, Scale: -1},
		{Source: input.Keyboard(input.KeyArrowRight), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Keyboard(input.KeySpace), Action: input.ActionJump, Scale: 1},
	}
}
