package runtime

import (
	"runtime"
	"testing"
	"time"
)

// The first Sample has nothing to subtract from, and the cumulative totals
// since process start are not what a per-frame reading means. Reporting them
// would put a spike on the graph for every collection the process ever ran.
func TestGCStatsTrackerPrimesBeforeReporting(t *testing.T) {
	var tracker GCStatsTracker

	if _, ok := tracker.Sample(); ok {
		t.Fatal("first sample reported a delta; it has no previous reading to subtract")
	}
	if _, ok := tracker.Sample(); !ok {
		t.Fatal("second sample did not report")
	}
}

// A quiet interval must read as quiet. If idle frames reported cycles the
// marking would be permanent and mean nothing.
func TestGCStatsTrackerReportsNothingWhileIdle(t *testing.T) {
	var tracker GCStatsTracker
	tracker.Sample()

	stats, ok := tracker.Sample()
	if !ok {
		t.Fatal("sample did not report")
	}
	if stats.Cycles != 0 {
		t.Fatalf("cycles = %d between two adjacent samples, want 0", stats.Cycles)
	}
	if stats.Pause != 0 {
		t.Fatalf("pause = %v with no collection, want 0", stats.Pause)
	}
}

// The whole point: a collection between two samples has to show up in the one
// that follows it, or the frame graph cannot mark the frame that paid for it.
func TestGCStatsTrackerSeesACollection(t *testing.T) {
	var tracker GCStatsTracker
	tracker.Sample()

	runtime.GC()

	stats, ok := tracker.Sample()
	if !ok {
		t.Fatal("sample did not report")
	}
	if stats.Cycles == 0 {
		t.Fatal("a forced collection was not counted")
	}
	if stats.HeapGoal == 0 {
		t.Fatal("heap goal is zero; the next cycle's threshold is unknown")
	}
}

// Allocation is what actually drives collection frequency, so the delta has to
// track it. The sink keeps the compiler from optimising the garbage away.
func TestGCStatsTrackerCountsAllocation(t *testing.T) {
	var tracker GCStatsTracker
	tracker.Sample()

	var sink [][]byte
	for range 64 {
		sink = append(sink, make([]byte, 64*1024))
	}
	runtime.KeepAlive(sink)

	stats, ok := tracker.Sample()
	if !ok {
		t.Fatal("sample did not report")
	}
	if want := uint64(4 << 20); stats.AllocBytes < want {
		t.Fatalf("alloc = %d bytes after allocating at least %d", stats.AllocBytes, want)
	}
}

// Pause comes out of a histogram, so it is an approximation by construction.
// It still has to be a plausible duration rather than a garbage number, which
// is what a mistake in the bucket weighting would produce.
func TestGCStatsTrackerPauseStaysPlausible(t *testing.T) {
	var tracker GCStatsTracker
	tracker.Sample()

	for range 4 {
		runtime.GC()
	}

	stats, ok := tracker.Sample()
	if !ok {
		t.Fatal("sample did not report")
	}
	if stats.Pause <= 0 {
		t.Fatalf("pause = %v across four collections, want a positive duration", stats.Pause)
	}
	if stats.Pause > time.Second {
		t.Fatalf("pause = %v across four collections; the bucket weighting is wrong", stats.Pause)
	}
}
