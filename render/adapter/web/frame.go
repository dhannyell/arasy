package web

import (
	"github.com/dhannyell/frameforge/render"
)

type FrameDTO struct {
	Type     string       `json:"type"`
	Tick     uint64       `json:"tick"`
	Camera   CameraDTO    `json:"camera"`
	Clear    ColorDTO     `json:"clear"`
	Commands []CommandDTO `json:"commands"`
}

func NewFrameDTO(
	tick uint64,
	camera render.Camera,
	clear render.Color,
	commands []render.Command,
) FrameDTO {
	return FrameDTO{
		Type:     "frame",
		Tick:     tick,
		Camera:   NewCameraDTO(camera),
		Clear:    NewColorDTO(clear),
		Commands: NewCommandDTOs(commands),
	}
}
