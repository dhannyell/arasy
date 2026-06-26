package web

import "github.com/dhannyell/frameforge/math2d"

type RectDTO struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func NewRectDTO(rect math2d.Rect) RectDTO {
	return RectDTO{
		X:      rect.Min.X,
		Y:      rect.Min.Y,
		Width:  rect.Width(),
		Height: rect.Height(),
	}
}
