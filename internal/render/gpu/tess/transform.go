package tess

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// The transformed-shape paths below generate a shape's outline in world space,
// map every vertex through the command transform and camera, and draw the
// resulting screen-space polygon. Transforming each boundary vertex (rather
// than only the center or two corners) is what makes rotation, shear and
// non-uniform scale honored, matching the raster backend.

// appendConvexFanScreen fills a convex screen-space polygon by fanning from an
// interior screen-space center. Used for transformed circles and ellipses,
// whose transformed boundary stays convex and star-shaped from the center.
func appendConvexFanScreen(m *Mesh, c ctx, center math2d.Vec2, boundary []math2d.Vec2, col render.Color) {
	clipCenter := screenToClip(c, center)
	n := len(boundary)
	for i := 0; i < n; i++ {
		p0 := screenToClip(c, boundary[i])
		p1 := screenToClip(c, boundary[(i+1)%n])
		pushTri(m, clipCenter, p0, p1, col)
	}
}

// appendClosedStrokeScreen strokes a closed screen-space polygon as a
// connected ring. Adjacent edge quads share their join vertices exactly;
// independent line quads leave subpixel wedges at curved joins under
// fractional transforms.
func appendClosedStrokeScreen(m *Mesh, c ctx, boundary []math2d.Vec2, width float64, col render.Color) {
	n := len(boundary)
	if n < 3 || width <= 0 {
		return
	}

	halfWidth := math.Max(1, width) * 0.5
	if cap(m.strokeJoins) < n {
		m.strokeJoins = make([][2]math2d.Vec2, n)
	} else {
		m.strokeJoins = m.strokeJoins[:n]
	}
	joins := m.strokeJoins
	for i := 0; i < n; i++ {
		point := boundary[i]
		prevDX, prevDY, nextDX, nextDY, ok := boundaryDirections(boundary, i)
		if !ok {
			return
		}

		prevNX, prevNY := -prevDY, prevDX
		nextNX, nextNY := -nextDY, nextDX
		miterX, miterY := prevNX+nextNX, prevNY+nextNY
		miterLength := math.Hypot(miterX, miterY)
		if miterLength <= 1e-12 {
			miterX, miterY = nextNX, nextNY
			miterLength = 1
		}
		miterX /= miterLength
		miterY /= miterLength

		denominator := miterX*nextNX + miterY*nextNY
		offset := halfWidth
		if math.Abs(denominator) > 1e-12 {
			offset /= denominator
		}
		offset = math.Max(-halfWidth*4, math.Min(halfWidth*4, offset))

		joins[i][0] = math2d.Vec2{X: point.X + miterX*offset, Y: point.Y + miterY*offset}
		joins[i][1] = math2d.Vec2{X: point.X - miterX*offset, Y: point.Y - miterY*offset}
	}

	for i := 0; i < n; i++ {
		next := (i + 1) % n
		pushQuad(
			m,
			screenToClip(c, joins[i][0]),
			screenToClip(c, joins[next][0]),
			screenToClip(c, joins[next][1]),
			screenToClip(c, joins[i][1]),
			col,
		)
	}
}

func boundaryDirections(boundary []math2d.Vec2, index int) (prevX, prevY, nextX, nextY float64, ok bool) {
	point := boundary[index]
	prevOK, nextOK := false, false
	for step := 1; step < len(boundary) && (!prevOK || !nextOK); step++ {
		if !prevOK {
			prev := boundary[(index-step+len(boundary))%len(boundary)]
			prevX, prevY, prevOK = unitDirection(prev, point)
		}
		if !nextOK {
			next := boundary[(index+step)%len(boundary)]
			nextX, nextY, nextOK = unitDirection(point, next)
		}
	}
	return prevX, prevY, nextX, nextY, prevOK && nextOK
}

func unitDirection(from, to math2d.Vec2) (x, y float64, ok bool) {
	x = to.X - from.X
	y = to.Y - from.Y
	length := math.Hypot(x, y)
	if length <= 1e-12 {
		return 0, 0, false
	}
	return x / length, y / length, true
}

// appendOpenStrokeScreen strokes an open screen-space polyline (no closing
// edge), used by the transformed arc.
func appendOpenStrokeScreen(m *Mesh, c ctx, points []math2d.Vec2, width float64, col render.Color) {
	for i := 0; i+1 < len(points); i++ {
		appendLineQuad(m, c, points[i], points[i+1], width, col)
	}
}

// transformedStrokeWidth converts a world-space stroke width to screen pixels
// and scales it by the transform's mean linear factor (sqrt|det|), so a stroke
// thickens with its geometry under scale. Identity leaves the width unchanged.
func transformedStrokeWidth(c ctx, worldWidth float64, transform math2d.Affine) float64 {
	return c.cam.WorldScalarToScreen(worldWidth) * transform.MeanScale()
}
