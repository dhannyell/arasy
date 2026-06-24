package render

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
)

// AspectMode controls how the camera adapts when World and Viewport have
// different aspect ratios.
type AspectMode int

const (
	// AspectStretch fills the whole viewport and may distort shapes.
	AspectStretch AspectMode = iota

	// AspectFit keeps the whole world visible and may leave empty borders.
	AspectFit

	// AspectExpand keeps the whole viewport filled and expands the visible world.
	AspectExpand
)

// Camera maps world coordinates to viewport coordinates.
type Camera struct {
	// World is the visible area in world coordinates.
	World math2d.Rect

	// Viewport is the target area in screen or image coordinates.
	Viewport math2d.Rect

	// Aspect controls how the camera handles aspect-ratio differences
	Aspect AspectMode
}

// WorldToScreen converts a point from world coordinates to viewport coordinates.
func (c Camera) WorldToScreen(point math2d.Vec2) math2d.Vec2 {
	world, viewport := c.resolveView()

	worldSize := world.Size()
	viewportSize := viewport.Size()

	if worldSize.X == 0 || worldSize.Y == 0 {
		return viewport.Min
	}

	x := (point.X - world.Min.X) / worldSize.X
	y := (point.Y - world.Min.Y) / worldSize.Y

	return math2d.Vec2{
		X: viewport.Min.X + x*viewportSize.X,
		Y: viewport.Min.Y + y*viewportSize.Y,
	}
}

// ScreenToWorld converts a point from viewport coordinates to world coordinates.
func (c Camera) ScreenToWorld(point math2d.Vec2) math2d.Vec2 {
	world, viewport := c.resolveView()

	worldSize := world.Size()
	viewportSize := viewport.Size()

	if viewportSize.X == 0 || viewportSize.Y == 0 {
		return world.Min
	}

	x := (point.X - viewport.Min.X) / viewportSize.X
	y := (point.Y - viewport.Min.Y) / viewportSize.Y

	return math2d.Vec2{
		X: world.Min.X + x*worldSize.X,
		Y: world.Min.Y + y*worldSize.Y,
	}
}

// WorldScalarToScreen converts a scalar world measurement to viewport units.
//
// It uses the smaller axis scale to avoid making scalar values too large
// when world and viewport have different aspect ratios.
func (c Camera) WorldScalarToScreen(value float64) float64 {
	world, viewport := c.resolveView()

	if world.Width() == 0 || world.Height() == 0 {
		return 0
	}

	scaleX := viewport.Width() / world.Width()
	scaleY := viewport.Height() / world.Height()

	if c.Aspect == AspectStretch {
		return value * math.Min(scaleX, scaleY)
	}

	return value * scaleX
}

// ScreenScalarToWorld converts a scalar viewport measurement to world units.
//
// It uses the smaller axis scale to match WorldScalarToScreen.
func (c Camera) ScreenScalarToWorld(value float64) float64 {
	world, viewport := c.resolveView()

	if world.Width() == 0 || world.Height() == 0 {
		return 0
	}

	scaleX := viewport.Width() / world.Width()
	scaleY := viewport.Height() / world.Height()

	scale := scaleX
	if c.Aspect == AspectStretch {
		scale = math.Min(scaleX, scaleY)
	}

	if scale == 0 {
		return 0
	}

	return value / scale
}

func (c Camera) resolveView() (math2d.Rect, math2d.Rect) {
	switch c.Aspect {
	case AspectFit:
		return c.fitView()
	case AspectExpand:
		return c.expandView()
	default:
		return c.World, c.Viewport
	}
}

func (c Camera) fitView() (math2d.Rect, math2d.Rect) {
	world := c.World
	viewport := c.Viewport

	if world.Width() == 0 || world.Height() == 0 {
		return world, viewport
	}

	scaleX := viewport.Width() / world.Width()
	scaleY := viewport.Height() / world.Height()
	scale := math.Min(scaleX, scaleY)

	usedWidth := world.Width() * scale
	usedHeight := world.Height() * scale

	offsetX := (viewport.Width() - usedWidth) / 2
	offsetY := (viewport.Height() - usedHeight) / 2

	viewport.Min.X += offsetX
	viewport.Min.Y += offsetY
	viewport.Max.X = viewport.Min.X + usedWidth
	viewport.Max.Y = viewport.Min.Y + usedHeight

	return world, viewport
}

func (c Camera) expandView() (math2d.Rect, math2d.Rect) {
	world := c.World
	viewport := c.Viewport

	if world.Width() == 0 || world.Height() == 0 {
		return world, viewport
	}

	worldAspect := world.Width() / world.Height()
	viewportAspect := viewport.Width() / viewport.Height()

	if worldAspect < viewportAspect {
		newWidth := world.Height() * viewportAspect
		extra := newWidth - world.Width()

		world.Min.X -= extra / 2
		world.Max.X += extra / 2
	} else {
		newHeight := world.Width() / viewportAspect
		extra := newHeight - world.Height()

		world.Min.Y -= extra / 2
		world.Max.Y += extra / 2
	}

	return world, viewport
}
