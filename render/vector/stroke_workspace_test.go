package vector

import (
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestAppendStrokeWithWorkspaceMatchesStatelessOutput(t *testing.T) {
	var pathBuilder Builder
	pathBuilder.MoveTo(math2d.Vec2{})
	pathBuilder.CubicTo(
		math2d.Vec2{X: 20, Y: 30},
		math2d.Vec2{X: 40, Y: -10},
		math2d.Vec2{X: 60, Y: 20},
	)
	path, err := pathBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}

	var workspace StrokeWorkspace
	for _, trimEnd := range []float64{0.35, 0.8, 1} {
		style := StrokeStyle{
			Color:     render.RGBA8(120, 80, 240, 255),
			Width:     3,
			LineCap:   CapRound,
			LineJoin:  JoinRound,
			TrimEnd:   trimEnd,
			Tolerance: 0.25,
		}
		want, err := path.AppendStroke(nil, render.BaseCommand{}, style)
		if err != nil {
			t.Fatal(err)
		}
		got, err := path.AppendStrokeWithWorkspace(nil, render.BaseCommand{}, style, &workspace)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("trim end %g: workspace output differs from stateless output", trimEnd)
		}
	}
}

func TestAppendStrokeWithWorkspaceReusesMeshBuffers(t *testing.T) {
	var pathBuilder Builder
	pathBuilder.MoveTo(math2d.Vec2{})
	pathBuilder.LineTo(math2d.Vec2{X: 20})
	pathBuilder.LineTo(math2d.Vec2{X: 30, Y: 10})
	path, err := pathBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}

	style := StrokeStyle{Width: 2, TrimEnd: 0.75, Tolerance: 0.25}
	var workspace StrokeWorkspace
	if _, err := path.AppendStrokeWithWorkspace(nil, render.BaseCommand{}, style, &workspace); err != nil {
		t.Fatal(err)
	}
	firstVertex := &workspace.vertices[:cap(workspace.vertices)][0]
	firstIndex := &workspace.indices[:cap(workspace.indices)][0]
	firstTrimmed := &workspace.trimmed[:cap(workspace.trimmed)][0]

	if _, err := path.AppendStrokeWithWorkspace(nil, render.BaseCommand{}, style, &workspace); err != nil {
		t.Fatal(err)
	}
	if &workspace.vertices[:cap(workspace.vertices)][0] != firstVertex {
		t.Fatal("vertex buffer was reallocated")
	}
	if &workspace.indices[:cap(workspace.indices)][0] != firstIndex {
		t.Fatal("index buffer was reallocated")
	}
	if &workspace.trimmed[:cap(workspace.trimmed)][0] != firstTrimmed {
		t.Fatal("trim buffer was reallocated")
	}
}
