package dmath

import "math"

// maxTrigRange bounds Cody-Waite argument reduction in Sin and Cos.
const maxTrigRange = 1 << 20

// pio2Hi and pio2Lo are the Cody-Waite split of pi/2. The high part keeps 33
// mantissa bits, so its product with the reduction multiple is exact within
// maxTrigRange.
const (
	pio2Hi  = 1.57079632673412561417e+00
	pio2Lo  = 6.07710050650619224932e-11
	invPio2 = 6.36619772367581382433e-01 // 2/pi
)

// Sin returns the reproducible sine of x. It returns NaN for NaN, infinities,
// and values outside maxTrigRange.
func Sin(x float64) float64 {
	if !trigInRange(x) {
		return math.NaN()
	}
	r, q := reducePio2(x)
	return trigQuadrant(r, q)
}

// Cos returns the reproducible cosine of x. It returns NaN for NaN, infinities,
// and values outside maxTrigRange.
func Cos(x float64) float64 {
	if !trigInRange(x) {
		return math.NaN()
	}
	r, q := reducePio2(x)
	return trigQuadrant(r, (q+1)&3)
}

// trigInRange reports whether x is finite and within maxTrigRange.
func trigInRange(x float64) bool {
	return x >= -maxTrigRange && x <= maxTrigRange
}

// reducePio2 applies Cody-Waite reduction and returns the remainder after
// subtracting the nearest multiple of pi/2 and that multiple's quadrant. The
// remainder is in [-pi/4, pi/4] and the quadrant is in [0, 3]. Explicit
// conversions prevent FMA fusion from changing the result across architectures.
func reducePio2(x float64) (r float64, q int64) {
	kf := x * invPio2
	var k int64
	if kf >= 0 {
		k = int64(kf + 0.5)
	} else {
		k = int64(kf - 0.5)
	}
	kf = float64(k)
	r = (x - float64(kf*pio2Hi)) - float64(kf*pio2Lo)
	// Bitwise AND maps negative k to the non-negative quadrant range.
	return r, k & 3
}

// trigQuadrant evaluates sine at residue r in quadrant q.
func trigQuadrant(r float64, q int64) float64 {
	switch q {
	case 0:
		return sinPoly(r)
	case 1:
		return cosPoly(r)
	case 2:
		return -sinPoly(r)
	default:
		return -cosPoly(r)
	}
}

// sinPoly evaluates the Taylor polynomial for sine through r^13 on |r| <=
// pi/4. Truncation error is below 1e-13.
func sinPoly(r float64) float64 {
	r2 := r * r
	p := 1.0 / 6227020800 // 1/13!
	p = -1.0/39916800 + float64(r2*p)
	p = 1.0/362880 + float64(r2*p)
	p = -1.0/5040 + float64(r2*p)
	p = 1.0/120 + float64(r2*p)
	p = -1.0/6 + float64(r2*p)
	return r + float64(r*float64(r2*p))
}

// cosPoly evaluates the Taylor polynomial for cosine through r^14 on |r| <=
// pi/4.
func cosPoly(r float64) float64 {
	r2 := r * r
	p := -1.0 / 87178291200 // -1/14!
	p = 1.0/479001600 + float64(r2*p)
	p = -1.0/3628800 + float64(r2*p)
	p = 1.0/40320 + float64(r2*p)
	p = -1.0/720 + float64(r2*p)
	p = 1.0/24 + float64(r2*p)
	p = -0.5 + float64(r2*p)
	return 1.0 + float64(r2*p)
}

// Atan2 returns the reproducible angle of vector (x, y) in [-pi, pi].
// It returns NaN if either argument is NaN.
func Atan2(y, x float64) float64 {
	switch {
	case math.IsNaN(x) || math.IsNaN(y):
		return math.NaN()
	case y == 0:
		if x >= 0 && !math.Signbit(x) {
			return y // preserve the sign of zero
		}
		if math.Signbit(y) {
			return -math.Pi
		}
		return math.Pi
	case x == 0:
		if y > 0 {
			return math.Pi / 2
		}
		return -math.Pi / 2
	}
	if math.IsInf(x, 0) && math.IsInf(y, 0) {
		// Match the standard quadrant for two infinite inputs.
		a := math.Pi / 4
		if x < 0 {
			a = 3 * math.Pi / 4
		}
		if y < 0 {
			return -a
		}
		return a
	}
	absY, absX := math.Abs(y), math.Abs(x)
	var a float64
	if absY <= absX {
		a = atanUnit(absY / absX)
	} else {
		a = math.Pi/2 - atanUnit(absX/absY)
	}
	if x < 0 {
		a = math.Pi - a
	}
	if y < 0 {
		return -a
	}
	return a
}

// atanUnit reduces atan(z) to the interval used by atanSeries. For z above
// sqrt(2)-1 it uses atan(z) = pi/4 + atan((z-1)/(z+1)).
func atanUnit(z float64) float64 {
	const tanPi8 = 0.41421356237309503 // sqrt(2) - 1
	if z <= tanPi8 {
		return atanSeries(z)
	}
	t := (z - 1.0) / (z + 1.0)
	return math.Pi/4 + atanSeries(t)
}

// atanSeries evaluates the Taylor polynomial for atan through t^21 on
// |t| <= sqrt(2)-1. Truncation error is below 1e-10.
func atanSeries(t float64) float64 {
	t2 := t * t
	p := 1.0 / 21
	p = -1.0/19 + float64(t2*p)
	p = 1.0/17 + float64(t2*p)
	p = -1.0/15 + float64(t2*p)
	p = 1.0/13 + float64(t2*p)
	p = -1.0/11 + float64(t2*p)
	p = 1.0/9 + float64(t2*p)
	p = -1.0/7 + float64(t2*p)
	p = 1.0/5 + float64(t2*p)
	p = -1.0/3 + float64(t2*p)
	return t + float64(t*float64(t2*p))
}

// Asin returns the reproducible arcsine of x in [-pi/2, pi/2]. It returns NaN
// when x is outside [-1, 1] or is NaN.
func Asin(x float64) float64 {
	if x < -1 || x > 1 || math.IsNaN(x) {
		return math.NaN()
	}
	// (1-x)*(1+x) keeps precision near |x| = 1, where 1-x*x cancels.
	return Atan2(x, math.Sqrt((1.0-x)*(1.0+x)))
}

// Acos returns the reproducible arccosine of x in [0, pi]. It returns NaN
// when x is outside [-1, 1] or is NaN.
func Acos(x float64) float64 {
	if x < -1 || x > 1 || math.IsNaN(x) {
		return math.NaN()
	}
	return Atan2(math.Sqrt((1.0-x)*(1.0+x)), x)
}
