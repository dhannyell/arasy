package tess

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
)

const earClipEpsilon = 1e-9

// earClip triangulates a simple polygon using the O(n^2) ear clipping algorithm.
//
// Points are expected in polygon boundary order, either clockwise or
// counter-clockwise. Concave polygons are supported; self-intersecting polygons
// are not. Degenerate inputs are handled conservatively by dropping nearly
// colinear vertices when no valid ear can be found.
func earClip(points []math2d.Vec2) [][3]math2d.Vec2 {
	if len(points) < 3 {
		return nil
	}

	orientation := polygonSignedArea(points)
	if math.Abs(orientation) <= earClipEpsilon {
		return nil
	}

	idx := make([]int, len(points))
	for i := range idx {
		idx[i] = i
	}

	tris := make([][3]math2d.Vec2, 0, len(points)-2)

	for len(idx) > 3 {
		earFound := false

		for i := range idx {
			prevIndex := idx[(i-1+len(idx))%len(idx)]
			curIndex := idx[i]
			nextIndex := idx[(i+1)%len(idx)]

			prev := points[prevIndex]
			cur := points[curIndex]
			next := points[nextIndex]

			if !isConvex(prev, cur, next, orientation) {
				continue
			}

			if containsAnyPoint(points, idx, prevIndex, curIndex, nextIndex) {
				continue
			}

			tris = append(tris, [3]math2d.Vec2{prev, cur, next})
			idx = append(idx[:i], idx[i+1:]...)
			earFound = true
			break
		}

		if !earFound {
			removeMostColinearPoint(points, &idx)
		}
	}

	if len(idx) == 3 {
		tris = append(tris, [3]math2d.Vec2{
			points[idx[0]],
			points[idx[1]],
			points[idx[2]],
		})
	}

	return tris
}

func polygonSignedArea(points []math2d.Vec2) float64 {
	area := 0.0
	for i := range points {
		a := points[i]
		b := points[(i+1)%len(points)]
		area += a.X*b.Y - a.Y*b.X
	}
	return area * 0.5
}

func isConvex(prev, cur, next math2d.Vec2, orientation float64) bool {
	cross := cross2(cur.Sub(prev), next.Sub(cur))
	if math.Abs(cross) <= earClipEpsilon {
		return false
	}

	return cross*orientation > 0
}

func containsAnyPoint(points []math2d.Vec2, idx []int, prevIndex, curIndex, nextIndex int) bool {
	a := points[prevIndex]
	b := points[curIndex]
	c := points[nextIndex]

	for _, pointIndex := range idx {
		if pointIndex == prevIndex || pointIndex == curIndex || pointIndex == nextIndex {
			continue
		}

		if pointInTriangle(points[pointIndex], a, b, c) {
			return true
		}
	}

	return false
}

func pointInTriangle(p, a, b, c math2d.Vec2) bool {
	ab := cross2(b.Sub(a), p.Sub(a))
	bc := cross2(c.Sub(b), p.Sub(b))
	ca := cross2(a.Sub(c), p.Sub(c))

	hasNeg := ab < -earClipEpsilon || bc < -earClipEpsilon || ca < -earClipEpsilon
	hasPos := ab > earClipEpsilon || bc > earClipEpsilon || ca > earClipEpsilon

	return !(hasNeg && hasPos)
}

func removeMostColinearPoint(points []math2d.Vec2, idx *[]int) {
	if len(*idx) <= 3 {
		return
	}

	removeAt := 0
	best := math.Inf(1)

	for i := range *idx {
		prev := points[(*idx)[(i-1+len(*idx))%len(*idx)]]
		cur := points[(*idx)[i]]
		next := points[(*idx)[(i+1)%len(*idx)]]

		score := math.Abs(cross2(cur.Sub(prev), next.Sub(cur)))
		if score < best {
			best = score
			removeAt = i
		}
	}

	*idx = append((*idx)[:removeAt], (*idx)[removeAt+1:]...)
}

func cross2(a, b math2d.Vec2) float64 {
	return a.X*b.Y - a.Y*b.X
}
