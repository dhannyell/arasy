package ease

import (
	"errors"
	"math"
)

var errInvalidCubicBezier = errors.New("ease: cubic-bezier coordinates must be finite and x coordinates must be in [0,1]")

// CubicBezier is a CSS cubic-bezier easing curve. Its x control points are
// constrained to [0,1], while its y control points may overshoot.
type CubicBezier struct {
	x1 float64
	y1 float64
	x2 float64
	y2 float64
}

// NewCubicBezier constructs a CSS cubic-bezier easing curve.
func NewCubicBezier(x1, y1, x2, y2 float64) (CubicBezier, error) {
	if !finite(x1) || !finite(y1) || !finite(x2) || !finite(y2) ||
		x1 < 0 || x1 > 1 || x2 < 0 || x2 > 1 {
		return CubicBezier{}, errInvalidCubicBezier
	}
	return CubicBezier{x1: x1, y1: y1, x2: x2, y2: y2}, nil
}

// MustCubicBezier is like NewCubicBezier but panics for invalid coordinates.
func MustCubicBezier(x1, y1, x2, y2 float64) CubicBezier {
	curve, err := NewCubicBezier(x1, y1, x2, y2)
	if err != nil {
		panic(err)
	}
	return curve
}

// Sample evaluates the curve at CSS input progress.
func (c CubicBezier) Sample(progress float64) float64 {
	if progress <= 0 {
		return 0
	}
	if progress >= 1 {
		return 1
	}

	// Newton-Raphson converges quickly for ordinary CSS curves. A bounded
	// bisection fallback handles flat derivatives without unbounded work.
	u := progress
	converged := false
	for range 8 {
		x := cubicCoordinate(u, c.x1, c.x2)
		dx := cubicDerivative(u, c.x1, c.x2)
		if math.Abs(x-progress) <= 1e-7 {
			converged = true
			break
		}
		if math.Abs(dx) <= 1e-7 {
			break
		}
		next := u - (x-progress)/dx
		if next < 0 || next > 1 {
			break
		}
		u = next
	}
	if !converged {
		lo, hi := 0.0, 1.0
		for range 24 {
			u = (lo + hi) * 0.5
			if cubicCoordinate(u, c.x1, c.x2) < progress {
				lo = u
			} else {
				hi = u
			}
		}
	}
	return cubicCoordinate(u, c.y1, c.y2)
}

func cubicCoordinate(t, p1, p2 float64) float64 {
	oneMinus := 1 - t
	return 3*oneMinus*oneMinus*t*p1 + 3*oneMinus*t*t*p2 + t*t*t
}

func cubicDerivative(t, p1, p2 float64) float64 {
	oneMinus := 1 - t
	return 3*oneMinus*oneMinus*p1 + 6*oneMinus*t*(p2-p1) + 3*t*t*(1-p2)
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
