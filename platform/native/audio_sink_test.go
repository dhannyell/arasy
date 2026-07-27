//go:build windows && amd64

package native

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dhannyell/arasy/audio"
)

type fakeTrackPlayer struct {
	reader   io.ReadSeeker
	seeks    []int64
	buffered int
}

func (p *fakeTrackPlayer) Play()             {}
func (p *fakeTrackPlayer) IsPlaying() bool   { return true }
func (p *fakeTrackPlayer) BufferedSize() int { return p.buffered }
func (p *fakeTrackPlayer) Seek(offset int64, whence int) (int64, error) {
	p.seeks = append(p.seeks, offset)
	return p.reader.Seek(offset, whence)
}

func newTrackTestSink(seconds float64) (*audioSink, *[]*fakeTrackPlayer) {
	clipID := audio.ClipID("music")
	clip := &clipPCM{
		Rate:    audioSampleRate,
		Samples: make([]int16, int(seconds*audioSampleRate)*audioChannels),
	}
	var players []*fakeTrackPlayer
	sink := &audioSink{
		clips:   map[audio.ClipID]*clipPCM{clipID: clip},
		active:  make(map[audio.TrackID]*activeTrack),
		pending: make(map[audio.TrackID]pendingTrack),
		desired: make(map[audio.TrackID]audio.Track),
		log:     slog.New(slog.DiscardHandler),
		start:   time.Now(),
		tickSec: 1,
		newPlayer: func(reader io.Reader) trackPlayer {
			player := &fakeTrackPlayer{reader: reader.(io.ReadSeeker)}
			players = append(players, player)
			return player
		},
	}
	return sink, &players
}

func TestActiveTrackAudiblePlayheadSubtractsBufferedFrames(t *testing.T) {
	v := newVoice(make([]int16, 1000*audioChannels), false, 0, 1)
	v.setPosition(700)
	track := activeTrack{
		player: &fakeTrackPlayer{buffered: 200 * bytesPerAudioFrame},
		voice:  v,
		frames: 1000,
	}

	got, ok := track.audiblePlayhead(1000)
	if !ok || got != 0.5 {
		t.Fatalf("audiblePlayhead() = (%v, %t), want (0.5, true)", got, ok)
	}
}

func TestActiveTrackAudiblePlayheadWrapsMultipleBufferedLoops(t *testing.T) {
	v := newVoice(make([]int16, 100*audioChannels), true, 0, 1)
	v.setPosition(20)
	track := activeTrack{
		player: &fakeTrackPlayer{buffered: 250 * bytesPerAudioFrame},
		voice:  v,
		frames: 100,
		loop:   true,
	}

	got, ok := track.audiblePlayhead(1000)
	if !ok || got != 0.07 {
		t.Fatalf("audiblePlayhead() = (%v, %t), want (0.07, true)", got, ok)
	}
}

func TestAudioSinkResyncsBackwardScrubThroughPlayerSeek(t *testing.T) {
	sink, players := newTrackTestSink(8)
	track := audio.Track{Clip: "music", Loop: true, StartTick: 2}

	// The late first observation starts three seconds into the track.
	sink.consume(audio.Output{Tick: 5, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	if len(*players) != 1 {
		t.Fatalf("players = %d, want 1", len(*players))
	}

	// Scrubbing back to tick 3 moves the desired playhead to one second.
	sink.consume(audio.Output{Tick: 3, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	want := int64(audioSampleRate * bytesPerAudioFrame)
	if len((*players)[0].seeks) != 1 || (*players)[0].seeks[0] != want {
		t.Fatalf("seek offsets = %v, want [%d]", (*players)[0].seeks, want)
	}
}

func TestAudioSinkContinuousPlaybackDoesNotSeek(t *testing.T) {
	sink, players := newTrackTestSink(8)
	track := audio.Track{Clip: "music", Loop: true, StartTick: 2}

	sink.consume(audio.Output{Tick: 3, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	if len(*players) != 1 {
		t.Fatalf("players = %d, want 1", len(*players))
	}

	// Advance the audible reader and both clocks by one second.
	sink.active["music"].voice.setPosition(2 * audioSampleRate)
	sink.start = sink.start.Add(-time.Second)
	sink.consume(audio.Output{Tick: 4, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	if len((*players)[0].seeks) != 0 {
		t.Fatalf("continuous playback seeks = %v, want none", (*players)[0].seeks)
	}
}

func TestAudioSinkRemovesFinishedOneShot(t *testing.T) {
	sink, _ := newTrackTestSink(2)
	track := audio.Track{Clip: "music", StartTick: 1}

	sink.consume(audio.Output{Tick: 1, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	if _, ok := sink.active["music"]; !ok {
		t.Fatal("track did not become active")
	}

	sink.consume(audio.Output{Tick: 4, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	if _, ok := sink.active["music"]; ok {
		t.Fatal("finished one-shot remains active")
	}
}

func TestAudioSinkReturnsActiveTrackToPendingAfterBackwardScrub(t *testing.T) {
	sink, _ := newTrackTestSink(8)
	track := audio.Track{Clip: "music", Loop: true, StartTick: 2}

	sink.consume(audio.Output{Tick: 3, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	if _, ok := sink.active["music"]; !ok {
		t.Fatal("track did not become active")
	}

	sink.consume(audio.Output{Tick: 0, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	if _, ok := sink.active["music"]; ok {
		t.Fatal("pending track remains active after backward scrub")
	}
	pending, ok := sink.pending["music"]
	if !ok || pending.clip != "music" {
		t.Fatalf("pending track = (%v, %t), want music", pending, ok)
	}
}

func TestAudioSinkWaitsForFutureTrackStart(t *testing.T) {
	clipID := audio.ClipID("music")
	var playerCount int
	sink := &audioSink{
		clips:   map[audio.ClipID]*clipPCM{clipID: {Rate: audioSampleRate, Samples: make([]int16, audioSampleRate*audioChannels*4)}},
		active:  make(map[audio.TrackID]*activeTrack),
		pending: make(map[audio.TrackID]pendingTrack),
		desired: make(map[audio.TrackID]audio.Track),
		log:     slog.New(slog.DiscardHandler),
		start:   time.Now(),
		tickSec: 1,
		newPlayer: func(io.Reader) trackPlayer {
			playerCount++
			return &fakeTrackPlayer{reader: newVoice(nil, false, 0, 0)}
		},
	}
	track := audio.Track{Clip: clipID, Loop: true, StartTick: 2}

	sink.consume(audio.Output{Tick: 0, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	if playerCount != 0 || sink.pending["music"].clip != clipID {
		t.Fatalf("future track created early: players=%d pending=%v", playerCount, sink.pending)
	}

	// Advance the sink clock to the requested start tick. Oto cannot schedule a
	// player, so the track must only be created when this reconcile observes it.
	sink.start = time.Now().Add(-2 * time.Second)
	sink.consume(audio.Output{Tick: 2, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	if playerCount != 1 {
		t.Fatalf("players at scheduled start = %d, want 1", playerCount)
	}
}

func TestAudioSinkLimitsCueVoices(t *testing.T) {
	sink := &audioSink{}
	clip := audio.ClipID("hit")
	for range maxCueVoicesClip {
		sink.cueVoices = append(sink.cueVoices, &cueVoice{clip: clip})
	}
	if sink.canPlayCue(clip) {
		t.Errorf("canPlayCue(%q) = true with %d active voices, want false", clip, maxCueVoicesClip)
	}
	if !sink.canPlayCue("other") {
		t.Error("canPlayCue(other) = false below the global cap, want true")
	}

	sink.cueVoices = nil
	for range maxCueVoices {
		sink.cueVoices = append(sink.cueVoices, &cueVoice{clip: "other"})
	}
	if sink.canPlayCue(clip) {
		t.Errorf("canPlayCue(%q) = true with %d active voices, want false", clip, maxCueVoices)
	}
}
