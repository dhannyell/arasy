package simbatch_test

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/internal/simbatch"
)

// TestArenaDeterministic pins the fixture contract: a scenario is a pure
// function of its index, so the same scenario must reproduce the same
// checksum, and different scenarios must build different worlds.
func TestArenaDeterministic(t *testing.T) {
	first := simbatch.Arena(3, 8, 120)
	second := simbatch.Arena(3, 8, 120)
	if first != second {
		t.Fatalf("same scenario diverged: %+v vs %+v", first, second)
	}
	if math.IsNaN(first.MeanSpeed) || math.IsInf(first.MeanSpeed, 0) {
		t.Fatalf("mean speed is not finite: %v", first.MeanSpeed)
	}
	other := simbatch.Arena(7, 8, 120)
	if other.Checksum == first.Checksum {
		t.Fatal("distinct scenarios produced the same checksum")
	}
}

const (
	batchScenarios = 64
	batchTicks     = 300
)

func runArenaBatch(workers int) []simbatch.ArenaResult {
	return simbatch.Run(batchScenarios, workers, func(i int) simbatch.ArenaResult {
		return simbatch.Arena(i, batchScenarios, batchTicks)
	})
}

// TestBatchOneVsManyWorkers is the plan's central guard: one worker and
// eight workers must produce bit-identical batches. A divergence here is
// an engine determinism finding (shared mutable state on the simulation
// path), not a harness bug.
func TestBatchOneVsManyWorkers(t *testing.T) {
	sequential := runArenaBatch(1)
	parallel := runArenaBatch(8)
	for i := range sequential {
		if sequential[i] != parallel[i] {
			t.Fatalf("scenario %d diverged: workers=1 %+v, workers=8 %+v",
				i, sequential[i], parallel[i])
		}
	}
}

// TestBatchRepetition guards against state that leaks between batches:
// the same parallel batch twice must be identical.
func TestBatchRepetition(t *testing.T) {
	first := runArenaBatch(8)
	second := runArenaBatch(8)
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("scenario %d diverged between repetitions: %+v vs %+v",
				i, first[i], second[i])
		}
	}
}
