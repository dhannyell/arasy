package audio

// TrackID names a continuous audio slot ("music", "ambience", "crowd").
type TrackID string

// Track is the desired state of a continuous sound. Sinks reconcile the
// desired set against what is actually playing every delivery: start what is
// missing, stop what is stale, retune gain on what stays.
type Track struct {
	Clip ClipID

	// Gain is the linear volume in [0, 1]; zero means 1.0. SetTrack normalizes
	// the zero value to 1.0 and clamps everything else into range.
	Gain float64
	Loop bool
	// FadeSec is the fade-in/out the sink applies when this track starts or
	// stops. Zero = no fade.
	FadeSec float64
	// StartTick is the simulation tick at which playback should begin.
	// Audio-capable hosts translate it to their own precise clock. Zero starts
	// immediately, preserving the useful zero value.
	StartTick uint64

	Paused       bool
	PausedAtTick uint64
	PausedTicks  uint64
}
