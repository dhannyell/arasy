package math3d

import (
	"math"

	"github.com/dhannyell/arasy/dmath"
	"github.com/dhannyell/arasy/math2d"
)

// Camera projects 3D world points into a 2D viewport.
type Camera struct {
	Eye      Vec3        // Eye is the camera position in world units.
	Target   Vec3        // Target is the world point at the view center.
	FOV      float64     // FOV is the vertical field of view in radians.
	Near     float64     // Near is the nearest accepted view-space depth.
	Viewport math2d.Rect // Viewport is the destination rectangle in pixels.
}

// basis returns the camera view axes as orthonormal unit vectors.
// A zero-length view produces zero axes.
func (c Camera) basis() (right, up, forward Vec3) {
	forward = c.Target.Sub(c.Eye).Normalize()

	// At the exact poles (looking along ±Z), the ground axes take over:
	// top-down shows +Y (map north) up the screen.
	worldUp := Vec3{0, 0, 1}
	if math.Abs(forward.X) < 1e-6 && math.Abs(forward.Y) < 1e-6 {
		if forward.Z > 0 {
			worldUp = Vec3{0, -1, 0}
		} else {
			worldUp = Vec3{0, 1, 0}
		}
	}

	right = forward.Cross(worldUp).Normalize()
	up = right.Cross(forward)

	return right, up, forward
}

// ProjectPoint maps a point in front of the camera to viewport coordinates.
// It returns the view-space depth and false when the point is behind Near.
func (c Camera) ProjectPoint(p Vec3) (screen math2d.Vec2, depth float64, ok bool) {
	right, up, forward := c.basis()

	d := p.Sub(c.Eye)
	x := d.Dot(right)
	y := d.Dot(up)
	z := d.Dot(forward)
	if z <= c.Near {
		return math2d.Vec2{}, z, false
	}

	halfFOV := c.FOV / 2.0
	focal := dmath.Cos(halfFOV) / dmath.Sin(halfFOV)

	scale := float64(focal*(c.Viewport.Height()/2)) / z
	center := c.Viewport.Center()

	return math2d.Vec2{
		X: center.X + float64(x*scale),
		// Screen Y grows down; view-space up must go up the screen
		Y: center.Y - float64(y*scale),
	}, z, true
}
