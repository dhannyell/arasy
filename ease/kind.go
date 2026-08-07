package ease

import (
	"math"

	"github.com/dhannyell/arasy/dmath"
)

type Kind uint8

const (
	Linear Kind = iota
	InQuad
	OutQuad
	InOutQuad
	InCubic
	OutCubic
	InOutCubic
	InQuart
	OutQuart
	InOutQuart
	InOutSine
	OutExpo
	OutBack
	OutElastic
	OutBounce
)

// Apply shapes a linear progress t∈[0,1] using curve k, returning t'∈[0,1].
// Endpoints are exact: Apply(k, 0) == 0 and Apply(k, 1) == 1 for all k.
func Apply(k Kind, t float64) float64 {
	return k.Sample(t)
}

// Sample shapes a linear progress value using k.
func (k Kind) Sample(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	switch k {
	case Linear:
		return t
	case InQuad:
		return t * t
	// The explicit float64 conversions round each product before it meets a
	// sum, so arm64 cannot fuse them into FMAs and drift from x86-64. Scene
	// tweens feed this result into checksummed simulation state. Products by
	// 2 or 0.5 are exact and stay uncast: fusing them cannot change a bit.
	case OutQuad:
		return 1 - float64((1-t)*(1-t))
	case InOutQuad:
		if t < 0.5 {
			return 2 * t * t
		}
		return 1 - (-2*t+2)*(-2*t+2)/2
	case InCubic:
		return t * t * t
	case OutCubic:
		return 1 - float64((1-t)*(1-t)*(1-t))
	case InOutCubic:
		if t < 0.5 {
			return 4 * t * t * t
		}
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)/2
	case InQuart:
		return t * t * t * t
	case OutQuart:
		return 1 - float64((1-t)*(1-t)*(1-t)*(1-t))
	case InOutQuart:
		if t < 0.5 {
			return 8 * t * t * t * t
		}
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)*(-2*t+2)/2
	// InOutSine, OutExpo, and OutElastic use dmath: math transcendentals
	// vary per architecture and would break replay through tweened state.
	// Their arguments stay far below the 2^20 domain limit of dmath trig,
	// because t is strictly inside (0, 1) here.
	case InOutSine:
		return -(dmath.Cos(float64(math.Pi*t)) - 1) / 2
	case OutExpo:
		if t >= 1 {
			return 1
		}
		return 1 - dmath.Pow(2, float64(-10*t))
	case OutBack:
		const c1, c3 = 1.70158, 1.70158 + 1
		return 1 + float64(c3*(t-1)*(t-1)*(t-1)) + float64(c1*(t-1)*(t-1))
	case OutElastic:
		if t <= 0 {
			return 0
		}
		if t >= 1 {
			return 1
		}
		const c4 = 2 * math.Pi / 3
		return float64(dmath.Pow(2, float64(-10*t))*dmath.Sin(float64((float64(t*10)-0.75)*c4))) + 1
	case OutBounce:
		return outBounce(t)
	default:
		return t
	}
}

func outBounce(t float64) float64 {
	const n1, d1 = 7.5625, 2.75
	switch {
	case t < 1/d1:
		return n1 * t * t
	case t < 2/d1:
		t -= 1.5 / d1
		return float64(n1*t*t) + 0.75
	case t < 2.5/d1:
		t -= 2.25 / d1
		return float64(n1*t*t) + 0.9375
	default:
		t -= 2.625 / d1
		return float64(n1*t*t) + 0.984375
	}
}

// Progress converts elapsed time and duration to a clamped t∈[0,1].
func Progress(elapsed, duration float64) float64 {
	if duration <= 0 {
		return 1
	}
	t := elapsed / duration
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}
