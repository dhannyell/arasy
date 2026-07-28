package anim

import "github.com/dhannyell/arasy/ease"

// Timeline evaluates animation tracks at one caller-owned elapsed time.
// Elapsed time and track boundaries must be finite.
type Timeline struct {
	elapsed float64
	active  bool
}

// At returns an active timeline evaluated at elapsed seconds.
func At(elapsed float64) Timeline {
	return Timeline{elapsed: elapsed, active: true}
}

// Inactive returns a timeline whose tracks remain at zero and report done.
func Inactive() Timeline {
	return Timeline{}
}

// Elapsed reports the caller-provided elapsed time.
func (t Timeline) Elapsed() float64 {
	return t.elapsed
}

// Done reports whether the timeline is inactive or has reached duration.
func (t Timeline) Done(duration float64) bool {
	return !t.active || t.elapsed >= duration
}

// Track evaluates the window [start, end] with curve.
func (t Timeline) Track(start, end float64, curve ease.Curve) Track {
	if !t.active {
		return Track{}
	}
	duration := end - start
	if duration <= 0 {
		return Track{value: 1, done: true}
	}
	progress := ease.Progress(t.elapsed-start, duration)
	return Track{value: curve.Sample(progress), done: t.elapsed >= end}
}

// Track is one eased window within a Timeline.
type Track struct {
	value float64
	done  bool
}

// Value returns the eased progress for this track.
func (t Track) Value() float64 {
	return t.value
}

// Lerp interpolates linearly from a to b using Value.
func (t Track) Lerp(a, b float64) float64 {
	return a + (b-a)*t.value
}

// Done reports whether the timeline has reached the track's end.
func (t Track) Done() bool {
	return t.done
}
