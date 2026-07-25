package runtime

import (
	"sort"
	"time"
)

const (
	frameStatsWindow         = 240
	frameStatsReportInterval = 250 * time.Millisecond
)

// FrameStats describes frame pacing over the most recent rolling window.
// FrameTime is the wall-clock delta between frames; the caller decides what
// work that interval includes.
type FrameStats struct {
	FPS           float64
	FrameTime     time.Duration
	MeanFrameTime time.Duration
	P99FrameTime  time.Duration
}

// FrameStatsTracker aggregates positive frame deltas into a rolling window and
// periodically produces summary statistics.
type FrameStatsTracker struct {
	samples       [frameStatsWindow]time.Duration
	scratch       [frameStatsWindow]time.Duration
	next          int
	count         int
	reportElapsed time.Duration
}

// Observe records frameTime and returns a report roughly every 250ms. Non-
// positive durations are ignored and never trigger a report.
func (t *FrameStatsTracker) Observe(frameTime time.Duration) (FrameStats, bool) {
	if frameTime <= 0 {
		return FrameStats{}, false
	}
	t.samples[t.next] = frameTime
	t.next = (t.next + 1) % len(t.samples)
	if t.count < len(t.samples) {
		t.count++
	}
	t.reportElapsed += frameTime
	if t.reportElapsed < frameStatsReportInterval {
		return FrameStats{}, false
	}
	t.reportElapsed = 0

	var total time.Duration
	for i := 0; i < t.count; i++ {
		total += t.samples[i]
		t.scratch[i] = t.samples[i]
	}
	sort.Slice(t.scratch[:t.count], func(i, j int) bool {
		return t.scratch[i] < t.scratch[j]
	})
	mean := total / time.Duration(t.count)
	rank := (99*t.count + 99) / 100
	return FrameStats{
		FPS:           float64(time.Second) / float64(mean),
		FrameTime:     frameTime,
		MeanFrameTime: mean,
		P99FrameTime:  t.scratch[rank-1],
	}, true
}
