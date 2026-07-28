package vector

import (
	"math"
	"slices"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

type LineCap uint8

const (
	CapButt LineCap = iota
	CapRound
	CapSquare
)

type LineJoin uint8

const (
	JoinMiter LineJoin = iota
	JoinRound
	JoinBevel
)

type StrokeStyle struct {
	Color      render.Color
	Width      float64
	LineCap    LineCap
	LineJoin   LineJoin
	MiterLimit float64
	TrimStart  float64
	TrimEnd    float64
	Tolerance  float64
}

const discSteps = 12

func (p Path) AppendStroke(dst []render.Command, base render.BaseCommand, style StrokeStyle) ([]render.Command, error) {
	if math.IsNaN(style.Width) || math.IsInf(style.Width, 0) || style.Width <= 0 ||
		style.LineCap > CapSquare || style.LineJoin > JoinBevel ||
		style.TrimStart < 0 || style.TrimStart > 1 || style.TrimEnd < 0 || style.TrimEnd > 1 ||
		style.TrimEnd != 0 && style.TrimEnd < style.TrimStart {
		return dst, ErrInvalidStroke
	}
	contours, err := p.flatten(style.Tolerance)
	if err != nil {
		return dst, err
	}
	trimEnd := style.TrimEnd
	if style.TrimStart == 0 && trimEnd == 0 {
		trimEnd = 1
	}
	var vertices []math2d.Vec2
	var indices []uint32
	for _, item := range contours {
		points, closed := trimContour(item.points, item.closed, style.TrimStart, trimEnd)
		if len(points) < 2 {
			continue
		}
		appendStrokeContour(&vertices, &indices, points, closed, style)
	}
	if len(indices) == 0 {
		return dst, nil
	}
	return append(dst, render.DrawTriangles{
		BaseCommand: base, Vertices: vertices, Indices: indices, Color: style.Color,
	}), nil
}

func trimContour(points []math2d.Vec2, closed bool, start, end float64) ([]math2d.Vec2, bool) {
	if start == 0 && end == 1 {
		return points, closed
	}
	total := 0.0
	for index := 1; index < len(points); index++ {
		total += distance(points[index-1], points[index])
	}
	if total == 0 || start == end {
		return nil, false
	}
	fromDistance, toDistance := start*total, end*total
	walked := 0.0
	out := make([]math2d.Vec2, 0, len(points))
	for index := 1; index < len(points); index++ {
		from, to := points[index-1], points[index]
		length := distance(from, to)
		segmentStart, segmentEnd := walked, walked+length
		if segmentEnd < fromDistance {
			walked = segmentEnd
			continue
		}
		if segmentStart > toDistance {
			break
		}
		a := math.Max(fromDistance, segmentStart)
		b := math.Min(toDistance, segmentEnd)
		if b >= a && length > 0 {
			delta := to.Sub(from)
			startPoint := math2d.Vec2{X: from.X + delta.X*(a-segmentStart)/length, Y: from.Y + delta.Y*(a-segmentStart)/length}
			endPoint := math2d.Vec2{X: from.X + delta.X*(b-segmentStart)/length, Y: from.Y + delta.Y*(b-segmentStart)/length}
			if len(out) == 0 || out[len(out)-1] != startPoint {
				out = append(out, startPoint)
			}
			if out[len(out)-1] != endPoint {
				out = append(out, endPoint)
			}
		}
		walked = segmentEnd
	}
	return out, false
}

func appendStrokeContour(vertices *[]math2d.Vec2, indices *[]uint32, points []math2d.Vec2, closed bool, style StrokeStyle) {
	if closed && len(points) > 1 && points[0] == points[len(points)-1] {
		points = points[:len(points)-1]
	}
	vertexGrowth, indexGrowth := strokeBufferGrowth(len(points), closed, style)
	*vertices = slices.Grow(*vertices, vertexGrowth)
	*indices = slices.Grow(*indices, indexGrowth)

	half := style.Width * 0.5
	segmentCount := len(points) - 1
	if closed {
		segmentCount = len(points)
	}
	for index := 0; index < segmentCount; index++ {
		from, to := points[index], points[(index+1)%len(points)]
		delta := to.Sub(from)
		length := distance(from, to)
		if length == 0 {
			continue
		}
		unit := math2d.Vec2{X: delta.X / length, Y: delta.Y / length}
		if !closed && style.LineCap == CapSquare {
			if index == 0 {
				from = from.Sub(math2d.Vec2{X: unit.X * half, Y: unit.Y * half})
			}
			if index == segmentCount-1 {
				to = to.Add(math2d.Vec2{X: unit.X * half, Y: unit.Y * half})
			}
		}
		normal := math2d.Vec2{X: -unit.Y * half, Y: unit.X * half}
		appendQuad(vertices, indices, from.Add(normal), to.Add(normal), to.Sub(normal), from.Sub(normal))
	}
	if style.LineJoin == JoinRound {
		first, last := 1, len(points)-1
		if closed {
			first, last = 0, len(points)-1
		}
		for index := first; index <= last; index++ {
			appendDisc(vertices, indices, points[index], half)
		}
	} else {
		first, last := 1, len(points)-2
		if closed {
			first, last = 0, len(points)-1
		}
		for index := first; index <= last; index++ {
			prev := points[(index+len(points)-1)%len(points)]
			center := points[index]
			next := points[(index+1)%len(points)]
			if style.LineJoin == JoinMiter {
				limit := style.MiterLimit
				if limit <= 0 {
					limit = 4
				}
				if appendMiterJoin(vertices, indices, prev, center, next, half, limit) {
					continue
				}
			}
			appendBevelJoin(vertices, indices, prev, center, next, half)
		}
	}
	if !closed && style.LineCap == CapRound {
		appendDisc(vertices, indices, points[0], half)
		appendDisc(vertices, indices, points[len(points)-1], half)
	}
}

func appendQuad(vertices *[]math2d.Vec2, indices *[]uint32, a, b, c, d math2d.Vec2) {
	base := uint32(len(*vertices))
	*vertices = append(*vertices, a, b, c, d)
	*indices = append(*indices, base, base+1, base+2, base, base+2, base+3)
}

func appendDisc(vertices *[]math2d.Vec2, indices *[]uint32, center math2d.Vec2, radius float64) {
	base := uint32(len(*vertices))
	*vertices = append(*vertices, center)
	for index := range discSteps {
		angle := float64(index) * 2 * math.Pi / discSteps
		*vertices = append(*vertices, math2d.Vec2{X: center.X + math.Cos(angle)*radius, Y: center.Y + math.Sin(angle)*radius})
	}
	for index := range discSteps {
		*indices = append(*indices, base, base+1+uint32(index), base+1+uint32((index+1)%discSteps))
	}
}

func strokeBufferGrowth(pointCount int, closed bool, style StrokeStyle) (vertices, indices int) {
	if pointCount < 2 {
		return 0, 0
	}
	segments := pointCount - 1
	joins := pointCount - 2
	if closed {
		segments = pointCount
		joins = pointCount
	}
	vertices, indices = segments*4, segments*6
	switch style.LineJoin {
	case JoinRound:
		roundJoins := joins
		if !closed {
			// appendStrokeContour includes the final point in its open round
			// join loop before adding the two endpoint caps.
			roundJoins = pointCount - 1
		}
		vertices += roundJoins * (discSteps + 1)
		indices += roundJoins * discSteps * 3
	case JoinMiter:
		// A successful miter emits a quad; a bevel fallback is smaller.
		vertices += joins * 4
		indices += joins * 6
	case JoinBevel:
		vertices += joins * 3
		indices += joins * 3
	}
	if !closed && style.LineCap == CapRound {
		vertices += 2 * (discSteps + 1)
		indices += 2 * discSteps * 3
	}
	return vertices, indices
}

func appendBevelJoin(vertices *[]math2d.Vec2, indices *[]uint32, prev, center, next math2d.Vec2, half float64) {
	a, b, ok := outerJoinPoints(prev, center, next, half)
	if !ok {
		return
	}
	base := uint32(len(*vertices))
	*vertices = append(*vertices, center, a, b)
	*indices = append(*indices, base, base+1, base+2)
}

func appendMiterJoin(vertices *[]math2d.Vec2, indices *[]uint32, prev, center, next math2d.Vec2, half, limit float64) bool {
	a, b, ok := outerJoinPoints(prev, center, next, half)
	if !ok {
		return false
	}
	in := center.Sub(prev)
	out := next.Sub(center)
	intersection, ok := lineIntersection(a, in, b, out)
	if !ok || distance(center, intersection) > half*limit {
		return false
	}
	// The miter join is a four-point fan from the polyline vertex to the
	// extended outside corner. Emitting only (a, intersection, b) leaves the
	// inner triangle (center, a, b) uncovered, which appears as a crack at
	// every flattened curve segment under MSAA.
	appendQuad(vertices, indices, center, a, intersection, b)
	return true
}

func outerJoinPoints(prev, center, next math2d.Vec2, half float64) (math2d.Vec2, math2d.Vec2, bool) {
	in, out := center.Sub(prev), next.Sub(center)
	inLength, outLength := distance(prev, center), distance(center, next)
	if inLength == 0 || outLength == 0 {
		return math2d.Vec2{}, math2d.Vec2{}, false
	}
	in = math2d.Vec2{X: in.X / inLength, Y: in.Y / inLength}
	out = math2d.Vec2{X: out.X / outLength, Y: out.Y / outLength}
	turn := cross(in, out)
	if math.Abs(turn) <= 1e-12 {
		return math2d.Vec2{}, math2d.Vec2{}, false
	}
	// The outside of a turn is opposite its signed normal: a positive
	// (counter-clockwise) turn opens on the right, while a negative turn
	// opens on the left. Choosing the inside leaves triangular cracks between
	// the independently tessellated segment quads.
	sign := -1.0
	if turn < 0 {
		sign = 1
	}
	a := center.Add(math2d.Vec2{X: -in.Y * half * sign, Y: in.X * half * sign})
	b := center.Add(math2d.Vec2{X: -out.Y * half * sign, Y: out.X * half * sign})
	return a, b, true
}

func lineIntersection(pointA, directionA, pointB, directionB math2d.Vec2) (math2d.Vec2, bool) {
	denominator := cross(directionA, directionB)
	if math.Abs(denominator) <= 1e-12 {
		return math2d.Vec2{}, false
	}
	t := cross(pointB.Sub(pointA), directionB) / denominator
	return pointA.Add(math2d.Vec2{X: directionA.X * t, Y: directionA.Y * t}), true
}
