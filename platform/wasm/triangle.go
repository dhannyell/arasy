package wasm

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
)

const triangleMiterLimit = 4

type offsetLine struct {
	point     math2d.Vec2
	direction math2d.Vec2
}

// inflateTriangle offsets every edge outwards by amount. Canvas2D uses the
// result only as an overlapping clip, so texture coordinates and the original
// affine mapping remain unchanged.
func inflateTriangle(triangle [3]math2d.Vec2, amount float64) [3]math2d.Vec2 {
	if amount <= 0 {
		return triangle
	}

	center := math2d.Vec2{
		X: (triangle[0].X + triangle[1].X + triangle[2].X) / 3,
		Y: (triangle[0].Y + triangle[1].Y + triangle[2].Y) / 3,
	}
	var lines [3]offsetLine
	for i := range triangle {
		start := triangle[i]
		end := triangle[(i+1)%len(triangle)]
		direction := end.Sub(start)
		length := math.Hypot(direction.X, direction.Y)
		if length == 0 {
			return triangle
		}
		normal := math2d.Vec2{X: -direction.Y / length, Y: direction.X / length}
		toCenter := center.Sub(start)
		if normal.X*toCenter.X+normal.Y*toCenter.Y > 0 {
			normal = normal.Mul(-1)
		}
		lines[i] = offsetLine{
			point:     start.Add(normal.Mul(amount)),
			direction: direction,
		}
	}

	result := triangle
	for i := range triangle {
		previous := lines[(i+len(lines)-1)%len(lines)]
		current := lines[i]
		intersection, ok := intersectLines(previous, current)
		if !ok {
			continue
		}
		delta := intersection.Sub(triangle[i])
		maxMiter := amount * triangleMiterLimit
		length := math.Hypot(delta.X, delta.Y)
		if length > maxMiter {
			intersection = triangle[i].Add(delta.Mul(maxMiter / length))
		}
		result[i] = intersection
	}
	return result
}

func intersectLines(a, b offsetLine) (math2d.Vec2, bool) {
	denominator := cross(a.direction, b.direction)
	if math.Abs(denominator) <= 1e-12 {
		return math2d.Vec2{}, false
	}
	delta := b.point.Sub(a.point)
	t := cross(delta, b.direction) / denominator
	return a.point.Add(a.direction.Mul(t)), true
}

func cross(a, b math2d.Vec2) float64 {
	return a.X*b.Y - a.Y*b.X
}
