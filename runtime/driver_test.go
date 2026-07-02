package runtime

import (
	"log/slog"
	"testing"
	"time"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
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

func (cameraGame) Init(ctx *UpdateContext) {}

func (cameraGame) Update(ctx *UpdateContext, dt float64) {}

func (cameraGame) Render(ctx *RenderContext) []render.Command {
	ctx.camera.World = math2d.NewRect(math2d.Vec2{X: 2, Y: 3}, math2d.Vec2{X: 10, Y: 10})
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

type updateCameraGame struct {
	viewport math2d.Rect
}

func (g *updateCameraGame) Setup() Config {
	return Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Aspect:   render.AspectStretch,
		},
	}
}

func (g *updateCameraGame) Init(ctx *UpdateContext) {}

func (g *updateCameraGame) Update(ctx *UpdateContext, dt float64) {
	g.viewport = ctx.Camera().Viewport
}

func (g *updateCameraGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestDriverUpdateContextUsesSurfaceViewport(t *testing.T) {
	game := &updateCameraGame{}
	driver := NewDriver(game, slog.Default())
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720})

	driver.SetViewport(viewport)
	driver.Step()

	if game.viewport != viewport {
		t.Fatalf("update viewport = %+v, want %+v", game.viewport, viewport)
	}
}

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
	ctx.World.Step(dt, ctx.StepConfig)
}

func (g *inputBackGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestDriverBackRestoresInputSnapshotBeforeManualStep(t *testing.T) {
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

	if driver.Tick() != 2 {
		t.Fatalf("tick after repeated back to tick = %v, want 2", driver.Tick())
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
		driver.Step()
	}

	driver.Control(ControlCommand{Kind: ControlBackToTick, Tick: 2})
	driver.AdvanceTime(0)
	driver.Control(ControlCommand{Kind: ControlPlay})
	driver.AdvanceTime(time.Second/60 + time.Nanosecond)

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
	driver.AdvanceTime(time.Second/60 + time.Nanosecond)
	if driver.Tick() != 0 {
		t.Fatalf("tick after first half-speed advance = %v, want 0", driver.Tick())
	}

	driver.AdvanceTime(time.Second/60 + time.Nanosecond)
	if driver.Tick() != 1 {
		t.Fatalf("tick after second half-speed advance = %v, want 1", driver.Tick())
	}

	driver.AdvanceTime(time.Second/60 + time.Nanosecond)
	if driver.Tick() != 1 {
		t.Fatalf("tick after third half-speed advance = %v, want 1", driver.Tick())
	}

	driver.AdvanceTime(time.Second/60 + time.Nanosecond)
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

// statefulGame is a minimal StatefulGame built on StateSlot[int]: no
// hand-written GameState/RestoreGameState/Clone — GameState() and
// RestoreGameState() come promoted from the embedded StateSlot.
type statefulGame struct {
	StateSlot[int]
}

func (g *statefulGame) Setup() Config                              { return Config{HistoryLen: 4} }
func (g *statefulGame) Init(ctx *UpdateContext)                    {}
func (g *statefulGame) Update(ctx *UpdateContext, dt float64)      { g.State++ }
func (g *statefulGame) Render(ctx *RenderContext) []render.Command { return nil }

var (
	_ Game         = (*statefulGame)(nil)
	_ StatefulGame = (*statefulGame)(nil)
)

func TestDriverBackRestoresGameStateInsteadOfResettingIt(t *testing.T) {
	game := &statefulGame{}
	driver := NewDriver(game, slog.Default())

	driver.Step() // State: 0 -> 1, snapshot captured at State=0
	driver.Step() // State: 1 -> 2, snapshot captured at State=1

	if game.State != 2 {
		t.Fatalf("counter = %d, want 2", game.State)
	}

	driver.Control(ControlCommand{Kind: ControlBack})
	driver.AdvanceTime(0)

	if game.State != 1 {
		t.Fatalf("counter after back = %d, want 1 (restored from snapshot, not reset to 0)", game.State)
	}
}

type pointerUIGame struct {
	rect   math2d.Rect
	theme  ui.Theme
	clicks int
}

func newPointerUIGame() *pointerUIGame {
	theme := ui.DefaultTheme()
	theme.Button.Normal.Background = render.RGBA(0.1, 0.1, 0.1, 1)
	theme.Button.Hovered.Background = render.RGBA(0.2, 0.2, 0.2, 1)
	theme.Button.Active.Background = render.RGBA(0.3, 0.3, 0.3, 1)

	return &pointerUIGame{
		rect:  math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 20, Y: 20}),
		theme: theme,
	}
}

func (g *pointerUIGame) Setup() Config {
	return Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Aspect:   render.AspectStretch,
		},
		HistoryLen: 8,
		UITheme:    g.theme,
	}
}

func (g *pointerUIGame) Init(ctx *UpdateContext) {}

func (g *pointerUIGame) Update(ctx *UpdateContext, dt float64) {
	if ctx.UI.Clicked("button", g.rect) {
		g.clicks++
	}
}

func (g *pointerUIGame) Render(ctx *RenderContext) []render.Command {
	ctx.UI.Button("button", "Button", g.rect)
	return nil
}

func TestDriverPointerClickIsConsumedOnce(t *testing.T) {
	game := newPointerUIGame()
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{
		mouseButtonEvent(input.EventButtonDown, 15, 15),
		mouseButtonEvent(input.EventButtonUp, 15, 15),
	})
	driver.Step()

	if game.clicks != 1 {
		t.Fatalf("clicks after click tick = %v, want 1", game.clicks)
	}

	driver.Step()

	if game.clicks != 1 {
		t.Fatalf("clicks after next tick = %v, want old pointer events consumed", game.clicks)
	}
}

func TestDriverPointerMoveDrivesRenderHover(t *testing.T) {
	game := newPointerUIGame()
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{mouseMoveEvent(15, 15)})

	if got, want := buttonFill(t, driver), game.theme.Button.Hovered.Background; got != want {
		t.Fatalf("button fill after hover = %+v, want hovered %+v", got, want)
	}
}

func TestDriverPointerLeaveReleasesButtonFromPreviousTick(t *testing.T) {
	game := newPointerUIGame()
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{mouseButtonEvent(input.EventButtonDown, 15, 15)})
	driver.Step()

	if got, want := buttonFill(t, driver), game.theme.Button.Active.Background; got != want {
		t.Fatalf("button fill after down = %+v, want active %+v", got, want)
	}

	driver.ApplyInput([]input.Event{mouseLeaveEvent()})
	driver.Step()

	if got, want := buttonFill(t, driver), game.theme.Button.Normal.Background; got != want {
		t.Fatalf("button fill after leave = %+v, want normal %+v", got, want)
	}
}

func TestDriverBackRestoresPointerSnapshot(t *testing.T) {
	game := newPointerUIGame()
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{mouseMoveEvent(15, 15)})
	driver.Step()
	driver.ApplyInput([]input.Event{mouseLeaveEvent()})

	if got, want := buttonFill(t, driver), game.theme.Button.Normal.Background; got != want {
		t.Fatalf("button fill after leave = %+v, want normal %+v", got, want)
	}

	driver.Control(ControlCommand{Kind: ControlBack})
	driver.AdvanceTime(0)

	if got, want := buttonFill(t, driver), game.theme.Button.Hovered.Background; got != want {
		t.Fatalf("button fill after back = %+v, want restored hover %+v", got, want)
	}
}

func TestDriverBackToTickRestoresUIStateSnapshot(t *testing.T) {
	game := newPointerUIGame()
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{mouseButtonEvent(input.EventButtonDown, 15, 15)})
	driver.Step()
	driver.Step()

	driver.ApplyInput([]input.Event{mouseLeaveEvent()})
	driver.Step()

	if got, want := buttonFill(t, driver), game.theme.Button.Normal.Background; got != want {
		t.Fatalf("button fill after leave = %+v, want normal %+v", got, want)
	}

	driver.Control(ControlCommand{Kind: ControlBackToTick, Tick: 1})
	driver.AdvanceTime(0)

	if got, want := buttonFill(t, driver), game.theme.Button.Active.Background; got != want {
		t.Fatalf("button fill after back to active tick = %+v, want active %+v", got, want)
	}
}

func mouseMoveEvent(x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseX},
		Kind:   input.EventPointerMove,
		X:      x,
		Y:      y,
	}
}

func mouseButtonEvent(kind input.EventKind, x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
		Kind:   kind,
		X:      x,
		Y:      y,
	}
}

func mouseLeaveEvent() input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseUnknown},
		Kind:   input.EventPointerLeave,
	}
}

func buttonFill(t *testing.T, driver *Driver) render.Color {
	t.Helper()

	for _, command := range driver.Frame().Commands {
		rect, ok := command.(render.DrawRect)
		if !ok || rect.Layer() < ui.LayerBase {
			continue
		}
		return rect.Fill
	}

	t.Fatal("button rect command missing")
	return render.Color{}
}
