package canvas

import (
	"testing"

	"github.com/dhannyell/frameforge/audio"
)

func TestAudioManifestDTO(t *testing.T) {
	dto := newAudioManifestDTO([]audio.Clip{
		{ID: "hit", Path: "sfx/hit.wav"},
		{ID: "music", Path: "music.wav"},
	})

	if dto.Type != "audioManifest" {
		t.Fatalf("type = %q, want audioManifest", dto.Type)
	}
	if len(dto.Clips) != 2 {
		t.Fatalf("clips len = %d, want 2", len(dto.Clips))
	}
	if dto.Clips[0].ID != "hit" || dto.Clips[0].URL != "/audio/sfx/hit.wav" {
		t.Fatalf("clip[0] = %+v, want hit at /audio/sfx/hit.wav", dto.Clips[0])
	}
	if dto.Clips[1].ID != "music" || dto.Clips[1].URL != "/audio/music.wav" {
		t.Fatalf("clip[1] = %+v, want music at /audio/music.wav", dto.Clips[1])
	}
}

func TestAudioMessageDTO(t *testing.T) {
	// Gain arrives already normalized from the runtime AudioPort; the DTO is a
	// pure pass-through and must not re-default or clamp it.
	dto := newAudioMessageDTO(audio.Output{
		Cues: []audio.Cue{
			{Tick: 7, Seq: 2, Clip: "hit", Gain: 0.5, Pan: -0.25},
		},
		Tracks: map[audio.TrackID]audio.Track{
			"music": {Clip: "song", Gain: 0.5, Loop: true, FadeSec: 0.5},
		},
	})

	if dto.Type != "audio" {
		t.Fatalf("type = %q, want audio", dto.Type)
	}
	if len(dto.Cues) != 1 {
		t.Fatalf("cues len = %d, want 1", len(dto.Cues))
	}
	cue := dto.Cues[0]
	if cue.Tick != 7 || cue.Seq != 2 || cue.Clip != "hit" || cue.Gain != 0.5 || cue.Pan != -0.25 {
		t.Fatalf("cue = %+v, want pass-through gain cue", cue)
	}

	track, ok := dto.Tracks["music"]
	if !ok {
		t.Fatalf("missing music track")
	}
	if track.Clip != "song" || track.Gain != 0.5 || !track.Loop || track.FadeSec != 0.5 {
		t.Fatalf("track = %+v, want pass-through looping song", track)
	}
}
