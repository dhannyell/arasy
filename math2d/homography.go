package math2d

import "math"

const homographyEpsilon = 1e-12

// Homography is a 2D projective transform
//
//	| H0 H1 H2 |   x' = (H0*x + H1*y + H2) / w
//	| H3 H4 H5 |   y' = (H3*x + H4*y + H5) / w
//	| H6 H7 H8 |   w  =  H6*x + H7*y + H8
type Homography [9]float64

// HomographyFromQuad builds the projective transform that maps the unit
// square (0,0)(1,0)(1,1)(0,1) onto the four corners c[0..3], given in that
// same order (top-left, top-right, bottom-right, bottom-left in destination
// space). It uses Heckbert's closed-form square->quad solution: no 8x8 solve,
// the special source (the unit square) lets the system decouple.
//
// The returned matrix keeps H[8] = 1. ok is false when the corners are
// degenerate (any three collinear), which would make the map non-invertible.
func HomographyFromQuad(c [4]Vec2) (Homography, bool) {
	p0, p1, p2, p3 := c[0], c[1], c[2], c[3]

	// s is the "diagonal defect": p0 - p1 + p2 - p3. It is exactly zero when
	// the quad is a parallelogram (the two diagonals share a midpoint), which
	// is the same as "no perspective". s != 0 means the map needs a bottom row.
	sX := p0.X - p1.X + p2.X - p3.X
	sY := p0.Y - p1.Y + p2.Y - p3.Y

	// Affine fast path: w is constant (H6 = H7 = 0), no division needed later.
	if math.Abs(sX) <= homographyEpsilon && math.Abs(sY) <= homographyEpsilon {
		out := Homography{
			p1.X - p0.X, p3.X - p0.X, p0.X,
			p1.Y - p0.Y, p3.Y - p0.Y, p0.Y,
			0, 0, 1,
		}
		// A collinear parallelogram has zero area -> not invertible.
		if math.Abs(out.det()) <= homographyEpsilon {
			return Homography{}, false
		}
		return out, true
	}

	// Projective path. The perspective coefficients g, h are the unique
	// solution of the 2x2 system  g*d1 + h*d2 = s,  with d1 = p1-p2, d2 = p3-p2.
	d1X := p1.X - p2.X
	d1Y := p1.Y - p2.Y
	d2X := p3.X - p2.X
	d2Y := p3.Y - p2.Y

	den := d1X*d2Y - d2X*d1Y // cross(d1, d2) = twice the area of triangle p2,p1,p3
	if math.Abs(den) <= homographyEpsilon {
		return Homography{}, false // p1, p2, p3 collinear -> no unique solution
	}

	invDen := 1 / den
	g := (sX*d2Y - d2X*sY) * invDen // Cramer's rule
	h := (d1X*sY - sX*d1Y) * invDen

	// Top two rows are then forced so that (1,0)->p1 and (0,1)->p3; the bottom
	// row (g, h, 1) is what bends (1,1) onto p2 with the right perspective.
	out := Homography{
		p1.X - p0.X + g*p1.X, p3.X - p0.X + h*p3.X, p0.X,
		p1.Y - p0.Y + g*p1.Y, p3.Y - p0.Y + h*p3.Y, p0.Y,
		g, h, 1,
	}
	if math.Abs(out.det()) <= homographyEpsilon {
		return Homography{}, false
	}
	return out, true
}

func (h Homography) Apply(p Vec2) Vec2 {
	x, y := p.X, p.Y
	w := h[6]*x + h[7]*y + h[8]
	return Vec2{
		X: (h[0]*x + h[1]*y + h[2]) / w,
		Y: (h[3]*x + h[4]*y + h[5]) / w,
	}
}
func (h Homography) ApplyW(p Vec2) (Vec2, float64) {
	x, y := p.X, p.Y
	w := h[6]*x + h[7]*y + h[8]
	return Vec2{
		X: h[0]*x + h[1]*y + h[2],
		Y: h[3]*x + h[4]*y + h[5],
	}, w
}
func (h Homography) Inverse() (Homography, bool) {
	det := h.det()
	if math.Abs(det) <= homographyEpsilon {
		return Homography{}, false
	}
	invDet := 1 / det
	return Homography{
		(h[4]*h[8] - h[5]*h[7]) * invDet,
		(h[2]*h[7] - h[1]*h[8]) * invDet,
		(h[1]*h[5] - h[2]*h[4]) * invDet,
		(h[5]*h[6] - h[3]*h[8]) * invDet,
		(h[0]*h[8] - h[2]*h[6]) * invDet,
		(h[2]*h[3] - h[0]*h[5]) * invDet,
		(h[3]*h[7] - h[4]*h[6]) * invDet,
		(h[1]*h[6] - h[0]*h[7]) * invDet,
		(h[0]*h[4] - h[1]*h[3]) * invDet,
	}, true
}
func (h Homography) IsAffine() bool {
	return math.Abs(h[6]) <= homographyEpsilon && math.Abs(h[7]) <= homographyEpsilon
}

func (h Homography) det() float64 {
	return h[0]*(h[4]*h[8]-h[5]*h[7]) -
		h[1]*(h[3]*h[8]-h[5]*h[6]) +
		h[2]*(h[3]*h[7]-h[4]*h[6])
}
