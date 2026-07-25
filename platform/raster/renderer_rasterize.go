package raster

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"

	"github.com/dhannyell/arasy/math2d"
)

func drawLine(img *image.RGBA, from, to math2d.Vec2, width float64, bounds image.Rectangle, c color.RGBA) {
	if width <= 0 {
		return
	}

	line := to.Sub(from)
	length := line.Len()
	if length == 0 {
		fillCircle(img, from, width*0.5, bounds, c)
		return
	}

	radius := width * 0.5
	lengthSq := line.LenSq()
	radiusSq := radius * radius

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			point := math2d.Vec2{
				X: float64(x) + 0.5,
				Y: float64(y) + 0.5,
			}

			t := point.Sub(from).Dot(line) / lengthSq
			if t < 0 || t > 1 {
				continue
			}

			closest := from.Lerp(to, t)
			if point.DistanceSq(closest) <= radiusSq {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func fillPolygon(img *image.RGBA, points []math2d.Vec2, bounds image.Rectangle, c color.RGBA) {
	if len(points) < 3 {
		return
	}

	minY := points[0].Y
	maxY := points[0].Y
	for _, point := range points[1:] {
		minY = min(minY, point.Y)
		maxY = max(maxY, point.Y)
	}

	bounds = bounds.Intersect(image.Rect(
		bounds.Min.X,
		int(math.Floor(minY)),
		bounds.Max.X,
		int(math.Ceil(maxY))+1,
	))

	intersections := make([]float64, 0, len(points))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		scanY := float64(y) + 0.5
		intersections = intersections[:0]

		for i := range points {
			from := points[i]
			to := points[(i+1)%len(points)]
			if from.Y == to.Y {
				continue
			}

			edgeMinY := min(from.Y, to.Y)
			edgeMaxY := max(from.Y, to.Y)
			if scanY < edgeMinY || scanY >= edgeMaxY {
				continue
			}

			t := (scanY - from.Y) / (to.Y - from.Y)
			intersections = append(intersections, from.X+t*(to.X-from.X))
		}

		sort.Float64s(intersections)
		for i := 0; i+1 < len(intersections); i += 2 {
			startX := int(math.Ceil(intersections[i] - 0.5))
			endX := int(math.Ceil(intersections[i+1] - 0.5))
			if startX < bounds.Min.X {
				startX = bounds.Min.X
			}
			if endX > bounds.Max.X {
				endX = bounds.Max.X
			}
			for x := startX; x < endX; x++ {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func fillRect(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	draw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, draw.Over)
}

func fillCircle(img *image.RGBA, center math2d.Vec2, radius float64, bounds image.Rectangle, c color.RGBA) {
	radiusSq := radius * radius
	bounds = bounds.Intersect(circleBounds(center, radius))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) + 0.5 - center.X
			dy := float64(y) + 0.5 - center.Y

			if dx*dx+dy*dy <= radiusSq {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func fillEllipse(img *image.RGBA, center math2d.Vec2, radiusX, radiusY float64, bounds image.Rectangle, c color.RGBA) {
	rxSq := radiusX * radiusX
	rySq := radiusY * radiusY
	rr := rxSq * rySq
	bounds = bounds.Intersect(ellipseBounds(center, radiusX, radiusY))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) + 0.5 - center.X
			dy := float64(y) + 0.5 - center.Y
			if float64(dx*dx)*rySq+float64(dy*dy)*rxSq <= rr {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func strokeRect(img *image.RGBA, rect image.Rectangle, width int, bounds image.Rectangle, c color.RGBA) {
	if width <= 0 || rect.Empty() {
		return
	}

	// A stroke wide enough to cover the interior is a filled rect. Falling
	// back also keeps the bands below from overlapping each other.
	if width*2 >= rect.Dx() || width*2 >= rect.Dy() {
		fillRect(img, rect.Intersect(bounds), c)
		return
	}

	// The four bands must not overlap: fillRect composites with "over", so
	// any shared pixel (the corners, in the naive full-height side bands)
	// would be blended twice and come out darker with a translucent stroke.

	// top
	fillRect(img, image.Rect(
		rect.Min.X,
		rect.Min.Y,
		rect.Max.X,
		rect.Min.Y+width).Intersect(bounds), c)

	// bottom
	fillRect(img, image.Rect(
		rect.Min.X,
		rect.Max.Y-width,
		rect.Max.X,
		rect.Max.Y).Intersect(bounds), c)

	// left (between the top and bottom bands)
	fillRect(img, image.Rect(
		rect.Min.X,
		rect.Min.Y+width,
		rect.Min.X+width,
		rect.Max.Y-width).Intersect(bounds), c)

	// right (between the top and bottom bands)
	fillRect(img, image.Rect(
		rect.Max.X-width,
		rect.Min.Y+width,
		rect.Max.X,
		rect.Max.Y-width).Intersect(bounds), c)
}

func strokeCircle(img *image.RGBA, center math2d.Vec2, radius float64, width float64, bounds image.Rectangle, c color.RGBA) {
	if radius <= 0 || width <= 0 {
		return
	}

	outerRadius := radius
	innerRadius := radius - width
	if innerRadius < 0 {
		innerRadius = 0
	}

	outerSq := outerRadius * outerRadius
	innerSq := innerRadius * innerRadius

	bounds = bounds.Intersect(circleBounds(center, outerRadius))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) + 0.5 - center.X
			dy := float64(y) + 0.5 - center.Y
			distSq := dx*dx + dy*dy

			if distSq >= innerSq && distSq <= outerSq {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func strokeEllipse(img *image.RGBA, center math2d.Vec2, radiusX, radiusY, width float64, bounds image.Rectangle, c color.RGBA) {
	if radiusX <= 0 || radiusY <= 0 || width <= 0 {
		return
	}

	outerRadiusX := radiusX
	outerRadiusY := radiusY
	innerRadiusX := radiusX - width
	innerRadiusY := radiusY - width
	if innerRadiusX < 0 {
		innerRadiusX = 0
	}
	if innerRadiusY < 0 {
		innerRadiusY = 0
	}

	outerRxSq := outerRadiusX * outerRadiusX
	outerRySq := outerRadiusY * outerRadiusY
	outerRR := outerRxSq * outerRySq

	innerRxSq := innerRadiusX * innerRadiusX
	innerRySq := innerRadiusY * innerRadiusY
	innerRR := innerRxSq * innerRySq

	bounds = bounds.Intersect(ellipseBounds(center, outerRadiusX, outerRadiusY))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) + 0.5 - center.X
			dy := float64(y) + 0.5 - center.Y
			distSq := float64(dx*dx)*outerRySq + float64(dy*dy)*outerRxSq
			if distSq <= outerRR {
				if innerRadiusX <= 0 || innerRadiusY <= 0 || float64(dx*dx)*innerRySq+float64(dy*dy)*innerRxSq >= innerRR {
					img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
				}
			}
		}
	}
}

func strokeArc(img *image.RGBA, center math2d.Vec2, radius, width, startAngle, endAngle float64, bounds image.Rectangle, c color.RGBA) {
	if radius <= 0 || width <= 0 {
		return
	}

	outerRadius := radius
	innerRadius := radius - width
	if innerRadius < 0 {
		innerRadius = 0
	}

	outerSq := outerRadius * outerRadius
	innerSq := innerRadius * innerRadius

	bounds = bounds.Intersect(circleBounds(center, outerRadius))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dx := float64(x) + 0.5 - center.X
			dy := float64(y) + 0.5 - center.Y

			distSq := dx*dx + dy*dy
			if distSq < innerSq || distSq > outerSq {
				continue
			}

			angle := math.Atan2(dy, dx)
			if angle < 0 {
				angle += math.Pi * 2
			}

			if angleBetween(angle, startAngle, endAngle) {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func fillRoundedRect(img *image.RGBA, rect image.Rectangle, radius float64, bounds image.Rectangle, c color.RGBA) {
	if rect.Empty() {
		return
	}

	radius = clampRoundedRectRadius(rect, radius)
	if radius <= 0 {
		fillRect(img, rect.Intersect(bounds), c)
		return
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if pointInRoundedRect(float64(x)+0.5, float64(y)+0.5, rect, radius) {
				img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
			}
		}
	}
}

func strokeRoundedRect(img *image.RGBA, rect image.Rectangle, radius, width float64, bounds image.Rectangle, c color.RGBA) {
	if rect.Empty() || width <= 0 {
		return
	}

	radius = clampRoundedRectRadius(rect, radius)
	if radius <= 0 {
		strokeRect(img, rect, int(math.Ceil(width)), bounds, c)
		return
	}

	inner := rect.Inset(int(math.Ceil(width)))
	innerRadius := radius - width
	if innerRadius < 0 {
		innerRadius = 0
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			if !pointInRoundedRect(px, py, rect, radius) {
				continue
			}
			if !inner.Empty() && pointInRoundedRect(px, py, inner, innerRadius) {
				continue
			}
			img.SetRGBA(x, y, blendRGBA(img.RGBAAt(x, y), c))
		}
	}
}

func pointInRoundedRect(x, y float64, rect image.Rectangle, radius float64) bool {
	if x < float64(rect.Min.X) || x >= float64(rect.Max.X) || y < float64(rect.Min.Y) || y >= float64(rect.Max.Y) {
		return false
	}
	if radius <= 0 {
		return true
	}

	left := float64(rect.Min.X) + radius
	right := float64(rect.Max.X) - radius
	top := float64(rect.Min.Y) + radius
	bottom := float64(rect.Max.Y) - radius

	closestX := min(max(x, left), right)
	closestY := min(max(y, top), bottom)
	dx := x - closestX
	dy := y - closestY
	return dx*dx+dy*dy <= radius*radius
}

func clampRoundedRectRadius(rect image.Rectangle, radius float64) float64 {
	if radius <= 0 {
		return 0
	}

	maxRadius := float64(min(rect.Dx(), rect.Dy())) * 0.5
	if radius > maxRadius {
		return maxRadius
	}
	return radius
}
