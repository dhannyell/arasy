package fixedprobe

import (
	"fmt"
	"math"
	"testing"

	"github.com/dhannyell/fixed"
)

var (
	benchSinkInt   int
	benchSinkF     float64
	benchSinkRaw   int64
	benchProbeDtF  = 1.0 / 60.0
	benchProbeDtQ  = fixed.FromRatio(1, 60)
	benchProbeCell = fixed.FromInt(stepCellSize)
)

var probeSizes = []int{1000, 5000}

// BenchmarkProbeStep steps the full mirrored pipeline: integrate, wall
// bounce, broad phase, narrow phase, resolution.
func BenchmarkProbeStep(b *testing.B) {
	for _, size := range probeSizes {
		b.Run(fmt.Sprintf("float/%d", size), func(b *testing.B) {
			w := newFloatWorld(spreadFixture(size, 1, 0))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchSinkInt += w.step(benchProbeDtF, stepCellSize)
			}
		})
		b.Run(fmt.Sprintf("fixed/%d", size), func(b *testing.B) {
			w := newFixedWorld(spreadFixture(size, 1, 0))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchSinkInt += w.step(benchProbeDtQ, benchProbeCell)
			}
		})
	}
}

// BenchmarkProbeStepMixed adds the rect narrow-phase paths.
func BenchmarkProbeStepMixed(b *testing.B) {
	for _, size := range probeSizes {
		b.Run(fmt.Sprintf("float/%d", size), func(b *testing.B) {
			w := newFloatWorld(spreadFixture(size, 1, 2))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchSinkInt += w.step(benchProbeDtF, stepCellSize)
			}
		})
		b.Run(fmt.Sprintf("fixed/%d", size), func(b *testing.B) {
			w := newFixedWorld(spreadFixture(size, 1, 2))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchSinkInt += w.step(benchProbeDtQ, benchProbeCell)
			}
		})
	}
}

// BenchmarkProbeBroadPhase measures detection alone on a static world,
// matching the collision benchmark cell size.
func BenchmarkProbeBroadPhase(b *testing.B) {
	for _, size := range probeSizes {
		b.Run(fmt.Sprintf("float/%d", size), func(b *testing.B) {
			w := newFloatWorld(spreadFixture(size, 1, 0))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchSinkInt += len(w.detect(broadCellSize))
			}
		})
		b.Run(fmt.Sprintf("fixed/%d", size), func(b *testing.B) {
			w := newFixedWorld(spreadFixture(size, 1, 0))
			cell := fixed.FromInt(broadCellSize)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchSinkInt += len(w.detect(cell))
			}
		})
	}
}

// BenchmarkProbeResolve isolates contact resolution on a dense scenario.
// Positions and velocities are restored by memcpy each iteration; the copy
// cost is identical for both mirrors (8-byte elements on both sides).
func BenchmarkProbeResolve(b *testing.B) {
	const size = 1000

	b.Run("float", func(b *testing.B) {
		w := newFloatWorld(spreadFixture(size, 8, 0))
		contacts := append([]floatContact(nil), w.detect(broadCellSize)...)
		posX := append([]float64(nil), w.posX...)
		posY := append([]float64(nil), w.posY...)
		velX := append([]float64(nil), w.velX...)
		velY := append([]float64(nil), w.velY...)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			copy(w.posX, posX)
			copy(w.posY, posY)
			copy(w.velX, velX)
			copy(w.velY, velY)
			w.resolve(contacts)
		}
	})
	b.Run("fixed", func(b *testing.B) {
		w := newFixedWorld(spreadFixture(size, 8, 0))
		contacts := append([]fixedContact(nil), w.detect(fixed.FromInt(broadCellSize))...)
		posX := append([]fixed.Q(nil), w.posX...)
		posY := append([]fixed.Q(nil), w.posY...)
		velX := append([]fixed.Q(nil), w.velX...)
		velY := append([]fixed.Q(nil), w.velY...)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			copy(w.posX, posX)
			copy(w.posY, posY)
			copy(w.velX, velX)
			copy(w.velY, velY)
			w.resolve(contacts)
		}
	})
}

// BenchmarkProbeBallistics steps the composite flight kernel: drag, Magnus,
// gravity, ground bounce, roll, spin decay, then the shared pipeline.
func BenchmarkProbeBallistics(b *testing.B) {
	for _, size := range probeSizes {
		b.Run(fmt.Sprintf("float/%d", size), func(b *testing.B) {
			w := newFloatWorld(ballisticFixture(size))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchSinkInt += w.step(benchProbeDtF, stepCellSize)
			}
		})
		b.Run(fmt.Sprintf("fixed/%d", size), func(b *testing.B) {
			w := newFixedWorld(ballisticFixture(size))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				benchSinkInt += w.step(benchProbeDtQ, benchProbeCell)
			}
		})
	}
}

// BenchmarkScalarOps compares the raw scalar operations. Values rotate
// through a small table so the compiler cannot fold the loop.
func BenchmarkScalarOps(b *testing.B) {
	const mask = 255
	floats := make([]float64, mask+1)
	qs := make([]fixed.Q, mask+1)
	for i := range floats {
		q := fixed.FromRaw(int64(hash32(uint32(i), 9))<<8 | 1)
		qs[i] = q
		floats[i] = qf(q)
	}

	b.Run("mul/float", func(b *testing.B) {
		for i := range b.N {
			benchSinkF += floats[i&mask] * floats[(i+7)&mask]
		}
	})
	b.Run("mul/fixed", func(b *testing.B) {
		for i := range b.N {
			benchSinkRaw += qs[i&mask].Mul(qs[(i+7)&mask]).Raw()
		}
	})
	b.Run("div/float", func(b *testing.B) {
		for i := range b.N {
			benchSinkF += floats[i&mask] / floats[(i+7)&mask]
		}
	})
	b.Run("div/fixed", func(b *testing.B) {
		for i := range b.N {
			benchSinkRaw += qs[i&mask].Div(qs[(i+7)&mask]).Raw()
		}
	})
	b.Run("sqrt/float", func(b *testing.B) {
		for i := range b.N {
			benchSinkF += math.Sqrt(floats[i&mask])
		}
	})
	b.Run("sqrt/fixed", func(b *testing.B) {
		for i := range b.N {
			benchSinkRaw += qs[i&mask].Sqrt().Raw()
		}
	})
}
