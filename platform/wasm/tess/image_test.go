package tess

import (
	"math"
	"reflect"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

func TestAppendImageRectProducesSixVerticesWithUnitWAndFullUV(t *testing.T) {
	var mesh TexMesh
	AppendImage(&mesh, testCtx(), render.DrawImage{
		Dest: math2d.NewRect(math2d.Vec2{X: 10, Y: 20}, math2d.Vec2{X: 30, Y: 40}),
	}, 64, 64)

	if len(mesh.Vertices) != 6 {
		t.Fatalf("len(vertices) = %d, want 6", len(mesh.Vertices))
	}

	wantUV := [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	uvByTriangleIndex := [6]int{0, 1, 2, 0, 2, 3}
	for i, v := range mesh.Vertices {
		assertFloat32(t, v.W, 1)
		want := wantUV[uvByTriangleIndex[i]]
		assertFloat32(t, v.U, want[0])
		assertFloat32(t, v.V, want[1])
	}
}

func TestAppendImageQuadRectangularCornersMatchAppendImage(t *testing.T) {
	dest := math2d.NewRect(math2d.Vec2{X: 10, Y: 20}, math2d.Vec2{X: 30, Y: 40})

	var wantMesh TexMesh
	AppendImage(&wantMesh, testCtx(), render.DrawImage{Dest: dest}, 64, 64)

	var gotMesh TexMesh
	AppendImageQuad(&gotMesh, testCtx(), render.DrawImageQuad{
		Corners: [4]math2d.Vec2{
			{X: 10, Y: 20}, {X: 40, Y: 20}, {X: 40, Y: 60}, {X: 10, Y: 60},
		},
	}, 64, 64)

	if len(gotMesh.Vertices) != len(wantMesh.Vertices) {
		t.Fatalf("len(vertices) = %d, want %d", len(gotMesh.Vertices), len(wantMesh.Vertices))
	}
	for i := range wantMesh.Vertices {
		want := wantMesh.Vertices[i]
		got := gotMesh.Vertices[i]
		assertFloat32(t, got.XW, want.XW)
		assertFloat32(t, got.YW, want.YW)
		assertFloat32(t, got.W, want.W)
		assertFloat32(t, got.U, want.U)
		assertFloat32(t, got.V, want.V)
	}
}

func TestAppendImageQuadTrapezoidHasVaryingWAndExactClipCorners(t *testing.T) {
	c := testCtx()

	// A trapezoid narrower at the top than at the bottom: the homography is
	// projective (not affine), so w differs between the top and bottom
	// corners.
	corners := [4]math2d.Vec2{
		{X: 40, Y: 0}, {X: 60, Y: 0}, {X: 90, Y: 100}, {X: 10, Y: 100},
	}

	var mesh TexMesh
	AppendImageQuad(&mesh, c, render.DrawImageQuad{Corners: corners}, 0, 0)

	if len(mesh.Vertices) != 6 {
		t.Fatalf("len(vertices) = %d, want 6", len(mesh.Vertices))
	}

	// Vertex indices 0 and 1 (from the first triangle) are the top corners;
	// index 3 (start of the second triangle, the "d" corner) is a bottom
	// corner. Confirm top and bottom w differ.
	topW := mesh.Vertices[0].W
	bottomW := mesh.Vertices[2].W
	if math.Abs(float64(topW-bottomW)) < testEpsilon {
		t.Fatalf("top w (%v) and bottom w (%v) should differ for a trapezoid", topW, bottomW)
	}

	// Recovering the true clip-space position (XW/W, YW/W) must reproduce
	// screenToClip of each screen corner exactly.
	screenCorners := [4]math2d.Vec2{
		c.cam.WorldToScreen(corners[0]),
		c.cam.WorldToScreen(corners[1]),
		c.cam.WorldToScreen(corners[2]),
		c.cam.WorldToScreen(corners[3]),
	}
	wantClip := [4][2]float32{
		screenToClip(c, screenCorners[0]),
		screenToClip(c, screenCorners[1]),
		screenToClip(c, screenCorners[2]),
		screenToClip(c, screenCorners[3]),
	}
	// Triangle 0: vertices 0,1,2 map to quad corners 0,1,2.
	// Triangle 1: vertices 3,4,5 map to quad corners 0,2,3.
	quadCornerForVertex := [6]int{0, 1, 2, 0, 2, 3}
	for i, v := range mesh.Vertices {
		want := wantClip[quadCornerForVertex[i]]
		assertFloat32(t, v.XW/v.W, want[0])
		assertFloat32(t, v.YW/v.W, want[1])
	}
}

func TestAppendImageSrcSubRectNormalizesUV(t *testing.T) {
	var mesh TexMesh
	AppendImage(&mesh, testCtx(), render.DrawImage{
		Dest: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
		Src:  math2d.NewRectFromCorners(math2d.Vec2{X: 16, Y: 8}, math2d.Vec2{X: 48, Y: 24}),
	}, 64, 32)

	if len(mesh.Vertices) != 6 {
		t.Fatalf("len(vertices) = %d, want 6", len(mesh.Vertices))
	}

	wantUV := [4][2]float32{
		{16.0 / 64, 8.0 / 32},
		{48.0 / 64, 8.0 / 32},
		{48.0 / 64, 24.0 / 32},
		{16.0 / 64, 24.0 / 32},
	}
	quadCornerForVertex := [6]int{0, 1, 2, 0, 2, 3}
	for i, v := range mesh.Vertices {
		want := wantUV[quadCornerForVertex[i]]
		assertFloat32(t, v.U, want[0])
		assertFloat32(t, v.V, want[1])
	}
}

// TestAppendImageSrcClampsToTextureBounds mirrors the raster backend's
// tex.Bounds() intersection: a Src rect that overshoots the texture must be
// clamped rather than producing UV outside [0,1] (which would stretch the
// border under clamp-to-edge sampling instead of shrinking the sampled
// sub-rect like raster does).
func TestAppendImageSrcClampsToTextureBounds(t *testing.T) {
	var mesh TexMesh
	AppendImage(&mesh, testCtx(), render.DrawImage{
		Dest: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
		Src:  math2d.NewRectFromCorners(math2d.Vec2{X: -8, Y: -8}, math2d.Vec2{X: 40, Y: 24}),
	}, 32, 16)

	if len(mesh.Vertices) != 6 {
		t.Fatalf("len(vertices) = %d, want 6", len(mesh.Vertices))
	}

	wantUV := [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	quadCornerForVertex := [6]int{0, 1, 2, 0, 2, 3}
	for i, v := range mesh.Vertices {
		want := wantUV[quadCornerForVertex[i]]
		assertFloat32(t, v.U, want[0])
		assertFloat32(t, v.V, want[1])
	}
}

// TestAppendImageSrcEntirelyOutsideTextureSkipsDraw asserts that a Src rect
// with no overlap with the texture produces no vertices, matching raster's
// empty-intersection skip in drawImage/drawImageQuad.
func TestAppendImageSrcEntirelyOutsideTextureSkipsDraw(t *testing.T) {
	var mesh TexMesh
	AppendImage(&mesh, testCtx(), render.DrawImage{
		Dest: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
		Src:  math2d.NewRectFromCorners(math2d.Vec2{X: 100, Y: 100}, math2d.Vec2{X: 200, Y: 200}),
	}, 32, 16)

	if len(mesh.Vertices) != 0 {
		t.Fatalf("len(vertices) = %d, want 0 for a Src entirely outside the texture", len(mesh.Vertices))
	}
}

// TestAppendImageQuadAffineClipRestrictsGeneratedVertices covers the
// documented affine clip path (image.go AppendImageQuad): a parallelogram
// quad (zero diagonal defect -> homography.IsAffine()) with Clip set must
// actually clip, mirroring TestClipRestrictsGeneratedVertices for the solid
// mesh. This scenario had no test before.
func TestAppendImageQuadAffineClipRestrictsGeneratedVertices(t *testing.T) {
	c := testCtx()
	corners := [4]math2d.Vec2{
		{X: 10, Y: 10}, {X: 50, Y: 10}, {X: 60, Y: 50}, {X: 20, Y: 50},
	}

	_, homography, ok := imageQuadProjection(c, corners)
	if !ok || !homography.IsAffine() {
		t.Fatal("test fixture quad must be a valid affine (parallelogram) quad")
	}

	var unclipped TexMesh
	AppendImageQuad(&unclipped, c, render.DrawImageQuad{Corners: corners}, 0, 0)

	clipBoundary := screenToClip(c, math2d.Vec2{X: 35, Y: 0})[0]
	foundBeyondBoundary := false
	for _, v := range unclipped.Vertices {
		if texClipX(v) > float32(clipBoundary)+testEpsilon {
			foundBeyondBoundary = true
			break
		}
	}
	if !foundBeyondBoundary {
		t.Fatal("test fixture quad must extend past the clip boundary when unclipped")
	}

	var clipped TexMesh
	AppendImageQuad(&clipped, c, render.DrawImageQuad{
		BaseCommand: render.BaseCommand{
			Clip: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 35, Y: 100}),
		},
		Corners: corners,
	}, 0, 0)

	if len(clipped.Vertices) == 0 {
		t.Fatal("clipped affine quad produced no vertices")
	}
	for _, v := range clipped.Vertices {
		if x := texClipX(v); x > float32(clipBoundary)+testEpsilon {
			t.Fatalf("clipped vertex X = %v, want <= %v", x, clipBoundary)
		}
	}
}

// TestAppendImageQuadProjectiveClipIsBypassed covers the documented
// projective bypass (image.go AppendImageQuad, "Clip only when the
// homography is affine"): a genuinely projective (non-affine) quad with Clip
// set must draw the exact same vertices as without Clip. This scenario had
// no test before, and a regression here would silently start distorting the
// texture near the clip boundary of any projective DrawImageQuad.
func TestAppendImageQuadProjectiveClipIsBypassed(t *testing.T) {
	c := testCtx()
	// The same trapezoid as TestAppendImageQuadTrapezoidHasVaryingWAndExactClipCorners:
	// narrower at the top than the bottom, so the homography is projective.
	corners := [4]math2d.Vec2{
		{X: 40, Y: 0}, {X: 60, Y: 0}, {X: 90, Y: 100}, {X: 10, Y: 100},
	}

	_, homography, ok := imageQuadProjection(c, corners)
	if !ok || homography.IsAffine() {
		t.Fatal("test fixture quad must be a valid non-affine (projective) quad")
	}

	var unclipped TexMesh
	AppendImageQuad(&unclipped, c, render.DrawImageQuad{Corners: corners}, 0, 0)

	var clipped TexMesh
	AppendImageQuad(&clipped, c, render.DrawImageQuad{
		BaseCommand: render.BaseCommand{
			Clip: math2d.NewRect(math2d.Vec2{X: 45, Y: 10}, math2d.Vec2{X: 55, Y: 20}),
		},
		Corners: corners,
	}, 0, 0)

	if !reflect.DeepEqual(unclipped.Vertices, clipped.Vertices) {
		t.Fatal("projective quad's vertices changed when Clip was set; Clip must be bypassed for non-affine quads")
	}
}

func TestAppendImageQuadDegenerateCornersProduceNoVertices(t *testing.T) {
	var mesh TexMesh
	AppendImageQuad(&mesh, testCtx(), render.DrawImageQuad{
		Corners: [4]math2d.Vec2{
			{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 20, Y: 0}, {X: 30, Y: 0},
		},
	}, 64, 64)

	if len(mesh.Vertices) != 0 {
		t.Fatalf("len(vertices) = %d, want 0 for collinear corners", len(mesh.Vertices))
	}
}

func TestBuildSkipsNonConvexImageQuadBeforeTextureLookup(t *testing.T) {
	bowtie := render.DrawImageQuad{
		Texture: 1,
		Corners: [4]math2d.Vec2{
			{X: 0, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}, {X: 10, Y: 0},
		},
	}

	for _, sizes := range []TexSizes{nil, {{0, 0}, {16, 16}}} {
		var builder Builder
		runs := builder.Build(runtime.Frame{
			Camera:   testCamera(),
			Commands: []render.Command{bowtie},
		}, 100, 100, nil, sizes)
		if len(runs) != 0 {
			t.Fatalf("invalid quad produced %d runs", len(runs))
		}
	}
}

func TestTexTintZeroIsOpaqueWhiteAndNonZeroIsPremultiplied(t *testing.T) {
	got := texTint(render.Color{})
	want := [4]float32{1, 1, 1, 1}
	if got != want {
		t.Fatalf("texTint(zero) = %v, want %v", got, want)
	}

	tint := render.RGBA(1, 0.5, 0.25, 0.5)
	got = texTint(tint)
	want = [4]float32{
		float32(1 * 0.5),
		float32(0.5 * 0.5),
		float32(0.25 * 0.5),
		float32(0.5),
	}
	if got != want {
		t.Fatalf("texTint(tint) = %v, want %v", got, want)
	}
}

func TestAppendImageIsDeterministic(t *testing.T) {
	cmd := render.DrawImage{
		Dest:     math2d.NewRect(math2d.Vec2{X: 5, Y: 5}, math2d.Vec2{X: 20, Y: 20}),
		Src:      math2d.NewRectFromCorners(math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 16, Y: 16}),
		Tint:     render.RGBA(1, 1, 1, 0.5),
		Rotation: math.Pi / 6,
	}

	var first, second TexMesh
	AppendImage(&first, testCtx(), cmd, 32, 32)
	AppendImage(&second, testCtx(), cmd, 32, 32)

	if !reflect.DeepEqual(first.Vertices, second.Vertices) {
		t.Fatal("AppendImage is not deterministic for identical input")
	}
}

func TestAppendImageQuadIsDeterministic(t *testing.T) {
	cmd := render.DrawImageQuad{
		Corners: [4]math2d.Vec2{
			{X: 5, Y: 5}, {X: 25, Y: 8}, {X: 30, Y: 40}, {X: 2, Y: 35},
		},
		Src:  math2d.NewRectFromCorners(math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 16, Y: 16}),
		Tint: render.RGBA(1, 1, 1, 0.5),
	}

	var first, second TexMesh
	AppendImageQuad(&first, testCtx(), cmd, 32, 32)
	AppendImageQuad(&second, testCtx(), cmd, 32, 32)

	if !reflect.DeepEqual(first.Vertices, second.Vertices) {
		t.Fatal("AppendImageQuad is not deterministic for identical input")
	}
}

func TestTexBytesUseLittleEndianFloat32LayoutAndSize(t *testing.T) {
	mesh := TexMesh{Vertices: []TexVertex{
		{XW: -0.5, YW: 0.25, W: 2, R: 0.1, G: 0.2, B: 0.3, A: 0.4, U: 0.125, V: 0.875},
	}}

	got := TexBytes(mesh)
	if len(got) != len(mesh.Vertices)*40 {
		t.Fatalf("len(TexBytes) = %d, want %d", len(got), len(mesh.Vertices)*40)
	}

	assertEncodedFloat32(t, got, 0, -0.5)
	assertEncodedFloat32(t, got, 4, 0.25)
	assertEncodedFloat32(t, got, 8, 0)
	assertEncodedFloat32(t, got, 12, 2)
	assertEncodedFloat32(t, got, 16, 0.1)
	assertEncodedFloat32(t, got, 32, 0.125)
	assertEncodedFloat32(t, got, 36, 0.875)
}
