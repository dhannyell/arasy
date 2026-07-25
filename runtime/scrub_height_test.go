package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// ballisticScrubGame drops a projectile whose only per-tick varying simulation
// state is its Height (Z/VZ under gravity and bounce). If scrubbing restored
// everything but Height, the world checksum would not match — so this doubles
// as the "z survives snapshot/replay" round-trip test.
type ballisticScrubState struct {
	Ball scene.EntityID
}

type ballisticScrubGame struct {
	StateSlot[ballisticScrubState]
}

func (g *ballisticScrubGame) Setup() Config {
	return Config{HistoryLen: 120, KeyframeInterval: 10}
}

func (g *ballisticScrubGame) Init(ctx *UpdateContext) {
	g.State.Ball = ctx.World.Spawn()
	ctx.World.SetTransform(g.State.Ball, scene.Transform{
		Position: math2d.Vec2{X: 5, Y: 5},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	// No Velocity: X stays fixed so the checksum varies only with Height. The
	// engine auto-steps the world each tick.
	ctx.World.SetHeight(g.State.Ball, scene.Height{Z: 20, VZ: 0})
	ctx.World.SetBallistics(g.State.Ball, scene.Ballistics{Gravity: 9.8, BounceDamping: 0.6, RestVZ: 0.1})
}

func (g *ballisticScrubGame) Update(ctx *UpdateContext, dt float64) {}

func (g *ballisticScrubGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestScrubRestoresBallisticHeight(t *testing.T) {
	game := &ballisticScrubGame{}
	driver := NewDriver(game, slog.Default())

	checksums := make([]uint64, 0, 61)
	checksums = append(checksums, driver.Checksum()) // tick 0
	for range 60 {
		driver.Step()
		checksums = append(checksums, driver.Checksum())
	}

	// Sanity: the projectile's Height must actually move the checksum, otherwise
	// a constant hash would make the round-trip check vacuous.
	if checksums[10] == checksums[0] {
		t.Fatal("checksum did not change while the projectile was in flight")
	}

	for _, target := range []uint64{15, 45, 5, 60, 30} {
		driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: target})
		driver.AdvanceTime(0)

		if got := driver.State().Tick; got != target {
			t.Fatalf("scrub to %d: tick = %d", target, got)
		}
		if got, want := driver.Checksum(), checksums[target]; got != want {
			t.Fatalf("scrub to %d: Height not restored (checksum %d, want %d)", target, got, want)
		}
	}
}
