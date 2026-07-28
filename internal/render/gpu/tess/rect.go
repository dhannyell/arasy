package tess

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// AppendRect converts a render.DrawRect command into solid-color triangles.
//
// Coordinates are converted from world space to screen space and then to clip
// space. Fill is emitted first, then stroke, matching the usual painter order.
//
// Rect strokes are generated inside the rectangle bounds.
// Rounded rectangles are approximated with deterministic corner segments.
func AppendRect(m *Mesh, c ctx, cmd render.DrawRect) {
	if !cmd.Transform.IsIdentity() {
		appendTransformedRect(m, c, cmd)
		return
	}

	start := len(m.Vertices)

	rMin := c.cam.WorldToScreen(cmd.Rect.Min)
	rMax := c.cam.WorldToScreen(cmd.Rect.Max)

	x0 := math.Min(rMin.X, rMax.X)
	y0 := math.Min(rMin.Y, rMax.Y)
	x1 := math.Max(rMin.X, rMax.X)
	y1 := math.Max(rMin.Y, rMax.Y)

	w := x1 - x0
	h := y1 - y0
	if w <= 0 || h <= 0 {
		return
	}

	if cmd.CornerRadius <= 0 {
		appendSquareRect(m, c, x0, y0, x1, y1, cmd)
	} else {
		appendRoundedRect(m, c, x0, y0, x1, y1, cmd)
	}

	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}

// appendTransformedRect draws a rect under a non-identity transform. Rounded
// corners are tessellated in local space before every boundary point is
// transformed, so scale, rotation, and shear preserve CornerRadius.
func appendTransformedRect(m *Mesh, c ctx, cmd render.DrawRect) {
	start := len(m.Vertices)

	r := cmd.Rect
	points := []math2d.Vec2{
		{X: r.Min.X, Y: r.Min.Y},
		{X: r.Max.X, Y: r.Min.Y},
		{X: r.Max.X, Y: r.Max.Y},
		{X: r.Min.X, Y: r.Max.Y},
	}
	if cmd.CornerRadius > 0 {
		screenRadius := c.cam.WorldScalarToScreen(cmd.CornerRadius) * cmd.Transform.MeanScale()
		points = roundedRectBoundary(r, cmd.CornerRadius, clampInt(int(math.Ceil(screenRadius)), 4, 24))
	}
	for i := range points {
		points[i] = c.cam.WorldToScreen(cmd.Transform.Apply(points[i]))
	}

	if cmd.Fill.A > 0 {
		appendPolygonFill(m, c, points, cmd.Fill)
	}

	if cmd.Stroke.A > 0 {
		if sw := transformedStrokeWidth(c, cmd.StrokeWidth, cmd.Transform); sw > 0 {
			appendClosedStrokeScreen(m, c, points, sw, cmd.Stroke)
		}
	}

	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}

func roundedRectBoundary(rect math2d.Rect, radius float64, segments int) []math2d.Vec2 {
	radius = math.Min(radius, math.Min(rect.Width(), rect.Height())/2)
	if radius <= 0 {
		return []math2d.Vec2{
			{X: rect.Min.X, Y: rect.Min.Y},
			{X: rect.Max.X, Y: rect.Min.Y},
			{X: rect.Max.X, Y: rect.Max.Y},
			{X: rect.Min.X, Y: rect.Max.Y},
		}
	}

	centers := [4]math2d.Vec2{
		{X: rect.Min.X + radius, Y: rect.Min.Y + radius},
		{X: rect.Max.X - radius, Y: rect.Min.Y + radius},
		{X: rect.Max.X - radius, Y: rect.Max.Y - radius},
		{X: rect.Min.X + radius, Y: rect.Max.Y - radius},
	}
	startAngles := [4]float64{math.Pi, 1.5 * math.Pi, 0, 0.5 * math.Pi}
	points := make([]math2d.Vec2, 0, 4*(segments+1)-1)
	for corner, center := range centers {
		for step := 0; step <= segments; step++ {
			if corner == len(centers)-1 && step == segments {
				break
			}
			angle := startAngles[corner] + 0.5*math.Pi*float64(step)/float64(segments)
			points = append(points, math2d.Vec2{
				X: center.X + math.Cos(angle)*radius,
				Y: center.Y + math.Sin(angle)*radius,
			})
		}
	}
	return points
}

func appendSquareRect(m *Mesh, c ctx, x0, y0, x1, y1 float64, cmd render.DrawRect) {
	if cmd.Fill.A > 0 {
		pushScreenQuad(m, c, x0, y0, x1, y0, x1, y1, x0, y1, cmd.Fill)
	}

	if cmd.Stroke.A <= 0 || cmd.StrokeWidth <= 0 {
		return
	}

	sw := c.cam.WorldScalarToScreen(cmd.StrokeWidth)
	if sw <= 0 {
		return
	}

	sw = math.Min(sw, math.Min(x1-x0, y1-y0))

	// Top.
	pushScreenQuad(m, c, x0, y0, x1, y0, x1, y0+sw, x0, y0+sw, cmd.Stroke)

	// Right.
	pushScreenQuad(m, c, x1-sw, y0, x1, y0, x1, y1, x1-sw, y1, cmd.Stroke)

	// Bottom.
	pushScreenQuad(m, c, x0, y1-sw, x1, y1-sw, x1, y1, x0, y1, cmd.Stroke)

	// Left.
	pushScreenQuad(m, c, x0, y0, x0+sw, y0, x0+sw, y1, x0, y1, cmd.Stroke)
}

func appendRoundedRect(m *Mesh, c ctx, x0, y0, x1, y1 float64, cmd render.DrawRect) {
	w := x1 - x0
	h := y1 - y0

	r := c.cam.WorldScalarToScreen(cmd.CornerRadius)
	r = math.Min(r, math.Min(w, h)/2)
	if r <= 0 {
		appendSquareRect(m, c, x0, y0, x1, y1, cmd)
		return
	}

	nSeg := clampInt(int(math.Ceil(r)), 4, 24)

	if cmd.Fill.A > 0 {
		// Center.
		pushScreenQuad(m, c, x0+r, y0, x1-r, y0, x1-r, y1, x0+r, y1, cmd.Fill)

		// Left and right middle strips.
		pushScreenQuad(m, c, x0, y0+r, x0+r, y0+r, x0+r, y1-r, x0, y1-r, cmd.Fill)
		pushScreenQuad(m, c, x1-r, y0+r, x1, y0+r, x1, y1-r, x1-r, y1-r, cmd.Fill)

		// Corners, angles are in screen space where Y grows downward.
		appendCornerFan(m, c, x0+r, y0+r, r, math.Pi, 1.5*math.Pi, nSeg, cmd.Fill)
		appendCornerFan(m, c, x1-r, y0+r, r, 1.5*math.Pi, 2*math.Pi, nSeg, cmd.Fill)
		appendCornerFan(m, c, x1-r, y1-r, r, 0, 0.5*math.Pi, nSeg, cmd.Fill)
		appendCornerFan(m, c, x0+r, y1-r, r, 0.5*math.Pi, math.Pi, nSeg, cmd.Fill)
	}

	if cmd.Stroke.A <= 0 || cmd.StrokeWidth <= 0 {
		return
	}

	sw := c.cam.WorldScalarToScreen(cmd.StrokeWidth)
	if sw <= 0 {
		return
	}

	sw = math.Min(sw, math.Min(w, h)/2)
	innerR := math.Max(0, r-sw)

	// Straight stroke parts.
	pushScreenQuad(m, c, x0+r, y0, x1-r, y0, x1-r, y0+sw, x0+r, y0+sw, cmd.Stroke)
	pushScreenQuad(m, c, x1-sw, y0+r, x1, y0+r, x1, y1-r, x1-sw, y1-r, cmd.Stroke)
	pushScreenQuad(m, c, x0+r, y1-sw, x1-r, y1-sw, x1-r, y1, x0+r, y1, cmd.Stroke)
	pushScreenQuad(m, c, x0, y0+r, x0+sw, y0+r, x0+sw, y1-r, x0, y1-r, cmd.Stroke)

	// Rounded corner stroke rings.
	appendCornerRing(m, c, x0+r, y0+r, innerR, r, math.Pi, 1.5*math.Pi, nSeg, cmd.Stroke)
	appendCornerRing(m, c, x1-r, y0+r, innerR, r, 1.5*math.Pi, 2*math.Pi, nSeg, cmd.Stroke)
	appendCornerRing(m, c, x1-r, y1-r, innerR, r, 0, 0.5*math.Pi, nSeg, cmd.Stroke)
	appendCornerRing(m, c, x0+r, y1-r, innerR, r, 0.5*math.Pi, math.Pi, nSeg, cmd.Stroke)
}

func appendCornerFan(m *Mesh, c ctx, cx, cy, r, a0, a1 float64, n int, col render.Color) {
	center := screenToClip(c, math2d.Vec2{X: cx, Y: cy})

	for i := 0; i < n; i++ {
		t0 := a0 + (a1-a0)*float64(i)/float64(n)
		t1 := a0 + (a1-a0)*float64(i+1)/float64(n)

		p0 := screenToClip(c, math2d.Vec2{X: cx + math.Cos(t0)*r, Y: cy + math.Sin(t0)*r})
		p1 := screenToClip(c, math2d.Vec2{X: cx + math.Cos(t1)*r, Y: cy + math.Sin(t1)*r})

		pushTri(m, center, p0, p1, col)
	}
}

func appendCornerRing(m *Mesh, c ctx, cx, cy, innerR, outerR, a0, a1 float64, n int, col render.Color) {
	for i := 0; i < n; i++ {
		t0 := a0 + (a1-a0)*float64(i)/float64(n)
		t1 := a0 + (a1-a0)*float64(i+1)/float64(n)

		outer0 := screenToClip(c, math2d.Vec2{X: cx + math.Cos(t0)*outerR, Y: cy + math.Sin(t0)*outerR})
		outer1 := screenToClip(c, math2d.Vec2{X: cx + math.Cos(t1)*outerR, Y: cy + math.Sin(t1)*outerR})
		inner1 := screenToClip(c, math2d.Vec2{X: cx + math.Cos(t1)*innerR, Y: cy + math.Sin(t1)*innerR})
		inner0 := screenToClip(c, math2d.Vec2{X: cx + math.Cos(t0)*innerR, Y: cy + math.Sin(t0)*innerR})

		pushQuad(m, outer0, outer1, inner1, inner0, col)
	}
}

func pushScreenQuad(
	m *Mesh,
	c ctx,
	ax, ay float64,
	bx, by float64,
	cx, cy float64,
	dx, dy float64,
	col render.Color,
) {
	pushQuad(
		m,
		screenToClip(c, math2d.Vec2{X: ax, Y: ay}),
		screenToClip(c, math2d.Vec2{X: bx, Y: by}),
		screenToClip(c, math2d.Vec2{X: cx, Y: cy}),
		screenToClip(c, math2d.Vec2{X: dx, Y: dy}),
		col,
	)
}

func screenToClip(c ctx, p math2d.Vec2) [2]float32 {
	return [2]float32{
		float32(2*p.X/c.w - 1),
		float32(1 - 2*p.Y/c.h),
	}
}

func clipRect(c ctx, r math2d.Rect) [4]float32 {
	a := screenToClip(c, c.cam.WorldToScreen(r.Min))
	b := screenToClip(c, c.cam.WorldToScreen(r.Max))

	return [4]float32{
		min32(a[0], b[0]),
		max32(a[0], b[0]),
		min32(a[1], b[1]),
		max32(a[1], b[1]),
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
