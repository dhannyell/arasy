package ease

import "math"

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
	case OutQuad:
		return 1 - (1-t)*(1-t)
	case InOutQuad:
		if t < 0.5 {
			return 2 * t * t
		}
		return 1 - (-2*t+2)*(-2*t+2)/2
	case InCubic:
		return t * t * t
	case OutCubic:
		return 1 - (1-t)*(1-t)*(1-t)
	case InOutCubic:
		if t < 0.5 {
			return 4 * t * t * t
		}
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)/2
	case InQuart:
		return t * t * t * t
	case OutQuart:
		return 1 - (1-t)*(1-t)*(1-t)*(1-t)
	case InOutQuart:
		if t < 0.5 {
			return 8 * t * t * t * t
		}
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)*(-2*t+2)/2
	case InOutSine:
		return -(math.Cos(math.Pi*t) - 1) / 2
	case OutExpo:
		if t >= 1 {
			return 1
		}
		return 1 - math.Pow(2, -10*t)
	case OutBack:
		const c1, c3 = 1.70158, 1.70158 + 1
		return 1 + c3*(t-1)*(t-1)*(t-1) + c1*(t-1)*(t-1)
	case OutElastic:
		if t <= 0 {
			return 0
		}
		if t >= 1 {
			return 1
		}
		const c4 = 2 * math.Pi / 3
		return math.Pow(2, -10*t)*math.Sin((t*10-0.75)*c4) + 1
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
		return n1*t*t + 0.75
	case t < 2.5/d1:
		t -= 2.25 / d1
		return n1*t*t + 0.9375
	default:
		t -= 2.625 / d1
		return n1*t*t + 0.984375
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
