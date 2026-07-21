package raster

import (
	"image"
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// fillStrokeClosedPolygon fills and strokes a screen-space polygon, the shared
// tail of every transformed-shape path. Fill is a scanline polygon; stroke is
// one line per edge (no miter joins — acceptable for decorative transforms).
// strokeWidth is already in screen pixels.
func (r *Renderer) fillStrokeClosedPolygon(img *image.RGBA, points []math2d.Vec2, fill, stroke render.Color, strokeWidth float64, clip math2d.Rect) {
	if fill.A > 0 {
		fillPolygon(img, points, r.clipBounds(img, polygonBounds(points), clip), toRGBA(fill))
	}
	if stroke.A > 0 && strokeWidth > 0 {
		for i := range points {
			from := points[i]
			to := points[(i+1)%len(points)]
			drawLine(img, from, to, strokeWidth, r.clipBounds(img, lineBounds(from, to, strokeWidth), clip), toRGBA(stroke))
		}
	}
}

// transformedStrokeWidth converts a world-space stroke width to screen pixels
// and scales it by the transform's mean linear factor, so a stroke thickens
// under scale the way its enclosing geometry does. This is an approximation:
// a non-uniform scale has no single stroke width, so sqrt(|det|) is used.
func (r *Renderer) transformedStrokeWidth(worldWidth float64, transform math2d.Affine) float64 {
	return r.Camera.WorldScalarToScreen(worldWidth) * transform.MeanScale()
}

// drawTransformedRect draws a rect with a non-identity transform. CornerRadius
// is ignored under transform (a rounded rect degrades to a sharp one); pivoted
// rounded panels are a follow-up when a consumer needs them.
func (r *Renderer) drawTransformedRect(img *image.RGBA, command render.DrawRect) {
	rect := command.Rect
	corners := [4]math2d.Vec2{
		command.Transform.Apply(math2d.Vec2{X: rect.Min.X, Y: rect.Min.Y}),
		command.Transform.Apply(math2d.Vec2{X: rect.Max.X, Y: rect.Min.Y}),
		command.Transform.Apply(math2d.Vec2{X: rect.Max.X, Y: rect.Max.Y}),
		command.Transform.Apply(math2d.Vec2{X: rect.Min.X, Y: rect.Max.Y}),
	}
	points := make([]math2d.Vec2, 4)
	for i, c := range corners {
		points[i] = r.Camera.WorldToScreen(c)
	}

	r.fillStrokeClosedPolygon(img, points, command.Fill, command.Stroke,
		r.transformedStrokeWidth(command.StrokeWidth, command.Transform), command.Clip)
}

// drawTransformedCircle draws a circle with a non-identity transform (e.g. scale, skew).
func (r *Renderer) drawTransformedCircle(img *image.RGBA, command render.DrawCircle) {
	center := command.Center
	radius := command.Radius
	n := circleSegmentCount(r.Camera.WorldScalarToScreen(radius))
	points := make([]math2d.Vec2, n)
	for i := 0; i < n; i++ {
		angle := 2 * math.Pi * float64(i) / float64(n)
		p := math2d.Vec2{X: center.X + math.Cos(angle)*radius, Y: center.Y + math.Sin(angle)*radius}
		points[i] = r.Camera.WorldToScreen(command.Transform.Apply(p))
	}

	r.fillStrokeClosedPolygon(img, points, command.Fill, command.Stroke,
		r.transformedStrokeWidth(command.StrokeWidth, command.Transform), command.Clip)
}

// circleSegmentCount returns the number of segments for a circle polygon approximation.
func circleSegmentCount(screenRadius float64) int {
	n := int(screenRadius)
	if n < 12 {
		n = 12
	}
	if n > 96 {
		n = 96
	}
	return n
}

// drawTransformedEllipse draws an ellipse with a non-identity transform.
func (r *Renderer) drawTransformedEllipse(img *image.RGBA, command render.DrawEllipse) {
	center := command.Center
	rx, ry := command.RadiusX, command.RadiusY
	n := circleSegmentCount((r.Camera.WorldScalarToScreen(rx) + r.Camera.WorldScalarToScreen(ry)) / 2)
	points := make([]math2d.Vec2, n)
	for i := 0; i < n; i++ {
		angle := 2 * math.Pi * float64(i) / float64(n)
		p := math2d.Vec2{X: center.X + math.Cos(angle)*rx, Y: center.Y + math.Sin(angle)*ry}
		points[i] = r.Camera.WorldToScreen(command.Transform.Apply(p))
	}

	r.fillStrokeClosedPolygon(img, points, command.Fill, command.Stroke,
		r.transformedStrokeWidth(command.StrokeWidth, command.Transform), command.Clip)
}

// drawTransformedLine draws a line with a non-identity transform.
func (r *Renderer) drawTransformedLine(img *image.RGBA, command render.DrawLine) {
	from := r.Camera.WorldToScreen(command.Transform.Apply(command.From))
	to := r.Camera.WorldToScreen(command.Transform.Apply(command.To))
	width := r.transformedStrokeWidth(command.Width, command.Transform)
	bounds := r.clipBounds(img, lineBounds(from, to, width), command.Clip)
	drawLine(img, from, to, width, bounds, toRGBA(command.Color))
}

// drawTransformedPolygon draws a polygon with a non-identity transform.
func (r *Renderer) drawTransformedPolygon(img *image.RGBA, command render.DrawPolygon) {
	if len(command.Points) < 3 {
		return
	}

	points := make([]math2d.Vec2, len(command.Points))
	for i, p := range command.Points {
		points[i] = r.Camera.WorldToScreen(command.Transform.Apply(p))
	}

	r.fillStrokeClosedPolygon(img, points, command.Fill, command.Stroke,
		r.transformedStrokeWidth(command.StrokeWidth, command.Transform), command.Clip)
}

// drawTransformedArc draws an arc with a non-identity transform.
func (r *Renderer) drawTransformedArc(img *image.RGBA, command render.DrawArc) {
	center := command.Center
	radius := command.Radius
	n := circleSegmentCount(r.Camera.WorldScalarToScreen(radius))
	// Determine how many segments cover the arc angle
	arcLen := command.EndAngle - command.StartAngle
	if arcLen < 0 {
		arcLen += 2 * math.Pi
	}
	n = int(float64(n) * arcLen / (2 * math.Pi))
	if n < 4 {
		n = 4
	}

	points := make([]math2d.Vec2, n+1)
	for i := 0; i <= n; i++ {
		t := command.StartAngle + arcLen*float64(i)/float64(n)
		p := math2d.Vec2{X: center.X + math.Cos(t)*radius, Y: center.Y + math.Sin(t)*radius}
		points[i] = r.Camera.WorldToScreen(command.Transform.Apply(p))
	}

	width := r.transformedStrokeWidth(command.Width, command.Transform)
	for i := 0; i < n; i++ {
		from := points[i]
		to := points[i+1]
		drawLine(img, from, to, width, r.clipBounds(img, lineBounds(from, to, width), command.Clip), toRGBA(command.Color))
	}
}
