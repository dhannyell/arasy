package dto

import (
	"github.com/dhannyell/frameforge/render"
)

type ColorDTO struct {
	R float64 `json:"r"`
	G float64 `json:"g"`
	B float64 `json:"b"`
	A float64 `json:"a"`
}

func NewColorDTO(color render.Color) ColorDTO {
	return ColorDTO{
		R: color.R,
		G: color.G,
		B: color.B,
		A: color.A,
	}
}

func newOptionalColorDTO(color render.Color) *ColorDTO {
	if color == (render.Color{}) {
		return nil
	}

	return new(NewColorDTO(color))
}
