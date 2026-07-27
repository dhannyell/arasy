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
	reader io.ReadSeeker
	seeks  []int64
}

func (p *fakeTrackPlayer) Play()             {}
func (p *fakeTrackPlayer) IsPlaying() bool   { return true }
func (p *fakeTrackPlayer) BufferedSize() int { return 0 }
func (p *fakeTrackPlayer) Seek(offset int64, whence int) (int64, error) {
	p.seeks = append(p.seeks, offset)
	return p.reader.Seek(offset, whence)
}

func TestAudioSinkResyncsScheduledTrackThroughPlayerSeek(t *testing.T) {
	clipID := audio.ClipID("music")
	clip := &clipPCM{Rate: audioSampleRate, Samples: make([]int16, audioSampleRate*audioChannels*8)}
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
	track := audio.Track{Clip: clipID, Loop: true, StartTick: 2}

	// The late first observation starts one second into the track.
	sink.consume(audio.Output{Tick: 3, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	if len(players) != 1 {
		t.Fatalf("players = %d, want 1", len(players))
	}
	// A continuous timeline leaves the player at its expected playhead.
	sink.consume(audio.Output{Tick: 3, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	if len(players[0].seeks) != 0 {
		t.Fatalf("continuous playback seeks = %v, want none", players[0].seeks)
	}

	// At tick 5, the desired playhead is three seconds. The native sink must
	// seek the Oto player (not reset it), which also clears its read-ahead.
	sink.consume(audio.Output{Tick: 5, Tracks: map[audio.TrackID]audio.Track{"music": track}})
	want := int64(3 * audioSampleRate * bytesPerAudioFrame)
	if len(players[0].seeks) != 1 || players[0].seeks[0] != want {
		t.Fatalf("seek offsets = %v, want [%d]", players[0].seeks, want)
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
