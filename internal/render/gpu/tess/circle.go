package tess

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

const (
	// maxError deliberately keeps chord deviation below one eighth of a
	// screen pixel. A 0.5px tolerance left visible edge artifacts on the
	// large overlapping circles in the native default splash, even with
	// 4x MSAA. BenchmarkTessSplashCircles tracks the cost of this quality
	// requirement separately from the boundary-anchored fan seam fix.
	maxError          = 0.125
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
	if c.perVertex(cmd.Transform) {
		appendTransformedCircle(m, c, cmd)
		return
	}

	// A circle lies in the world, so a tilted camera draws it as an ellipse.
	if c.cam.Foreshortens() {
		AppendEllipse(m, c, render.DrawEllipse{
			Center:      cmd.Center,
			RadiusX:     cmd.Radius,
			RadiusY:     cmd.Radius,
			Fill:        cmd.Fill,
			Stroke:      cmd.Stroke,
			StrokeWidth: cmd.StrokeWidth,
			BaseCommand: cmd.BaseCommand,
		})
		return
	}

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

// appendTransformedCircle draws a circle under a non-identity transform. The
// boundary is generated in world space and every vertex is transformed, so a
// non-uniform scale or shear turns the circle into the correct ellipse/oval
// (matching the raster backend), rather than only relocating the center.
func appendTransformedCircle(m *Mesh, c ctx, cmd render.DrawCircle) {
	radius := c.cam.WorldScalarToScreen(cmd.Radius)
	if radius <= 0 {
		return
	}

	start := len(m.Vertices)

	// The view is part of how big this ends up: scaling it in only after the
	// segment count was chosen draws a large circle as a dodecagon. MeanScale
	// is 1 without a view, so nothing else moves.
	n := circleSegments(radius * c.cam.View.MeanScale())

	center := c.cam.WorldToScreen(cmd.Transform.Apply(cmd.Center))
	boundary := make([]math2d.Vec2, n)
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		w := math2d.Vec2{X: cmd.Center.X + math.Cos(a)*cmd.Radius, Y: cmd.Center.Y + math.Sin(a)*cmd.Radius}
		boundary[i] = c.cam.WorldToScreen(cmd.Transform.Apply(w))
	}

	if cmd.Fill.A > 0 {
		appendConvexFanScreen(m, c, center, boundary, cmd.Fill)
	}

	if cmd.Stroke.A > 0 && cmd.StrokeWidth > 0 {
		if sw := transformedStrokeWidth(c, cmd.StrokeWidth, cmd.Transform); sw > 0 {
			appendClosedStrokeScreen(m, c, boundary, sw, cmd.Stroke)
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

// appendCircleFan appends a filled circle as a convex polygon triangulated
// from one boundary vertex.
//
// A center fan gives every triangle a radial edge. Under multisampling, some
// WebGPU implementations expose the closing radial edge as a hairline when
// the circle overlays another translucent shape. Anchoring at the boundary
// represents the exact same convex polygon without that center junction.
func appendCircleFan(m *Mesh, c ctx, center math2d.Vec2, radius float64, n int, col render.Color) {
	cosine, sine := circleUnit(0, n)
	anchor := screenToClip(c, math2d.Vec2{
		X: center.X + cosine*radius,
		Y: center.Y + sine*radius,
	})
	for i := 1; i < n-1; i++ {
		cos0, sin0 := circleUnit(i, n)
		cos1, sin1 := circleUnit(i+1, n)
		p0 := screenToClip(c, math2d.Vec2{X: center.X + cos0*radius, Y: center.Y + sin0*radius})
		p1 := screenToClip(c, math2d.Vec2{X: center.X + cos1*radius, Y: center.Y + sin1*radius})
		pushTri(m, anchor, p0, p1, col)
	}
}

// appendCircleRing appends a stroked circle as a ring of quads.
//
// Each segment is represented by one quad between innerRadius and outerRadius;
// pushQuad expands that quad into two triangles for the solid-color mesh.
func appendCircleRing(m *Mesh, c ctx, center math2d.Vec2, innerRadius, outerRadius float64, n int, col render.Color) {
	for i := range n {
		cos0, sin0 := circleUnit(i, n)
		cos1, sin1 := circleUnit(i+1, n)

		outer0 := screenToClip(c, math2d.Vec2{X: center.X + cos0*outerRadius, Y: center.Y + sin0*outerRadius})
		outer1 := screenToClip(c, math2d.Vec2{X: center.X + cos1*outerRadius, Y: center.Y + sin1*outerRadius})
		inner1 := screenToClip(c, math2d.Vec2{X: center.X + cos1*innerRadius, Y: center.Y + sin1*innerRadius})
		inner0 := screenToClip(c, math2d.Vec2{X: center.X + cos0*innerRadius, Y: center.Y + sin0*innerRadius})

		pushQuad(m, outer0, outer1, inner1, inner0, col)
	}
}

// circleUnit returns an exact shared endpoint for the first and closing
// segments. Evaluating sin(2*pi) leaves a tiny non-zero value which can make
// multisample rasterizers expose one sample along the closing radial edge.
func circleUnit(i, n int) (cosine, sine float64) {
	if i == 0 || i == n {
		return 1, 0
	}
	sine, cosine = math.Sincos(2 * math.Pi * float64(i) / float64(n))
	return cosine, sine
}
