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
	if _, err := path.AppendFill(nil, render.BaseCommand{}, vector.FillStyle{Tolerance: 0.1}); !errors.Is(err, vector.ErrFillContour) {
		t.Fatalf("compound fill error = %v", err)
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
