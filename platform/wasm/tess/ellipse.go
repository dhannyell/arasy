package tess

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// AppendEllipse tessellates a DrawEllipse command into solid-color triangles.
//
// The ellipse is axis-aligned and approximated in screen space. Fill is a
// triangle fan; stroke is a ring of quads between inner and outer radii.
// Segment count is derived from the larger semi-axis so the tighter curvature
// of the minor axis is still adequately sampled.
func AppendEllipse(m *Mesh, c ctx, cmd render.DrawEllipse) {
	start := len(m.Vertices)
	center := c.cam.WorldToScreen(cmd.Center)
	rx := c.cam.WorldScalarToScreen(cmd.RadiusX)
	ry := c.cam.WorldScalarToScreen(cmd.RadiusY)
	if rx <= 0 || ry <= 0 {
		return
	}
	// Use the larger radius for segment count — guarantees the tighter
	// (minor-axis) curvature never exceeds maxError.
	n := circleSegments(max(rx, ry))
	if cmd.Fill.A > 0 {
		appendEllipseFan(m, c, center, rx, ry, n, cmd.Fill)
	}
	if cmd.Stroke.A > 0 && cmd.StrokeWidth > 0 {
		sw := c.cam.WorldScalarToScreen(cmd.StrokeWidth)
		if sw > 0 {
			innerRx := rx - sw
			innerRy := ry - sw
			if innerRx < 0 {
				innerRx = 0
			}
			if innerRy < 0 {
				innerRy = 0
			}
			appendEllipseRing(m, c, center, innerRx, innerRy, rx, ry, n, cmd.Stroke)
		}
	}
	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}
func appendEllipseFan(m *Mesh, c ctx, center math2d.Vec2, rx, ry float64, n int, col render.Color) {
	clipCenter := screenToClip(c, center)
	for i := range n {
		a0 := 2 * math.Pi * float64(i) / float64(n)
		a1 := 2 * math.Pi * float64(i+1) / float64(n)
		cos0, sin0 := math.Cos(a0), math.Sin(a0)
		cos1, sin1 := math.Cos(a1), math.Sin(a1)
		p0 := screenToClip(c, math2d.Vec2{
			X: center.X + cos0*rx,
			Y: center.Y + sin0*ry,
		})
		p1 := screenToClip(c, math2d.Vec2{
			X: center.X + cos1*rx,
			Y: center.Y + sin1*ry,
		})
		pushTri(m, clipCenter, p0, p1, col)
	}
}
func appendEllipseRing(m *Mesh, c ctx, center math2d.Vec2, innerRx, innerRy, outerRx, outerRy float64, n int, col render.Color) {
	for i := range n {
		a0 := 2 * math.Pi * float64(i) / float64(n)
		a1 := 2 * math.Pi * float64(i+1) / float64(n)
		cos0, sin0 := math.Cos(a0), math.Sin(a0)
		cos1, sin1 := math.Cos(a1), math.Sin(a1)
		outer0 := screenToClip(c, math2d.Vec2{X: center.X + cos0*outerRx, Y: center.Y + sin0*outerRy})
		outer1 := screenToClip(c, math2d.Vec2{X: center.X + cos1*outerRx, Y: center.Y + sin1*outerRy})
		inner1 := screenToClip(c, math2d.Vec2{X: center.X + cos1*innerRx, Y: center.Y + sin1*innerRy})
		inner0 := screenToClip(c, math2d.Vec2{X: center.X + cos0*innerRx, Y: center.Y + sin0*innerRy})
		pushQuad(m, outer0, outer1, inner1, inner0, col)
	}
}
