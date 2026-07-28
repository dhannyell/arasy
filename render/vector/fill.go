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
	points    []math2d.Vec2
	indices   []uint32
	order     []int
	scanY     []float64
	crossings []fillCrossing
}

type fillCrossing struct {
	from, to math2d.Vec2
	x        float64
	winding  int
}

// AppendFill tessellates every closed contour in p using the non-zero winding
// rule. Disconnected contours and oppositely wound counter-forms are supported.
func (p Path) AppendFill(dst []render.Command, base render.BaseCommand, style FillStyle) ([]render.Command, error) {
	return p.appendFill(dst, base, style, nil)
}

// AppendFillWithWorkspace appends a fill while reusing buffers retained by
// workspace. It uses the same non-zero winding semantics as AppendFill. See
// FillWorkspace for ownership and lifetime requirements.
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
	if len(contours) == 0 {
		return dst, ErrFillContour
	}
	for _, contour := range contours {
		if !contour.closed {
			return dst, ErrFillContour
		}
	}
	if len(contours) > 1 {
		return appendCompoundFill(dst, base, style, contours, workspace)
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

// appendCompoundFill tessellates disconnected contours and holes according to
// SVG's default non-zero winding rule. Horizontal slabs are bounded by every
// contour vertex, so their active edges cannot reorder within a slab. Each
// filled interval is therefore an exact trapezoid of the flattened geometry.
//
// The common single-contour case stays on ear clipping above: it emits fewer
// triangles. Compound paths pay this additional work only when they need it.
func appendCompoundFill(
	dst []render.Command,
	base render.BaseCommand,
	style FillStyle,
	contours []contour,
	workspace *FillWorkspace,
) ([]render.Command, error) {
	var vertices []math2d.Vec2
	var indices []uint32
	var scanY []float64
	var crossings []fillCrossing
	if workspace != nil {
		vertices = workspace.points[:0]
		indices = workspace.indices[:0]
		scanY = workspace.scanY[:0]
		crossings = workspace.crossings[:0]
	}

	for _, contour := range contours {
		points := openContour(contour.points)
		if len(points) < 3 || signedArea(points) == 0 {
			return dst, ErrDegenerateGeometry
		}
		if selfIntersects(points, true) {
			return dst, ErrSelfIntersection
		}
		for _, point := range points {
			scanY = append(scanY, point.Y)
		}
	}
	slices.Sort(scanY)
	scanY = slices.Compact(scanY)
	if len(scanY) < 2 {
		return dst, ErrDegenerateGeometry
	}

	for slab := 0; slab+1 < len(scanY); slab++ {
		top, bottom := scanY[slab], scanY[slab+1]
		if top == bottom {
			continue
		}
		middle := top + (bottom-top)*0.5
		crossings = crossings[:0]
		for _, contour := range contours {
			points := openContour(contour.points)
			for index, from := range points {
				to := points[(index+1)%len(points)]
				if from.Y == to.Y ||
					!((from.Y <= middle && middle < to.Y) ||
						(to.Y <= middle && middle < from.Y)) {
					continue
				}
				winding := -1
				if to.Y > from.Y {
					winding = 1
				}
				crossings = append(crossings, fillCrossing{
					from:    from,
					to:      to,
					x:       edgeXAtY(from, to, middle),
					winding: winding,
				})
			}
		}
		slices.SortFunc(crossings, func(a, b fillCrossing) int {
			switch {
			case a.x < b.x:
				return -1
			case a.x > b.x:
				return 1
			default:
				return 0
			}
		})

		winding := 0
		var left fillCrossing
		for index := 0; index < len(crossings); {
			next := index + 1
			delta := crossings[index].winding
			for next < len(crossings) && crossings[next].x == crossings[index].x {
				delta += crossings[next].winding
				next++
			}
			previous := winding
			winding += delta
			switch {
			case previous == 0 && winding != 0:
				left = crossings[index]
			case previous != 0 && winding == 0:
				right := crossings[index]
				vertices, indices = appendFillTrapezoid(
					vertices,
					indices,
					math2d.Vec2{X: edgeXAtY(left.from, left.to, top), Y: top},
					math2d.Vec2{X: edgeXAtY(right.from, right.to, top), Y: top},
					math2d.Vec2{X: edgeXAtY(right.from, right.to, bottom), Y: bottom},
					math2d.Vec2{X: edgeXAtY(left.from, left.to, bottom), Y: bottom},
				)
			}
			index = next
		}
		if winding != 0 {
			return dst, ErrDegenerateGeometry
		}
	}

	if workspace != nil {
		workspace.points = vertices
		workspace.indices = indices
		workspace.scanY = scanY
		workspace.crossings = crossings
	}
	if len(indices) == 0 {
		return dst, ErrDegenerateGeometry
	}
	return append(dst, render.DrawTriangles{
		BaseCommand: base, Vertices: vertices, Indices: indices, Color: style.Color,
	}), nil
}

func openContour(points []math2d.Vec2) []math2d.Vec2 {
	if len(points) > 1 && points[0] == points[len(points)-1] {
		return points[:len(points)-1]
	}
	return points
}

func edgeXAtY(from, to math2d.Vec2, y float64) float64 {
	return from.X + (y-from.Y)*(to.X-from.X)/(to.Y-from.Y)
}

func appendFillTrapezoid(
	vertices []math2d.Vec2,
	indices []uint32,
	topLeft, topRight, bottomRight, bottomLeft math2d.Vec2,
) ([]math2d.Vec2, []uint32) {
	start := uint32(len(vertices))
	vertices = append(vertices, topLeft, topRight, bottomRight, bottomLeft)
	if cross(topRight.Sub(topLeft), bottomRight.Sub(topLeft)) != 0 {
		indices = append(indices, start, start+1, start+2)
	}
	if cross(bottomRight.Sub(topLeft), bottomLeft.Sub(topLeft)) != 0 {
		indices = append(indices, start, start+2, start+3)
	}
	return vertices, indices
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
