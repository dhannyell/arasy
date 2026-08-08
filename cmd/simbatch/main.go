// Command simbatch runs a batch of bounce-arena scenarios across a worker
// pool and prints the outcome distribution. It is the reference consumer of
// internal/simbatch: thousands of independent worlds sweeping ball
// restitution, one goroutine per world, results in scenario order.
//
// The scenario index is the seed. Use -only to rerun one scenario and
// print every parameter it derives from that index.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/dhannyell/arasy/internal/simbatch"
)

const distributionBands = 8

func main() {
	n := flag.Int("n", 10000, "number of scenarios in the batch")
	ticks := flag.Int("ticks", 600, "simulation ticks per scenario")
	workers := flag.Int("workers", runtime.NumCPU(), "worker goroutines; 1 is the sequential reference path")
	only := flag.Int("only", -1, "run a single scenario by index and print its derived parameters")
	flag.Parse()

	if *n < 1 || *ticks < 1 {
		fmt.Fprintln(os.Stderr, "simbatch: -n and -ticks must be at least 1")
		os.Exit(2)
	}
	if *only >= 0 {
		if *only >= *n {
			fmt.Fprintf(os.Stderr, "simbatch: -only %d is outside the batch 0..%d\n", *only, *n-1)
			os.Exit(2)
		}
		runOnly(*only, *n, *ticks)
		return
	}
	runBatch(*n, *ticks, *workers)
}

// runOnly reproduces one scenario in isolation, printing the parameters it
// derives from its index so a batch finding can be inspected on its own.
func runOnly(only, n, ticks int) {
	result := simbatch.Arena(only, n, ticks)
	fmt.Printf("scenario %d of %d, %d ticks\n", only, n, ticks)
	fmt.Printf("restitution %.4f\n", simbatch.ArenaRestitution(only, n))
	for b, ball := range simbatch.ArenaLayout(only) {
		fmt.Printf("ball %d  position (%.0f, %.0f)  velocity (%.0f, %.0f)\n",
			b, ball.Position.X, ball.Position.Y, ball.Velocity.X, ball.Velocity.Y)
	}
	fmt.Printf("contacts %d  mean speed %.4f\n", result.Contacts, result.MeanSpeed)
	fmt.Printf("checksum %016x\n", result.Checksum)
}

func runBatch(n, ticks, workers int) {
	start := time.Now()
	results := simbatch.Run(n, workers, func(i int) simbatch.ArenaResult {
		return simbatch.Arena(i, n, ticks)
	})
	elapsed := time.Since(start)

	printDistribution(results, n)

	totalTicks := n * ticks
	fmt.Printf("\nscenarios %d  ticks/scenario %d  total ticks %d\n", n, ticks, totalTicks)
	fmt.Printf("wall time %.2fs  ticks/s %.0f\n",
		elapsed.Seconds(), float64(totalTicks)/elapsed.Seconds())
	fmt.Printf("workers=%d checksum(batch)=%016x\n", workers, foldChecksums(results))
}

// printDistribution groups scenarios into restitution bands. The sweep is
// linear over the index, so equal index ranges are equal restitution ranges.
func printDistribution(results []simbatch.ArenaResult, n int) {
	bands := distributionBands
	if n < bands {
		bands = n
	}
	fmt.Println("restitution     scenarios  contacts/scenario  mean speed")
	for band := range bands {
		lo := band * n / bands
		hi := (band + 1) * n / bands
		sumContacts, sumSpeed := 0, 0.0
		for _, r := range results[lo:hi] {
			sumContacts += r.Contacts
			sumSpeed += r.MeanSpeed
		}
		count := float64(hi - lo)
		fmt.Printf("%.3f - %.3f  %9d  %17.1f  %10.2f\n",
			simbatch.ArenaRestitution(lo, n), simbatch.ArenaRestitution(hi-1, n),
			hi-lo, float64(sumContacts)/count, sumSpeed/count)
	}
	speeds := make([]float64, len(results))
	for i, r := range results {
		speeds[i] = r.MeanSpeed
	}
	sort.Float64s(speeds)
	fmt.Printf("mean speed  min %.2f  median %.2f  max %.2f\n",
		speeds[0], speeds[len(speeds)/2], speeds[len(speeds)-1])
}

// foldChecksums folds the batch checksums in scenario order (FNV-1a over
// uint64 words), so two batches match only when every scenario matches.
func foldChecksums(results []simbatch.ArenaResult) uint64 {
	const offset, prime = uint64(14695981039346656037), uint64(1099511628211)
	h := offset
	for _, r := range results {
		h ^= r.Checksum
		h *= prime
	}
	return h
}
