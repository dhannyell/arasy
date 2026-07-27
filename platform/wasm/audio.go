//go:build js && wasm

package wasm

import (
	"io/fs"
	"log/slog"
	"math"
	"syscall/js"

	"github.com/dhannyell/arasy/audio"
)

type audioSink struct {
	ctx      js.Value
	buffers  map[audio.ClipID]js.Value
	active   map[audio.TrackID]activeTrack
	pending  map[audio.TrackID]pendingTrack
	desired  map[audio.TrackID]audio.Track
	log      *slog.Logger
	unlocked bool
	tick     uint64
	alpha    float64
	tickSec  float64

	callbacks []js.Func

	simulationPaused bool
}

type activeTrack struct {
	source      js.Value
	gain        js.Value
	clip        audio.ClipID
	start       float64
	startOffset float64
	loop        bool
	paused      bool
}

type pendingTrack struct {
	clip  audio.ClipID
	start float64
}

func newAudioSink(clips []audio.Clip, files fs.FS, simulationHz int, log *slog.Logger) *audioSink {
	audioContext := js.Global().Get("AudioContext")
	if audioContext.IsUndefined() {
		audioContext = js.Global().Get("webkitAudioContext")
	}
	if audioContext.IsUndefined() {
		log.Warn("Web Audio API unavailable; audio disabled")
		return nil
	}

	s := &audioSink{
		ctx:     audioContext.New(),
		buffers: make(map[audio.ClipID]js.Value),
		active:  make(map[audio.TrackID]activeTrack),
		pending: make(map[audio.TrackID]pendingTrack),
		desired: make(map[audio.TrackID]audio.Track),
		log:     log,
		tickSec: 1 / float64(simulationHz),
	}
	s.registerUnlock()
	for _, clip := range clips {
		s.loadClip(files, clip)
	}
	return s
}

func (t activeTrack) audiblePlayhead(now, duration float64) (float64, bool) {
	if now < t.start {
		return 0, false
	}
	position := (now - t.start) + t.startOffset
	if t.loop && duration > 0 {
		position = math.Mod(position, duration)
	}
	return position, true
}

func (s *audioSink) registerUnlock() {
	unlock := js.FuncOf(func(this js.Value, args []js.Value) any {
		s.unlocked = true
		if !documentHidden() && s.ctx.Get("state").String() == "suspended" && !s.simulationPaused {
			s.ctx.Call("resume")
		}
		return nil
	})
	visibility := js.FuncOf(func(this js.Value, args []js.Value) any {
		if documentHidden() {
			if s.ctx.Get("state").String() == "running" {
				s.ctx.Call("suspend")
			}
			return nil
		}
		if s.unlocked && s.ctx.Get("state").String() == "suspended" && !s.simulationPaused {
			s.ctx.Call("resume")
		}
		return nil
	})
	s.callbacks = append(s.callbacks, unlock, visibility)

	opts := map[string]any{"once": true}
	doc := js.Global().Get("document")
	doc.Call("addEventListener", "pointerdown", unlock, opts)
	doc.Call("addEventListener", "keydown", unlock, opts)
	doc.Call("addEventListener", "visibilitychange", visibility)
}

func (s *audioSink) loadClip(files fs.FS, clip audio.Clip) {
	data, err := fs.ReadFile(files, clip.Path)
	if err != nil {
		s.log.Warn("audio clip load failed", "id", clip.ID, "path", clip.Path, "err", err)
		return
	}

	array := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(array, data)
	promise := s.ctx.Call("decodeAudioData", array.Get("buffer"))

	onFulfilled := js.FuncOf(func(this js.Value, args []js.Value) any {
		s.buffers[clip.ID] = args[0]
		s.reconcileTracks()
		return nil
	})
	onRejected := js.FuncOf(func(this js.Value, args []js.Value) any {
		s.log.Warn("audio clip decode failed", "id", clip.ID, "path", clip.Path, "err", jsErrorString(args))
		return nil
	})
	s.callbacks = append(s.callbacks, onFulfilled, onRejected)
	promise.Call("then", onFulfilled, onRejected)
}

func (s *audioSink) consume(out audio.Output) {
	s.playCues(out.Cues)
	s.tick = out.Tick
	s.alpha = out.Alpha
	s.desired = out.Tracks
	s.reconcileTracks()
}

func (s *audioSink) playCues(cues []audio.Cue) {
	if s.ctx.Get("state").String() != "running" {
		return
	}

	voices := make(map[audio.ClipID]int)
	for _, cue := range cues {
		if voices[cue.Clip] >= 4 {
			continue
		}
		buffer, ok := s.buffers[cue.Clip]
		if !ok {
			continue
		}
		voices[cue.Clip]++

		source := s.ctx.Call("createBufferSource")
		source.Set("buffer", buffer)
		panner := s.ctx.Call("createStereoPanner")
		panner.Get("pan").Set("value", cue.Pan)
		gain := s.ctx.Call("createGain")
		gain.Get("gain").Set("value", cue.Gain)

		source.Call("connect", panner)
		panner.Call("connect", gain)
		gain.Call("connect", s.ctx.Get("destination"))
		source.Call("start", 0)
	}
}

func (s *audioSink) reconcileTracks() {
	now := s.ctx.Get("currentTime").Float()
	for id, active := range s.active {
		desired, isDesired := s.desired[id]
		if !isDesired {
			s.fadeAndStop(active, now, 0)
			delete(s.active, id)
			continue
		}
		if active.clip != desired.Clip {
			continue
		}
		if desired.Paused {
			s.pauseTrack(id, active, desired.FadeSec, now)
			continue
		}

		duration := s.buffers[active.clip].Get("duration").Float()
		expected, state := audio.ExpectedTrackPlayhead(
			float64(s.tick),
			s.alpha,
			desired,
			s.tickSec,
			duration,
		)

		switch state {
		case audio.PlayheadUnanchored:
			s.retuneTrack(active, desired.Gain, desired.FadeSec, now)
		case audio.PlayheadPending:
			active.source.Call("stop", now)
			delete(s.active, id)
		case audio.PlayheadFinished:
			active.source.Call("stop", now)
			delete(s.active, id)
		case audio.PlayheadPlaying:
			if active.paused {
				s.resyncTrack(id, active, desired, s.buffers[active.clip], expected, now)
				continue
			}
			actual, ok := active.audiblePlayhead(now, duration)
			if !ok {
				s.retuneTrack(active, desired.Gain, desired.FadeSec, now)
				continue
			}
			isResync := audio.NeedsResync(actual, expected, duration, audio.DefaultResyncToleranceSec, active.loop)
			if isResync {
				s.resyncTrack(id, active, desired, s.buffers[active.clip], expected, now)
				continue
			}
			s.retuneTrack(active, desired.Gain, desired.FadeSec, now)
		}
	}
	for id := range s.pending {
		if _, ok := s.desired[id]; !ok {
			delete(s.pending, id)
		}
	}

	for id, track := range s.desired {
		current, active := s.active[id]
		fadeSec := track.FadeSec
		gainValue := track.Gain

		if active && current.clip == track.Clip {
			if track.Paused {
				continue
			}
			s.retuneTrack(current, gainValue, fadeSec, now)
			continue
		}
		if active {
			s.fadeAndStop(current, now, fadeSec)
			delete(s.active, id)
		}

		waiting := pendingTrack{
			clip:  track.Clip,
			start: audio.ScheduledTrackStart(now, s.tick, s.alpha, track.StartTick, s.tickSec),
		}
		s.pending[id] = waiting

		buffer, ok := s.buffers[track.Clip]
		if !ok {
			continue
		}
		start, offset, expired := audio.ScheduledPlayback(now, waiting.start, buffer.Get("duration").Float(), track.Loop)
		if expired {
			continue
		}

		source := s.ctx.Call("createBufferSource")
		source.Set("buffer", buffer)
		source.Set("loop", track.Loop)
		gain := s.ctx.Call("createGain")
		gain.Get("gain").Call("setValueAtTime", 0, start)
		gain.Get("gain").Call("linearRampToValueAtTime", gainValue, start+fadeSec)
		source.Call("connect", gain)
		gain.Call("connect", s.ctx.Get("destination"))
		source.Call("start", start, offset)
		s.active[id] = activeTrack{
			source:      source,
			gain:        gain,
			clip:        track.Clip,
			start:       start,
			startOffset: offset,
			loop:        track.Loop,
			paused:      false,
		}
		delete(s.pending, id)
	}
}

func (s *audioSink) pauseTrack(id audio.TrackID, track activeTrack, fadeSec, now float64) {
	if track.paused {
		return
	}
	gain := track.gain.Get("gain")
	gain.Call("cancelScheduledValues", now)
	if now < track.start {
		gain.Call("setValueAtTime", 0, track.start)
	} else {
		gain.Call("setValueAtTime", gain.Get("value").Float(), now)
		gain.Call("linearRampToValueAtTime", 0, now+fadeSec)
	}
	track.paused = true
	s.active[id] = track
}

func (s *audioSink) retuneTrack(track activeTrack, gainValue, fadeSec, now float64) {
	gain := track.gain.Get("gain")
	gain.Call("cancelScheduledValues", now)
	if now < track.start {
		gain.Call("setValueAtTime", 0, track.start)
		gain.Call("linearRampToValueAtTime", gainValue, track.start+fadeSec)
		return
	}
	gain.Call("setValueAtTime", gain.Get("value").Float(), now)
	gain.Call("linearRampToValueAtTime", gainValue, now+0.05)
}

func (s *audioSink) fadeAndStop(track activeTrack, now, fadeSec float64) {
	gain := track.gain.Get("gain")
	gain.Call("cancelScheduledValues", now)
	if now < track.start {
		track.source.Call("stop", now)
		return
	}
	gain.Call("setValueAtTime", gain.Get("value").Float(), now)
	gain.Call("linearRampToValueAtTime", 0, now+fadeSec)
	track.source.Call("stop", now+fadeSec)
}

func (s *audioSink) resyncTrack(
	id audio.TrackID,
	current activeTrack,
	track audio.Track,
	buffer js.Value,
	offset, now float64,
) {
	old := current.gain.Get("gain")
	old.Call("cancelScheduledValues", now)
	old.Call("setValueAtTime", old.Get("value").Float(), now)
	old.Call("linearRampToValueAtTime", 0, now+audio.DefaultResyncFadeSec)
	current.source.Call("stop", now+audio.DefaultResyncFadeSec)

	source := s.ctx.Call("createBufferSource")
	source.Set("buffer", buffer)
	source.Set("loop", track.Loop)
	gain := s.ctx.Call("createGain")
	gain.Get("gain").Call("setValueAtTime", 0, now)
	gain.Get("gain").Call("linearRampToValueAtTime", track.Gain, now+audio.DefaultResyncFadeSec)
	source.Call("connect", gain)
	gain.Call("connect", s.ctx.Get("destination"))
	source.Call("start", now, offset)

	s.active[id] = activeTrack{
		source:      source,
		gain:        gain,
		clip:        track.Clip,
		start:       now,
		startOffset: offset,
		loop:        track.Loop,
		paused:      false,
	}
}

func (s *audioSink) setAudioContextState(paused bool) {
	if s.simulationPaused == paused {
		return
	}
	s.simulationPaused = paused

	state := s.ctx.Get("state").String()
	if paused {
		if state == "running" {
			s.ctx.Call("suspend")
		}
		return
	}

	if s.unlocked && !documentHidden() && state == "suspended" {
		s.ctx.Call("resume")
	}
}

func jsErrorString(args []js.Value) string {
	if len(args) == 0 {
		return ""
	}
	return args[0].String()
}

func documentHidden() bool {
	return js.Global().Get("document").Get("hidden").Bool()
}
