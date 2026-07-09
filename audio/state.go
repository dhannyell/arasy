package audio

import "maps"

// State is the desired continuous-audio state owned by the simulation. It is
// captured in rewind snapshots and reconstructed by replay, so scrubbing the
// timeline restores the music that was playing at the target tick.
type State struct {
	Tracks map[TrackID]Track
}

// Clone is used by the runtime driver when capturing rewind snapshots.
func (s State) Clone() State {
	return State{Tracks: maps.Clone(s.Tracks)}
}

// Output is one delivery to a platform sink: the one-shot cues fired since
// the last delivery plus a snapshot of the desired continuous tracks.
type Output struct {
	Cues   []Cue
	Tracks map[TrackID]Track
}
