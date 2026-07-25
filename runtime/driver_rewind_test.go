package runtime

import (
	"log/slog"
	"testing"
	"time"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

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

func (g *inputBackGame) Init(ctx *UpdateContext) {
	g.player = ctx.World.Spawn()
	ctx.World.SetTransform(g.player, scene.Transform{
		Scale: math2d.Vec2{X: 1, Y: 1},
	})
	ctx.World.SetVelocity(g.player, scene.Velocity{})
}

func (g *inputBackGame) Update(ctx *UpdateContext, dt float64) {
	ctx.World.SetVelocity(g.player, scene.Velocity{
		Linear: math2d.Vec2{X: ctx.Input.Value(input.ActionMoveX) * 600},
	})
}

func (g *inputBackGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestDriverBackRestoresInputSnapshotBeforeStep(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{
		{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Kind:   input.EventButtonDown,
		},
	})
	driver.Step()

	transform, ok := driver.world.Transform(game.player)
	if !ok {
		t.Fatal("expected player transform")
	}
	if transform.Position.X != 10 {
		t.Fatalf("position after input advance = %v, want 10", transform.Position.X)
	}

	driver.Control(ControlCommand{Kind: ControlBack})
	driver.AdvanceTime(0)

	transform, ok = driver.world.Transform(game.player)
	if !ok {
		t.Fatal("expected player transform after back")
	}
	if transform.Position.X != 0 {
		t.Fatalf("position after back = %v, want 0", transform.Position.X)
	}

	driver.Control(ControlCommand{Kind: ControlStep})
	driver.AdvanceTime(0)

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
		driver.Step()
	}

	driver.Control(ControlCommand{Kind: ControlBackToTick, Tick: 2})
	driver.AdvanceTime(0)

	if driver.State().Tick != 2 {
		t.Fatalf("tick after back to tick = %v, want 2", driver.State().Tick)
	}

	transform, ok := driver.world.Transform(game.player)
	if !ok {
		t.Fatal("expected player transform after back to tick")
	}
	if transform.Position.X != 20 {
		t.Fatalf("position after back to tick = %v, want 20", transform.Position.X)
	}

	state := driver.State()
	if state.HistoryLen != 1 || state.HistoryStartTick != 0 || state.HistoryEndTick != 0 {
		t.Fatalf("history after back to tick = len %v range %v-%v, want len 1 range 0-0",
			state.HistoryLen,
			state.HistoryStartTick,
			state.HistoryEndTick,
		)
	}
}

func TestDriverBackToTickIsIdempotentForRepeatedTarget(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{
		{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Kind:   input.EventButtonDown,
		},
	})
	for range 5 {
		driver.Step()
	}

	driver.Control(ControlCommand{Kind: ControlBackToTick, Tick: 2})
	driver.AdvanceTime(0)

	// Sending the same target again (as a scrubber UI throttled by wall
	// time might) must be a no-op, not rewind a second tick further.
	driver.Control(ControlCommand{Kind: ControlBackToTick, Tick: 2})
	driver.AdvanceTime(0)

	if driver.State().Tick != 2 {
		t.Fatalf("tick after repeated back to tick = %v, want 2", driver.State().Tick)
	}

	transform, ok := driver.world.Transform(game.player)
	if !ok {
		t.Fatal("expected player transform after repeated back to tick")
	}
	if transform.Position.X != 20 {
		t.Fatalf("position after repeated back to tick = %v, want 20", transform.Position.X)
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
		driver.Step()
	}

	driver.Control(ControlCommand{Kind: ControlBackSeconds, Seconds: 2.0 / 60.0})
	driver.AdvanceTime(0)

	if driver.State().Tick != 3 {
		t.Fatalf("tick after back seconds = %v, want 3", driver.State().Tick)
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
		driver.Step()
	}

	driver.Control(ControlCommand{Kind: ControlBackToTick, Tick: 2})
	driver.AdvanceTime(0)
	driver.Control(ControlCommand{Kind: ControlPlay})
	driver.AdvanceTime(time.Second/60 + time.Nanosecond)

	if driver.State().Tick != 3 {
		t.Fatalf("tick after play following back to tick = %v, want 3", driver.State().Tick)
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
	driver.AdvanceTime(time.Second/60 + time.Nanosecond)
	if driver.State().Tick != 0 {
		t.Fatalf("tick after first half-speed advance = %v, want 0", driver.State().Tick)
	}

	driver.AdvanceTime(time.Second/60 + time.Nanosecond)
	if driver.State().Tick != 1 {
		t.Fatalf("tick after second half-speed advance = %v, want 1", driver.State().Tick)
	}

	driver.AdvanceTime(time.Second/60 + time.Nanosecond)
	if driver.State().Tick != 1 {
		t.Fatalf("tick after third half-speed advance = %v, want 1", driver.State().Tick)
	}

	driver.AdvanceTime(time.Second/60 + time.Nanosecond)
	if driver.State().Tick != 2 {
		t.Fatalf("tick after fourth half-speed advance = %v, want 2", driver.State().Tick)
	}
}

func TestDriverSetSpeedClampsFractionalRange(t *testing.T) {
	driver := NewDriver(&inputBackGame{}, slog.Default())

	driver.Control(ControlCommand{Kind: ControlSetSpeed, Speed: 0.5})
	if driver.State().Speed != 0.5 {
		t.Fatalf("speed = %v, want 0.5", driver.State().Speed)
	}

	driver.Control(ControlCommand{Kind: ControlSetSpeed, Speed: 0.01})
	if driver.State().Speed != 0.25 {
		t.Fatalf("low clamped speed = %v, want 0.25", driver.State().Speed)
	}

	driver.Control(ControlCommand{Kind: ControlSetSpeed, Speed: 99})
	if driver.State().Speed != 8 {
		t.Fatalf("high clamped speed = %v, want 8", driver.State().Speed)
	}
}

var _ Game = (*inputBackGame)(nil)
