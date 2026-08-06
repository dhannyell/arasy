package scene_test

import (
	"fmt"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// BenchmarkWorldDespawn characterizes bulk removal before DespawnBatch: the
// front-first case repeatedly shifts every following dense entry, while
// back-first removal only pops the tail of each ordered store.
func BenchmarkWorldDespawn(b *testing.B) {
	for _, count := range []int{1_000, 10_000, 50_000} {
		b.Run(fmt.Sprintf("sequential-front/%d", count), func(b *testing.B) {
			benchmarkSequentialDespawn(b, count, false)
		})
		b.Run(fmt.Sprintf("sequential-back/%d", count), func(b *testing.B) {
			benchmarkSequentialDespawn(b, count, true)
		})
		b.Run(fmt.Sprintf("batch/%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				b.StopTimer()
				world, ids := benchmarkDespawnWorld(count)
				b.StartTimer()
				world.DespawnBatch(ids)
			}
		})
	}
}

func benchmarkSequentialDespawn(b *testing.B, count int, reverse bool) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		world, ids := benchmarkDespawnWorld(count)
		b.StartTimer()
		if reverse {
			for i := len(ids) - 1; i >= 0; i-- {
				world.Despawn(ids[i])
			}
			continue
		}
		for _, id := range ids {
			world.Despawn(id)
		}
	}
}

func benchmarkDespawnWorld(count int) (*scene.World, []scene.EntityID) {
	world := scene.NewWorld()
	ids := make([]scene.EntityID, 0, count)
	for i := range count {
		id := world.Spawn()
		world.SetTransform(id, scene.Transform{
			Position: math2d.Vec2{X: float64(i), Y: float64(i % 31)},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		world.SetImageSprite(id, scene.ImageSprite{
			BaseSprite: scene.BaseSprite{
				Color: render.RGBA(1, 1, 1, 1),
			},
			Texture: 1,
			Size:    math2d.Vec2{X: 16, Y: 16},
		})
		ids = append(ids, id)
	}
	return world, ids
}
