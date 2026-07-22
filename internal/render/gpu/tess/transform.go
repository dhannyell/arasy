package tess

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
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

// appendClosedStrokeScreen strokes a closed screen-space polygon as one quad
// per edge, matching the raster backend's transformed-shape stroke.
func appendClosedStrokeScreen(m *Mesh, c ctx, boundary []math2d.Vec2, width float64, col render.Color) {
	n := len(boundary)
	for i := 0; i < n; i++ {
		appendLineQuad(m, c, boundary[i], boundary[(i+1)%n], width, col)
	}
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
