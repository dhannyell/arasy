package math2d

import "math"

// Affine is a 2D affine transform:
//
//	| A C E |   x' = A*x + C*y + E
//	| B D F |   y' = B*x + D*y + F
//	| 0 0 1 |
//
// The zero value is treated as identity everywhere a command carries it
// (same convention as the zero Clip rect = no clipping).
type Affine struct {
	A, B, C, D, E, F float64
}

// Identity returns the identity transform.
func Identity() Affine {
	return Affine{A: 1, D: 1}
}

// Translate returns an affine translation by v.
func Translate(v Vec2) Affine {
	return Affine{A: 1, D: 1, E: v.X, F: v.Y}
}

// Rotate returns an affine rotation by rad radians.
func Rotate(rad float64) Affine {
	cos := math.Cos(rad)
	sin := math.Sin(rad)
	return Affine{A: cos, B: sin, C: -sin, D: cos}
}

// Scale returns an affine scale by v (per-axis).
func Scale(v Vec2) Affine {
	return Affine{A: v.X, D: v.Y}
}

// Shear returns an affine shear (skew):
//
//	x' = x + shx*y
//	y' = y + shy*x
//
// shx displaces X proportionally to Y; shy displaces Y proportionally to X.
func Shear(shx, shy float64) Affine {
	return Affine{A: 1, B: shy, C: shx, D: 1}
}

// Mul returns m∘n: applies n first, then m (composition à esquerda).
// Mul is the mathematical composition: m.Mul(n).Apply(p) == m.Apply(n.Apply(p)).
//
// The float64 casts pin every product before it meets an add, so arm64 cannot
// fuse them into FMAs and drift from x86-64 — ScreenToWorld runs this on the
// picking path, where one ulp is a replay divergence.
func (m Affine) Mul(n Affine) Affine {
	return Affine{
		A: float64(m.A*n.A) + float64(m.C*n.B),
		B: float64(m.B*n.A) + float64(m.D*n.B),
		C: float64(m.A*n.C) + float64(m.C*n.D),
		D: float64(m.B*n.C) + float64(m.D*n.D),
		E: float64(m.A*n.E) + float64(m.C*n.F) + m.E,
		F: float64(m.B*n.E) + float64(m.D*n.F) + m.F,
	}
}

// Apply transforms point p by this affine matrix.
// The zero value acts as identity (no transformation).
func (m Affine) Apply(p Vec2) Vec2 {
	if m.IsIdentity() {
		return p
	}
	return Vec2{
		X: float64(m.A*p.X) + float64(m.C*p.Y) + m.E,
		Y: float64(m.B*p.X) + float64(m.D*p.Y) + m.F,
	}
}

// About returns a transform that applies m around pivot:
//
//	Translate(pivot)·m·Translate(-pivot)
func (m Affine) About(pivot Vec2) Affine {
	tTo := Translate(pivot)
	tFrom := Translate(Vec2{X: -pivot.X, Y: -pivot.Y})
	return tTo.Mul(m).Mul(tFrom)
}

// Inverse returns the inverse of m. ok is false when the determinant is near
// zero (singular / non-invertible).
func (m Affine) Inverse() (Affine, bool) {
	det := float64(m.A*m.D) - float64(m.B*m.C)
	if math.Abs(det) <= 1e-12 {
		return Affine{}, false
	}
	invDet := 1 / det
	return Affine{
		A: m.D * invDet,
		B: -m.B * invDet,
		C: -m.C * invDet,
		D: m.A * invDet,
		E: float64(float64(m.C*m.F)-float64(m.D*m.E)) * invDet,
		F: float64(float64(m.B*m.E)-float64(m.A*m.F)) * invDet,
	}, true
}

// MeanScale returns the average linear scale factor of m's 2×2 part,
// sqrt(|det|). Backends use it to grow a stroke width — which has no single
// direction — under a non-uniform or rotated transform. Identity returns 1;
// a singular (degenerate) transform returns 0.
func (m Affine) MeanScale() float64 {
	if m.IsIdentity() {
		return 1
	}
	det := float64(m.A*m.D) - float64(m.B*m.C)
	return math.Sqrt(math.Abs(det))
}

// IsIdentity reports whether m is the identity transform.
// Both the zero value Affine{} and Identity() return true.
func (m Affine) IsIdentity() bool {
	return m.A == 1 && m.B == 0 && m.C == 0 && m.D == 1 && m.E == 0 && m.F == 0 ||
		m == Affine{}
}
