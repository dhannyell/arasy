package runtime

import (
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
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

	frame := driver.Frame(0)
	want := math2d.NewRect(math2d.Vec2{X: 2, Y: 3}, math2d.Vec2{X: 10, Y: 10})

	if frame.Camera.World != want {
		t.Fatalf("camera world = %+v, want %+v", frame.Camera.World, want)
	}
}

type framePassGame struct{}

func (framePassGame) Setup() Config { return cameraGame{}.Setup() }

func (framePassGame) Init(ctx *UpdateContext) {}

func (framePassGame) Update(ctx *UpdateContext, dt float64) {}

func (framePassGame) Render(ctx *RenderContext) []render.Command {
	commands := []render.Command{render.DrawRect{
		Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
	}}
	passCamera := ctx.Camera()
	passCamera.Viewport = math2d.NewRect(math2d.Vec2{X: 4, Y: 5}, math2d.Vec2{X: 20, Y: 20})
	ctx.AddPass(passCamera, commands)
	commands[0] = nil
	ctx.AddPass(render.Camera{}, nil)
	return nil
}

func TestDriverFrameIncludesExtraPassesInDeclarationOrder(t *testing.T) {
	driver := NewDriver(framePassGame{}, slog.Default())
	frame := driver.Frame(0)

	if len(frame.Passes) != 2 {
		t.Fatalf("pass count = %d, want 2", len(frame.Passes))
	}
	if got := frame.Passes[0].Camera.Viewport.Min; got != (math2d.Vec2{X: 4, Y: 5}) {
		t.Fatalf("first pass viewport min = %v, want (4, 5)", got)
	}
	if len(frame.Passes[0].Commands) != 1 {
		t.Fatalf("first pass command count = %d, want 1", len(frame.Passes[0].Commands))
	}
	if _, ok := frame.Passes[0].Commands[0].(render.DrawRect); !ok {
		t.Fatalf("first pass command = %T, want copied DrawRect", frame.Passes[0].Commands[0])
	}
}

var _ Game = cameraGame{}

type renderClockGame struct {
	values []struct {
		time float64
		dt   float64
		tick uint64
	}
}

func (g *renderClockGame) Setup() Config { return cameraGame{}.Setup() }

func (g *renderClockGame) Init(ctx *UpdateContext) {}

func (g *renderClockGame) Update(ctx *UpdateContext, dt float64) {}

func (g *renderClockGame) Render(ctx *RenderContext) []render.Command {
	g.values = append(g.values, struct {
		time float64
		dt   float64
		tick uint64
	}{time: ctx.Time, dt: ctx.Dt, tick: ctx.Tick})
	return nil
}

func TestDriverFrameUsesRenderElapsedForContextTime(t *testing.T) {
	game := &renderClockGame{}
	driver := NewDriver(game, slog.Default())

	driver.Step()
	driver.Frame(100 * time.Millisecond)
	driver.Frame(250 * time.Millisecond)
	driver.Frame(0)

	if len(game.values) != 3 {
		t.Fatalf("render calls = %d, want 3", len(game.values))
	}
	if math.Abs(game.values[0].time-0.1) > 1e-9 || math.Abs(game.values[0].dt-0.1) > 1e-9 {
		t.Fatalf("first render time/dt = %+v, want 0.1/0.1", game.values[0])
	}
	if math.Abs(game.values[1].time-0.35) > 1e-9 || math.Abs(game.values[1].dt-0.25) > 1e-9 {
		t.Fatalf("second render time/dt = %+v, want 0.35/0.25", game.values[1])
	}
	if math.Abs(game.values[2].time-0.35) > 1e-9 || game.values[2].dt != 0 {
		t.Fatalf("Commands render time/dt = %+v, want 0.35/0", game.values[2])
	}
	for _, v := range game.values {
		if v.tick != 1 {
			t.Fatalf("render tick = %d, want simulation tick 1", v.tick)
		}
	}
}

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

func TestDriverSetViewportIgnoresDuplicateViewport(t *testing.T) {
	driver := NewDriver(cameraGame{}, slog.Default())
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720})
	other := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 600})

	driver.SetViewport(viewport)
	driver.SetViewport(viewport)
	driver.Step()
	driver.SetViewport(viewport)

	if got := len(driver.timeline.viewports); got != 1 {
		t.Fatalf("viewportLog len after duplicate viewport = %d, want 1", got)
	}

	driver.SetViewport(other)
	if got := len(driver.timeline.viewports); got != 2 {
		t.Fatalf("viewportLog len after new viewport = %d, want 2", got)
	}
}

func TestDriverForkFromScrubCommitsCurrentViewport(t *testing.T) {
	driver := NewDriver(cameraGame{}, slog.Default())
	first := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100})
	second := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 200})

	driver.SetViewport(first)
	driver.Step()
	driver.SetViewport(second)
	driver.Step()

	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 1})
	driver.AdvanceTime(0)
	driver.Step()

	if got := len(driver.timeline.viewports); got != 2 {
		t.Fatalf("viewportLog len after fork = %d, want 2", got)
	}
	if got := driver.timeline.viewports[1]; got.tick != 1 || got.viewport != second {
		t.Fatalf("fork viewport entry = %+v, want tick 1 viewport %+v", got, second)
	}
}
