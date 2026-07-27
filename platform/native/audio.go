//go:build windows && amd64

package native

import (
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"sync"
	"time"

	"github.com/dhannyell/arasy/audio"
	"github.com/ebitengine/oto/v3"
)

const bytesPerAudioFrame = audioChannels * 2 // FormatSignedInt16LE

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

type trackPlayer interface {
	Play()
	IsPlaying() bool
	BufferedSize() int
	Seek(offset int64, whence int) (int64, error)
}

type audioSink struct {
	ctx              *oto.Context
	clips            map[audio.ClipID]*clipPCM
	active           map[audio.TrackID]*activeTrack
	pending          map[audio.TrackID]pendingTrack
	desired          map[audio.TrackID]audio.Track
	cueVoices        []*cueVoice
	retiring         []trackPlayer
	log              *slog.Logger
	start            time.Time
	tick             uint64
	alpha            float64
	tickSec          float64
	closed           bool
	newPlayer        func(io.Reader) trackPlayer
	simulationPaused bool
	unfocused        bool
	contextSuspended bool
}

type cueVoice struct {
	clip   audio.ClipID
	player trackPlayer
}

type activeTrack struct {
	player trackPlayer
	voice  *voice
	clip   audio.ClipID
	start  float64
	frames int
	loop   bool
	paused bool
}

type pendingTrack struct {
	clip  audio.ClipID
	start float64
}

func (s *audioSink) player(r io.Reader) trackPlayer {
	if s.newPlayer != nil {
		return s.newPlayer(r)
	}
	return s.ctx.NewPlayer(r)
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

// audiblePlayhead reports the position the device is actually playing, in
// seconds from the start of the clip. The reader runs ahead of the device by
// the player's buffered bytes (half a second by default), so that read-ahead
// is subtracted; a negative result means the reader already wrapped past the
// loop point while the buffer still holds the tail.
func (t *activeTrack) audiblePlayhead(rate int) (float64, bool) {
	if t.frames == 0 || rate <= 0 {
		return 0, false
	}
	audible := t.voice.position() - t.player.BufferedSize()/bytesPerAudioFrame
	if audible < 0 {
		if !t.loop {
			return 0, false
		}
		audible += t.frames
	}
	return float64(audible) / float64(rate), true
}

// resyncTrack repositions an active track. Seek clears the player's buffered
// read-ahead, then gain is rebuilt from zero to avoid a click on restart.
func (t *activeTrack) resyncTrack(offset, gain float64, rate int) {
	byteOffset := int64(offset * float64(rate*bytesPerAudioFrame))
	_, err := t.player.Seek(byteOffset, io.SeekStart)
	if err != nil {
		return
	}
	t.voice.setGain(0, 0, rate)
	t.voice.setGain(gain, audio.DefaultResyncFadeSec, rate)
	t.player.Play()
	t.paused = false
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
		player := s.player(v)
		player.Play()
		s.cueVoices = append(s.cueVoices, &cueVoice{clip: cue.Clip, player: player})
	}
}

func (s *audioSink) reconcileTracks() {
	now := time.Since(s.start).Seconds()
	for id, active := range s.active {
		desired, isDesired := s.desired[id]
		if !isDesired {
			active.voice.fadeOut(0, audioSampleRate)
			s.retiring = append(s.retiring, active.player)
			delete(s.active, id)
			continue
		}
		if active.clip != desired.Clip {
			continue
		}
		if desired.Paused {
			s.pauseTrack(active, desired.FadeSec)
			continue
		}

		duration := clipDuration(s.clips[desired.Clip])
		expected, state := audio.ExpectedTrackPlayhead(
			float64(s.tick),
			s.alpha,
			desired,
			s.tickSec,
			duration,
		)

		switch state {
		case audio.PlayheadUnanchored:
			active.voice.setGain(desired.Gain, desired.FadeSec, audioSampleRate)
		case audio.PlayheadPending:
			active.voice.fadeOut(0, audioSampleRate)
			s.retiring = append(s.retiring, active.player)
			delete(s.active, id)
		case audio.PlayheadFinished:
			active.voice.fadeOut(0, audioSampleRate)
			s.retiring = append(s.retiring, active.player)
			delete(s.active, id)
		case audio.PlayheadPlaying:
			if active.paused {
				active.resyncTrack(expected, desired.Gain, audioSampleRate)
				continue
			}
			actual, ok := active.audiblePlayhead(audioSampleRate)
			if !ok {
				active.voice.setGain(desired.Gain, desired.FadeSec, audioSampleRate)
				continue
			}
			isResync := audio.NeedsResync(actual, expected, duration, audio.DefaultResyncToleranceSec, active.loop)
			if isResync {
				active.resyncTrack(expected, desired.Gain, audioSampleRate)
				continue
			}
			active.voice.setGain(desired.Gain, desired.FadeSec, audioSampleRate)
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
			if track.Paused {
				continue
			}
			current.voice.setGain(track.Gain, track.FadeSec, audioSampleRate)
			continue
		}
		if active {
			current.voice.fadeOut(track.FadeSec, audioSampleRate)
			s.retiring = append(s.retiring, current.player)
			delete(s.active, id)
		}

		waiting := pendingTrack{
			clip:  track.Clip,
			start: audio.ScheduledTrackStart(now, s.tick, s.alpha, track.StartTick, s.tickSec),
		}
		s.pending[id] = waiting

		clip, ok := s.clips[track.Clip]
		if !ok {
			continue
		}

		// Unlike Web Audio, oto has no scheduled start: hold the track in pending
		// until its start instant arrives.
		if now < waiting.start {
			continue
		}

		start, offset, expired := audio.ScheduledPlayback(now, waiting.start, clipDuration(clip), track.Loop)
		if expired {
			continue
		}

		v := newVoice(clip.Samples, track.Loop, 0, 0)
		v.setPosition(int(offset * audioSampleRate))
		v.setGain(track.Gain, track.FadeSec, audioSampleRate)
		player := s.player(v)
		player.Play()
		s.active[id] = &activeTrack{
			player: player,
			voice:  v,
			clip:   track.Clip,
			start:  start,
			frames: len(clip.Samples) / audioChannels,
			loop:   track.Loop,
		}
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

func (s *audioSink) pauseTrack(track *activeTrack, fadeSec float64) {
	if track.paused {
		return
	}

	track.voice.setGain(0, fadeSec, audioSampleRate)
	track.paused = true
}

func (s *audioSink) setAudioContextState(paused bool) {
	if s == nil || s.closed || s.simulationPaused == paused {
		return
	}
	s.simulationPaused = paused
	s.syncAudioContext()
}

func (s *audioSink) setFocused(focused bool) {
	if s == nil || s.closed || s.unfocused == !focused {
		return
	}
	s.unfocused = !focused
	s.syncAudioContext()
}

func (s *audioSink) syncAudioContext() {
	shouldSuspend := s.simulationPaused || s.unfocused
	if s.contextSuspended == shouldSuspend {
		return
	}
	s.contextSuspended = shouldSuspend

	if shouldSuspend {
		s.suspend()
		return
	}
	s.resume()
}

func clipDuration(clip *clipPCM) float64 {
	if clip == nil || clip.Rate <= 0 {
		return 0
	}
	return float64(len(clip.Samples)/audioChannels) / float64(clip.Rate)
}
