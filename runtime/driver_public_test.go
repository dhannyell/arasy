package runtime_test

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

type pumpGame struct {
	simulationHz int
	updates      int
	totalDT      float64
}

func (g *pumpGame) Setup() runtime.Config {
	return runtime.Config{SimulationHz: g.simulationHz}
}

func (g *pumpGame) Init(ctx *runtime.UpdateContext) {}

func (g *pumpGame) Update(ctx *runtime.UpdateContext, dt float64) {
	g.updates++
	g.totalDT += dt
}

func (g *pumpGame) Render(ctx *runtime.RenderContext) []render.Command { return nil }

func TestDriverAdvanceTimeClampsAndDiscardsExcess(t *testing.T) {
	game := &pumpGame{simulationHz: 1}
	driver := runtime.NewDriver(game, testLogger())

	stats := driver.AdvanceTime(10 * time.Second)
	if stats.Steps != 5 || !stats.Clamped {
		t.Fatalf("stats = %+v, want 5 clamped steps", stats)
	}
	if driver.State().Tick != 5 || game.updates != 5 {
		t.Fatalf("tick=%d updates=%d, want 5/5", driver.State().Tick, game.updates)
	}

	stats = driver.AdvanceTime(0)
	if stats.Steps != 0 || stats.Clamped {
		t.Fatalf("stats after clamp = %+v, want 0 unclamped steps", stats)
	}
}

func TestDriverAdvanceTimeAccumulatesFractionalElapsed(t *testing.T) {
	game := &pumpGame{simulationHz: 2}
	driver := runtime.NewDriver(game, testLogger())

	stats := driver.AdvanceTime(250 * time.Millisecond)
	if stats.Steps != 0 || stats.Alpha != 0.5 {
		t.Fatalf("first stats = %+v, want 0 steps alpha 0.5", stats)
	}

	stats = driver.AdvanceTime(250 * time.Millisecond)
	if stats.Steps != 1 || stats.Alpha != 0 {
		t.Fatalf("second stats = %+v, want 1 step alpha 0", stats)
	}
	if driver.State().Tick != 1 || game.updates != 1 {
		t.Fatalf("tick=%d updates=%d, want 1/1", driver.State().Tick, game.updates)
	}
}

func TestDriverStepDoesNotConsumeClockAccumulator(t *testing.T) {
	game := &pumpGame{simulationHz: 2}
	driver := runtime.NewDriver(game, testLogger())

	stats := driver.AdvanceTime(250 * time.Millisecond)
	if stats.Steps != 0 || stats.Alpha != 0.5 {
		t.Fatalf("stats before step = %+v, want 0 steps alpha 0.5", stats)
	}

	stats = driver.Step()
	if stats.Steps != 1 || stats.Alpha != 0.5 {
		t.Fatalf("step stats = %+v, want 1 step alpha 0.5", stats)
	}

	stats = driver.AdvanceTime(250 * time.Millisecond)
	if stats.Steps != 1 || stats.Alpha != 0 {
		t.Fatalf("stats after step = %+v, want accumulated 1 step alpha 0", stats)
	}
}

func TestDriverControlStepRunsWhenAdvanceTimeHasNoElapsed(t *testing.T) {
	game := &pumpGame{simulationHz: 60}
	driver := runtime.NewDriver(game, testLogger())

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlStep})
	stats := driver.AdvanceTime(0)

	if stats.Steps != 1 || driver.State().Tick != 1 || game.updates != 1 {
		t.Fatalf("stats=%+v tick=%d updates=%d, want one manual step", stats, driver.State().Tick, game.updates)
	}
}

func TestDriverStepAndAdvanceTimeReachSameStateForSameSimulatedDuration(t *testing.T) {
	stepGame := &pumpGame{simulationHz: 2}
	stepDriver := runtime.NewDriver(stepGame, testLogger())
	for range 4 {
		stepDriver.Step()
	}

	timeGame := &pumpGame{simulationHz: 2}
	timeDriver := runtime.NewDriver(timeGame, testLogger())
	for range 2 {
		timeDriver.AdvanceTime(time.Second)
	}

	if stepGame.updates != timeGame.updates {
		t.Fatalf("updates step=%d time=%d, want equal", stepGame.updates, timeGame.updates)
	}
	if stepGame.totalDT != timeGame.totalDT {
		t.Fatalf("total dt step=%v time=%v, want equal", stepGame.totalDT, timeGame.totalDT)
	}
	if stepDriver.State().Tick != timeDriver.State().Tick {
		t.Fatalf("ticks step=%d time=%d, want equal", stepDriver.State().Tick, timeDriver.State().Tick)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
