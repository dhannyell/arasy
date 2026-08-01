package runtime

import (
	"math"
	"testing"
	"time"
)

func TestFrameStatsTrackerReportsRollingSummary(t *testing.T) {
	var tracker FrameStatsTracker
	const frame = 16600 * time.Microsecond
	var stats FrameStats
	var ok bool
	for range 16 {
		stats, ok = tracker.Observe(frame)
	}
	if !ok {
		t.Fatal("expected a report after 250ms")
	}
	if stats.MeanFrameTime != frame || stats.P99FrameTime != frame {
		t.Fatalf("unexpected summary: mean=%v p99=%v", stats.MeanFrameTime, stats.P99FrameTime)
	}
	if math.Abs(stats.FPS-1/(frame.Seconds())) > 0.01 {
		t.Fatalf("FPS=%v, want approximately %v", stats.FPS, 1/frame.Seconds())
	}
}

func TestFrameStatsTrackerUsesWindowAndP99(t *testing.T) {
	var tracker FrameStatsTracker
	for range 15 {
		tracker.Observe(10 * time.Millisecond)
	}
	var stats FrameStats
	stats, ok := tracker.Observe(100 * time.Millisecond)
	if !ok {
		t.Fatal("expected a report")
	}
	if stats.P99FrameTime != 100*time.Millisecond {
		t.Fatalf("P99=%v, want 100ms", stats.P99FrameTime)
	}
	if stats.MeanFrameTime >= 20*time.Millisecond {
		t.Fatalf("mean=%v, outlier moved mean too far", stats.MeanFrameTime)
	}
}

func TestFrameStatsTrackerIgnoresNegativeFrames(t *testing.T) {
	var tracker FrameStatsTracker
	if _, ok := tracker.Observe(-time.Millisecond); ok {
		t.Fatal("negative frame time should not report")
	}
}

// A host clock coarse enough to report zero for the quickest frames must not
// cost those frames their place in the count: they are the reason the rate is
// high in the first place.
func TestFrameStatsTrackerCountsSubTickFrames(t *testing.T) {
	var tracker FrameStatsTracker
	// Four frames per tick, three of which the clock cannot resolve.
	const tick = 400 * time.Microsecond
	var stats FrameStats
	var ok bool
	for i := range 4000 {
		sample := time.Duration(0)
		if i%4 == 3 {
			sample = tick
		}
		if report, reported := tracker.Observe(sample); reported {
			stats, ok = report, true
		}
	}
	if !ok {
		t.Fatal("expected a report")
	}
	// 4 frames every 400us is 10000 FPS. Averaging only the resolvable frames
	// would report a quarter of that.
	if math.Abs(stats.FPS-10000) > 1 {
		t.Fatalf("FPS=%v, want 10000", stats.FPS)
	}
}
