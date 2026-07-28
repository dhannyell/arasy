package vector

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
)

const maxFlattenDepth = 18

type contour struct {
	points []math2d.Vec2
	closed bool
}

func (p Path) flatten(tolerance float64) ([]contour, error) {
	if math.IsNaN(tolerance) || math.IsInf(tolerance, 0) || tolerance <= 0 {
		return nil, ErrInvalidTolerance
	}
	var contours []contour
	var current, start math2d.Vec2
	for _, command := range p.segments {
		switch command.verb {
		case verbMove:
			contours = append(contours, contour{points: []math2d.Vec2{command.p0}})
			current, start = command.p0, command.p0
		case verbLine:
			contours[len(contours)-1].points = append(contours[len(contours)-1].points, command.p0)
			current = command.p0
		case verbQuad:
			points := &contours[len(contours)-1].points
			if err := flattenQuad(points, current, command.p0, command.p1, tolerance, 0); err != nil {
				return nil, err
			}
			current = command.p1
		case verbCubic:
			points := &contours[len(contours)-1].points
			if err := flattenCubic(points, current, command.p0, command.p1, command.p2, tolerance, 0); err != nil {
				return nil, err
			}
			current = command.p2
		case verbClose:
			index := len(contours) - 1
			contours[index].closed = true
			if contours[index].points[len(contours[index].points)-1] != start {
				contours[index].points = append(contours[index].points, start)
			}
			current = start
		}
	}
	return contours, nil
}

func flattenQuad(dst *[]math2d.Vec2, p0, p1, p2 math2d.Vec2, tolerance float64, depth int) error {
	if distanceToLine(p1, p0, p2) <= tolerance {
		*dst = append(*dst, p2)
		return nil
	}
	if depth >= maxFlattenDepth {
		return ErrFlattenDepth
	}
	p01, p12 := midpoint(p0, p1), midpoint(p1, p2)
	center := midpoint(p01, p12)
	if err := flattenQuad(dst, p0, p01, center, tolerance, depth+1); err != nil {
		return err
	}
	return flattenQuad(dst, center, p12, p2, tolerance, depth+1)
}

func flattenCubic(dst *[]math2d.Vec2, p0, p1, p2, p3 math2d.Vec2, tolerance float64, depth int) error {
	if math.Max(distanceToLine(p1, p0, p3), distanceToLine(p2, p0, p3)) <= tolerance {
		*dst = append(*dst, p3)
		return nil
	}
	if depth >= maxFlattenDepth {
		return ErrFlattenDepth
	}
	p01, p12, p23 := midpoint(p0, p1), midpoint(p1, p2), midpoint(p2, p3)
	p012, p123 := midpoint(p01, p12), midpoint(p12, p23)
	center := midpoint(p012, p123)
	if err := flattenCubic(dst, p0, p01, p012, center, tolerance, depth+1); err != nil {
		return err
	}
	return flattenCubic(dst, center, p123, p23, p3, tolerance, depth+1)
}

func midpoint(a, b math2d.Vec2) math2d.Vec2 {
	return math2d.Vec2{X: (a.X + b.X) * 0.5, Y: (a.Y + b.Y) * 0.5}
}

func distanceToLine(point, from, to math2d.Vec2) float64 {
	delta := to.Sub(from)
	length := math.Sqrt(delta.X*delta.X + delta.Y*delta.Y)
	if length == 0 {
		d := point.Sub(from)
		return math.Sqrt(d.X*d.X + d.Y*d.Y)
	}
	return math.Abs(delta.X*(from.Y-point.Y)-delta.Y*(from.X-point.X)) / length
}
