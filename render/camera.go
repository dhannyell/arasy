package render

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
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

// ResolvedCamera contains the camera values used for one render pass.
type ResolvedCamera struct {
	World    math2d.Rect
	Viewport math2d.Rect
	ScaleX   float64
	ScaleY   float64
}

// Camera maps world coordinates to viewport coordinates.
type Camera struct {
	// World is the visible area in world coordinates.
	World math2d.Rect

	// Viewport is the target area in screen or image coordinates.
	Viewport math2d.Rect

	// Aspect controls how the camera handles aspect-ratio differences.
	Aspect AspectMode

	// DepthScale controls how much world Y contributes to screen position.
	// Use it for a tilted top-down or dimetric view. Zero uses 1. Values above
	// 1 and NaN also use 1. Smaller values are clamped to a usable minimum.
	// The setting changes rendering only. World units and simulation stay the
	// same.
	DepthScale float64

	// View maps world coordinates before DepthScale. Use it when the world
	// layout and the screen layout differ, such as for an isometric grid. The
	// zero value is identity. ScreenToWorld applies the inverse mapping.
	// Render backends use View for batch positions. Sprite, glyph and particle
	// corners stay screen-space billboards. DrawParticles still ignores
	// BaseCommand.Transform; this is a command rule, not a camera rule.
	View math2d.Affine
}

// Skews reports whether View does anything, so a backend can route shape
// geometry through its per-vertex path. A sheared shape cannot be represented
// by its centre and axis-aligned size alone.
func (c Camera) Skews() bool {
	return !c.View.IsIdentity()
}

// minDepthScale keeps a degenerate DepthScale from collapsing the view to a
// line and from making the inverse mapping blow up. Anything flatter than
// this is already unusable on screen.
const minDepthScale = 1e-4

// depthScale resolves DepthScale into the factor the mapping actually uses.
//
// Zero means 1 rather than "flat as a line": a Camera{} predates this field
// and has to keep meaning an untilted, top-down camera.
func (c Camera) depthScale() float64 {
	return clampDepthScale(c.DepthScale)
}

// clampDepthScale is shared with Projection so the camera and the sprites that
// stand up against it can never disagree on what a given value means.
func clampDepthScale(depthScale float64) float64 {
	if depthScale == 0 || depthScale > 1 || math.IsNaN(depthScale) {
		return 1
	}

	if depthScale < minDepthScale {
		return minDepthScale
	}

	return depthScale
}

// WorldToScreen converts a point from world coordinates to viewport coordinates.
func (c Camera) WorldToScreen(point math2d.Vec2) math2d.Vec2 {
	world, viewport := c.resolveView()

	worldSize := world.Size()
	viewportSize := viewport.Size()

	if worldSize.X == 0 || worldSize.Y == 0 {
		return viewport.Min
	}

	// The view affine comes first, so World and Viewport keep framing what
	// ends up on screen rather than what the game happens to store.
	point = c.View.Apply(point)

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

	inView := math2d.Vec2{
		X: world.Min.X + x*worldSize.X,
		Y: world.Min.Y + y*worldSize.Y,
	}

	// Undo the view affine last, so the pair stays an exact round trip. A
	// singular view has no inverse; the point is already as far back as it
	// goes.
	if inverse, ok := c.View.Inverse(); ok {
		return inverse.Apply(inView)
	}

	return inView
}

// WorldScalarToScreen converts a scalar world measurement to viewport units.
//
// It uses the smaller axis scale to avoid making scalar values too large
// when world and viewport have different aspect ratios.
//
// DepthScale does not apply: a scalar has no axis, and foreshortening only
// means something along one. A radius or a stroke width converts the same
// whatever the camera's tilt.
func (c Camera) WorldScalarToScreen(value float64) float64 {
	world, viewport := c.resolveAspect()

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

// WorldRadiiToScreen converts a shape's world-space radii into the screen
// radii it draws with.
//
// Unlike WorldScalarToScreen this is anisotropic, because a radius does have
// an axis: it is the geometry of something lying in the world, and a tilted
// camera sees the world's depth axis foreshortened. A circle on the ground is
// an ellipse on screen. Stroke widths and corner radii are genuinely axis-less
// and keep using WorldScalarToScreen.
func (c Camera) WorldRadiiToScreen(radii math2d.Vec2) math2d.Vec2 {
	return math2d.Vec2{
		X: c.WorldScalarToScreen(radii.X),
		Y: c.WorldScalarToScreen(radii.Y) * c.depthScale(),
	}
}

// Foreshortens reports whether the camera's tilt changes the shape of world
// geometry and not merely where it lands. Backends use it to pick the
// axis-aware drawing path for shapes that would otherwise be drawn from a
// single radius.
func (c Camera) Foreshortens() bool {
	return c.depthScale() != 1
}

// ScreenScalarToWorld converts a scalar viewport measurement to world units.
//
// It uses the smaller axis scale to match WorldScalarToScreen, and likewise
// ignores DepthScale.
func (c Camera) ScreenScalarToWorld(value float64) float64 {
	world, viewport := c.resolveAspect()

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

// resolveView returns the world and viewport rects every point mapping goes
// through: the aspect-adjusted view, then foreshortened by DepthScale.
func (c Camera) resolveView() (math2d.Rect, math2d.Rect) {
	world, viewport := c.resolveAspect()

	return c.foreshortenWorld(world), viewport
}

// foreshortenWorld widens the visible depth range by the inverse of the depth
// scale, around the view's centre. Growing the world the camera sees and
// squashing what it draws towards the centre are the same transform read from
// either end, and doing it here means every consumer of the view -- point
// mapping, culling, the backends' projection matrices -- foreshortens the same
// way for free.
func (c Camera) foreshortenWorld(world math2d.Rect) math2d.Rect {
	scale := c.depthScale()
	if scale == 1 {
		return world
	}

	centerY := world.Center().Y
	halfHeight := world.Height() / 2 / scale

	world.Min.Y = centerY - halfHeight
	world.Max.Y = centerY + halfHeight

	return world
}

func (c Camera) resolveAspect() (math2d.Rect, math2d.Rect) {
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

// Resolve returns the aspect-adjusted camera and scale.
func (c Camera) Resolve() ResolvedCamera {
	world, viewport := c.resolveView()

	scaleX := 0.0
	scaleY := 0.0

	if world.Width() != 0 {
		scaleX = viewport.Width() / world.Width()
	}

	if world.Height() != 0 {
		scaleY = viewport.Height() / world.Height()
	}

	return ResolvedCamera{
		World:    world,
		Viewport: viewport,
		ScaleX:   scaleX,
		ScaleY:   scaleY,
	}
}
