package utilityai

import "github.com/dhannyell/arasy/dmath"

// CurveKind selects the response-curve shape.
type CurveKind uint8

const (
	CurveLinear     CurveKind = iota // y = m*x + b
	CurveQuadratic                   // y = m*x*x + b
	CurveCubic                       // y = m*x*x*x + b
	CurveSmoothStep                  // y = m*(x*x*(3-2*x)) + b
	CurveStep                        // y = 0 if x < c, else 1
	CurveLogistic                    // y = 1/(1+e^(-m*(x-c))) + b
)

// Curve maps a normalized input in [0,1] to a score in [0,1]. The output
// is always clamped. M is the slope, B the shift, C the CurveStep threshold.
type Curve struct {
	Kind    CurveKind
	M, B, C float64
}

// Evaluate applies the curve to one input. The float64 casts block FMA
// fusion, so the sums round the same on all architectures.
func (c Curve) Evaluate(x float64) float64 {
	var y float64
	switch c.Kind {
	case CurveLinear:
		y = float64(c.M*x) + c.B
	case CurveQuadratic:
		y = float64(c.M*x*x) + c.B
	case CurveCubic:
		y = float64(c.M*x*x*x) + c.B
	case CurveSmoothStep:
		// 2*x is exact, so an FMA fusion of 3-2*x cannot change the bits.
		s := x * x * (3 - 2*x)
		y = float64(c.M*s) + c.B
	case CurveStep:
		if x < c.C {
			y = 0
		} else {
			y = 1
		}
	case CurveLogistic:
		// M is the steepness, C the midpoint, B a post-curve shift.
		y = 1.0/(1.0+dmath.Exp(-c.M*(x-c.C))) + c.B
	}
	return clamp01(y)
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
