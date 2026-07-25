package runtime_test

// Retained-batch benchmarks measure the runtime-side publish/drain path:
// BatchPort.UpdateSprites/UpdateText (copy + pending-map insert) and
// Driver.ConsumeFrameUpdates (sort + drain), independent of any host
// renderer. This is the "lookup steady state" and "replace/delete" cost the
// driver itself pays every frame, whether or not a game touches batches.

import (
	"strconv"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

func benchBatchInstances(n int) []render.SpriteInstance {
	instances := make([]render.SpriteInstance, n)
	for i := range n {
		instances[i] = render.SpriteInstance{
			Texture: 1,
			Dest:    math2d.NewRect(math2d.Vec2{X: float64(i), Y: 0}, math2d.Vec2{X: 8, Y: 8}),
		}
	}
	return instances
}

// BenchmarkBatchPublishAndConsumeReplace measures publishing n distinct
// sprite batch IDs in one frame (each a BatchReplace with 32 instances) and
// draining them: the steady-state cost of a game that republishes many small
// batches every frame.
func BenchmarkBatchPublishAndConsumeReplace(b *testing.B) {
	instances := benchBatchInstances(32)
	for _, n := range []int{100, 1000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			game := &batchGame{}
			driver := runtime.NewDriver(game, testLogger())
			game.renderFn = func(ctx *runtime.RenderContext) {
				for id := 0; id < n; id++ {
					ctx.Batches.UpdateSprites(render.SpriteBatchID(id), instances)
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				driver.Frame(0)
				_ = driver.ConsumeFrameUpdates()
			}
		})
	}
}

// BenchmarkBatchPublishAndConsumeSteadyState measures the drain cost of a
// frame that republishes nothing: Render still runs, but no batch call means
// ConsumeFrameUpdates has zero pending updates. This is the common case for
// stable content — sprites/labels published once in Init or Update and never
// touched again in Render.
func BenchmarkBatchPublishAndConsumeSteadyState(b *testing.B) {
	game := &batchGame{}
	driver := runtime.NewDriver(game, testLogger())

	b.ReportAllocs()
	for b.Loop() {
		driver.Frame(0)
		_ = driver.ConsumeFrameUpdates()
	}
}

// BenchmarkBatchDeleteCollapsesReplace measures publishing then immediately
// deleting n batch IDs in the same frame: the pending map still collapses to
// one BatchDelete entry per ID, so this isolates the map-write cost from the
// content copy BenchmarkBatchPublishAndConsumeReplace also pays.
func BenchmarkBatchDeleteCollapsesReplace(b *testing.B) {
	instances := benchBatchInstances(32)
	for _, n := range []int{100, 1000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			game := &batchGame{}
			driver := runtime.NewDriver(game, testLogger())
			game.renderFn = func(ctx *runtime.RenderContext) {
				for id := 0; id < n; id++ {
					ctx.Batches.UpdateSprites(render.SpriteBatchID(id), instances)
					ctx.Batches.DeleteSprites(render.SpriteBatchID(id))
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				driver.Frame(0)
				_ = driver.ConsumeFrameUpdates()
			}
		})
	}
}
