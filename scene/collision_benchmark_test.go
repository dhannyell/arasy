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

func BenchmarkDetectCircleOverlaps(b *testing.B) {
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

func BenchmarkDetectCircleOverlapsGrid(b *testing.B) {
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

func BenchmarkDetectCircleOverlapsGridDense(b *testing.B) {
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

func BenchmarkDetectCircleOverlapsGridLargeColliders(b *testing.B) {
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

func BenchmarkResolveCircleOverlapsGrid(b *testing.B) {
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
		y := float64(i%cols)*spacingY + spacingY*0.5

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

func ceilSqrt(value int) int {
	result := 1
	for result*result < value {
		result++
	}
	return result
}
