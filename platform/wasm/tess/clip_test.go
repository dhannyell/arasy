package tess

import (
	"math"
	"testing"
)

// TestClipTexTrianglesClipsToRectangleAndInterpolatesUV directly exercises
// clipTexTriangles (only reached transitively through AppendImage/
// AppendImageQuad's Clip path otherwise): a unit quad in clip space, with UV
// spanning [0,1] linearly across X, is clipped to its right half. Every
// resulting vertex must stay within the clip rectangle, and U must remain
// the same linear function of clip-space X the source quad defined --
// proving the Sutherland-Hodgman intersection point carries the interpolated
// attributes, not just position.
func TestClipTexTrianglesClipsToRectangleAndInterpolatesUV(t *testing.T) {
	verts := []TexVertex{
		{XW: -1, YW: -1, W: 1, U: 0, V: 0},
		{XW: 1, YW: -1, W: 1, U: 1, V: 0},
		{XW: 1, YW: 1, W: 1, U: 1, V: 1},
		{XW: -1, YW: -1, W: 1, U: 0, V: 0},
		{XW: 1, YW: 1, W: 1, U: 1, V: 1},
		{XW: -1, YW: 1, W: 1, U: 0, V: 1},
	}

	// Keep only the right half: clip-space x in [0, 1].
	clipped := clipTexTriangles(verts, [4]float32{0, 1, -1, 1})

	if len(clipped) == 0 {
		t.Fatal("clipped triangle list is empty")
	}
	if len(clipped)%3 != 0 {
		t.Fatalf("len(clipped) = %d, want a multiple of 3", len(clipped))
	}

	for _, v := range clipped {
		x := texClipX(v)
		if x < -testEpsilon || x > 1+testEpsilon {
			t.Fatalf("vertex X = %v, want within [0,1]", x)
		}
		wantU := (x + 1) / 2
		if math.Abs(float64(v.U-wantU)) > testEpsilon {
			t.Fatalf("interpolated U = %v, want %v (linear in clip-space X)", v.U, wantU)
		}
	}
}

// TestClipTexTrianglesDiscardsFullyOutsideTriangle confirms a triangle
// wholly outside the clip rectangle is dropped rather than producing
// degenerate/empty-area vertices.
func TestClipTexTrianglesDiscardsFullyOutsideTriangle(t *testing.T) {
	verts := []TexVertex{
		{XW: 2, YW: 2, W: 1},
		{XW: 3, YW: 2, W: 1},
		{XW: 3, YW: 3, W: 1},
	}

	clipped := clipTexTriangles(verts, [4]float32{-1, 1, -1, 1})

	if len(clipped) != 0 {
		t.Fatalf("len(clipped) = %d, want 0 for a triangle fully outside the clip rect", len(clipped))
	}
}
