package audio

// Cue is a fire-and-forget playback request emitted by the simulation.
// (Tick, Seq) uniquely identifies a cue within a run: Seq restarts at zero
// each tick, so re-simulated timelines produce identical keys — the hook a
// future rollback layer needs to dedupe already-heard cues.
type Cue struct {
	Tick uint64
	Seq  uint32
	Clip ClipID

	// Gain is the linear volume in [0, 1]. The zero value means 1.0 —
	// constructors normalize it (and clamp the rest into range) so a
	// zero-valued options struct stays useful.
	Gain float64
	// Pan is the stereo position in [-1, 1]; 0 is centered.
	Pan float64
}

// CueOptions tunes a one-shot. Zero value = defaults (gain 1, centered).
type CueOptions struct {
	Gain float64
	Pan  float64
}
