package ease

type Kind uint8

const (
	Linear Kind = iota
	InQuad
	OutQuad
	InCubic
	OutCubic
)

// Apply is called after Progress to shape the interpolation.
func Apply(k Kind, t float64) float64 {
	switch k {
	case Linear:
		return t
	case InQuad:
		return t * t
	case OutQuad:
		return 1 - (1-t)*(1-t)
	case InCubic:
		return t * t * t
	case OutCubic:
		return 1 - (1-t)*(1-t)*(1-t)
	default:
		return t
	}
}

// Progress is called once per tick to calculate where we are in time.
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
