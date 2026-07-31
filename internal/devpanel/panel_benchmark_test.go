//go:build arr_dev

package devpanel

import (
	"testing"
	"time"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

// benchGame builds a world busy enough for frame costs to matter: a few
// thousand entities with sprites, so Frame() walks real work and a careless
// per-frame overhead would show up in the numbers.
type benchGame struct{}

func (benchGame) Setup() runtime.Config { return testGame{}.Setup() }
func (g benchGame) Init(ctx *runtime.UpdateContext) {
	for i := 0; i < 3000; i++ {
		id := ctx.World.Spawn()
		ctx.World.SetTransform(id, scene.Transform{
			Position: math2d.Vec2{X: float64(i % 60 * 13), Y: float64(i / 60 * 13)},
		})
		ctx.World.SetRectSprite(id, scene.RectSprite{
			BaseSprite: scene.BaseSprite{Layer: i % 4},
			Rect: math2d.NewRect(math2d.Vec2{X: -4, Y: -4},
				math2d.Vec2{X: 8, Y: 8}),
		})
	}
}
func (g benchGame) Update(*runtime.UpdateContext, float64) {}
func (g benchGame) Render(ctx *runtime.RenderContext) []render.Command {
	return ctx.World.RenderCommands()
}

func newBenchDriver(b *testing.B) *runtime.Driver {
	b.Helper()
	driver := runtime.NewDriver(benchGame{}, quietLog())
	driver.SkipSplash()
	driver.SetViewport(testViewport())
	driver.Step()
	return driver
}

// BenchmarkFrameAlone is the release-mode baseline: no panel exists.
func BenchmarkFrameAlone(b *testing.B) {
	driver := newBenchDriver(b)
	elapsed := 16 * time.Millisecond
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame := driver.Frame(elapsed)
		_ = frame
	}
}

// BenchmarkFrameWithClosedPanel is the arr_dev build with the panel mounted
// but never opened: the cost the tag must keep at zero.
func BenchmarkFrameWithClosedPanel(b *testing.B) {
	driver := newBenchDriver(b)
	panel := New(driver, Options{Log: quietLog()})
	elapsed := 16 * time.Millisecond
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		panel.HandleInput(nil, testViewport())
		frame := driver.Frame(elapsed)
		frame.Commands = append(frame.Commands, panel.Render(frame.Camera, elapsed, Stats{})...)
		_ = frame
	}
}

// BenchmarkFrameWithOpenPanel frames with the panel open on the Sim tab: the
// acknowledged cost of having tooling on screen.
func BenchmarkFrameWithOpenPanel(b *testing.B) {
	driver := newBenchDriver(b)
	panel := New(driver, Options{Log: quietLog(), DevMode: true})
	panel.SetVisible(true)
	elapsed := 16 * time.Millisecond
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		panel.HandleInput(nil, testViewport())
		frame := driver.Frame(elapsed)
		frame.Commands = append(frame.Commands, panel.Render(frame.Camera, elapsed, Stats{})...)
		_ = frame
	}
}
