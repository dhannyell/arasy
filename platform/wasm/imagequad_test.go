package wasm

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func gridSizeForCorners(t *testing.T, corners [4]math2d.Vec2) int {
	t.Helper()
	homography, ok := math2d.HomographyFromQuad(corners)
	if !ok {
		t.Fatalf("HomographyFromQuad(%v) ok = false, want true", corners)
	}
	return imageQuadGridSize(corners, homography)
}

func TestImageQuadGridSizeParallelogramIsOne(t *testing.T) {
	corners := [4]math2d.Vec2{
		{X: 100, Y: 100},
		{X: 300, Y: 120},
		{X: 340, Y: 320},
		{X: 140, Y: 300},
	}
	if got := gridSizeForCorners(t, corners); got != 1 {
		t.Fatalf("grid size = %d, want 1 for a parallelogram (no perspective)", got)
	}
}

func TestImageQuadGridSizeSubToleranceTaperIsOne(t *testing.T) {
	// A barely tapered small trapezoid: the center error is below the
	// tolerance, so the quad renders as a single cell.
	corners := [4]math2d.Vec2{
		{X: 100, Y: 100},
		{X: 120, Y: 100},
		{X: 120.5, Y: 116},
		{X: 99.5, Y: 116},
	}
	if got := gridSizeForCorners(t, corners); got != 1 {
		t.Fatalf("grid size = %d, want 1 when the taper error is sub-tolerance", got)
	}
}

func TestImageQuadGridSizeNoteScaleTrapezoidStaysSmall(t *testing.T) {
	// A note-sprite-sized trapezoid with visible perspective taper must use
	// a small grid, not the worst-case maximum.
	corners := [4]math2d.Vec2{
		{X: 100, Y: 100},
		{X: 140, Y: 100},
		{X: 145, Y: 132},
		{X: 95, Y: 132},
	}
	if got := gridSizeForCorners(t, corners); got > 4 {
		t.Fatalf("grid size = %d, want <= 4 for a note-scale trapezoid", got)
	}
}

func TestImageQuadGridSizeCapsAtMax(t *testing.T) {
	// Extreme foreshortening: a huge base collapsing to a narrow far edge.
	corners := [4]math2d.Vec2{
		{X: 630, Y: 10},
		{X: 650, Y: 10},
		{X: 1280, Y: 960},
		{X: 0, Y: 960},
	}
	if got := gridSizeForCorners(t, corners); got != imageQuadMaxGridSize {
		t.Fatalf("grid size = %d, want cap %d for extreme perspective", got, imageQuadMaxGridSize)
	}
}

func TestImageQuadGridSizeBoundsCellError(t *testing.T) {
	// The tolerance is guaranteed whenever the grid does not hit the cap;
	// capped quads accept the residual error, matching the old fixed grid.
	quads := []struct {
		name    string
		corners [4]math2d.Vec2
	}{
		{
			name: "golden-scene quad",
			corners: [4]math2d.Vec2{
				{X: 377, Y: 217}, {X: 563, Y: 198}, {X: 608, Y: 371}, {X: 339, Y: 396},
			},
		},
		{
			name: "highway trapezoid",
			corners: [4]math2d.Vec2{
				{X: 540, Y: 190}, {X: 740, Y: 190}, {X: 900, Y: 790}, {X: 380, Y: 790},
			},
		},
	}
	for _, tc := range quads {
		t.Run(tc.name, func(t *testing.T) {
			homography, ok := math2d.HomographyFromQuad(tc.corners)
			if !ok {
				t.Fatalf("HomographyFromQuad ok = false, want true")
			}
			n := imageQuadGridSize(tc.corners, homography)
			if n < 1 || n > imageQuadMaxGridSize {
				t.Fatalf("grid size = %d, want within [1, %d]", n, imageQuadMaxGridSize)
			}
			err := imageQuadMaxCellErrorPx(homography, n)
			if n < imageQuadMaxGridSize && err > imageQuadTolerancePx {
				t.Fatalf("max cell error = %v px at grid %d, want <= %v", err, n, imageQuadTolerancePx)
			}
		})
	}
}
