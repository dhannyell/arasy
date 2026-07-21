package tess

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

// clipToScreen inverts screenToClip for the 100×100 test context, so a
// transformed shape's clip-space vertices can be checked against expected
// screen-space geometry.
func clipToScreen(v Vertex) math2d.Vec2 {
	return math2d.Vec2{X: (float64(v.X) + 1) * 50, Y: (1 - float64(v.Y)) * 50}
}

func uniqueScreenPoints(vertices []Vertex) []math2d.Vec2 {
	var out []math2d.Vec2
	for _, v := range vertices {
		p := clipToScreen(v)
		found := false
		for _, q := range out {
			if math.Abs(p.X-q.X) < 1e-4 && math.Abs(p.Y-q.Y) < 1e-4 {
				found = true
				break
			}
		}
		if !found {
			out = append(out, p)
		}
	}
	return out
}

func containsPoint(points []math2d.Vec2, want math2d.Vec2) bool {
	for _, p := range points {
		if math.Abs(p.X-want.X) < 1e-3 && math.Abs(p.Y-want.Y) < 1e-3 {
			return true
		}
	}
	return false
}

func screenBounds(vertices []Vertex) (minX, minY, maxX, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, v := range vertices {
		p := clipToScreen(v)
		minX = math.Min(minX, p.X)
		minY = math.Min(minY, p.Y)
		maxX = math.Max(maxX, p.X)
		maxY = math.Max(maxY, p.Y)
	}
	return minX, minY, maxX, maxY
}

// rectWorldCorners returns a NewRect(min, size)'s four world corners.
func rectWorldCorners(min, size math2d.Vec2) [4]math2d.Vec2 {
	return [4]math2d.Vec2{
		{X: min.X, Y: min.Y},
		{X: min.X + size.X, Y: min.Y},
		{X: min.X + size.X, Y: min.Y + size.Y},
		{X: min.X, Y: min.Y + size.Y},
	}
}

// TestAppendRectShearProducesParallelogramNotAABB is the regression guard for
// the original bug: a sheared rect transformed only its Min/Max corners and
// re-derived an axis-aligned box. The fill must be the true parallelogram,
// i.e. every one of the four sheared corners appears and the AABB-only corner
// does not.
func TestAppendRectShearProducesParallelogramNotAABB(t *testing.T) {
	min, size := math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 30, Y: 20}
	transform := math2d.Shear(0.5, 0)

	var mesh Mesh
	AppendRect(&mesh, testCtx(), render.DrawRect{
		BaseCommand: render.BaseCommand{Transform: transform},
		Rect:        math2d.NewRect(min, size),
		Fill:        render.RGBA(1, 0, 0, 1),
	})

	if len(mesh.Vertices) != 6 {
		t.Fatalf("len(vertices) = %d, want 6 (two triangles)", len(mesh.Vertices))
	}

	got := uniqueScreenPoints(mesh.Vertices)
	if len(got) != 4 {
		t.Fatalf("unique corners = %d (%v), want 4", len(got), got)
	}
	for _, w := range rectWorldCorners(min, size) {
		if tw := transform.Apply(w); !containsPoint(got, tw) {
			t.Fatalf("sheared corner %v missing from fill %v", tw, got)
		}
	}
	// (55,10) belongs to the AABB of the transformed Min/Max corners
	// ([15,55]×[10,30]) but not to the parallelogram; the old bug would draw it.
	if containsPoint(got, math2d.Vec2{X: 55, Y: 10}) {
		t.Fatal("fill contains AABB corner (55,10); rect was not truly sheared")
	}
}

// TestAppendRectRotationHonoredByAllCorners rotates a rect about its center and
// checks all four rotated corners appear (not an axis-aligned bounding box).
func TestAppendRectRotationHonoredByAllCorners(t *testing.T) {
	min, size := math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 30, Y: 20}
	center := math2d.Vec2{X: min.X + size.X/2, Y: min.Y + size.Y/2}
	transform := math2d.Rotate(math.Pi / 2).About(center)

	var mesh Mesh
	AppendRect(&mesh, testCtx(), render.DrawRect{
		BaseCommand: render.BaseCommand{Transform: transform},
		Rect:        math2d.NewRect(min, size),
		Fill:        render.RGBA(0, 1, 0, 1),
	})

	got := uniqueScreenPoints(mesh.Vertices)
	for _, w := range rectWorldCorners(min, size) {
		if tw := transform.Apply(w); !containsPoint(got, tw) {
			t.Fatalf("rotated corner %v missing from fill %v", tw, got)
		}
	}
}

// TestAppendCircleNonUniformScaleBecomesEllipse guards the circle bug: only the
// center was transformed, leaving an isotropic radius. Under a non-uniform
// scale the boundary must widen/narrow per axis.
func TestAppendCircleNonUniformScaleBecomesEllipse(t *testing.T) {
	center := math2d.Vec2{X: 50, Y: 50}
	var mesh Mesh
	AppendCircle(&mesh, testCtx(), render.DrawCircle{
		BaseCommand: render.BaseCommand{Transform: math2d.Scale(math2d.Vec2{X: 2, Y: 0.5}).About(center)},
		Center:      center,
		Radius:      10,
		Fill:        render.RGBA(0, 0, 1, 1),
	})

	if len(mesh.Vertices) == 0 {
		t.Fatal("transformed circle produced no vertices")
	}
	minX, minY, maxX, maxY := screenBounds(mesh.Vertices)
	width := maxX - minX
	height := maxY - minY
	// Radius 10 scaled ×2 horizontally and ×0.5 vertically → 40 wide, 10 tall.
	if math.Abs(width-40) > 0.5 {
		t.Fatalf("ellipse width = %v, want ~40", width)
	}
	if math.Abs(height-10) > 0.5 {
		t.Fatalf("ellipse height = %v, want ~10", height)
	}
}

// TestBuildTransformedImageUsesTexturedRunNotInstancedSprite verifies a
// transformed DrawImage leaves the instanced sprite fast path (which carries no
// per-sprite matrix) and draws as an explicit textured quad instead.
func TestBuildTransformedImageUsesTexturedRunNotInstancedSprite(t *testing.T) {
	sizes := TexSizes{{}, {8, 8}}
	dest := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 30, Y: 30})

	var builder Builder
	runs := builder.Build(runtime.Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			render.DrawImage{
				BaseCommand: render.BaseCommand{Transform: math2d.Rotate(0.4).About(math2d.Vec2{X: 20, Y: 20})},
				Texture:     1,
				Dest:        dest,
			},
		},
	}, 100, 100, nil, sizes)

	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0].Kind != DrawRunTextured {
		t.Fatalf("run kind = %v, want DrawRunTextured for a transformed sprite", runs[0].Kind)
	}
}

// TestAppendImageQuadTransformMatchesPreTransformedCorners confirms the command
// transform is applied to the quad corners: transforming via BaseCommand.
// Transform must equal baking the same transform into the corners directly.
func TestAppendImageQuadTransformMatchesPreTransformedCorners(t *testing.T) {
	transform := math2d.Rotate(0.3).About(math2d.Vec2{X: 20, Y: 20})
	corners := [4]math2d.Vec2{
		{X: 10, Y: 10}, {X: 30, Y: 10}, {X: 30, Y: 30}, {X: 10, Y: 30},
	}

	var viaTransform TexMesh
	AppendImageQuad(&viaTransform, testCtx(), render.DrawImageQuad{
		BaseCommand: render.BaseCommand{Transform: transform},
		Corners:     corners,
	}, 8, 8)

	baked := corners
	for i := range baked {
		baked[i] = transform.Apply(baked[i])
	}
	var viaCorners TexMesh
	AppendImageQuad(&viaCorners, testCtx(), render.DrawImageQuad{Corners: baked}, 8, 8)

	if len(viaTransform.Vertices) != len(viaCorners.Vertices) || len(viaTransform.Vertices) == 0 {
		t.Fatalf("vertex counts differ: %d vs %d", len(viaTransform.Vertices), len(viaCorners.Vertices))
	}
	for i := range viaCorners.Vertices {
		a, b := viaTransform.Vertices[i], viaCorners.Vertices[i]
		assertFloat32(t, a.XW, b.XW)
		assertFloat32(t, a.YW, b.YW)
		assertFloat32(t, a.W, b.W)
	}
}
