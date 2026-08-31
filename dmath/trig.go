package dmath

import (
	"math"

	"github.com/dhannyell/fixed"
)

// maxTrigRange bounds the supported argument range of Sin and Cos.
const maxTrigRange = 1 << 20

// invTwoPi and twoPi convert radians to turns and back. The constant
// expressions round once, at compile time.
const (
	invTwoPi = 1 / (2 * math.Pi)
	twoPi    = 2 * math.Pi
)

// qFromFloat converts f to the Q32.32 grid; any part off the grid truncates
// toward zero. Transitional adapter; it leaves with the radian float surface.
func qFromFloat(f float64) fixed.Q {
	return fixed.FromRaw(int64(f * 0x1p32))
}

// floatFromQ converts q to float64, exactly for every trigonometric result.
func floatFromQ(q fixed.Q) float64 {
	return float64(q.Raw()) * 0x1p-32
}

// Sin returns the reproducible sine of x. It returns NaN for NaN, infinities,
// and values outside maxTrigRange. The body converts x to turns and delegates
// to the integer kernel of the fixed module.
func Sin(x float64) float64 {
	if !trigInRange(x) {
		return math.NaN()
	}
	t := float64(x * invTwoPi) // explicit rounding: no FMA into the subtraction
	t -= float64(int64(t))
	return floatFromQ(fixed.SinTurns(qFromFloat(t)))
}

// Cos returns the reproducible cosine of x. It returns NaN for NaN,
// infinities, and values outside maxTrigRange.
func Cos(x float64) float64 {
	if !trigInRange(x) {
		return math.NaN()
	}
	t := float64(x * invTwoPi) // explicit rounding: no FMA into the subtraction
	t -= float64(int64(t))
	return floatFromQ(fixed.CosTurns(qFromFloat(t)))
}

// trigInRange reports whether x is finite and within maxTrigRange.
func trigInRange(x float64) bool {
	return x >= -maxTrigRange && x <= maxTrigRange
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
	if math.IsInf(x, 0) || math.IsInf(y, 0) {
		return atan2Inf(y, x)
	}
	qy, qx := scaledPair(y, x)
	a := floatFromQ(fixed.Atan2Turns(qy, qx)) * twoPi
	if a == 0 && math.Signbit(y) {
		return math.Float64frombits(1 << 63) // -0: the angle underflowed the Q grid
	}
	return a
}

// atan2Inf resolves infinite arguments to their exact axis and diagonal
// angles, matching the standard-library conventions.
func atan2Inf(y, x float64) float64 {
	var a float64
	switch {
	case math.IsInf(y, 0) && math.IsInf(x, 0):
		a = math.Pi / 4
		if x < 0 {
			a = 3 * math.Pi / 4
		}
	case math.IsInf(y, 0):
		a = math.Pi / 2
	case x > 0:
		a = 0
	default:
		a = math.Pi
	}
	if y < 0 {
		return -a
	}
	return a
}

// scaledPair rescales y and x by one shared power of two so the larger
// magnitude lands in [1, 2), then converts both to Q. Power-of-two scaling
// never changes the angle.
func scaledPair(y, x float64) (qy, qx fixed.Q) {
	m := math.Abs(x)
	if a := math.Abs(y); a > m {
		m = a
	}
	if math.Float64bits(m)>>52 == 0 {
		// A subnormal pair first scales into the normal range, exactly.
		y *= 0x1p120
		x *= 0x1p120
		m *= 0x1p120
	}
	pow := math.Float64frombits(math.Float64bits(m) & (0x7FF << 52))
	return qFromFloat(y / pow), qFromFloat(x / pow)
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
