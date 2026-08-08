package simbatch_test

import (
	"testing"

	"github.com/dhannyell/arasy/internal/simbatch"
)

// TestRun covers the base contract: a parallel batch returns one result
// per scenario, ordered by scenario index.
func TestRun(t *testing.T) {
	got := simbatch.Run(100, 4, func(i int) int { return i * i })
	if len(got) != 100 {
		t.Fatalf("len(results) = %d, want 100", len(got))
	}
	for i, v := range got {
		if v != i*i {
			t.Fatalf("results[%d] = %d, want %d", i, v, i*i)
		}
	}
}

// TestRunSequentialReferencePath pins the workers <= 1 contract: scenarios
// run in ascending order in the calling goroutine. The 1-vs-N determinism
// guard compares against this path. The unsynchronized append is safe only
// under that contract, so the race detector also verifies it.
func TestRunSequentialReferencePath(t *testing.T) {
	var calls []int
	simbatch.Run(5, 1, func(i int) int {
		calls = append(calls, i)
		return i
	})
	if len(calls) != 5 {
		t.Fatalf("ran %d scenarios, want 5", len(calls))
	}
	for i, v := range calls {
		if v != i {
			t.Fatalf("call order = %v, want ascending from 0", calls)
		}
	}
}

// TestRunClampsWorkersToScenarios keeps a small batch correct when the
// pool is larger than the work.
func TestRunClampsWorkersToScenarios(t *testing.T) {
	got := simbatch.Run(3, 16, func(i int) int { return i + 1 })
	want := []int{1, 2, 3}
	for i, v := range got {
		if v != want[i] {
			t.Fatalf("results = %v, want %v", got, want)
		}
	}
}

// TestRunEmptyBatch returns an empty result set without calling run.
func TestRunEmptyBatch(t *testing.T) {
	called := false
	got := simbatch.Run(0, 8, func(i int) int {
		called = true
		return i
	})
	if len(got) != 0 {
		t.Fatalf("len(results) = %d, want 0", len(got))
	}
	if called {
		t.Fatal("run was called for an empty batch")
	}
}
