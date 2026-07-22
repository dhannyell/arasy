//go:build windows && amd64

package native

import (
	"testing"

	"github.com/dhannyell/frameforge/audio"
)

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
