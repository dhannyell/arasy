//go:build js && wasm

package wasm

import (
	"io/fs"
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/frameforge/audio"
)

type audioSink struct {
	ctx      js.Value
	buffers  map[audio.ClipID]js.Value
	active   map[audio.TrackID]activeTrack
	desired  map[audio.TrackID]audio.Track
	log      *slog.Logger
	unlocked bool

	callbacks []js.Func
}

type activeTrack struct {
	source js.Value
	gain   js.Value
	clip   audio.ClipID
}

func newAudioSink(clips []audio.Clip, files fs.FS, log *slog.Logger) *audioSink {
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
		desired: make(map[audio.TrackID]audio.Track),
		log:     log,
	}
	s.registerUnlock()
	for _, clip := range clips {
		s.loadClip(files, clip)
	}
	return s
}

func (s *audioSink) registerUnlock() {
	unlock := js.FuncOf(func(this js.Value, args []js.Value) any {
		s.unlocked = true
		if !documentHidden() && s.ctx.Get("state").String() == "suspended" {
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
		if s.unlocked && s.ctx.Get("state").String() == "suspended" {
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
		gainValue := cue.Gain
		if gainValue == 0 {
			gainValue = 1
		}
		gain.Get("gain").Set("value", gainValue)

		source.Call("connect", panner)
		panner.Call("connect", gain)
		gain.Call("connect", s.ctx.Get("destination"))
		source.Call("start", 0)
	}
}

func (s *audioSink) reconcileTracks() {
	now := s.ctx.Get("currentTime").Float()
	for id, track := range s.active {
		if _, ok := s.desired[id]; !ok {
			s.fadeAndStop(track, now, 0)
			delete(s.active, id)
		}
	}

	for id, track := range s.desired {
		buffer, ok := s.buffers[track.ClipID]
		if !ok {
			continue
		}
		current, active := s.active[id]
		fadeSec := track.FadeSec
		gainValue := track.Gain
		if gainValue == 0 {
			gainValue = 1
		}

		if !active || current.clip != track.ClipID {
			if active {
				s.fadeAndStop(current, now, fadeSec)
			}
			source := s.ctx.Call("createBufferSource")
			source.Set("buffer", buffer)
			source.Set("loop", track.Loop)
			gain := s.ctx.Call("createGain")
			gain.Get("gain").Call("setValueAtTime", 0, now)
			gain.Get("gain").Call("linearRampToValueAtTime", gainValue, now+fadeSec)
			source.Call("connect", gain)
			gain.Call("connect", s.ctx.Get("destination"))
			source.Call("start", 0)
			s.active[id] = activeTrack{source: source, gain: gain, clip: track.ClipID}
			continue
		}

		current.gain.Get("gain").Call("setValueAtTime", current.gain.Get("gain").Get("value").Float(), now)
		current.gain.Get("gain").Call("linearRampToValueAtTime", gainValue, now+0.05)
	}
}

func (s *audioSink) fadeAndStop(track activeTrack, now, fadeSec float64) {
	gain := track.gain.Get("gain")
	gain.Call("setValueAtTime", gain.Get("value").Float(), now)
	gain.Call("linearRampToValueAtTime", 0, now+fadeSec)
	track.source.Call("stop", now+fadeSec)
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
