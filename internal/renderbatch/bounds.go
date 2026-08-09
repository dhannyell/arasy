package renderbatch

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// IsFiniteRect reports whether every coordinate of r is finite (no NaN or
// Inf). Backends drop instances with non-finite geometry, logging a
// one-time warning per batch.
func IsFiniteRect(r math2d.Rect) bool {
	return isFinite(r.Min.X) && isFinite(r.Min.Y) && isFinite(r.Max.X) && isFinite(r.Max.Y)
}

// IsFiniteSpriteGeometry reports whether every value that affects the
// sprite's destination geometry is finite. A non-finite rotation would feed
// NaNs to CPU transforms and GPU storage records even when Dest itself is
// valid.
func IsFiniteSpriteGeometry(inst render.SpriteInstance) bool {
	return IsFiniteRect(inst.Dest) && isFinite(inst.Rotation)
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// SpriteBounds returns the world-space axis-aligned bounding box of inst,
// including Rotation: the four Dest corners are rotated around the Dest
// center. ok is false when Dest is non-finite, in which case the returned
// rect is meaningless and the adapter must drop the instance.
func SpriteBounds(inst render.SpriteInstance) (bounds math2d.Rect, ok bool) {
	if !IsFiniteSpriteGeometry(inst) {
		return math2d.Rect{}, false
	}
	center := inst.Dest.Center()
	sin, cos := math.Sin(inst.Rotation), math.Cos(inst.Rotation)
	corners := [4]math2d.Vec2{
		inst.Dest.Min,
		{X: inst.Dest.Max.X, Y: inst.Dest.Min.Y},
		inst.Dest.Max,
		{X: inst.Dest.Min.X, Y: inst.Dest.Max.Y},
	}
	bounds = math2d.Rect{Min: center, Max: center}
	for _, corner := range corners {
		dx := corner.X - center.X
		dy := corner.Y - center.Y
		p := math2d.Vec2{
			X: center.X + dx*cos - dy*sin,
			Y: center.Y + dx*sin + dy*cos,
		}
		bounds.Min.X = min(bounds.Min.X, p.X)
		bounds.Min.Y = min(bounds.Min.Y, p.Y)
		bounds.Max.X = max(bounds.Max.X, p.X)
		bounds.Max.Y = max(bounds.Max.Y, p.Y)
	}
	return bounds, true
}

// SpriteBoundsForCamera returns bounds for culling one sprite under camera.
// It follows the renderer rule: map Dest to screen, rotate the billboard, and
// map its corners back to world space.
func SpriteBoundsForCamera(inst render.SpriteInstance, camera render.Camera) (bounds math2d.Rect, ok bool) {
	if !IsFiniteSpriteGeometry(inst) {
		return math2d.Rect{}, false
	}
	screenMin := camera.WorldToScreen(inst.Dest.Min)
	screenMax := camera.WorldToScreen(inst.Dest.Max)
	center := math2d.Vec2{
		X: (screenMin.X + screenMax.X) * 0.5,
		Y: (screenMin.Y + screenMax.Y) * 0.5,
	}
	halfX := (screenMax.X - screenMin.X) * 0.5
	halfY := (screenMax.Y - screenMin.Y) * 0.5
	sin, cos := math.Sin(inst.Rotation), math.Cos(inst.Rotation)
	screenCorners := [4]math2d.Vec2{
		{X: -halfX, Y: -halfY},
		{X: halfX, Y: -halfY},
		{X: halfX, Y: halfY},
		{X: -halfX, Y: halfY},
	}
	for i, corner := range screenCorners {
		rotated := math2d.Vec2{
			X: center.X + corner.X*cos - corner.Y*sin,
			Y: center.Y + corner.X*sin + corner.Y*cos,
		}
		p := camera.ScreenToWorld(rotated)
		if i == 0 {
			bounds = math2d.Rect{Min: p, Max: p}
			continue
		}
		bounds.Min.X = min(bounds.Min.X, p.X)
		bounds.Min.Y = min(bounds.Min.Y, p.Y)
		bounds.Max.X = max(bounds.Max.X, p.X)
		bounds.Max.Y = max(bounds.Max.Y, p.Y)
	}
	return bounds, true
}

// BatchBounds unions the world bounds of every finite instance. ok is false
// when the batch has no finite instance (empty or fully invalid content),
// in which case nothing can be culled or drawn.
func BatchBounds(instances []render.SpriteInstance) (bounds math2d.Rect, ok bool) {
	for _, inst := range instances {
		b, valid := SpriteBounds(inst)
		if !valid {
			continue
		}
		if !ok {
			bounds, ok = b, true
			continue
		}
		bounds.Min.X = min(bounds.Min.X, b.Min.X)
		bounds.Min.Y = min(bounds.Min.Y, b.Min.Y)
		bounds.Max.X = max(bounds.Max.X, b.Max.X)
		bounds.Max.Y = max(bounds.Max.Y, b.Max.Y)
	}
	return bounds, ok
}

// BatchBoundsForCamera returns camera-aware bounds for a retained sprite batch.
func BatchBoundsForCamera(instances []render.SpriteInstance, camera render.Camera) (bounds math2d.Rect, ok bool) {
	for _, inst := range instances {
		b, valid := SpriteBoundsForCamera(inst, camera)
		if !valid {
			continue
		}
		if !ok {
			bounds, ok = b, true
			continue
		}
		bounds.Min.X = min(bounds.Min.X, b.Min.X)
		bounds.Min.Y = min(bounds.Min.Y, b.Min.Y)
		bounds.Max.X = max(bounds.Max.X, b.Max.X)
		bounds.Max.Y = max(bounds.Max.Y, b.Max.Y)
	}
	return bounds, ok
}

// VerticalInflation returns the conservative vertical bounds inflation of
// inst under anim: abs(Bounce*Intensity) * (1 + abs(Pulse.Boost)). It may
// overestimate the real bounce offset, never underestimate. It is zero when
// the batch bounce is inactive (BounceHz <= 0 or Intensity == 0).
func VerticalInflation(inst render.SpriteInstance, anim render.SpriteBatchAnimation) float64 {
	if anim.BounceHz <= 0 || anim.Intensity == 0 {
		return 0
	}
	return math.Abs(float64(inst.Animation.Bounce)*anim.Intensity) * (1 + math.Abs(anim.Pulse.Boost))
}

// InflatedBatchBounds returns BatchBounds expanded vertically by the
// largest VerticalInflation across instances, for whole-batch culling of
// animated batches. ok matches BatchBounds.
func InflatedBatchBounds(instances []render.SpriteInstance, anim render.SpriteBatchAnimation) (bounds math2d.Rect, ok bool) {
	inflation := 0.0
	for _, inst := range instances {
		inflation = max(inflation, VerticalInflation(inst, anim))
	}
	bounds, ok = BatchBounds(instances)
	if !ok {
		return bounds, false
	}
	bounds.Min.Y -= inflation
	bounds.Max.Y += inflation
	return bounds, true
}

// InflatedBatchBoundsForCamera returns animated batch bounds for camera-aware
// culling.
func InflatedBatchBoundsForCamera(instances []render.SpriteInstance, anim render.SpriteBatchAnimation, camera render.Camera) (bounds math2d.Rect, ok bool) {
	inflation := 0.0
	for _, inst := range instances {
		inflation = max(inflation, VerticalInflation(inst, anim))
	}
	bounds, ok = BatchBoundsForCamera(instances, camera)
	if !ok {
		return bounds, false
	}
	bounds.Min.Y -= inflation
	bounds.Max.Y += inflation
	return bounds, true
}

// WorldBoundsPreserveScreenRotation reports whether rotating a world-space
// AABB is conservative for the camera's scale-then-rotate draw transform.
// With anisotropic scale, callers retaining only world bounds must skip
// culling rotated content rather than risk dropping visible pixels.
func WorldBoundsPreserveScreenRotation(camera render.Camera) bool {
	resolved := camera.Resolve()
	return math.Abs(resolved.ScaleX) == math.Abs(resolved.ScaleY)
}

// BoundsForView returns bounds after the camera view is applied. It keeps an
// axis-aligned box that contains all four transformed corners.
func BoundsForView(bounds math2d.Rect, view math2d.Affine) math2d.Rect {
	if view.IsIdentity() {
		return bounds
	}

	corners := [4]math2d.Vec2{
		bounds.Min,
		{X: bounds.Max.X, Y: bounds.Min.Y},
		bounds.Max,
		{X: bounds.Min.X, Y: bounds.Max.Y},
	}
	transformed := view.Apply(corners[0])
	result := math2d.Rect{Min: transformed, Max: transformed}
	for _, corner := range corners[1:] {
		transformed = view.Apply(corner)
		result.Min.X = min(result.Min.X, transformed.X)
		result.Min.Y = min(result.Min.Y, transformed.Y)
		result.Max.X = max(result.Max.X, transformed.X)
		result.Max.Y = max(result.Max.Y, transformed.Y)
	}
	return result
}

// IntersectsView reports whether two rectangles overlap. Renderers use it for
// whole-batch culling after they combine the camera view and command clip.
func IntersectsView(bounds, view math2d.Rect) bool {
	return bounds.Overlaps(view)
}
