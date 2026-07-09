package audio

// TrackID names a continuous audio slot ("music", "ambience", "crowd").
type TrackID string

// Track is the desired state of a continuous sound. Sinks reconcile the
// desired set against what is actually playing every delivery: start what is
// missing, stop what is stale, retune gain on what stays.
type Track struct {
	ClipID ClipID

	// Gain is the linear volume; zero means 1.0 (normalized on SetTrack).
	Gain float64
	Loop bool
	// FadeSec is the fade-in/out the sink applies when this track starts or
	// stops. Zero = no fade.
	FadeSec float64
}
