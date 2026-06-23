package render

import "github.com/dhannyell/frameforge/math2d"

// Camera maps world coordinates to viewport coordinates.
type Camera struct {
	World    math2d.Rect
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

func (c Camera) sizes() (worldSize math2d.Vec2, viewportSize math2d.Vec2) {
	return c.World.Size(), c.Viewport.Size()
}
