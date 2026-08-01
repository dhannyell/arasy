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

// FrameStatsTracker aggregates frame deltas into a rolling window and
// periodically produces summary statistics.
type FrameStatsTracker struct {
	samples       [frameStatsWindow]time.Duration
	scratch       [frameStatsWindow]time.Duration
	next          int
	count         int
	reportElapsed time.Duration
	reportFrames  int
}

// Observe records frameTime and returns a report roughly every 250ms. Only
// negative durations are ignored, and they never trigger a report.
//
// A zero delta is a frame, not a non-event. Above roughly a thousand frames a
// second two consecutive host timestamps can land in the same clock tick, and
// discarding those frames would drop the fastest ones from the window while
// keeping the time they took — which reads as a frame rate a fifth lower than
// the one the machine is actually delivering.
func (t *FrameStatsTracker) Observe(frameTime time.Duration) (FrameStats, bool) {
	if frameTime < 0 {
		return FrameStats{}, false
	}
	t.samples[t.next] = frameTime
	t.next = (t.next + 1) % len(t.samples)
	if t.count < len(t.samples) {
		t.count++
	}
	t.reportElapsed += frameTime
	t.reportFrames++
	if t.reportElapsed < frameStatsReportInterval {
		return FrameStats{}, false
	}
	// The frame rate is frames over wall time, the same quantity an external
	// overlay counts. Deriving it from the window's mean instead would measure
	// only the 240 frames before the report, and reports land at fixed time
	// intervals rather than fixed frame counts -- so the window preferentially
	// covers whatever the machine was doing slowly.
	fps := float64(t.reportFrames) * float64(time.Second) / float64(t.reportElapsed)
	t.reportElapsed = 0
	t.reportFrames = 0

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
		FPS:           fps,
		FrameTime:     frameTime,
		MeanFrameTime: mean,
		P99FrameTime:  t.scratch[rank-1],
	}, true
}
