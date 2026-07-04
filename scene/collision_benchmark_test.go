package scene_test

import (
	"fmt"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
)

var (
	benchmarkSpreadSizes = []int{10, 25, 50, 75, 100, 1000, 5000}
	benchmarkGridSizes   = []int{10, 25, 50, 75, 100, 1000}
)

func BenchmarkCollisionOverlapsAllPairs(b *testing.B) {
	for _, size := range benchmarkSpreadSizes {
		b.Run(fmt.Sprintf("narrow/%d/spread", size), func(b *testing.B) {
			world := benchmarkWorldSpread(size, 1000, 1000, 1)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = world.DetectCircleOverlaps()
			}
		})
	}
}

func BenchmarkCollisionOverlapsGrid(b *testing.B) {
	for _, size := range benchmarkSpreadSizes {
		b.Run(fmt.Sprintf("grid/%d/spread", size), func(b *testing.B) {
			world := benchmarkWorldSpread(size, 1000, 1000, 1)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = world.DetectCircleOverlapsGrid(16)
			}
		})
	}
}

func BenchmarkCollisionOverlapsGridDense(b *testing.B) {
	for _, size := range benchmarkGridSizes {
		b.Run(fmt.Sprintf("grid/%d/dense", size), func(b *testing.B) {
			world := benchmarkWorldSpread(size, 64, 64, 1)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = world.DetectCircleOverlapsGrid(16)
			}
		})
	}
}

func BenchmarkCollisionOverlapsGridLargeColliders(b *testing.B) {
	for _, size := range benchmarkGridSizes {
		b.Run(fmt.Sprintf("grid/%d/large-colliders", size), func(b *testing.B) {
			world := benchmarkWorldSpread(size, 1000, 1000, 12)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = world.DetectCircleOverlapsGrid(16)
			}
		})
	}
}

// BenchmarkCollisionOverlapsGridMixed measures the grid broad-phase (the
// only detection path that sees rects) over a world where half the entities
// carry a RectCollider instead of a CircleCollider.
func BenchmarkCollisionOverlapsGridMixed(b *testing.B) {
	for _, size := range benchmarkSpreadSizes {
		b.Run(fmt.Sprintf("grid/%d/mixed", size), func(b *testing.B) {
			world := benchmarkWorldSpreadMixed(size, 1000, 1000, 1, 2)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = world.DetectCircleOverlapsGrid(16)
			}
		})
	}
}

// BenchmarkCollisionOverlapsGridBreakoutProfile profiles a "mostly static
// world" shape representative of the breakout demo: a fixed 8x9 grid of
// static rects (bricks) plus one dynamic circle (the ball).
func BenchmarkCollisionOverlapsGridBreakoutProfile(b *testing.B) {
	world := scene.NewWorld()

	const cols, rows = 8, 9
	const brickWidth, brickHeight = 10.0, 6.0

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			x := float64(col)*brickWidth + brickWidth*0.5
			y := float64(row)*brickHeight + brickHeight*0.5

			brick := world.Spawn()
			world.SetTransform(brick, scene.Transform{
				Position: math2d.Vec2{X: x, Y: y},
				Scale:    math2d.Vec2{X: 1, Y: 1},
			})
			world.SetRectCollider(brick, scene.RectCollider{
				HalfExtends: math2d.Vec2{X: brickWidth / 2, Y: brickHeight / 2},
			})
			world.SetBody(brick, scene.Body{
				InvMass:     0,
				Restitution: 1,
			})
		}
	}

	ball := world.Spawn()
	world.SetTransform(ball, scene.Transform{
		Position: math2d.Vec2{X: float64(cols) * brickWidth / 2, Y: float64(rows) * brickHeight},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(ball, scene.CircleCollider{Radius: 1})
	world.SetBody(ball, scene.Body{InvMass: 1, Restitution: 1})

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = world.DetectCircleOverlapsGrid(16)
	}
}

func BenchmarkCollisionResolveGrid(b *testing.B) {
	for _, size := range benchmarkGridSizes {
		b.Run(fmt.Sprintf("resolve-grid/%d/spread", size), func(b *testing.B) {
			world := benchmarkWorldSpread(size, 1000, 1000, 1)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				clone := world.Clone()
				clone.ResolveCircleOverlapsGrid(16)
			}
		})
	}
}

func benchmarkWorldSpread(count int, width, height, radius float64) *scene.World {
	world := scene.NewWorld()

	cols := ceilSqrt(count)
	spacingX := width / float64(cols)
	spacingY := height / float64(cols)

	for i := 0; i < count; i++ {
		x := float64(i%cols)*spacingX + spacingX*0.5
		y := float64(i/cols)*spacingY + spacingY*0.5

		entity := world.Spawn()
		world.SetTransform(entity, scene.Transform{
			Position: math2d.Vec2{X: x, Y: y},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		world.SetCircleCollider(entity, scene.CircleCollider{
			Radius: radius,
		})
		world.SetBody(entity, scene.Body{
			InvMass:     1,
			Restitution: 1,
		})
	}

	return world
}

// benchmarkWorldSpreadMixed places entities on the same deterministic grid as
// benchmarkWorldSpread, but gives every rectEvery-th entity a RectCollider
// (half extents = radius) instead of a CircleCollider. rectEvery <= 0 means
// no entity gets a RectCollider (identical to benchmarkWorldSpread).
func benchmarkWorldSpreadMixed(count int, width, height, radius float64, rectEvery int) *scene.World {
	world := scene.NewWorld()

	cols := ceilSqrt(count)
	spacingX := width / float64(cols)
	spacingY := height / float64(cols)

	for i := 0; i < count; i++ {
		x := float64(i%cols)*spacingX + spacingX*0.5
		y := float64(i/cols)*spacingY + spacingY*0.5

		entity := world.Spawn()
		world.SetTransform(entity, scene.Transform{
			Position: math2d.Vec2{X: x, Y: y},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		if rectEvery > 0 && i%rectEvery == 0 {
			world.SetRectCollider(entity, scene.RectCollider{
				HalfExtends: math2d.Vec2{X: radius, Y: radius},
			})
		} else {
			world.SetCircleCollider(entity, scene.CircleCollider{
				Radius: radius,
			})
		}
		world.SetBody(entity, scene.Body{
			InvMass:     1,
			Restitution: 1,
		})
	}

	return world
}

func ceilSqrt(value int) int {
	result := 1
	for result*result < value {
		result++
	}
	return result
}
