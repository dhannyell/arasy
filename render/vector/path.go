package vector

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
)

type verb uint8

const (
	verbMove verb = iota
	verbLine
	verbQuad
	verbCubic
	verbClose
)

type segment struct {
	verb       verb
	p0, p1, p2 math2d.Vec2
}

// Path is immutable path geometry produced by Builder.Build.
type Path struct {
	segments []segment
}

func finitePoint(point math2d.Vec2) bool {
	return !math.IsNaN(point.X) && !math.IsInf(point.X, 0) &&
		!math.IsNaN(point.Y) && !math.IsInf(point.Y, 0)
}
