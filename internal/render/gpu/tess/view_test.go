package tess

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// skewCtx frames the same 100x100 viewport as testCtx, but through an
// isometric view. Its mean scale is 1, so the world it shows is the same size
// as the one testCtx shows and the two are comparable.
func skewCtx() ctx {
	c := testCtx()
	c.cam.View = math2d.Affine{A: 1, C: -1, B: 0.5, D: 0.5}
	c.cam.World = math2d.NewRect(math2d.Vec2{X: -50, Y: -50}, math2d.Vec2{X: 100, Y: 100})

	return c
}

// clipCorners returns the distinct clip-space positions in a mesh, which for a
// filled quad or a fan is the shape's boundary.
func meshHas(mesh Mesh, want [2]float32) bool {
	for _, vertex := range mesh.Vertices {
		if math.Abs(float64(vertex.X-want[0])) < testEpsilon &&
			math.Abs(float64(vertex.Y-want[1])) < testEpsilon {
			return true
		}
	}

	return false
}

// A rectangle under a skewing view is a parallelogram, and the fast path that
// builds one from a screen min and max cannot express that. Both GPU backends
// share this tessellator, so this is where the two of them are held to the
// raster reference at once.
func TestSkewingViewGivesARectItsFourWorldCorners(t *testing.T) {
	c := skewCtx()

	var mesh Mesh
	rect := math2d.NewRectFromCorners(math2d.Vec2{X: 4, Y: 6}, math2d.Vec2{X: 20, Y: 14})
	AppendRect(&mesh, c, render.DrawRect{Rect: rect, Fill: render.RGBA(1, 0, 0, 1)})

	corners := []math2d.Vec2{
		{X: rect.Min.X, Y: rect.Min.Y},
		{X: rect.Max.X, Y: rect.Min.Y},
		{X: rect.Max.X, Y: rect.Max.Y},
		{X: rect.Min.X, Y: rect.Max.Y},
	}

	for _, corner := range corners {
		want := screenToClip(c, c.cam.WorldToScreen(corner))
		if !meshHas(mesh, want) {
			t.Fatalf("corner %v is missing from the mesh; it was drawn as an upright rect", corner)
		}
	}
}

// The same rectangle drawn without a view has to come out upright, or the
// fast path has been given up for every game that never asked for one.
func TestNoViewKeepsTheRectUpright(t *testing.T) {
	c := testCtx()

	var mesh Mesh
	rect := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 30, Y: 20})
	AppendRect(&mesh, c, render.DrawRect{Rect: rect, Fill: render.RGBA(1, 0, 0, 1)})

	if len(mesh.Vertices) != 6 {
		t.Fatalf("an unskewed rect emitted %d vertices, want 6", len(mesh.Vertices))
	}

	// Two x values and two y values between them: the definition of upright.
	xs, ys := map[float32]bool{}, map[float32]bool{}
	for _, vertex := range mesh.Vertices {
		xs[vertex.X] = true
		ys[vertex.Y] = true
	}

	if len(xs) != 2 || len(ys) != 2 {
		t.Fatalf("an unskewed rect used %d x values and %d y values, want 2 and 2",
			len(xs), len(ys))
	}
}

// A circle under a skewing view is a slanted ellipse. Rebuilding it on screen
// from a centre and one radius draws a circle, which is the give-away that the
// view was only used to move it.
func TestSkewingViewSlantsACircle(t *testing.T) {
	c := skewCtx()

	var mesh Mesh
	AppendCircle(&mesh, c, render.DrawCircle{
		Center: math2d.Vec2{X: 10, Y: 10},
		Radius: 8,
		Fill:   render.RGBA(1, 0, 0, 1),
	})

	if len(mesh.Vertices) == 0 {
		t.Fatal("no circle was drawn")
	}

	var minX, maxX, minY, maxY float32
	for i, vertex := range mesh.Vertices {
		if i == 0 {
			minX, maxX, minY, maxY = vertex.X, vertex.X, vertex.Y, vertex.Y
			continue
		}
		minX, maxX = min(minX, vertex.X), max(maxX, vertex.X)
		minY, maxY = min(minY, vertex.Y), max(maxY, vertex.Y)
	}

	// This view halves the vertical, so the drawn shape has to be visibly
	// wider than it is tall in clip space, whose axes are already equalised.
	width, height := float64(maxX-minX), float64(maxY-minY)
	if width <= height*1.5 {
		t.Fatalf("circle bounds are %vx%v, which is not a slanted ellipse", width, height)
	}
}

// A point is a small circle, and a skewing view has to slant it like one.
// The fast path rebuilds it from a centre and a radius, which stays
// axis-aligned while every shape around it shears.
func TestSkewingViewSlantsAPoint(t *testing.T) {
	c := skewCtx()

	var mesh Mesh
	AppendPoint(&mesh, c, render.DrawPoint{
		Center: math2d.Vec2{X: 10, Y: 10},
		Radius: 8,
		Color:  render.RGBA(1, 0, 0, 1),
	})

	if len(mesh.Vertices) == 0 {
		t.Fatal("no point was drawn")
	}

	var minX, maxX, minY, maxY float32
	for i, vertex := range mesh.Vertices {
		if i == 0 {
			minX, maxX, minY, maxY = vertex.X, vertex.X, vertex.Y, vertex.Y
			continue
		}
		minX, maxX = min(minX, vertex.X), max(maxX, vertex.X)
		minY, maxY = min(minY, vertex.Y), max(maxY, vertex.Y)
	}

	width, height := float64(maxX-minX), float64(maxY-minY)
	if width <= height*1.5 {
		t.Fatalf("point bounds are %vx%v, which is not a slanted ellipse", width, height)
	}
}

// The segment count is taken from the screen radius, which is measured before
// the view scales the shape up. A view that magnifies would otherwise draw a
// large circle as a dodecagon.
func TestSegmentCountFollowsTheViewsScale(t *testing.T) {
	c := testCtx()
	c.cam.View = math2d.Affine{A: 20, C: -20, B: 10, D: 10}
	c.cam.World = math2d.NewRect(math2d.Vec2{X: -50, Y: -50}, math2d.Vec2{X: 100, Y: 100})

	var coarse, fine Mesh
	command := render.DrawCircle{
		Center: math2d.Vec2{X: 2, Y: 2},
		Radius: 2,
		Fill:   render.RGBA(1, 0, 0, 1),
	}

	AppendCircle(&fine, c, command)

	c.cam.View = math2d.Affine{}
	AppendCircle(&coarse, c, command)

	if len(fine.Vertices) <= len(coarse.Vertices) {
		t.Fatalf("a twenty-times larger circle got %d vertices against %d, so the view "+
			"was not counted", len(fine.Vertices), len(coarse.Vertices))
	}
}
