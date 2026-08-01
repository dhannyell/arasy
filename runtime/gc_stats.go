package runtime

import (
	"math"
	"runtime/metrics"
	"time"
)

// GCStats describes garbage collection since the previous sample. It answers
// the question a frame graph raises and cannot settle on its own: that spike —
// was it the collector?
//
// Attribution stops at "how much was allocated", deliberately. No function
// forces a collection: a cycle starts when the heap reaches the GOGC goal, and
// whichever allocation happens to cross that line is arbitrary. The call site
// that trips it is usually innocent; the one that allocated the bulk since the
// last cycle is the one worth finding, and AllocBytes is what points at it.
type GCStats struct {
	// Cycles completed since the previous sample. Anything above zero means
	// the collector ran during the interval.
	Cycles uint64
	// Pause is stop-the-world time in the interval, summed across cycles. This
	// is the part a player would feel.
	Pause time.Duration
	// AllocBytes is what the program allocated during the interval, whether or
	// not it survived. A steady per-frame figure here is the allocation rate
	// that sets how often the collector has to run.
	AllocBytes uint64
	// HeapLive and HeapGoal are the current live heap and the size that
	// triggers the next cycle. Their ratio is how close the next one is.
	HeapLive uint64
	HeapGoal uint64
}

// Metric names are resolved once; every one of these has existed since Go 1.21
// except the pause histogram, which predates it.
const (
	gcMetricCycles = "/gc/cycles/total:gc-cycles"
	gcMetricPauses = "/gc/pauses:seconds"
	gcMetricAllocs = "/gc/heap/allocs:bytes"
	gcMetricLive   = "/gc/heap/live:bytes"
	gcMetricGoal   = "/gc/heap/goal:bytes"
)

// GCStatsTracker turns the runtime's cumulative collector counters into
// per-interval deltas. It is deliberately built on runtime/metrics rather than
// runtime.ReadMemStats: ReadMemStats stops the world to answer, which is a
// strange thing to do in a tool for finding pauses.
//
// The zero value is ready to use, and Sample is cheap enough to call every
// frame. Both hosts share it, browser included.
type GCStatsTracker struct {
	samples []metrics.Sample
	primed  bool

	prevCycles uint64
	prevPause  time.Duration
	prevAllocs uint64
}

// Sample reports collector activity since the previous call. The first call
// primes the counters and reports ok false: a delta needs two readings, and
// the cumulative totals from process start are not what a frame graph wants.
func (t *GCStatsTracker) Sample() (GCStats, bool) {
	if t.samples == nil {
		t.samples = []metrics.Sample{
			{Name: gcMetricCycles},
			{Name: gcMetricPauses},
			{Name: gcMetricAllocs},
			{Name: gcMetricLive},
			{Name: gcMetricGoal},
		}
	}
	metrics.Read(t.samples)

	cycles, ok := uint64Value(t.samples[0])
	if !ok {
		return GCStats{}, false
	}
	pause, ok := histogramTotal(t.samples[1])
	if !ok {
		return GCStats{}, false
	}
	allocs, ok := uint64Value(t.samples[2])
	if !ok {
		return GCStats{}, false
	}
	live, _ := uint64Value(t.samples[3])
	goal, _ := uint64Value(t.samples[4])

	stats := GCStats{HeapLive: live, HeapGoal: goal}
	if t.primed {
		stats.Cycles = cycles - t.prevCycles
		stats.Pause = pause - t.prevPause
		stats.AllocBytes = allocs - t.prevAllocs
	}
	wasPrimed := t.primed
	t.prevCycles, t.prevPause, t.prevAllocs = cycles, pause, allocs
	t.primed = true
	return stats, wasPrimed
}

func uint64Value(sample metrics.Sample) (uint64, bool) {
	if sample.Value.Kind() != metrics.KindUint64 {
		return 0, false
	}
	return sample.Value.Uint64(), true
}

// histogramTotal collapses a pause histogram into the total time it
// represents. Each bucket count is weighted by the midpoint of its bounds,
// which is the usual approximation; the open-ended first and last buckets fall
// back to the finite bound they do have.
func histogramTotal(sample metrics.Sample) (time.Duration, bool) {
	if sample.Value.Kind() != metrics.KindFloat64Histogram {
		return 0, false
	}
	histogram := sample.Value.Float64Histogram()
	if histogram == nil {
		return 0, false
	}
	total := 0.0
	for i, count := range histogram.Counts {
		if count == 0 {
			continue
		}
		lo, hi := histogram.Buckets[i], histogram.Buckets[i+1]
		loInf, hiInf := math.IsInf(lo, 0), math.IsInf(hi, 0)
		switch {
		case loInf && hiInf:
			continue
		case loInf:
			total += float64(count) * hi
		case hiInf:
			total += float64(count) * lo
		default:
			total += float64(count) * (lo + hi) / 2
		}
	}
	return time.Duration(total * float64(time.Second)), true
}
