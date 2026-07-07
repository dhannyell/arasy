package tess

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

const (
	maxError          = 0.5
	minCircleSegments = 12
	maxCircleSegments = 192
)

// AppendCircle tessellates a DrawCircle command into solid-color triangles.
//
// The circle is approximated in screen space so visual quality follows the
// final pixel radius after camera scaling. Fill is emitted as a triangle fan;
// stroke is emitted as a ring of quads between the inner and outer radii.
// When cmd.Clip is non-zero, only the vertices emitted by this call are clipped.
func AppendCircle(m *Mesh, c ctx, cmd render.DrawCircle) {
	start := len(m.Vertices)

	center := c.cam.WorldToScreen(cmd.Center)
	radius := c.cam.WorldScalarToScreen(cmd.Radius)
	if radius <= 0 {
		return
	}

	n := circleSegments(radius)

	if cmd.Fill.A > 0 {
		appendCircleFan(m, c, center, radius, n, cmd.Fill)
	}

	if cmd.Stroke.A > 0 && cmd.StrokeWidth > 0 {
		strokeWidth := c.cam.WorldScalarToScreen(cmd.StrokeWidth)
		if strokeWidth > 0 {
			innerRadius := radius - strokeWidth
			if innerRadius < 0 {
				innerRadius = 0
			}
			appendCircleRing(m, c, center, innerRadius, radius, n, cmd.Stroke)
		}
	}

	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}

// circleSegments returns the deterministic segment count used to approximate a
// screen-space circle.
//
// The count is derived from a maximum pixel error between the true circle and
// each segment chord, then clamped to keep small circles readable and large
// circles from producing excessive geometry.
func circleSegments(screenRadius float64) int {
	if screenRadius <= 0 {
		return 0
	}

	if screenRadius <= maxError {
		return minCircleSegments
	}

	n := int(math.Ceil(math.Pi / math.Acos(1-maxError/screenRadius)))
	return clampInt(n, minCircleSegments, maxCircleSegments)
}

// appendCircleFan appends a filled circle as a fan of triangles.
//
// Each triangle starts at the center and spans one edge segment on the circle
// boundary. Points are generated in screen space and converted to clip space
// before being appended to the mesh.
func appendCircleFan(m *Mesh, c ctx, center math2d.Vec2, radius float64, n int, col render.Color) {
	clipCenter := screenToClip(c, center)

	for i := 0; i < n; i++ {
		a0 := 2 * math.Pi * float64(i) / float64(n)
		a1 := 2 * math.Pi * float64(i+1) / float64(n)

		p0 := screenToClip(c, math2d.Vec2{
			X: center.X + math.Cos(a0)*radius,
			Y: center.Y + math.Sin(a0)*radius,
		})
		p1 := screenToClip(c, math2d.Vec2{
			X: center.X + math.Cos(a1)*radius,
			Y: center.Y + math.Sin(a1)*radius,
		})

		pushTri(m, clipCenter, p0, p1, col)
	}
}

// appendCircleRing appends a stroked circle as a ring of quads.
//
// Each segment is represented by one quad between innerRadius and outerRadius;
// pushQuad expands that quad into two triangles for the solid-color mesh.
func appendCircleRing(m *Mesh, c ctx, center math2d.Vec2, innerRadius, outerRadius float64, n int, col render.Color) {
	for i := 0; i < n; i++ {
		a0 := 2 * math.Pi * float64(i) / float64(n)
		a1 := 2 * math.Pi * float64(i+1) / float64(n)

		outer0 := screenToClip(c, math2d.Vec2{X: center.X + math.Cos(a0)*outerRadius, Y: center.Y + math.Sin(a0)*outerRadius})
		outer1 := screenToClip(c, math2d.Vec2{X: center.X + math.Cos(a1)*outerRadius, Y: center.Y + math.Sin(a1)*outerRadius})
		inner1 := screenToClip(c, math2d.Vec2{X: center.X + math.Cos(a1)*innerRadius, Y: center.Y + math.Sin(a1)*innerRadius})
		inner0 := screenToClip(c, math2d.Vec2{X: center.X + math.Cos(a0)*innerRadius, Y: center.Y + math.Sin(a0)*innerRadius})

		pushQuad(m, outer0, outer1, inner1, inner0, col)
	}
}
