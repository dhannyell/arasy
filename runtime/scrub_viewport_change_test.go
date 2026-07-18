package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

// uiClickGame is a minimal game whose state transitions on a UI button click,
// like breakout's Play button. The button is at a fixed position in the
// reference canvas (SpaceRef), so its hit-test depends on the viewport.
type uiClickGame struct {
	state   bool
	clicked bool
	id      scene.EntityID
}

func (g *uiClickGame) Setup() Config {
	return Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		},
		SimulationHz: 60,
		HistoryLen:   100,
	}
}

func (g *uiClickGame) Init(ctx *UpdateContext) {
	g.state = false
	g.clicked = false
	g.id = ctx.World.Spawn()
	ctx.World.SetTransform(g.id, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
}

func (g *uiClickGame) Update(ctx *UpdateContext, dt float64) {
	if !g.state {
		ctx.HoldSimulation()
		rect := math2d.NewRect(math2d.Vec2{X: 40, Y: 40}, math2d.Vec2{X: 20, Y: 20})
		if ctx.UI.In(ui.SpaceRef).Clicked("btn", rect) {
			g.clicked = true
			g.state = true
		}
		return
	}
	transform, _ := ctx.World.Transform(g.id)
	transform.Position.X += 1
	ctx.World.SetTransform(g.id, transform)
}

func (g *uiClickGame) Render(ctx *RenderContext) []render.Command { return nil }

func mouseDownAt(x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
		Kind:   input.EventButtonDown, X: x, Y: y,
	}
}

func mouseUpAt(x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
		Kind:   input.EventButtonUp, X: x, Y: y,
	}
}

// TestScrubReplaysViewportChangesForUIHitTesting is the regression test for the
// breakout scrubber bug: when the viewport changes between recording and
// replay, pointer event coordinates (in screen pixels) map to the wrong
// reference-canvas position, so a UI button click that started the game live
// is missed during replay — the game stays frozen in its menu state.
func TestScrubReplaysViewportChangesForUIHitTesting(t *testing.T) {
	game := &uiClickGame{}
	driver := NewDriver(game, slog.Default())

	// Viewport 100×100: button at (40-60, 40-60); click at (50, 50) hits it.
	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}))
	driver.ApplyInput([]input.Event{mouseDownAt(50, 50), mouseUpAt(50, 50)})
	driver.Step()

	if !game.state {
		t.Fatal("live click did not start the game")
	}

	checksums := []uint64{driver.Checksum()}
	for range 5 {
		driver.Step()
		checksums = append(checksums, driver.Checksum())
	}

	// Resize to 200×200: screen (50,50) → reference (25,25) misses button.
	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 200}))

	driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 1})
	driver.AdvanceTime(0)

	if driver.State().Tick != 1 {
		t.Fatalf("tick after scrub = %d, want 1", driver.State().Tick)
	}
	if !game.state {
		t.Fatal("scrub to tick 1: game state = false — viewport change broke UI hit-testing during replay")
	}
	if got, want := driver.Checksum(), checksums[0]; got != want {
		t.Fatalf("scrub to tick 1: checksum = %d, want %d — replay diverged", got, want)
	}
}

// TestScrubWithoutViewportChangeStillWorks ensures the fix doesn't break the
// common case where the viewport never changes.
func TestScrubWithoutViewportChangeStillWorks(t *testing.T) {
	game := &uiClickGame{}
	driver := NewDriver(game, slog.Default())
	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}))

	checksums := []uint64{driver.Checksum()} // tick 0

	driver.ApplyInput([]input.Event{mouseDownAt(50, 50), mouseUpAt(50, 50)})
	driver.Step()
	checksums = append(checksums, driver.Checksum()) // tick 1
	if !game.state {
		t.Fatal("click did not start the game")
	}

	for range 5 {
		driver.Step()
		checksums = append(checksums, driver.Checksum()) // ticks 2..6
	}

	for _, target := range []uint64{3, 1, 5, 0} {
		driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: target})
		driver.AdvanceTime(0)
		if driver.State().Tick != target {
			t.Fatalf("scrub to %d: tick = %d", target, driver.State().Tick)
		}
		if got, want := driver.Checksum(), checksums[target]; got != want {
			t.Fatalf("scrub to %d: checksum = %d, want %d", target, got, want)
		}
	}
}

// TestScrubMultipleViewportChanges verifies multiple viewport changes are all
// replayed at their original ticks.
func TestScrubMultipleViewportChanges(t *testing.T) {
	game := &uiClickGame{}
	driver := NewDriver(game, slog.Default())

	checksums := []uint64{driver.Checksum()} // tick 0

	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}))
	driver.ApplyInput([]input.Event{mouseDownAt(50, 50), mouseUpAt(50, 50)})
	driver.Step()
	checksums = append(checksums, driver.Checksum()) // tick 1
	if !game.state {
		t.Fatal("click did not start the game")
	}

	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 200}))
	driver.Step()
	checksums = append(checksums, driver.Checksum()) // tick 2

	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 50, Y: 50}))
	driver.Step()
	checksums = append(checksums, driver.Checksum()) // tick 3

	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 300, Y: 300}))
	driver.Step()
	checksums = append(checksums, driver.Checksum()) // tick 4

	for range 3 {
		driver.Step()
		checksums = append(checksums, driver.Checksum()) // ticks 5..7
	}

	for _, target := range []uint64{1, 3, 5, 2, 0} {
		driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: target})
		driver.AdvanceTime(0)
		if driver.State().Tick != target {
			t.Fatalf("scrub to %d: tick = %d", target, driver.State().Tick)
		}
		if target >= 1 && !game.state {
			t.Fatalf("scrub to %d: game not started — viewport changes not replayed", target)
		}
		if got, want := driver.Checksum(), checksums[target]; got != want {
			t.Fatalf("scrub to %d: checksum = %d, want %d", target, got, want)
		}
	}
}
