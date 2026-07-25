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

func TestFrameStatsTrackerIgnoresNonPositiveFrames(t *testing.T) {
	var tracker FrameStatsTracker
	if _, ok := tracker.Observe(0); ok {
		t.Fatal("zero frame time should not report")
	}
	if _, ok := tracker.Observe(-time.Millisecond); ok {
		t.Fatal("negative frame time should not report")
	}
}
