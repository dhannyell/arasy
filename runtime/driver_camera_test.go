package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
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

func TestDriverSetViewportIgnoresDuplicateViewport(t *testing.T) {
	driver := NewDriver(cameraGame{}, slog.Default())
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720})
	other := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 600})

	driver.SetViewport(viewport)
	driver.SetViewport(viewport)
	driver.Step()
	driver.SetViewport(viewport)

	if got := len(driver.viewportLog); got != 1 {
		t.Fatalf("viewportLog len after duplicate viewport = %d, want 1", got)
	}

	driver.SetViewport(other)
	if got := len(driver.viewportLog); got != 2 {
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

	if got := len(driver.viewportLog); got != 2 {
		t.Fatalf("viewportLog len after fork = %d, want 2", got)
	}
	if got := driver.viewportLog[1]; got.tick != 1 || got.viewport != second {
		t.Fatalf("fork viewport entry = %+v, want tick 1 viewport %+v", got, second)
	}
}
