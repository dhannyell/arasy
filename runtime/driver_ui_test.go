package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/ui"
)

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
