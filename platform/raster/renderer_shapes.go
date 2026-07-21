package raster

import (
	"image"
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func (r *Renderer) drawRect(img *image.RGBA, command render.DrawRect) {
	screenMin := r.Camera.WorldToScreen(command.Rect.Min)
	screenMax := r.Camera.WorldToScreen(command.Rect.Max)

	rect := image.Rect(
		int(screenMin.X),
		int(screenMin.Y),
		int(screenMax.X),
		int(screenMax.Y),
	)
	bounds := r.clipBounds(img, rect, command.Clip)

	if command.Fill.A > 0 {
		if command.CornerRadius > 0 {
			fillRoundedRect(img, rect, r.Camera.WorldScalarToScreen(command.CornerRadius), bounds, toRGBA(command.Fill))
		} else {
			fillRect(img, rect.Intersect(bounds), toRGBA(command.Fill))
		}
	}

	if command.Stroke.A > 0 && command.StrokeWidth > 0 {
		strokeWidth := r.Camera.WorldScalarToScreen(command.StrokeWidth)
		if command.CornerRadius > 0 {
			strokeRoundedRect(img, rect, r.Camera.WorldScalarToScreen(command.CornerRadius), strokeWidth, bounds, toRGBA(command.Stroke))
		} else {
			strokeRect(img, rect, int(math.Ceil(strokeWidth)), bounds, toRGBA(command.Stroke))
		}
	}
}

func (r *Renderer) drawCircle(img *image.RGBA, command render.DrawCircle) {
	center := r.Camera.WorldToScreen(command.Center)

	radius := r.Camera.WorldScalarToScreen(command.Radius)
	bounds := r.clipBounds(img, circleBounds(center, radius), command.Clip)

	if command.Fill.A > 0 {
		fillCircle(img, center, radius, bounds, toRGBA(command.Fill))
	}

	if command.Stroke.A > 0 && command.StrokeWidth > 0 {
		strokeWidth := r.Camera.WorldScalarToScreen(command.StrokeWidth)
		strokeCircle(img, center, radius, strokeWidth, bounds, toRGBA(command.Stroke))
	}
}

func (r *Renderer) drawEllipse(img *image.RGBA, command render.DrawEllipse) {
	center := r.Camera.WorldToScreen(command.Center)

	radiusX := r.Camera.WorldScalarToScreen(command.RadiusX)
	radiusY := r.Camera.WorldScalarToScreen(command.RadiusY)

	if radiusX <= 0 || radiusY <= 0 {
		return
	}

	bounds := r.clipBounds(img, ellipseBounds(center, radiusX, radiusY), command.Clip)

	if command.Fill.A > 0 {
		fillEllipse(img, center, radiusX, radiusY, bounds, toRGBA(command.Fill))
	}
	if command.Stroke.A > 0 && command.StrokeWidth > 0 {
		strokeWidth := r.Camera.WorldScalarToScreen(command.StrokeWidth)
		strokeEllipse(img, center, radiusX, radiusY, strokeWidth, bounds, toRGBA(command.Stroke))
	}
}

func (r *Renderer) drawPoint(img *image.RGBA, command render.DrawPoint) {
	center := r.Camera.WorldToScreen(command.Center)
	radius := r.Camera.WorldScalarToScreen(command.Radius)
	bounds := r.clipBounds(img, circleBounds(center, radius), command.Clip)

	fillCircle(img, center, radius, bounds, toRGBA(command.Color))
}

func (r *Renderer) drawLine(img *image.RGBA, command render.DrawLine) {
	from := r.Camera.WorldToScreen(command.From)
	to := r.Camera.WorldToScreen(command.To)

	width := r.Camera.WorldScalarToScreen(command.Width)
	bounds := r.clipBounds(img, lineBounds(from, to, width), command.Clip)

	drawLine(img, from, to, width, bounds, toRGBA(command.Color))
}

func (r *Renderer) drawArc(img *image.RGBA, command render.DrawArc) {
	center := r.Camera.WorldToScreen(command.Center)

	radius := r.Camera.WorldScalarToScreen(command.Radius)

	width := r.Camera.WorldScalarToScreen(command.Width)
	bounds := r.clipBounds(img, circleBounds(center, radius), command.Clip)

	strokeArc(img, center, radius, width, command.StartAngle, command.EndAngle, bounds, toRGBA(command.Color))
}

func (r *Renderer) drawPolygon(img *image.RGBA, command render.DrawPolygon) {
	if len(command.Points) < 3 {
		return
	}

	points := make([]math2d.Vec2, len(command.Points))
	for i, point := range command.Points {
		points[i] = r.Camera.WorldToScreen(point)
	}

	if command.Fill.A > 0 {
		fillPolygon(img, points, r.clipBounds(img, polygonBounds(points), command.Clip), toRGBA(command.Fill))
	}

	strokeWidth := r.Camera.WorldScalarToScreen(command.StrokeWidth)
	if command.Stroke.A <= 0 || strokeWidth <= 0 {
		return
	}

	for i := range points {
		from := points[i]
		to := points[(i+1)%len(points)]
		drawLine(img, from, to, strokeWidth, r.clipBounds(img, lineBounds(from, to, strokeWidth), command.Clip), toRGBA(command.Stroke))
	}
}
