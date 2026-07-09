package breakout

import (
	"fmt"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/cmd/demos/breakout/audioassets"
	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

func (b *Breakout) Setup() runtime.Config {
	assets, err := asset.Load(audioassets.FS())
	if err != nil {
		panic(err)
	}
	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldWidth, Y: worldHeight}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 960}),
			Aspect:   render.AspectExpand,
		},
		Clear:               render.RGBA(0.03, 0.035, 0.045, 1),
		Bounds:              math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldWidth, Y: worldHeight}),
		Bounce:              scene.BounceConfig{OpenSides: scene.SideBottom},
		SimulationHz:        60,
		FPS:                 60,
		RenderInterpolation: true,
		InputBindings:       breakoutBindings(),
		Assets:              assets,
		HistoryLen:          1000,
		DebugOverlay: scene.DebugOverlayConfig{
			Bounds:   math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldWidth, Y: worldHeight}),
			CellSize: physicsCellSize,
		},
	}
}

func (b *Breakout) Init(ctx *runtime.UpdateContext) {
	b.State = MatchState{}
	b.State.bricks = spawnBuildBricks(ctx.World)
	b.State.bricksLeft = len(b.State.bricks)

	b.State.paddle.Rect = math2d.NewRect(math2d.Vec2{X: (worldWidth - paddleWidth) / 2, Y: paddleY}, math2d.Vec2{X: paddleWidth, Y: paddleHeight})
	b.State.paddle.Speed = paddleSpeed
	b.State.ball.Velocity = math2d.Vec2{}
	b.State.ball.Radius = ballRadius

	b.State.ball.id = ctx.World.SpawnBundle(scene.Bundle{
		Transform: &scene.Transform{Position: math2d.Vec2{X: 0, Y: 0}, Scale: math2d.Vec2{X: 1, Y: 1}},
		Velocity:  &scene.Velocity{Linear: b.State.ball.Velocity},
		CircleSprite: &scene.CircleSprite{
			Radius: ballRadius,
			BaseSprite: scene.BaseSprite{
				Color: render.RGBA(0.5, 1, 1, 1),
				Layer: 1,
			},
		},
		CircleCollider: &scene.CircleCollider{Radius: ballRadius},
		Body:           &scene.Body{InvMass: 1.0, Restitution: 1.0},
	})

	b.State.paddle.id = ctx.World.SpawnBundle(scene.Bundle{
		Transform: &scene.Transform{Position: b.State.paddle.Rect.Center()},
		RectSprite: &scene.RectSprite{
			BaseSprite: scene.BaseSprite{
				Color: render.RGBA(0.5, 1, 1, 1),
				Layer: 1,
			},
			Rect: b.State.paddle.Rect,
		},
		RectCollider: &scene.RectCollider{
			HalfExtends: math2d.Vec2{X: paddleWidth / 2, Y: paddleHeight / 2},
		},
		Body: &scene.Body{InvMass: 0.0, Restitution: 1.0},
	})

	b.State.lives = 3
	playTrack(ctx, "music", audio.Track{Clip: "music", Loop: true, Gain: 0.4, FadeSec: 0.5})
}

func (b *Breakout) Update(ctx *runtime.UpdateContext, dt float64) {
	ctx.Log.Debug("game", "bricks", b.State.bricksLeft)

	_, reset := hudLayout()
	if ctx.UI.ClickedIn("btn_reset", reset, ui.SpacePlayfield) {
		b.State.state = StateMenu
	}

	if b.State.state == StateMenu || b.State.state == StateGameOver || b.State.state == StateWon {
		ctx.HoldSimulation()
		_, _, play := menuLayout()
		if ctx.UI.Clicked("btn_play", play) {
			b.State.state = StateReady
			b.State.lives = 3
			b.respawnBricks(ctx.World)
			playTrack(ctx, "music", audio.Track{Clip: "music", Loop: true, Gain: 0.4, FadeSec: 0.5})
		}
		return
	}

	if b.State.bricksLeft <= 0 {
		b.State.state = StateWon
		stopTrack(ctx, "music")
		ctx.HoldSimulation()
		return
	}

	b.movePaddle(ctx.Input, dt)
	b.syncPaddleToWorld(ctx.World)

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

	if b.State.state != StatePlaying {
		ctx.HoldSimulation()
	}

}

func (b *Breakout) PostStep(ctx *runtime.UpdateContext, _ scene.StepStats) {
	b.syncBallFromWorld(ctx.World)
	b.resolveGameContacts(ctx)

	if b.State.ball.Position.Y-b.State.ball.Radius > worldHeight {
		b.State.lives--
		playCue(ctx, "ball_lost")
		b.attachBallToPaddle()
		ctx.World.SetTransform(b.State.ball.id, scene.Transform{Position: b.State.ball.Position})
		ctx.World.SetVelocity(b.State.ball.id, scene.Velocity{Linear: b.State.ball.Velocity})
		if b.State.lives <= 0 {
			b.State.state = StateGameOver
			stopTrack(ctx, "music")
			return
		}
		b.State.state = StateReady
	}
}

func (b *Breakout) Render(ctx *runtime.RenderContext) []render.Command {
	cmds := make([]render.Command, 0)

	cmds = append(cmds, render.DrawRect{
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
	b.renderScreenOverlay(ctx)

	cmds = append(cmds, ctx.World.RenderCommandsInterpolated(ctx.Alpha)...)

	return cmds
}

func (b *Breakout) renderScreenOverlay(ctx *runtime.RenderContext) {
	tr := ctx.UI.ScreenTransition("breakout_screen", stateScreenKey(b.State.state), ui.TransitionOptions{
		Kind:     ui.TransitionSlideLeft,
		Duration: 0.32,
		Ease:     ease.OutCubic,
	})

	if outgoing, ok := tr.Outgoing(); ok {
		ctx.UI.BeginGroup(tr.OutgoingGroup())
		drawScreenOverlay(ctx.UI, screenKeyState(outgoing))
		ctx.UI.EndGroup()
	}

	ctx.UI.BeginGroup(tr.IncomingGroup())
	drawScreenOverlay(ctx.UI, b.State.state)
	ctx.UI.EndGroup()
}

func drawScreenOverlay(drawer ui.Drawer, state State) {
	panel, label, play := menuLayout()
	switch state {
	case StateWon:
		drawer.Panel(panel)
		drawer.Label("You Win!", label)
		drawer.Button("btn_play", "Play Again", play)
	case StateGameOver:
		drawer.Panel(panel)
		drawer.Label("Game Over!", label)
		drawer.Button("btn_play", "Play Again", play)
	case StateMenu:
		drawer.Panel(panel)
		drawer.Label("Breakout", label)
		drawer.Button("btn_play", "Play", play)
	}
}

func stateScreenKey(state State) uint64 {
	return uint64(state) + 1
}

func screenKeyState(key uint64) State {
	if key == 0 {
		return StateMenu
	}
	return State(key - 1)
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

// syncPaddleToWorld pushes the paddle's game-owned Rect into its entity so the
// collider and sprite follow the player's input (the entity is the physics
// truth the ball collides with).
func (b *Breakout) syncPaddleToWorld(world *scene.World) {
	rect := b.State.paddle.Rect
	world.SetTransform(b.State.paddle.id, scene.Transform{Position: rect.Center()})
	if sprite, ok := world.RectSprite(b.State.paddle.id); ok {
		sprite.Rect = rect
		world.SetRectSprite(b.State.paddle.id, sprite)
	}
}

// resolveGameContacts applies breakout's game rules to the physics contacts of
// the last Step: paddle bounce angle by impact point, and brick destruction.
func (b *Breakout) resolveGameContacts(ctx *runtime.UpdateContext) {
	world := ctx.World
	for _, contact := range world.Contacts() {
		other, involvesBall := contactOther(contact, b.State.ball.id)
		if !involvesBall {
			continue
		}

		if other == b.State.paddle.id {
			b.bounceFromPaddle()
			world.SetVelocity(b.State.ball.id, scene.Velocity{Linear: b.State.ball.Velocity})
			playCue(ctx, "paddle_hit")
			continue
		}

		if idx := b.brickIndex(other); idx >= 0 && b.State.bricks[idx].Alive {
			b.State.bricks[idx].HP--
			if b.State.bricks[idx].HP <= 0 {
				b.State.bricks[idx].Alive = false
				world.Despawn(other)
				b.State.score += 10
				b.State.bricksLeft--
				b.speedUpBallAfterBrick()
				world.SetVelocity(b.State.ball.id, scene.Velocity{Linear: b.State.ball.Velocity})
				playCue(ctx, "brick_break")
			}
		}
	}
}

func playCue(ctx *runtime.UpdateContext, clip audio.ClipID) {
	if ctx.Audio != nil {
		ctx.Audio.Play(clip)
	}
}

func playTrack(ctx *runtime.UpdateContext, id audio.TrackID, track audio.Track) {
	if ctx.Audio != nil {
		ctx.Audio.SetTrack(id, track)
	}
}

func stopTrack(ctx *runtime.UpdateContext, id audio.TrackID) {
	if ctx.Audio != nil {
		ctx.Audio.StopTrack(id)
	}
}

func (b *Breakout) speedUpBallAfterBrick() {
	speed := b.State.ball.Velocity.Len()
	if speed == 0 {
		return
	}
	nextSpeed := math2d.Clamp(speed*ballSpeedUpPerBrick, ballSpeed, ballMaxSpeed)
	b.State.ball.Velocity = b.State.ball.Velocity.Normalize().Mul(nextSpeed)
}

// respawnBricks clears any live brick entities and rebuilds the wall, keeping
// the entity world and the game-side brick list in sync on (re)start.
func (b *Breakout) respawnBricks(world *scene.World) {
	for _, brick := range b.State.bricks {
		world.Despawn(brick.ID)
	}
	b.State.bricks = spawnBuildBricks(world)
	b.State.bricksLeft = len(b.State.bricks)
}

func (b *Breakout) brickIndex(id scene.EntityID) int {
	for i := range b.State.bricks {
		if b.State.bricks[i].ID == id {
			return i
		}
	}
	return -1
}

// contactOther returns the entity paired with id in the contact, and whether id
// took part at all.
func contactOther(contact scene.Contact, id scene.EntityID) (scene.EntityID, bool) {
	switch id {
	case contact.A:
		return contact.B, true
	case contact.B:
		return contact.A, true
	default:
		return 0, false
	}
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
