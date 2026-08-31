package fixedprobe

import (
	"fmt"
	"math"
	"math/bits"
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

// BenchmarkSpinDecayDivider pays one Div per entity against one prepared
// reciprocal per tick, on the invariant spin decay denominator. Each op
// covers 1000 entities.
func BenchmarkSpinDecayDivider(b *testing.B) {
	denom := fixed.One().Add(qSpinDecay.Mul(benchProbeDtQ))
	omegas := make([]fixed.Q, 1000)
	state := uint64(3)
	for i := range omegas {
		// Spins in the consumer's magnitude range, both signs.
		omegas[i] = fixed.FromRaw(int64(probeRand64(&state)) >> 20)
	}
	b.Run("div", func(b *testing.B) {
		var acc int64
		for range b.N {
			for _, o := range omegas {
				acc += o.Div(denom).Raw()
			}
		}
		benchSinkRaw = acc
	})
	b.Run("prepared", func(b *testing.B) {
		var acc int64
		for range b.N {
			p := prepareDivider(denom)
			for _, o := range omegas {
				acc += p.div(o).Raw()
			}
		}
		benchSinkRaw = acc
	})
}

// BenchmarkContactSolverDivider mimics the solver's kNormal pattern: one
// invariant divisor per contact, five dependent divisions by it (four
// substeps plus relaxation). The div side pays Q.Div per use, as under
// D-006. The prepared side pays one preparation per contact plus the
// inlined Möller–Granlund use, restricted to the solver domain: positive
// divisor, no saturation reachable. Each op covers one contact.
func BenchmarkContactSolverDivider(b *testing.B) {
	const contacts = 256
	one := fixed.One()
	denoms := make([]fixed.Q, contacts)
	state := uint64(11)
	for i := range denoms {
		// kNormal-like magnitudes: [1.25, 4) with live fraction bits.
		denoms[i] = fixed.FromRaw(int64(5<<30 + probeRand64(&state)%(11<<30)))
	}

	// The inlined use block must match Q.Div bit for bit before it is
	// allowed to produce a number.
	x := fixed.FromInt(2)
	xr := uint64(x.Raw())
	for i := range 4 * contacts {
		d := denoms[i&(contacts-1)]
		p := prepareDivider(d)
		for range 5 {
			x = x.Div(d).Add(one)

			hi, lo := xr>>32, xr<<32
			u1 := hi<<p.s | lo>>(64-p.s)
			u0 := lo << p.s
			q1, q0 := bits.Mul64(p.v, u1)
			var c uint64
			q0, c = bits.Add64(q0, u0, 0)
			q1, _ = bits.Add64(q1, u1, c)
			q1++
			r := u0 - q1*p.dn
			if r > q0 {
				q1--
				r += p.dn
			}
			if r >= p.dn {
				q1++
			}
			xr = uint64(fixed.FromRaw(int64(q1)).Add(one).Raw())
		}
		if uint64(x.Raw()) != xr {
			b.Fatalf("inline reciprocal diverged from Q.Div at contact %d", i)
		}
	}
	// The branchless correction variant must agree too.
	x = fixed.FromInt(2)
	xr = uint64(x.Raw())
	for i := range 4 * contacts {
		d := denoms[i&(contacts-1)]
		p := prepareDivider(d)
		for range 5 {
			x = x.Div(d).Add(one)

			hi, lo := xr>>32, xr<<32
			u1 := hi<<p.s | lo>>(64-p.s)
			u0 := lo << p.s
			q1, q0 := bits.Mul64(p.v, u1)
			var c uint64
			q0, c = bits.Add64(q0, u0, 0)
			q1, _ = bits.Add64(q1, u1, c)
			q1++
			r := u0 - q1*p.dn
			_, b1 := bits.Sub64(q0, r, 0)
			q1 -= b1
			r += p.dn & -b1
			_, b2 := bits.Sub64(r, p.dn, 0)
			q1 += 1 - b2
			xr = uint64(fixed.FromRaw(int64(q1)).Add(one).Raw())
		}
		if uint64(x.Raw()) != xr {
			b.Fatalf("branchless reciprocal diverged from Q.Div at contact %d", i)
		}
	}

	b.Run("div", func(b *testing.B) {
		x := fixed.FromInt(2)
		i := 0
		for range b.N {
			d := denoms[i&(contacts-1)]
			for range 5 {
				x = x.Div(d).Add(one)
			}
			i++
		}
		benchSinkRaw = x.Raw()
	})
	b.Run("prepared", func(b *testing.B) {
		xr := uint64(fixed.FromInt(2).Raw())
		i := 0
		for range b.N {
			p := prepareDivider(denoms[i&(contacts-1)])
			for range 5 {
				hi, lo := xr>>32, xr<<32
				u1 := hi<<p.s | lo>>(64-p.s)
				u0 := lo << p.s
				q1, q0 := bits.Mul64(p.v, u1)
				var c uint64
				q0, c = bits.Add64(q0, u0, 0)
				q1, _ = bits.Add64(q1, u1, c)
				q1++
				r := u0 - q1*p.dn
				if r > q0 {
					q1--
					r += p.dn
				}
				if r >= p.dn {
					q1++
				}
				xr = uint64(fixed.FromRaw(int64(q1)).Add(one).Raw())
			}
			i++
		}
		benchSinkRaw = int64(xr)
	})
	b.Run("prepare-only", func(b *testing.B) {
		// Isolates the setup: the reciprocal is a worst-case DIVQ
		// (64-bit quotient), so it is not one cheap division.
		var acc uint64
		i := 0
		for range b.N {
			p := prepareDivider(denoms[i&(contacts-1)])
			acc += p.v
			i++
		}
		benchSinkRaw = int64(acc)
	})
	b.Run("prepared-branchless", func(b *testing.B) {
		xr := uint64(fixed.FromInt(2).Raw())
		i := 0
		for range b.N {
			p := prepareDivider(denoms[i&(contacts-1)])
			for range 5 {
				hi, lo := xr>>32, xr<<32
				u1 := hi<<p.s | lo>>(64-p.s)
				u0 := lo << p.s
				q1, q0 := bits.Mul64(p.v, u1)
				var c uint64
				q0, c = bits.Add64(q0, u0, 0)
				q1, _ = bits.Add64(q1, u1, c)
				q1++
				r := u0 - q1*p.dn
				_, b1 := bits.Sub64(q0, r, 0)
				q1 -= b1
				r += p.dn & -b1
				_, b2 := bits.Sub64(r, p.dn, 0)
				q1 += 1 - b2
				xr = uint64(fixed.FromRaw(int64(q1)).Add(one).Raw())
			}
			i++
		}
		benchSinkRaw = int64(xr)
	})
}
