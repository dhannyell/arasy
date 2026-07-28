package vector

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

type FillStyle struct {
	Color     render.Color
	Tolerance float64
}

func (p Path) AppendFill(dst []render.Command, base render.BaseCommand, style FillStyle) ([]render.Command, error) {
	contours, err := p.flatten(style.Tolerance)
	if err != nil {
		return dst, err
	}
	if len(contours) != 1 || !contours[0].closed {
		return dst, ErrFillContour
	}
	points := append([]math2d.Vec2(nil), contours[0].points...)
	if len(points) > 1 && points[0] == points[len(points)-1] {
		points = points[:len(points)-1]
	}
	if len(points) < 3 {
		return dst, ErrDegenerateGeometry
	}
	if selfIntersects(points, true) {
		return dst, ErrSelfIntersection
	}
	indices, ok := earClip(points)
	if !ok {
		return dst, ErrDegenerateGeometry
	}
	return append(dst, render.DrawTriangles{
		BaseCommand: base, Vertices: points, Indices: indices, Color: style.Color,
	}), nil
}

func earClip(points []math2d.Vec2) ([]uint32, bool) {
	order := make([]int, len(points))
	for index := range order {
		order[index] = index
	}
	if signedArea(points) < 0 {
		for left, right := 0, len(order)-1; left < right; left, right = left+1, right-1 {
			order[left], order[right] = order[right], order[left]
		}
	}
	indices := make([]uint32, 0, (len(points)-2)*3)
	for len(order) > 3 {
		found := false
		for index := range order {
			prev := order[(index+len(order)-1)%len(order)]
			curr := order[index]
			next := order[(index+1)%len(order)]
			if cross(points[curr].Sub(points[prev]), points[next].Sub(points[curr])) <= 0 {
				continue
			}
			contains := false
			for _, candidate := range order {
				if candidate != prev && candidate != curr && candidate != next &&
					pointInTriangle(points[candidate], points[prev], points[curr], points[next]) {
					contains = true
					break
				}
			}
			if contains {
				continue
			}
			indices = append(indices, uint32(prev), uint32(curr), uint32(next))
			order = append(order[:index], order[index+1:]...)
			found = true
			break
		}
		if !found {
			return nil, false
		}
	}
	indices = append(indices, uint32(order[0]), uint32(order[1]), uint32(order[2]))
	return indices, true
}

func signedArea(points []math2d.Vec2) float64 {
	area := 0.0
	for index, point := range points {
		next := points[(index+1)%len(points)]
		area += point.X*next.Y - next.X*point.Y
	}
	return area * 0.5
}

func cross(a, b math2d.Vec2) float64 { return a.X*b.Y - a.Y*b.X }

func pointInTriangle(p, a, b, c math2d.Vec2) bool {
	ab := cross(b.Sub(a), p.Sub(a))
	bc := cross(c.Sub(b), p.Sub(b))
	ca := cross(a.Sub(c), p.Sub(c))
	return ab >= 0 && bc >= 0 && ca >= 0
}

func selfIntersects(points []math2d.Vec2, closed bool) bool {
	edgeCount := len(points) - 1
	if closed {
		edgeCount = len(points)
	}
	for first := 0; first < edgeCount; first++ {
		a, b := points[first], points[(first+1)%len(points)]
		for second := first + 1; second < edgeCount; second++ {
			if second == first+1 || closed && first == 0 && second == edgeCount-1 {
				continue
			}
			c, d := points[second], points[(second+1)%len(points)]
			if segmentsIntersect(a, b, c, d) {
				return true
			}
		}
	}
	return false
}

func segmentsIntersect(a, b, c, d math2d.Vec2) bool {
	abC := cross(b.Sub(a), c.Sub(a))
	abD := cross(b.Sub(a), d.Sub(a))
	cdA := cross(d.Sub(c), a.Sub(c))
	cdB := cross(d.Sub(c), b.Sub(c))
	return math.Signbit(abC) != math.Signbit(abD) && math.Signbit(cdA) != math.Signbit(cdB)
}
