package particle

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
)

const emitterBenchmarkCapacity = 65536

type emitterBenchmarkProfile struct {
	name   string
	live   int
	sparse bool
}

var emitterBenchmarkProfiles = []emitterBenchmarkProfile{
	{name: "empty_65536", live: 0},
	{name: "dense_256_of_65536", live: 256},
	{name: "sparse_256_of_65536", live: 256, sparse: true},
	{name: "full_65536", live: emitterBenchmarkCapacity},
}

func BenchmarkEmitterStep(b *testing.B) {
	for _, profile := range emitterBenchmarkProfiles {
		b.Run(profile.name, func(b *testing.B) {
			emitter := newEmitterBenchmark(b, profile)
			position := math2d.Vec2{}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				emitter.Step(position, 0, 0, 0, 1.0/60.0, emitterBenchmarkCapacity)
			}
		})
	}
}

func BenchmarkEmitterAlive(b *testing.B) {
	for _, profile := range emitterBenchmarkProfiles {
		b.Run(profile.name, func(b *testing.B) {
			emitter := newEmitterBenchmark(b, profile)
			dst := make([]LiveParticle, 0, profile.live)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				dst = emitter.Alive(dst[:0])
			}
		})
	}
}

func newEmitterBenchmark(b *testing.B, profile emitterBenchmarkProfile) *Emitter {
	b.Helper()

	decl := EffectDecl{
		Capacity:   emitterBenchmarkCapacity,
		Lifetime:   Range{Min: math.MaxFloat64, Max: math.MaxFloat64},
		Speed:      Range{},
		Drag:       1,
		SizeStart:  Range{Min: 1, Max: 1},
		SizeEnd:    Range{Min: 1, Max: 1},
		ColorStart: [4]float64{1, 1, 1, 1},
		ColorEnd:   [4]float64{1, 1, 1, 1},
	}
	emitter := NewEmitter(decl, 1)

	switch {
	case profile.live == 0:
		// NewEmitter already represents the empty profile.
	case profile.live == emitterBenchmarkCapacity:
		spawnBenchmarkParticles(emitter, emitterBenchmarkCapacity)
	case profile.sparse:
		spawnBenchmarkParticles(emitter, emitterBenchmarkCapacity)
		for i := range emitter.pool {
			emitter.pool[i].Age = 0
			emitter.pool[i].Life = 0.5
		}
		stride := emitterBenchmarkCapacity / profile.live
		for i := range profile.live {
			emitter.pool[i*stride].Life = math.MaxFloat64
		}
		emitter.Step(math2d.Vec2{}, 0, 0, 0, 1, emitterBenchmarkCapacity)
	default:
		spawnBenchmarkParticles(emitter, profile.live)
	}

	if got := len(emitter.pool) - len(emitter.free); got != profile.live {
		b.Fatalf("benchmark profile %s has %d live particles, want %d", profile.name, got, profile.live)
	}
	return emitter
}

func spawnBenchmarkParticles(emitter *Emitter, count int) {
	emitter.Decl.EmitRate = float64(count)
	emitter.Step(math2d.Vec2{}, 0, 1, 0, 1, emitterBenchmarkCapacity)
	emitter.Decl.EmitRate = 0
}
