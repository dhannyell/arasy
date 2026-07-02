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

type BaseSprite struct {
	Layer int
	Color render.Color
}

type CircleSprite struct {
	BaseSprite
	Radius float64
}

type RectSprite struct {
	BaseSprite
	Rect        math2d.Rect
	Stroke      render.Color
	StrokeWidth float64
}

type Velocity struct {
	Linear math2d.Vec2
}

type Acceleration struct {
	Linear math2d.Vec2
}

type Damping struct {
	Linear float64
}

type Body struct {
	// InvMass is the inverse mass. A value of 0 makes the body static.
	InvMass float64
	// Restitution controls bounciness. 0 absorbs normal velocity, 1 preserves it.
	Restitution float64
}

type CircleCollider struct {
	Radius float64
}

type RectCollider struct {
	// HalfExtents are the half-dimensions of the rectangle, centered at
	// Transform.Position. Transform.Rotation is ignored (AABB).
	HalfExtends math2d.Vec2
}
