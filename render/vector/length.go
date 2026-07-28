package vector

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
)

func (p Path) PointAt(progress, tolerance float64) (math2d.Vec2, math2d.Vec2, error) {
	if math.IsNaN(progress) || math.IsInf(progress, 0) || progress < 0 || progress > 1 {
		return math2d.Vec2{}, math2d.Vec2{}, ErrInvalidProgress
	}
	contours, err := p.flatten(tolerance)
	if err != nil {
		return math2d.Vec2{}, math2d.Vec2{}, err
	}
	total := 0.0
	for _, contour := range contours {
		for index := 1; index < len(contour.points); index++ {
			total += distance(contour.points[index-1], contour.points[index])
		}
	}
	if total == 0 {
		return math2d.Vec2{}, math2d.Vec2{}, ErrDegenerateGeometry
	}
	target, walked := progress*total, 0.0
	for _, contour := range contours {
		for index := 1; index < len(contour.points); index++ {
			from, to := contour.points[index-1], contour.points[index]
			length := distance(from, to)
			if walked+length >= target || progress == 1 && walked+length == total {
				fraction := (target - walked) / length
				delta := to.Sub(from)
				return math2d.Vec2{X: from.X + delta.X*fraction, Y: from.Y + delta.Y*fraction},
					math2d.Vec2{X: delta.X / length, Y: delta.Y / length}, nil
			}
			walked += length
		}
	}
	return math2d.Vec2{}, math2d.Vec2{}, ErrDegenerateGeometry
}

func distance(a, b math2d.Vec2) float64 {
	delta := b.Sub(a)
	return math.Sqrt(delta.X*delta.X + delta.Y*delta.Y)
}
