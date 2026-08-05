package tess

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// AppendLine tessellates a DrawLine command into a screen-space quad.
//
// The line width is measured after camera scaling, so the resulting geometry
// matches the rendered pixel thickness. Degenerate zero-length lines are
// skipped here; point-like line caps are outside the WebGPU v1 tessellation
// scope.
func AppendLine(m *Mesh, c ctx, cmd render.DrawLine) {
	if cmd.Color.A <= 0 || cmd.Width <= 0 {
		return
	}

	start := len(m.Vertices)

	from := c.cam.WorldToScreen(cmd.Transform.Apply(cmd.From))
	to := c.cam.WorldToScreen(cmd.Transform.Apply(cmd.To))
	width := transformedStrokeWidth(c, cmd.Width, cmd.Transform)

	appendLineQuad(m, c, from, to, width, cmd.Color)

	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}

// AppendPoint tessellates a DrawPoint command as a filled circle.
func AppendPoint(m *Mesh, c ctx, cmd render.DrawPoint) {
	if cmd.Color.A <= 0 || cmd.Radius <= 0 {
		return
	}

	// A point is a small circle, so a skewing camera slants it like one:
	// rebuild it per vertex or it stays axis-aligned while every other shape
	// around it shears.
	if c.perVertex(cmd.Transform) {
		AppendCircle(m, c, render.DrawCircle{
			Center:      cmd.Center,
			Radius:      cmd.Radius,
			Fill:        cmd.Color,
			BaseCommand: cmd.BaseCommand,
		})
		return
	}

	start := len(m.Vertices)

	center := c.cam.WorldToScreen(cmd.Center)
	radii := c.cam.WorldRadiiToScreen(math2d.Vec2{X: cmd.Radius, Y: cmd.Radius})
	if radii.X <= 0 || radii.Y <= 0 {
		return
	}

	appendEllipseFan(m, c, center, radii.X, radii.Y, circleSegments(max(radii.X, radii.Y)), cmd.Color)

	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}

func appendLineQuad(m *Mesh, c ctx, from, to math2d.Vec2, width float64, col render.Color) {
	dx := to.X - from.X
	dy := to.Y - from.Y
	length := math.Hypot(dx, dy)
	if length == 0 || width <= 0 {
		return
	}

	halfWidth := math.Max(1, width) * 0.5

	nx := -dy / length * halfWidth
	ny := dx / length * halfWidth

	a := math2d.Vec2{X: from.X + nx, Y: from.Y + ny}
	b := math2d.Vec2{X: to.X + nx, Y: to.Y + ny}
	cc := math2d.Vec2{X: to.X - nx, Y: to.Y - ny}
	d := math2d.Vec2{X: from.X - nx, Y: from.Y - ny}

	pushQuad(
		m,
		screenToClip(c, a),
		screenToClip(c, b),
		screenToClip(c, cc),
		screenToClip(c, d),
		col,
	)
}
