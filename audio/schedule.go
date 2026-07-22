package audio

import "math"

func ScheduledTrackStart(now float64, currentTick uint64, alpha float64, startTick uint64, tickSec float64) float64 {
	if startTick == 0 {
		return now
	}
	remainingTicks := float64(startTick) - (float64(currentTick) + alpha)
	return now + remainingTicks*tickSec
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
