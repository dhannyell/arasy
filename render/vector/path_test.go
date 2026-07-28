package vector_test

import (
	"errors"
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/render/vector"
)

func TestFlattenCurvesRespondsToTolerance(t *testing.T) {
	var builder vector.Builder
	builder.MoveTo(math2d.Vec2{})
	builder.LineTo(math2d.Vec2{X: 1})
	builder.QuadTo(math2d.Vec2{X: 2, Y: 3}, math2d.Vec2{X: 4})
	builder.CubicTo(math2d.Vec2{X: 5, Y: -4}, math2d.Vec2{X: 7, Y: 4}, math2d.Vec2{X: 8})
	path, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	coarse, err := path.AppendStroke(nil, render.BaseCommand{}, vector.StrokeStyle{
		Color: render.RGBA(1, 1, 1, 1), Width: 1, Tolerance: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	fine, err := path.AppendStroke(nil, render.BaseCommand{}, vector.StrokeStyle{
		Color: render.RGBA(1, 1, 1, 1), Width: 1, Tolerance: 0.01,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fine[0].(render.DrawTriangles).Vertices) <= len(coarse[0].(render.DrawTriangles).Vertices) {
		t.Fatal("smaller tolerance did not produce more flattened vertices")
	}
}

func TestFillSupportsConcaveContourAndBuilderComposition(t *testing.T) {
	path := polygonPath(t, []math2d.Vec2{{}, {X: 4}, {X: 2, Y: 2}, {X: 4, Y: 4}, {Y: 4}})
	commands, err := path.AppendFill(nil, render.BaseCommand{}, vector.FillStyle{
		Color: render.RGBA(1, 0, 0, 0.8), Tolerance: 0.1,
	})
	if err != nil {
		t.Fatal(err)
	}
	var commandBuilder render.Builder
	commandBuilder.Reset(nil)
	commandBuilder.Push(render.Group{
		Transform: math2d.Translate(math2d.Vec2{X: 10}),
		Opacity:   0.5,
		Clip:      math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
	})
	commandBuilder.Add(commands[0])
	commandBuilder.Pop()
	command := commandBuilder.Commands()[0].(render.DrawTriangles)
	if command.Color.A != 0.4 || command.Transform.Apply(command.Vertices[0]).X != 10 {
		t.Fatalf("composed command = %+v", command)
	}
}

func TestFillSupportsDisconnectedContoursAndHoles(t *testing.T) {
	var builder vector.Builder
	// Counter-clockwise outer contour.
	builder.MoveTo(math2d.Vec2{})
	builder.LineTo(math2d.Vec2{X: 10})
	builder.LineTo(math2d.Vec2{X: 10, Y: 10})
	builder.LineTo(math2d.Vec2{Y: 10})
	builder.Close()
	// Clockwise inner contour: the non-zero winding rule makes it a hole.
	builder.MoveTo(math2d.Vec2{X: 3, Y: 3})
	builder.LineTo(math2d.Vec2{X: 3, Y: 7})
	builder.LineTo(math2d.Vec2{X: 7, Y: 7})
	builder.LineTo(math2d.Vec2{X: 7, Y: 3})
	builder.Close()
	// A disconnected contour remains filled regardless of its orientation.
	builder.MoveTo(math2d.Vec2{X: 12})
	builder.LineTo(math2d.Vec2{X: 14})
	builder.LineTo(math2d.Vec2{X: 14, Y: 2})
	builder.LineTo(math2d.Vec2{X: 12, Y: 2})
	builder.Close()
	path, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	commands, err := path.AppendFill(nil, render.BaseCommand{}, vector.FillStyle{
		Color: render.RGBA(1, 1, 1, 1), Tolerance: 0.1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(commands))
	}
	triangles := commands[0].(render.DrawTriangles)
	if got, want := triangleMeshArea(triangles), 88.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("filled area = %v, want %v", got, want)
	}
	if triangleMeshContains(triangles, math2d.Vec2{X: 5, Y: 5}) {
		t.Fatal("counter-form center was filled")
	}
	if !triangleMeshContains(triangles, math2d.Vec2{X: 1, Y: 1}) {
		t.Fatal("outer contour was not filled")
	}
	if !triangleMeshContains(triangles, math2d.Vec2{X: 13, Y: 1}) {
		t.Fatal("disconnected contour was not filled")
	}
}

func TestFillCompoundUsesNonZeroWinding(t *testing.T) {
	var builder vector.Builder
	for _, offset := range []float64{0, 3} {
		size := 10 - offset*2
		builder.MoveTo(math2d.Vec2{X: offset, Y: offset})
		builder.LineTo(math2d.Vec2{X: offset + size, Y: offset})
		builder.LineTo(math2d.Vec2{X: offset + size, Y: offset + size})
		builder.LineTo(math2d.Vec2{X: offset, Y: offset + size})
		builder.Close()
	}
	path, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	commands, err := path.AppendFill(nil, render.BaseCommand{}, vector.FillStyle{
		Color: render.RGBA(1, 1, 1, 1), Tolerance: 0.1,
	})
	if err != nil {
		t.Fatal(err)
	}
	triangles := commands[0].(render.DrawTriangles)
	if got, want := triangleMeshArea(triangles), 100.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("filled area = %v, want %v", got, want)
	}
	if !triangleMeshContains(triangles, math2d.Vec2{X: 5, Y: 5}) {
		t.Fatal("same-winding nested contour incorrectly created a hole")
	}
}

func TestFillRejectsUnsupportedGeometry(t *testing.T) {
	self := polygonPath(t, []math2d.Vec2{{}, {X: 4, Y: 4}, {X: 4}, {Y: 4}})
	if _, err := self.AppendFill(nil, render.BaseCommand{}, vector.FillStyle{Tolerance: 0.1}); !errors.Is(err, vector.ErrSelfIntersection) {
		t.Fatalf("self-intersection error = %v", err)
	}

	var builder vector.Builder
	builder.MoveTo(math2d.Vec2{})
	builder.LineTo(math2d.Vec2{X: 1})
	builder.LineTo(math2d.Vec2{Y: 1})
	builder.Close()
	builder.MoveTo(math2d.Vec2{X: 0.2, Y: 0.2})
	builder.LineTo(math2d.Vec2{X: 0.3, Y: 0.2})
	builder.LineTo(math2d.Vec2{X: 0.2, Y: 0.3})
	builder.Close()
	path, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := path.AppendFill(nil, render.BaseCommand{}, vector.FillStyle{Tolerance: 0.1}); err != nil {
		t.Fatalf("compound fill error = %v", err)
	}

	var open vector.Builder
	open.MoveTo(math2d.Vec2{})
	open.LineTo(math2d.Vec2{X: 1})
	open.LineTo(math2d.Vec2{Y: 1})
	openPath, err := open.Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openPath.AppendFill(nil, render.BaseCommand{}, vector.FillStyle{Tolerance: 0.1}); !errors.Is(err, vector.ErrFillContour) {
		t.Fatalf("open contour error = %v", err)
	}
}

func TestStrokeTrimCapsAndJoins(t *testing.T) {
	var builder vector.Builder
	builder.MoveTo(math2d.Vec2{})
	builder.LineTo(math2d.Vec2{X: 10})
	builder.LineTo(math2d.Vec2{X: 10, Y: 10})
	path, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []vector.StrokeStyle{
		{Width: 2, LineCap: vector.CapButt, LineJoin: vector.JoinBevel, TrimStart: 0.25, TrimEnd: 0.75, Tolerance: 0.1},
		{Width: 2, LineCap: vector.CapRound, LineJoin: vector.JoinRound, Tolerance: 0.1},
		{Width: 2, LineCap: vector.CapSquare, LineJoin: vector.JoinMiter, MiterLimit: 4, Tolerance: 0.1},
	} {
		commands, err := path.AppendStroke(nil, render.BaseCommand{}, style)
		if err != nil {
			t.Fatal(err)
		}
		if len(commands) != 1 || len(commands[0].(render.DrawTriangles).Indices) == 0 {
			t.Fatalf("style %+v produced no geometry", style)
		}
	}
}

func TestPointAtUsesFlattenedLength(t *testing.T) {
	var builder vector.Builder
	builder.MoveTo(math2d.Vec2{})
	builder.LineTo(math2d.Vec2{X: 10})
	builder.LineTo(math2d.Vec2{X: 10, Y: 10})
	path, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		progress float64
		point    math2d.Vec2
		tangent  math2d.Vec2
	}{
		{0, math2d.Vec2{}, math2d.Vec2{X: 1}},
		{0.5, math2d.Vec2{X: 10}, math2d.Vec2{X: 1}},
		{1, math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{Y: 1}},
	}
	for _, test := range tests {
		point, tangent, err := path.PointAt(test.progress, 0.1)
		if err != nil || point != test.point || tangent != test.tangent {
			t.Fatalf("PointAt(%v) = %v,%v,%v", test.progress, point, tangent, err)
		}
	}
}

func TestPathRejectsNonFiniteAndFlattenDepth(t *testing.T) {
	var invalid vector.Builder
	invalid.MoveTo(math2d.Vec2{X: math.NaN()})
	if _, err := invalid.Build(); !errors.Is(err, vector.ErrNonFinite) {
		t.Fatalf("non-finite error = %v", err)
	}

	var builder vector.Builder
	builder.MoveTo(math2d.Vec2{})
	builder.CubicTo(
		math2d.Vec2{X: 1e9, Y: 1e9},
		math2d.Vec2{X: -1e9, Y: 1e9},
		math2d.Vec2{X: 1},
	)
	path, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := path.PointAt(0.5, math.SmallestNonzeroFloat64); !errors.Is(err, vector.ErrFlattenDepth) {
		t.Fatalf("depth error = %v", err)
	}
}

func polygonPath(t *testing.T, points []math2d.Vec2) vector.Path {
	t.Helper()
	var builder vector.Builder
	builder.MoveTo(points[0])
	for _, point := range points[1:] {
		builder.LineTo(point)
	}
	builder.Close()
	path, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func triangleMeshArea(mesh render.DrawTriangles) float64 {
	area := 0.0
	for index := 0; index < len(mesh.Indices); index += 3 {
		a := mesh.Vertices[mesh.Indices[index]]
		b := mesh.Vertices[mesh.Indices[index+1]]
		c := mesh.Vertices[mesh.Indices[index+2]]
		area += math.Abs((b.X-a.X)*(c.Y-a.Y)-(b.Y-a.Y)*(c.X-a.X)) * 0.5
	}
	return area
}

func triangleMeshContains(mesh render.DrawTriangles, point math2d.Vec2) bool {
	for index := 0; index < len(mesh.Indices); index += 3 {
		a := mesh.Vertices[mesh.Indices[index]]
		b := mesh.Vertices[mesh.Indices[index+1]]
		c := mesh.Vertices[mesh.Indices[index+2]]
		ab := (b.X-a.X)*(point.Y-a.Y) - (b.Y-a.Y)*(point.X-a.X)
		bc := (c.X-b.X)*(point.Y-b.Y) - (c.Y-b.Y)*(point.X-b.X)
		ca := (a.X-c.X)*(point.Y-c.Y) - (a.Y-c.Y)*(point.X-c.X)
		if ab >= 0 && bc >= 0 && ca >= 0 || ab <= 0 && bc <= 0 && ca <= 0 {
			return true
		}
	}
	return false
}
