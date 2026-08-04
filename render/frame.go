package render

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
)

// FrameGround is the Camera.World that shows a ground rectangle under view:
// the box around the ground's projected corners, grown by margin on every
// side. Margin is a fraction of the framed size, so 0.15 reads as "the board
// plus fifteen percent of air".
//
// It exists because Camera.World frames view space, not world space. Under a
// skewing view a square board projects to a diamond, and the numbers that
// frame a diamond are exactly the kind a game should never have to work out
// by eye.
func FrameGround(view math2d.Affine, ground math2d.Rect, margin float64) math2d.Rect {
	min := view.Apply(ground.Min)
	max := min
	for _, corner := range []math2d.Vec2{
		{X: ground.Max.X, Y: ground.Min.Y},
		{X: ground.Min.X, Y: ground.Max.Y},
		ground.Max,
	} {
		projected := view.Apply(corner)
		min = math2d.Vec2{X: math.Min(min.X, projected.X), Y: math.Min(min.Y, projected.Y)}
		max = math2d.Vec2{X: math.Max(max.X, projected.X), Y: math.Max(max.Y, projected.Y)}
	}

	air := math2d.Vec2{X: (max.X - min.X) * margin, Y: (max.Y - min.Y) * margin}
	return math2d.NewRectFromCorners(min.Sub(air), max.Add(air))
}
