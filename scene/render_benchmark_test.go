package scene_test

import (
	"fmt"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

var benchmarkRenderCommandSink []render.Command

func BenchmarkRenderCommands(b *testing.B) {
	for _, count := range []int{100, 1000} {
		for _, alpha := range []float64{0, 0.5} {
			b.Run(fmt.Sprintf("sprites/%d/alpha-%g", count, alpha), func(b *testing.B) {
				world := benchmarkRenderWorld(count)
				commands := make([]render.Command, 0, count)

				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					commands = world.RenderCommandsInto(commands[:0], alpha)
					benchmarkRenderCommandSink = commands
				}
			})
		}
	}
}

func benchmarkRenderWorld(count int) *scene.World {
	world := scene.NewWorld()
	for i := range count {
		id := world.Spawn()
		world.SetTransform(id, scene.Transform{
			Position: math2d.Vec2{X: float64(i % 50), Y: float64(i / 50)},
			Rotation: float64(i%8) * 0.1,
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})

		switch i % 3 {
		case 0:
			world.SetCircleSprite(id, scene.CircleSprite{
				BaseSprite: scene.BaseSprite{Layer: i % 7, Color: render.RGBA(1, 0, 0, 1)},
				Radius:     1,
			})
		case 1:
			world.SetRectSprite(id, scene.RectSprite{
				BaseSprite: scene.BaseSprite{Layer: i % 7, Color: render.RGBA(0, 1, 0, 1)},
				Rect:       math2d.NewRect(math2d.Vec2{X: -1, Y: -1}, math2d.Vec2{X: 2, Y: 2}),
			})
		case 2:
			world.SetImageSprite(id, scene.ImageSprite{
				Layer:   i % 7,
				Texture: render.TextureID(i%5 + 1),
				Size:    math2d.Vec2{X: 2, Y: 2},
				Tint:    render.RGBA(0, 0, 1, 1),
			})
		}
	}
	world.SyncRenderState()
	return world
}
