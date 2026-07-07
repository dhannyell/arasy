package runtime

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

type randDriverGame struct {
	seed       uint64
	historyLen int
	setupSeed  func() uint64

	player scene.EntityID
	draws  []uint64
}

func (g *randDriverGame) Setup() Config {
	seed := g.seed
	if g.setupSeed != nil {
		seed = g.setupSeed()
	}
	return Config{
		Bounds: math2d.NewRect(
			math2d.Vec2{X: -1000, Y: -1000},
			math2d.Vec2{X: 2000, Y: 2000},
		),
		InputBindings: []input.Binding{
			{
				Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
				Action: input.ActionMoveX,
				Scale:  1,
			},
		},
		HistoryLen: g.historyLen,
		Seed:       seed,
	}
}

func (g *randDriverGame) Init(ctx *UpdateContext) {
	g.player = ctx.World.Spawn()
	ctx.World.SetTransform(g.player, scene.Transform{
		Scale: math2d.Vec2{X: 1, Y: 1},
	})
}

func (g *randDriverGame) Update(ctx *UpdateContext, dt float64) {
	draw := ctx.Rand.Uint64()
	g.draws = append(g.draws, draw)

	inputScale := 1.0
	if ctx.Input.Value(input.ActionMoveX) > 0 {
		inputScale = 2.0
	}
	ctx.World.SetTransform(g.player, scene.Transform{
		Position: math2d.Vec2{
			X: float64(draw%1000) * inputScale,
			Y: float64((draw / 1000) % 1000),
		},
		Scale: math2d.Vec2{X: 1, Y: 1},
	})
}

func (g *randDriverGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestRandDriverSameSeedProducesSameRun(t *testing.T) {
	leftGame := &randDriverGame{seed: 42}
	left := NewDriver(leftGame, slog.Default())
	rightGame := &randDriverGame{seed: 42}
	right := NewDriver(rightGame, slog.Default())

	for tick := range 100 {
		applyRandDriverInput(left, tick)
		applyRandDriverInput(right, tick)
		left.Step()
		right.Step()
	}

	if !reflect.DeepEqual(leftGame.draws, rightGame.draws) {
		t.Fatal("same seed and input script produced different random draws")
	}
	if got, want := left.Checksum(), right.Checksum(); got != want {
		t.Fatalf("same seed and input script checksum = %d, want %d", got, want)
	}
}

func TestRandDriverDifferentSeedsDiverge(t *testing.T) {
	leftGame := &randDriverGame{seed: 1}
	left := NewDriver(leftGame, slog.Default())
	rightGame := &randDriverGame{seed: 2}
	right := NewDriver(rightGame, slog.Default())

	for range 20 {
		left.Step()
		right.Step()
	}

	if reflect.DeepEqual(leftGame.draws, rightGame.draws) {
		t.Fatal("different seeds produced identical random draws")
	}
}

func TestRandDriverBackRestoresStream(t *testing.T) {
	game := &randDriverGame{seed: 42, historyLen: 32}
	driver := NewDriver(game, slog.Default())

	const totalTicks = 20
	for range totalTicks {
		driver.Step()
	}
	original := append([]uint64(nil), game.draws...)

	const target = 7
	driver.Control(ControlCommand{Kind: ControlBackToTick, Tick: target})
	driver.AdvanceTime(0)
	if driver.State().Tick != target {
		t.Fatalf("tick after back = %d, want %d", driver.State().Tick, target)
	}

	base := len(game.draws)
	for driver.State().Tick < totalTicks {
		driver.Step()
	}
	got := game.draws[base:]
	if !reflect.DeepEqual(got, original[target:]) {
		t.Fatalf("draws after back = %016x, want %016x", got, original[target:])
	}
}

func TestRandDriverScrubRoundTripRestoresStream(t *testing.T) {
	game := &randDriverGame{seed: 42, historyLen: 32}
	driver := NewDriver(game, slog.Default())

	checksums := []uint64{driver.Checksum()}
	for tick := range 30 {
		applyRandDriverInput(driver, tick)
		driver.Step()
		checksums = append(checksums, driver.Checksum())
	}

	for _, target := range []uint64{10, 25, 5, 30} {
		driver.Control(ControlCommand{Kind: ControlScrubToTick, Tick: target})
		driver.AdvanceTime(0)
		if driver.State().Tick != target {
			t.Fatalf("scrub to %d: tick = %d, want %d", target, driver.State().Tick, target)
		}
		if got, want := driver.Checksum(), checksums[target]; got != want {
			t.Fatalf("scrub to %d: checksum = %d, want %d", target, got, want)
		}
	}
}

func TestRandReplayUsesRecordedSeed(t *testing.T) {
	var setupCalls uint64
	nextSeed := func() uint64 {
		setupCalls++
		return setupCalls
	}
	game := &randDriverGame{setupSeed: nextSeed}
	driver := NewDriver(game, slog.Default())

	driver.StartRecording()
	for tick := range 60 {
		applyRandDriverInput(driver, tick)
		driver.Step()
	}
	rec := driver.StopRecording()
	if rec.Seed != 1 {
		t.Fatalf("recording seed = %d, want first setup seed 1", rec.Seed)
	}

	replay := Replay(&randDriverGame{setupSeed: nextSeed}, slog.Default(), rec)
	if got, want := replay.Checksum(), driver.Checksum(); got != want {
		t.Fatalf("replay checksum = %d, want %d", got, want)
	}
	if setupCalls < 2 {
		t.Fatalf("setup calls = %d, want replay game to produce a different default seed", setupCalls)
	}
}

func TestRandDriverResetReseedsStream(t *testing.T) {
	game := &randDriverGame{seed: 42}
	driver := NewDriver(game, slog.Default())

	const ticks = 12
	for range ticks {
		driver.Step()
	}
	firstRun := append([]uint64(nil), game.draws...)

	driver.Control(ControlCommand{Kind: ControlReset})
	driver.AdvanceTime(0)

	base := len(game.draws)
	for range ticks {
		driver.Step()
	}
	secondRun := game.draws[base:]

	if !reflect.DeepEqual(secondRun, firstRun) {
		t.Fatalf("draws after reset = %016x, want %016x", secondRun, firstRun)
	}
}

func applyRandDriverInput(driver *Driver, tick int) {
	kind := input.EventButtonUp
	if tick%7 < 3 {
		kind = input.EventButtonDown
	}
	driver.ApplyInput([]input.Event{
		{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: kind},
	})
}
