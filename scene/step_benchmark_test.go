package scene_test

import (
	"fmt"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
)

func BenchmarkWorldStep(b *testing.B) {
	for _, size := range benchmarkGridSizes {
		b.Run(fmt.Sprintf("step/%d/spread", size), func(b *testing.B) {
			world := benchmarkWorldSpread(size, 1000, 1000, 1)
			cfg := scene.StepConfig{
				Bounds: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1000, Y: 1000}),
			}

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				world.Step(1.0/60.0, cfg)
			}
		})
	}

	for _, size := range benchmarkGridSizes {
		b.Run(fmt.Sprintf("step/%d/mixed", size), func(b *testing.B) {
			world := benchmarkWorldSpreadMixed(size, 1000, 1000, 1, 2)
			cfg := scene.StepConfig{
				Bounds: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1000, Y: 1000}),
			}

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				world.Step(1.0/60.0, cfg)
			}
		})
	}
}

func BenchmarkWorldClone(b *testing.B) {
	for _, size := range benchmarkGridSizes {
		b.Run(fmt.Sprintf("clone/%d/spread", size), func(b *testing.B) {
			world := benchmarkWorldSpread(size, 1000, 1000, 1)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = world.Clone()
			}
		})
	}

	for _, size := range benchmarkGridSizes {
		b.Run(fmt.Sprintf("clone/%d/mixed", size), func(b *testing.B) {
			world := benchmarkWorldSpreadMixed(size, 1000, 1000, 1, 2)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = world.Clone()
			}
		})
	}
}

func BenchmarkWorldChurn(b *testing.B) {
	cases := []struct {
		created int
		live    int
	}{
		{created: 100_000, live: 1_000},
		{created: 100_000, live: 5_000},
		{created: 1_000_000, live: 1_000},
		{created: 1_000_000, live: 5_000},
	}

	for _, tc := range cases {
		world := benchmarkWorldChurn(tc.created, tc.live)
		maxID := tc.created
		sparseBytes := 5 * maxID * 4
		name := fmt.Sprintf("created=%d/live=%d/maxID=%d/sparseBytes=%d", tc.created, tc.live, maxID, sparseBytes)

		b.Run(name+"/clone", func(b *testing.B) {
			b.ReportAllocs()
			b.ReportMetric(float64(maxID), "maxID")
			b.ReportMetric(float64(tc.live), "live")
			b.ReportMetric(float64(sparseBytes), "sparse-bytes")
			b.ResetTimer()

			for range b.N {
				_ = world.Clone()
			}
		})

		b.Run(name+"/checksum", func(b *testing.B) {
			b.ReportAllocs()
			b.ReportMetric(float64(maxID), "maxID")
			b.ReportMetric(float64(tc.live), "live")
			b.ReportMetric(float64(sparseBytes), "sparse-bytes")
			b.ResetTimer()

			for range b.N {
				_ = world.Checksum()
			}
		})

		b.Run(name+"/step", func(b *testing.B) {
			cfg := scene.StepConfig{
				Bounds: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1000, Y: 1000}),
			}
			b.ReportAllocs()
			b.ReportMetric(float64(maxID), "maxID")
			b.ReportMetric(float64(tc.live), "live")
			b.ReportMetric(float64(sparseBytes), "sparse-bytes")
			b.ResetTimer()

			for range b.N {
				world.Step(1.0/60.0, cfg)
			}
		})

		// /clone-restore mirrors the seek/rewind path: a fresh Clone followed
		// by the first random access, which rebuilds each populated store's
		// sparse index (O(maxID)). Plain /clone is lazy and never pays this, so
		// the gap between /clone and /clone-restore is the hidden rebuild cost.
		b.Run(name+"/clone-restore", func(b *testing.B) {
			// The last spawned id is always live and carries every component,
			// so one lookup per store forces every sparse rebuild.
			liveID := scene.EntityID(tc.created)
			b.ReportAllocs()
			b.ReportMetric(float64(maxID), "maxID")
			b.ReportMetric(float64(tc.live), "live")
			b.ReportMetric(float64(sparseBytes), "sparse-bytes")
			b.ResetTimer()

			for range b.N {
				c := world.Clone()
				_, ok := c.Transform(liveID)
				_, ok2 := c.Velocity(liveID)
				_, ok3 := c.CircleCollider(liveID)
				_, ok4 := c.CircleSprite(liveID)
				_, ok5 := c.Body(liveID)
				churnRestoreSink = ok && ok2 && ok3 && ok4 && ok5
			}
		})
	}
}

// churnRestoreSink keeps the /clone-restore lookups from being optimized away.
var churnRestoreSink bool

func benchmarkWorldChurn(created, live int) *scene.World {
	world := scene.NewWorld()
	liveStart := created - live
	for i := 0; i < created; i++ {
		id := world.Spawn()
		setChurnComponents(world, id, i)
		if i < liveStart {
			world.Despawn(id)
		}
	}

	return world
}

func setChurnComponents(world *scene.World, id scene.EntityID, i int) {
	x := float64(i % 1000)
	y := float64(i / 1000)
	world.SetTransform(id, scene.Transform{
		Position: math2d.Vec2{X: x, Y: y},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetVelocity(id, scene.Velocity{
		Linear: math2d.Vec2{X: 1, Y: 0.5},
	})
	world.SetCircleCollider(id, scene.CircleCollider{Radius: 1})
	world.SetCircleSprite(id, scene.CircleSprite{Radius: 1})
	world.SetBody(id, scene.Body{InvMass: 1, Restitution: 0.5})
}

// BenchmarkWorldChurnMidRemoval exercises the O(N) path the tail-only churn in
// BenchmarkWorldChurn never hits: each iteration despawns an entity in the
// middle of the live set and respawns one at the tail. The mid despawn forces
// slices.Delete plus a sparse reindex over half the dense array in every
// populated store, so per-op cost is expected to scale with the live count.
func BenchmarkWorldChurnMidRemoval(b *testing.B) {
	for _, live := range []int{1000, 5000} {
		b.Run(fmt.Sprintf("live=%d/mid-remove-respawn", live), func(b *testing.B) {
			world := scene.NewWorld()
			ids := make([]scene.EntityID, 0, live)
			for i := 0; i < live; i++ {
				id := world.Spawn()
				setChurnComponents(world, id, i)
				ids = append(ids, id)
			}

			b.ReportAllocs()
			b.ReportMetric(float64(live), "live")
			b.ResetTimer()

			for n := range b.N {
				// ids stays ascending: removing a middle entry keeps the rest
				// ordered, and the respawned id is monotonic so it appends last.
				midIdx := len(ids) / 2
				world.Despawn(ids[midIdx])

				newID := world.Spawn()
				setChurnComponents(world, newID, live+n)

				ids = append(ids[:midIdx], ids[midIdx+1:]...)
				ids = append(ids, newID)
			}
		})
	}
}
