package math2d

// Rect represents an axis-aligned rectangle.
// Min is inclusive. Max is exclusive for point containment.
type Rect struct {
	Min Vec2
	Max Vec2
}

// NewRect creates a rectangle from position and size.
func NewRect(pos Vec2, size Vec2) Rect {
	return Rect{
		Min: pos,
		Max: pos.Add(size),
	}
}

// NewRectFromCorners creates a rectangle from two opposite corners.
func NewRectFromCorners(firstCorner, oppositeCorner Vec2) Rect {
	return Rect{
		Min: Vec2{
			X: min(firstCorner.X, oppositeCorner.X),
			Y: min(firstCorner.Y, oppositeCorner.Y),
		},
		Max: Vec2{
			X: max(firstCorner.X, oppositeCorner.X),
			Y: max(firstCorner.Y, oppositeCorner.Y),
		},
	}
}

// Width returns the rectangle width.
func (r Rect) Width() float64 {
	return r.Max.X - r.Min.X
}

// Height returns the rectangle height.
func (r Rect) Height() float64 {
	return r.Max.Y - r.Min.Y
}

// Size returns the rectangle size.
func (r Rect) Size() Vec2 {
	return Vec2{
		X: r.Width(),
		Y: r.Height(),
	}
}

// Center returns the rectangle center point.
func (r Rect) Center() Vec2 {
	return Vec2{
		X: (r.Min.X + r.Max.X) * 0.5,
		Y: (r.Min.Y + r.Max.Y) * 0.5,
	}
}

// Scaled returns r resized by factor around its own center, keeping the
// center fixed. A factor of 1 returns an equal rect; e.g. used for hover/press
// scale effects on buttons and cards.
func (r Rect) Scaled(factor float64) Rect {
	c := r.Center()
	hw := r.Width() / 2 * factor
	hh := r.Height() / 2 * factor
	return Rect{
		Min: Vec2{X: c.X - hw, Y: c.Y - hh},
		Max: Vec2{X: c.X + hw, Y: c.Y + hh},
	}
}

// Contains reports whether p is inside r.
// Min is inclusive and Max is exclusive.
func (r Rect) Contains(p Vec2) bool {
	return p.X >= r.Min.X &&
		p.X < r.Max.X &&
		p.Y >= r.Min.Y &&
		p.Y < r.Max.Y
}

// Overlaps returns true only when rectangles have positive overlapping area.
func (r Rect) Overlaps(other Rect) bool {
	return r.Min.X < other.Max.X &&
		r.Max.X > other.Min.X &&
		r.Min.Y < other.Max.Y &&
		r.Max.Y > other.Min.Y
}

// TouchesOrOverlaps returns true when rectangles overlap or touch edges.
func (r Rect) TouchesOrOverlaps(other Rect) bool {
	return r.Min.X <= other.Max.X &&
		r.Max.X >= other.Min.X &&
		r.Min.Y <= other.Max.Y &&
		r.Max.Y >= other.Min.Y
}
