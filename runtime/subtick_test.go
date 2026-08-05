package runtime_test

import (
	"math"
	"testing"
	"time"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// subTickGame captures the continuous-clock readings seen during Render.
type subTickGame struct {
	interpolation bool

	tick             uint64
	subTick          float64
	simTime          float64
	simTimeAt        float64
	ticksSinceFuture float64
	ticksSinceZero   float64
}

func (g *subTickGame) Setup() runtime.Config {
	return runtime.Config{
		RenderInterpolation: g.interpolation,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		},
	}
}

func (g *subTickGame) Init(ctx *runtime.UpdateContext) {}

func (g *subTickGame) Update(ctx *runtime.UpdateContext, dt float64) {}

func (g *subTickGame) Render(ctx *runtime.RenderContext) []render.Command {
	g.tick = ctx.Tick
	g.subTick = ctx.SubTick()
	g.simTime = ctx.SimTime()
	g.simTimeAt = ctx.SimTimeSince(ctx.Tick + 1)
	g.ticksSinceFuture = ctx.TicksSince(ctx.Tick + 10)
	g.ticksSinceZero = ctx.TicksSince(0)
	return nil
}

// runSubTickGame advances the driver by elapsed and renders one frame.
func runSubTickGame(interpolation bool, elapsed time.Duration) *subTickGame {
	game := &subTickGame{interpolation: interpolation}
	driver := runtime.NewDriver(game, testLogger())
	driver.AdvanceTime(elapsed)
	driver.Frame(elapsed)
	return game
}

// The default config has interpolation off, so this degradation path is what
// every game hits first.
func TestSubTickEqualsTickWithoutInterpolation(t *testing.T) {
	// 2.5 ticks at 60 Hz: the driver consumes 2 ticks and drops the fraction.
	game := runSubTickGame(false, 5*time.Second/120)

	if game.tick != 2 {
		t.Fatalf("tick expected 2, got %d", game.tick)
	}

	if game.subTick != float64(game.tick) {
		t.Fatalf("sub tick expected %f, got %v", game.subTick, game.tick)
	}

	expectedSimTime := float64(game.tick) / 60
	if math.Abs(game.simTime-expectedSimTime) > 1e-9 {
		t.Fatalf("simTime expected %f, got %v", expectedSimTime, game.simTime)
	}

	expectedSimTimeAt := -1.0 / 60
	if math.Abs(game.simTimeAt-expectedSimTimeAt) > 1e-9 {
		t.Fatalf("simTimeAt expected %f, got %v", expectedSimTimeAt, game.simTimeAt)
	}
}

func TestSubTickIncludesFractionWithInterpolation(t *testing.T) {
	// 1.5 ticks at 60 Hz: 1 consumed tick plus half a tick in the accumulator.
	game := runSubTickGame(true, 3*time.Second/120)

	want := float64(game.tick) + 0.5
	if math.Abs(game.subTick-want) > 1e-6 {
		t.Fatalf("SubTick() = %v, want %v", game.subTick, want)
	}
}

func TestTicksSinceFutureStartIsZero(t *testing.T) {
	game := runSubTickGame(true, 3*time.Second/120)

	if game.ticksSinceFuture != 0 {
		t.Fatalf("TicksSince(future) = %v, want 0", game.ticksSinceFuture)
	}
}

func TestTicksSinceCountsFractionalTicks(t *testing.T) {
	// 3.5 ticks at 60 Hz: TicksSince(0) must read 3.5, not 3.
	game := runSubTickGame(true, 7*time.Second/120)

	if math.Abs(game.ticksSinceZero-3.5) > 1e-6 {
		t.Fatalf("TicksSince(0) = %v, want 3.5", game.ticksSinceZero)
	}
}
