package renderbatch

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// IsFiniteRect reports whether every coordinate of r is finite (no NaN or
// Inf). Backends drop instances with non-finite geometry, logging a
// one-time warning per batch.
func IsFiniteRect(r math2d.Rect) bool {
	return isFinite(r.Min.X) && isFinite(r.Min.Y) && isFinite(r.Max.X) && isFinite(r.Max.Y)
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// SpriteBounds returns the world-space axis-aligned bounding box of inst,
// including Rotation: the four Dest corners are rotated around the Dest
// center. ok is false when Dest is non-finite, in which case the returned
// rect is meaningless and the adapter must drop the instance.
func SpriteBounds(inst render.SpriteInstance) (bounds math2d.Rect, ok bool) {
	if !IsFiniteRect(inst.Dest) {
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

// IntersectsView reports whether batch bounds overlap the rendered view
// rect, for whole-batch culling. When the command carries a Clip, the
// caller intersects view and clip first; this helper is a plain
// rect-vs-rect overlap test.
func IntersectsView(bounds, view math2d.Rect) bool {
	return bounds.Overlaps(view)
}
