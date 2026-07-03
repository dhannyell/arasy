package runtime_test

// Seek benchmark suite: measures the cost of every time-travel path the
// Driver exposes today, through the same public API the platforms use
// (Control + AdvanceTime / Step / Replay). It is the baseline for the
// keyframe-recorder redesign (docs/plano-recorder-keyframes-e-persistencia):
// any change to the recorder must be compared against these numbers.
//
// What each benchmark isolates:
//
//   - BenchmarkDriverStep       — live per-tick cost, with and without the
//     snapshot recorder. The history=0 vs history=600 delta is the price of
//     capturing one full driver snapshot per tick (World.Clone + input +
//     UI + ring shift once the window is full).
//   - BenchmarkDriverBack       — O(1) seek: restore the previous snapshot.
//     Each timed iteration pops exactly one snapshot; the refill step that
//     re-creates it runs with the timer stopped. Run with -benchtime=Nx:
//     the untimed refill dominates wall time, not measured time.
//   - BenchmarkDriverScrubToTick — O(target) seek: deterministic replay of
//     the input log from tick 0 up to the target. Cost depends only on the
//     target tick, never on the distance from the current tick — "to-last"
//     is the pathological one-tick rewind from the head of a long timeline.
//   - BenchmarkReplay           — full-run reconstruction from a Recording
//     with no snapshot recorder (HistoryLen=0): the pure input-log cost,
//     i.e. what one re-simulated tick costs with zero snapshot overhead.
//     Each iteration also verifies the replayed checksum, so the benchmark
//     doubles as a determinism check.
//
// BackToTick/BackSeconds share doBackToTick's restore machinery with Back
// and are O(history scan) + one restore — they are not benchmarked
// separately because their cost does not depend on seek distance.
//
// The workload (seekBenchGame) mirrors the scene package benchmarks: a grid
// of moving circle colliders with bodies, spread over a 1000×1000 world, so
// World.Step exercises the full pipeline (integration, bounds bounce, grid
// broad phase, contact resolution). A deterministic key-press pattern is
// injected while priming so the scrub log carries real events, like an
// interactive session.

import (
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
)

var seekBenchSizes = []int{100, 1000}

const (
	// seekBenchHistory is the snapshot window used across the suite:
	// 600 ticks = 10s at 60Hz, the HistoryLen a dev-tooling session uses.
	seekBenchHistory = 600

	// seekBenchInputPeriod is how often the priming loop toggles the bound
	// key, producing an input log with realistic edge-event density.
	seekBenchInputPeriod = 15
)

// seekBenchTimelines are the recorded-run lengths scrubbed over: 10s and
// 1min at 60Hz. The long one exposes the O(target) growth of ScrubToTick.
var seekBenchTimelines = []int{600, 3600}

type seekBenchGame struct {
	entities   int
	historyLen int
	player     scene.EntityID
}

func (g *seekBenchGame) Setup() runtime.Config {
	return runtime.Config{
		Bounds: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1000, Y: 1000}),
		InputBindings: []input.Binding{{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Action: input.ActionMoveX,
			Scale:  1,
		}},
		HistoryLen: g.historyLen,
	}
}

func (g *seekBenchGame) Init(ctx *runtime.UpdateContext) {
	cols := seekBenchCeilSqrt(g.entities)
	spacing := 1000.0 / float64(cols)

	for i := 0; i < g.entities; i++ {
		entity := ctx.World.Spawn()
		ctx.World.SetTransform(entity, scene.Transform{
			Position: math2d.Vec2{
				X: float64(i%cols)*spacing + spacing*0.5,
				Y: float64(i/cols)*spacing + spacing*0.5,
			},
			Scale: math2d.Vec2{X: 1, Y: 1},
		})
		ctx.World.SetCircleCollider(entity, scene.CircleCollider{Radius: 1})
		ctx.World.SetBody(entity, scene.Body{InvMass: 1, Restitution: 1})
		// Deterministic pseudo-random velocities keep the grid moving and
		// colliding without any RNG (see AGENTS.md, Simulation Determinism).
		ctx.World.SetVelocity(entity, scene.Velocity{
			Linear: math2d.Vec2{
				X: float64(i%7-3) * 25,
				Y: float64(i%5-2) * 25,
			},
		})

		if i == 0 {
			g.player = entity
		}
	}
}

func (g *seekBenchGame) Update(ctx *runtime.UpdateContext, dt float64) {
	ctx.World.SetVelocity(g.player, scene.Velocity{
		Linear: math2d.Vec2{X: ctx.Input.Value(input.ActionMoveX) * 120},
	})
}

func (g *seekBenchGame) Render(ctx *runtime.RenderContext) []render.Command { return nil }

func BenchmarkDriverStep(b *testing.B) {
	for _, size := range seekBenchSizes {
		for _, history := range []int{0, seekBenchHistory} {
			b.Run(fmt.Sprintf("entities=%d/history=%d", size, history), func(b *testing.B) {
				driver := newSeekDriver(size, history)
				// Prime past the window so history=600 measures the steady
				// state (capture + ring shift), not the fill-up phase.
				primeTicks(driver, seekBenchHistory)

				b.ReportAllocs()
				b.ResetTimer()

				for range b.N {
					driver.Step()
				}
			})
		}
	}
}

func BenchmarkDriverBack(b *testing.B) {
	for _, size := range seekBenchSizes {
		b.Run(fmt.Sprintf("entities=%d", size), func(b *testing.B) {
			driver := newSeekDriver(size, seekBenchHistory)
			primeTicks(driver, seekBenchHistory)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				driver.Control(runtime.ControlCommand{Kind: runtime.ControlBack})
				driver.AdvanceTime(0)

				// Back consumed one snapshot; re-create it untimed so every
				// iteration measures the same single-snapshot restore.
				b.StopTimer()
				driver.Step()
				b.StartTimer()
			}
		})
	}
}

func BenchmarkDriverScrubToTick(b *testing.B) {
	for _, size := range seekBenchSizes {
		for _, timeline := range seekBenchTimelines {
			targets := []struct {
				name string
				tick uint64
			}{
				{"to-1", 1},
				{"to-half", uint64(timeline / 2)},
				// One tick before the head: the worst case of holding the
				// rewind key at the end of a long recorded run.
				{"to-last", uint64(timeline - 1)},
			}

			for _, target := range targets {
				name := fmt.Sprintf("entities=%d/timeline=%d/%s", size, timeline, target.name)
				b.Run(name, func(b *testing.B) {
					driver := newSeekDriver(size, seekBenchHistory)
					primeTicks(driver, timeline)

					b.ReportAllocs()
					b.ResetTimer()

					// ScrubToTick is non-destructive (the scrub log always
					// covers [0, maxTick]), so the same jump repeats safely.
					for range b.N {
						driver.Control(runtime.ControlCommand{
							Kind: runtime.ControlScrubToTick,
							Tick: target.tick,
						})
						driver.AdvanceTime(0)
					}
				})
			}
		}
	}
}

func BenchmarkReplay(b *testing.B) {
	for _, size := range seekBenchSizes {
		b.Run(fmt.Sprintf("entities=%d/ticks=%d", size, seekBenchHistory), func(b *testing.B) {
			recordGame := &seekBenchGame{entities: size}
			recorded := runtime.NewDriver(recordGame, seekBenchLogger())
			recorded.StartRecording()
			primeTicks(recorded, seekBenchHistory)
			rec := recorded.StopRecording()
			want := recorded.Checksum()

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				replayed := runtime.Replay(&seekBenchGame{entities: size}, seekBenchLogger(), rec)
				if got := replayed.Checksum(); got != want {
					b.Fatalf("replay checksum = %d, want %d (simulation is not deterministic)", got, want)
				}
			}
		})
	}
}

func newSeekDriver(entities, historyLen int) *runtime.Driver {
	game := &seekBenchGame{entities: entities, historyLen: historyLen}
	return runtime.NewDriver(game, seekBenchLogger())
}

// primeTicks advances the driver n ticks, toggling the bound key on a fixed
// period so the recorded input log resembles an interactive session. The
// pattern is deterministic: priming the same driver twice yields identical
// timelines and checksums.
func primeTicks(driver *runtime.Driver, n int) {
	for i := range n {
		if i%seekBenchInputPeriod == 0 {
			kind := input.EventButtonDown
			if (i/seekBenchInputPeriod)%2 == 1 {
				kind = input.EventButtonUp
			}
			driver.ApplyInput([]input.Event{{
				Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
				Kind:   kind,
			}})
		}
		driver.Step()
	}
}

// seekBenchLogger discards output: the driver logs on every reset, and
// ScrubToTick resets an internal replay driver per call — a real handler
// would pollute the measurement with I/O.
func seekBenchLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func seekBenchCeilSqrt(value int) int {
	result := 1
	for result*result < value {
		result++
	}
	return result
}
