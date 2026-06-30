package football

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

func TestFootballControlledPlayerRewindsWithWorld(t *testing.T) {
	game := &Football{}
	driver := runtime.NewDriver(game, slog.Default())

	initial := controlledPlayerPosition(t, driver, game)

	driver.ApplyInput([]input.Event{
		{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Kind:   input.EventButtonDown,
		},
	})
	driver.Advance(1.0 / 60.0)
	afterFirstAdvance := controlledPlayerPosition(t, driver, game)
	driver.Advance(1.0 / 60.0)
	afterSecondAdvance := controlledPlayerPosition(t, driver, game)

	if afterFirstAdvance == initial {
		t.Fatal("controlled player did not move after input")
	}
	if afterSecondAdvance == afterFirstAdvance {
		t.Fatal("controlled player did not move on second advance")
	}

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlBack})
	driver.Advance(1.0 / 60.0)

	afterBack := controlledPlayerPosition(t, driver, game)
	if afterBack != afterFirstAdvance {
		t.Fatalf("controlled player after back = %+v, want %+v", afterBack, afterFirstAdvance)
	}
}

func TestHUDLayoutKeepsWidgetsInsideViewport(t *testing.T) {
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720})

	score, reset := hudLayout(viewport)

	if score.Width() != 120 || score.Height() != 28 {
		t.Fatalf("score size = %vx%v, want 120x28", score.Width(), score.Height())
	}
	if reset.Width() != 92 || reset.Height() != 28 {
		t.Fatalf("reset size = %vx%v, want 92x28", reset.Width(), reset.Height())
	}
	if !viewport.Contains(score.Min) || !viewport.Contains(math2d.Vec2{X: score.Max.X - 1, Y: score.Max.Y - 1}) {
		t.Fatalf("score rect %+v should fit inside viewport %+v", score, viewport)
	}
	if !viewport.Contains(reset.Min) || !viewport.Contains(math2d.Vec2{X: reset.Max.X - 1, Y: reset.Max.Y - 1}) {
		t.Fatalf("reset rect %+v should fit inside viewport %+v", reset, viewport)
	}
}

func controlledPlayerPosition(t *testing.T, driver *runtime.Driver, game *Football) math2d.Vec2 {
	t.Helper()

	target := math2d.Vec2{X: fieldLeft + 18, Y: fieldCenter.Y}
	commands := driver.Frame().Commands
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
