package scene_test

import (
	"fmt"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
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

	// Plan 153 budget case: v1 assumes at most 16 declared lights, and this
	// is that worst case over every sprite — all layers lit, every light
	// with a live radius. Compare against sprites/1000 above.
	for _, alpha := range []float64{0, 0.5} {
		b.Run(fmt.Sprintf("sprites/1000/lights/16/alpha-%g", alpha), func(b *testing.B) {
			world := benchmarkRenderWorld(1000)
			world.SetLitLayers(0, 6)
			for i := range 16 {
				torch := world.Spawn()
				world.SetTransform(torch, scene.Transform{
					Position: math2d.Vec2{X: float64(i%4)*12 + 5, Y: float64(i/4)*5 + 2},
				})
				world.SetPointLight(torch, scene.PointLight{
					Color:   render.RGBA(1, 0.9, 0.7, 1),
					Radius:  10,
					Falloff: 1,
				})
			}
			commands := make([]render.Command, 0, 1000)

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				commands = world.RenderCommandsInto(commands[:0], alpha)
				benchmarkRenderCommandSink = commands
			}
		})
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
				BaseSprite: scene.BaseSprite{
					Layer: i % 7,
					Color: render.RGBA(0, 0, 1, 1),
				},
				Texture: render.TextureID(i%5 + 1),
				Size:    math2d.Vec2{X: 2, Y: 2},
			})
		}
	}
	world.SyncRenderState()
	return world
}
