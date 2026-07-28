package raster

import (
	"image"
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
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

// drawTransformedRect draws a rect with a non-identity transform. Rounded
// corners are built in local space before every boundary point is transformed,
// preserving CornerRadius under scale, rotation, and shear.
func (r *Renderer) drawTransformedRect(img *image.RGBA, command render.DrawRect) {
	rect := command.Rect
	points := []math2d.Vec2{
		{X: rect.Min.X, Y: rect.Min.Y},
		{X: rect.Max.X, Y: rect.Min.Y},
		{X: rect.Max.X, Y: rect.Max.Y},
		{X: rect.Min.X, Y: rect.Max.Y},
	}
	if command.CornerRadius > 0 {
		screenRadius := r.Camera.WorldScalarToScreen(command.CornerRadius) * command.Transform.MeanScale()
		points = transformedRoundedRectBoundary(rect, command.CornerRadius, circleSegmentCount(screenRadius)/4)
	}
	for i := range points {
		points[i] = r.Camera.WorldToScreen(command.Transform.Apply(points[i]))
	}

	r.fillStrokeClosedPolygon(img, points, command.Fill, command.Stroke,
		r.transformedStrokeWidth(command.StrokeWidth, command.Transform), command.Clip)
}

func transformedRoundedRectBoundary(rect math2d.Rect, radius float64, segments int) []math2d.Vec2 {
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
