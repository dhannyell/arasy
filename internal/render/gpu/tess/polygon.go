package tess

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// AppendPolygon tessellates a DrawPolygon command into solid-color triangles.
//
// Filled polygons are triangulated with ear clipping so concave shapes are
// supported. Stroke is emitted as one quad per edge, matching the simple
// screen-space line behavior used by the raster backend.
func AppendPolygon(m *Mesh, c ctx, cmd render.DrawPolygon) {
	if len(cmd.Points) < 3 {
		return
	}

	start := len(m.Vertices)

	points := make([]math2d.Vec2, len(cmd.Points))
	for i, point := range cmd.Points {
		points[i] = c.cam.WorldToScreen(cmd.Transform.Apply(point))
	}

	if cmd.Fill.A > 0 {
		appendPolygonFill(m, c, points, cmd.Fill)
	}

	strokeWidth := transformedStrokeWidth(c, cmd.StrokeWidth, cmd.Transform)
	if cmd.Stroke.A > 0 && strokeWidth > 0 {
		appendPolygonStroke(m, c, points, strokeWidth, cmd.Stroke)
	}

	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}

func appendPolygonFill(m *Mesh, c ctx, points []math2d.Vec2, col render.Color) {
	tris := earClip(points)

	for _, tri := range tris {
		pushTri(
			m,
			screenToClip(c, tri[0]),
			screenToClip(c, tri[1]),
			screenToClip(c, tri[2]),
			col,
		)
	}
}

func appendPolygonStroke(m *Mesh, c ctx, points []math2d.Vec2, width float64, col render.Color) {
	for i := range points {
		from := points[i]
		to := points[(i+1)%len(points)]
		appendLineQuad(m, c, from, to, width, col)
	}
}
