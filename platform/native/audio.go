//go:build windows && amd64

package native

import (
	"errors"
	"io/fs"
	"log/slog"
	"sync"
	"time"

	"github.com/dhannyell/arasy/audio"
	"github.com/ebitengine/oto/v3"
)

const (
	audioSampleRate  = 48_000
	audioChannels    = 2
	maxCueVoices     = 32
	maxCueVoicesClip = 4
)

var processAudioContext struct {
	once  sync.Once
	ctx   *oto.Context
	ready <-chan struct{}
	err   error
}

type audioSink struct {
	ctx       *oto.Context
	clips     map[audio.ClipID]*clipPCM
	active    map[audio.TrackID]*activeTrack
	pending   map[audio.TrackID]pendingTrack
	desired   map[audio.TrackID]audio.Track
	cueVoices []*cueVoice
	retiring  []*oto.Player
	log       *slog.Logger
	start     time.Time
	tick      uint64
	alpha     float64
	tickSec   float64
	closed    bool
}

type cueVoice struct {
	clip   audio.ClipID
	player *oto.Player
}

type activeTrack struct {
	player *oto.Player
	voice  *voice
	clip   audio.ClipID
	start  float64
}

type pendingTrack struct {
	clip  audio.ClipID
	start float64
}

func newAudioSink(clips []audio.Clip, files fs.FS, simulationHz int, log *slog.Logger) (*audioSink, error) {
	ctx, err := nativeAudioContext()
	if err != nil {
		return nil, err
	}
	if simulationHz <= 0 {
		return nil, errors.New("native/audio: invalid simulation frequency")
	}
	if log == nil {
		log = slog.Default()
	}

	s := &audioSink{
		ctx:     ctx,
		clips:   make(map[audio.ClipID]*clipPCM),
		active:  make(map[audio.TrackID]*activeTrack),
		pending: make(map[audio.TrackID]pendingTrack),
		desired: make(map[audio.TrackID]audio.Track),
		log:     log,
		start:   time.Now(),
		tickSec: 1 / float64(simulationHz),
	}
	for _, clip := range clips {
		data, readErr := fs.ReadFile(files, clip.Path)
		if readErr != nil {
			log.Warn("native audio clip load failed", "id", clip.ID, "path", clip.Path, "err", readErr)
			continue
		}
		pcm, decodeErr := decodeClip(data, clip.Path, audioSampleRate)
		if decodeErr != nil {
			log.Warn("native audio clip decode failed", "id", clip.ID, "path", clip.Path, "err", decodeErr)
			continue
		}
		s.clips[clip.ID] = pcm
	}
	return s, nil
}

func nativeAudioContext() (*oto.Context, error) {
	processAudioContext.once.Do(func() {
		processAudioContext.ctx, processAudioContext.ready, processAudioContext.err = oto.NewContext(&oto.NewContextOptions{
			SampleRate:   audioSampleRate,
			ChannelCount: 2,
			Format:       oto.FormatSignedInt16LE,
		})
	})
	if processAudioContext.err != nil {
		return nil, processAudioContext.err
	}
	<-processAudioContext.ready
	if err := processAudioContext.ctx.Err(); err != nil {
		return nil, err
	}
	return processAudioContext.ctx, nil
}

func (s *audioSink) consume(out audio.Output) {
	if s.closed {
		return
	}
	s.collectFinishedPlayers()
	s.playCues(out.Cues)
	s.tick = out.Tick
	s.alpha = out.Alpha
	s.desired = out.Tracks
	s.reconcileTracks()
}

func (s *audioSink) playCues(cues []audio.Cue) {
	for _, cue := range cues {
		if !s.canPlayCue(cue.Clip) {
			continue
		}
		clip, ok := s.clips[cue.Clip]
		if !ok {
			continue
		}
		v := newVoice(clip.Samples, false, cue.Pan, cue.Gain)
		player := s.ctx.NewPlayer(v)
		player.Play()
		s.cueVoices = append(s.cueVoices, &cueVoice{clip: cue.Clip, player: player})
	}
}

func (s *audioSink) reconcileTracks() {
	now := time.Since(s.start).Seconds()
	for id, track := range s.active {
		if _, ok := s.desired[id]; !ok {
			track.voice.fadeOut(0, audioSampleRate)
			s.retiring = append(s.retiring, track.player)
			delete(s.active, id)
		}
	}
	for id := range s.pending {
		if _, ok := s.desired[id]; !ok {
			delete(s.pending, id)
		}
	}

	for id, track := range s.desired {
		current, active := s.active[id]
		if active && current.clip == track.Clip {
			current.voice.setGain(track.Gain, track.FadeSec, audioSampleRate)
			continue
		}
		if active {
			current.voice.fadeOut(track.FadeSec, audioSampleRate)
			s.retiring = append(s.retiring, current.player)
			delete(s.active, id)
		}

		waiting, ok := s.pending[id]
		if !ok || waiting.clip != track.Clip {
			waiting = pendingTrack{
				clip:  track.Clip,
				start: audio.ScheduledTrackStart(now, s.tick, s.alpha, track.StartTick, s.tickSec),
			}
			s.pending[id] = waiting
		}

		clip, ok := s.clips[track.Clip]
		if !ok {
			continue
		}
		start, offset, expired := audio.ScheduledPlayback(now, waiting.start, clipDuration(clip), track.Loop)
		if expired {
			continue
		}

		v := newVoice(clip.Samples, track.Loop, 0, 0)
		v.setPosition(int(offset * audioSampleRate))
		v.setGain(track.Gain, track.FadeSec, audioSampleRate)
		player := s.ctx.NewPlayer(v)
		player.Play()
		s.active[id] = &activeTrack{player: player, voice: v, clip: track.Clip, start: start}
		delete(s.pending, id)
	}
}

func (s *audioSink) canPlayCue(clip audio.ClipID) bool {
	return len(s.cueVoices) < maxCueVoices && s.cueVoiceCount(clip) < maxCueVoicesClip
}

func (s *audioSink) cueVoiceCount(clip audio.ClipID) int {
	count := 0
	for _, voice := range s.cueVoices {
		if voice.clip == clip {
			count++
		}
	}
	return count
}

func (s *audioSink) collectFinishedPlayers() {
	activeCues := s.cueVoices[:0]
	for _, cue := range s.cueVoices {
		if cue.player.IsPlaying() {
			activeCues = append(activeCues, cue)
			continue
		}
		// Oto v3.4 releases its mixer player through a finalizer; Player.Close
		// is deprecated and intentionally a no-op.
	}
	s.cueVoices = activeCues

	retiring := s.retiring[:0]
	for _, player := range s.retiring {
		if player.IsPlaying() {
			retiring = append(retiring, player)
			continue
		}
		// See the corresponding cue-player cleanup above.
	}
	s.retiring = retiring
}

func (s *audioSink) suspend() {
	if s == nil || s.closed {
		return
	}
	if err := s.ctx.Suspend(); err != nil {
		s.log.Warn("native audio suspend failed", "err", err)
	}
}

func (s *audioSink) resume() {
	if s == nil || s.closed {
		return
	}
	if err := s.ctx.Resume(); err != nil {
		s.log.Warn("native audio resume failed", "err", err)
	}
}

func (s *audioSink) close() {
	if s == nil || s.closed {
		return
	}
	// Dropping these references lets oto v3.4 release its mixer players. Its
	// deprecated Player.Close method is intentionally a no-op.
	s.cueVoices = nil
	s.retiring = nil
	s.active = nil
	s.pending = nil
	s.desired = nil
	s.closed = true
}

func clipDuration(clip *clipPCM) float64 {
	if clip == nil || clip.Rate <= 0 {
		return 0
	}
	return float64(len(clip.Samples)/audioChannels) / float64(clip.Rate)
}
