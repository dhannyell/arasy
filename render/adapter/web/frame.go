package web

import (
	"github.com/dhannyell/frameforge/render"
)

type FrameDTO struct {
	Type     string       `json:"type"`
	Tick     uint64       `json:"tick"`
	Camera   CameraDTO    `json:"camera"`
	Clear    ColorDTO     `json:"clear"`
	State    StateDTO     `json:"state"`
	Commands []CommandDTO `json:"commands"`
}

type StateDTO struct {
	Paused           bool    `json:"paused"`
	Speed            float64 `json:"speed"`
	Debug            uint32  `json:"debug"`
	HistoryLen       int     `json:"historyLen"`
	HistoryStartTick uint64  `json:"historyStartTick"`
	HistoryEndTick   uint64  `json:"historyEndTick"`

	RenderFPS    int `json:"renderFPS"`
	SimulationHz int `json:"simulationHz"`
}

func NewFrameDTO(
	tick uint64,
	camera render.Camera,
	clear render.Color,
	commands []render.Command,
) FrameDTO {
	return NewFrameDTOWithState(tick, camera, clear, StateDTO{}, commands)
}

func NewFrameDTOWithState(
	tick uint64,
	camera render.Camera,
	clear render.Color,
	state StateDTO,
	commands []render.Command,
) FrameDTO {
	return FrameDTO{
		Type:     "frame",
		Tick:     tick,
		Camera:   NewCameraDTO(camera),
		Clear:    NewColorDTO(clear),
		State:    state,
		Commands: NewCommandDTOs(commands),
	}
}
