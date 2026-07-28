package vector

import (
	"math"
	"sync/atomic"

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

type flattenCacheEntry struct {
	tolerance float64
	contours  []contour
}

type flattenCache struct {
	latest atomic.Pointer[flattenCacheEntry]
}

// Path is immutable path geometry produced by Builder.Build. Copies share an
// internal cache of the most recently flattened tolerance.
type Path struct {
	segments []segment
	cache    *flattenCache
}

func finitePoint(point math2d.Vec2) bool {
	return !math.IsNaN(point.X) && !math.IsInf(point.X, 0) &&
		!math.IsNaN(point.Y) && !math.IsInf(point.Y, 0)
}
