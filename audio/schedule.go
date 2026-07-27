package audio

import "math"

// PlayheadState classifies where an anchored track should be for a given
// simulation time.
type PlayheadState int

const (
	PlayheadUnanchored PlayheadState = iota
	PlayheadPending
	PlayheadPlaying
	PlayheadFinished
)

const (
	// DefaultResyncToleranceSec is the drift a host tolerates before repositioning
	// a track. continuous playback never triggers a resync.
	DefaultResyncToleranceSec = 0.05
	DefaultResyncFadeSec      = 0.01
)

// ExpectedPlayhead returns where an anchored track should be, in seconds from
// the start of the clip, for the given simulation time.
func ExpectedPlayhead(
	currentTick float64,
	alpha float64,
	startTick uint64,
	tickSec,
	duration float64,
	loop bool,
) (offset float64, state PlayheadState) {
	if startTick == 0 {
		return 0, PlayheadUnanchored
	}
	elapsed := (float64(currentTick)+alpha)*tickSec - float64(startTick)*tickSec
	if elapsed < 0 {
		return 0, PlayheadPending
	}
	if duration <= 0 {
		return elapsed, PlayheadPlaying
	}
	if loop {
		return math.Mod(elapsed, duration), PlayheadPlaying
	}
	if elapsed >= duration {
		return 0, PlayheadFinished
	}
	return elapsed, PlayheadPlaying
}

// ExpectedTrackPlayhead returns the playhead for a track after subtracting its
// accumulated paused ticks. While the track is paused, elapsed time remains
// frozen at the tick where the pause began.
func ExpectedTrackPlayhead(currentTick, alpha float64, track Track, tickSec, duration float64) (offset float64, state PlayheadState) {
	if track.StartTick == 0 {
		return 0, PlayheadUnanchored
	}

	pausedTicks := float64(track.PausedTicks)
	if track.Paused && currentTick+alpha > float64(track.PausedAtTick) {
		pausedTicks += currentTick + alpha - float64(track.PausedAtTick)
	}
	elapsed := (currentTick + alpha - float64(track.StartTick) - pausedTicks) * tickSec
	if elapsed < 0 {
		return 0, PlayheadPending
	}
	if duration <= 0 {
		return elapsed, PlayheadPlaying
	}
	if track.Loop {
		return math.Mod(elapsed, duration), PlayheadPlaying
	}
	if elapsed >= duration {
		return 0, PlayheadFinished
	}
	return elapsed, PlayheadPlaying
}

// PlayheadDistance measures the gap between two playhead positions. Looping
// tracks are circular: 0.01 and duration-0.01 are 0.02 apart, not a full clip.
func PlayheadDistance(a, b, duration float64, loop bool) float64 {
	gap := math.Abs(a - b)
	if !loop || duration <= 0 {
		return gap
	}
	gap = math.Mod(gap, duration)
	return math.Min(gap, duration-gap)
}

func ScheduledTrackStart(now float64, currentTick uint64, alpha float64, startTick uint64, tickSec float64) float64 {
	if startTick == 0 {
		return now
	}
	remainingTicks := float64(startTick) - (float64(currentTick) + alpha)
	return now + remainingTicks*tickSec
}

func NeedsResync(actual, expected, duration, tolerance float64, loop bool) bool {
	return PlayheadDistance(actual, expected, duration, loop) > tolerance
}

func ScheduledPlayback(now, scheduledAt, duration float64, loop bool) (start, offset float64, expired bool) {
	if now <= scheduledAt {
		return scheduledAt, 0, false
	}

	offset = now - scheduledAt
	if loop && duration > 0 {
		return now, math.Mod(offset, duration), false
	}
	if duration > 0 && offset >= duration {
		return now, 0, true
	}
	return now, offset, false
}
