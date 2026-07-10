package runtime_test

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"

	"log/slog"
)

// runTickGame records the viewport and input action value seen by Update, and
// plays an audio cue every tick, so a single RunTick call can be checked for
// all of the canonical order's effects: viewport, controls, input, advance,
// and audio drain.
type runTickGame struct {
	player   scene.EntityID
	viewport math2d.Rect
	moveX    float64
	updates  int
}

func (g *runTickGame) Setup() runtime.Config {
	assets, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"audio":[{"id":"beep","file":"beep.wav"}]}`)},
		"beep.wav":      {Data: []byte("beep")},
	})
	if err != nil {
		panic(err)
	}
	return runtime.Config{
		Bounds: math2d.NewRect(
			math2d.Vec2{X: -100, Y: -100},
			math2d.Vec2{X: 200, Y: 200},
		),
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Aspect:   render.AspectExpand,
		},
		InputBindings: []input.Binding{
			{
				Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
				Action: input.ActionMoveX,
				Scale:  1,
			},
		},
		Assets: assets,
	}
}

func (g *runTickGame) Init(ctx *runtime.UpdateContext) {
	g.player = ctx.World.Spawn()
}

func (g *runTickGame) Update(ctx *runtime.UpdateContext, dt float64) {
	g.updates++
	g.viewport = ctx.Camera().Viewport
	g.moveX = ctx.Input.Value(input.ActionMoveX)
	ctx.Audio.Play("beep")
}

func (g *runTickGame) Render(ctx *runtime.RenderContext) []render.Command { return nil }

var _ runtime.Game = (*runTickGame)(nil)

// TestRunTickAppliesCanonicalOrder verifies Driver.RunTick applies a host
// frame's viewport, control commands, and input events, advances exactly the
// requested number of ticks, and drains the audio produced during that
// advance — matching the documented viewport -> controls -> input -> advance
// -> audio order.
func TestRunTickAppliesCanonicalOrder(t *testing.T) {
	game := &runTickGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720})
	batch := runtime.TickBatch{
		Viewport: &viewport,
		Controls: []runtime.ControlCommand{{Kind: runtime.ControlStep}},
		Events: []input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
		},
	}

	// ControlStep forces exactly one tick regardless of elapsed, so the
	// advance count is deterministic even though elapsed is nonzero here.
	stats, out := driver.RunTick(batch, 250*time.Millisecond)

	if stats.Steps != 1 {
		t.Fatalf("stats.Steps = %d, want 1", stats.Steps)
	}
	if driver.State().Tick != 1 {
		t.Fatalf("tick after RunTick = %d, want 1", driver.State().Tick)
	}
	if game.updates != 1 {
		t.Fatalf("Update calls = %d, want 1", game.updates)
	}
	if game.viewport != viewport {
		t.Fatalf("viewport seen by Update = %+v, want %+v — RunTick did not apply the batch viewport before stepping", game.viewport, viewport)
	}
	if game.moveX != 1 {
		t.Fatalf("moveX seen by Update = %v, want 1 — RunTick did not apply the batch input event before stepping", game.moveX)
	}
	if len(out.Cues) != 1 || out.Cues[0].Clip != "beep" {
		t.Fatalf("audio cues = %+v, want one 'beep' cue drained by RunTick", out.Cues)
	}

	// The cue was already drained by RunTick's own ConsumeAudio call.
	if drained := driver.ConsumeAudio(); len(drained.Cues) != 0 {
		t.Fatalf("cues remaining after RunTick = %d, want 0 (drained once by RunTick)", len(drained.Cues))
	}
}
