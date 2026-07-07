package math2d

import "math"

// Vec2 represents a 2D vector or point
type Vec2 struct {
	// Testing if float64 is better than float32 for this struct
	X float64
	Y float64
}

// Add returns the sum of two vectors
func (v Vec2) Add(other Vec2) Vec2 {
	return Vec2{
		X: v.X + other.X,
		Y: v.Y + other.Y,
	}
}

// Sub returns the difference between two vectors.
func (v Vec2) Sub(other Vec2) Vec2 {
	return Vec2{
		X: v.X - other.X,
		Y: v.Y - other.Y,
	}
}

// Mul returns v scaled by scalar
func (v Vec2) Mul(scalar float64) Vec2 {
	return Vec2{
		X: v.X * scalar,
		Y: v.Y * scalar,
	}
}

// Div returns v divided by scalar
func (v Vec2) Div(scalar float64) Vec2 {
	if scalar == 0 {
		return v
	}

	return Vec2{
		X: v.X / scalar,
		Y: v.Y / scalar,
	}
}

// Dot returns the dot product between two vectors.
// The explicit float64 conversions force intermediate rounding so the compiler
// cannot fuse the multiply-add (FMA), which would produce different bits on
// arm64 vs amd64 (see golang/go#17895).
func (v Vec2) Dot(other Vec2) float64 {
	return float64(v.X*other.X) + float64(v.Y*other.Y)
}

// LenSq returns the squared length of vector
func (v Vec2) LenSq() float64 {
	return v.Dot(v)
}

// Len return the length of the vector
func (v Vec2) Len() float64 {
	return math.Sqrt(v.LenSq())
}

// Normalize returns a unit vector with the same direction as the input vector.
// If the input vector has zero length, it returns the zero vector
func (v Vec2) Normalize() Vec2 {
	length := v.Len()
	if length == 0 {
		return Vec2{}
	}

	return v.Div(length)
}

// Distance returns the distance between v and other vector
func (v Vec2) Distance(other Vec2) float64 {
	return v.Sub(other).Len()
}

// DistanceSq returns the squared distance between v and other vector
func (v Vec2) DistanceSq(other Vec2) float64 {
	return v.Sub(other).LenSq()
}

// Lerp linearly interpolates between v and target vector by t.
// The explicit float64 conversion on the product prevents FMA fusion on arm64.
func (v Vec2) Lerp(target Vec2, t float64) Vec2 {
	return Vec2{
		X: v.X + float64((target.X-v.X)*t),
		Y: v.Y + float64((target.Y-v.Y)*t),
	}
}
