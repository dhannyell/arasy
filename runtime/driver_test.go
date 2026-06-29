package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

type cameraGame struct{}

func (cameraGame) Setup() Config {
	return Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Aspect:   render.AspectExpand,
		},
	}
}

func (cameraGame) Init(ctx *Context) {}

func (cameraGame) Update(ctx *Context, dt float64) {}

func (cameraGame) Render(ctx *Context) []render.Command {
	ctx.Camera.World = math2d.NewRect(math2d.Vec2{X: 2, Y: 3}, math2d.Vec2{X: 10, Y: 10})
	return nil
}

func TestDriverFrameReturnsCameraModifiedByRender(t *testing.T) {
	driver := NewDriver(cameraGame{}, slog.Default())

	frame := driver.Frame()
	want := math2d.NewRect(math2d.Vec2{X: 2, Y: 3}, math2d.Vec2{X: 10, Y: 10})

	if frame.Camera.World != want {
		t.Fatalf("camera world = %+v, want %+v", frame.Camera.World, want)
	}
}

var _ Game = cameraGame{}

type inputBackGame struct {
	player scene.EntityID
}

func (g *inputBackGame) Setup() Config {
	return Config{
		Bounds: math2d.NewRect(
			math2d.Vec2{X: -100, Y: -100},
			math2d.Vec2{X: 200, Y: 200},
		),
		InputBindings: []input.Binding{
			{
				Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
				Action: input.ActionMoveX,
				Scale:  1,
			},
		},
		HistoryLen: 4,
	}
}

func (g *inputBackGame) Init(ctx *Context) {
	g.player = ctx.World.Spawn()
	ctx.World.SetTransform(g.player, scene.Transform{
		Scale: math2d.Vec2{X: 1, Y: 1},
	})
	ctx.World.SetVelocity(g.player, scene.Velocity{})
}

func (g *inputBackGame) Update(ctx *Context, dt float64) {
	ctx.World.SetVelocity(g.player, scene.Velocity{
		Linear: math2d.Vec2{X: ctx.Input.Value(input.ActionMoveX) * 10},
	})
	ctx.World.Step(dt, ctx.StepConfig)
}

func (g *inputBackGame) Render(ctx *Context) []render.Command { return nil }

func TestDriverBackRestoresInputSnapshotBeforeManualStep(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{
		{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Kind:   input.EventButtonDown,
		},
	})
	driver.Advance(1)

	transform, ok := driver.world.Transform(game.player)
	if !ok {
		t.Fatal("expected player transform")
	}
	if transform.Position.X != 10 {
		t.Fatalf("position after input advance = %v, want 10", transform.Position.X)
	}

	driver.Control(ControlCommand{Kind: ControlBack})
	driver.Advance(1)

	transform, ok = driver.world.Transform(game.player)
	if !ok {
		t.Fatal("expected player transform after back")
	}
	if transform.Position.X != 0 {
		t.Fatalf("position after back = %v, want 0", transform.Position.X)
	}

	driver.Control(ControlCommand{Kind: ControlStep})
	driver.Advance(1)

	transform, ok = driver.world.Transform(game.player)
	if !ok {
		t.Fatal("expected player transform after step")
	}
	if transform.Position.X != 10 {
		t.Fatalf("position after step following back = %v, want 10", transform.Position.X)
	}
}

func TestDriverBackToTickRestoresTargetSnapshot(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{
		{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Kind:   input.EventButtonDown,
		},
	})
	for range 5 {
		driver.Advance(1)
	}

	driver.Control(ControlCommand{Kind: ControlBackToTick, Tick: 2})
	driver.Advance(1)

	if driver.Tick() != 2 {
		t.Fatalf("tick after back to tick = %v, want 2", driver.Tick())
	}

	transform, ok := driver.world.Transform(game.player)
	if !ok {
		t.Fatal("expected player transform after back to tick")
	}
	if transform.Position.X != 20 {
		t.Fatalf("position after back to tick = %v, want 20", transform.Position.X)
	}

	state := driver.State()
	if state.HistoryLen != 1 || state.HistoryStartTick != 1 || state.HistoryEndTick != 1 {
		t.Fatalf("history after back to tick = len %v range %v-%v, want len 1 range 1-1",
			state.HistoryLen,
			state.HistoryStartTick,
			state.HistoryEndTick,
		)
	}
}

func TestDriverBackSecondsRestoresRelativeSnapshot(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{
		{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Kind:   input.EventButtonDown,
		},
	})
	for range 5 {
		driver.Advance(1)
	}

	driver.Control(ControlCommand{Kind: ControlBackSeconds, Seconds: 2.0 / 60.0})
	driver.Advance(1)

	if driver.Tick() != 3 {
		t.Fatalf("tick after back seconds = %v, want 3", driver.Tick())
	}

	transform, ok := driver.world.Transform(game.player)
	if !ok {
		t.Fatal("expected player transform after back seconds")
	}
	if transform.Position.X != 30 {
		t.Fatalf("position after back seconds = %v, want 30", transform.Position.X)
	}
}

func TestDriverPlayAfterBackToTickContinuesFromRestoredTick(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{
		{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Kind:   input.EventButtonDown,
		},
	})
	for range 5 {
		driver.Advance(1)
	}

	driver.Control(ControlCommand{Kind: ControlBackToTick, Tick: 2})
	driver.Advance(1)
	driver.Control(ControlCommand{Kind: ControlPlay})
	driver.Advance(1)

	if driver.Tick() != 3 {
		t.Fatalf("tick after play following back to tick = %v, want 3", driver.Tick())
	}

	transform, ok := driver.world.Transform(game.player)
	if !ok {
		t.Fatal("expected player transform after play following back to tick")
	}
	if transform.Position.X != 30 {
		t.Fatalf("position after play following back to tick = %v, want 30", transform.Position.X)
	}
}

func TestDriverSlowMotionSpeedAccumulatesFractionalTicks(t *testing.T) {
	driver := NewDriver(&inputBackGame{}, slog.Default())

	driver.Control(ControlCommand{Kind: ControlSetSpeed, Speed: 0.5})
	driver.Advance(1)
	if driver.Tick() != 0 {
		t.Fatalf("tick after first half-speed advance = %v, want 0", driver.Tick())
	}

	driver.Advance(1)
	if driver.Tick() != 1 {
		t.Fatalf("tick after second half-speed advance = %v, want 1", driver.Tick())
	}

	driver.Advance(1)
	if driver.Tick() != 1 {
		t.Fatalf("tick after third half-speed advance = %v, want 1", driver.Tick())
	}

	driver.Advance(1)
	if driver.Tick() != 2 {
		t.Fatalf("tick after fourth half-speed advance = %v, want 2", driver.Tick())
	}
}

func TestDriverSetSpeedClampsFractionalRange(t *testing.T) {
	driver := NewDriver(&inputBackGame{}, slog.Default())

	driver.Control(ControlCommand{Kind: ControlSetSpeed, Speed: 0.5})
	if driver.CurrentSpeed() != 0.5 {
		t.Fatalf("speed = %v, want 0.5", driver.CurrentSpeed())
	}

	driver.Control(ControlCommand{Kind: ControlSetSpeed, Speed: 0.01})
	if driver.CurrentSpeed() != 0.25 {
		t.Fatalf("low clamped speed = %v, want 0.25", driver.CurrentSpeed())
	}

	driver.Control(ControlCommand{Kind: ControlSetSpeed, Speed: 99})
	if driver.CurrentSpeed() != 8 {
		t.Fatalf("high clamped speed = %v, want 8", driver.CurrentSpeed())
	}
}

var _ Game = (*inputBackGame)(nil)
