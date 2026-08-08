// Package simbatch runs independent simulation scenarios across a bounded
// worker pool and returns the results in scenario order.
//
// Each run(i) call must be self-contained. It builds or clones its own
// world, and it reads no shared mutable state. Results land at index i of
// a pre-allocated slice, so the output never depends on scheduling.
package simbatch

import "sync"

// Run executes run(0..scenarios-1) on at most workers goroutines and
// returns one result per scenario, in scenario order. workers <= 1 runs
// every scenario in the calling goroutine. The sequential path is the
// reference: a batch with one worker and a batch with many workers must
// produce identical results.
func Run[R any](scenarios, workers int, run func(scenario int) R) []R {
	results := make([]R, scenarios)
	if workers <= 1 {
		for i := range results {
			results[i] = run(i)
		}
		return results
	}
	if workers > scenarios {
		workers = scenarios
	}
	// Each scenario index has a single writer, so results needs no lock.
	indices := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for i := range indices {
				results[i] = run(i)
			}
		}()
	}
	for i := range results {
		indices <- i
	}
	close(indices)
	wg.Wait()
	return results
}
