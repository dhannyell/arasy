package render

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
)

// Camera maps world coordinates to viewport coordinates.
type Camera struct {
	// World first vector represents the initial position and the second the size
	World math2d.Rect
	// Viewport first vector represents the initial position and the second the size
	Viewport math2d.Rect
}

// WorldToScreen converts a point from world coordinates to viewport coordinates.
func (c Camera) WorldToScreen(point math2d.Vec2) math2d.Vec2 {
	worldSize, viewportSize := c.sizes()

	x := (point.X - c.World.Min.X) / worldSize.X
	y := (point.Y - c.World.Min.Y) / worldSize.Y

	return math2d.Vec2{
		X: c.Viewport.Min.X + x*viewportSize.X,
		Y: c.Viewport.Min.Y + y*viewportSize.Y,
	}
}

// ScreenToWorld converts a point from viewport coordinates to world coordinates.
func (c Camera) ScreenToWorld(point math2d.Vec2) math2d.Vec2 {
	worldSize, viewportSize := c.sizes()

	x := (point.X - c.Viewport.Min.X) / viewportSize.X
	y := (point.Y - c.Viewport.Min.Y) / viewportSize.Y

	return math2d.Vec2{
		X: c.World.Min.X + x*worldSize.X,
		Y: c.World.Min.Y + y*worldSize.Y,
	}
}

// WorldScalarToScreen converts a scalar world measurement to viewport units.
//
// It uses the horizontal camera scale. This is suitable when the world and
// viewport have matching aspect ratios.
func (c Camera) WorldScalarToScreen(value float64) float64 {
	if c.World.Width() == 0 || c.World.Height() == 0 {
		return 0
	}

	scaleX := c.Viewport.Width() / c.World.Width()
	scaleY := c.Viewport.Height() / c.World.Height()

	return value * math.Min(scaleX, scaleY)
}

// ScreenScalarToWorld converts a scalar viewport measurement to world units.
//
// It uses the horizontal camera scale.
func (c Camera) ScreenScalarToWorld(value float64) float64 {
	scaleX := c.Viewport.Width() / c.World.Width()
	scaleY := c.Viewport.Height() / c.World.Height()

	scale := math.Min(scaleX, scaleY)
	if scale == 0 {
		return 0
	}

	return value / scale
}

func (c Camera) sizes() (worldSize math2d.Vec2, viewportSize math2d.Vec2) {
	return c.World.Size(), c.Viewport.Size()
}
