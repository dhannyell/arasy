package scene

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

type EntityID uint32

type Transform struct {
	Position math2d.Vec2
	Rotation float64
	Scale    math2d.Vec2
}

type CircleSprite struct {
	Radius float64
	Color  render.Color
	Layer  int
}

type Velocity struct {
	Linear math2d.Vec2
}

type CircleCollider struct {
	Radius float64
}
