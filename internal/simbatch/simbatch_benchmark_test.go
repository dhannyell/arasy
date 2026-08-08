package simbatch_test

import (
	"runtime"
	"testing"

	"github.com/dhannyell/arasy/internal/simbatch"
)

// Both benchmarks run the same batch, so their ratio is the measured
// parallel speedup on the host. The sequential result is the regression
// baseline for batch throughput; the ratio is recorded, not asserted,
// because CI hardware is not stable enough for a speedup gate.
const (
	benchScenarios = 64
	benchTicks     = 300
)

func benchmarkBatch(b *testing.B, workers int) {
	b.ReportAllocs()
	for b.Loop() {
		simbatch.Run(benchScenarios, workers, func(i int) simbatch.ArenaResult {
			return simbatch.Arena(i, benchScenarios, benchTicks)
		})
	}
}

func BenchmarkSimbatchSequential(b *testing.B) { benchmarkBatch(b, 1) }

func BenchmarkSimbatchParallel(b *testing.B) { benchmarkBatch(b, runtime.NumCPU()) }
