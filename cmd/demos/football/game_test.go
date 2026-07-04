package football

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
)

func TestFootballControlledPlayerRewindsWithWorld(t *testing.T) {
	game := &Football{}
	driver := runtime.NewDriver(game, slog.Default())

	initial := controlledPlayerPosition(t, driver, game)

	clickPlay(t, driver, game)

	driver.ApplyInput([]input.Event{
		{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Kind:   input.EventButtonDown,
		},
	})
	driver.Step()
	afterFirstAdvance := controlledPlayerPosition(t, driver, game)
	driver.Step()
	afterSecondAdvance := controlledPlayerPosition(t, driver, game)

	if afterFirstAdvance == initial {
		t.Fatal("controlled player did not move after input")
	}
	if afterSecondAdvance == afterFirstAdvance {
		t.Fatal("controlled player did not move on second advance")
	}

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlBack})
	driver.AdvanceTime(0)

	afterBack := controlledPlayerPosition(t, driver, game)
	if afterBack != afterFirstAdvance {
		t.Fatalf("controlled player after back = %+v, want %+v", afterBack, afterFirstAdvance)
	}
}

func TestHUDLayoutKeepsWidgetsInsideViewport(t *testing.T) {
	ref := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: viewportWidth, Y: viewportHeight})

	score, reset := hudLayout()

	if score.Width() != 120 || score.Height() != 28 {
		t.Fatalf("score size = %vx%v, want 120x28", score.Width(), score.Height())
	}
	if reset.Width() != 92 || reset.Height() != 28 {
		t.Fatalf("reset size = %vx%v, want 92x28", reset.Width(), reset.Height())
	}
	if !ref.Contains(score.Min) || !ref.Contains(math2d.Vec2{X: score.Max.X - 1, Y: score.Max.Y - 1}) {
		t.Fatalf("score rect %+v should fit inside reference %+v", score, ref)
	}
	if !ref.Contains(reset.Min) || !ref.Contains(math2d.Vec2{X: reset.Max.X - 1, Y: reset.Max.Y - 1}) {
		t.Fatalf("reset rect %+v should fit inside reference %+v", reset, ref)
	}
}

func TestGoalBannerRectIsCenteredOnField(t *testing.T) {
	rect := goalBannerRect(12)
	wantCenterX := fieldCenter.X

	if rect.Width() != goalBannerWidth || rect.Height() != goalBannerHeight {
		t.Fatalf("banner size = %vx%v, want %vx%v", rect.Width(), rect.Height(), goalBannerWidth, goalBannerHeight)
	}
	if rect.Center().X != wantCenterX {
		t.Fatalf("banner center X = %v, want %v", rect.Center().X, wantCenterX)
	}
	if rect.Min.X < fieldLeft || rect.Max.X > fieldLeft+fieldWidth {
		t.Fatalf("banner rect %+v should fit inside field X range [%v,%v]", rect, fieldLeft, fieldLeft+fieldWidth)
	}
}

func TestResetBallInstallsTweenFromCurrentPosition(t *testing.T) {
	demo := BuildPlayableDemoWorld()
	start := math2d.Vec2{X: fieldLeft, Y: fieldCenter.Y}
	demo.World.SetTransform(demo.Ball, scene.Transform{
		Position: start,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	demo.World.SetVelocity(demo.Ball, scene.Velocity{Linear: math2d.Vec2{X: 10}})

	resetBall(demo.World, demo.Ball)

	transform, _ := demo.World.Transform(demo.Ball)
	if transform.Position != start {
		t.Fatalf("resetBall teleported ball to %v, want it to stay at %v until the tween advances", transform.Position, start)
	}
	velocity, _ := demo.World.Velocity(demo.Ball)
	if velocity.Linear != (math2d.Vec2{}) {
		t.Fatalf("resetBall velocity = %v, want zero", velocity.Linear)
	}
	tween, ok := demo.World.TransformTween(demo.Ball)
	if !ok {
		t.Fatal("resetBall did not install a transform tween")
	}
	if tween.From.Position != start {
		t.Fatalf("tween starts at %v, want %v", tween.From.Position, start)
	}
	if tween.To.Position != fieldCenter {
		t.Fatalf("tween ends at %v, want %v", tween.To.Position, fieldCenter)
	}
	if _, ok := demo.World.CircleSpriteTween(demo.Ball); ok {
		t.Fatal("resetBall should not install the goal blink tween")
	}
}

func TestBallBlinkTweensAndRestoresSprite(t *testing.T) {
	demo := BuildPlayableDemoWorld()

	startBallBlink(demo.World, demo.Ball)

	tween, ok := demo.World.CircleSpriteTween(demo.Ball)
	if !ok {
		t.Fatal("startBallBlink did not install a circle sprite tween")
	}
	if tween.From.Color != ballColor {
		t.Fatalf("blink starts from %v, want %v", tween.From.Color, ballColor)
	}
	if tween.To.Color == ballColor {
		t.Fatal("blink target color should differ from the ball color")
	}
	if tween.To.Radius <= ballVisualRadius {
		t.Fatalf("blink target radius = %v, want > %v", tween.To.Radius, ballVisualRadius)
	}

	demo.World.Step(goalBlinkPulseTime/2, scene.StepConfig{})
	stopBallBlink(demo.World, demo.Ball)

	if _, ok := demo.World.CircleSpriteTween(demo.Ball); ok {
		t.Fatal("stopBallBlink did not remove the circle sprite tween")
	}
	sprite, _ := demo.World.CircleSprite(demo.Ball)
	if sprite.Color != ballColor {
		t.Fatalf("ball color after stop = %v, want %v", sprite.Color, ballColor)
	}
	if sprite.Radius != ballVisualRadius {
		t.Fatalf("ball radius after stop = %v, want %v", sprite.Radius, ballVisualRadius)
	}
}

func TestGoalBlinkDoneUsesDurationSeconds(t *testing.T) {
	lastGoalTick := uint64(10)
	earlyTick := lastGoalTick + 36
	doneTick := lastGoalTick + uint64(goalBlinkDuration*goalBlinkSimHz)

	if goalBlinkDone(earlyTick, lastGoalTick) {
		t.Fatalf("blink ended at tick %d, want it to last %.1fs", earlyTick, goalBlinkDuration)
	}
	if !goalBlinkDone(doneTick, lastGoalTick) {
		t.Fatalf("blink still active at tick %d, want done after %.1fs", doneTick, goalBlinkDuration)
	}
}

func controlledPlayerPosition(t *testing.T, driver *runtime.Driver, game *Football) math2d.Vec2 {
	t.Helper()

	target := math2d.Vec2{X: fieldLeft + 18, Y: fieldCenter.Y}
	commands := driver.Frame(0).Commands
	var (
		best     math2d.Vec2
		bestDist float64
		found    bool
	)

	for _, command := range commands {
		circle, ok := command.(render.DrawCircle)
		if !ok {
			continue
		}
		if circle.Radius != 1.2 || circle.Fill != homeTeamColor {
			continue
		}
		dist := circle.Center.DistanceSq(target)
		if !found || dist < bestDist {
			best = circle.Center
			bestDist = dist
			found = true
		}
	}

	if !found {
		t.Fatal("controlled player render command missing")
	}
	return best
}

func clickPlay(t *testing.T, driver *runtime.Driver, game *Football) {
	t.Helper()

	_, play := menuLayout()
	pos := play.Center()
	driver.ApplyInput([]input.Event{
		{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
			Kind:   input.EventButtonDown,
			X:      pos.X,
			Y:      pos.Y,
		},
		{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
			Kind:   input.EventButtonUp,
			X:      pos.X,
			Y:      pos.Y,
		},
	})
	driver.Step()

	if !game.State.Started {
		t.Fatal("play click did not start the game")
	}
}
