package tess

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// AppendArc tessellates a DrawArc command into solid-color triangles.
//
// The arc is emitted as a partial ring between radius-width and radius. Segment
// count is derived from the screen-space radius and angular sweep so the same
// visual error rule used for circles also applies to arcs.
func AppendArc(m *Mesh, c ctx, cmd render.DrawArc) {
	if !cmd.Transform.IsIdentity() {
		appendTransformedArc(m, c, cmd)
		return
	}

	start := len(m.Vertices)

	center := c.cam.WorldToScreen(cmd.Center)
	radius := c.cam.WorldScalarToScreen(cmd.Radius)
	width := c.cam.WorldScalarToScreen(cmd.Width)

	if radius <= 0 || width <= 0 || cmd.Color.A <= 0 {
		return
	}

	sweep := arcSweep(cmd.StartAngle, cmd.EndAngle)
	if sweep <= 0 {
		return
	}

	innerRadius := radius - width
	if innerRadius < 0 {
		innerRadius = 0
	}

	n := arcSegments(radius, sweep)
	appendArcRing(m, c, center, innerRadius, radius, cmd.StartAngle, sweep, n, cmd.Color)

	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}

// appendTransformedArc draws an arc under a non-identity transform. Arc points
// are generated in world space and transformed per vertex, then stroked as an
// open polyline (matching the raster backend), so the arc's angular position
// and shape follow rotation and shear rather than only its center moving.
func appendTransformedArc(m *Mesh, c ctx, cmd render.DrawArc) {
	radius := c.cam.WorldScalarToScreen(cmd.Radius)
	width := transformedStrokeWidth(c, cmd.Width, cmd.Transform)
	if radius <= 0 || width <= 0 || cmd.Color.A <= 0 {
		return
	}

	sweep := arcSweep(cmd.StartAngle, cmd.EndAngle)
	if sweep <= 0 {
		return
	}

	n := arcSegments(radius, sweep)
	if n <= 0 {
		return
	}

	start := len(m.Vertices)
	points := make([]math2d.Vec2, n+1)
	for i := 0; i <= n; i++ {
		a := cmd.StartAngle + sweep*float64(i)/float64(n)
		w := math2d.Vec2{X: cmd.Center.X + math.Cos(a)*cmd.Radius, Y: cmd.Center.Y + math.Sin(a)*cmd.Radius}
		points[i] = c.cam.WorldToScreen(cmd.Transform.Apply(w))
	}

	appendOpenStrokeScreen(m, c, points, width, cmd.Color)

	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}

func arcSweep(start, end float64) float64 {
	const tau = 2 * math.Pi

	start = normalizeAngle(start)
	end = normalizeAngle(end)

	if end >= start {
		return end - start
	}
	return end + tau - start
}

func normalizeAngle(angle float64) float64 {
	const tau = 2 * math.Pi

	angle = math.Mod(angle, tau)
	if angle < 0 {
		angle += tau
	}

	return angle
}

func arcSegments(screenRadius, sweep float64) int {
	if screenRadius <= 0 || sweep <= 0 {
		return 0
	}

	fullCircleSegments := circleSegments(screenRadius)
	n := int(math.Ceil(float64(fullCircleSegments) * sweep / (2 * math.Pi)))

	return clampInt(n, 4, fullCircleSegments)
}

func appendArcRing(
	m *Mesh,
	c ctx,
	center math2d.Vec2,
	innerRadius, outerRadius float64,
	startAngle, sweep float64,
	n int,
	col render.Color,
) {
	if n <= 0 {
		return
	}

	step := sweep / float64(n)

	for i := 0; i < n; i++ {
		a0 := startAngle + step*float64(i)
		a1 := startAngle + step*float64(i+1)

		outer0 := screenToClip(c, math2d.Vec2{X: center.X + math.Cos(a0)*outerRadius, Y: center.Y + math.Sin(a0)*outerRadius})
		outer1 := screenToClip(c, math2d.Vec2{X: center.X + math.Cos(a1)*outerRadius, Y: center.Y + math.Sin(a1)*outerRadius})
		inner1 := screenToClip(c, math2d.Vec2{X: center.X + math.Cos(a1)*innerRadius, Y: center.Y + math.Sin(a1)*innerRadius})
		inner0 := screenToClip(c, math2d.Vec2{X: center.X + math.Cos(a0)*innerRadius, Y: center.Y + math.Sin(a0)*innerRadius})

		pushQuad(m, outer0, outer1, inner1, inner0, col)
	}
}
