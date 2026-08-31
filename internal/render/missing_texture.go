// Package render holds render diagnostics shared by every backend, like the
// canonical missing-texture placeholder.
package render

import (
	"image"
	"image/color"
	"math"

	"github.com/dhannyell/arasy/math2d"
	publicrender "github.com/dhannyell/arasy/render"
)

const (
	// MissingTextureSize is the canonical placeholder side length in pixels.
	MissingTextureSize = 32

	missingTextureInset       = 0.125
	missingTextureStrokeRatio = 0.08
	missingTextureMaxStroke   = 8.0
)

// MissingTextureBackground returns the placeholder background color.
func MissingTextureBackground() publicrender.Color {
	return publicrender.RGBA8(255, 0, 255, 255)
}

// MissingTextureMarkColor returns the placeholder diagnostic-mark color.
func MissingTextureMarkColor() publicrender.Color {
	return publicrender.RGBA8(0, 0, 0, 255)
}

// MissingTextureMark maps the canonical inset X onto a destination quad.
func MissingTextureMark(corners [4]math2d.Vec2) (lines [2][2]math2d.Vec2, width float64) {
	minimumEdge := math.Inf(1)
	for i := range corners {
		minimumEdge = min(minimumEdge, corners[i].Distance(corners[(i+1)%len(corners)]))
	}
	width = max(1, min(minimumEdge*missingTextureStrokeRatio, missingTextureMaxStroke))
	lines[0] = [2]math2d.Vec2{
		corners[0].Lerp(corners[2], missingTextureInset),
		corners[2].Lerp(corners[0], missingTextureInset),
	}
	lines[1] = [2]math2d.Vec2{
		corners[1].Lerp(corners[3], missingTextureInset),
		corners[3].Lerp(corners[1], missingTextureInset),
	}
	return lines, width
}

// MissingTextureRGBA builds the canonical opaque-magenta placeholder with a black X.
func MissingTextureRGBA() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, MissingTextureSize, MissingTextureSize))
	background := color.RGBA{R: 255, B: 255, A: 255}
	mark := color.RGBA{A: 255}
	for y := 0; y < MissingTextureSize; y++ {
		for x := 0; x < MissingTextureSize; x++ {
			img.SetRGBA(x, y, background)
		}
	}

	corners := [4]math2d.Vec2{
		{},
		{X: MissingTextureSize},
		{X: MissingTextureSize, Y: MissingTextureSize},
		{Y: MissingTextureSize},
	}
	lines, width := MissingTextureMark(corners)
	radiusSq := width * width / 4
	for y := 0; y < MissingTextureSize; y++ {
		for x := 0; x < MissingTextureSize; x++ {
			point := math2d.Vec2{X: float64(x) + 0.5, Y: float64(y) + 0.5}
			if pointSegmentDistanceSq(point, lines[0][0], lines[0][1]) <= radiusSq ||
				pointSegmentDistanceSq(point, lines[1][0], lines[1][1]) <= radiusSq {
				img.SetRGBA(x, y, mark)
			}
		}
	}
	return img
}

func pointSegmentDistanceSq(point, from, to math2d.Vec2) float64 {
	delta := to.Sub(from)
	lengthSq := delta.LenSq()
	if lengthSq == 0 {
		return point.DistanceSq(from)
	}
	t := max(0, min(1, point.Sub(from).Dot(delta)/lengthSq))
	return point.DistanceSq(from.Add(delta.Mul(t)))
}
