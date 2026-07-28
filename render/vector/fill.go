package vector

import (
	"math"
	"slices"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

type FillStyle struct {
	Color     render.Color
	Tolerance float64
}

// FillWorkspace retains triangulation and output buffers between calls to
// AppendFillWithWorkspace. It is intended for sequential frame rendering.
//
// A workspace must not be used concurrently. Commands returned by
// AppendFillWithWorkspace remain valid only until the next call that uses the
// same workspace.
type FillWorkspace struct {
	points  []math2d.Vec2
	indices []uint32
	order   []int
}

func (p Path) AppendFill(dst []render.Command, base render.BaseCommand, style FillStyle) ([]render.Command, error) {
	return p.appendFill(dst, base, style, nil)
}

// AppendFillWithWorkspace appends a fill while reusing buffers retained by
// workspace. See FillWorkspace for ownership and lifetime requirements.
func (p Path) AppendFillWithWorkspace(
	dst []render.Command,
	base render.BaseCommand,
	style FillStyle,
	workspace *FillWorkspace,
) ([]render.Command, error) {
	if workspace == nil {
		return p.AppendFill(dst, base, style)
	}
	return p.appendFill(dst, base, style, workspace)
}

func (p Path) appendFill(
	dst []render.Command,
	base render.BaseCommand,
	style FillStyle,
	workspace *FillWorkspace,
) ([]render.Command, error) {
	contours, err := p.flatten(style.Tolerance)
	if err != nil {
		return dst, err
	}
	if len(contours) != 1 || !contours[0].closed {
		return dst, ErrFillContour
	}
	var points []math2d.Vec2
	if workspace != nil {
		points = workspace.points[:0]
	}
	points = append(points, contours[0].points...)
	if len(points) > 1 && points[0] == points[len(points)-1] {
		points = points[:len(points)-1]
	}
	if workspace != nil {
		workspace.points = points
	}
	if len(points) < 3 {
		return dst, ErrDegenerateGeometry
	}
	if selfIntersects(points, true) {
		return dst, ErrSelfIntersection
	}
	var indices []uint32
	var order []int
	if workspace != nil {
		indices = workspace.indices[:0]
		order = workspace.order[:0]
	}
	indices, order, ok := earClipInto(indices, order, points)
	if workspace != nil {
		workspace.indices = indices
		workspace.order = order
	}
	if !ok {
		return dst, ErrDegenerateGeometry
	}
	return append(dst, render.DrawTriangles{
		BaseCommand: base, Vertices: points, Indices: indices, Color: style.Color,
	}), nil
}

func earClipInto(indices []uint32, order []int, points []math2d.Vec2) ([]uint32, []int, bool) {
	order = slices.Grow(order[:0], len(points))
	for range points {
		order = append(order, 0)
	}
	for index := range order {
		order[index] = index
	}
	if signedArea(points) < 0 {
		for left, right := 0, len(order)-1; left < right; left, right = left+1, right-1 {
			order[left], order[right] = order[right], order[left]
		}
	}
	indices = slices.Grow(indices[:0], (len(points)-2)*3)
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
			return indices, order, false
		}
	}
	indices = append(indices, uint32(order[0]), uint32(order[1]), uint32(order[2]))
	return indices, order, true
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
