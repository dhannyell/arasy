package dto

import (
	"math"

	"github.com/dhannyell/frameforge/render"
)

type CameraDTO struct {
	Transform   Transform2DDTO `json:"transform"`
	ScalarScale float64        `json:"scalarScale"`
}

type Transform2DDTO struct {
	A float64 `json:"a"`
	// We don't have rotation yet. Defined for compatibility.
	B float64 `json:"b"`
	// We don't have skew yet. Defined for compatibility.
	C float64 `json:"c"`
	D float64 `json:"d"`
	E float64 `json:"e"`
	F float64 `json:"f"`
}

func NewCameraDTO(camera render.Camera) CameraDTO {
	resolved := camera.Resolve()

	world := resolved.World
	viewport := resolved.Viewport

	scaleX := resolved.ScaleX
	scaleY := resolved.ScaleY

	return CameraDTO{
		Transform: Transform2DDTO{
			A: scaleX,
			B: 0,
			C: 0,
			D: scaleY,
			E: viewport.Min.X - world.Min.X*scaleX,
			F: viewport.Min.Y - world.Min.Y*scaleY,
		},
		ScalarScale: math.Min(scaleX, scaleY),
	}
}
