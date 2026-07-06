package web

import "github.com/dhannyell/frameforge/math2d"

type Vec2DTO struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func NewVec2DTO(vec2 math2d.Vec2) Vec2DTO {
	return Vec2DTO{
		X: vec2.X,
		Y: vec2.Y,
	}
}

func NewVec2SliceDTO(vec2 []math2d.Vec2) []*Vec2DTO {
	result := make([]*Vec2DTO, len(vec2))

	for i := range vec2 {
		result[i] = new(NewVec2DTO(vec2[i]))
	}

	return result
}
