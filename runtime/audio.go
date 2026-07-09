package runtime

import (
	"maps"

	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/math2d"
)

const (
	// maxPendingCues bounds the cue queue for hosts that never consume audio
	// (the offline raster host, tests). Oldest cues are dropped first: a sink
	// that starts consuming later should hear the recent past, not tick zero.
	maxPendingCues = 512

	defaultGain = 1.0
)

// driverAudio owns the simulation-side audio state: the desired continuous
// tracks (snapshotted for rewind) and the pending one-shot cues (derived
// output, never snapshotted).
type driverAudio struct {
	state        audio.State
	pending      []audio.Cue
	tick         uint64
	seq          uint32
	suppressCues bool
}

func (a *driverAudio) beginTick(tick uint64) {
	a.tick = tick
	a.seq = 0
}

// normalizeGain maps the zero value to the default and clamps everything else
// into the valid [0, 1] range. Cues and tracks share the same policy.
func normalizeGain(gain float64) float64 {
	if gain == 0 {
		return defaultGain
	}
	return math2d.Clamp(gain, 0.0, 1.0)
}

func (a *driverAudio) play(clip audio.ClipID, opts audio.CueOptions) {
	opts.Gain = normalizeGain(opts.Gain)
	cue := audio.Cue{Tick: a.tick, Seq: a.seq, Clip: clip, Gain: opts.Gain, Pan: opts.Pan}
	a.seq++
	if a.suppressCues {
		return
	}
	a.pending = append(a.pending, cue)
	if len(a.pending) > maxPendingCues {
		a.pending = a.pending[len(a.pending)-maxPendingCues:]
	}
}

func (a *driverAudio) setTrack(id audio.TrackID, track audio.Track) {
	if a.state.Tracks == nil {
		a.state.Tracks = make(map[audio.TrackID]audio.Track)
	}
	track.Gain = normalizeGain(track.Gain)
	a.state.Tracks[id] = track
}

func (a *driverAudio) stopTrack(id audio.TrackID) {
	delete(a.state.Tracks, id)
}

func (a *driverAudio) reset() {
	a.tick = 0
	a.seq = 0
	a.pending = a.pending[:0]
	a.state = audio.State{}
	a.suppressCues = false
}

func (a *driverAudio) consume() audio.Output {
	output := audio.Output{
		Cues:   a.pending,
		Tracks: maps.Clone(a.state.Tracks),
	}
	a.pending = nil

	return output
}

// AudioPort is the game-facing audio API, reachable as ctx.Audio inside
// Init, Update and PostStep. All emissions are tick-stamped and deterministic:
// the same simulation always fires the same cues with the same keys.
type AudioPort struct {
	audio *driverAudio
}

func (p *AudioPort) Play(clip audio.ClipID) {
	p.audio.play(clip, audio.CueOptions{})
}

func (p *AudioPort) PlayWith(clip audio.ClipID, opts audio.CueOptions) {
	p.audio.play(clip, opts)
}

func (p *AudioPort) SetTrack(id audio.TrackID, track audio.Track) {
	p.audio.setTrack(id, track)
}

func (p *AudioPort) StopTrack(id audio.TrackID) {
	p.audio.stopTrack(id)
}
