package canvas

import (
	"maps"

	"github.com/dhannyell/frameforge/audio"
)

type audioManifestDTO struct {
	Type  string         `json:"type"`
	Clips []audioClipDTO `json:"clips"`
}

type audioClipDTO struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type audioMessageDTO struct {
	Type   string                   `json:"type"`
	Cues   []audioCueDTO            `json:"cues,omitempty"`
	Tracks map[string]audioTrackDTO `json:"tracks"`
}

type audioCueDTO struct {
	Tick uint64  `json:"tick"`
	Seq  uint32  `json:"seq"`
	Clip string  `json:"clip"`
	Gain float64 `json:"gain"`
	Pan  float64 `json:"pan"`
}

type audioTrackDTO struct {
	Clip    string  `json:"clip"`
	Gain    float64 `json:"gain"`
	Loop    bool    `json:"loop"`
	FadeSec float64 `json:"fadeSec"`
}

func newAudioManifestDTO(clips []audio.Clip) audioManifestDTO {
	dto := audioManifestDTO{
		Type:  "audioManifest",
		Clips: make([]audioClipDTO, 0, len(clips)),
	}
	for _, clip := range clips {
		dto.Clips = append(dto.Clips, audioClipDTO{
			ID:  string(clip.ID),
			URL: "/audio/" + clip.Path,
		})
	}
	return dto
}

func newAudioMessageDTO(out audio.Output) audioMessageDTO {
	dto := audioMessageDTO{
		Type:   "audio",
		Cues:   make([]audioCueDTO, 0, len(out.Cues)),
		Tracks: make(map[string]audioTrackDTO, len(out.Tracks)),
	}
	for _, cue := range out.Cues {
		// Gain is already normalized by the runtime (AudioPort); pass it through.
		dto.Cues = append(dto.Cues, audioCueDTO{
			Tick: cue.Tick,
			Seq:  cue.Seq,
			Clip: string(cue.Clip),
			Gain: cue.Gain,
			Pan:  cue.Pan,
		})
	}
	for id, track := range out.Tracks {
		dto.Tracks[string(id)] = audioTrackDTO{
			Clip:    string(track.Clip),
			Gain:    track.Gain,
			Loop:    track.Loop,
			FadeSec: track.FadeSec,
		}
	}
	return dto
}

func cloneAudioTrackDTOs(src map[string]audioTrackDTO) map[string]audioTrackDTO {
	return maps.Clone(src)
}
