package audio_test

import (
	"testing"

	"github.com/dhannyell/frameforge/audio"
)

func TestStateClone(t *testing.T) {
	tracks := make(map[audio.TrackID]audio.Track, 1)
	tracks["original"] = audio.Track{
		Clip: "original_clip",
		Gain: 1.0,
	}
	originalState := audio.State{Tracks: tracks}
	cloneState := originalState.Clone()

	tracks["music"] = audio.Track{Gain: 2.0}

	if cloneState.Tracks["original"].Gain != 1.0 {
		t.Fatalf("clone was mutated by original")
	}
}
