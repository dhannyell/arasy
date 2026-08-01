//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"sort"
	"testing"
	"time"
)

// BenchmarkPacerPrecision measures what the software pacer can actually hold
// on this machine: 120 paced frames per sample, reporting the interval
// distribution. It exists to make "the limiter cannot hold 60.00" a measured
// fact instead of a suspicion — on Windows the sleep granularity keeps the
// mean a hair above the budget (~59.9fps) regardless of the spin tail.
func BenchmarkPacerPrecision(b *testing.B) {
	for _, spin := range []time.Duration{0, time.Millisecond} {
		pacer := framePacer{budget: time.Second / 60, spinMargin: spin}
		var deltas []time.Duration
		last := time.Now()
		for i := 0; i < 120; i++ {
			workUntil := time.Now().Add(2 * time.Millisecond)
			for time.Now().Before(workUntil) {
			}
			pacer.wait()
			now := time.Now()
			deltas = append(deltas, now.Sub(last))
			last = now
		}
		sort.Slice(deltas, func(i, j int) bool { return deltas[i] < deltas[j] })
		mean := time.Duration(0)
		for _, d := range deltas {
			mean += d
		}
		mean /= time.Duration(len(deltas))
		b.Logf("spin=%v: mean=%v p50=%v p95=%v p99=%v max=%v",
			spin, mean, deltas[len(deltas)/2], deltas[95*len(deltas)/100],
			deltas[99*len(deltas)/100], deltas[len(deltas)-1])
	}
}
