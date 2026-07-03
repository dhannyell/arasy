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
