package raster

import (
	"image"
	"math"

	"github.com/dhannyell/frameforge/math2d"
)

func (r *Renderer) clipBounds(img *image.RGBA, bounds image.Rectangle, clip math2d.Rect) image.Rectangle {
	bounds = bounds.Intersect(img.Bounds())
	if isZeroRect(clip) {
		return bounds
	}

	screenMin := r.Camera.WorldToScreen(clip.Min)
	screenMax := r.Camera.WorldToScreen(clip.Max)
	clipRect := image.Rect(
		int(math.Floor(min(screenMin.X, screenMax.X))),
		int(math.Floor(min(screenMin.Y, screenMax.Y))),
		int(math.Ceil(max(screenMin.X, screenMax.X))),
		int(math.Ceil(max(screenMin.Y, screenMax.Y))),
	)
	return bounds.Intersect(clipRect)
}

func circleBounds(center math2d.Vec2, radius float64) image.Rectangle {
	return image.Rect(
		int(math.Floor(center.X-radius)),
		int(math.Floor(center.Y-radius)),
		int(math.Ceil(center.X+radius))+1,
		int(math.Ceil(center.Y+radius))+1,
	)
}

func lineBounds(from, to math2d.Vec2, width float64) image.Rectangle {
	radius := width * 0.5
	return image.Rect(
		int(math.Floor(min(from.X, to.X)-radius)),
		int(math.Floor(min(from.Y, to.Y)-radius)),
		int(math.Ceil(max(from.X, to.X)+radius))+1,
		int(math.Ceil(max(from.Y, to.Y)+radius))+1,
	)
}

func polygonBounds(points []math2d.Vec2) image.Rectangle {
	if len(points) == 0 {
		return image.Rectangle{}
	}

	minX := points[0].X
	maxX := points[0].X
	minY := points[0].Y
	maxY := points[0].Y
	for _, point := range points[1:] {
		minX = min(minX, point.X)
		maxX = max(maxX, point.X)
		minY = min(minY, point.Y)
		maxY = max(maxY, point.Y)
	}

	return image.Rect(
		int(math.Floor(minX)),
		int(math.Floor(minY)),
		int(math.Ceil(maxX))+1,
		int(math.Ceil(maxY))+1,
	)
}

func rotatedRectBounds(anchor math2d.Vec2, offsetX, offsetY, width, height, angle float64) image.Rectangle {
	if angle == 0 {
		return image.Rect(
			int(math.Floor(anchor.X+offsetX)),
			int(math.Floor(anchor.Y+offsetY)),
			int(math.Ceil(anchor.X+offsetX+width)),
			int(math.Ceil(anchor.Y+offsetY+height)),
		)
	}

	corners := rotatedRectCorners(anchor, offsetX, offsetY, width, height, angle)

	minX := math.Inf(1)
	maxX := math.Inf(-1)
	minY := math.Inf(1)
	maxY := math.Inf(-1)
	for _, corner := range corners {
		minX = min(minX, corner.X)
		maxX = max(maxX, corner.X)
		minY = min(minY, corner.Y)
		maxY = max(maxY, corner.Y)
	}

	return image.Rect(
		int(math.Floor(minX)),
		int(math.Floor(minY)),
		int(math.Ceil(maxX)),
		int(math.Ceil(maxY)),
	)
}

// rotatedRectCorners returns the four corners, in screen space, of a
// destination rect anchored at anchor with the given local-space offset and
// size, rotated by angle around anchor. It mirrors the per-pixel transform
// blitScaledRotated inverts, so the placeholder polygon and the eventual
// image blit always agree on the area they cover.
func rotatedRectCorners(anchor math2d.Vec2, offsetX, offsetY, width, height, angle float64) [4]math2d.Vec2 {
	cosAngle := math.Cos(angle)
	sinAngle := math.Sin(angle)
	local := [4]math2d.Vec2{
		{X: offsetX, Y: offsetY},
		{X: offsetX + width, Y: offsetY},
		{X: offsetX + width, Y: offsetY + height},
		{X: offsetX, Y: offsetY + height},
	}

	var corners [4]math2d.Vec2
	for i, corner := range local {
		corners[i] = math2d.Vec2{
			X: anchor.X + corner.X*cosAngle - corner.Y*sinAngle,
			Y: anchor.Y + corner.X*sinAngle + corner.Y*cosAngle,
		}
	}
	return corners
}

func isZeroRect(rect math2d.Rect) bool {
	return rect.Min == (math2d.Vec2{}) && rect.Max == (math2d.Vec2{})
}

func angleBetween(angle float64, start, end float64) bool {
	angle = normalizeAngle(angle)
	start = normalizeAngle(start)
	end = normalizeAngle(end)

	if start <= end {
		return angle >= start && angle <= end
	}

	return angle >= start || angle <= end
}

func normalizeAngle(angle float64) float64 {
	const tau = math.Pi * 2

	angle = math.Mod(angle, tau)
	if angle < 0 {
		angle += tau
	}

	return angle
}
