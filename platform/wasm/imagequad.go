package wasm

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
)

const (
	imageQuadMaxGridSize = 16
	// imageQuadTolerancePx bounds the screen-space error accepted when the
	// projective quad map is approximated by a grid of affine triangles.
	// Canvas2D is a development-host approximation, so half a pixel keeps
	// the warp invisible while letting shallow quads use tiny grids.
	imageQuadTolerancePx = 0.5
)

// imageQuadGridSize returns the per-axis subdivision count needed so a grid
// of affine triangles approximates homography within imageQuadTolerancePx on
// screen. corners are the screen-space quad corners; homography maps the
// unit square onto them.
//
// The count starts from the single-cell center error (piecewise-linear error
// shrinks quadratically with subdivision) and is then verified against the
// actual per-cell error, growing until the tolerance holds or the grid
// reaches imageQuadMaxGridSize.
func imageQuadGridSize(corners [4]math2d.Vec2, homography math2d.Homography) int {
	center := homography.Apply(math2d.Vec2{X: 0.5, Y: 0.5})
	diagonalMid := corners[1].Add(corners[3]).Mul(0.5)
	errPx := math.Hypot(center.X-diagonalMid.X, center.Y-diagonalMid.Y)
	if math.IsNaN(errPx) || math.IsInf(errPx, 0) {
		return imageQuadMaxGridSize
	}
	if errPx <= imageQuadTolerancePx {
		return 1
	}

	n := int(math.Ceil(math.Sqrt(errPx / imageQuadTolerancePx)))
	if n < 1 {
		n = 1
	}
	if n > imageQuadMaxGridSize {
		return imageQuadMaxGridSize
	}
	for n < imageQuadMaxGridSize && imageQuadMaxCellErrorPx(homography, n) > imageQuadTolerancePx {
		n++
	}
	return n
}

// imageQuadMaxCellErrorPx samples every cell of an n x n grid at its center
// and reports the worst distance between the true projective image and the
// affine estimate used by the triangle pair (the midpoint of the cell's
// shared diagonal).
func imageQuadMaxCellErrorPx(homography math2d.Homography, n int) float64 {
	worst := 0.0
	for row := 0; row < n; row++ {
		v0 := float64(row) / float64(n)
		v1 := float64(row+1) / float64(n)
		vMid := (v0 + v1) / 2
		for col := 0; col < n; col++ {
			u0 := float64(col) / float64(n)
			u1 := float64(col+1) / float64(n)
			uMid := (u0 + u1) / 2

			p10 := homography.Apply(math2d.Vec2{X: u1, Y: v0})
			p01 := homography.Apply(math2d.Vec2{X: u0, Y: v1})
			estimate := p10.Add(p01).Mul(0.5)
			truth := homography.Apply(math2d.Vec2{X: uMid, Y: vMid})

			err := math.Hypot(truth.X-estimate.X, truth.Y-estimate.Y)
			if math.IsNaN(err) || math.IsInf(err, 0) {
				return math.Inf(1)
			}
			if err > worst {
				worst = err
			}
		}
	}
	return worst
}
