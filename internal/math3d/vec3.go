// Package math3d contains the 3D projection proof of concept.
// World X and Y are the ground plane. Z is height.
package math3d

import "math"

// Vec3 is a point or vector in 3D world space.
type Vec3 struct {
	X float64 // X coordinate.
	Y float64 // Y coordinate.
	Z float64 // Height.
}

// Dot returns the dot product of two vectors.
// Explicit conversions keep the result stable across platforms.
func (v Vec3) Dot(other Vec3) float64 {
	return float64(v.X*other.X) + float64(v.Y*other.Y) + float64(v.Z*other.Z)
}

// Cross returns the cross product of two vectors.
// Explicit conversions keep the result stable across platforms.
func (v Vec3) Cross(other Vec3) Vec3 {
	return Vec3{
		X: float64(v.Y*other.Z) - float64(v.Z*other.Y),
		Y: float64(v.Z*other.X) - float64(v.X*other.Z),
		Z: float64(v.X*other.Y) - float64(v.Y*other.X),
	}
}

// Add returns the sum of two vectors.
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{v.X + other.X, v.Y + other.Y, v.Z + other.Z}
}

// Sub returns the vector from other to v.
func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{v.X - other.X, v.Y - other.Y, v.Z - other.Z}
}

// Mul returns the vector scaled by scalar.
func (v Vec3) Mul(scalar float64) Vec3 {
	return Vec3{v.X * scalar, v.Y * scalar, v.Z * scalar}
}

// LenSq returns the squared length of the vector.
func (v Vec3) LenSq() float64 { return v.Dot(v) }

// Len returns the length of the vector.
func (v Vec3) Len() float64 { return math.Sqrt(v.LenSq()) }

// Normalize returns a unit vector with the same direction as the input vector.
// If the input vector has zero length, it returns the zero vector.
func (v Vec3) Normalize() Vec3 {
	length := v.Len()
	if length == 0 {
		return Vec3{}
	}
	return Vec3{v.X / length, v.Y / length, v.Z / length}
}
